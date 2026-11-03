package java

import (
	"strings"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

func parse(t *testing.T, src string) *syntax.Node {
	t.Helper()
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return root
}

// Expected strings are upstream tree-sitter's str(root_node) for the same
// source (tools/oracle/oracle.py --sexp). Each one pins a tree-sitter-java
// quirk the parser reproduces.
func TestShapesMatchTreeSitter(t *testing.T) {
	cases := []struct{ src, want string }{
		{"class A { void m() { switch (x) { case 1: case 2: f(); default: g(); } } }",
			"(program (class_declaration name: (identifier) body: (class_body (method_declaration type: (void_type) name: (identifier) parameters: (formal_parameters) body: (block (switch_expression condition: (parenthesized_expression (identifier)) body: (switch_block (switch_block_statement_group (switch_label (decimal_integer_literal))) (switch_block_statement_group (switch_label (decimal_integer_literal)) (expression_statement (method_invocation name: (identifier) arguments: (argument_list)))) (switch_block_statement_group (switch_label) (expression_statement (method_invocation name: (identifier) arguments: (argument_list)))))))))))"},
		{"class A { void m() { switch (o) { case null, default -> f(); case A, B -> g(); } } }",
			"(program (class_declaration name: (identifier) body: (class_body (method_declaration type: (void_type) name: (identifier) parameters: (formal_parameters) body: (block (switch_expression condition: (parenthesized_expression (identifier)) body: (switch_block (switch_rule (switch_label (null_literal) (identifier)) (expression_statement (method_invocation name: (identifier) arguments: (argument_list)))) (switch_rule (switch_label (identifier) (identifier)) (expression_statement (method_invocation name: (identifier) arguments: (argument_list)))))))))))"},
		{"class A { int m() { return (Foo) -y + (int) -y; } }",
			"(program (class_declaration name: (identifier) body: (class_body (method_declaration type: (integral_type) name: (identifier) parameters: (formal_parameters) body: (block (return_statement (binary_expression left: (binary_expression left: (parenthesized_expression (identifier)) right: (identifier)) right: (cast_expression type: (integral_type) value: (unary_expression operand: (identifier))))))))))"},
		{"class A { void m() { a = -b++; c = (byte) i++; } }",
			"(program (class_declaration name: (identifier) body: (class_body (method_declaration type: (void_type) name: (identifier) parameters: (formal_parameters) body: (block (expression_statement (assignment_expression left: (identifier) right: (update_expression (unary_expression operand: (identifier))))) (expression_statement (assignment_expression left: (identifier) right: (update_expression (cast_expression type: (integral_type) value: (identifier))))))))))"},
		{"class A { void m() { x = (a) + b * c; y = (a) + b; z = k + (a) + b * c; } }",
			"(program (class_declaration name: (identifier) body: (class_body (method_declaration type: (void_type) name: (identifier) parameters: (formal_parameters) body: (block (expression_statement (assignment_expression left: (identifier) right: (binary_expression left: (cast_expression type: (type_identifier) value: (unary_expression operand: (identifier))) right: (identifier)))) (expression_statement (assignment_expression left: (identifier) right: (binary_expression left: (parenthesized_expression (identifier)) right: (identifier)))) (expression_statement (assignment_expression left: (identifier) right: (binary_expression left: (binary_expression left: (identifier) right: (parenthesized_expression (identifier))) right: (binary_expression left: (identifier) right: (identifier))))))))))"},
		{"class A { List<List<String>> l; int s = a >> 2 >>> 1; }",
			"(program (class_declaration name: (identifier) body: (class_body (field_declaration type: (generic_type (type_identifier) (type_arguments (generic_type (type_identifier) (type_arguments (type_identifier))))) declarator: (variable_declarator name: (identifier))) (field_declaration type: (integral_type) declarator: (variable_declarator name: (identifier) value: (binary_expression left: (binary_expression left: (identifier) right: (decimal_integer_literal)) right: (decimal_integer_literal)))))))"},
		{"class A { String t = \"\"\"\n a\\q\\n \\\n\"\"\"; }",
			"(program (class_declaration name: (identifier) body: (class_body (field_declaration type: (type_identifier) declarator: (variable_declarator name: (identifier) value: (string_literal (multiline_string_fragment) (escape_sequence) (multiline_string_fragment)))))))"},
		{"module m { provides a.B with c.D, e.F; }",
			"(program (module_declaration name: (identifier) body: (module_body (provides_module_directive provided: (scoped_identifier scope: (identifier) name: (identifier)) (scoped_identifier scope: (identifier) name: (identifier)) provider: (scoped_identifier scope: (identifier) name: (identifier))))))"},
		{"class A { void m() { yield++; int v = switch (k) { default -> { yield -1; } }; } }",
			"(program (class_declaration name: (identifier) body: (class_body (method_declaration type: (void_type) name: (identifier) parameters: (formal_parameters) body: (block (expression_statement (update_expression (identifier))) (local_variable_declaration type: (integral_type) declarator: (variable_declarator name: (identifier) value: (switch_expression condition: (parenthesized_expression (identifier)) body: (switch_block (switch_rule (switch_label) (block (yield_statement (unary_expression operand: (decimal_integer_literal))))))))))))))"},
		{"void main() { }\nint x = 1;\n",
			"(program (method_declaration type: (void_type) name: (identifier) parameters: (formal_parameters) body: (block)) (local_variable_declaration type: (integral_type) declarator: (variable_declarator name: (identifier) value: (decimal_integer_literal))))"},
	}
	for _, c := range cases {
		if got := syntax.SExp(parse(t, c.src)); got != c.want {
			t.Errorf("%q\n got  %s\n want %s", c.src, got, c.want)
		}
	}
}

func TestProgramSpansFirstTokenToEOF(t *testing.T) {
	root := parse(t, "  \n class A {}  \n")
	if root.Type != "program" || root.StartByte != 4 || root.EndByte != 17 {
		t.Fatalf("program span = [%d,%d), want [4,17)", root.StartByte, root.EndByte)
	}
	if root.StartPoint != (syntax.Point{Row: 1, Column: 1}) || root.EndPoint != (syntax.Point{Row: 2, Column: 0}) {
		t.Fatalf("program points = %v-%v", root.StartPoint, root.EndPoint)
	}
	empty := parse(t, "   ")
	if empty.StartByte != 3 || empty.EndByte != 3 || len(empty.Children) != 0 {
		t.Fatalf("whitespace-only program = %+v", empty)
	}
}

func TestCommentsAreExtrasInTheDeepestEnclosingNode(t *testing.T) {
	src := "// lead\nclass A {\n  int x = /* v */ 1; // tail\n}\n"
	root := parse(t, src)
	text := func(n *syntax.Node) string { return n.Text([]byte(src)) }
	lead := root.Children[0]
	if lead.Type != "line_comment" || !lead.Extra || text(lead) != "// lead" {
		t.Fatalf("leading comment must be the program's first child, got %s %q", lead.Type, text(lead))
	}
	if root.StartByte != 0 {
		t.Fatalf("program must start at the leading comment, got %d", root.StartByte)
	}
	body := root.Children[1].ChildByFieldName("body")
	field := body.Children[1]
	decl := field.ChildByFieldName("declarator")
	if c := decl.Children[2]; c.Type != "block_comment" || text(c) != "/* v */" {
		t.Fatalf("inline comment must sit inside the declarator, got %s %q", c.Type, text(c))
	}
	if c := body.Children[2]; c.Type != "line_comment" || text(c) != "// tail" {
		t.Fatalf("trailing comment must bubble up to class_body, got %s %q", c.Type, text(c))
	}
}

func TestDeepExpressionsDoNotOverflow(t *testing.T) {
	src := "class Huge { int huge() { return " + strings.Repeat("1 + ", 1199) + "1; } }"
	root := parse(t, src)
	type item struct {
		n     *syntax.Node
		depth int
	}
	depth, stack := 0, []item{{root, 0}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		depth = max(depth, it.depth)
		for _, c := range it.n.Children {
			stack = append(stack, item{c, it.depth + 1})
		}
	}
	if depth < 1200 {
		t.Fatalf("tree depth = %d, want >= 1200 (left-nested binary chain)", depth)
	}
}

func TestInvalidInputIsAnError(t *testing.T) {
	for _, src := range []string{"class A {", "class A { void m() { x = ; } }", "class A { String s = \"open"} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("Parse(%q) succeeded, want an error", src)
		}
	}
}
