package react

import (
	"path/filepath"
	"sort"
	"strconv"

	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/lang"
	"github.com/matheusAle/coderefinery/internal/syntax"
)

// Unit kinds and entry kinds.
const (
	KindComponent = "component"
	KindHook      = "hook"
	KindCallback  = "callback" // a hook's callback argument: Cart#useEffect#1
	KindHandler   = "handler"  // a local function or inline JSX handler: Cart.onSubmit
)

// Props summarises a component's props.
type Props struct {
	Count       int
	Boolean     int
	Optional    int
	Callback    int
	Rest        bool
	PassThrough []string // received and only forwarded unchanged to child components
	Open        bool     // the props type has parts that could not be resolved: the counts are lower bounds
}

// Hooks summarises a unit's hook usage.
type Hooks struct {
	State         int // useState calls
	Effects       int // useEffect / useLayoutEffect / useInsertionEffect calls
	MaxDeps       int // largest dependency array literal
	SetterEffects int // effects whose body only calls state setters
}

// Cost summarises render-cost risks.
type Cost struct {
	Depth           int // deepest JSX element nesting (fragments not counted)
	Elements        int // JSX elements (fragments not counted)
	InlineFunctions int // JSX attributes whose value is a function literal
	InlineObjects   int // JSX attributes whose value is an object or array literal
	Nested          int // components defined inside this unit
}

// Unit is one component or custom hook.
type Unit struct {
	Name      string
	Namespace string
	Kind      string
	Path      string
	Line      int
	Render    int // 1 + decisions in returned JSX, early returns, .map lists
	Logic     int // 1 + the other decisions charged to the unit
	Props     Props
	Hooks     Hooks
	Cost      Cost
}

// Entry is one CRAP-scored row: a unit body, a hook callback or a handler.
type Entry struct {
	Name       string
	Namespace  string
	Kind       string
	Unit       string
	Path       string
	StartLine  int
	EndLine    int
	Complexity int
	Coverage   *float64
	Crap       *float64
}

// File is everything found in one source file.
type File struct {
	Units   []Unit
	Entries []Entry
}

// analyzeSource parses one file (path relative to the project root) and
// returns its units and entries in source order. Props types imported from
// other modules are followed through the run's resolver.
func (r *resolver) analyzeSource(text, path, projectRoot string) (File, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(r.root, path)
	}
	mod := r.files[abs]
	if mod == nil {
		tree, err := lang.ParseText(lang.GrammarFor(path), text)
		if err != nil {
			return File{}, err
		}
		mod = r.add(abs, tree)
	}
	tree := mod.tree
	a := &analyzer{src: tree.Source, root: tree.Root, path: path, types: r, mod: mod,
		namespace: crap.TypeScriptModuleNamespace(path, projectRoot)}
	tree.Root.Descendants(func(n *syntax.Node) bool {
		if n.Type != "function_declaration" && n.Type != "variable_declarator" {
			return true
		}
		if insideFunction(n) {
			return true
		}
		if d, ok := definitionOf(a.src, n); ok {
			if isComponent(d) {
				a.unit(d, d.name, KindComponent)
			} else if isHook(d) {
				a.unit(d, d.name, KindHook)
			}
		}
		return true
	})
	sort.SliceStable(a.out.Entries, func(i, j int) bool { return a.out.Entries[i].StartLine < a.out.Entries[j].StartLine })
	sort.SliceStable(a.out.Units, func(i, j int) bool { return a.out.Units[i].Line < a.out.Units[j].Line })
	return a.out, nil
}

type analyzer struct {
	src       []byte
	root      *syntax.Node
	path      string
	namespace string
	types     *resolver
	mod       *tsModule // this file, as the resolver indexed it
	out       File
}

func (a *analyzer) entry(span *syntax.Node, name, kind, unit string, complexity int) {
	a.out.Entries = append(a.out.Entries, Entry{
		Name: name, Namespace: a.namespace, Kind: kind, Unit: unit, Path: a.path,
		StartLine: span.StartLine(), EndLine: span.EndLine(), Complexity: complexity,
	})
}

// subComplexity scores an extracted callback or handler like crap scores a
// function: every decision in its subtree, nested callbacks included.
func subComplexity(fn *syntax.Node) int {
	return crap.Complexity(fn, crap.TypeScriptDecision, func(*syntax.Node) bool { return false })
}

// unitWalk holds the state of one unit's walk.
type unitWalk struct {
	a        *analyzer
	unit     *Unit
	setters  map[string]bool
	used     map[string]bool // claimed entry names
	hookSeen map[string]int  // hook name -> callbacks numbered so far
	cc       int             // crap decisions charged to the unit
	render   int             // render decisions (crap decisions in render, plus .map lists)
	logic    int             // crap decisions outside render
}

