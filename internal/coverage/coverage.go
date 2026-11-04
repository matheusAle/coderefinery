// Package coverage is upstream crapper/coverage.py (loaders, report-key
// matching, per-function percentages) plus runners.go (crapper/runners.py).
// Mutator reuses NormalizePath, LoadBundle and the JaCoCo reader.
package coverage

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
	"github.com/matheusAle/coderefinery/internal/source"
)

// Hits is (covered, total) for one line.
type Hits struct{ Covered, Total int }

// FileCoverage is one LCOV record: line -> hits from DA, plus BRDA branch
// hits aggregated per line.
type FileCoverage struct {
	Lines    map[int]Hits
	Branches map[int]Hits
}

func newFileCoverage() *FileCoverage {
	return &FileCoverage{Lines: map[int]Hits{}, Branches: map[int]Hits{}}
}

// OrderedLcov is a dict[str, FileCoverage] keeping first-insertion order
// (Python dict.update semantics: replacing a key keeps its position).
type OrderedLcov struct {
	Keys    []string
	Records map[string]*FileCoverage
}

func NewOrderedLcov() *OrderedLcov { return &OrderedLcov{Records: map[string]*FileCoverage{}} }

func (o *OrderedLcov) Set(key string, rec *FileCoverage) {
	if _, ok := o.Records[key]; !ok {
		o.Keys = append(o.Keys, key)
	}
	o.Records[key] = rec
}

// Segment is one Go coverprofile block.
type Segment struct{ Start, End, Statements, Hits int }

// GoProfile is file -> segments, keys in first-seen order.
type GoProfile struct {
	Keys     []string
	Segments map[string][]Segment
}

func (g *GoProfile) add(key string, segs ...Segment) {
	if g.Segments == nil {
		g.Segments = map[string][]Segment{}
	}
	if _, ok := g.Segments[key]; !ok {
		g.Keys = append(g.Keys, key)
	}
	g.Segments[key] = append(g.Segments[key], segs...)
}

// JacocoMethod is one method's INSTRUCTION counter.
type JacocoMethod struct{ Missed, Covered, Line int }

func (m JacocoMethod) Percent() float64 { return Percent(m.Covered, m.Missed+m.Covered) }

// Jacoco is "binary.class.name#method" -> methods.
type Jacoco map[string][]JacocoMethod

// FormMap is one Cloverage HTML file: line -> forms.
type FormMap = map[int]Hits

// Bundle is coverage.CoverageBundle.
type Bundle struct {
	Lcov        *OrderedLcov
	GoProfile   *GoProfile // nil = None
	Jacoco      Jacoco     // nil = None
	FormKeys    []string
	FormHTML    map[string]FormMap
	SourceParts [][]string
}

// NewBundle is CoverageBundle() (empty: every function scores 0%).
func NewBundle() *Bundle {
	return &Bundle{Lcov: NewOrderedLcov(), FormHTML: map[string]FormMap{}}
}

