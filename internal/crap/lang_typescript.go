package crap

import (
	"sort"
	"strconv"
	"strings"

	"github.com/matheusAle/coderefinery/internal/lang"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/syntax"
)

// TypeScript, TSX and JavaScript entries (upstream
// crapper/languages/typescript.py): top-level functions, class methods,
// const/let arrow and function-expression declarators, and inline Express
// route callbacks (`GET /users`, `GET /users#2`). JS files report language
// "typescript" too.

func init() { Readers["typescript"] = TypeScriptFunctions }

var tsDecisions = map[string]bool{
	"if_statement": true, "for_statement": true, "for_in_statement": true, "while_statement": true,
	"do_statement": true, "catch_clause": true, "ternary_expression": true, "switch_case": true,
	"switch_default": true,
}

var tsSkip = map[string]bool{
	"class_declaration": true, "abstract_class_declaration": true, "interface_declaration": true,
}

var tsFunctionTypes = map[string]bool{
	"function_declaration": true, "method_definition": true, "arrow_function": true, "function_expression": true,
}

var tsRouteMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true, "head": true,
	"options": true, "all": true, "use": true,
}

var tsCallbacks = map[string]bool{"arrow_function": true, "function_expression": true}

func tsIsDecision(n *syntax.Node) bool {
	if tsDecisions[n.Type] || BinaryLogic(n) {
		return true
	}
	if n.Type == "binary_expression" && n.HasChildOfType("??") {
		return true
	}
	// `a?.b` and `a?.[0]` wrap `?.` in optional_chain; `a?.()` is a bare
	// `?.` in TypeScript. Count the wrapper once, a bare token when unwrapped.
	if n.Type == "optional_chain" {
		return true
	}
	return n.Type == "?." && (n.Parent == nil || n.Parent.Type != "optional_chain")
}

// tsGrammar is typescript._grammar.
func tsGrammar(path string) string {
	switch {
	case strings.HasSuffix(path, ".tsx"):
		return "tsx"
	case strings.HasSuffix(path, ".jsx"), strings.HasSuffix(path, ".mjs"),
		strings.HasSuffix(path, ".cjs"), strings.HasSuffix(path, ".js"):
		return "javascript"
	}
	return "typescript"
}

func stripScriptSuffix(relative string) string {
	for _, suffix := range []string{".tsx", ".mts", ".cts", ".jsx", ".mjs", ".cjs", ".ts", ".js"} {
		if strings.HasSuffix(relative, suffix) {
			return strings.TrimSuffix(relative, suffix)
		}
	}
	return relative
}

