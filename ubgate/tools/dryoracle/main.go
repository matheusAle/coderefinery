// Command dryoracle proves dryer parity per candidate: it runs upstream
// dryer's extraction + normalization (dryer_oracle.py, with the upstream
// dryer venv) and ubgate's internal/dry over the same files, and requires
// equal start line, end line, node count, fingerprint count and printed
// normalized form for every candidate, in order (spec-dryer Appendix B).
//
//	go run ./tools/dryoracle [-python PY] [-q] PATH...
//
// PATHs are files or directories (walked; .venv, node_modules, .git and
// __pycache__ skipped; every file dryer has a language for is kept, test
// files included). Exit status: 0 all equal, 1 any divergence, 2 usage.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/dry"
	"github.com/matheusAle/ub-mode/ubgate/internal/lang"
	"github.com/matheusAle/ub-mode/ubgate/internal/source"
)

// Record is one oracle line: a candidate, or a Clojure reader warning.
type Record struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Nodes   int    `json:"nodes"`
	FPs     int    `json:"fps"`
	Norm    string `json:"norm"`
	Warning string `json:"warning,omitempty"`
	Error   string `json:"error,omitempty"` // upstream crashed on the file
}

func (r Record) String() string {
	if r.Warning != "" {
		return "warning " + r.Warning
	}
	return fmt.Sprintf("%d-%d nodes=%d fps=%d %s", r.Start, r.End, r.Nodes, r.FPs, r.Norm)
}

// Result is the comparison for one file.
type Result struct {
	Path     string
	OK       bool
	Ours     int // candidates on our side
	Theirs   int
	Upstream string // upstream crashed on the file: not comparable
	ParseErr error  // our parser failed (or is not registered)
	Context  string
}

// Parsable reports whether path's language is Clojure or has a registered
// parser.
func Parsable(path string) bool {
	language := dry.LanguageOf(path)
	if language == "clojure" {
		return true
	}
	g := lang.GrammarFor(path)
	return g != "" && lang.Supported(g)
}

// FindRoot walks up from dir to the ub-mode folder.
func FindRoot(dir string) (string, error) {
	if env := os.Getenv("UBMODE"); env != "" {
		return env, nil
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "upstream", "dryer", ".venv", "bin", "python")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", errors.New("cannot find the ub-mode root (set UBMODE)")
		}
	}
}

