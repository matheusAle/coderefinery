package java_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// the checked-in Java corpus (testdata/java: upstream test snippets, spec
// cases and a small project). It needs the upstream venv (tree-sitter
// 0.26.0) and is skipped when that is missing.
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
	files, err := treediff.Collect([]string{filepath.Join(root, "ubgate", "testdata", "java")}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 30 {
		t.Fatalf("corpus too small: %d files", len(files))
	}
	results, err := treediff.Run(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("%s diverges (line %d): %v\n%s", r.Path, r.Line, r.Err, r.Context)
		}
	}
}
