// Package testresult is the normalised test-result model of
// docs/design-tests.md §1: JUnit XML (pytest, Vitest, jest-junit,
// mocha-junit-reporter, Surefire, Gradle, nextest, Kaocha, test2junit) and
// `go test -json` become one Run of Cases with canonical, tool-stable IDs.
package testresult

import (
	"path"
	"sort"
	"strings"
)

// Status is a test case outcome.
type Status string

const (
	Pass  Status = "pass"
	Fail  Status = "fail"
	Skip  Status = "skip"
	Error Status = "error"
)

// rank orders statuses when duplicate IDs merge: error > fail > pass > skip.
func (s Status) rank() int {
	switch s {
	case Error:
		return 3
	case Fail:
		return 2
	case Pass:
		return 1
	}
	return 0
}

// Case is one test case.
type Case struct {
	ID       string   // canonical id (§1.1), the kill-matrix join key
	Locator  string   // repo-relative POSIX test file; else the tool's container (Go import path, nextest binary id, FQCN)
	File     string   // repo-relative POSIX test file, "" when unresolved
	Suite    []string // class / describe path, outermost first
	Name     string   // leaf name incl. parameters
	Base     string   // ID without the trailing parameter / subtest part (== ID when none)
	Parent   bool     // Go: a test with subtests; never credited with a kill
	Status   Status
	Duration float64 // seconds
	Failure  string  // failure message + text, trimmed, at most 4 KiB
}

// Run is one test run.
type Run struct {
	Tool        string // pytest|vitest|jest|mocha|surefire|gradle|nextest|kaocha|test2junit|go|junit
	Cases       []Case // sorted by ID, IDs unique
	BuildFailed bool   // a collection / compile / package failure not tied to a case
}

const maxFailure = 4096

func capFailure(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxFailure {
		return s
	}
	cut := maxFailure
	for cut > 0 && s[cut]&0xC0 == 0x80 { // keep UTF-8 whole
		cut--
	}
	return s[:cut]
}

// MakeID is §1.1: Locator::Suite...::Name with empty parts dropped.
func MakeID(locator string, suite []string, name string) string {
	parts := make([]string, 0, len(suite)+2)
	for _, p := range append(append([]string{locator}, suite...), name) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "::")
}

// finish fills ID/Base from the parts; baseName is Name with the tool's
// parameter or subtest part stripped.
func (c *Case) finish(baseName string) {
	if len(c.Suite) == 0 {
		c.Suite = nil
	}
	c.ID = MakeID(c.Locator, c.Suite, c.Name)
	c.Base = MakeID(c.Locator, c.Suite, baseName)
	c.Failure = capFailure(c.Failure)
}

// normalize sorts by ID and merges duplicates (error>fail>pass>skip,
// durations summed, failures joined).
func (r *Run) normalize() {
	byID := map[string]int{}
	var out []Case
	for _, c := range r.Cases {
		i, ok := byID[c.ID]
		if !ok {
			byID[c.ID] = len(out)
			out = append(out, c)
			continue
		}
		prev := &out[i]
		prev.Duration += c.Duration
		if c.Status.rank() > prev.Status.rank() {
			prev.Status = c.Status
		}
		if c.Failure != "" && !strings.Contains(prev.Failure, c.Failure) {
			prev.Failure = capFailure(strings.TrimSpace(prev.Failure + "\n" + c.Failure))
		}
		prev.Parent = prev.Parent || c.Parent
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	r.Cases = out
}

// Merge combines runs (e.g. several modules or Surefire files). The tool is
// the first non-empty one, "mixed" when they differ.
func Merge(runs ...*Run) *Run {
	out := &Run{}
	for _, r := range runs {
		if r == nil {
			continue
		}
		switch {
		case out.Tool == "":
			out.Tool = r.Tool
		case r.Tool != "" && r.Tool != out.Tool:
			out.Tool = "mixed"
		}
		out.Cases = append(out.Cases, r.Cases...)
		out.BuildFailed = out.BuildFailed || r.BuildFailed
	}
	out.normalize()
	return out
}

// Failing is the IDs with fail|error that are not Go parents, sorted.
func (r *Run) Failing() []string {
	var out []string
	for _, c := range r.Cases {
		if (c.Status == Fail || c.Status == Error) && !c.Parent {
			out = append(out, c.ID)
		}
	}
	return out
}

// Counts is pass, fail, skip, error over non-parent cases.
func (r *Run) Counts() (pass, fail, skip, errs int) {
	for _, c := range r.Cases {
		if c.Parent {
			continue
		}
		switch c.Status {
		case Pass:
			pass++
		case Fail:
			fail++
		case Skip:
			skip++
		case Error:
			errs++
		}
	}
	return
}

// Rebase prefixes the file-based paths (File, and Locator when it is the
// file) with prefix, the module dir relative to the repo root, so IDs are
// repo-relative. "" or "." changes nothing.
func (r *Run) Rebase(prefix string) {
	prefix = strings.Trim(path.Clean("/"+prefix), "/")
	if prefix == "" {
		return
	}
	for i := range r.Cases {
		c := &r.Cases[i]
		if c.File == "" || strings.HasPrefix(c.File, "/") {
			continue
		}
		oldID, oldBase := c.ID, c.Base
		file := prefix + "/" + c.File
		if c.Locator == c.File {
			c.Locator = file
			c.ID = file + strings.TrimPrefix(oldID, c.File)
			c.Base = file + strings.TrimPrefix(oldBase, c.File)
		}
		c.File = file
	}
	r.normalize()
}
