package python_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// the checked-in corpus and the python fixture. It needs the upstream venv
// (tree-sitter 0.26.0) and is skipped when that is missing.
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
	paths := []string{filepath.Join(root, "ubgate", "testdata", "python")}
	if _, err := os.Stat(filepath.Join(root, "fixtures", "python")); err == nil {
		paths = append(paths, filepath.Join(root, "fixtures", "python"))
	}
	files, err := treediff.Collect(paths, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 5 {
		t.Fatalf("corpus too small: %v", files)
	}
	results, err := treediff.Run(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("%s diverges (line %d):\n%s", r.Path, r.Line, r.Context)
		}
	}
}
