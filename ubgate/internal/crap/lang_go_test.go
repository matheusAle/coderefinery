package crap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/model"
)

// goRows runs the reader from an empty working directory: relative paths
// resolve against the CWD, and no go.mod may sit above it (upstream runs its
// tests from a directory without one).
func goRows(t *testing.T, source, path, root string) []fnRow {
	t.Helper()
	t.Chdir(t.TempDir())
	fns, err := GoFunctions(source, path, root)
	if err != nil {
		t.Fatalf("GoFunctions: %v", err)
	}
	var rows []fnRow
	for _, f := range fns {
		if f.Language != "go" || f.Path != path || f.StartByte != -1 || f.EndByte != -1 {
			t.Fatalf("unexpected metadata %+v", f)
		}
		rows = append(rows, fnRow{f.Namespace, f.Name, f.Complexity})
	}
	return rows
}

// Ported from upstream tests/test_go.py.
func TestGoCountsTheSameDecisionsAsCrap4go(t *testing.T) {
	source := "package sample\n\nfunc Branchy(x int, ch chan int) int {\n\tif x > 0 && x < 10 {\n\t\treturn 1\n\t}\n\tswitch x {\n\tcase 1, 2:\n\t\treturn 2\n\tdefault:\n\t\treturn 3\n\t}\n}\n"
	expectRows(t, goRows(t, source, "sample.go", ""), fnRow{"sample", "Branchy", 5})
}

func TestGoMethodsAreNamespacedByReceiverType(t *testing.T) {
	source := "package sample\n\ntype Widget struct{}\n\nfunc (w *Widget) Run() {\n\tfor i := 0; i < 10; i++ {\n\t}\n\tfor _, item := range items {\n\t}\n}\n\nfunc Simple() int { return 1 }\n"
	expectRows(t, goRows(t, source, "widget.go", ""),
		fnRow{"sample.Widget", "Run", 3},
		fnRow{"sample", "Simple", 1})
}

func TestGoImportPathComesFromGoMod(t *testing.T) {
	root := t.TempDir()
	goWrite(t, filepath.Join(root, "go.mod"), "module github.com/acme/demo\n")
	source := "package game\n\ntype Board struct{}\n\nfunc (b *Board) Place() {\n\tif ready && ok {\n\t\treturn\n\t}\n}\n"
	goWrite(t, filepath.Join(root, "internal", "game", "board.go"), source)
	expectRows(t, goRows(t, source, "internal/game/board.go", root),
		fnRow{"github.com/acme/demo/internal/game.Board", "Place", 3})
}

// Ported from upstream tests/test_edges.py::test_go_edges.
func TestGoEdges(t *testing.T) {
	expectRows(t, goRows(t, "func Orphan() int { return 1 }\n", "board.go", ""), fnRow{"", "Orphan", 1})
	source := "package p\nfunc Declared(x int)\nfunc Real() int { return 1 }\nfunc () Run() {}\n"
	expectRows(t, goRows(t, source, "board.go", ""), fnRow{"p", "Real", 1})

	root := t.TempDir()
	goWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	path := filepath.Join(root, "board.go")
	goWrite(t, path, "package demo\nfunc Place() int { return 1 }\n")
	expectRows(t, goRows(t, "package demo\nfunc Place() int { return 1 }\n", path, root), fnRow{"example.com/demo", "Place", 1})
}