// NormalizePath is coverage.normalize_path.
func NormalizePath(p string) string {
	text := strings.ReplaceAll(unquote(p), `\`, "/")
	text = strings.TrimPrefix(text, "file:")
	for strings.HasPrefix(text, "./") {
		text = text[2:]
	}
	for strings.Contains(text, "//") {
		text = strings.ReplaceAll(text, "//", "/")
	}
	return text
}

// unquote is urllib.parse.unquote (UTF-8, errors=replace, '+' kept).
func unquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var raw []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				raw = append(raw, byte(v))
				i += 2
				continue
			}
		}
		raw = append(raw, s[i])
	}
	return source.DecodeReplace(raw)
}

func segments(p string) []string {
	var out []string
	for _, part := range strings.Split(NormalizePath(p), "/") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func hasSuffixParts(parts, suffix []string) bool {
	if len(suffix) > len(parts) {
		return false
	}
	off := len(parts) - len(suffix)
	for i := range suffix {
		if parts[off+i] != suffix[i] {
			return false
		}
	}
	return true
}

var srcPrefix = regexp.MustCompile(`^src/`)

// Candidates is coverage._candidates. The absolute form resolves relative
// paths against the process CWD (upstream quirk).
func Candidates(sourcePath string) []string {
	rel := NormalizePath(sourcePath)
	abs := rel
	if sourcePath != "" {
		abs = NormalizePath(discover.Resolve(sourcePath))
	}
	values := []string{rel, abs, srcPrefix.ReplaceAllString(rel, ""), strings.ReplaceAll(abs, "/src/", "/")}
	var found []string
	for _, v := range values {
		if v == "" {
			continue
		}
		dup := false
		for _, f := range found {
			if f == v {
				dup = true
			}
		}
		if !dup {
			found = append(found, v)
		}
	}
	return found
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

// SelectKey is coverage._select_key.
func SelectKey(keys []string, sourcePath string, sourceParts [][]string) (string, bool) {
	norms := make([]string, len(keys))
	for i, k := range keys {
		norms[i] = NormalizePath(k)
	}
	cands := Candidates(sourcePath)
	for _, c := range cands {
		for i := range keys {
			if norms[i] == c {
				return keys[i], true
			}
		}
	}
	var partsOf [][]string
	for _, c := range cands {
		if p := segments(c); len(p) > 0 {
			partsOf = append(partsOf, p)
		}
	}
	// forward: the key is longer than the source
	type fwd struct {
		extra int
		key   string
	}
	var forward []fwd
	for _, parts := range partsOf {
		for i, key := range keys {
			kp := segments(norms[i])
			if !hasSuffixParts(kp, parts) || longerSourceOwns(kp, len(parts), sourceParts) {
				continue
			}
			forward = append(forward, fwd{len(kp) - len(parts), key})
		}
	}
	if len(forward) > 0 {
		smallest := forward[0].extra
		for _, f := range forward {
			smallest = min(smallest, f.extra)
		}
		var unique []string
		for _, f := range forward {
			if f.extra == smallest && !containsStr(unique, f.key) {
				unique = append(unique, f.key)
			}
		}
		if len(unique) == 1 {
			return unique[0], true
		}
		return "", false
	}
	// reverse: the key is shorter than the source
	var reverse []string
	for _, parts := range partsOf {
		for i, key := range keys {
			kp := segments(norms[i])
			if len(kp) == 0 || len(kp) >= len(parts) || !hasSuffixParts(parts, kp) {
				continue
			}
			if anotherSourceEndsWith(kp, sourceParts, partsOf) {
				continue
			}
			if !containsStr(reverse, key) {
				reverse = append(reverse, key)
			}
		}
	}
	if len(reverse) == 1 {
		return reverse[0], true
	}
	return "", false
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func longerSourceOwns(keyParts []string, sourceLen int, sourceParts [][]string) bool {
	for _, other := range sourceParts {
		if len(other) <= sourceLen {
			continue
		}
		if len(other) <= len(keyParts) && hasSuffixParts(keyParts, other) {
			return true
		}
	}
	return false
}

func anotherSourceEndsWith(keyParts []string, sourceParts, own [][]string) bool {
	for _, other := range sourceParts {
		isOwn := false
		for _, o := range own {
			if equalParts(o, other) {
				isOwn = true
				break
			}
		}
		if isOwn || len(other) < len(keyParts) {
			continue
		}
		if hasSuffixParts(other, keyParts) {
			return true
		}
	}
	return false
}

// Percent is coverage.coverage_percent.
func Percent(covered, total int) float64 {
	if total <= 0 {
		return 0.0
	}
	return (100.0 * float64(covered)) / float64(total)
}

// PercentForRange is coverage.percent_for_range (lines non-nil).
func PercentForRange(lines map[int]Hits, start, end int) float64 {
	covered, total := 0, 0
	for line := start; line <= end; line++ {
		if h, ok := lines[line]; ok {
			covered += h.Covered
			total += h.Total
		}
	}
	if total == 0 {
		return 0.0
	}
	return Percent(covered, total)
}

var formSpan = regexp.MustCompile(`<span[^>]*title="(\d+) out of (\d+) forms covered"[^>]*>\s*(\d+)&nbsp;`)

// ParseFormCoverage is coverage.parse_form_coverage.
func ParseFormCoverage(html string) FormMap {
	found := FormMap{}
	for _, m := range formSpan.FindAllStringSubmatch(html, -1) {
		total, _ := strconv.Atoi(m[2])
		if total <= 0 {
			continue
		}
		covered, _ := strconv.Atoi(m[1])
		line, _ := strconv.Atoi(m[3])
		found[line] = Hits{covered, total}
	}
	return found
}

var (
	lcovDA   = regexp.MustCompile(`^DA:([0-9]+),([0-9]+)`)
	lcovBRDA = regexp.MustCompile(`^BRDA:([0-9]+),([^,]*),([^,]*),(-|[0-9]+)`)
)

// SplitLines is Python str.splitlines() (no keepends).
func SplitLines(text string) []string {
	var out []string
	start := 0
	i := 0
	for i < len(text) {
		r, size := rune(text[i]), 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(text[i:])
		}
		brk := 0
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			brk = size
		case '\r':
			brk = 1
			if i+1 < len(text) && text[i+1] == '\n' {
				brk = 2
			}
		}
		if brk > 0 {
			out = append(out, text[start:i])
			i += brk
			start = i
			continue
		}
		i += size
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// PyStrip is Python str.strip() (Unicode whitespace).
func PyStrip(s string) string {
	return strings.TrimFunc(s, isPySpace)
}

func isPySpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// ParseLcov is coverage.parse_lcov.
func ParseLcov(text string) *OrderedLcov {
	out := NewOrderedLcov()
	var currentFile *string
	current := newFileCoverage()
	store := func() {
		if currentFile != nil {
			out.Set(*currentFile, current)
		}
	}
	for _, raw := range SplitLines(text) {
		line := PyStrip(raw)
		switch {
		case strings.HasPrefix(line, "SF:"):
			store()
			f := line[3:]
			currentFile = &f
			current = newFileCoverage()
		case line == "end_of_record":
			store()
			currentFile = nil
			current = newFileCoverage()
		case currentFile != nil:
			if m := lcovDA.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				hits, _ := strconv.Atoi(m[2])
				covered := 0
				if hits > 0 {
					covered = 1
				}
				current.Lines[n] = Hits{covered, 1}
			} else if m := lcovBRDA.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				h := current.Branches[n]
				taken, err := strconv.Atoi(m[4])
				if m[4] != "-" && err == nil && taken > 0 {
					h.Covered++
				}
				h.Total++
				current.Branches[n] = h
			}
		}
	}
	store()
	return out
}

// BranchPercent is coverage.branch_percent; ok=false means "no branches".
func BranchPercent(rec *FileCoverage, start, end int) (float64, bool) {
	if len(rec.Branches) == 0 {
		return 0, false
	}
	covered, total := 0, 0
	for line := start; line <= end; line++ {
		if h, ok := rec.Branches[line]; ok {
			covered += h.Covered
			total += h.Total
		}
	}
	if total == 0 {
		return 0, false
	}
	return Percent(covered, total), true
}

// ParseGoProfile is coverage.parse_go_profile; malformed lines are fatal.
func ParseGoProfile(text string) (*GoProfile, error) {
	out := &GoProfile{Segments: map[string][]Segment{}}
	for i, raw := range SplitLines(text) {
		line := PyStrip(raw)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.FieldsFunc(line, isPySpace)
		bad := func() error {
			return fmt.Errorf("invalid coverage segment on line %d: %s", i+1, pyfmt.PyRepr(raw))
		}
		if len(fields) != 3 || !strings.Contains(fields[0], ":") {
			return nil, bad()
		}
		file, rng, _ := strings.Cut(fields[0], ":")
		a, b, ok := strings.Cut(rng, ",")
		if !ok {
			return nil, fmt.Errorf("not enough values to unpack (expected 2, got 1)")
		}
		start, err1 := pyInt(strings.SplitN(a, ".", 2)[0])
		end, err2 := pyInt(strings.SplitN(b, ".", 2)[0])
		stmts, err3 := pyInt(fields[1])
		hits, err4 := pyInt(fields[2])
		for _, err := range []error{err1, err2, err3, err4} {
			if err != nil {
				return nil, err
			}
		}
		out.add(file, Segment{start, end, stmts, hits})
	}
	return out, nil
}

func pyInt(s string) (int, error) {
	v, err := strconv.Atoi(strings.ReplaceAll(PyStrip(s), "_", ""))
	if err != nil {
		return 0, fmt.Errorf("invalid literal for int() with base 10: %s", pyfmt.PyRepr(s))
	}
	return v, nil
}

// GoPercent is coverage.go_percent; ok=false = not in the profile.
func GoPercent(profile *GoProfile, path string, start, end int, sourceParts [][]string) (float64, bool) {
	if profile == nil {
		return 0, false
	}
	key, ok := SelectKey(profile.Keys, path, sourceParts)
	if !ok {
		return 0, false
	}
	total, covered := 0, 0
	for _, s := range profile.Segments[key] {
		if s.End < start || s.Start > end {
			continue
		}
		total += s.Statements
		if s.Hits > 0 {
			covered += s.Statements
		}
	}
	if total == 0 {
		return 0.0, true
	}
	return Percent(covered, total), true
}

var doctype = regexp.MustCompile(`(?i)<!DOCTYPE[^>]*>`)

type xmlElem struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []xmlElem  `xml:",any"`
}

func (e xmlElem) attr(name string) (string, bool) {
	for _, a := range e.Attrs {
		if a.Name.Local == name && a.Name.Space == "" {
			return a.Value, true
		}
	}
	return "", false
}

// ParseJacocoIndex is coverage.parse_jacoco_index.
func ParseJacocoIndex(text string) (Jacoco, error) {
	cleaned := doctype.ReplaceAllString(text, "")
	dec := xml.NewDecoder(strings.NewReader(cleaned))
	dec.Strict = true
	var root xmlElem
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	found := Jacoco{}
	var visit func(e xmlElem) error
	visit = func(e xmlElem) error {
		if e.XMLName.Local == "class" {
			name, _ := e.attr("name")
			cname := strings.ReplaceAll(name, "/", ".")
			for _, m := range e.Children {
				if m.XMLName.Local != "method" {
					continue
				}
				var counter *xmlElem
				for i := range m.Children {
					if c := m.Children[i]; c.XMLName.Local == "counter" {
						if t, _ := c.attr("type"); t == "INSTRUCTION" {
							counter = &m.Children[i]
							break
						}
					}
				}
				if counter == nil {
					continue
				}
				missed, err := attrInt(*counter, "missed")
				if err != nil {
					return err
				}
				covered, err := attrInt(*counter, "covered")
				if err != nil {
					return err
				}
				line := 0
				if v, ok := m.attr("line"); ok && v != "" {
					if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
						line = n
					}
				}
				mname, _ := m.attr("name")
				key := cname + "#" + mname
				found[key] = append(found[key], JacocoMethod{missed, covered, line})
			}
		}
		for _, c := range e.Children {
			if err := visit(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	return found, nil
}

func attrInt(e xmlElem, name string) (int, error) {
	v, _ := e.attr(name)
	if v == "" {
		return 0, nil
	}
	return pyInt(v)
}

func jacocoFromIndex(classes Jacoco, names []string, method string, line int) (float64, bool) {
	var pool []JacocoMethod
	for _, n := range names {
		pool = append(pool, classes[n+"#"+method]...)
	}
	if len(pool) == 0 {
		return 0, false
	}
	for _, m := range pool {
		if m.Line == line {
			return m.Percent(), true
		}
	}
	best := pool[0]
	for _, m := range pool[1:] {
		if abs(m.Line-line) < abs(best.Line-line) {
			best = m
		}
	}
	return best.Percent(), true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// BindSources is CoverageBundle.bind_sources.
func (b *Bundle) BindSources(paths []string) {
	var found [][]string
	for _, src := range paths {
		for _, c := range Candidates(src) {
			parts := segments(c)
			if len(parts) == 0 {
				continue
			}
			dup := false
			for _, f := range found {
				if equalParts(f, parts) {
					dup = true
					break
				}
			}
			if !dup {
				found = append(found, parts)
			}
		}
	}
	b.SourceParts = found
}

// PercentFor is CoverageBundle.percent_for; ok=false = not in any report.
func (b *Bundle) PercentFor(fn model.Function) (float64, bool) {
	switch fn.Language {
	case "java":
		var names []string
		if fn.JacocoClass != "" {
			names = append(names, fn.JacocoClass)
		}
		if !containsStr(names, fn.Namespace) {
			names = append(names, fn.Namespace)
		}
		j := b.Jacoco
		if j == nil {
			j = Jacoco{}
		}
		return jacocoFromIndex(j, names, fn.Name, fn.StartLine)
	case "go":
		return GoPercent(b.GoProfile, fn.Path, fn.StartLine, fn.EndLine, b.SourceParts)
	}
	if len(b.FormHTML) > 0 {
		if key, ok := SelectKey(b.FormKeys, fn.Path, b.SourceParts); ok {
			return PercentForRange(b.FormHTML[key], fn.StartLine, fn.EndLine), true
		}
	}
	if b.Lcov == nil || len(b.Lcov.Keys) == 0 {
		return 0, false
	}
	key, ok := SelectKey(b.Lcov.Keys, fn.Path, b.SourceParts)
	if !ok {
		return 0, false
	}
	rec := b.Lcov.Records[key]
	if pct, ok := BranchPercent(rec, fn.StartLine, fn.EndLine); ok {
		return pct, true
	}
	return PercentForRange(rec.Lines, fn.StartLine, fn.EndLine), true
}

func readText(path string) (string, bool) {
	if !discover.IsFile(path) {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return source.Text(raw), true
}

// globSorted is pathlib glob with one "*" per path segment, sorted (the
// Go port sorts each glob; upstream uses filesystem order).
func globSorted(pattern string) []string {
	m, _ := filepath.Glob(pattern)
	sort.Strings(m)
	return m
}

// recursiveGlob is root.glob(prefix + "/**/" + name): name at any depth
// (zero or more directories) below prefix, dot-directories included.
func recursiveGlob(prefix, name string) []string {
	var out []string
	if !discover.IsDir(prefix) {
		return nil
	}
	_ = filepath.WalkDir(prefix, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == name {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func mergeTexts(paths []string) []string {
	var texts []string
	seen := map[string]bool{}
	for _, p := range paths {
		r := discover.Resolve(p)
		if seen[r] {
			continue
		}
		seen[r] = true
		if text, ok := readText(p); ok && text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

// LoadBundle is coverage.load_bundle. Malformed Go profiles and JaCoCo XML
// are errors (upstream crashes with exit 1).
func LoadBundle(root string) (*Bundle, error) {
	b := NewBundle()
	lcovPaths := append(recursiveGlob(filepath.Join(root, "target", "coverage"), "lcov.info"),
		recursiveGlob(filepath.Join(root, "coverage"), "lcov.info")...)
	for _, text := range mergeTexts(lcovPaths) {
		parsed := ParseLcov(text)
		for _, k := range parsed.Keys {
			b.Lcov.Set(k, parsed.Records[k])
		}
	}
	goPaths := []string{
		filepath.Join(root, "target", "coverage", "coverage.out"),
		filepath.Join(root, "target", "coverage", "go", "coverage.out"),
		filepath.Join(root, "coverage.out"),
	}
	goPaths = append(goPaths, globSorted(filepath.Join(root, "*", "target", "coverage", "go", "coverage.out"))...)
	goPaths = append(goPaths, globSorted(filepath.Join(root, "*", "*", "target", "coverage", "go", "coverage.out"))...)
	merged := &GoProfile{}
	for _, text := range mergeTexts(goPaths) {
		prof, err := ParseGoProfile(text)
		if err != nil {
			return nil, err
		}
		for _, k := range prof.Keys {
			merged.add(k, prof.Segments[k]...)
		}
	}
	if len(merged.Keys) > 0 {
		b.GoProfile = merged
	}
	jacocoPaths := []string{filepath.Join(root, "target", "site", "jacoco", "jacoco.xml")}
	jacocoPaths = append(jacocoPaths, globSorted(filepath.Join(root, "*", "target", "site", "jacoco", "jacoco.xml"))...)
	jacocoPaths = append(jacocoPaths, globSorted(filepath.Join(root, "*", "*", "target", "site", "jacoco", "jacoco.xml"))...)
	jac := Jacoco{}
	for _, text := range mergeTexts(jacocoPaths) {
		idx, err := ParseJacocoIndex(text)
		if err != nil {
			return nil, err
		}
		for k, v := range idx {
			jac[k] = append(jac[k], v...)
		}
	}
	if len(jac) > 0 {
		b.Jacoco = jac
	}
	covDir := filepath.Join(root, "target", "coverage")
	if discover.IsDir(covDir) {
		for _, p := range recursiveGlobSuffix(covDir, ".html") {
			text, ok := readText(p)
			if !ok || text == "" || !strings.Contains(text, "forms covered") {
				continue
			}
			rel, _ := filepath.Rel(covDir, p)
			rel = strings.TrimSuffix(filepath.ToSlash(rel), ".html")
			if _, dup := b.FormHTML[rel]; !dup {
				b.FormKeys = append(b.FormKeys, rel)
			}
			b.FormHTML[rel] = ParseFormCoverage(text)
		}
	}
	return b, nil
}

func recursiveGlobSuffix(dir, suffix string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), suffix) {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}
