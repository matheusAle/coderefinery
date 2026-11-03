package mutate

// Ported from upstream tests/test_boundaries.py
// (test_privacy_headers_digests_owners_and_mutation_fields) plus the
// mutation-id encoding probes of spec-mutator §2.4.

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/coverage"
	"github.com/matheusAle/ub-mode/ubgate/internal/model"
)

func linesOf(s string) []string { return coverage.SplitLines(s) }

func TestPrivacyFollowsEachLanguagesRule(t *testing.T) {
	cases := []struct {
		language, name, source string
		start, end             int
		want                   bool
	}{
		{"python", "__init__", "def __init__(self):\n    pass\n", 1, 2, false},
		{"python", "_hide", "def _hide():\n    return 1\n", 1, 2, true},
		{"python", "__x", "def __x():\n    return 1\n", 1, 2, true},
		{"rust", "place", "fn place() {}\n", 1, 1, true},
		{"rust", "place", "#[inline]\npub fn place() {}\n", 1, 2, false},
		{"rust", "place", "pub(crate) fn place() {}\n", 1, 1, true},
		{"typescript", "#place", "#place() {}\n", 1, 1, true},
		{"typescript", "place", "private place() {}\n", 1, 1, true},
		{"typescript", "place", "function place() {}\n", 1, 1, false},
		{"typescript", "place", "function place() { privately() }\n", 1, 1, false},
		{"java", "place", "  void place() {\n    // private note\n  }\n", 1, 3, true},
		{"go", "place", "func place() {}\n", 1, 1, true},
		{"go", "_x", "func _x() {}\n", 1, 1, false},
		{"go", "Place", "func Place() {}\n", 1, 1, false},
		{"clojure", "hide", "(ns demo.core)\n(defn- hide []\n  false)\n", 2, 3, true},
		{"clojure", "place", "(defn place [])\n", 1, 1, false},
		{"cobol", "place", "PLACE.\n", 1, 1, false},
	}
	for _, c := range cases {
		if got := isPrivate(c.language, c.name, linesOf(c.source), c.start, c.end); got != c.want {
			t.Errorf("isPrivate(%s, %s) = %v, want %v", c.language, c.name, got, c.want)
		}
	}
	if formID("hide", true) != "defn-/hide" || formID("place", false) != "defn/place" {
		t.Fatal("form ids")
	}
}

func TestDigestHashesTheRstrippedFunctionText(t *testing.T) {
	sum := sha256.Sum256([]byte("def place(x):\n    return x > 0"))
	if got := digest(linesOf("def place(x):\n    return x > 0\n"), 1, 2); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest %s", got)
	}
	// Trailing whitespace (including \x1f, a Python space) does not count.
	if digest(linesOf("def a():  \x1f\n    pass\t\n"), 1, 2) != digest(linesOf("def a():\n    pass\n"), 1, 2) {
		t.Fatal("rstrip must use Python's whitespace set")
	}
}

func TestDigestOfAFormIsTheHashOfItsFunctionHashes(t *testing.T) {
	root := tempRoot(t)
	src := "def add(a, b):\n    return a + b\n"
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), src)
	u := newFileUnit(src, path, root, "src/demo.py")
	inner := digest(u.lines, 1, 2)
	if got := u.digests()[formKey{"demo", "defn/add"}]; got != sha256Hex(inner) {
		t.Fatalf("form digest %s, want sha256(%s)", got, inner)
	}
}

func TestOwnerIsTheTightestFunction(t *testing.T) {
	fns := []model.Function{
		{Name: "outer", StartLine: 1, EndLine: 10, StartByte: -1, EndByte: -1},
		{Name: "inner", StartLine: 8, EndLine: 9, StartByte: -1, EndByte: -1},
	}
	if i := owner(fns, 8, 0); i != 1 {
		t.Fatalf("owner %d, want inner", i)
	}
	if i := owner(fns, 11, 0); i != -1 {
		t.Fatalf("a site outside every function has no owner, got %d", i)
	}
	spans := []model.Function{
		{Name: "a", StartLine: 1, EndLine: 1, StartByte: 0, EndByte: 10},
		{Name: "b", StartLine: 1, EndLine: 1, StartByte: 10, EndByte: 30},
	}
	if i := owner(spans, 1, 12); i != 1 {
		t.Fatalf("byte spans decide on one line, got %d", i)
	}
}

func TestMutationIDsAreCompactPythonJSON(t *testing.T) {
	got := mutationID("src/caf/é<&.py", "caf.é<&", "defn/g", 23, 24, ">", ">=")
	esc := `\` + "u00e9" // json.dumps escape of é
	want := `["src/caf/` + esc + `<&.py","caf.` + esc + `<&","defn/g",23,24,">",">="]`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if f, ok := mutationFile(got); !ok || f != "src/caf/é<&.py" {
		t.Fatalf("decoded file %q %v", f, ok)
	}
	if ns, ok := mutationNamespace(got); !ok || ns != "caf.é<&" {
		t.Fatalf("decoded namespace %q", ns)
	}
	for _, bad := range [][2]any{{"not-json", 0}, {`"hello"`, 0}, {"[1]", 0}, {`["only"]`, 1}} {
		if _, ok := mutationField(bad[0].(string), bad[1].(int)); ok {
			t.Errorf("mutationField(%v) must be None", bad)
		}
	}
}

func TestFormSpansMarkPrivateForms(t *testing.T) {
	root := tempRoot(t)
	src := "def _hide():\n    return 1\n"
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), src)
	spans := newFileUnit(src, path, root, "src/demo.py").spans()
	if len(spans) != 1 || !spans[0].private || spans[0].key != (formKey{"demo", "defn-/_hide"}) {
		t.Fatalf("spans %+v", spans)
	}
}
