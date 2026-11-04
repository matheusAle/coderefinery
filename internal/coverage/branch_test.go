package coverage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The samples under testdata/branches: coverage.json (coverage.py 7.17
// --branch on internal/crap/testdata/branches_python), coverage.out (go test
// -covermode=count on fixtures/go), v8-/ast-coverage-final.json and lcov.info
// (Vitest 3.2.7 on fixtures/typescript, default and AST-aware remapping;
// src/inventory.ts only) are real reports. jacoco.xml, llvm-cov-*.json and
// clojure/core.clj.html are written by hand in the tools' formats (no
// Maven, cargo-llvm-cov or Clojure toolchain here).

func sample(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "branches", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func scratchDir(t *testing.T) string {
	t.Helper()
	if local := os.Getenv("REPO_LOCAL"); local != "" {
		base := filepath.Join(local, "scratch", "gotest")
		if err := os.MkdirAll(base, 0o777); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TMPDIR", base)
	}
	return t.TempDir()
}

func missed(f *BranchFile) []ArmHit {
	var out []ArmHit
	for _, a := range f.Arms {
		if a.Hits < max(1, a.Of) && (a.Kind == KindLineCount || a.Kind == KindForm) || a.Hits == 0 && a.Kind != KindLineCount && a.Kind != KindForm {
			out = append(out, a)
		}
	}
	return out
}

func TestCoveragePyArcsKeepReachedFlag(t *testing.T) {
	set, err := ParseCoveragePyJSON(sample(t, "coverage.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set.Keys, []string{"billing/pricing.py"}) {
		t.Fatalf("keys %v", set.Keys)
	}
	f := set.Files["billing/pricing.py"]
	if f.Granularity != GranBranch || f.Language != "python" {
		t.Fatalf("file %+v", f)
	}
	want := []ArmHit{
		{Line: 7, FromLine: 7, ToLine: 8, Kind: KindArc, Reached: 1},
		{Line: 16, FromLine: 16, ToLine: 17, Kind: KindArc, Reached: 1},
		{Line: 21, FromLine: 21, ToLine: 22, Kind: KindArc, Reached: 1},
		{Line: 30, FromLine: 30, ToLine: 31, Kind: KindArc, Reached: 1},
		{Line: 37, FromLine: 37, ToLine: 38, Kind: KindArc, Reached: 0},
		{Line: 37, FromLine: 37, ToLine: 39, Kind: KindArc, Reached: 0},
	}
	if got := missed(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing arcs\n got %+v\nwant %+v", got, want)
	}
	if len(f.Arms) <= len(want) {
		t.Fatalf("executed arcs dropped: %d arms", len(f.Arms))
	}
}

func TestCoveragePyWithoutBranchIsEmpty(t *testing.T) {
	set, err := ParseCoveragePyJSON([]byte(`{"meta":{"branch_coverage":false},"files":{"a.py":{"executed_lines":[1]}}}`))
	if err != nil || len(set.Keys) != 0 {
		t.Fatalf("set %+v err %v", set, err)
	}
	if _, err := ParseCoveragePyJSON([]byte(`{`)); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}

func TestGoBlocksKeepColumnsAndAddRepeats(t *testing.T) {
	set, err := ParseGoBlocks(string(sample(t, "coverage.out")))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set.Keys, []string{"example.com/ubfix/calc.go", "example.com/ubfix/geometry/shapes.go"}) {
		t.Fatalf("keys %v", set.Keys)
	}
	calc := set.Files["example.com/ubfix/calc.go"]
	if calc.Granularity != GranBlock || len(calc.Arms) != 26 {
		t.Fatalf("calc: %s, %d blocks", calc.Granularity, len(calc.Arms))
	}
	if got, want := calc.Arms[6], (ArmHit{Line: 21, Column: 16, EndLine: 23, EndColumn: 3, Kind: KindBlock, Reached: -1}); got != want {
		t.Fatalf("block 7 %+v, want %+v", got, want)
	}
	twice, err := ParseGoBlocks("mode: count\nm/a.go:1.2,3.4 1 2\nm/a.go:1.2,3.4 1 5\n")
	if err != nil || twice.Files["m/a.go"].Arms[0].Hits != 7 || len(twice.Files["m/a.go"].Arms) != 1 {
		t.Fatalf("repeat %+v %v", twice.Files["m/a.go"], err)
	}
	if _, err := ParseGoBlocks("m/a.go:1.2 1\n"); err == nil {
		t.Fatal("malformed segment accepted")
	}
}

func TestIstanbulTypedEntriesAreArms(t *testing.T) {
	set, err := ParseIstanbul(sample(t, "ast-coverage-final.json"))
	if err != nil {
		t.Fatal(err)
	}
	f := set.Files["/project/src/inventory.ts"]
	if f == nil || f.Granularity != GranBranch || f.Language != "typescript" {
		t.Fatalf("file %+v", f)
	}
	want := []ArmHit{
		{Line: 39, Column: 5, Index: 0, Of: 2, Kind: "if"},
		{Line: 39, Column: 5, Index: 1, Of: 2, Kind: "if"},
		{Line: 39, Column: 9, Index: 0, Of: 2, Kind: "binary-expr"},
		{Line: 39, Column: 9, Index: 1, Of: 2, Kind: "binary-expr"},
		{Line: 40, Column: 19, Index: 0, Of: 2, Kind: "binary-expr"},
		{Line: 40, Column: 19, Index: 1, Of: 2, Kind: "binary-expr"},
		{Line: 47, Column: 5, Index: 0, Of: 2, Kind: "if", Reached: 2},
		{Line: 55, Column: 10, Index: 1, Of: 2, Kind: "cond-expr", Reached: 1},
		{Line: 59, Column: 3, Index: 1, Of: 2, Kind: "cond-expr", Reached: 1},
		{Line: 65, Column: 5, Index: 1, Of: 4, Kind: "switch", Reached: 2},
		{Line: 66, Column: 5, Index: 2, Of: 4, Kind: "switch", Reached: 2},
	}
	if got := missed(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("missed arms\n got %+v\nwant %+v", got, want)
	}
}

func TestIstanbulV8RangesAreCountedPerLine(t *testing.T) {
	set, err := ParseIstanbul(sample(t, "v8-coverage-final.json"))
	if err != nil {
		t.Fatal(err)
	}
	f := set.Files["/project/src/inventory.ts"]
	if f.Granularity != GranBranchCount {
		t.Fatalf("granularity %s", f.Granularity)
	}
	want := []ArmHit{
		{Line: 47, Kind: KindLineCount, Of: 1, Reached: 2},
		{Line: 55, Kind: KindLineCount, Of: 1, Reached: 1},
		{Line: 59, Kind: KindLineCount, Of: 1, Reached: 1},
		{Line: 65, Kind: KindLineCount, Of: 1, Reached: 2},
		{Line: 66, Kind: KindLineCount, Of: 1, Reached: 2},
	}
	if got := missed(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("missed lines\n got %+v\nwant %+v", got, want)
	}
}

func TestLcovBranchesNeedBRDA(t *testing.T) {
	set := LcovBranches(ParseLcov(string(sample(t, "lcov.info")) + "SF:src/plain.py\nDA:1,1\nend_of_record\n"))
	if !reflect.DeepEqual(set.Keys, []string{"src/inventory.ts"}) {
		t.Fatalf("keys %v", set.Keys)
	}
	f := set.Files["src/inventory.ts"]
	if f.Granularity != GranBranchCount || f.Language != "typescript" {
		t.Fatalf("file %+v", f)
	}
	got := missed(f)
	if len(got) != 5 || got[0] != (ArmHit{Line: 47, Kind: KindLineCount, Of: 1, Reached: 1}) {
		t.Fatalf("missed %+v", got)
	}
}

func TestJacocoLinesCountBranches(t *testing.T) {
	set, err := ParseJacocoLines(string(sample(t, "jacoco.xml")))
	if err != nil {
		t.Fatal(err)
	}
	f := set.Files["demo/Board.java"]
	if f == nil || f.Granularity != GranBranchCount {
		t.Fatalf("keys %v", set.Keys)
	}
	want := []ArmHit{
		{Line: 18, Kind: KindLineCount, Of: 2, Hits: 1, Reached: 3},
		{Line: 26, Kind: KindLineCount, Of: 4, Hits: 3, Reached: 6},
		{Line: 33, Kind: KindLineCount, Of: 2, Hits: 0, Reached: 0},
		{Line: 39, Kind: KindLineCount, Of: 3, Hits: 2, Reached: 6},
		{Line: 42, Kind: KindLineCount, Of: 2, Hits: 1, Reached: 4},
		{Line: 52, Kind: KindLineCount, Of: 4, Hits: 3, Reached: 4},
	}
	if got := missed(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("missed\n got %+v\nwant %+v", got, want)
	}
}

func TestLlvmRegionsAndBranches(t *testing.T) {
	regions, err := ParseLlvmCov(sample(t, "llvm-cov-regions.json"))
	if err != nil {
		t.Fatal(err)
	}
	f := regions.Files["/build/ub-demo/src/lib.rs"]
	if f.Granularity != GranRegion {
		t.Fatalf("granularity %s", f.Granularity)
	}
	// the gap region (kind 3) is dropped; run's two instantiations add up
	if len(f.Arms) != 10 || f.Arms[8] != (ArmHit{Line: 19, Column: 20, EndLine: 19, EndColumn: 25, Kind: KindRegion, Hits: 3, Reached: -1}) {
		t.Fatalf("regions %+v", f.Arms)
	}
	branches, err := ParseLlvmCov(sample(t, "llvm-cov-branches.json"))
	if err != nil {
		t.Fatal(err)
	}
	b := branches.Files["/build/ub-demo/src/lib.rs"]
	want := []ArmHit{
		{Line: 4, Column: 8, EndLine: 4, EndColumn: 13, Index: 1, Of: 2, Kind: KindLLVM, Reached: 1},
		{Line: 4, Column: 17, EndLine: 4, EndColumn: 23, Index: 0, Of: 2, Kind: KindLLVM, Reached: 1},
		{Line: 19, Column: 12, EndLine: 19, EndColumn: 19, Index: 1, Of: 2, Kind: KindLLVM, Reached: 2},
	}
	if b.Granularity != GranBranch || !reflect.DeepEqual(missed(b), want) {
		t.Fatalf("branches %s %+v", b.Granularity, missed(b))
	}
}

func TestCloverageFormsPerLine(t *testing.T) {
	f := FormBranches(ParseFormCoverage(string(sample(t, "clojure/core.clj.html"))))
	want := []ArmHit{
		{Line: 6, Kind: KindForm, Of: 4, Hits: 2, Reached: 2},
		{Line: 11, Kind: KindForm, Of: 4, Hits: 3, Reached: 3},
		{Line: 12, Kind: KindForm, Of: 2},
		{Line: 14, Kind: KindForm, Of: 1},
		{Line: 15, Kind: KindForm, Of: 4},
	}
	if f.Granularity != GranForm || !reflect.DeepEqual(missed(f), want) {
		t.Fatalf("forms %s %+v", f.Granularity, missed(f))
	}
}

func write(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o666); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBranchesPrefersTestOutputsAndNamesMissing(t *testing.T) {
	root := scratchDir(t)
	// crap's Python LCOV has no BRDA; the go profile is crap's; the
	// coderefinery test JSON for typescript wins over crap's LCOV.
	write(t, root, "target/coverage/python/lcov.info", []byte("SF:src/a.py\nDA:1,1\nend_of_record\n"))
	write(t, root, "target/coverage/go/coverage.out", sample(t, "coverage.out"))
	write(t, root, "target/coverage/typescript/lcov.info", sample(t, "lcov.info"))
	write(t, root, ".metrics/tests/typescript/coverage/coverage-final.json", sample(t, "ast-coverage-final.json"))
	write(t, root, "target/site/jacoco/jacoco.xml", sample(t, "jacoco.xml"))
	b, err := LoadBranches(root)
	if err != nil {
		t.Fatal(err)
	}
	wantGran := map[string]string{"go": GranBlock, "typescript": GranBranch, "java": GranBranchCount}
	if !reflect.DeepEqual(b.Granularity, wantGran) {
		t.Fatalf("granularity %v", b.Granularity)
	}
	for _, l := range []string{"python", "clojure"} {
		if b.Missing[l] != MissingRunTest {
			t.Errorf("missing %s = %q", l, b.Missing[l])
		}
	}
	if b.File("src/inventory.ts", "typescript", nil) != b.Files["/project/src/inventory.ts"] {
		t.Fatal("typescript key not matched")
	}
	if b.File("calc.go", "go", nil) == nil || b.File("calc.go", "python", nil) != nil {
		t.Fatal("go key lookup")
	}
	if b.File("src/main/java/demo/Board.java", "java", nil) == nil {
		t.Fatal("jacoco key not matched")
	}
	// a malformed report is an error, as for coverage.LoadBundle
	write(t, root, ".metrics/tests/python/coverage/coverage.json", []byte("{"))
	if _, err := LoadBranches(root); err == nil {
		t.Fatal("malformed coverage.json accepted")
	}
}

func TestMissingRustNamesTarpaulin(t *testing.T) {
	bin := scratchDir(t)
	write(t, bin, "cargo-tarpaulin", []byte("#!/bin/sh\n"))
	if err := os.Chmod(filepath.Join(bin, "cargo-tarpaulin"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := missingReason("rust"); got != MissingTarpaulin {
		t.Fatalf("rust: %q", got)
	}
	if got := missingReason("python"); got != MissingRunTest {
		t.Fatalf("python: %q", got)
	}
	write(t, bin, "cargo-llvm-cov", []byte("#!/bin/sh\n"))
	_ = os.Chmod(filepath.Join(bin, "cargo-llvm-cov"), 0o755)
	if got := missingReason("rust"); got != MissingRunTest {
		t.Fatalf("rust with llvm-cov: %q", got)
	}
}
