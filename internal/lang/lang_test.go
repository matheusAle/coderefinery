package lang

import "testing"

func TestGrammarForMapsExtensionsLikeTheUpstreamTools(t *testing.T) {
	cases := map[string]string{
		"a.py": "python", "a.go": "go", "A.java": "java", "lib.rs": "rust", "app.tsx": "tsx",
		"a.js": "javascript", "a.jsx": "javascript", "a.mjs": "javascript", "a.cjs": "javascript",
		"a.ts": "typescript", "a.mts": "typescript", "a.cts": "typescript", "types.d.ts": "typescript",
		"core.clj": "", "README": "", "a.PY": "", "dir.py/file": "",
	}
	for path, want := range cases {
		if got := GrammarFor(path); got != want {
			t.Errorf("GrammarFor(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestParseReturnsATreeForEveryRegisteredGrammar(t *testing.T) {
	for grammar, src := range map[string]string{
		"python": "x = 1\n", "go": "package p\n", "java": "class A {}\n", "rust": "fn f() {}\n",
		"typescript": "let x = 1;\n", "tsx": "const a = <b/>;\n", "javascript": "let x = 1;\n",
	} {
		tree, err := ParseText(grammar, src)
		if err != nil || !Supported(grammar) || tree.Language != grammar || string(tree.Source) != src || tree.Root.EndByte != len(src) {
			t.Errorf("%s: tree %+v, err %v", grammar, tree, err)
		}
	}
	if tree, err := Parse("cobol", []byte("x")); tree != nil || err == nil || err.Error() != `no parser for grammar "cobol"` || Supported("cobol") {
		t.Errorf("unknown grammar: %v, %v", tree, err)
	}
	if _, err := Parse("python", []byte("def (:\n")); err == nil {
		t.Error("invalid input is an error (DEVIATIONS A2)")
	}
}
