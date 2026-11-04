// Package testreport is `coderefinery tests` (docs/design-tests.md §4): the
// kill matrix from `mutate --attribute`, unique kills, a greedy covering set,
// shadowed and zero-kill tests, test levels, .metrics/tests.edn.
package testreport

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/mutate"
	"github.com/matheusAle/coderefinery/internal/testresult"
	"github.com/matheusAle/coderefinery/internal/testrun"
)

// Mutant is one recorded mutant, decoded from its mutation id.
type Mutant struct {
	ID, File, Namespace, Form, Original, Mutant string
	Start, End                                  int
	Killed                                      bool
	KilledBy                                    []string // real test IDs only, sorted
	Sentinel                                    string   // "", "<build>", "<timeout>"
}

// Matrix is the kill matrix.
type Matrix struct {
	Tests        []testresult.Case // results ∪ killers; sorted by ID
	Mutants      []Mutant          // sorted by ID
	Kills        map[string][]int  // test ID -> ascending indices into Mutants
	Unattributed map[string]int    // sentinel -> count
	Attributed   bool              // some outcome has :killed-by
	Failing      []string          // failing (non-flaky) cases of the latest results
	BuildFailed  bool              // the latest results had a build/collection failure
	Flaky        map[string]bool
	Runs         map[string]map[string]bool // test ID -> functions executed (contexts); nil when unknown
	Stale        []string                   // killer ids missing from the results (dropped)
	caseCounts   map[string]int             // ByBase: cases per group row
}

func decodeMutant(o mutate.SnapshotOutcome) Mutant {
	m := Mutant{ID: o.ID, Killed: o.Killed}
	var parts []any
	if json.Unmarshal([]byte(o.ID), &parts) == nil && len(parts) == 7 {
		str := func(i int) string { s, _ := parts[i].(string); return s }
		num := func(i int) int { f, _ := parts[i].(float64); return int(f) }
		m.File, m.Namespace, m.Form, m.Start, m.End, m.Original, m.Mutant = str(0), str(1), str(2), num(3), num(4), str(5), str(6)
	}
	return m
}

func isSentinel(id string) bool { return id == "<build>" || id == "<timeout>" }

// Load builds the matrix from .metrics/mutate and .metrics/tests.
func Load(root string, stderr io.Writer) (*Matrix, error) {
	m := &Matrix{Kills: map[string][]int{}, Unattributed: map[string]int{}, Flaky: map[string]bool{}}
	results, err := loadResults(root, m)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, c := range results {
		known[c.ID] = true
	}
	outcomes := mutate.SnapshotOutcomes(root)
	unseen := map[string]bool{}
	for _, o := range outcomes {
		mu := decodeMutant(o)
		if o.KilledBy != nil {
			m.Attributed = true
		}
		if o.Killed {
			for _, id := range o.KilledBy {
				switch {
				case isSentinel(id):
					if mu.Sentinel == "" {
						mu.Sentinel = id
					}
				case len(results) > 0 && !known[id]:
					unseen[id] = true
				default:
					mu.KilledBy = append(mu.KilledBy, id)
				}
			}
			if len(mu.KilledBy) == 0 && mu.Sentinel != "" {
				m.Unattributed[mu.Sentinel]++
			}
		}
		m.Mutants = append(m.Mutants, mu)
	}
	for id := range unseen {
		m.Stale = append(m.Stale, id)
	}
	sort.Strings(m.Stale)
	for _, id := range m.Stale {
		fmt.Fprintf(stderr, "stale test id %s\n", id)
	}
	tests := map[string]testresult.Case{}
	for _, c := range results {
		if !c.Parent {
			tests[c.ID] = c
		}
	}
	for i, mu := range m.Mutants {
		for _, id := range mu.KilledBy {
			m.Kills[id] = append(m.Kills[id], i)
			if _, ok := tests[id]; !ok {
				tests[id] = synthesize(id)
			}
		}
	}
	for _, c := range tests {
		m.Tests = append(m.Tests, c)
	}
	sort.Slice(m.Tests, func(i, j int) bool { return m.Tests[i].ID < m.Tests[j].ID })
	return m, nil
}

// synthesize is a Case for a killer that has no results (no `test` run):
// pytest-style ids "file::...::name[param]", Go "pkg::TestX/sub".
func synthesize(id string) testresult.Case {
	c := testresult.Case{ID: id, Base: id, Status: testresult.Pass, Duration: -1} // -1: no measured duration
	parts := strings.Split(id, "::")
	c.Locator = parts[0]
	if discover.LanguageOf(parts[0]) != "" {
		c.File = parts[0] // a source-file locator (not a Go import path, FQCN or nextest binary)
	}
	c.Name = parts[len(parts)-1]
	if len(parts) > 2 {
		c.Suite = parts[1 : len(parts)-1]
	}
	switch {
	case strings.HasSuffix(c.Name, "]") && strings.Contains(c.Name, "["):
		c.Base = strings.TrimSuffix(id, c.Name) + c.Name[:strings.Index(c.Name, "[")]
	case strings.HasPrefix(c.Name, "Test") && strings.Contains(c.Name, "/"):
		c.Base = strings.TrimSuffix(id, c.Name) + c.Name[:strings.Index(c.Name, "/")]
	}
	return c
}

