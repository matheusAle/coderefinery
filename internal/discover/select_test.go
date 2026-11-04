package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func selectTree(t *testing.T) string {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	for _, rel := range []string{"src/b.py", "src/a.py", "src/a_test.py", "lib/c.go", "notes.md"} {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, nil, 0o644)
	}
	return root
}

func selection(root string, positionals ...string) Selection {
	return Selection{
		ProjectRoot: root,
		Positionals: positionals,
		Walk:        IterSourceFiles,
		KeepChanged: func(string, string) bool { return true },
		KeepFile:    func(p string) bool { return LanguageOf(p) != "" },
	}
}

func TestSelectWalksTheRootWhenNothingIsNamed(t *testing.T) {
	root := selectTree(t)
	got, err := Select(selection(root))
	want := []string{filepath.Join(root, "lib/c.go"), filepath.Join(root, "src/a.py"), filepath.Join(root, "src/b.py")}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Select = %v, %v; want %v", got, err, want)
	}
}

func TestSelectSplitsPositionalsIntoPathsAndFilters(t *testing.T) {
	root := selectTree(t)
	// "src" exists (walked), "src/a_test.py" is named (kept: a source
	// language), "notes.md" is named but has no language, "src/a" is a filter.
	got, err := Select(selection(root, "src", "src/a_test.py", "notes.md", "src/a"))
	want := []string{filepath.Join(root, "src/a.py"), filepath.Join(root, "src/a_test.py")}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Select = %v, %v; want %v", got, err, want)
	}
	if got, _ := Select(selection(root, "nothing-matches")); got != nil {
		t.Fatalf("an unmatched filter selects nothing, got %v", got)
	}
}

func TestSelectSourceRootsWinOverNamedPaths(t *testing.T) {
	root := selectTree(t)
	s := selection(root, "src")
	s.SourceRoots = []string{"lib", filepath.Join(root, "lib")}
	got, _ := Select(s)
	if want := []string{filepath.Join(root, "lib/c.go")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Select = %v, want %v (deduplicated, src ignored)", got, want)
	}
}

func TestSelectReportsAndDropsFilesOutsideTheRootOnlyWithOutside(t *testing.T) {
	root := selectTree(t)
	outside := filepath.Join(root, "lib/c.go")
	s := selection(filepath.Join(root, "src"), outside)
	if got, _ := Select(s); !reflect.DeepEqual(got, []string{outside}) {
		t.Fatalf("without Outside the file is kept, got %v", got)
	}
	var reported []string
	s.Outside = func(p, r string) { reported = append(reported, p+" | "+r) }
	got, _ := Select(s)
	if got != nil || !reflect.DeepEqual(reported, []string{outside + " | " + filepath.Join(root, "src")}) {
		t.Fatalf("Select = %v, reported %v", got, reported)
	}
}

func TestJoinRootKeepsAbsolutePaths(t *testing.T) {
	if got := JoinRoot("/r", "a/b"); got != "/r/a/b" {
		t.Errorf("JoinRoot relative = %q", got)
	}
	if got := JoinRoot("/r", "/x/y"); got != "/x/y" {
		t.Errorf("JoinRoot absolute = %q", got)
	}
}
