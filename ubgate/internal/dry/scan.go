package dry

import (
	"fmt"
	"sort"

	"github.com/matheusAle/ub-mode/ubgate/internal/discover"
	"github.com/matheusAle/ub-mode/ubgate/internal/source"
)

// Scanning and pairing: upstream scan.py and model.py (spec-dryer §5).

// Span is model.Span.
type Span struct {
	File               string
	StartLine, EndLine int
}

// Entry is model.Entry: a normalized form ready to compare.
type Entry struct {
	Language     string
	Span         Span
	Nodes        int
	Fingerprints map[string]struct{}
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

// EntriesInSource is extract.entries_in_source: the entries of one source
// text, unfiltered, plus the Clojure reader warning. A file the hand parser
// cannot parse contributes nothing (DEVIATIONS A2).
func EntriesInSource(language, text, path, file string) ([]Entry, string) {
	cands, warning, _ := Candidates(language, text, path)
	entries := make([]Entry, 0, len(cands))
	for _, c := range cands {
		entries = append(entries, Entry{
			Language:     language,
			Span:         Span{File: file, StartLine: c.StartLine, EndLine: c.EndLine},
			Nodes:        NodeCount(c.Norm),
			Fingerprints: Fingerprints(c.Norm),
		})
	}
	return entries, warning
}

// ScanFiles is scan.scan_files: kept entries in file order, and one
// "<file>:<line>: <msg>" warning per unreadable Clojure tail.
func ScanFiles(files []string, root string, minLines, minNodes int) ([]Entry, []string, error) {
	var entries []Entry
	var warnings []string
	for _, path := range files {
		language := LanguageOf(path)
		if language == "" {
			continue
		}
		text, err := source.Read(path)
		if err != nil {
			return nil, nil, err
		}
		file := displayPath(path, root)
		found, warning := EntriesInSource(language, text, path, file)
		if warning != "" {
			warnings = append(warnings, fmt.Sprintf("%s:%s", file, warning))
		}
		for _, e := range found {
			if keep(e, minLines, minNodes) {
				entries = append(entries, e)
			}
		}
	}
	return entries, warnings, nil
}

// FindDuplicates is scan.find_duplicates: every unordered pair of entries
// of one language (first-seen language order), except two entries with the
// same file and span, whose Jaccard score is at least threshold; sorted by
// score descending, then language, left file/start, right file/start.
func FindDuplicates(entries []Entry, threshold float64) []Duplicate {
	var order []string
	groups := map[string][]Entry{}
	for _, e := range entries {
		if _, ok := groups[e.Language]; !ok {
			order = append(order, e.Language)
		}
		groups[e.Language] = append(groups[e.Language], e)
	}
	var found []Duplicate
	for _, language := range order {
		group := groups[language]
		for i, left := range group {
			for _, right := range group[i+1:] {
				if left.Span == right.Span {
					continue
				}
				score := Jaccard(left.Fingerprints, right.Fingerprints)
				if score < threshold {
					continue
				}
				found = append(found, Duplicate{Score: score, Language: language, Left: left.Span,
					Right: right.Span, LeftNodes: left.Nodes, RightNodes: right.Nodes})
			}
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return duplicateLess(found[i], found[j]) })
	return found
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