// loadResults reads every module's latest results (runs.edn), records
// failures, flaky ids and python contexts.
func loadResults(root string, m *Matrix) ([]testresult.Case, error) {
	runs, err := testrun.ReadRuns(root)
	if err != nil {
		return nil, err
	}
	var cases []testresult.Case
	for _, r := range runs {
		for _, id := range r.Flaky {
			m.Flaky[id] = true
		}
		if r.Results == "" {
			m.BuildFailed = true
			continue
		}
		run, err := testrun.LoadModuleResults(root, r.Results, r.Module, r.Tool)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", r.Results, err)
		}
		if run.BuildFailed {
			m.BuildFailed = true
		}
		for _, id := range run.Failing() {
			if !m.Flaky[id] {
				m.Failing = append(m.Failing, id)
			}
		}
		cases = append(cases, run.Cases...)
		if r.Contexts && r.Coverage != "" {
			readContexts(filepath.Join(root, filepath.FromSlash(r.Coverage)), r.Module, m)
		}
	}
	sort.Strings(m.Failing)
	return cases, nil
}

// readContexts records, per test id, the functions coverage.py saw it run.
func readContexts(report, module string, m *Matrix) {
	raw, err := os.ReadFile(report)
	if err != nil {
		return
	}
	var doc struct {
		Files map[string]struct {
			Contexts  map[string][]string `json:"contexts"`
			Functions map[string]struct {
				Executed []int `json:"executed_lines"`
			} `json:"functions"`
		} `json:"files"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return
	}
	if m.Runs == nil {
		m.Runs = map[string]map[string]bool{}
	}
	prefix := ""
	if module != "." && module != "" {
		prefix = module + "/"
	}
	for file, f := range doc.Files {
		lineFn := map[int]string{}
		for name, fn := range f.Functions {
			if name == "" {
				continue
			}
			for _, l := range fn.Executed {
				lineFn[l] = name
			}
		}
		for line, labels := range f.Contexts {
			n, err := strconv.Atoi(line)
			if err != nil {
				continue
			}
			fn, ok := lineFn[n]
			if !ok {
				continue
			}
			for _, label := range labels {
				id := mutate.ContextNodeID(label)
				if id == "" {
					continue
				}
				id = prefix + id
				if m.Runs[id] == nil {
					m.Runs[id] = map[string]bool{}
				}
				m.Runs[id][prefix+file+"::"+fn] = true
			}
		}
	}
}

// ByBase collapses cases onto their Base (kills = union, durations summed).
func (m *Matrix) ByBase() *Matrix {
	out := *m
	out.Kills = map[string][]int{}
	out.Tests = nil
	out.Flaky = map[string]bool{}
	out.Runs = nil
	if m.Runs != nil {
		out.Runs = map[string]map[string]bool{}
	}
	group := map[string]*testresult.Case{}
	var order []string
	counts := map[string]int{}
	for _, c := range m.Tests {
		base := c.Base
		if base == "" {
			base = c.ID
		}
		g := group[base]
		if g == nil {
			cp := c
			cp.ID, cp.Name = base, strings.TrimPrefix(base, strings.TrimSuffix(c.ID, c.Name))
			cp.Duration = 0
			group[base] = &cp
			g = &cp
			order = append(order, base)
		}
		if c.Duration < 0 || g.Duration < 0 {
			g.Duration = -1
		} else {
			g.Duration += c.Duration
		}
		counts[base]++
		if m.Flaky[c.ID] {
			out.Flaky[base] = true
		}
		set := map[int]bool{}
		for _, i := range out.Kills[base] {
			set[i] = true
		}
		for _, i := range m.Kills[c.ID] {
			set[i] = true
		}
		out.Kills[base] = sortedInts(set)
		if m.Runs != nil {
			if fns := m.Runs[c.ID]; fns != nil {
				if out.Runs[base] == nil {
					out.Runs[base] = map[string]bool{}
				}
				for f := range fns {
					out.Runs[base][f] = true
				}
			}
		}
	}
	sort.Strings(order)
	for _, b := range order {
		out.Tests = append(out.Tests, *group[b])
	}
	out.caseCounts = counts
	return &out
}

func sortedInts(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for i := range set {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}
