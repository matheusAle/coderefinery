// Package react implements `coderefinery react` (an addition, not an
// upstream port): static quality metrics for React function components and
// custom hooks, read from the TSX/JSX tree the crap reader parses
// (docs/design-react.md).
package react

import (
	"regexp"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

var (
	pascalName = regexp.MustCompile(`^[A-Z]`)
	hookName   = regexp.MustCompile(`^use[A-Z0-9]`)
	onName     = regexp.MustCompile(`^on[A-Z]`)
)

// wrappers are the calls a component definition may be wrapped in.
var wrappers = map[string]bool{"memo": true, "forwardRef": true}

// effectHooks run their callback after render; depHooks take a dependency array.
var (
	effectHooks = map[string]bool{"useEffect": true, "useLayoutEffect": true, "useInsertionEffect": true}
	depHooks    = map[string]bool{
		"useEffect": true, "useLayoutEffect": true, "useInsertionEffect": true,
		"useMemo": true, "useCallback": true, "useImperativeHandle": true,
	}
)

func isFunction(n *syntax.Node) bool {
	switch n.Type {
	case "arrow_function", "function_expression", "function_declaration", "generator_function_declaration":
		return true
	}
	return false
}

func isElement(n *syntax.Node) bool {
	return n.Type == "jsx_element" || n.Type == "jsx_self_closing_element"
}

// isFragment is `<>...</>`: a jsx_element whose opening tag has no name.
func isFragment(n *syntax.Node) bool {
	if n.Type != "jsx_element" {
		return false
	}
	open := n.ChildByFieldName("open_tag")
	return open != nil && open.ChildByFieldName("name") == nil
}

// unparen skips parenthesized_expression layers downward.
func unparen(n *syntax.Node) *syntax.Node {
	for n != nil && n.Type == "parenthesized_expression" {
		var inner *syntax.Node
		for _, c := range n.Children {
			if c.Named && c.Type != "comment" {
				inner = c
				break
			}
		}
		n = inner
	}
	return n
}

// upParen climbs parenthesized_expression layers upward.
func upParen(n *syntax.Node) *syntax.Node {
	p := n.Parent
	for p != nil && p.Type == "parenthesized_expression" {
		p = p.Parent
	}
	return p
}

// calleeName is the called name of a call: `f` for f(), `memo` for
// React.memo(); "" for anything else.
func calleeName(src []byte, call *syntax.Node) string {
	if call == nil || call.Type != "call_expression" {
		return ""
	}
	fn := call.ChildByFieldName("function")
	switch {
	case fn == nil:
		return ""
	case fn.Type == "identifier":
		return fn.Text(src)
	case fn.Type == "member_expression":
		obj, prop := fn.ChildByFieldName("object"), fn.ChildByFieldName("property")
		if obj != nil && obj.Type == "identifier" && obj.Text(src) == "React" && prop != nil {
			return prop.Text(src)
		}
	}
	return ""
}

// hookCall returns the hook name when n is a call to `useX(...)` or `React.useX(...)`.
func hookCall(src []byte, n *syntax.Node) string {
	name := calleeName(src, n)
	if hookName.MatchString(name) {
		return name
	}
	return ""
}

func arguments(call *syntax.Node) []*syntax.Node {
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return nil
	}
	var out []*syntax.Node
	for _, c := range args.NamedChildren() {
		if c.Type != "comment" {
			out = append(out, c)
		}
	}
	return out
}

// unwrapDefinition returns the function a declarator value defines, looking
// through memo/forwardRef wrappers; wrapped reports whether one was seen.
func unwrapDefinition(src []byte, value *syntax.Node) (fn *syntax.Node, wrapped bool) {
	value = unparen(value)
	for value != nil && value.Type == "call_expression" && wrappers[calleeName(src, value)] {
		args := arguments(value)
		if len(args) == 0 {
			return nil, false
		}
		value, wrapped = unparen(args[0]), true
	}
	if value != nil && (value.Type == "arrow_function" || value.Type == "function_expression") {
		return value, wrapped
	}
	return nil, false
}

// definition is one named function the walker may turn into a unit.
type definition struct {
	name    string
	fn      *syntax.Node // the function node
	span    *syntax.Node // the node whose lines the entry reports (declarator or declaration)
	wrapped bool         // memo/forwardRef
}

// definitionOf recognises `function Name() {}` and `const Name = <fn | memo(fn) | forwardRef(fn)>`.
func definitionOf(src []byte, n *syntax.Node) (definition, bool) {
	switch n.Type {
	case "function_declaration":
		ident := n.ChildByFieldName("name")
		if ident == nil || n.ChildByFieldName("body") == nil {
			return definition{}, false
		}
		return definition{name: ident.Text(src), fn: n, span: n}, true
	case "variable_declarator":
		ident, value := n.ChildByFieldName("name"), n.ChildByFieldName("value")
		if ident == nil || ident.Type != "identifier" || value == nil {
			return definition{}, false
		}
		fn, wrapped := unwrapDefinition(src, value)
		if fn == nil {
			return definition{}, false
		}
		return definition{name: ident.Text(src), fn: fn, span: n, wrapped: wrapped}, true
	}
	return definition{}, false
}

// ownReturnsJSX reports whether fn returns JSX: its expression body, or one
// of its own return statements (not those of nested functions), holds JSX.
func ownReturnsJSX(fn *syntax.Node) bool {
	body := fn.ChildByFieldName("body")
	if body == nil {
		return false
	}
	if body.Type != "statement_block" {
		return containsJSX(body)
	}
	found := false
	syntax.Walk(body, func(n *syntax.Node) bool {
		if found || isFunction(n) || n.Type == "class_declaration" || n.Type == "class" {
			return false
		}
		if n.Type == "return_statement" && containsJSX(n) {
			found = true
			return false
		}
		return true
	})
	return found
}

func containsJSX(n *syntax.Node) bool {
	found := false
	syntax.Walk(n, func(c *syntax.Node) bool {
		if found {
			return false
		}
		if isElement(c) {
			found = true
			return false
		}
		return true
	})
	return found
}

func isComponent(d definition) bool {
	return (pascalName.MatchString(d.name) || d.wrapped) && ownReturnsJSX(d.fn)
}

func isHook(d definition) bool { return hookName.MatchString(d.name) }

// insideFunction reports whether n sits in any function or class.
func insideFunction(n *syntax.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if isFunction(p) || p.Type == "class_body" || p.Type == "method_definition" {
			return true
		}
	}
	return false
}