// TypeScriptModuleNamespace is typescript._module_namespace.
func TypeScriptModuleNamespace(path, sourceRoot string) string {
	relative := stripSrc(stripRoot(strings.ReplaceAll(path, `\`, "/"), sourceRoot))
	return strings.ReplaceAll(stripScriptSuffix(relative), "/", ".")
}

func tsInsideFunction(n *syntax.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if tsFunctionTypes[p.Type] {
			return true
		}
	}
	return false
}

func tsClassNames(src []byte, n *syntax.Node) []string {
	var names []string
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == "class_declaration" || p.Type == "abstract_class_declaration" {
			if ident := p.ChildOfType("type_identifier", "identifier"); ident != nil {
				names = append(names, ident.Text(src))
			}
		}
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return names
}

func tsHasBody(n *syntax.Node) bool { return n.ChildOfType("statement_block") != nil }

// tsExpression unwraps parenthesized_expression layers.
func tsExpression(n *syntax.Node) *syntax.Node {
	for n != nil && n.Type == "parenthesized_expression" {
		var inner *syntax.Node
		for _, c := range n.Children {
			if c.Type != "(" && c.Type != ")" {
				inner = c
				break
			}
		}
		n = inner
	}
	return n
}

func tsCallCallee(call *syntax.Node) *syntax.Node {
	for _, c := range call.Children {
		if c.Type != "arguments" {
			return c
		}
	}
	return nil
}

func tsPropertyName(src []byte, n *syntax.Node) (string, bool) {
	if n == nil || n.Type != "member_expression" {
		return "", false
	}
	ident := n.ChildOfType("property_identifier")
	if ident == nil {
		return "", false
	}
	return ident.Text(src), true
}

func tsRouteMethod(src []byte, call *syntax.Node) (string, bool) {
	name, ok := tsPropertyName(src, tsCallCallee(call))
	if !ok {
		return "", false
	}
	lowered := strings.ToLower(name)
	if tsRouteMethods[lowered] {
		return lowered, true
	}
	return "", false
}

func tsLiteralText(src []byte, n *syntax.Node) (string, bool) {
	switch n.Type {
	case "string":
		var b strings.Builder
		for _, c := range n.Children {
			if c.Type == "string_fragment" {
				b.WriteString(c.Text(src))
			}
		}
		return b.String(), true
	case "template_string":
		text := n.Text(src)
		if len(text) >= 2 && text[0] == '`' && text[len(text)-1] == '`' {
			return text[1 : len(text)-1], true
		}
	}
	return "", false
}

func tsStringArgument(src []byte, call *syntax.Node) (string, bool) {
	arguments := call.ChildOfType("arguments")
	if arguments == nil {
		return "", false
	}
	for _, c := range arguments.Children {
		expr := tsExpression(c)
		if expr == nil {
			continue
		}
		if text, ok := tsLiteralText(src, expr); ok {
			return text, true
		}
	}
	return "", false
}

// tsPathFromRoute finds the path of a preceding `.route("/path")` in a
// chained call.
func tsPathFromRoute(src []byte, n *syntax.Node) (string, bool) {
	current := n
	for current != nil {
		if current.Type == "member_expression" {
			current = current.Child(0)
			continue
		}
		if current.Type != "call_expression" {
			return "", false
		}
		callee := tsCallCallee(current)
		if name, ok := tsPropertyName(src, callee); ok && name == "route" {
			return tsStringArgument(src, current)
		}
		current = callee
	}
	return "", false
}

func tsRouteCallbacks(call *syntax.Node) []*syntax.Node {
	arguments := call.ChildOfType("arguments")
	if arguments == nil {
		return nil
	}
	var found []*syntax.Node
	for _, c := range arguments.Children {
		if expr := tsExpression(c); expr != nil && tsCallbacks[expr.Type] {
			found = append(found, expr)
		}
	}
	return found
}

func tsIsRouteCallback(src []byte, n *syntax.Node) bool {
	if !tsCallbacks[n.Type] {
		return false
	}
	current := n.Parent
	for current != nil && current.Type == "parenthesized_expression" {
		current = current.Parent
	}
	if current == nil || current.Type != "arguments" {
		return false
	}
	call := current.Parent
	if call == nil || call.Type != "call_expression" {
		return false
	}
	_, ok := tsRouteMethod(src, call)
	return ok
}

func tsComplexity(src []byte, root *syntax.Node) int {
	return Complexity(root, tsIsDecision, func(n *syntax.Node) bool {
		if n == root {
			return false
		}
		return tsSkip[n.Type] || tsIsRouteCallback(src, n)
	})
}

type tsFound struct {
	start int
	fn    model.Function
}

type tsReader struct {
	src    []byte
	module string
	path   string
	found  []tsFound
}

func (r *tsReader) add(n *syntax.Node, name, namespace string, complexity int) {
	r.found = append(r.found, tsFound{n.StartByte, model.Function{
		Name: name, Namespace: namespace, Complexity: complexity,
		StartLine: n.StartLine(), EndLine: n.EndLine(),
		Path: r.path, Language: "typescript",
		StartByte: n.StartByte, EndByte: n.EndByte,
	}})
}

func (r *tsReader) function(n *syntax.Node) {
	if tsInsideFunction(n) || !tsHasBody(n) {
		return
	}
	ident := n.ChildOfType("identifier")
	if ident == nil {
		return
	}
	r.add(n, ident.Text(r.src), r.module, tsComplexity(r.src, n))
}

func (r *tsReader) method(n *syntax.Node) {
	if !tsHasBody(n) {
		return
	}
	ident := n.ChildOfType("property_identifier", "identifier")
	classes := tsClassNames(r.src, n)
	if ident == nil || len(classes) == 0 {
		return
	}
	r.add(n, ident.Text(r.src), r.module+"."+strings.Join(classes, "."), tsComplexity(r.src, n))
}

func (r *tsReader) arrows(n *syntax.Node) {
	if tsInsideFunction(n) {
		return
	}
	for _, declarator := range n.Children {
		if declarator.Type != "variable_declarator" {
			continue
		}
		ident := declarator.ChildOfType("identifier")
		value := declarator.ChildOfType("arrow_function", "function_expression")
		if ident == nil || value == nil {
			continue
		}
		r.add(declarator, ident.Text(r.src), r.module, tsComplexity(r.src, value))
	}
}

type tsRoute struct {
	callback *syntax.Node
	method   string
	route    string
}

func (r *tsReader) collectRoutes(routes []tsRoute, call *syntax.Node) []tsRoute {
	method, ok := tsRouteMethod(r.src, call)
	if !ok {
		return routes
	}
	callbacks := tsRouteCallbacks(call)
	if len(callbacks) == 0 {
		return routes
	}
	route, _ := tsStringArgument(r.src, call)
	if route == "" {
		route, _ = tsPathFromRoute(r.src, tsCallCallee(call))
	}
	for _, cb := range callbacks {
		routes = append(routes, tsRoute{cb, method, route})
	}
	return routes
}

func tsRouteLabel(method, path string) string {
	label := strings.ToUpper(method)
	if path != "" {
		return label + " " + path
	}
	return label
}

// tsClaim is typescript._claim: the first free name among L, L#2, L#3...
func tsClaim(used map[string]bool, name string) string {
	if !used[name] {
		used[name] = true
		return name
	}
	number := 2
	for used[name+"#"+strconv.Itoa(number)] {
		number++
	}
	chosen := name + "#" + strconv.Itoa(number)
	used[chosen] = true
	return chosen
}

func (r *tsReader) appendRoutes(routes []tsRoute) {
	sort.SliceStable(routes, func(i, j int) bool { return routes[i].callback.StartByte < routes[j].callback.StartByte })
	used := map[string]bool{}
	for _, rt := range routes {
		name := tsClaim(used, tsRouteLabel(rt.method, rt.route))
		r.add(rt.callback, name, r.module, tsComplexity(r.src, rt.callback))
	}
}

// TypeScriptFunctions is typescript.functions_in_source.
func TypeScriptFunctions(source, path, sourceRoot string) ([]model.Function, error) {
	tree, err := lang.ParseText(tsGrammar(path), source)
	if err != nil {
		return nil, err
	}
	r := &tsReader{src: tree.Source, module: TypeScriptModuleNamespace(path, sourceRoot), path: path}
	var routes []tsRoute
	tree.Root.Descendants(func(n *syntax.Node) bool {
		switch n.Type {
		case "function_declaration":
			r.function(n)
		case "method_definition":
			r.method(n)
		case "lexical_declaration":
			r.arrows(n)
		case "call_expression":
			routes = r.collectRoutes(routes, n)
		}
		return true
	})
	r.appendRoutes(routes)
	sort.SliceStable(r.found, func(i, j int) bool { return r.found[i].start < r.found[j].start })
	out := make([]model.Function, len(r.found))
	for i, f := range r.found {
		out[i] = f.fn
	}
	return out, nil
}
