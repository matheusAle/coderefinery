package typescript

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

func parseFor(t *testing.T, grammar, src string) *syntax.Node {
	t.Helper()
	parse := map[string]func([]byte) (*syntax.Node, error){
		"ts": ParseTypeScript, "tsx": ParseTSX, "js": ParseJavaScript,
	}[grammar]
	root, err := parse([]byte(src))
	if err != nil {
		t.Fatalf("%s parse(%q): %v", grammar, src, err)
	}
	return root
}

// Expected strings are upstream tree-sitter's str(root_node) for the same
// source (tools/oracle/oracle.py --sexp).
func TestShapesMatchTreeSitter(t *testing.T) {
	cases := []struct{ grammar, src, want string }{
		{"ts", "x = a?.b ?? c?.();\n", "(program (expression_statement (assignment_expression left: (identifier) right: (binary_expression left: (member_expression object: (identifier) optional_chain: (optional_chain) property: (property_identifier)) right: (call_expression function: (identifier) arguments: (arguments))))))"},
		{"js", "x = a?.b ?? c?.();\n", "(program (expression_statement (assignment_expression left: (identifier) right: (binary_expression left: (member_expression object: (identifier) optional_chain: (optional_chain) property: (property_identifier)) right: (call_expression function: (identifier) optional_chain: (optional_chain) arguments: (arguments))))))"},
		{"ts", "x = await a.get<T>(b) + c as D;\n", "(program (expression_statement (assignment_expression left: (identifier) right: (as_expression (binary_expression left: (call_expression function: (await_expression (member_expression object: (identifier) property: (property_identifier))) type_arguments: (type_arguments (type_identifier)) arguments: (arguments (identifier))) right: (identifier)) (type_identifier)))))"},
		{"ts", "x = <T>y!;\n", "(program (expression_statement (assignment_expression left: (identifier) right: (non_null_expression (type_assertion (type_arguments (type_identifier)) (identifier))))))"},
		{"ts", "let r = a / b / c, s = /a[/]b/g;\n", "(program (lexical_declaration (variable_declarator name: (identifier) value: (binary_expression left: (binary_expression left: (identifier) right: (identifier)) right: (identifier))) (variable_declarator name: (identifier) value: (regex pattern: (regex_pattern) flags: (regex_flags)))))"},
		{"tsx", "const W = <T,>(x: T) => <div a=\"1\">{x}</div>;\n", "(program (lexical_declaration (variable_declarator name: (identifier) value: (arrow_function type_parameters: (type_parameters (type_parameter name: (type_identifier))) parameters: (formal_parameters (required_parameter pattern: (identifier) type: (type_annotation (type_identifier)))) body: (jsx_element open_tag: (jsx_opening_element name: (identifier) attribute: (jsx_attribute (property_identifier) (string (string_fragment)))) (jsx_expression (identifier)) close_tag: (jsx_closing_element name: (identifier)))))))"},
		{"js", "x = <p>hi {a} &amp; there</p>;\n", "(program (expression_statement (assignment_expression left: (identifier) right: (jsx_element open_tag: (jsx_opening_element name: (identifier)) (jsx_text) (jsx_expression (identifier)) (jsx_text) (html_character_reference) (jsx_text) close_tag: (jsx_closing_element name: (identifier))))))"},
		{"ts", "type M<T> = { readonly [K in keyof T]?: T[K] extends infer U ? U : never };\n", "(program (type_alias_declaration name: (type_identifier) type_parameters: (type_parameters (type_parameter name: (type_identifier))) value: (object_type (index_signature (mapped_type_clause name: (type_identifier) type: (index_type_query (type_identifier))) type: (opting_type_annotation (conditional_type left: (lookup_type (type_identifier) (type_identifier)) right: (infer_type (type_identifier)) consequence: (type_identifier) alternative: (predefined_type)))))))"},
		{"ts", "for (const [k, v] of m) label: while (k) break label;\n", "(program (for_in_statement left: (array_pattern (identifier) (identifier)) right: (identifier) body: (labeled_statement label: (statement_identifier) body: (while_statement condition: (parenthesized_expression (identifier)) body: (break_statement label: (statement_identifier))))))"},
	}
	for _, c := range cases {
		if got := syntax.SExp(parseFor(t, c.grammar, c.src)); got != c.want {
			t.Errorf("%s %q\n got  %s\n want %s", c.grammar, c.src, got, c.want)
		}
	}
}

