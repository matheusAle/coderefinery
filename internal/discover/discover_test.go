package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLanguageOfAndTestFiles(t *testing.T) {
	langs := map[string]string{"a.clj": "clojure", "a.bb": "clojure", "A.java": "java", "a.go": "go", "a.tsx": "typescript",
		"a.cjs": "typescript", "a.rs": "rust", "a.py": "python", "types.d.ts": "", "x.d.mts": "typescript", ".py": "", "a.PY": "", "README": ""}
	for in, want := range langs {
		if got := LanguageOf(in); got != want {
			t.Errorf("LanguageOf(%q) = %q, want %q", in, got, want)
		}
	}
	tests := map[string]bool{"conftest.py": true, "src/app.test.js": true, "src/app.spec.mjs": true, "tests/test_board.py": true,
		"app.test.cts": true, "x_test.go": true, "a_test.py": true, "/home/u/spec/proj/a.py": true, "src/app.py": false, "src/testing.py": false}
	for in, want := range tests {
		if got := IsTestFile(in); got != want {
			t.Errorf("IsTestFile(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIterSourceFilesSkipsTestAndBuildDirs(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	for _, rel := range []string{"src/app.py", "tests/hidden.py", "target/gen.py", "src/notes.md", "src/x_test.py"} {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, nil, 0o644)
	}
	if got := IterSourceFiles([]string{root}); !reflect.DeepEqual(got, []string{filepath.Join(root, "src", "app.py")}) {
		t.Fatalf("got %v", got)
	}
	if got := IterSourceFiles([]string{filepath.Join(root, "missing")}); got != nil {
		t.Fatalf("missing root: %v", got)
	}
	file := filepath.Join(root, "src", "notes.md")
	if got := IterSourceFiles([]string{file}); !reflect.DeepEqual(got, []string{file}) {
		t.Fatalf("a file root is returned unchecked: %v", got)
	}
}

func TestSuffixAndParts(t *testing.T) {
	if Suffix("a/b.tar.gz") != ".gz" || Suffix(".py") != "" || Suffix("x.") != "" {
		t.Error("suffix")
	}
	if !reflect.DeepEqual(Parts("/a/b/c.py"), []string{"/", "a", "b", "c.py"}) {
		t.Error("parts")
	}
}

func TestRelativeOrSelfFallsBackToThePath(t *testing.T) {
	cases := map[[2]string]string{{"/r/a/b.go", "/r"}: "a/b.go", {"/r", "/r"}: ".", {"/rx/b.go", "/r"}: "/rx/b.go"}
	for in, want := range cases {
		if got := RelativeOrSelf(in[0], in[1]); got != want {
			t.Errorf("RelativeOrSelf(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
