package mutate

// React mutation operators (mutate --react, docs/design-react.md §13). A
// coderefinery addition: upstream mutator has no React operators. They read
// the same TSX/JSX tree as the upstream rules and add sites next to them, so
// every site still belongs to a crapper function (its form id), keeps the
// upstream mutation id shape and goes through the same coverage gate,
// selection, snapshot and --attribute paths. Each site carries a Label, the
// description the site log and the scan print instead of `<orig> -> <mutant>`.

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// depHooks are the React hooks whose last argument is a dependency array
// (the `deps` metric of `coderefinery react` reads the same set).
var depHooks = map[string]bool{
	"useEffect": true, "useLayoutEffect": true, "useInsertionEffect": true,
	"useMemo": true, "useCallback": true, "useImperativeHandle": true,
}

// keptProps are never dropped: `key` only steers reconciliation, and
// `className` / `style` are presentation a behaviour test cannot observe.
var keptProps = map[string]bool{"key": true, "className": true, "style": true}

const labelWidth = 40

type reactFinder struct {
	data  []byte
	found []RawSite
}

// reactSites returns the React operators' sites of a parsed TS/TSX/JS tree,
// in tree order (discoverRaw sorts them with the upstream sites).
func reactSites(root *syntax.Node, data []byte) []RawSite {
	f := &reactFinder{data: data}
	visit(root, f.node)
	return f.found
}

func (f *reactFinder) node(n *syntax.Node) {
	switch n.Type {
	case "binary_expression":
		f.conditionalRender(n)
	case "ternary_expression":
		f.swapBranches(n)
	case "jsx_attribute":
		f.attribute(n)
	case "call_expression":
		f.dependencies(n)
	}
}

// add records the replacement of data[start:end] by mutant; at is the node
// whose line and statement anchor the site.
func (f *reactFinder) add(at *syntax.Node, start, end int, mutant, label string) {
	original := string(f.data[start:end])
	if mutant == original {
		return
	}
	f.found = append(f.found, RawSite{
		Line:       at.StartPoint.Row + 1,
		AnchorLine: statementLine(at),
		Start:      start,
		End:        end,
		Original:   original,
		Mutant:     mutant,
		Category:   "react",
		Label:      label,
	})
}

func (f *reactFinder) text(n *syntax.Node) string { return n.Text(f.data) }

// unparen strips parenthesized_expression wrappers.
func unparen(n *syntax.Node) *syntax.Node {
	for n != nil && n.Type == "parenthesized_expression" {
		inner := n.NamedChildren()
		if len(inner) != 1 {
			return n
		}
		n = inner[0]
	}
	return n
}

func isJSX(n *syntax.Node) bool {
	n = unparen(n)
	return n != nil && (n.Type == "jsx_element" || n.Type == "jsx_self_closing_element")
}

// elementLabel is `<Name/>` for a self-closing element, `<Name>` for an
// element with children and `<>` for a fragment.
func (f *reactFinder) elementLabel(n *syntax.Node) string {
	n = unparen(n)
	if n.Type == "jsx_self_closing_element" {
		return "<" + f.text(n.ChildByFieldName("name")) + "/>"
	}
	open := n.ChildByFieldName("open_tag")
	if open == nil || open.ChildByFieldName("name") == nil {
		return "<>"
	}
	return "<" + f.text(open.ChildByFieldName("name")) + ">"
}

// short is a branch or dependency as a label: whitespace runs collapsed,
// cut to labelWidth characters with `…`.
func short(text string) string {
	text = strings.Join(strings.FieldsFunc(text, unicode.IsSpace), " ")
	if utf8.RuneCountInString(text) <= labelWidth {
		return text
	}
	return string([]rune(text)[:labelWidth-1]) + "…"
}

func (f *reactFinder) branchLabel(n *syntax.Node) string {
	if isJSX(n) {
		return f.elementLabel(n)
	}
	return short(f.text(n))
}

// conditionalRender: `cond && <X/>` becomes `false` (drop render) and
// `<X/>` (always render).
func (f *reactFinder) conditionalRender(n *syntax.Node) {
	right := n.ChildByFieldName("right")
	if right == nil || !isJSX(right) || !hasOperator(n, "&&") {
		return
	}
	el := f.elementLabel(right)
	f.add(n, n.StartByte, n.EndByte, "false", "drop render "+el)
	f.add(n, n.StartByte, n.EndByte, f.text(right), "always render "+el)
}