// Trailing same-line comments follow the scanner's automatic semicolon:
// typescript absorbs them into any statement or block; javascript only
// into statements and (some) empty blocks.
func TestTrailingCommentPlacement(t *testing.T) {
	cases := []struct{ grammar, src, want string }{
		{"ts", "f() // c\ng();\n", "(program (expression_statement (call_expression function: (identifier) arguments: (arguments)) (comment)) (expression_statement (call_expression function: (identifier) arguments: (arguments))))"},
		{"js", "f() // c\ng();\n", "(program (expression_statement (call_expression function: (identifier) arguments: (arguments)) (comment)) (expression_statement (call_expression function: (identifier) arguments: (arguments))))"},
		{"ts", "if (a) {} // c\nb;\n", "(program (if_statement condition: (parenthesized_expression (identifier)) consequence: (statement_block (comment))) (expression_statement (identifier)))"},
		{"js", "if (a) { b() } // c\nc;\n", "(program (if_statement condition: (parenthesized_expression (identifier)) consequence: (statement_block (expression_statement (call_expression function: (identifier) arguments: (arguments))))) (comment) (expression_statement (identifier)))"},
	}
	for _, c := range cases {
		if got := syntax.SExp(parseFor(t, c.grammar, c.src)); got != c.want {
			t.Errorf("%s %q\n got  %s\n want %s", c.grammar, c.src, got, c.want)
		}
	}
	// the absorbed comment stretches the statement's span to its end
	stmt := parseFor(t, "ts", "f() // c\ng();\n").Children[0]
	if stmt.Type != "expression_statement" || stmt.EndByte != 8 || stmt.Children[1].Extra != true {
		t.Fatalf("statement %s [%d,%d) children %v", stmt.Type, stmt.StartByte, stmt.EndByte, stmt.Children)
	}
}

func TestNodeMetadata(t *testing.T) {
	src := "export class A {\n  #n = 1;\n  m(x?: number): void { if (x) this.#n++; }\n}\n"
	root := parseFor(t, "ts", src)
	if root.StartByte != 0 || root.EndByte != len(src) || root.EndPoint != (syntax.Point{Row: 4, Column: 0}) {
		t.Fatalf("program span [%d,%d) %v", root.StartByte, root.EndByte, root.EndPoint)
	}
	class := root.Children[0].ChildByFieldName("declaration")
	if class == nil || class.Type != "class_declaration" || class.ChildByFieldName("name").Text([]byte(src)) != "A" {
		t.Fatalf("declaration %v", class)
	}
	body := class.ChildByFieldName("body")
	field, method := body.NamedChildren()[0], body.NamedChildren()[1]
	if field.Type != "public_field_definition" || field.ChildByFieldName("name").Type != "private_property_identifier" {
		t.Fatalf("field %s", syntax.SExp(field))
	}
	if method.Type != "method_definition" || method.StartPoint != (syntax.Point{Row: 2, Column: 2}) || method.EndLine() != 3 {
		t.Fatalf("method %s at %v-%v", method.Type, method.StartPoint, method.EndPoint)
	}
	param := method.ChildByFieldName("parameters").NamedChildren()[0]
	if param.Type != "optional_parameter" || param.Parent != method.ChildByFieldName("parameters") {
		t.Fatalf("parameter %s", syntax.SExp(param))
	}
	// anonymous tokens keep their spelling as type
	if q := param.Children[1]; q.Type != "?" || q.Named {
		t.Fatalf("optional marker %+v", q)
	}
}

func TestEmptyAndWhitespaceOnlyPrograms(t *testing.T) {
	for _, c := range []struct {
		src        string
		start, end int
	}{{"", 0, 0}, {"  \n", 3, 3}, {"\n\n  x;\n", 4, 7}, {"// c\n", 0, 5}} {
		root := parseFor(t, "js", c.src)
		if root.Type != "program" || root.StartByte != c.start || root.EndByte != c.end {
			t.Errorf("%q: program [%d,%d), want [%d,%d)", c.src, root.StartByte, root.EndByte, c.start, c.end)
		}
	}
}

func TestDeepExpressionDoesNotOverflow(t *testing.T) {
	src := "x = " + repeat("a + ", 1500) + "a;\n"
	root := parseFor(t, "ts", src)
	depth := 0
	right := root.Children[0].Children[0].ChildByFieldName("right")
	for n := right; n.Type == "binary_expression"; n = n.ChildByFieldName("left") {
		depth++
	}
	if depth != 1500 {
		t.Fatalf("binary depth %d, want 1500", depth)
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestInvalidInputIsAnError(t *testing.T) {
	for _, src := range []string{"function (", "x = `unterminated", "let = = 1;", "<a>b & c</a>;"} {
		if _, err := ParseTSX([]byte(src)); err == nil {
			t.Errorf("%q: want a parse error", src)
		}
	}
}

// `f<typeof import("m")>()` is a comparison to tree-sitter; when that does
// not parse we fall back to the call reading instead of failing.
func TestImportTypeArgumentsFallBackToACall(t *testing.T) {
	ok := syntax.SExp(parseFor(t, "ts", "x = f<typeof import(\"m\")>(\"y\");\n"))
	if want := "(program (expression_statement (assignment_expression left: (identifier) right: (binary_expression left: (binary_expression left: (identifier) right: (unary_expression argument: (call_expression function: (import) arguments: (arguments (string (string_fragment)))))) right: (parenthesized_expression (string (string_fragment)))))))"; ok != want {
		t.Fatalf("comparison reading\n got  %s\n want %s", ok, want)
	}
	call := parseFor(t, "ts", "await orig<typeof import(\"m\")>();\n")
	if got := call.Children[0].Children[0].Type; got != "call_expression" {
		t.Fatalf("fallback reading %s", syntax.SExp(call))
	}
}
