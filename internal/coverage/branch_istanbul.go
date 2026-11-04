package coverage

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/matheusAle/coderefinery/internal/discover"
)

type istanbulPos struct {
	Line   *int `json:"line"`
	Column *int `json:"column"`
}

type istanbulLoc struct {
	Start istanbulPos `json:"start"`
	End   istanbulPos `json:"end"`
}

type istanbulFile struct {
	StatementMap map[string]istanbulLoc `json:"statementMap"`
	S            map[string]int         `json:"s"`
	BranchMap    map[string]struct {
		Type      string        `json:"type"`
		Line      int           `json:"line"`
		Loc       istanbulLoc   `json:"loc"`
		Locations []istanbulLoc `json:"locations"`
	} `json:"branchMap"`
	B map[string][]int `json:"b"`
}

// istanbulTyped are the Istanbul branch types that name their arms
// (babel-plugin-istanbul, Jest, Vitest with AST-aware remapping).
var istanbulTyped = map[string]bool{"if": true, "cond-expr": true, "binary-expr": true, "switch": true, "default-arg": true}

func (p istanbulPos) at() (int, int, bool) {
	if p.Line == nil {
		return 0, 0, false
	}
	col := 0
	if p.Column != nil {
		col = *p.Column + 1 // Istanbul columns are 0-based
	}
	return *p.Line, col, true
}

func before(l1, c1, l2, c2 int) bool { return l1 < l2 || l1 == l2 && c1 <= c2 }

// statementHits is the count of the innermost statement holding (line, col).
func (f *istanbulFile) statementHits(line, col int) int {
	best, bestSpan, hits := false, [2]int{}, 0
	for id, loc := range f.StatementMap {
		sl, sc, ok1 := loc.Start.at()
		el, ec, ok2 := loc.End.at()
		if !ok1 {
			continue
		}
		if !ok2 {
			el, ec = sl, 1<<30
		}
		if !before(sl, sc, line, col) || !before(line, col, el, ec) {
			continue
		}
		span := [2]int{el - sl, ec - sc}
		if !best || span[0] < bestSpan[0] || span[0] == bestSpan[0] && span[1] < bestSpan[1] {
			best, bestSpan, hits = true, span, f.S[id]
		}
	}
	return hits
}

// ParseIstanbul reads an Istanbul coverage-final.json (Vitest, Jest, c8).
// Typed entries (if, cond-expr, binary-expr, switch, default-arg) give one
// ArmHit per arm: granularity branch. Vitest's default V8 conversion only has
// untyped "branch" block ranges; those are counted per line: branch-count.
func ParseIstanbul(data []byte) (*BranchSet, error) {
	var doc map[string]*istanbulFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("istanbul JSON: %v", err)
	}
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	set := &BranchSet{}
	for _, key := range keys {
		if f := istanbulBranches(doc[key], discover.LanguageOf(NormalizePath(key))); f != nil {
			set.add(key, f)
		}
	}
	return set, nil
}

func istanbulBranches(file *istanbulFile, language string) *BranchFile {
	if file == nil || len(file.BranchMap) == 0 {
		return nil
	}
	ids := make([]string, 0, len(file.BranchMap))
	for id := range file.BranchMap {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if len(ids[i]) != len(ids[j]) {
			return len(ids[i]) < len(ids[j])
		}
		return ids[i] < ids[j]
	})
	typed := &BranchFile{Language: language, Granularity: GranBranch}
	lines := map[int]*ArmHit{}
	for _, id := range ids {
		entry, counts := file.BranchMap[id], file.B[id]
		line, col, ok := entry.Loc.Start.at()
		if !ok {
			line, col = entry.Line, 0
		}
		if !istanbulTyped[entry.Type] {
			for k, loc := range entry.Locations {
				l, c, ok := loc.Start.at()
				if !ok || k >= len(counts) {
					continue
				}
				h := lines[l]
				if h == nil {
					h = &ArmHit{Line: l, Kind: KindLineCount}
					lines[l] = h
				}
				h.Of++
				if counts[k] > 0 {
					h.Hits++
				}
				h.Reached = max(h.Reached, counts[k], file.statementHits(l, c))
			}
			continue
		}
		reached := file.statementHits(line, col)
		for _, n := range counts {
			reached = max(reached, n)
		}
		for k := range entry.Locations {
			if k >= len(counts) {
				break
			}
			hit := ArmHit{Line: line, Column: col, Index: k, Of: len(entry.Locations), Kind: entry.Type,
				Hits: counts[k], Reached: reached}
			if entry.Type == "switch" {
				if l, c, ok := entry.Locations[k].Start.at(); ok {
					hit.Line, hit.Column = l, c
				}
			}
			typed.Arms = append(typed.Arms, hit)
		}
	}
	if len(typed.Arms) > 0 {
		return typed
	}
	counted := &BranchFile{Language: language, Granularity: GranBranchCount}
	order := make([]int, 0, len(lines))
	for l := range lines {
		order = append(order, l)
	}
	sort.Ints(order)
	for _, l := range order {
		counted.Arms = append(counted.Arms, *lines[l])
	}
	if len(counted.Arms) == 0 {
		return nil
	}
	return counted
}
