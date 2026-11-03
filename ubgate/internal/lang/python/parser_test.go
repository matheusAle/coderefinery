package python

import (
	"strings"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

func sexp(t *testing.T, src string) string {
	t.Helper()
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return syntax.SExp(root)
}

// Expected strings are upstream tree-sitter's str(root_node) for the same
// source (tools/oracle/oracle.py --sexp).
func TestShapesMatchTreeSitter(t *testing.T) {
	cases := map[string]string{
		"def f(x):\n    return x if x else 0\n": "(module (function_definition name: (identifier) parameters: (parameters (identifier)) body: (block (return_statement (conditional_expression (identifier) (identifier) (integer))))))",
		"a & b ^ c\n":                           "(module (binary_operator left: (identifier) right: (binary_operator left: (identifier) right: (identifier))))",
		"a, b = b, a\n":                         "(module (assignment left: (pattern_list (identifier) (identifier)) right: (expression_list (identifier) (identifier))))",
		"True, None\n":                          "(module (tuple_expression (true) (none)))",
		"x: A[int] | B[c] | D\n":                "(module (assignment left: (identifier) type: (type (union_type (type (union_type (type (generic_type (identifier) (type_parameter (type (identifier))))) (type (generic_type (identifier) (type_parameter (type (identifier))))))) (type (identifier))))))",
		"[*a.b]\n":                              "(module (list (attribute object: (list_splat (identifier)) attribute: (identifier))))",
		"[*a.b, c]\n":                           "(module (list (list_splat (attribute object: (identifier) attribute: (identifier))) (identifier)))",
		"type(a).b = 1\n":                       "(module (type_alias_statement left: (type (attribute object: (parenthesized_expression (identifier)) attribute: (identifier))) right: (type (integer))))",
		"print \"hi\"\n":                        "(module (print_statement argument: (string (string_start) (string_content) (string_end))))",
		"a not in b\n":                          "(module (comparison_operator (identifier) (identifier)))",
	}
	for src, want := range cases {
		if got := sexp(t, src); got != want {
			t.Errorf("%q\n got  %s\n want %s", src, got, want)
		}
	}
}

func TestMatchStatementShape(t *testing.T) {
	got := sexp(t, "match x:\n    case -1 | [a, *r] if a:\n        pass\n    case _:\n        pass\n")
	want := "(module (match_statement subject: (identifier) body: (block alternative: (case_clause (case_pattern (union_pattern (integer) (list_pattern (case_pattern (dotted_name (identifier))) (case_pattern (splat_pattern (identifier)))))) guard: (if_clause (identifier)) consequence: (block (pass_statement))) alternative: (case_clause (case_pattern) consequence: (block (pass_statement))))))"
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestCommentsAndLayoutTokens(t *testing.T) {
	src := "def a(x):\n    return x\n    # trailing\n\ny = 1  # c\n"
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	fn := root.Children[0]
	block := fn.ChildByFieldName("body")
	last := block.Children[len(block.Children)-1]
	if last.Type != "comment" || !last.Extra || last.Text([]byte(src)) != "# trailing" {
		t.Fatalf("trailing comment must stay in the block, got %+v", last)
	}
	if fn.EndLine() != 3 || block.EndPoint != (syntax.Point{Row: 2, Column: 14}) {
		t.Fatalf("function ends at line %d, block at %+v", fn.EndLine(), block.EndPoint)
	}
	if root.StartByte != 0 || root.EndByte != len(src) || root.Children[2].Type != "comment" {
		t.Fatalf("module span/children: %d-%d %v", root.StartByte, root.EndByte, syntax.SExp(root))
	}
	for _, n := range []*syntax.Node{fn, block} {
		if n.Parent == nil {
			t.Fatal("Finish must set parents")
		}
	}
}

func TestDeepExpressionDoesNotBlowTheStack(t *testing.T) {
	src := "x = " + strings.Repeat("1 + ", 3000) + "1\n"
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	// Left-associative: the left spine holds 3000 binary_operator nodes.
	spine := 0
	n := root.Children[0].ChildByFieldName("right")
	for n.Type == "binary_operator" {
		spine++
		n = n.ChildByFieldName("left")
	}
	if spine != 3000 || n.Type != "integer" {
		t.Fatalf("spine %d ending in %s, want 3000 ending in integer", spine, n.Type)
	}
	count := 0
	root.Descendants(func(*syntax.Node) bool { count++; return true })
	// module + assignment + x + "=" + 3001 integers + 3000 "+" + 3000 binaries
	if count != 9005 {
		t.Fatalf("node count %d, want 9005", count)
	}
}

func TestUnsupportedInputIsAnError(t *testing.T) {
	for _, src := range []string{"x = 'unterminated\n", "def (:\n", "a = `b`\n"} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("Parse(%q) should fail", src)
		}
	}
}