func TestGoDecisionsLinesAndClosures(t *testing.T) {
	source := `package p

// Doc comments are not part of the entry.
func Every(xs []int, ch chan int, v any) int {
	f := func() bool { return len(xs) > 0 || v == nil }
	for range xs {
	}
	switch v.(type) {
	case int, string:
	case nil:
	default:
	}
	select {
	case <-ch:
	case ch <- 1:
	default:
	}
	if f() && !f() {
	}
	return 0
}

func (Pair[K, V]) Generic() {}
`
	t.Chdir(t.TempDir())
	fns, err := GoFunctions(source, "p.go", "")
	if err != nil {
		t.Fatal(err)
	}
	got := []model.Function{}
	for _, f := range fns {
		f.Path, f.Language = "", ""
		got = append(got, f)
	}
	want := []model.Function{
		// 1 + || + for + 3 type cases + 3 select cases + if + && = 11
		{Name: "Every", Namespace: "p", Complexity: 11, StartLine: 4, EndLine: 21, StartByte: -1, EndByte: -1},
		{Name: "Generic", Namespace: "p.Pair", Complexity: 1, StartLine: 23, EndLine: 23, StartByte: -1, EndByte: -1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestGoPackageNamespaceRules(t *testing.T) {
	root := t.TempDir()
	goWrite(t, filepath.Join(root, "go.mod"), "// header\r\n  module   \"quoted\" // kept  \r\n")
	goWrite(t, filepath.Join(root, "sub", "go.mod"), "go 1.22\n")
	goWrite(t, filepath.Join(root, "a", "b", "x.go"), "")
	cases := []struct{ path, pkg, want string }{
		{filepath.Join(root, "x.go"), "p", `"quoted" // kept`},
		{filepath.Join(root, "a", "b", "x.go"), "p", `"quoted" // kept/a/b`},
		// the nearest go.mod stops the walk even without a module line
		{filepath.Join(root, "sub", "x.go"), "fallback", "fallback"},
	}
	for _, c := range cases {
		got, err := GoPackageNamespace(c.path, c.pkg)
		if err != nil || got != c.want {
			t.Errorf("GoPackageNamespace(%s) = %q, %v; want %q", c.path, got, err, c.want)
		}
	}
	tab := t.TempDir()
	goWrite(t, filepath.Join(tab, "go.mod"), "module\tx\n")
	if got, _ := GoPackageNamespace(filepath.Join(tab, "x.go"), "pkg"); got != "pkg" {
		t.Errorf("a tab after module must not match, got %q", got)
	}
	bad := t.TempDir()
	goWrite(t, filepath.Join(bad, "go.mod"), "module \xff\n")
	if _, err := GoPackageNamespace(filepath.Join(bad, "x.go"), "pkg"); err == nil {
		t.Error("invalid UTF-8 go.mod must fail")
	}
}

func TestGoParseErrorYieldsNoFunctions(t *testing.T) {
	fns, err := GoFunctions("package p\nfunc place() int { return \n", "p.go", "")
	if err == nil || fns != nil {
		t.Fatalf("want a parse error, got %v %v", fns, err)
	}
}

func TestGoDeepExpressionDoesNotOverflow(t *testing.T) {
	source := "package p\nfunc place() int {\n\treturn " + strings.Repeat("1 + ", 1199) + "1\n}\n"
	expectRows(t, goRows(t, source, "p.go", ""), fnRow{"p", "place", 1})
}

// Smoke: the whole Go fixture through AnalyzeFiles (no coverage), against
// upstream crapper's --no-coverage report.
func TestGoFixtureSmoke(t *testing.T) {
	fixture, err := filepath.Abs("../../../fixtures/go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("fixtures/go not present")
	}
	files := []string{filepath.Join(fixture, "calc.go"), filepath.Join(fixture, "geometry", "shapes.go")}
	entries, err := AnalyzeFiles(files, fixture, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		if e.Coverage != nil || e.Crap != nil || e.Language != "go" {
			t.Fatalf("unexpected entry %+v", e)
		}
		got = append(got, fmt.Sprintf("%s %s %d %s", e.Name, e.Namespace, e.Complexity, e.Path))
	}
	// upstream crapper --no-coverage on fixtures/go (harness golden)
	want := []string{
		"Grade example.com/ubfix 7 calc.go",
		"Collect example.com/ubfix/geometry 6 geometry/shapes.go",
		"Mean example.com/ubfix 5 calc.go",
		"SumPositive example.com/ubfix 3 calc.go",
		"SumWeights example.com/ubfix 3 calc.go",
		"clampIndex example.com/ubfix 3 calc.go",
		"Area example.com/ubfix/geometry.Rect 3 geometry/shapes.go",
		"Scale example.com/ubfix/geometry.Rect 2 geometry/shapes.go",
		"square example.com/ubfix/geometry.Rect 2 geometry/shapes.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries:\n got  %q\n want %q", got, want)
	}
}

func goWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