// Collect expands paths into the files dryer has a language for, sorted.
func Collect(paths []string) ([]string, error) {
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
				case ".venv", "venv", "node_modules", ".git", "__pycache__", ".metrics":
					return filepath.SkipDir
				}
				return nil
			}
			if dry.LanguageOf(path) != "" {
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

// Oracle runs dryer_oracle.py over files in batches: path -> records.
func Oracle(python, script string, files []string) (map[string][]Record, error) {
	all := map[string][]Record{}
	for len(files) > 0 {
		n := min(len(files), 300)
		cmd := exec.Command(python, append([]string{"-I", "-B", script}, files[:n]...)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("oracle: %v: %s", err, stderr.String())
		}
		sc := bufio.NewScanner(bytes.NewReader(out))
		sc.Buffer(make([]byte, 1<<20), 1<<30)
		cur := ""
		for sc.Scan() {
			line := sc.Text()
			if name, ok := strings.CutPrefix(line, "=== FILE "); ok {
				cur = name
				all[cur] = []Record{}
				continue
			}
			var r Record
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				return nil, fmt.Errorf("oracle line %q: %v", line, err)
			}
			all[cur] = append(all[cur], r)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
		files = files[n:]
	}
	return all, nil
}

// Ours extracts the records of one file with internal/dry.
func Ours(path string) ([]Record, error) {
	text, err := source.Read(path)
	if err != nil {
		return nil, err
	}
	cands, warning, err := dry.Candidates(dry.LanguageOf(path), text, path)
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, c := range cands {
		out = append(out, Record{Start: c.StartLine, End: c.EndLine, Nodes: dry.NodeCount(c.Norm),
			FPs: len(dry.Fingerprints(c.Norm)), Norm: dry.Pr(c.Norm)})
	}
	if warning != "" {
		out = append(out, Record{Warning: warning})
	}
	return out, nil
}

// Compare reports the first differing record ("" when equal).
func Compare(ours, theirs []Record) string {
	for i := 0; i < max(len(ours), len(theirs)); i++ {
		switch {
		case i >= len(ours):
			return fmt.Sprintf("candidate %d missing\n  theirs: %s", i+1, theirs[i])
		case i >= len(theirs):
			return fmt.Sprintf("candidate %d extra\n  ours:   %s", i+1, ours[i])
		case ours[i] != theirs[i]:
			o, t := ours[i], theirs[i]
			at := firstDiff(o.Norm, t.Norm)
			return fmt.Sprintf("candidate %d differs at norm byte %d\n  ours:   %s\n  theirs: %s", i+1, at,
				excerpt(o, at), excerpt(t, at))
		}
	}
	return ""
}

// excerpt is the record header and 80 bytes of the normalized form either
// side of byte at.
func excerpt(r Record, at int) string {
	if r.Warning != "" {
		return r.String()
	}
	lo, hi := max(0, at-80), min(len(r.Norm), at+80)
	return fmt.Sprintf("%d-%d nodes=%d fps=%d ...%s...", r.Start, r.End, r.Nodes, r.FPs, r.Norm[lo:hi])
}

func firstDiff(a, b string) int {
	for i := 0; i < min(len(a), len(b)); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

// Run compares every file.
func Run(python, script string, files []string) ([]Result, error) {
	theirs, err := Oracle(python, script, files)
	if err != nil {
		return nil, err
	}
	var results []Result
	for _, f := range files {
		r := Result{Path: f, Theirs: len(theirs[f])}
		if t := theirs[f]; len(t) == 1 && t[0].Error != "" {
			r.Upstream, r.Theirs = t[0].Error, 0
			results = append(results, r)
			continue
		}
		ours, err := Ours(f)
		if err != nil {
			r.ParseErr = err
			r.Context = "parse error: " + err.Error()
		} else {
			r.Ours = len(ours)
			r.Context = Compare(ours, theirs[f])
			r.OK = r.Context == ""
		}
		results = append(results, r)
	}
	return results, nil
}

func main() {
	cwd, _ := os.Getwd()
	root, _ := FindRoot(cwd)
	python := flag.String("python", filepath.Join(root, "upstream", "dryer", ".venv", "bin", "python"), "upstream dryer venv python")
	script := flag.String("oracle", filepath.Join(root, "ubgate", "tools", "dryoracle", "dryer_oracle.py"), "dryer_oracle.py")
	quiet := flag.Bool("q", false, "only print failures and the summary")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: dryoracle [-python PY] [-oracle SCRIPT] [-q] PATH...")
		os.Exit(2)
	}
	files, err := Collect(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	results, err := Run(*python, *script, files)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	failed, crashed, cands, candsOK := 0, 0, 0, 0
	for _, r := range results {
		if r.Upstream != "" {
			crashed++
			fmt.Printf("UPSTREAM-CRASH %s: %s\n", r.Path, r.Upstream)
			continue
		}
		cands += r.Theirs
		if r.OK {
			candsOK += r.Theirs
			if !*quiet {
				fmt.Printf("PASS %s (%d)\n", r.Path, r.Theirs)
			}
			continue
		}
		failed++
		fmt.Printf("FAIL %s (ours %d, theirs %d): %s\n", r.Path, r.Ours, r.Theirs, r.Context)
	}
	fmt.Printf("SUMMARY files=%d identical=%d failed=%d upstream-crashed=%d candidates-in-identical-files=%d/%d\n",
		len(results), len(results)-failed-crashed, failed, crashed, candsOK, cands)
	if failed > 0 {
		os.Exit(1)
	}
}
