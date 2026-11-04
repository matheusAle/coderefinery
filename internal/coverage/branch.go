package coverage

// Branch data for `crap --branches` and `mutate --branches`
// (docs/design-tests.md §6). Each loader says what its tool can locate:
//
//	branch        coverage.py arcs, Istanbul if/cond-expr/binary-expr/switch, llvm-cov --branch
//	branch-count  LCOV BRDA, JaCoCo line mb/cb, V8 block ranges: how many arms on a line, not which
//	block         Go coverprofile blocks (never called a branch)
//	region        llvm-cov code regions
//	form          Cloverage form counts per line
//
// The coderefinery test outputs under .metrics/tests/<lang>/ win; without
// them the CRAP reports already on disk are read.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
)

// ArmHit is one arm, block, region or per-line count as the tool reports it.
type ArmHit struct {
	Line, Column, EndLine, EndColumn int // 1-based; columns 0 when the tool has none
	FromLine, ToLine                 int // coverage.py arcs only (ToLine < 0 = function exit)
	Index, Of                        int // arm k of n (Istanbul, llvm branch); line-count/form: Of arms or forms, Hits of them covered
	Kind                             string
	Hits                             int
	Reached                          int // hits of the decision itself when the tool knows them, -1 when it does not
}

// Arm and count kinds.
const (
	KindArc       = "arc"
	KindBlock     = "block"
	KindRegion    = "region"
	KindLLVM      = "llvm-branch"
	KindLineCount = "line-count"
	KindForm      = "form"
)

// Granularities.
const (
	GranBranch      = "branch"
	GranBranchCount = "branch-count"
	GranBlock       = "block"
	GranRegion      = "region"
	GranForm        = "form"
)

// Why a language has no branch data.
const (
	MissingRunTest   = "run coderefinery test"
	MissingTarpaulin = "tarpaulin reports lines only"
)

// BranchFile is one source file's branch data.
type BranchFile struct {
	Language    string
	Granularity string
	Arms        []ArmHit
}

// BranchBundle is every report's branch data, keyed like the report.
type BranchBundle struct {
	Files       map[string]*BranchFile
	Keys        []string          // report keys, first-seen order
	Granularity map[string]string // language -> granularity of its data
	Missing     map[string]string // language -> why there is no data
}

// BranchLanguages are the languages LoadBranches looks for.
var BranchLanguages = []string{"clojure", "go", "java", "python", "rust", "typescript"}

// BranchSet is one report's files in first-seen key order.
type BranchSet struct {
	Keys  []string
	Files map[string]*BranchFile
}

func (s *BranchSet) add(key string, f *BranchFile) {
	if s.Files == nil {
		s.Files = map[string]*BranchFile{}
	}
	if old, ok := s.Files[key]; ok {
		old.Arms = append(old.Arms, f.Arms...)
		return
	}
	s.Keys = append(s.Keys, key)
	s.Files[key] = f
}

func (s *BranchSet) merge(o *BranchSet) {
	if o == nil {
		return
	}
	for _, k := range o.Keys {
		s.add(k, o.Files[k])
	}
}

// File returns the branch data for a source path (relative to the root, as
// crap names it) of the given language, or nil.
func (b *BranchBundle) File(path, language string, sourceParts [][]string) *BranchFile {
	var keys []string
	for _, k := range b.Keys {
		if b.Files[k].Language == language {
			keys = append(keys, k)
		}
	}
	key, ok := SelectKey(keys, path, sourceParts)
	if !ok {
		return nil
	}
	return b.Files[key]
}

