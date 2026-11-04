package golang_test

import (
	"testing"

	"github.com/matheusAle/coderefinery/internal/treediff"
)

// TestParityWithTreeSitter diffs our trees against upstream tree-sitter for
// testdata/go (*.go.txt: not .go, so gofmt and the go tool leave the
// deliberately odd sources alone) and the go fixture.
// It needs the upstream venv and is skipped without it.
func TestParityWithTreeSitter(t *testing.T) {
	treediff.Parity(t, treediff.Corpus{Lang: "go", Dirs: []string{"testdata/go"}, Min: 10},
		treediff.Corpus{Optional: []string{"fixtures/go"}})
}
