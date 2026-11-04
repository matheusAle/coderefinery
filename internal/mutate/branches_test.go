package mutate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBranchAnnotatorExplainsUncoveredSites(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for dst, src := range map[string]string{
		"billing/pricing.py":                           "../crap/testdata/branches_python/billing/pricing.py",
		".metrics/tests/python/coverage/coverage.json": "../coverage/testdata/branches/coverage.json",
	} {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.MkdirAll(filepath.Dir(filepath.Join(root, dst)), 0o755)
		if err := os.WriteFile(filepath.Join(root, dst), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result := RunResult{Path: "billing/pricing.py"}
	annotate := branchAnnotator(root, result, lineSet{5: true, 16: true})
	for _, c := range []struct {
		line   int
		status string
		want   string
	}{
		{17, "uncovered", " · in the true arm of :16 (if total <= 0), never taken"},
		{22, "uncovered", " · in the body arm of :21 (for item in items), never taken"},
		{31, "uncovered", " · in the case 2 arm of :30 (case \"CA\" | \"MX\"), never taken"},
		{38, "uncovered", " · line never executed"},
		{17, "killed", ""},
		{17, "survived", ""},
	} {
		if got := annotate(Site{Line: c.line}, c.status); got != c.want {
			t.Errorf("line %d %s: %q, want %q", c.line, c.status, got, c.want)
		}
	}
	if got := branchAnnotator(root, result, nil)(Site{Line: 17}, "uncovered"); got != " · no coverage data for file" {
		t.Errorf("no coverage data: %q", got)
	}
	// a file without branch data still says why the line is uncovered
	other := branchAnnotator(root, RunResult{Path: "billing/other.py"}, lineSet{})
	if got := other(Site{Line: 3}, "uncovered"); got != " · line never executed" {
		t.Errorf("no branch data: %q", got)
	}
}
