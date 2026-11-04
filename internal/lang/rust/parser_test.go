package rust

import (
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/syntax"
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
// source (tools/oracle/oracle.py --sexp).
func TestShapesMatchTreeSitter(t *testing.T) {
	cases := map[string]string{
		// `..` and a bare `break` give way to an operator that could continue
		// the expression; a block-like statement ends before `-` but not `.`.
		"fn f() { a..-b; break (x); if a {} - 1; match x {}.f(); }\n": "(source_file (function_item name: (identifier) parameters: (parameters) body: (block (expression_statement (binary_expression left: (range_expression (identifier)) right: (identifier))) (expression_statement (call_expression function: (break_expression) arguments: (arguments (identifier)))) (expression_statement (if_expression condition: (identifier) consequence: (block))) (expression_statement (unary_expression (integer_literal))) (expression_statement (call_expression function: (field_expression value: (match_expression value: (identifier) body: (match_block)) field: (field_identifier)) arguments: (arguments))))))",
		// char literal vs lifetime, raw string
		"fn f() { let c = '\\''; let l: &'a str = r#\"x\"#; }\n": "(source_file (function_item name: (identifier) parameters: (parameters) body: (block (let_declaration pattern: (identifier) value: (char_literal)) (let_declaration pattern: (identifier) type: (reference_type (lifetime (identifier)) type: (primitive_type)) value: (raw_string_literal (string_content))))))",
		"fn f() { if let Some(x) = y && x > 0 {} }\n":            "(source_file (function_item name: (identifier) parameters: (parameters) body: (block (expression_statement (if_expression condition: (let_chain (let_condition pattern: (tuple_struct_pattern type: (identifier) (identifier)) value: (identifier)) (binary_expression left: (identifier) right: (integer_literal))) consequence: (block))))))",
		// a prefix `..=` is `..` then `=`; tuple indices are integers
		"fn f() { v.drain(..=5); x.0.1; }\n": "(source_file (function_item name: (identifier) parameters: (parameters) body: (block (expression_statement (call_expression function: (field_expression value: (identifier) field: (field_identifier)) arguments: (arguments (assignment_expression left: (range_expression) right: (integer_literal))))) (expression_statement (field_expression value: (field_expression value: (identifier) field: (integer_literal)) field: (integer_literal))))))",
		// `>>` closes two lists, `||` is an empty closure, `&&` two references
		"fn f() { Vec::<Vec<u8>>::new(); || x; &&y; }\n":                 "(source_file (function_item name: (identifier) parameters: (parameters) body: (block (expression_statement (call_expression function: (scoped_identifier path: (generic_type type: (type_identifier) type_arguments: (type_arguments (generic_type type: (type_identifier) type_arguments: (type_arguments (primitive_type))))) name: (identifier)) arguments: (arguments))) (expression_statement (closure_expression parameters: (closure_parameters) body: (identifier))) (expression_statement (reference_expression value: (reference_expression value: (identifier)))))))",
		"macro_rules! m { ($($x:expr),* ; $y:ident) => { $($x)+* }; }\n": "(source_file (macro_definition name: (identifier) (macro_rule left: (token_tree_pattern (token_repetition_pattern (token_binding_pattern name: (metavariable) type: (fragment_specifier))) (token_binding_pattern name: (metavariable) type: (fragment_specifier))) right: (token_tree (token_repetition (metavariable))))))",
		"/* a /* b */ c */ fn f() {} // tail\n":                          "(source_file (block_comment) (function_item name: (identifier) parameters: (parameters) body: (block)) (line_comment))",
	}
	for src, want := range cases {
		if got := syntax.SExp(parse(t, src)); got != want {
			t.Errorf("%q\n got  %s\n want %s", src, got, want)
		}
	}
}

func TestDocCommentsKeepMarkersAndNewlines(t *testing.T) {
	src := "/// outer\n//! inner\n/** block */\nfn f() {}\n"
	root := parse(t, src)
	dump := syntax.Dump(root, []byte(src))
	for _, want := range []string{
		`line_comment [0,10) 0:0-1:0 EXTRA`,
		`outer: outer_doc_comment_marker [2,3) 0:2-0:3`,
		`doc: doc_comment [3,10) 0:3-1:0 " outer\n"`,
		`inner: inner_doc_comment_marker [12,13) 1:2-1:3`,
		`outer: outer_doc_comment_marker [22,23) 2:2-2:3 "*"`,
		`doc: doc_comment [23,30) 2:3-2:10 " block "`,
	} {
		if !strings.Contains(dump, want) {
			t.Errorf("dump lacks %q:\n%s", want, dump)
		}
	}
	if got := root.Children[3].Type; got != "function_item" || root.StartByte != 0 || root.EndByte != len(src) {
		t.Fatalf("root %s [%d,%d), fourth child %s", root.Type, root.StartByte, root.EndByte, got)
	}
}

func TestRawStringContentSkipsLeadingWhitespace(t *testing.T) {
	src := "fn f() { r\"  \n x\"; r\"\"; }\n"
	dump := syntax.Dump(parse(t, src), []byte(src))
	for _, want := range []string{`string_content [15,16) 1:1-1:2 "x"`, `string_content [21,21) 1:7-1:7 ""`} {
		if !strings.Contains(dump, want) {
			t.Errorf("dump lacks %q:\n%s", want, dump)
		}
	}
}

func TestEmptyAndWhitespaceOnlySources(t *testing.T) {
	for src, want := range map[string]string{"": "source_file [0,0) 0:0-0:0 \"\"\n", "  \n": "source_file [3,3) 1:0-1:0 \"\"\n"} {
		if got := syntax.Dump(parse(t, src), []byte(src)); got != want {
			t.Errorf("%q: got %q, want %q", src, got, want)
		}
	}
}

func TestInvalidSourceIsAnError(t *testing.T) {
	for _, src := range []string{"fn f( {", "fn f() { \"unterminated }", "/* open", "fn f() { let x = ; }"} {
		if root, err := Parse([]byte(src)); err == nil || root != nil {
			t.Errorf("%q: got %v, %v; want an error", src, root, err)
		}
	}
}

func TestDeepExpressionsDoNotOverflow(t *testing.T) {
	src := "fn f() -> i32 { 0" + strings.Repeat(" + 1", 1200) + " }\n"
	root := parse(t, src)
	depth := 0
	for n := root; len(n.Children) > 0; n = n.Children[len(n.Children)-1] {
		if n.Type == "block" {
			n = n.Children[1]
			for n.Type == "binary_expression" {
				depth++
				n = n.ChildByFieldName("left")
			}
			break
		}
	}
	if depth != 1200 {
		t.Fatalf("binary nesting %d, want 1200", depth)
	}
}
