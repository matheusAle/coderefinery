package mutate

// Which lines the tests already exercise (upstream mutator/coverage.py).

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/source"
	"github.com/matheusAle/coderefinery/internal/testplan"
)

// lineSet is a set of covered lines. nil means "no coverage data" (None).
type lineSet map[int]bool

func parts(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func equalParts(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// asSuffix: how many parts longer adds in front of shorter, when it ends
// with it (-1 when it does not).
func asSuffix(longer, shorter []string) int {
	if len(longer) >= len(shorter) && equalParts(longer[len(longer)-len(shorter):], shorter) {
		return len(longer) - len(shorter)
	}
	return -1
}

type rank struct{ kind, n int }

func (a rank) less(b rank) bool { return a.kind < b.kind || a.kind == b.kind && a.n < b.n }

// suffixRank is coverage._suffix_rank.
func suffixRank(key, src []string) (rank, bool) {
	if len(src) == 0 || len(key) == 0 {
		return rank{}, false
	}
	if extra := asSuffix(key, src); extra >= 0 {
		return rank{0, extra}, true
	}
	if len(src) <= len(key) {
		return rank{}, false
	}
	if missing := asSuffix(src, key); missing >= 0 {
		return rank{1, missing}, true
	}
	return rank{}, false
}

// lookup is coverage._lookup over keys (in report order): an exact
// normalized match, else the unique best suffix match. -1 = None.
func lookup(keys []string, sourcePath string) int {
	if len(keys) == 0 {
		return -1
	}
	// {normalize(key): value}: a later duplicate replaces the value but keeps
	// the first key's position.
	var normKeys []string
	at := map[string]int{}
	for i, k := range keys {
		nk := coverage.NormalizePath(k)
		if _, ok := at[nk]; !ok {
			normKeys = append(normKeys, nk)
		}
		at[nk] = i
	}
	var candidates []string
	if sourcePath != "" {
		for _, v := range []string{coverage.NormalizePath(sourcePath), coverage.NormalizePath(discover.Resolve(sourcePath))} {
			if v != "" && !containsString(candidates, v) {
				candidates = append(candidates, v)
			}
		}
	}
	for _, c := range candidates {
		if i, ok := at[c]; ok {
			return i
		}
	}
	var sources [][]string
	for _, c := range candidates {
		if p := parts(c); len(p) > 0 && !containsParts(sources, p) {
			sources = append(sources, p)
		}
	}
	var bestRank rank
	found := false
	var best [][]string
	bestValue := -1
	for _, nk := range normKeys {
		kp := parts(nk)
		var r rank
		ok := false
		for _, s := range sources {
			if sr, sok := suffixRank(kp, s); sok && (!ok || sr.less(r)) {
				r, ok = sr, true
			}
		}
		if !ok {
			continue
		}
		switch {
		case !found || r.less(bestRank):
			bestRank, found, best, bestValue = r, true, [][]string{kp}, at[nk]
		case r == bestRank:
			if !containsParts(best, kp) {
				best = append(best, kp)
			}
		}
	}
	if len(best) != 1 {
		return -1
	}
	return bestValue
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func containsParts(list [][]string, p []string) bool {
	for _, x := range list {
		if equalParts(x, p) {
			return true
		}
	}
	return false
}

func coveredFromHits(lines map[int]coverage.Hits) lineSet {
	out := lineSet{}
	for n, h := range lines {
		if h.Covered > 0 {
			out[n] = true
		}
	}
	return out
}

// ---- JaCoCo (mutator reads line counters, not crapper's method index) ----

type jacocoElem struct {
	XMLName  xml.Name
	Attrs    []xml.Attr   `xml:",any,attr"`
	Children []jacocoElem `xml:",any"`
}

func (e jacocoElem) attr(name string) string {
	for _, a := range e.Attrs {
		if a.Name.Local == name && a.Name.Space == "" {
			return a.Value
		}
	}
	return ""
}

var doctype = regexp.MustCompile(`(?i)<!DOCTYPE[^>]*>`)

// jacocoInt is int(value or "0") with Python's whitespace/underscore rules.
func jacocoInt(s string) (int, bool) {
	if s == "" {
		return 0, true
	}
	n, err := strconv.Atoi(strings.ReplaceAll(strings.TrimFunc(s, isPySpace), "_", ""))
	return n, err == nil
}

func sourceLines(src jacocoElem) lineSet {
	covered := lineSet{}
	for _, line := range src.Children {
		if line.XMLName.Local != "line" {
			continue
		}
		nr, ok1 := jacocoInt(line.attr("nr"))
		ci, ok2 := jacocoInt(line.attr("ci"))
		cb, ok3 := jacocoInt(line.attr("cb"))
		if ok1 && ok2 && ok3 && (ci > 0 || cb > 0) {
			covered[nr] = true
		}
	}
	return covered
}

// packageLines walks every <package> (root.iter: any depth, root included).
func packageLines(e jacocoElem, keys *[]string, found map[string]lineSet) {
	if e.XMLName.Local == "package" {
		pkg := strings.Trim(e.attr("name"), "/")
		for _, src := range e.Children {
			if src.XMLName.Local != "sourcefile" {
				continue
			}
			key := src.attr("name")
			if pkg != "" {
				key = pkg + "/" + key
			}
			if _, ok := found[key]; !ok {
				*keys = append(*keys, key)
			}
			found[key] = sourceLines(src)
		}
	}
	for _, c := range e.Children {
		packageLines(c, keys, found)
	}
}

func jacocoPaths(root string) []string {
	paths := []string{filepath.Join(root, "target", "site", "jacoco", "jacoco.xml")}
	for _, pattern := range []string{"*/target/site/jacoco/jacoco.xml", "*/*/target/site/jacoco/jacoco.xml"} {
		m, _ := filepath.Glob(filepath.Join(root, pattern))
		sort.Strings(m)
		paths = append(paths, m...)
	}
	return paths
}

// jacocoIndex is coverage._jacoco_index: unparseable reports are skipped.
func jacocoIndex(root string) ([]string, map[string]lineSet) {
	var keys []string
	found := map[string]lineSet{}
	seen := map[string]bool{}
	for _, p := range jacocoPaths(root) {
		if !discover.IsFile(p) {
			continue
		}
		r := discover.Resolve(p)
		if seen[r] {
			continue
		}
		seen[r] = true
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		cleaned := doctype.ReplaceAllString(source.DecodeReplace(raw), "")
		dec := xml.NewDecoder(strings.NewReader(cleaned))
		var top jacocoElem
		if err := dec.Decode(&top); err != nil {
			continue
		}
		packageLines(top, &keys, found)
	}
	return keys, found
}

// ---- covered_lines ----

// coveredLines is coverage.covered_lines: the hit lines, or nil when the
// file is absent from coverage. fixGo maps Go coverprofile import paths
// through go.mod first (--fix-go-coverage, DEVIATIONS.md M1).
func coveredLines(root, sourcePath, language string, fixGo bool) (lineSet, error) {
	path := discover.Resolve(sourcePath)
	if language == "java" {
		keys, index := jacocoIndex(root)
		if i := lookup(keys, path); i >= 0 {
			return index[keys[i]], nil
		}
		return nil, nil
	}
	bundle, err := coverage.LoadBundle(root)
	if err != nil {
		return nil, err
	}
	switch language {
	case "go":
		return goLines(bundle.GoProfile, path, root, fixGo), nil
	case "clojure":
		if i := lookup(bundle.FormKeys, path); i >= 0 {
			return coveredFromHits(bundle.FormHTML[bundle.FormKeys[i]]), nil
		}
	}
	if bundle.Lcov == nil {
		return nil, nil
	}
	if i := lookup(bundle.Lcov.Keys, path); i >= 0 {
		return coveredFromHits(bundle.Lcov.Records[bundle.Lcov.Keys[i]].Lines), nil
	}
	return nil, nil
}

func goLines(profile *coverage.GoProfile, path, root string, fixGo bool) lineSet {
	if profile == nil || len(profile.Keys) == 0 {
		return nil
	}
	key := ""
	if fixGo {
		if importPath := goImportPath(path, root); importPath != "" {
			if _, ok := profile.Segments[importPath]; ok {
				key = importPath
			}
		}
	}
	if key == "" {
		i := lookup(profile.Keys, path)
		if i < 0 {
			return nil
		}
		key = profile.Keys[i]
	}
	covered := lineSet{}
	for _, s := range profile.Segments[key] {
		if s.Hits <= 0 {
			continue
		}
		for line := s.Start; line <= s.End; line++ {
			covered[line] = true
		}
	}
	return covered
}

var moduleLine = regexp.MustCompile(`(?m)^\s*module\s+("?)([^\s"]+)("?)\s*$`)

// goImportPath is the coverprofile key `go test` writes for path: the
// nearest go.mod's module path joined with the file's path below it.
func goImportPath(path, root string) string {
	dir := testplan.Nearest(path, "go.mod", root)
	if dir == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	m := moduleLine.FindStringSubmatch(string(raw))
	if m == nil {
		return ""
	}
	rel, ok := discover.RelativeTo(path, dir)
	if !ok {
		return ""
	}
	return m[2] + "/" + rel
}
