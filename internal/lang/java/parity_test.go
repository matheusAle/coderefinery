package java_test

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// testdata/java (upstream test snippets, spec cases and a small project).
// It needs the upstream venv and is skipped without it.
func TestParityWithTreeSitter(t *testing.T) {
	treediff.Parity(t, treediff.Corpus{Dirs: []string{"testdata/java"}, Min: 30})
}
