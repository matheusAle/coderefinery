// Package treediff compares ubgate's parsers with upstream tree-sitter.
// It runs tools/oracle/oracle.py (with an upstream venv interpreter) over a
// set of files and diffs its dump against syntax.Dump of our parse, line by
// line, reporting the first divergence per file.
package treediff

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/lang"
	"github.com/matheusAle/ub-mode/ubgate/internal/source"
	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// Config locates the oracle.
type Config struct {
	Python string // upstream venv interpreter (tree-sitter 0.26.0 + language-pack 1.23.1)
	Oracle string // path to tools/oracle/oracle.py
	Lang   string // force a grammar; "" = by extension
}

// Result is the outcome for one file.
type Result struct {
	Path    string
	OK      bool
	Err     error  // our parser failed
	Line    int    // 1-based first differing dump line
	Context string // human-readable divergence report
}

// FindRoot walks up from dir to the ub-mode folder (the one holding
// upstream/crapper/.venv and ubgate/).
func FindRoot(dir string) (string, error) {
	if env := os.Getenv("UBMODE"); env != "" {
		return env, nil
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "upstream", "crapper", ".venv", "bin", "python")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", errors.New("cannot find the ub-mode root (set UBMODE)")
		}
	}
}

// DefaultConfig derives interpreter and oracle paths from the ub-mode root.
func DefaultConfig(root string) Config {
	return Config{
		Python: filepath.Join(root, "upstream", "crapper", ".venv", "bin", "python"),
		Oracle: filepath.Join(root, "ubgate", "tools", "oracle", "oracle.py"),
	}
}

// Collect expands paths (files or directories) into the files that have a
// grammar with a registered parser (or every regular file when lang is
// forced), sorted. Directories named .venv, node_modules, .git are skipped.
func Collect(paths []string, forced string) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch d.Name() {
				case ".venv", "venv", "node_modules", ".git", "__pycache__":
					return filepath.SkipDir
				}
				return nil
			}
			g := forced
			if g == "" {
				g = lang.GrammarFor(path)
			}
			if g != "" && lang.Supported(g) {
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// Oracle runs the upstream dumper over files (in batches, to stay under the
// argv limit) and returns path -> dump.
func Oracle(cfg Config, files []string) (map[string]string, error) {
	dumps := map[string]string{}
	for len(files) > 0 {
		n := min(len(files), 400)
		part, err := oracleBatch(cfg, files[:n])
		if err != nil {
			return nil, err
		}
		for k, v := range part {
			dumps[k] = v
		}
		files = files[n:]
	}
	return dumps, nil
}

func oracleBatch(cfg Config, files []string) (map[string]string, error) {
	args := []string{"-I", cfg.Oracle, "--headers"}
	if cfg.Lang != "" {
		args = append(args, "--lang", cfg.Lang)
	}
	args = append(args, files...)
	cmd := exec.Command(cfg.Python, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("oracle: %v: %s", err, stderr.String())
	}
	dumps := map[string]string{}
	var cur string
	var b strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<30)
	flush := func() {
		if cur != "" {
			dumps[cur] = b.String()
		}
		b.Reset()
	}
	for sc.Scan() {
		line := sc.Text()
		if name, ok := strings.CutPrefix(line, "=== FILE "); ok {
			flush()
			cur = name
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	flush()
	return dumps, sc.Err()
}

// Ours parses one file the way the tools do and returns its dump.
func Ours(path, forced string) (string, error) {
	text, err := source.Read(path)
	if err != nil {
		return "", err
	}
	g := forced
	if g == "" {
		g = lang.GrammarFor(path)
	}
	tree, err := lang.ParseText(g, text)
	if err != nil {
		return "", err
	}
	return syntax.Dump(tree.Root, tree.Source), nil
}

// Run compares every file and returns one Result per file.
func Run(cfg Config, files []string) ([]Result, error) {
	theirs, err := Oracle(cfg, files)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, f := range files {
		r := Result{Path: f}
		ours, err := Ours(f, cfg.Lang)
		if err != nil {
			r.Err = err
			r.Context = "parse error: " + err.Error()
			results = append(results, r)
			continue
		}
		r.Line, r.Context = Compare(ours, theirs[f])
		r.OK = r.Line == 0
		results = append(results, r)
	}
	return results, nil
}

// Compare returns 0 when the dumps are equal, else the 1-based first
// differing line and a report with a few lines of shared context.
func Compare(ours, theirs string) (int, string) {
	if ours == theirs {
		return 0, ""
	}
	a := strings.Split(ours, "\n")
	b := strings.Split(theirs, "\n")
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	var r strings.Builder
	for j := max(0, i-4); j < i; j++ {
		fmt.Fprintf(&r, "    %s\n", a[j])
	}
	line := func(s []string, k int) string {
		if k < len(s) {
			return s[k]
		}
		return "<end>"
	}
	fmt.Fprintf(&r, "  ours:   %s\n  theirs: %s\n", line(a, i), line(b, i))
	return i + 1, r.String()
}
