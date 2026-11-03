package golang_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// the checked-in corpus (testdata/go/*.go.txt: not .go, so gofmt and the go
// tool leave the deliberately odd sources alone) and the go fixture. It
// needs the upstream venv (tree-sitter 0.26.0) and is skipped without it.
func TestParityWithTreeSitter(t *testing.T) {
	cwd, _ := os.Getwd()
	root, err := treediff.FindRoot(cwd)
	if err != nil {
		t.Skip("upstream venv not found:", err)
	}
	cfg := treediff.DefaultConfig(root)
	if _, err := os.Stat(cfg.Python); err != nil {
		t.Skip("upstream venv not found")
	}
	check := func(lang string, paths ...string) int {
		files, err := treediff.Collect(paths, lang)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Lang = lang
		results, err := treediff.Run(cfg, files)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range results {
			if !r.OK {
				t.Errorf("%s diverges (line %d): %v\n%s", r.Path, r.Line, r.Err, r.Context)
			}
		}
		return len(results)
	}
	if n := check("go", filepath.Join(root, "ubgate", "testdata", "go")); n < 10 {
		t.Fatalf("corpus too small: %d files", n)
	}
	if _, err := os.Stat(filepath.Join(root, "fixtures", "go")); err == nil {
		check("", filepath.Join(root, "fixtures", "go"))
	}
}
