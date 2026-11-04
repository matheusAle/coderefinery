package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOracleParity runs the upstream dryer oracle over the repo fixtures,
// the upstream dryer sources and internal/dry's construct corpus and
// requires identical candidates (spans, node counts, fingerprint counts,
// printed normalized forms) for every file whose language has a parser.
// Skipped when the upstream dryer venv is missing.
func TestOracleParity(t *testing.T) {
	cwd, _ := os.Getwd()
	root, err := FindRoot(cwd)
	if err != nil {
		t.Skip(err)
	}
	python := filepath.Join(root, "upstream", "dryer", ".venv", "bin", "python")
	if _, err := os.Stat(python); err != nil {
		t.Skip("upstream dryer venv missing")
	}
	files, err := Collect([]string{
		filepath.Join(root, "fixtures"),
		filepath.Join(root, "upstream", "dryer", "src"),
		filepath.Join(root, "internal", "dry", "testdata", "corpus"),
	})
	if err != nil {
		t.Fatal(err)
	}
	results, err := Run(python, filepath.Join(root, "tools", "dryoracle", "dryer_oracle.py"), files)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, r := range results {
		if r.Upstream != "" || r.ParseErr != nil && !Parsable(r.Path) {
			continue // the language's parser has not landed yet
		}
		checked++
		if !r.OK {
			t.Errorf("%s: %s", r.Path, r.Context)
		}
	}
	if checked < 20 {
		t.Fatalf("only %d files checked", checked)
	}
}