// hasOperator: op is the binary expression's own operator token (an
// anonymous direct child, not one inside an operand).
func hasOperator(n *syntax.Node, op string) bool {
	for _, c := range n.Children {
		if !c.Named && c.Type == op {
			return true
		}
	}
	return false
}

// swapBranches: `c ? <A/> : <B/>` becomes `c ? <B/> : <A/>` when a branch
// renders JSX. The replaced span runs from the consequence to the end of the
// alternative, so the condition stays out of the mutation id.
func (f *reactFinder) swapBranches(n *syntax.Node) {
	yes, no := n.ChildByFieldName("consequence"), n.ChildByFieldName("alternative")
	if yes == nil || no == nil || !isJSX(yes) && !isJSX(no) {
		return
	}
	mutant := f.text(no) + string(f.data[yes.EndByte:no.StartByte]) + f.text(yes)
	f.add(n, yes.StartByte, no.EndByte, mutant, "swap ternary "+f.branchLabel(yes)+" : "+f.branchLabel(no))
}

// attribute: a handler `onX={...}` becomes `onX={() => {}}`; any other
// attribute but keptProps is dropped. Spreads (`{...rest}`) are not
// jsx_attribute nodes and stay.
func (f *reactFinder) attribute(n *syntax.Node) {
	named := n.NamedChildren()
	if len(named) == 0 {
		return
	}
	name := f.text(named[0])
	var value *syntax.Node
	if len(named) > 1 {
		value = named[1]
	}
	if isHandlerName(name) {
		f.noopHandler(n, name, value)
		return
	}
	if keptProps[name] {
		return
	}
	element := n.Parent // jsx_self_closing_element, or jsx_opening_element of a jsx_element
	if element.Type == "jsx_opening_element" {
		element = element.Parent
	}
	f.add(n, n.StartByte, n.EndByte, "", "drop prop "+name+" from "+f.elementLabel(element))
}

// isHandlerName: `on` then an upper-case letter (onClick, onValueChange).
func isHandlerName(name string) bool {
	rest, ok := strings.CutPrefix(name, "on")
	if !ok || rest == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return unicode.IsUpper(r)
}

func (f *reactFinder) noopHandler(n *syntax.Node, name string, value *syntax.Node) {
	if value == nil || value.Type != "jsx_expression" {
		return
	}
	inner := value.NamedChildren()
	if len(inner) != 1 || inner[0].Extra {
		return
	}
	expr := inner[0]
	f.add(n, expr.StartByte, expr.EndByte, "() => {}", "noop "+name)
}

// hookName is the dependency hook a call invokes (`useEffect(...)`,
// `React.useEffect(...)`), "" for any other call.
func (f *reactFinder) hookName(call *syntax.Node) string {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return ""
	}
	name := f.text(fn)
	if fn.Type == "member_expression" {
		name = f.text(fn.ChildByFieldName("property"))
	} else if fn.Type != "identifier" {
		return ""
	}
	if depHooks[name] {
		return name
	}
	return ""
}

// dependencies: each element of a dependency-array literal is removed in
// turn, with the comma that separates it from its neighbour.
func (f *reactFinder) dependencies(call *syntax.Node) {
	hook := f.hookName(call)
	if hook == "" {
		return
	}
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return
	}
	var last *syntax.Node
	for _, a := range args.NamedChildren() {
		if !a.Extra {
			last = a
		}
	}
	if last == nil || last.Type != "array" {
		return
	}
	var deps []*syntax.Node
	for _, d := range last.NamedChildren() {
		if !d.Extra {
			deps = append(deps, d)
		}
	}
	for i, d := range deps {
		start, end := d.StartByte, d.EndByte
		switch {
		case i+1 < len(deps):
			end = deps[i+1].StartByte
		case i > 0:
			start = deps[i-1].EndByte
		}
		f.add(d, start, end, "", "drop dep "+short(f.text(d))+" from "+hook)
	}
}
