package java

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// The escape arms crap --branches reported untested. Expected strings are
// upstream tree-sitter's str(root_node) (tools/oracle/oracle.py --sexp).
func TestStringEscapesMatchTreeSitter(t *testing.T) {
	const field = "(program (class_declaration name: (identifier) body: (class_body (field_declaration type: (type_identifier) declarator: (variable_declarator name: (identifier) value: "
	cases := []struct{ src, want string }{
		{"class A { String s = \"\\u0041\\t\\101\\\\\"; }\n", field + "(string_literal (escape_sequence) (escape_sequence) (escape_sequence) (escape_sequence)))))))"},
		{"class A { String s = \"\\x41\\u{1F600}z\"; }\n", field + "(string_literal (escape_sequence) (escape_sequence) (string_fragment)))))))"},
		{"class A { String s = \"\"\"\n  a\\x41\\u0041\\q\n  \"\"\"; }\n", field + "(string_literal (multiline_string_fragment) (escape_sequence) (escape_sequence) (multiline_string_fragment)))))))"},
	}
	for _, c := range cases {
		if got := syntax.SExp(parse(t, c.src)); got != c.want {
			t.Errorf("%q\n got  %s\n want %s", c.src, got, c.want)
		}
	}
	for _, bad := range []string{"class A { String s = \"\\", "class A { String s = \"\\x4\"; }", "class A { String s = \"\\u{}\"; }", "class A { String s = \"\\u12\"; }"} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("Parse(%q): want an error for the bad escape", bad)
		}
	}
}
