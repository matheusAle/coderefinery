package coverage

import (
	"encoding/json"
	"fmt"
	"sort"
)

type coveragePyJSON struct {
	Meta struct {
		BranchCoverage bool `json:"branch_coverage"`
	} `json:"meta"`
	Files map[string]struct {
		ExecutedLines    []int    `json:"executed_lines"`
		ExecutedBranches [][2]int `json:"executed_branches"`
		MissingBranches  [][2]int `json:"missing_branches"`
	} `json:"files"`
}

// ParseCoveragePyJSON reads `coverage json` output. Only a --branch run has
// arcs; the arcs are statement-level (if/elif/for/while/match), with no
// and/or short-circuits and no conditional expressions. Granularity: branch.
func ParseCoveragePyJSON(data []byte) (*BranchSet, error) {
	var doc coveragePyJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("coverage.py JSON: %v", err)
	}
	set := &BranchSet{}
	if !doc.Meta.BranchCoverage {
		return set, nil
	}
	keys := make([]string, 0, len(doc.Files))
	for k := range doc.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rec := doc.Files[key]
		executed := map[int]bool{}
		for _, l := range rec.ExecutedLines {
			executed[l] = true
		}
		f := &BranchFile{Language: "python", Granularity: GranBranch}
		arc := func(a [2]int, hits int) {
			reached := 0
			if executed[a[0]] {
				reached = 1
			}
			f.Arms = append(f.Arms, ArmHit{Line: a[0], FromLine: a[0], ToLine: a[1], Kind: KindArc, Hits: hits, Reached: reached})
		}
		for _, a := range rec.ExecutedBranches {
			arc(a, 1)
		}
		for _, a := range rec.MissingBranches {
			arc(a, 0)
		}
		if len(f.Arms) > 0 {
			set.add(key, f)
		}
	}
	return set, nil
}
