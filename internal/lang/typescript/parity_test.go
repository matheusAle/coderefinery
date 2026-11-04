package typescript_test

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// testdata/typescript and the typescript and javascript fixtures.
// It needs the upstream venv and is skipped without it.
func TestParityWithTreeSitter(t *testing.T) {
	treediff.Parity(t, treediff.Corpus{
		Dirs:     []string{"testdata/typescript"},
		Optional: []string{"fixtures/typescript", "fixtures/javascript"},
		Min:      15,
	})
}
