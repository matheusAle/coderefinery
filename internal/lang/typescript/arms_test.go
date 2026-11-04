package typescript

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// Arms crap --branches reported untested: `export default` declarations,
// string/template escapes and JSX character references. Expected strings
// are upstream tree-sitter's str(root_node) (tools/oracle/oracle.py --sexp).
func TestExportDefaultEscapesAndCharRefsMatchTreeSitter(t *testing.T) {
	const fn = "parameters: (formal_parameters) body: (statement_block)"
	cases := []struct{ grammar, src, want string }{
		{"ts", "export default function f() {}\n", "(program (export_statement declaration: (function_declaration name: (identifier) " + fn + ")))"},
		{"ts", "export default function* g() {}\n", "(program (export_statement declaration: (generator_function_declaration name: (identifier) " + fn + ")))"},
		{"ts", "export default function () {}\n", "(program (export_statement value: (function_expression " + fn + ")))"},
		{"ts", "export default async function h() {}\n", "(program (export_statement declaration: (function_declaration name: (identifier) " + fn + ")))"},
		{"ts", "export default async function* k() {}\n", "(program (export_statement declaration: (generator_function_declaration name: (identifier) " + fn + ")))"},
		{"ts", "export default async function () {}\n", "(program (export_statement value: (function_expression " + fn + ")))"},
		{"ts", "export default class C extends B {}\n", "(program (export_statement declaration: (class_declaration name: (type_identifier) (class_heritage (extends_clause value: (identifier))) body: (class_body))))"},
		{"ts", "export default class extends B {}\n", "(program (export_statement value: (class (class_heritage (extends_clause value: (identifier))) body: (class_body))))"},
		{"ts", "export default abstract class A {}\n", "(program (export_statement declaration: (abstract_class_declaration name: (type_identifier) body: (class_body))))"},
		{"ts", "export default interface I {}\n", "(program (export_statement declaration: (interface_declaration name: (type_identifier) body: (interface_body))))"},
		{"ts", "export default x + 1;\n", "(program (export_statement value: (binary_expression left: (identifier) right: (number))))"},
		{"ts", "x = \"\\x41\\u0041\\u{1F600}\\0\\12\\n\";\n", "(program (expression_statement (assignment_expression left: (identifier) right: (string (escape_sequence) (escape_sequence) (escape_sequence) (escape_sequence) (escape_sequence) (escape_sequence)))))"},
		{"ts", "x = '\\u{41}b\\x7az';\n", "(program (expression_statement (assignment_expression left: (identifier) right: (string (escape_sequence) (string_fragment) (escape_sequence) (string_fragment)))))"},
		{"ts", "x = `a\\x41${b}\\u0042`;\n", "(program (expression_statement (assignment_expression left: (identifier) right: (template_string (string_fragment) (escape_sequence) (template_substitution (identifier)) (escape_sequence)))))"},
		{"js", "x = <p>&#x41; &#65; &amp;</p>;\n", "(program (expression_statement (assignment_expression left: (identifier) right: (jsx_element open_tag: (jsx_opening_element name: (identifier)) (html_character_reference) (jsx_text) (html_character_reference) (jsx_text) (html_character_reference) close_tag: (jsx_closing_element name: (identifier))))))"},
	}
	for _, c := range cases {
		if got := syntax.SExp(parseFor(t, c.grammar, c.src)); got != c.want {
			t.Errorf("%s %q\n got  %s\n want %s", c.grammar, c.src, got, c.want)
		}
	}
	// Smoke: the escapes keep their source text.
	src := "x = '\\u{41}b\\x7az';\n"
	root := parseFor(t, "ts", src)
	var escapes []string
	root.Descendants(func(n *syntax.Node) bool {
		if n.Type == "escape_sequence" {
			escapes = append(escapes, n.Text([]byte(src)))
		}
		return true
	})
	if len(escapes) != 2 || escapes[0] != `\u{41}` || escapes[1] != `\x7a` {
		t.Errorf("escape texts %q", escapes)
	}
}
