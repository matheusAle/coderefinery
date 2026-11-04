package rust_test

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// testdata/rust (a small workspace plus the upstream test snippets).
// It needs the upstream venv and is skipped without it.
func TestParityWithTreeSitter(t *testing.T) {
	treediff.Parity(t, treediff.Corpus{Dirs: []string{"testdata/rust"}, Min: 10})
}