func (a *analyzer) unit(d definition, name, kind string) {
	u := &Unit{Name: name, Namespace: a.namespace, Kind: kind, Path: a.path, Line: d.span.StartLine()}
	w := &unitWalk{a: a, unit: u, setters: stateSetters(a.src, d.fn),
		used: map[string]bool{name: true}, hookSeen: map[string]int{}}
	body := d.fn.ChildByFieldName("body")
	if body != nil {
		w.visit(body, body.Type != "statement_block", true, 0)
	}
	u.Render, u.Logic = 1+w.render, 1+w.logic
	if kind == KindComponent {
		u.Props = a.props(d)
	}
	a.entry(d.span, name, kind, name, 1+w.cc)
	a.out.Units = append(a.out.Units, *u)
}

// claim returns the first free name among name, name#2, name#3...
func (w *unitWalk) claim(name string) string {
	if !w.used[name] {
		w.used[name] = true
		return name
	}
	for k := 2; ; k++ {
		c := name + "#" + strconv.Itoa(k)
		if !w.used[c] {
			w.used[c] = true
			return c
		}
	}
}

// visit walks n. inRender: n is part of what the unit renders. owned: no
// function lies between n and the unit. depth: enclosing JSX elements.
func (w *unitWalk) visit(n *syntax.Node, inRender, owned bool, depth int) {
	src := w.a.src
	if isFunction(n) {
		if w.extract(n, owned) {
			return
		}
		owned = false
	}
	if n.Type == "class_declaration" || n.Type == "class" {
		return
	}
	early := false
	switch {
	case n.Type == "return_statement" && owned && w.unit.Kind == KindComponent:
		inRender = true
	case n.Type == "if_statement" && owned && w.unit.Kind == KindComponent && returnsEarly(n):
		early = true
	case isElement(n):
		inRender = true
		if !isFragment(n) {
			depth++
			w.unit.Cost.Elements++
			w.unit.Cost.Depth = max(w.unit.Cost.Depth, depth)
		}
	case n.Type == "jsx_attribute":
		if v := attributeValue(n); v != nil && (v.Type == "object" || v.Type == "array") {
			w.unit.Cost.InlineObjects++
		}
	case n.Type == "call_expression":
		if inRender && isMapList(src, n) {
			w.render++
		}
		if hook := hookCall(src, n); hook != "" {
			w.hook(n, hook)
		}
	}
	if crap.TypeScriptDecision(n) {
		w.cc++
		if inRender || early {
			w.render++
		} else {
			w.logic++
		}
	}
	for _, c := range n.Children {
		// An early return's condition decides what renders.
		w.visit(c, inRender || (early && c.Field == "condition"), owned, depth)
	}
}

// extract turns a nested function into its own entry (or unit) and reports
// whether it did; the caller then skips the subtree.
func (w *unitWalk) extract(fn *syntax.Node, owned bool) bool {
	src, u := w.a.src, w.unit
	parent := upParen(fn)
	switch {
	case parent != nil && parent.Type == "jsx_expression" && parent.Parent != nil && parent.Parent.Type == "jsx_attribute":
		u.Cost.InlineFunctions++
		name := attributeName(src, parent.Parent)
		w.a.entry(fn, w.claim(u.Name+"."+name), KindHandler, u.Name, subComplexity(fn))
		return true
	case parent != nil && parent.Type == "arguments" && hookCall(src, parent.Parent) != "":
		hook := hookCall(src, parent.Parent)
		w.hookSeen[hook]++
		name := u.Name + "#" + hook + "#" + strconv.Itoa(w.hookSeen[hook])
		w.a.entry(fn, w.claim(name), KindCallback, u.Name, subComplexity(fn))
		return true
	}
	if !owned {
		return false
	}
	d, ok := localDefinition(src, fn)
	if !ok {
		return false
	}
	if isComponent(d) {
		u.Cost.Nested++
		w.a.unit(d, w.claim(u.Name+"."+d.name), KindComponent)
		return true
	}
	w.a.entry(d.span, w.claim(u.Name+"."+d.name), KindHandler, u.Name, subComplexity(fn))
	return true
}

// localDefinition finds the definition a nested function belongs to:
// `function f() {}` or `const f = fn` (memo/forwardRef wrappers included).
func localDefinition(src []byte, fn *syntax.Node) (definition, bool) {
	if fn.Type == "function_declaration" {
		return definitionOf(src, fn)
	}
	p := upParen(fn)
	for p != nil && p.Type == "arguments" && wrappers[calleeName(src, p.Parent)] {
		p = upParen(p.Parent)
	}
	if p == nil || p.Type != "variable_declarator" {
		return definition{}, false
	}
	d, ok := definitionOf(src, p)
	if !ok || d.fn != fn {
		return definition{}, false
	}
	return d, true
}

