package python_test

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// testdata/python and the python fixture.
// It needs the upstream venv and is skipped without it.
func TestParityWithTreeSitter(t *testing.T) {
	treediff.Parity(t, treediff.Corpus{Dirs: []string{"testdata/python"}, Optional: []string{"fixtures/python"}, Min: 5})
}
