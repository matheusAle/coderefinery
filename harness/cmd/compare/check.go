package main

import (
	"bytes"
	"fmt"
	"strings"

	"ubmode/harness/internal/edn"
	"ubmode/harness/internal/textdiff"
)

// Settings shared by every comparison.
type Settings struct {
	Norm         Normaliser
	EDN          edn.Options
	StrictStderr bool
	DiffLines    int
}

// FileResult compares one .metrics file.
type FileResult struct {
	Name     string
	Missing  bool // golden/upstream has it, ours does not
	Extra    bool // ours has it, golden/upstream does not
	Semantic bool
	Raw      bool
	Diffs    []edn.Diff
	ParseErr string
	TextDiff string
}

// Result is the outcome of one case.
type Result struct {
	Case       Case
	Err        string // harness or "ours" could not run
	WantExit   int
	GotExit    int
	StdoutOK   bool
	StderrOK   bool
	StdoutDiff string
	StderrDiff string
	Files      []FileResult
	Duration   string
}

func (r Result) ExitOK() bool { return r.WantExit == r.GotExit }

func (r Result) MetricsOK() bool {
	for _, f := range r.Files {
		if f.Missing || f.Extra || !f.Semantic {
			return false
		}
	}
	return true
}

func (r Result) Pass(strictStderr bool) bool {
	return r.Err == "" && r.ExitOK() && r.StdoutOK && r.MetricsOK() && (r.StderrOK || !strictStderr)
}

// compareRuns judges `got` (ours) against `want` (golden or live upstream).
func compareRuns(c Case, want, got Run, s Settings) Result {
	res := Result{Case: c, WantExit: want.Exit, GotExit: got.Exit, Duration: got.Duration.Round(1e6).String()}
	if want.Err != "" {
		res.Err = "upstream: " + want.Err
	}
	if got.Err != "" {
		if res.Err != "" {
			res.Err += "; "
		}
		res.Err += "ours: " + got.Err
	}
	wo, go_ := s.Norm.Text(want.Stdout, want.Root), s.Norm.Text(got.Stdout, got.Root)
	res.StdoutOK = wo == go_
	res.StdoutDiff = textdiff.Unified("want/stdout", "got/stdout", wo, go_, 3, s.DiffLines)
	we, ge := s.Norm.Text(want.Stderr, want.Root), s.Norm.Text(got.Stderr, got.Root)
	res.StderrOK = we == ge
	res.StderrDiff = textdiff.Unified("want/stderr", "got/stderr", we, ge, 3, s.DiffLines)

	names := map[string]bool{}
	for n := range want.Metrics {
		names[n] = true
	}
	for n := range got.Metrics {
		names[n] = true
	}
	for _, name := range sortedSet(names) {
		wb, inWant := want.Metrics[name]
		gb, inGot := got.Metrics[name]
		fr := FileResult{Name: name, Missing: inWant && !inGot, Extra: inGot && !inWant}
		if inWant && inGot {
			fr.Raw = bytes.Equal(s.Norm.Raw(wb), s.Norm.Raw(gb))
			compareEDN(&fr, wb, gb, s)
		}
		res.Files = append(res.Files, fr)
	}
	return res
}

func compareEDN(fr *FileResult, wb, gb []byte, s Settings) {
	if !strings.HasSuffix(fr.Name, ".edn") {
		fr.Semantic = fr.Raw
		if !fr.Raw {
			fr.TextDiff = textdiff.Unified("want/"+fr.Name, "got/"+fr.Name, string(wb), string(gb), 3, s.DiffLines)
		}
		return
	}
	wv, werr := edn.Parse(string(wb))
	gv, gerr := edn.Parse(string(gb))
	if werr != nil || gerr != nil {
		var msgs []string
		if werr != nil {
			msgs = append(msgs, "want: "+werr.Error())
		}
		if gerr != nil {
			msgs = append(msgs, "got: "+gerr.Error())
		}
		fr.ParseErr = strings.Join(msgs, "; ")
		fr.Semantic = false
		fr.TextDiff = textdiff.Unified("want/"+fr.Name, "got/"+fr.Name, string(wb), string(gb), 3, s.DiffLines)
		return
	}
	fr.Diffs = edn.Compare(wv, gv, s.EDN)
	fr.Semantic = len(fr.Diffs) == 0
}

func sortedSet(m map[string]bool) []string {
	b := make(map[string][]byte, len(m))
	for k := range m {
		b[k] = nil
	}
	return sortedNames(b)
}

func (fr FileResult) describe() string {
	switch {
	case fr.Missing:
		return fmt.Sprintf("  %s: missing in ours", fr.Name)
	case fr.Extra:
		return fmt.Sprintf("  %s: extra in ours (not in golden/upstream)", fr.Name)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %s: semantic=%s raw=%s\n", fr.Name, okWord(fr.Semantic), eqWord(fr.Raw))
	if fr.ParseErr != "" {
		fmt.Fprintf(&b, "    parse error: %s\n", fr.ParseErr)
	}
	for _, d := range fr.Diffs {
		fmt.Fprintf(&b, "    %s\n", d)
	}
	if fr.TextDiff != "" {
		b.WriteString(indent(fr.TextDiff, "    "))
	}
	return strings.TrimRight(b.String(), "\n")
}

func okWord(ok bool) string {
	if ok {
		return "ok"
	}
	return "DIFF"
}

func eqWord(ok bool) string {
	if ok {
		return "identical"
	}
	return "different"
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}
