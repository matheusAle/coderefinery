package rust_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusAle/coderefinery/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// the checked-in corpus (testdata/rust: a small workspace plus the upstream
// test snippets). It needs the upstream venv (tree-sitter 0.26.0) and is
// skipped when that is missing.
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
	files, err := treediff.Collect([]string{filepath.Join(root, "testdata", "rust")}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 10 {
		t.Fatalf("corpus too small: %v", files)
	}
	results, err := treediff.Run(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("%s diverges (line %d):\n%s%v", r.Path, r.Line, r.Context, r.Err)
		}
	}
}
