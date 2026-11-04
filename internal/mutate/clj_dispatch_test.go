package mutate

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

func cljTypes(n *syntax.Node) []string {
	var out []string
	n.Descendants(func(c *syntax.Node) bool { out = append(out, c.Type); return true })
	return out
}

// The `#` dispatch forms crap --branches reported untested. tree-sitter-clojure
// (oracle.py --lang clojure --sexp) reads the probe as
// (list_lit sym_lit regex_lit var_quoting_lit read_cond_lit
// splicing_read_cond_lit sym_lit[old_meta] evaling_lit char_lit x3).
func TestReaderDispatchFormsTakeTreeSitterClojureTypes(t *testing.T) {
	src := "(f #\"a\\\"b\" #'v #?(:clj 1) #?@(:clj [1]) #^:m x #=(+ 1 2) \\a \\newline \\u0041)\n"
	list := parseClojure([]byte(src)).Children[0]
	var got []string
	for _, c := range list.NamedChildren() {
		got = append(got, c.Type)
	}
	want := []string{"sym_lit", "regex_lit", "var_quoting_lit", "read_cond_lit", "splicing_read_cond_lit", "sym_lit", "evaling_lit", "char_lit", "char_lit", "char_lit"}
	if list.Type != "list_lit" || len(got) != len(want) {
		t.Fatalf("%s children %v, want %v", list.Type, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("child %d = %s, want %s (all: %v)", i, got[i], want[i], got)
		}
	}
	texts := map[string]string{}
	for _, c := range list.NamedChildren() {
		texts[c.Type] = c.Text([]byte(src))
	}
	if texts["regex_lit"] != `#"a\"b"` || texts["evaling_lit"] != "#=(+ 1 2)" || texts["var_quoting_lit"] != "#'v" {
		t.Errorf("spans %q", texts)
	}
	// The #^ metadata becomes the first child of the symbol it precedes.
	// tree-sitter calls it old_meta_lit; the reader emits meta_lit for both
	// ^ and #^, which is all the site rules read (DEVIATIONS M5).
	if meta := list.NamedChildren()[5]; meta.Text([]byte(src)) != "#^:m x" || meta.Children[0].Type != "meta_lit" || !containsString(cljTypes(meta), "sym_name") {
		t.Errorf("old meta %q %v", meta.Text([]byte(src)), cljTypes(meta))
	}
}