// LoadBranches reads the branch data of every language under root.
func LoadBranches(root string) (*BranchBundle, error) {
	root = discover.Resolve(root)
	b := &BranchBundle{Files: map[string]*BranchFile{}, Granularity: map[string]string{}, Missing: map[string]string{}}
	var lcov *BranchSet
	crapLcov := func() *BranchSet {
		if lcov == nil {
			lcov = lcovBranches(append(recursiveGlob(filepath.Join(root, "target", "coverage"), "lcov.info"),
				recursiveGlob(filepath.Join(root, "coverage"), "lcov.info")...))
		}
		return lcov
	}
	for _, language := range BranchLanguages {
		set, err := testBranches(filepath.Join(root, ".metrics", "tests", language), language)
		if err != nil {
			return nil, err
		}
		if len(set.Keys) == 0 {
			if set, err = reportBranches(root, language, crapLcov); err != nil {
				return nil, err
			}
		}
		if len(set.Keys) == 0 {
			b.Missing[language] = missingReason(language)
			continue
		}
		b.Granularity[language] = set.Files[set.Keys[0]].Granularity
		for _, k := range set.Keys {
			if _, dup := b.Files[k]; !dup {
				b.Keys = append(b.Keys, k)
				b.Files[k] = set.Files[k]
			}
		}
	}
	return b, nil
}

// missingReason: tarpaulin is what crap runs when cargo-llvm-cov is absent.
func missingReason(language string) string {
	if language == "rust" {
		if _, err := exec.LookPath("cargo-llvm-cov"); err != nil {
			if _, err := exec.LookPath("cargo-tarpaulin"); err == nil {
				return MissingTarpaulin
			}
		}
	}
	return MissingRunTest
}

// testBranches reads `coderefinery test` outputs (.metrics/tests/<lang>/**).
func testBranches(dir, language string) (*BranchSet, error) {
	set := &BranchSet{}
	if !discover.IsDir(dir) {
		return set, nil
	}
	var err error
	switch language {
	case "python":
		err = eachFile(recursiveGlob(dir, "coverage.json"), func(data []byte) error {
			got, err := ParseCoveragePyJSON(data)
			set.merge(got)
			return err
		})
	case "go":
		set, err = goBranches(recursiveGlob(dir, "coverage.out"))
	case "typescript":
		err = eachFile(recursiveGlob(dir, "coverage-final.json"), func(data []byte) error {
			got, err := ParseIstanbul(data)
			set.merge(got)
			return err
		})
		if err == nil && len(set.Keys) == 0 {
			set = lcovBranches(recursiveGlob(dir, "lcov.info")).only(language)
		}
	case "java":
		set, err = jacocoBranches(recursiveGlob(dir, "jacoco.xml"))
	case "rust":
		err = eachFile(recursiveGlob(dir, "llvm-cov.json"), func(data []byte) error {
			got, err := ParseLlvmCov(data)
			set.merge(got)
			return err
		})
		if err == nil && len(set.Keys) == 0 {
			set = lcovBranches(recursiveGlob(dir, "lcov.info")).only(language)
		}
	case "clojure":
		set = formBranches(dir)
	}
	return set, err
}

// reportBranches reads the CRAP coverage reports (coverage.LoadBundle's paths).
func reportBranches(root, language string, lcov func() *BranchSet) (*BranchSet, error) {
	switch language {
	case "go":
		paths := []string{
			filepath.Join(root, "target", "coverage", "coverage.out"),
			filepath.Join(root, "target", "coverage", "go", "coverage.out"),
			filepath.Join(root, "coverage.out"),
		}
		paths = append(paths, globSorted(filepath.Join(root, "*", "target", "coverage", "go", "coverage.out"))...)
		paths = append(paths, globSorted(filepath.Join(root, "*", "*", "target", "coverage", "go", "coverage.out"))...)
		return goBranches(paths)
	case "java":
		paths := []string{filepath.Join(root, "target", "site", "jacoco", "jacoco.xml")}
		paths = append(paths, globSorted(filepath.Join(root, "*", "target", "site", "jacoco", "jacoco.xml"))...)
		paths = append(paths, globSorted(filepath.Join(root, "*", "*", "target", "site", "jacoco", "jacoco.xml"))...)
		return jacocoBranches(paths)
	case "clojure":
		return formBranches(filepath.Join(root, "target", "coverage")), nil
	}
	return lcov().only(language), nil
}

