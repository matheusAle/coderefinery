package dry

import (
	"cmp"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/matheusAle/ub-mode/ubgate/internal/discover"
	"github.com/matheusAle/ub-mode/ubgate/internal/source"
)

// Scanning and pairing: upstream scan.py and model.py (spec-dryer §5).

// Span is model.Span.
type Span struct {
	File               string
	StartLine, EndLine int
}

// Entry is model.Entry: a normalized form ready to compare. Fingerprints
// are the sorted ids its language's Fingerprinter gave the fingerprint set.
type Entry struct {
	Language     string
	Span         Span
	Nodes        int
	Fingerprints []uint32
}

// Duplicate is model.Duplicate.
type Duplicate struct {
	Score                 float64
	Language              string
	Left, Right           Span
	LeftNodes, RightNodes int
}

// displayPath is scan.display_path: relative to root when inside it.
func displayPath(path, root string) string {
	resolved := discover.Resolve(path)
	if rel, ok := discover.RelativeTo(resolved, discover.Resolve(root)); ok {
		return rel
	}
	return resolved
}

func keep(e Entry, minLines, minNodes int) bool {
	return e.Span.EndLine-e.Span.StartLine+1 >= minLines && e.Nodes >= minNodes
}

// EntriesInSource is extract.entries_in_source followed by scan's
// --min-lines/--min-nodes filter: the kept entries of one source text, plus
// the Clojure reader warning. Filtering first spares fingerprinting the
// forms that are dropped anyway. A file the hand parser cannot parse
// contributes nothing (DEVIATIONS A2).
func EntriesInSource(fp *Fingerprinter, language, text, path, file string, minLines, minNodes int) ([]Entry, string) {
	cands, warning, _ := Candidates(language, text, path)
	var entries []Entry
	for _, c := range cands {
		e := Entry{Language: language, Span: Span{File: file, StartLine: c.StartLine, EndLine: c.EndLine},
			Nodes: NodeCount(c.Norm)}
		if keep(e, minLines, minNodes) {
			e.Fingerprints = fp.Set(c.Norm)
			entries = append(entries, e)
		}
	}
	return entries, warning
}

// ScanFiles is scan.scan_files: kept entries in file order, and one
// "<file>:<line>: <msg>" warning per unreadable Clojure tail. A language
// whose structural fingerprints turn out ambiguous is scanned again with
// printed fingerprints (see Fingerprinter).
func ScanFiles(files []string, root string, minLines, minNodes int) ([]Entry, []string, error) {
	entries, warnings, ambiguous, err := scanFiles(files, root, minLines, minNodes, nil)
	if err != nil || len(ambiguous) == 0 {
		return entries, warnings, err
	}
	entries, warnings, _, err = scanFiles(files, root, minLines, minNodes, ambiguous)
	return entries, warnings, err
}

// scanFiles is one ScanFiles pass, fingerprinting the languages in
// byString by printed form. It also returns the languages whose structural
// fingerprints were ambiguous. The interners, and with them every distinct
// fingerprint key, are garbage once it returns.
func scanFiles(files []string, root string, minLines, minNodes int, byString map[string]bool) ([]Entry, []string, map[string]bool, error) {
	var entries []Entry
	var warnings []string
	interners := map[string]*Fingerprinter{}
	for _, path := range files {
		language := LanguageOf(path)
		if language == "" {
			continue
		}
		text, err := source.Read(path)
		if err != nil {
			return nil, nil, nil, err
		}
		fp := interners[language]
		if fp == nil {
			fp = NewFingerprinter(byString[language])
			interners[language] = fp
		}
		file := displayPath(path, root)
		found, warning := EntriesInSource(fp, language, text, path, file, minLines, minNodes)
		if warning != "" {
			warnings = append(warnings, fmt.Sprintf("%s:%s", file, warning))
		}
		entries = append(entries, found...)
	}
	ambiguous := map[string]bool{}
	for language, fp := range interners {
		if fp.ambiguous {
			ambiguous[language] = true
		}
	}
	return entries, warnings, ambiguous, nil
}

// FindDuplicates is scan.find_duplicates: every unordered pair of entries
// of one language (first-seen language order), except two entries with the
// same file and span, whose Jaccard score is at least threshold; sorted by
// score descending, then language, left file/start, right file/start, and
// enumeration order (upstream's stable sort). Pairs are scored in parallel
// and only when their set sizes allow the threshold (see sharedNeeded); the
// hits are put back in enumeration order before the sort, so the result is
// the same as upstream's nested loop.
func FindDuplicates(entries []Entry, threshold float64) []Duplicate {
	var order []string
	groups := map[string][]Entry{}
	for _, e := range entries {
		if _, ok := groups[e.Language]; !ok {
			order = append(order, e.Language)
		}
		groups[e.Language] = append(groups[e.Language], e)
	}
	var hits []pairHit
	for g, language := range order {
		hits = append(hits, groupHits(groups[language], g, threshold)...)
	}
	slices.SortFunc(hits, func(a, b pairHit) int {
		return cmp.Or(cmp.Compare(a.group, b.group), cmp.Compare(a.left, b.left), cmp.Compare(a.right, b.right))
	})
	found := make([]Duplicate, len(hits))
	for i, h := range hits {
		found[i] = h.dup
	}
	sort.SliceStable(found, func(i, j int) bool { return duplicateLess(found[i], found[j]) })
	return found
}