// hook records one hook call's metrics.
func (w *unitWalk) hook(call *syntax.Node, hook string) {
	src, h := w.a.src, &w.unit.Hooks
	args := arguments(call)
	switch {
	case hook == "useState":
		h.State++
	case effectHooks[hook]:
		h.Effects++
		if len(args) > 0 && setterOnly(src, unparen(args[0]), w.setters) {
			h.SetterEffects++
		}
	}
	if depHooks[hook] && len(args) > 1 {
		if deps := unparen(args[len(args)-1]); deps != nil && deps.Type == "array" {
			h.MaxDeps = max(h.MaxDeps, len(nonComment(deps.NamedChildren())))
		}
	}
}

func nonComment(nodes []*syntax.Node) []*syntax.Node {
	var out []*syntax.Node
	for _, n := range nodes {
		if n.Type != "comment" {
			out = append(out, n)
		}
	}
	return out
}

// returnsEarly: an if statement whose consequence is, or directly holds, a return.
func returnsEarly(n *syntax.Node) bool {
	c := n.ChildByFieldName("consequence")
	if c == nil {
		return false
	}
	if c.Type == "return_statement" {
		return true
	}
	return c.Type == "statement_block" && c.HasChildOfType("return_statement")
}

// isMapList: `xs.map(...)`, a list render when it sits in rendered JSX.
func isMapList(src []byte, call *syntax.Node) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Type != "member_expression" {
		return false
	}
	prop := fn.ChildByFieldName("property")
	return prop != nil && prop.Text(src) == "map"
}

// attributeValue is the expression of `name={expr}`, nil otherwise.
func attributeValue(attr *syntax.Node) *syntax.Node {
	for _, c := range attr.Children {
		if c.Type == "jsx_expression" {
			for _, e := range c.NamedChildren() {
				if e.Type != "comment" {
					return unparen(e)
				}
			}
		}
	}
	return nil
}

func attributeName(src []byte, attr *syntax.Node) string {
	if len(attr.Children) > 0 {
		return attr.Children[0].Text(src)
	}
	return "attr"
}

// stateSetters collects the setter names of `const [x, setX] = useState(...)`
// in fn (nested functions excluded).
func stateSetters(src []byte, fn *syntax.Node) map[string]bool {
	setters := map[string]bool{}
	body := fn.ChildByFieldName("body")
	if body == nil {
		return setters
	}
	syntax.Walk(body, func(n *syntax.Node) bool {
		if isFunction(n) {
			return false
		}
		if n.Type != "variable_declarator" || hookCall(src, unparen(n.ChildByFieldName("value"))) != "useState" {
			return true
		}
		if pat := n.ChildByFieldName("name"); pat != nil && pat.Type == "array_pattern" {
			if ids := nonComment(pat.NamedChildren()); len(ids) > 1 && ids[1].Type == "identifier" {
				setters[ids[1].Text(src)] = true
			}
		}
		return true
	})
	return setters
}

// setterOnly reports whether an effect callback only calls state setters:
// every statement is a setter call, or an if whose branches are.
func setterOnly(src []byte, fn *syntax.Node, setters map[string]bool) bool {
	if fn == nil || (fn.Type != "arrow_function" && fn.Type != "function_expression") || len(setters) == 0 {
		return false
	}
	body := fn.ChildByFieldName("body")
	if body == nil {
		return false
	}
	if body.Type != "statement_block" {
		return isSetterCall(src, unparen(body), setters)
	}
	return settersOnlyBlock(src, body, setters)
}

func settersOnlyBlock(src []byte, block *syntax.Node, setters map[string]bool) bool {
	stmts := nonComment(block.NamedChildren())
	if len(stmts) == 0 {
		return false
	}
	for _, s := range stmts {
		if !setterStatement(src, s, setters) {
			return false
		}
	}
	return true
}

func setterStatement(src []byte, s *syntax.Node, setters map[string]bool) bool {
	switch s.Type {
	case "expression_statement":
		exprs := nonComment(s.NamedChildren())
		return len(exprs) == 1 && isSetterCall(src, unparen(exprs[0]), setters)
	case "statement_block":
		return settersOnlyBlock(src, s, setters)
	case "if_statement":
		for _, field := range []string{"consequence", "alternative"} {
			branch := s.ChildByFieldName(field)
			if branch == nil {
				continue
			}
			if branch.Type == "else_clause" {
				inner := nonComment(branch.NamedChildren())
				if len(inner) != 1 {
					return false
				}
				branch = inner[0]
			}
			if !setterStatement(src, branch, setters) {
				return false
			}
		}
		return true
	}
	return false
}

func isSetterCall(src []byte, n *syntax.Node, setters map[string]bool) bool {
	if n == nil || n.Type != "call_expression" {
		return false
	}
	fn := n.ChildByFieldName("function")
	return fn != nil && fn.Type == "identifier" && setters[fn.Text(src)]
}
