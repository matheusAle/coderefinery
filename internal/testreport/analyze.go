package testreport

import (
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Level is a test level.
type Level string

const (
	Unit        Level = "unit"
	Integration Level = "integration"
	API         Level = "api"
	E2E         Level = "e2e"
)

var levelOrder = map[Level]int{Unit: 0, Integration: 1, API: 2, E2E: 3}

// Row is one report row (a case, or a parametrized group with --group).
type Row struct {
	ID, File           string
	Level              Level
	LevelInferred      bool // decided by fan-out, shown as "unit~"
	Kills, Unique, Fns int
	Runs               int // functions executed per contexts; -1 when unknown
	InCover            bool
	ShadowedBy         []string
	Flaky              bool
	Cases              int
	Duration           float64 // seconds; +Inf when unknown
	kills              []int
	namespaces         map[string]bool
}

// Offender is a zero-kill or shadowed row (exit 3).
func (r Row) Offender() bool { return !r.Flaky && (r.Kills == 0 || len(r.ShadowedBy) > 0) }

var levelDirs = []struct {
	level Level
	dirs  []string
}{
	{E2E, []string{"e2e", "end2end", "end-to-end", "cypress", "playwright", "acceptance"}},
	{API, []string{"api", "contract", "contracts", "http", "rest", "graphql"}},
	{Integration, []string{"integration", "integration_tests", "integration-tests", "it", "functional"}},
	{Unit, []string{"unit", "unit_tests", "unittest"}},
}

var levelNames = []struct {
	level Level
	re    *regexp.Regexp
}{
	{E2E, regexp.MustCompile(`(\.e2e\.|_e2e)`)},
	{API, regexp.MustCompile(`(\.api\.|^test_api_|_api_test\.)`)},
	{Integration, regexp.MustCompile(`(\.int\.|\.integration\.|^test_integration_|_integration_test\.)`)},
	{Unit, regexp.MustCompile(`\.unit\.`)},
}

var javaIT = regexp.MustCompile(`(IT|ITCase)\.java$`) // case-sensitive

var rustIntegration = regexp.MustCompile(`(^|/)tests/[^/]+\.rs$`)

// LevelOf is §4's path rule: lowercased directory segments first, then the
// file name; ok=false means it must be inferred from fan-out.
func LevelOf(file string) (Level, bool) {
	if file == "" {
		return "", false
	}
	dir, name := path.Split(file)
	for _, seg := range strings.Split(strings.ToLower(strings.Trim(dir, "/")), "/") {
		for _, l := range levelDirs {
			for _, d := range l.dirs {
				if seg == d {
					return l.level, true
				}
			}
		}
	}
	if javaIT.MatchString(name) {
		return Integration, true
	}
	for _, l := range levelNames {
		if l.re.MatchString(strings.ToLower(name)) {
			return l.level, true
		}
	}
	if rustIntegration.MatchString(file) {
		return Integration, true
	}
	return "", false
}

// Analyze computes the rows over the whole matrix (§4 definitions).
func Analyze(m *Matrix, durations map[string]float64) []Row {
	rows := make([]Row, len(m.Tests))
	owners := map[int]int{} // mutant -> number of non-flaky rows killing it
	for i, c := range m.Tests {
		r := Row{ID: c.ID, File: c.File, Runs: -1, Cases: 1, Flaky: m.Flaky[c.ID], kills: m.Kills[c.ID], namespaces: map[string]bool{}}
		if n := m.caseCounts[c.ID]; n > 0 {
			r.Cases = n
		}
		r.Duration = math.Inf(1)
		if d, ok := durations[c.ID]; ok {
			r.Duration = d
		}
		forms := map[string]bool{}
		for _, k := range r.kills {
			mu := m.Mutants[k]
			forms[mu.Namespace+"\x00"+mu.Form] = true
			r.namespaces[mu.Namespace] = true
		}
		r.Kills, r.Fns = len(r.kills), len(forms)
		if m.Runs != nil {
			r.Runs = len(m.Runs[c.ID])
		}
		locator := c.File
		if locator == "" {
			locator = c.Locator
		}
		if lvl, ok := LevelOf(locator); ok {
			r.Level = lvl
		} else {
			fanout := max(len(r.namespaces), r.Runs)
			r.Level, r.LevelInferred = Unit, true
			if fanout >= 3 {
				r.Level = Integration
			}
		}
		if !r.Flaky {
			for _, k := range r.kills {
				owners[k]++
			}
		}
		rows[i] = r
	}
	for i := range rows {
		if rows[i].Flaky {
			continue
		}
		for _, k := range rows[i].kills {
			if owners[k] == 1 {
				rows[i].Unique++
			}
		}
	}
	universe := map[int]bool{}
	var candidates []int
	for i, r := range rows {
		if r.Flaky {
			continue
		}
		candidates = append(candidates, i)
		for _, k := range r.kills {
			universe[k] = true
		}
	}
	cover := greedy(rows, candidates, universe)
	inCover := map[int]bool{}
	for _, i := range cover {
		rows[i].InCover = true
		inCover[i] = true
	}
	for i := range rows {
		r := &rows[i]
		if r.Flaky || r.InCover || r.Kills == 0 {
			continue
		}
		own := map[int]bool{}
		for _, k := range r.kills {
			own[k] = true
		}
		for _, j := range greedy(rows, cover, own) {
			r.ShadowedBy = append(r.ShadowedBy, rows[j].ID)
		}
	}
	return rows
}

// greedy is the covering-set heuristic: the row with the most uncovered
// kills; ties to the lower level, the shorter duration, the smaller ID.
func greedy(rows []Row, candidates []int, universe map[int]bool) []int {
	left := map[int]bool{}
	for k := range universe {
		left[k] = true
	}
	var picked []int
	used := map[int]bool{}
	for len(left) > 0 {
		best, bestGain := -1, 0
		for _, i := range candidates {
			if used[i] {
				continue
			}
			gain := 0
			for _, k := range rows[i].kills {
				if left[k] {
					gain++
				}
			}
			if gain == 0 {
				continue
			}
			if best < 0 || better(rows[i], gain, rows[best], bestGain) {
				best, bestGain = i, gain
			}
		}
		if best < 0 {
			break
		}
		used[best] = true
		picked = append(picked, best)
		for _, k := range rows[best].kills {
			delete(left, k)
		}
	}
	return picked
}

func better(a Row, ga int, b Row, gb int) bool {
	if ga != gb {
		return ga > gb
	}
	if levelOrder[a.Level] != levelOrder[b.Level] {
		return levelOrder[a.Level] < levelOrder[b.Level]
	}
	if a.Duration != b.Duration {
		return a.Duration < b.Duration
	}
	return a.ID < b.ID
}

// sortRows is (offender last, -unique, -kills, ID); flaky rows last of all.
func sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Flaky != b.Flaky {
			return !a.Flaky
		}
		if a.Offender() != b.Offender() {
			return !a.Offender()
		}
		if a.Unique != b.Unique {
			return a.Unique > b.Unique
		}
		if a.Kills != b.Kills {
			return a.Kills > b.Kills
		}
		return a.ID < b.ID
	})
}