// pairHit is a reported pair and its enumeration position: language group,
// then the left and right indexes inside the group.
type pairHit struct {
	group, left, right int
	dup                Duplicate
}

// pairChunk is how many rows of the size-sorted group a worker claims at once.
const pairChunk = 16

// groupHits scores the pairs of one language group across GOMAXPROCS
// workers. Entries are visited by ascending set size, so for a row p every
// later q has |q| >= |p| and the size bound |p|/|q| only falls as q grows:
// the first q the bound rules out ends the row.
func groupHits(group []Entry, g int, threshold float64) []pairHit {
	bySize := make([]int, len(group))
	for i := range bySize {
		bySize[i] = i
	}
	slices.SortStableFunc(bySize, func(a, b int) int {
		return cmp.Compare(len(group[a].Fingerprints), len(group[b].Fingerprints))
	})
	workers := min(runtime.GOMAXPROCS(0), (len(group)+pairChunk-1)/pairChunk)
	results := make([][]pairHit, workers)
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for {
				start := int(next.Add(pairChunk)) - pairChunk
				if start >= len(bySize) {
					return
				}
				for p := start; p < min(start+pairChunk, len(bySize)); p++ {
					results[w] = rowHits(group, bySize, p, g, threshold, results[w])
				}
			}
		})
	}
	wg.Wait()
	return slices.Concat(results...)
}

// rowHits appends the hits of size-sorted row p against every later row.
func rowHits(group []Entry, bySize []int, p, g int, threshold float64, hits []pairHit) []pairHit {
	a := group[bySize[p]]
	for _, iq := range bySize[p+1:] {
		b := group[iq]
		need, ok := sharedNeeded(len(a.Fingerprints), len(b.Fingerprints), threshold)
		if !ok {
			break
		}
		if a.Span == b.Span {
			continue
		}
		shared, ok := countShared(a.Fingerprints, b.Fingerprints, need)
		if !ok {
			continue
		}
		left, right, il, ir := a, b, bySize[p], iq
		if il > ir {
			left, right, il, ir = b, a, ir, il
		}
		hits = append(hits, pairHit{group: g, left: il, right: ir, dup: Duplicate{
			Score: jaccard(shared, len(left.Fingerprints), len(right.Fingerprints)), Language: left.Language,
			Left: left.Span, Right: right.Span, LeftNodes: left.Nodes, RightNodes: right.Nodes}})
	}
	return hits
}

// reaches is upstream's `not score < threshold`: equal is reported, and a
// NaN threshold reports every pair.
func reaches(score, threshold float64) bool { return !(score < threshold) }

// sharedNeeded is the least shared count whose score reaches threshold for
// two sets of small <= large ids, and false when no count can (even
// shared == small, score small/large). The score is the exact float
// expression the pair would get, and it never falls as shared grows
// (numerator up, denominator down, rounding monotone), so the answer is
// exact: a pair is only skipped when its real score is below threshold.
// The algebraic estimate s >= t(small+large)/(1+t) is only a starting
// point; the loops settle on the boundary with the real expression.
func sharedNeeded(small, large int, threshold float64) (int, bool) {
	if !reaches(jaccard(small, small, large), threshold) {
		return 0, false
	}
	s := 0 // a NaN or non-positive threshold is reached by any count
	if threshold > 0 {
		s = min(small, int(math.Ceil(threshold*float64(small+large)/(1+threshold))))
	}
	for s > 0 && reaches(jaccard(s-1, small, large), threshold) {
		s--
	}
	for !reaches(jaccard(s, small, large), threshold) {
		s++
	}
	return s, true
}

// countShared is |A∩B| of two sorted id sets, by merge. It gives up (false)
// as soon as either side has missed so many ids that fewer than need can
// still be shared.
func countShared(left, right []uint32, need int) (int, bool) {
	leftSlack, rightSlack := len(left)-need, len(right)-need
	i, j, shared := 0, 0, 0
	for i < len(left) && j < len(right) {
		switch l, r := left[i], right[j]; {
		case l == r:
			shared++
			i++
			j++
		case l < r:
			i++
			if leftSlack--; leftSlack < 0 {
				return shared, false
			}
		default:
			j++
			if rightSlack--; rightSlack < 0 {
				return shared, false
			}
		}
	}
	return shared, shared >= need
}

func duplicateLess(a, b Duplicate) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Language != b.Language {
		return a.Language < b.Language
	}
	if a.Left.File != b.Left.File {
		return a.Left.File < b.Left.File
	}
	if a.Left.StartLine != b.Left.StartLine {
		return a.Left.StartLine < b.Left.StartLine
	}
	if a.Right.File != b.Right.File {
		return a.Right.File < b.Right.File
	}
	return a.Right.StartLine < b.Right.StartLine
}
