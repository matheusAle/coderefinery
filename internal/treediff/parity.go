package treediff

import (
	"os"
	"path/filepath"
)

// T is the part of *testing.T that Parity uses (this package stays free of
// the testing import, since tools/treediff links it).
type T interface {
	Helper()
	Skip(args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Corpus is one set of files a parser's parity test compares.
type Corpus struct {
	Lang     string   // grammar forced on every file; "" = by extension
	Dirs     []string // below the repo root, required
	Optional []string // below the repo root, compared when present (fixtures/)
	Min      int      // fail when fewer files are found
}

// Parity is every parser's TestParityWithTreeSitter: it diffs our trees
// against upstream tree-sitter for each corpus and reports each diverging
// file. It needs the upstream venv (tree-sitter 0.26.0) and skips without it.
func Parity(t T, corpora ...Corpus) {
	t.Helper()
	cwd, _ := os.Getwd()
	root, err := FindRoot(cwd)
	if err != nil {
		t.Skip("upstream venv not found:", err)
	}
	cfg := DefaultConfig(root)
	if _, err := os.Stat(cfg.Python); err != nil {
		t.Skip("upstream venv not found")
	}
	for _, c := range corpora {
		var paths []string
		for _, d := range c.Dirs {
			paths = append(paths, filepath.Join(root, d))
		}
		for _, d := range c.Optional {
			if _, err := os.Stat(filepath.Join(root, d)); err == nil {
				paths = append(paths, filepath.Join(root, d))
			}
		}
		files, err := Collect(paths, c.Lang)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) < c.Min {
			t.Fatalf("corpus too small: %d files in %v", len(files), paths)
		}
		if len(files) == 0 {
			continue
		}
		cfg.Lang = c.Lang
		results, err := Run(cfg, files)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range results {
			if !r.OK {
				t.Errorf("%s diverges (line %d): %v\n%s", r.Path, r.Line, r.Err, r.Context)
			}
		}
	}
}
