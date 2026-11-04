package react

import (
	"strings"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// member is what a props type says about one prop.
type member struct{ optional, boolean, callback bool }

// members is the evaluated member list of a props type. open: part of the
// type could not be resolved (a library or DOM type, a mapped or conditional
// type, a missing module...), so the list is a lower bound: only known
// members are listed, nothing is guessed.
type members struct {
	order []string
	by    map[string]*member
	open  bool
}

func (ms *members) add(name string, m member) {
	if ms.by == nil {
		ms.by = map[string]*member{}
	}
	if old, ok := ms.by[name]; ok { // A & B, interface extends: optional only if optional in both
		old.optional = old.optional && m.optional
		old.boolean = old.boolean || m.boolean
		old.callback = old.callback || m.callback
		return
	}
	ms.by[name] = &m
	ms.order = append(ms.order, name)
}

func (ms *members) merge(other members) {
	for _, name := range other.order {
		ms.add(name, *other.by[name])
	}
	ms.open = ms.open || other.open
}

func openMembers() members { return members{open: true} }

// typeRef is a type expression with the module it is written in and the
// type parameters in scope there.
type typeRef struct {
	mod  *tsModule
	node *syntax.Node // nil: a type parameter with no argument and no default
	env  scope
}

type scope map[string]typeRef

// maxTypeDepth bounds the evaluation (deeply nested or self-referencing
// aliases): past it the type is open.
const maxTypeDepth = 32

// eval evaluates the members of a props type.
type eval struct {
	r      *resolver
	active map[*syntax.Node]bool // declarations being expanded: a cycle back to one is open
}

func (r *resolver) propsMembers(mod *tsModule, t *syntax.Node) members {
	e := &eval{r: r, active: map[*syntax.Node]bool{}}
	return e.members(typeRef{mod: mod, node: t}, 0)
}

func (e *eval) members(t typeRef, depth int) members {
	if t.node == nil || depth > maxTypeDepth {
		return openMembers()
	}
	src := t.mod.src
	switch n := t.node; n.Type {
	case "type_annotation", "parenthesized_type", "readonly_type":
		inner := nonComment(n.NamedChildren())
		if len(inner) != 1 {
			return openMembers()
		}
		return e.members(typeRef{t.mod, inner[0], t.env}, depth+1)
	case "intersection_type":
		var out members
		for _, c := range nonComment(n.NamedChildren()) {
			out.merge(e.members(typeRef{t.mod, c, t.env}, depth+1))
		}
		return out
	case "union_type":
		return e.union(t, depth)
	case "object_type", "interface_body":
		return objectMembers(src, n)
	case "type_identifier":
		name := n.Text(src)
		if arg, ok := t.env[name]; ok {
			return e.members(arg, depth+1)
		}
		return e.named(t.mod, name, nil, t, depth)
	case "nested_type_identifier":
		return e.qualified(t.mod, n, nil, t, depth)
	case "generic_type":
		return e.generic(t, depth)
	case "literal_type":
		return members{}
	case "predefined_type":
		switch n.Text(src) {
		case "any", "unknown", "object":
			return openMembers()
		}
		return members{}
	}
	// function, conditional, mapped, typeof, indexed access, import("x").T...
	return openMembers()
}

// union lists every member of every branch: a member missing from some
// branch is optional. `undefined` and `null` branches are skipped.
func (e *eval) union(t typeRef, depth int) members {
	var branches []members
	for _, c := range nonComment(t.node.NamedChildren()) {
		if text := c.Text(t.mod.src); text == "undefined" || text == "null" {
			continue
		}
		branches = append(branches, e.members(typeRef{t.mod, c, t.env}, depth+1))
	}
	var out members
	for _, b := range branches {
		out.open = out.open || b.open
		for _, name := range b.order {
			m := *b.by[name]
			for _, other := range branches {
				if _, ok := other.by[name]; !ok {
					m.optional = true
				}
			}
			if old, ok := out.by[name]; ok {
				old.boolean = old.boolean || m.boolean
				old.callback = old.callback || m.callback
				continue
			}
			out.add(name, m)
		}
	}
	return out
}

// utilities are the type operators evaluated on member lists.
var utilities = map[string]bool{"Omit": true, "Pick": true, "Partial": true, "Required": true,
	"Readonly": true, "PropsWithChildren": true}

func (e *eval) generic(t typeRef, depth int) members {
	src := t.mod.src
	name := t.node.ChildByFieldName("name")
	var args []*syntax.Node
	if a := t.node.ChildByFieldName("type_arguments"); a != nil {
		args = nonComment(a.NamedChildren())
	}
	if name == nil {
		return openMembers()
	}
	if name.Type == "nested_type_identifier" {
		if op := lastSegment(name.Text(src)); strings.HasPrefix(name.Text(src), "React.") && utilities[op] {
			return e.utility(op, args, t, depth)
		}
		return e.qualified(t.mod, name, args, t, depth)
	}
	text := name.Text(src)
	if _, shadowed := t.env[text]; !shadowed && utilities[text] && !t.mod.declares(text) {
		return e.utility(text, args, t, depth)
	}
	if arg, ok := t.env[text]; ok { // a type parameter applied to arguments
		return e.members(arg, depth+1)
	}
	return e.named(t.mod, text, args, t, depth)
}

func (m *tsModule) declares(name string) bool {
	_, local := m.decls[name]
	_, imported := m.imports[name]
	return local || imported
}

func lastSegment(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func (e *eval) utility(op string, args []*syntax.Node, t typeRef, depth int) members {
	if len(args) == 0 {
		return openMembers()
	}
	base := e.members(typeRef{t.mod, args[0], t.env}, depth+1)
	switch op {
	case "Partial", "Required":
		for _, m := range base.by {
			m.optional = op == "Partial"
		}
		return base
	case "Readonly":
		return base
	case "PropsWithChildren": // P & { children?: ReactNode }
		base.add("children", member{optional: true})
		return base
	}
	if len(args) < 2 {
		return openMembers()
	}
	keys, ok := e.keys(typeRef{t.mod, args[1], t.env}, depth+1)
	if !ok { // which keys are dropped or kept is unknown
		return openMembers()
	}
	out := members{open: base.open}
	if op == "Omit" {
		drop := map[string]bool{}
		for _, k := range keys {
			drop[k] = true
		}
		for _, name := range base.order {
			if !drop[name] {
				out.add(name, *base.by[name])
			}
		}
		return out
	}
	// Pick: a picked key the (open) base does not list is still a prop; its type is unknown.
	out.open = false
	for _, k := range keys {
		if m, ok := base.by[k]; ok {
			out.add(k, *m)
		} else if base.open {
			out.add(k, member{callback: onName.MatchString(k)})
			out.open = true
		}
	}
	return out
}

// keys evaluates a key set: string literals, unions of them, `keyof T` of
// a closed T, or an alias of those.
func (e *eval) keys(t typeRef, depth int) ([]string, bool) {
	if t.node == nil || depth > maxTypeDepth {
		return nil, false
	}
	src := t.mod.src
	switch n := t.node; n.Type {
	case "literal_type":
		if s := n.ChildOfType("string"); s != nil {
			return []string{unquote(src, s)}, true
		}
	case "parenthesized_type":
		if inner := nonComment(n.NamedChildren()); len(inner) == 1 {
			return e.keys(typeRef{t.mod, inner[0], t.env}, depth+1)
		}
	case "union_type":
		var out []string
		for _, c := range nonComment(n.NamedChildren()) {
			k, ok := e.keys(typeRef{t.mod, c, t.env}, depth+1)
			if !ok {
				return nil, false
			}
			out = append(out, k...)
		}
		return out, true
	case "index_type_query": // keyof T
		if inner := nonComment(n.NamedChildren()); len(inner) == 1 {
			ms := e.members(typeRef{t.mod, inner[0], t.env}, depth+1)
			return ms.order, !ms.open
		}
	case "type_identifier":
		if arg, ok := t.env[n.Text(src)]; ok {
			return e.keys(arg, depth+1)
		}
		if mod, decls := e.r.lookup(t.mod, n.Text(src), 0); len(decls) == 1 && decls[0].Type == "type_alias_declaration" &&
			decls[0].ChildByFieldName("type_parameters") == nil && !e.active[decls[0]] {
			e.active[decls[0]] = true
			defer delete(e.active, decls[0])
			return e.keys(typeRef{mod, decls[0].ChildByFieldName("value"), nil}, depth+1)
		}
	}
	return nil, false
}

// named resolves a type name in mod (its declarations, then its imports)
// and instantiates it with args.
func (e *eval) named(mod *tsModule, name string, args []*syntax.Node, at typeRef, depth int) members {
	declMod, decls := e.r.lookup(mod, name, 0)
	return e.instantiate(declMod, decls, args, at, depth)
}

// qualified resolves `NS.T` where NS is a namespace import of a project
// module (`import * as NS from "./x"`); `React.T` and other packages are open.
func (e *eval) qualified(mod *tsModule, n *syntax.Node, args []*syntax.Node, at typeRef, depth int) members {
	ns, name := n.ChildByFieldName("module"), n.ChildByFieldName("name")
	if ns == nil || name == nil || ns.Type != "identifier" {
		return openMembers()
	}
	ref, ok := mod.imports[ns.Text(mod.src)]
	if !ok || ref.name != "*" {
		return openMembers()
	}
	target := e.r.resolveSpec(mod, ref.spec)
	if target == nil {
		return openMembers()
	}
	declMod, decls := e.r.exported(target, name.Text(mod.src), map[string]bool{}, 0)
	return e.instantiate(declMod, decls, args, at, depth)
}

// instantiate expands interface or type alias declarations, binding their
// type parameters to args (written at `at`), else to their defaults.
func (e *eval) instantiate(mod *tsModule, decls []*syntax.Node, args []*syntax.Node, at typeRef, depth int) members {
	if len(decls) == 0 {
		return openMembers()
	}
	var out members
	for _, d := range decls {
		if e.active[d] {
			return openMembers()
		}
		e.active[d] = true
		env := bindParams(mod, d, args, at)
		switch d.Type {
		case "interface_declaration":
			if ext := d.ChildOfType("extends_type_clause"); ext != nil {
				for _, base := range nonComment(ext.NamedChildren()) {
					out.merge(e.members(typeRef{mod, base, env}, depth+1))
				}
			}
			out.merge(e.members(typeRef{mod, d.ChildByFieldName("body"), env}, depth+1))
		case "type_alias_declaration":
			out.merge(e.members(typeRef{mod, d.ChildByFieldName("value"), env}, depth+1))
		}
		delete(e.active, d)
	}
	return out
}

// bindParams maps a declaration's type parameters to the arguments given,
// else to their defaults (evaluated in the declaration's own scope), else to
// nothing (open).
func bindParams(mod *tsModule, d *syntax.Node, args []*syntax.Node, at typeRef) scope {
	params := d.ChildByFieldName("type_parameters")
	if params == nil {
		return nil
	}
	env := scope{}
	for i, p := range nonComment(params.NamedChildren()) {
		name := p.ChildByFieldName("name")
		if name == nil {
			continue
		}
		switch def := p.ChildByFieldName("value"); {
		case i < len(args):
			env[name.Text(mod.src)] = typeRef{at.mod, args[i], at.env}
		case def != nil:
			env[name.Text(mod.src)] = typeRef{mod, firstNamed(def), env}
		default:
			env[name.Text(mod.src)] = typeRef{mod: mod}
		}
	}
	return env
}

func firstNamed(n *syntax.Node) *syntax.Node {
	if inner := nonComment(n.NamedChildren()); len(inner) > 0 {
		return inner[0]
	}
	return nil
}

// maxImportHops bounds import and re-export chains.
const maxImportHops = 16

// lookup finds the declarations a name refers to in mod: its own type
// declarations, else the module it imports the name from.
func (r *resolver) lookup(mod *tsModule, name string, hops int) (*tsModule, []*syntax.Node) {
	if decls := mod.decls[name]; len(decls) > 0 {
		return mod, decls
	}
	ref, ok := mod.imports[name]
	if !ok || ref.name == "*" || hops > maxImportHops {
		return nil, nil
	}
	target := r.resolveSpec(mod, ref.spec)
	if target == nil {
		return nil, nil
	}
	return r.exported(target, ref.name, map[string]bool{}, hops+1)
}

// exported finds what a module exports under name ("default" included),
// through local export lists, `export { X } from`, and `export *`.
func (r *resolver) exported(mod *tsModule, name string, seen map[string]bool, hops int) (*tsModule, []*syntax.Node) {
	key := mod.path + "\x00" + name
	if seen[key] || hops > maxImportHops {
		return nil, nil
	}
	seen[key] = true
	local := name
	if name == "default" {
		local = mod.defName
	} else if l, ok := mod.exports[name]; ok {
		local = l
	}
	if local != "" {
		if m, decls := r.lookup(mod, local, hops+1); len(decls) > 0 {
			return m, decls
		}
	}
	if ref, ok := mod.reexport[name]; ok && ref.name != "*" {
		if target := r.resolveSpec(mod, ref.spec); target != nil {
			return r.exported(target, ref.name, seen, hops+1)
		}
		return nil, nil
	}
	if name == "default" {
		return nil, nil
	}
	for _, spec := range mod.stars {
		if target := r.resolveSpec(mod, spec); target != nil {
			if m, decls := r.exported(target, name, seen, hops+1); len(decls) > 0 {
				return m, decls
			}
		}
	}
	return nil, nil
}

// objectMembers reads the signatures of an object type or interface body.
// An index signature (`[k: string]: T`, a mapped type) admits any key: open.
func objectMembers(src []byte, body *syntax.Node) members {
	var out members
	for _, m := range body.NamedChildren() {
		if m.Type == "index_signature" {
			out.open = true
			continue
		}
		ident := m.ChildByFieldName("name")
		if ident == nil {
			continue
		}
		name := unquote(src, ident)
		switch m.Type {
		case "property_signature":
			mem := member{optional: m.HasChildOfType("?"), callback: onName.MatchString(name)}
			if t := m.ChildByFieldName("type"); t != nil {
				if inner := nonComment(t.NamedChildren()); len(inner) == 1 {
					mem.boolean = isBooleanType(src, inner[0])
					mem.callback = mem.callback || inner[0].Type == "function_type"
				}
			}
			out.add(name, mem)
		case "method_signature":
			out.add(name, member{optional: m.HasChildOfType("?"), callback: true})
		}
	}
	return out
}

func isBooleanType(src []byte, t *syntax.Node) bool {
	switch t.Type {
	case "predefined_type":
		return t.Text(src) == "boolean"
	case "literal_type":
		text := t.Text(src)
		return text == "true" || text == "false"
	case "union_type":
		for _, c := range nonComment(t.NamedChildren()) {
			if !isBooleanType(src, c) && c.Text(src) != "undefined" && c.Text(src) != "null" {
				return false
			}
		}
		return true
	}
	return false
}
