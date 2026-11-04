package dry

import (
	"fmt"
	"math"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"testing"
)

// Interning, merge Jaccard and size-bound pruning: the pair loop must give
// exactly what upstream's nested loop over printed fingerprint sets gives.

// nestedLoopDuplicates is upstream scan.find_duplicates verbatim (every
// pair, set-probe Jaccard, stable sort): the reference FindDuplicates is
// checked against.
func nestedLoopDuplicates(entries []Entry, threshold float64) []Duplicate {
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
				in := map[uint32]bool{}
				for _, id := range left.Fingerprints {
					in[id] = true
				}
				shared := 0
				for _, id := range right.Fingerprints {
					if in[id] {
						shared++
					}
				}
				score := 0.0
				if union := len(left.Fingerprints) + len(right.Fingerprints) - shared; union != 0 {
					score = float64(shared) / float64(union)
				}
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

func ids(xs ...uint32) []uint32 { return xs }

func idRange(from, to uint32) []uint32 {
	var out []uint32
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}

func idEntry(file string, start int, fps []uint32) Entry {
	return Entry{Language: "go", Span: Span{file, start, start + 3}, Nodes: 20 + start, Fingerprints: fps}
}

func TestStructuralIdsCountTheSameFingerprintsAsPrintedStrings(t *testing.T) {
	call := List(K("call"), symbolOf("select"), List(K("argument_list"), K("symbol")))
	fp := NewFingerprinter(false)
	got := fp.Set(call)
	if want := ids(0, 1, 2, 3, 4, 5, 6); !slices.Equal(got, want) {
		t.Fatalf("ids %v, want %v (7 printed fingerprints: %q)", got, want, sortedFingerprints(Fingerprints(call)))
	}
	// the same subtrees inside another value get the same ids
	again := fp.Set(List(K("block"), List(K("argument_list"), K("symbol")), symbolOf("select")))
	if want := ids(1, 2, 3, 4, 5, 7, 8); !slices.Equal(again, want) {
		t.Fatalf("second value ids %v, want %v", again, want)
	}
	if shared, ok := countShared(got, again, 0); !ok || shared != 5 {
		t.Fatalf("shared %d (%v), want 5: :symbol \"select\" [:symbol \"select\"] :argument_list [:argument_list :symbol]", shared, ok)
	}
	// a keyword and a string with the same text print differently (:x vs "x")
	if k, s := fp.Set(K("x")), fp.Set(Str("x")); slices.Equal(k, s) {
		t.Fatalf("keyword x and string x share id %v", k)
	}
	if fp.ambiguous {
		t.Fatal("ordinary node types flagged as ambiguous")
	}
}

func TestStructuralAndPrintedFingerprintsScoreAlike(t *testing.T) {
	values := []*Value{
		List(K("call"), symbolOf("select"), List(K("argument_list"), K("symbol"))),
		List(K("call"), symbolOf("filter"), List(K("argument_list"), K("symbol"), K("literal"))),
		List(K("block"), List(K("argument_list"), K("symbol")), List(K("argument_list"), K("symbol"))),
		List(K("block"), List(K("call"), symbolOf("select"), List(K("argument_list"), K("symbol")))),
		List(K("list"), Str(`a"b\c`), Str(`a\"b`), K("literal")),
		List(),
	}
	byStructure, byString := NewFingerprinter(false), NewFingerprinter(true)
	for i, a := range values {
		for _, b := range values[i:] {
			sa, sb := byStructure.Set(a), byStructure.Set(b)
			pa, pb := byString.Set(a), byString.Set(b)
			shared, _ := countShared(sa, sb, 0)
			printedShared, _ := countShared(pa, pb, 0)
			if len(sa) != len(Fingerprints(a)) || shared != printedShared {
				t.Errorf("%s ~ %s: structure %d ids / %d shared, printed %d / %d", Pr(a), Pr(b), len(sa), shared,
					len(pa), printedShared)
			}
		}
	}
}

// A keyword name holding " " or "]" prints like a different list, so
// structural ids would miss a shared fingerprint; the interner says so and
// ScanFiles falls back to printed strings for that language.
func TestAKeywordThatPrintsLikeAListIsAmbiguous(t *testing.T) {
	two, glued := List(K("a"), K("b")), List(K("a :b"))
	if Pr(two) != Pr(glued) {
		t.Fatalf("%s vs %s", Pr(two), Pr(glued))
	}
	fp := NewFingerprinter(false)
	fp.Set(two)
	if fp.ambiguous {
		t.Fatal("[:a :b] flagged")
	}
	fp.Set(glued)
	if !fp.ambiguous {
		t.Fatal("keyword \"a :b\" not flagged")
	}
	byString := NewFingerprinter(true)
	l, r := byString.Set(two), byString.Set(glued)
	shared, _ := countShared(l, r, 0)
	if got := jaccard(shared, len(l), len(r)); got != 0.25 {
		t.Fatalf("printed jaccard %v, want 1/4 ([:a :b] shared out of :a :b :a\\ :b [:a :b])", got)
	}
}

func TestScanningByStructureEqualsScanningByString(t *testing.T) {
	needGrammars(t, "go", "python", "typescript")
	fixtures := filepath.Join("..", "..", "fixtures")
	var files []string
	for _, root := range []string{filepath.Join("testdata", "corpus"), filepath.Join(fixtures, "go"),
		filepath.Join(fixtures, "python", "src"), filepath.Join(fixtures, "typescript", "src"), filepath.Join("..", "syntax")} {
		files = append(files, allFiles(t, root)...)
	}
	structural, warnings, err := ScanFiles(files, ".", 1, 1)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("scan: %v %v", err, warnings)
	}
	all := map[string]bool{"go": true, "python": true, "typescript": true, "clojure": true}
	printed, _, ambiguous, err := scanFiles(files, ".", 1, 1, all)
	if err != nil || len(ambiguous) != 0 {
		t.Fatalf("by string: %v, ambiguous %v", err, ambiguous)
	}
	languages := map[string]int{}
	for _, e := range structural {
		languages[e.Language]++
	}
	if len(languages) != 4 || len(structural) != len(printed) {
		t.Fatalf("languages %v, %d vs %d entries", languages, len(structural), len(printed))
	}
	for _, threshold := range []float64{0.3, 0.82} {
		got := FindDuplicates(structural, threshold)
		if want := FindDuplicates(printed, threshold); !reflect.DeepEqual(got, want) {
			t.Fatalf("threshold %v: structural %d pairs, printed %d", threshold, len(got), len(want))
		}
		if want := nestedLoopDuplicates(printed, threshold); !reflect.DeepEqual(got, want) {
			t.Fatalf("threshold %v: %d pairs, nested loop %d", threshold, len(got), len(want))
		}
		if threshold == 0.3 && len(got) < 100 {
			t.Fatalf("only %d pairs at 0.3; the corpus no longer exercises pairing", len(got))
		}
	}
}

