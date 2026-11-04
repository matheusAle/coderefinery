package react

import (
	"sort"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

type prop struct {
	local    string // the binding the body reads ("" when read as props.x)
	optional bool
	boolean  bool
	callback bool
}

type propSet struct {
	order []string
	byKey map[string]*prop
	rest  bool
}

func (s *propSet) get(name string) *prop {
	if s.byKey == nil {
		s.byKey = map[string]*prop{}
	}
	p, ok := s.byKey[name]
	if !ok {
		p = &prop{callback: onName.MatchString(name)}
		s.byKey[name] = p
		s.order = append(s.order, name)
	}
	return p
}

// props reads the first parameter: destructured names and defaults, the
// members of its Props type (resolved across the project's modules, see
// types.go), and, for a plain `props` parameter, `props.x` reads and
// `const {x} = props`.
func (a *analyzer) props(d definition) Props {
	set := &propSet{}
	param, typ := firstParam(d.fn)
	if typ == nil {
		typ = typeArgument(a.src, d)
	}
	open := false
	if typ != nil {
		ms := a.types.propsMembers(a.mod, typ)
		for _, name := range ms.order {
			m, p := ms.by[name], set.get(name)
			p.optional = p.optional || m.optional
			p.boolean = p.boolean || m.boolean
			p.callback = p.callback || m.callback
		}
		open = ms.open
	}
	body := d.fn.ChildByFieldName("body")
	propsName := ""
	switch {
	case param == nil:
	case param.Type == "object_pattern":
		a.pattern(set, param)
	case param.Type == "identifier":
		propsName = param.Text(a.src)
		if body != nil {
			a.propsReads(set, body, propsName)
		}
	}
	out := Props{Count: len(set.order), Rest: set.rest, Open: open}
	for _, name := range set.order {
		p := set.byKey[name]
		out.Boolean += b2i(p.boolean)
		out.Optional += b2i(p.optional)
		out.Callback += b2i(p.callback)
	}
	if body != nil {
		out.PassThrough = a.passThrough(set, body, propsName)
	}
	return out
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// firstParam returns the first parameter's pattern and its type annotation.
func firstParam(fn *syntax.Node) (pattern, typ *syntax.Node) {
	if p := fn.ChildByFieldName("parameter"); p != nil { // `props => ...`
		return p, nil
	}
	params := fn.ChildByFieldName("parameters")
	if params == nil {
		return nil, nil
	}
	first := nonComment(params.NamedChildren())
	if len(first) == 0 {
		return nil, nil
	}
	p := first[0]
	if p.Type == "required_parameter" || p.Type == "optional_parameter" {
		typ = p.ChildByFieldName("type")
		p = p.ChildByFieldName("pattern")
	}
	if p != nil && p.Type == "assignment_pattern" { // JS `({a} = {})`
		p = p.ChildByFieldName("left")
	}
	return p, typ
}

// typeArgument is P in `const C: React.FC<P> = ...`, `forwardRef<R, P>(...)`
// or `memo<P>(...)`: the last type argument.
func typeArgument(src []byte, d definition) *syntax.Node {
	if d.span.Type != "variable_declarator" {
		return nil
	}
	var args *syntax.Node
	if t := d.span.ChildByFieldName("type"); t != nil {
		syntax.Walk(t, func(n *syntax.Node) bool {
			if args == nil && n.Type == "type_arguments" {
				args = n
			}
			return args == nil
		})
	}
	for call := unparen(d.span.ChildByFieldName("value")); args == nil && call != nil && call.Type == "call_expression" && wrappers[calleeName(src, call)]; {
		args = call.ChildByFieldName("type_arguments")
		next := arguments(call)
		if len(next) == 0 {
			break
		}
		call = unparen(next[0])
	}
	if args == nil {
		return nil
	}
	types := nonComment(args.NamedChildren())
	if len(types) == 0 {
		return nil
	}
	return types[len(types)-1]
}

// pattern adds the names of a destructuring pattern `{ a, b = true, c: d, ...rest }`.
func (a *analyzer) pattern(set *propSet, pat *syntax.Node) {
	for _, c := range pat.NamedChildren() {
		switch c.Type {
		case "shorthand_property_identifier_pattern":
			set.get(c.Text(a.src)).local = c.Text(a.src)
		case "object_assignment_pattern":
			left, right := c.ChildByFieldName("left"), c.ChildByFieldName("right")
			if left == nil {
				continue
			}
			p := set.get(left.Text(a.src))
			p.local, p.optional = left.Text(a.src), true
			p.boolean = p.boolean || isBoolLiteral(right)
		case "pair_pattern":
			key, value := c.ChildByFieldName("key"), c.ChildByFieldName("value")
			if key == nil || value == nil {
				continue
			}
			p := set.get(unquote(a.src, key))
			if value.Type == "assignment_pattern" {
				p.optional = true
				p.boolean = p.boolean || isBoolLiteral(value.ChildByFieldName("right"))
				value = value.ChildByFieldName("left")
			}
			if value != nil && value.Type == "identifier" {
				p.local = value.Text(a.src)
			}
		case "rest_pattern":
			set.rest = true
		}
	}
}

func isBoolLiteral(n *syntax.Node) bool {
	return n != nil && (n.Type == "true" || n.Type == "false")
}

func unquote(src []byte, key *syntax.Node) string {
	if key.Type == "string" {
		for _, c := range key.Children {
			if c.Type == "string_fragment" {
				return c.Text(src)
			}
		}
	}
	return key.Text(src)
}

// propsReads adds what a plain `props` parameter is read as: `props.x`,
// `const { x } = props`, and `{...props}` (rest).
func (a *analyzer) propsReads(set *propSet, body *syntax.Node, propsName string) {
	syntax.Walk(body, func(n *syntax.Node) bool {
		switch n.Type {
		case "member_expression":
			obj, prop := n.ChildByFieldName("object"), n.ChildByFieldName("property")
			if obj != nil && obj.Type == "identifier" && obj.Text(a.src) == propsName && prop != nil && prop.Type == "property_identifier" {
				set.get(prop.Text(a.src))
			}
		case "variable_declarator":
			value := unparen(n.ChildByFieldName("value"))
			if pat := n.ChildByFieldName("name"); pat != nil && pat.Type == "object_pattern" &&
				value != nil && value.Type == "identifier" && value.Text(a.src) == propsName {
				a.pattern(set, pat)
			}
		case "spread_element":
			if inner := nonComment(n.NamedChildren()); len(inner) == 1 && inner[0].Type == "identifier" &&
				inner[0].Text(a.src) == propsName {
				set.rest = true
			}
		}
		return true
	})
}

// passThrough lists the props whose every read is an unchanged child
// component attribute (`<Child x={x} />`, `<Child x={props.x} />`): the
// prop-drilling signal. Shadowing is not tracked.
func (a *analyzer) passThrough(set *propSet, body *syntax.Node, propsName string) []string {
	byLocal := map[string]string{}
	for _, name := range set.order {
		if l := set.byKey[name].local; l != "" {
			byLocal[l] = name
		}
	}
	forwarded, used := map[string]bool{}, map[string]bool{}
	note := func(name string, ref *syntax.Node) {
		if forwardsToComponent(a.src, ref) {
			forwarded[name] = true
		} else {
			used[name] = true
		}
	}
	syntax.Walk(body, func(n *syntax.Node) bool {
		switch n.Type {
		case "identifier":
			if name, ok := byLocal[n.Text(a.src)]; ok {
				note(name, n)
			}
		case "shorthand_property_identifier":
			if name, ok := byLocal[n.Text(a.src)]; ok {
				used[name] = true
			}
		case "member_expression":
			if propsName == "" {
				break
			}
			obj, prop := n.ChildByFieldName("object"), n.ChildByFieldName("property")
			if obj != nil && obj.Type == "identifier" && obj.Text(a.src) == propsName && prop != nil {
				if _, ok := set.byKey[prop.Text(a.src)]; ok {
					note(prop.Text(a.src), n)
				}
				return false
			}
		}
		return true
	})
	var out []string
	for name := range forwarded {
		if !used[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// forwardsToComponent: ref is the whole value of an attribute of a
// PascalCase (or dotted) JSX element.
func forwardsToComponent(src []byte, ref *syntax.Node) bool {
	expr := upParen(ref)
	if expr == nil || expr.Type != "jsx_expression" || expr.Parent == nil || expr.Parent.Type != "jsx_attribute" {
		return false
	}
	element := expr.Parent.Parent
	if element == nil {
		return false
	}
	name := element.ChildByFieldName("name")
	if name == nil {
		return false
	}
	return name.Type == "member_expression" || name.Type == "nested_identifier" || pascalName.MatchString(name.Text(src))
}
