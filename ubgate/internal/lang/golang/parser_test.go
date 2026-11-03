package golang

import (
	"strings"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

func dump(t *testing.T, src string) string {
	t.Helper()
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return syntax.Dump(root, []byte(src))
}

// Recorded from upstream tree-sitter (tools/oracle/oracle.py): the syntactic
// m[k](x) conversion, the hidden "\n\n" terminator stretching statement_list
// and a trailing comment kept inside it.
func TestConversionTerminatorAndComment(t *testing.T) {
	want := `
source_file [0,43) 0:0-5:0
  package_clause [0,9) 0:0-0:9
    "package" [0,7) 0:0-0:7 "package"
    package_identifier [8,9) 0:8-0:9 "p"
  function_declaration [10,42) 1:0-4:1
    "func" [10,14) 1:0-1:4 "func"
    name: identifier [15,16) 1:5-1:6 "f"
    parameters: parameter_list [16,18) 1:6-1:8
      "(" [16,17) 1:6-1:7 "("
      ")" [17,18) 1:7-1:8 ")"
    body: block [19,42) 1:9-4:1
      "{" [19,20) 1:9-1:10 "{"
      statement_list [22,41) 2:1-4:0
        short_var_declaration [22,34) 2:1-2:13
          left: expression_list [22,23) 2:1-2:2
            identifier [22,23) 2:1-2:2 "z"
          ":=" [24,26) 2:3-2:5 ":="
          right: expression_list [27,34) 2:6-2:13
            type_conversion_expression [27,34) 2:6-2:13
              type: generic_type [27,31) 2:6-2:10
                type: type_identifier [27,28) 2:6-2:7 "m"
                type_arguments: type_arguments [28,31) 2:7-2:10
                  "[" [28,29) 2:7-2:8 "["
                  type_elem [29,30) 2:8-2:9
                    type_identifier [29,30) 2:8-2:9 "k"
                  "]" [30,31) 2:9-2:10 "]"
              "(" [31,32) 2:10-2:11 "("
              operand: identifier [32,33) 2:11-2:12 "x"
              ")" [33,34) 2:12-2:13 ")"
        comment [35,39) 2:14-2:18 EXTRA "// c"
      "}" [41,42) 4:0-4:1 "}"
`
	got := dump(t, "package p\nfunc f() {\n\tz := m[k](x) // c\n\n}\n")
	if got != strings.TrimPrefix(want, "\n") {
		t.Fatalf("dump mismatch:\n%s", got)
	}
}

// A top-level var with a value at EOF (no newline) ends with tree-sitter's
// zero-width "\0" terminator; escapes split string content.
func TestEOFTerminatorAndEscapes(t *testing.T) {
	src := "package p\nvar s = \"a\\x41\""
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	last := root.Children[len(root.Children)-1]
	if last.Type != "" || last.Named || last.StartByte != len(src) || last.EndByte != len(src) {
		t.Fatalf("last child = %+v, want zero-width \"\" at EOF", last)
	}
	str := root.Children[1].Children[1].ChildByFieldName("value").Children[0]
	var kinds []string
	for _, c := range str.Children {
		kinds = append(kinds, c.Type+"="+c.Text([]byte(src)))
	}
	if got := strings.Join(kinds, " "); got != `"=" interpreted_string_literal_content=a escape_sequence=\x41 "="` {
		t.Fatalf("string children: %s", got)
	}
	if root.StartByte != 0 || root.EndByte != len(src) || root.EndPoint != (syntax.Point{Row: 1, Column: 15}) {
		t.Fatalf("source_file span %d-%d %v", root.StartByte, root.EndByte, root.EndPoint)
	}
}

func TestParameterGroupingQuirks(t *testing.T) {
	src := "package p\nfunc f(a1, a2, a3, a4, a5, a6, a7, a8, a9, z int, q, func(int)) {}\n"
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	params := root.Children[1].ChildByFieldName("parameters")
	var decls []string
	for _, d := range params.NamedChildren() {
		var parts []string
		for _, c := range d.NamedChildren() {
			parts = append(parts, c.Field+":"+c.Text([]byte(src)))
		}
		decls = append(decls, strings.Join(parts, ","))
	}
	want := []string{
		"type:a1",
		"name:a2,name:a3,name:a4,name:a5,name:a6,name:a7,name:a8,name:a9,name:z,type:int",
		"name:q,name:func,type:(int)",
	}
	if strings.Join(decls, " | ") != strings.Join(want, " | ") {
		t.Fatalf("parameter declarations:\n got  %q\n want %q", decls, want)
	}
}

func TestInvalidInputIsAnError(t *testing.T) {
	for _, src := range []string{"package p\nfunc f() { return \n", "package p\nvar x = )\n", "\xef\xbb\xbfpackage p\n"} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("Parse(%q) succeeded, want an error", src)
		}
	}
}

func TestDeepExpressionsAreFine(t *testing.T) {
	src := "package p\nvar x = " + strings.Repeat("(", 2000) + "1" + strings.Repeat(")", 2000) + " + " + strings.Repeat("1 + ", 3000) + "1\n"
	root, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	n := root.Children[1].Children[1].ChildByFieldName("value")
	depth := 0
	for len(n.Children) > 0 {
		n = n.Children[0]
		if n.Type == "(" {
			n = n.Parent.Children[1]
		}
		depth++
	}
	if depth < 5000 || n.Type != "int_literal" {
		t.Fatalf("leftmost leaf %q at depth %d", n.Type, depth)
	}
}