func TestSharedNeededIsTheExactBoundary(t *testing.T) {
	thresholds := []float64{0.82, 23.0 / 28, math.Nextafter(23.0/28, 0), math.Nextafter(23.0/28, 1), 0.8,
		math.Nextafter(0.8, 1), 1.0 / 3, 1, 0, -1, 1.5, math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, threshold := range thresholds {
		for large := 0; large <= 60; large++ {
			for small := 0; small <= large; small++ {
				want := -1
				for s := 0; s <= small; s++ {
					if !(jaccard(s, small, large) < threshold) {
						want = s
						break
					}
				}
				got, ok := sharedNeeded(small, large, threshold)
				if ok != (want >= 0) || (ok && got != want) {
					t.Fatalf("threshold %v, sizes %d/%d: got %d %v, want %d", threshold, small, large, got, ok, want)
				}
			}
		}
	}
}

func TestCountSharedStopsOnceTheNeedIsOutOfReach(t *testing.T) {
	left, right := ids(1, 2, 3, 4, 9), ids(1, 2, 5, 6, 9)
	if shared, ok := countShared(left, right, 3); !ok || shared != 3 {
		t.Fatalf("need 3: %d %v", shared, ok)
	}
	if _, ok := countShared(left, right, 4); ok {
		t.Fatal("need 4 reported reachable with 3 shared")
	}
	if shared, ok := countShared(nil, right, 0); !ok || shared != 0 {
		t.Fatalf("empty left: %d %v", shared, ok)
	}
}

func TestAPairExactlyAtTheThresholdIsReported(t *testing.T) {
	// 23 shared, 2 + 3 own: 23/28 = 0.821428571429, the TS fixture's borderline score
	left := idEntry("a.go", 1, append(idRange(0, 23), 100, 101))
	right := idEntry("b.go", 1, append(idRange(0, 23), 200, 201, 202))
	at := FindDuplicates([]Entry{left, right}, 23.0/28)
	if len(at) != 1 || at[0].Score != 23.0/28 || ednFloat(at[0].Score) != "0.821428571429" ||
		at[0].Left != left.Span || at[0].Right != right.Span || at[0].LeftNodes != 21 || at[0].RightNodes != 21 {
		t.Fatalf("at 23/28: %+v", at)
	}
	if above := FindDuplicates([]Entry{left, right}, math.Nextafter(23.0/28, 1)); len(above) != 0 {
		t.Fatalf("one ulp above 23/28: %+v", above)
	}
	if below := FindDuplicates([]Entry{left, right}, 0.82); len(below) != 1 {
		t.Fatalf("at the default 0.82: %+v", below)
	}
}

func TestTheSizeBoundKeepsASubsetAtTheThreshold(t *testing.T) {
	// 4 ids inside 5: the score is the size bound itself, 4/5, equal to the
	// parsed --threshold 0.8
	small, large := idEntry("a.go", 1, idRange(0, 4)), idEntry("b.go", 1, idRange(0, 5))
	for _, order := range [][]Entry{{small, large}, {large, small}} {
		got := FindDuplicates(order, 0.8)
		if len(got) != 1 || got[0].Score != 0.8 || got[0].Left != order[0].Span || got[0].Right != order[1].Span {
			t.Fatalf("at 0.8: %+v", got)
		}
		if got := FindDuplicates(order, math.Nextafter(0.8, 1)); len(got) != 0 {
			t.Fatalf("one ulp above 0.8: %+v", got)
		}
	}
}

func TestFindDuplicatesMatchesTheNestedLoop(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4)) // several workers even on a one-core runner
	rng := rand.New(rand.NewPCG(7, 11))
	var entries []Entry
	// near copies of a few base sets, so every threshold has hits
	var bases [][]uint32
	for range 6 {
		bases = append(bases, idRange(0, uint32(8+rng.IntN(30))))
	}
	for i := range 400 {
		fps := slices.Clone(bases[rng.IntN(len(bases))])
		for range rng.IntN(4) {
			fps[rng.IntN(len(fps))] = uint32(100 + rng.IntN(20))
		}
		// few files and start lines: many (file, start) sort keys tie, so the
		// result also checks that ties keep enumeration order
		start := 1 + rng.IntN(4)
		entries = append(entries, Entry{Language: []string{"go", "python"}[i%2],
			Span:  Span{fmt.Sprintf("f%d.x", rng.IntN(3)), start, start + rng.IntN(3)},
			Nodes: 20 + rng.IntN(50), Fingerprints: sortedUnique(fps)})
	}
	for _, threshold := range []float64{0.6, 0.82, 1, math.NaN()} {
		got := FindDuplicates(entries, threshold)
		want := nestedLoopDuplicates(entries, threshold)
		t.Logf("threshold %v: %d pairs", threshold, len(want))
		if len(want) == 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("threshold %v: %d pairs, nested loop %d", threshold, len(got), len(want))
		}
		if math.IsNaN(threshold) && len(got) != distinctSpanPairs(entries) {
			t.Fatalf("NaN threshold reported %d pairs, want every pair of distinct spans (%d)", len(got), distinctSpanPairs(entries))
		}
	}
}

func distinctSpanPairs(entries []Entry) int {
	n := 0
	for i, a := range entries {
		for _, b := range entries[i+1:] {
			if a.Language == b.Language && a.Span != b.Span {
				n++
			}
		}
	}
	return n
}
