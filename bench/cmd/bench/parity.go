package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// testedAt is mutator's run timestamp in .metrics/mutate/*.edn; it is masked
// like harness/run.sh does (the only field that legitimately differs).
var testedAt = regexp.MustCompile(`:tested-at "[^"]*"`)

// Parity is one output comparison (one JSON line in parity.jsonl).
type Parity struct {
	Workload  string   `json:"workload"`
	Repo      string   `json:"repo"`
	Artifact  string   `json:"artifact"` // stdout, .metrics/crap.edn, ...
	Identical bool     `json:"identical"`
	LinesA    int      `json:"lines_upstream"`
	LinesB    int      `json:"lines_coderefinery"`
	Common    int      `json:"common_lines"` // multiset intersection
	MatchRate float64  `json:"match_rate"`   // common / max(lines)
	OnlyA     []string `json:"only_upstream,omitempty"`
	OnlyB     []string `json:"only_coderefinery,omitempty"`
	Missing   string   `json:"missing,omitempty"`
}

func parityMain(args []string) error {
	fs := flag.NewFlagSet("parity", flag.ExitOnError)
	var (
		workload = fs.String("workload", "", "workload label")
		repo     = fs.String("repo", "", "repo label")
		artifact = fs.String("artifact", "stdout", "artifact label")
		a        = fs.String("a", "", "upstream output file")
		b        = fs.String("b", "", "coderefinery output file")
		maskA    = fs.String("mask-a", "", "path prefix in -a replaced by <ROOT> (also its /private and symlink-resolved forms)")
		maskB    = fs.String("mask-b", "", "path prefix in -b replaced by <ROOT>")
		show     = fs.Int("show", 8, "keep at most this many differing lines per side")
		keepTime = fs.Bool("keep-tested-at", false, "do not mask :tested-at \"...\" values")
		out      = fs.String("out", "", "append a JSON line here (default stdout)")
	)
	fs.Parse(args)
	p := Parity{Workload: *workload, Repo: *repo, Artifact: *artifact}
	da, ea := os.ReadFile(*a)
	db, eb := os.ReadFile(*b)
	switch {
	case ea != nil && eb != nil:
		p.Missing = "both"
		p.Identical = true
		p.MatchRate = 1
	case ea != nil:
		p.Missing = "upstream"
	case eb != nil:
		p.Missing = "coderefinery"
	default:
		da, db = mask(da, *maskA), mask(db, *maskB)
		if !*keepTime {
			da = testedAt.ReplaceAll(da, []byte(`:tested-at "<T>"`))
			db = testedAt.ReplaceAll(db, []byte(`:tested-at "<T>"`))
		}
		p.Identical = bytes.Equal(da, db)
		la, lb := lines(da), lines(db)
		p.LinesA, p.LinesB = len(la), len(lb)
		bag := map[string]int{}
		for _, l := range lb {
			bag[l]++
		}
		for _, l := range la {
			if bag[l] > 0 {
				bag[l]--
				p.Common++
			} else if len(p.OnlyA) < *show {
				p.OnlyA = append(p.OnlyA, l)
			}
		}
		bag = map[string]int{}
		for _, l := range la {
			bag[l]++
		}
		for _, l := range lb {
			if bag[l] > 0 {
				bag[l]--
			} else if len(p.OnlyB) < *show {
				p.OnlyB = append(p.OnlyB, l)
			}
		}
		den := max(p.LinesA, p.LinesB)
		p.MatchRate = 1
		if den > 0 {
			p.MatchRate = float64(p.Common) / float64(den)
		}
	}
	w := os.Stdout
	if *out != "" {
		f, err := os.OpenFile(*out, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	if err := json.NewEncoder(w).Encode(p); err != nil {
		return err
	}
	status := "IDENTICAL"
	if !p.Identical {
		status = fmt.Sprintf("DIFFERENT (%.2f%% lines match)", 100*p.MatchRate)
	}
	if p.Missing != "" {
		status += " missing=" + p.Missing
	}
	fmt.Fprintf(os.Stderr, "  parity %-12s %-11s %-22s %s\n", p.Workload, p.Repo, p.Artifact, status)
	return nil
}

func lines(b []byte) []string {
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func mask(b []byte, root string) []byte {
	if root == "" {
		return b
	}
	forms := []string{root}
	if r, err := filepath.EvalSymlinks(root); err == nil && r != root {
		forms = append(forms, r)
	}
	for _, f := range append([]string(nil), forms...) {
		forms = append(forms, "/private"+f)
	}
	// longest first so /private/x is replaced before /x
	for i := len(forms) - 1; i >= 0; i-- {
		b = bytes.ReplaceAll(b, []byte(forms[i]), []byte("<ROOT>"))
	}
	return b
}