func (s *BranchSet) only(language string) *BranchSet {
	out := &BranchSet{}
	for _, k := range s.Keys {
		if s.Files[k].Language == language {
			out.add(k, s.Files[k])
		}
	}
	return out
}

// eachFile calls fn with the bytes of each distinct readable file.
func eachFile(paths []string, fn func([]byte) error) error {
	seen := map[string]bool{}
	for _, p := range paths {
		r := discover.Resolve(p)
		if seen[r] || !discover.IsFile(p) {
			continue
		}
		seen[r] = true
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := fn(data); err != nil {
			return err
		}
	}
	return nil
}

func goBranches(paths []string) (*BranchSet, error) {
	set := &BranchSet{}
	err := eachFile(paths, func(data []byte) error {
		got, err := ParseGoBlocks(string(data))
		set.merge(got)
		return err
	})
	return set, err
}

func jacocoBranches(paths []string) (*BranchSet, error) {
	set := &BranchSet{}
	err := eachFile(paths, func(data []byte) error {
		got, err := ParseJacocoLines(string(data))
		set.merge(got)
		return err
	})
	return set, err
}

func lcovBranches(paths []string) *BranchSet {
	set := &BranchSet{}
	for _, text := range mergeTexts(paths) {
		set.merge(LcovBranches(ParseLcov(text)))
	}
	return set
}

// formBranches reads Cloverage HTML under dir; keys are the paths below the
// report dir ("html/" dropped) without ".html", like coverage.LoadBundle.
func formBranches(dir string) *BranchSet {
	set := &BranchSet{}
	if !discover.IsDir(dir) {
		return set
	}
	for _, p := range recursiveGlobSuffix(dir, ".html") {
		text, ok := readText(p)
		if !ok || !strings.Contains(text, "forms covered") {
			continue
		}
		rel, _ := filepath.Rel(dir, p)
		rel = strings.TrimSuffix(filepath.ToSlash(rel), ".html")
		if i := strings.Index(rel, "html/"); i >= 0 && (i == 0 || rel[i-1] == '/') {
			rel = rel[i+len("html/"):]
		}
		set.add(rel, FormBranches(ParseFormCoverage(text)))
	}
	return set
}

// FormBranches turns Cloverage line counts into form ArmHits.
func FormBranches(forms FormMap) *BranchFile {
	f := &BranchFile{Language: "clojure", Granularity: GranForm}
	for _, line := range sortedLines(forms) {
		h := forms[line]
		f.Arms = append(f.Arms, ArmHit{Line: line, Kind: KindForm, Of: h.Total, Hits: h.Covered, Reached: h.Covered})
	}
	return f
}

// LcovBranches keeps the LCOV records that carry BRDA: one line-count per
// line (Of = arms, Hits = arms taken, Reached = the line's DA hit flag).
func LcovBranches(o *OrderedLcov) *BranchSet {
	set := &BranchSet{}
	for _, key := range o.Keys {
		rec := o.Records[key]
		if len(rec.Branches) == 0 {
			continue
		}
		f := &BranchFile{Language: discover.LanguageOf(NormalizePath(key)), Granularity: GranBranchCount}
		for _, line := range sortedLines(rec.Branches) {
			h := rec.Branches[line]
			reached := -1
			if da, ok := rec.Lines[line]; ok {
				reached = da.Covered
			}
			f.Arms = append(f.Arms, ArmHit{Line: line, Kind: KindLineCount, Of: h.Total, Hits: h.Covered, Reached: reached})
		}
		set.add(key, f)
	}
	return set
}

func sortedLines(m map[int]Hits) []int {
	lines := make([]int, 0, len(m))
	for l := range m {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	return lines
}
