package coverage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/model"
)

func fn(lang, path string, start, end int) model.Function {
	return model.Function{Name: "f", Namespace: "n", Language: lang, Path: path, StartLine: start, EndLine: end}
}

func pct(t *testing.T, b *Bundle, f model.Function) (float64, bool) {
	t.Helper()
	return b.PercentFor(f)
}

func lcovBundle(text string) *Bundle {
	b := NewBundle()
	b.Lcov = ParseLcov(text)
	return b
}

// Ported from upstream tests/test_coverage.py (LCOV / key matching parts)
// and spec-crapper §8.7.
func TestLcovLinePercentAndMissingFile(t *testing.T) {
	b := lcovBundle("SF:src/demo/core.py\nDA:3,1\nDA:4,0\nend_of_record\n")
	if got, ok := pct(t, b, fn("python", "src/demo/core.py", 3, 4)); !ok || got != 50.0 {
		t.Fatalf("got %v %v, want 50", got, ok)
	}
	if _, ok := pct(t, b, fn("python", "src/demo/other.py", 3, 4)); ok {
		t.Fatal("missing file must be nil")
	}
}

func TestLcovBranchesWinWhenTheSpanHasThem(t *testing.T) {
	rec := ParseLcov("SF:a.py\nDA:2,1\nDA:3,1\nBRDA:3,0,0,1\nBRDA:3,0,1,-\nend_of_record\n").Records["a.py"]
	if !reflect.DeepEqual(rec.Lines, map[int]Hits{2: {1, 1}, 3: {1, 1}}) || !reflect.DeepEqual(rec.Branches, map[int]Hits{3: {1, 2}}) {
		t.Fatalf("record %+v", rec)
	}
	b := lcovBundle("SF:a.py\nDA:2,1\nDA:3,1\nBRDA:3,0,0,1\nBRDA:3,0,1,-\nend_of_record\n")
	if got, _ := pct(t, b, fn("python", "a.py", 2, 3)); got != 50.0 {
		t.Fatalf("branch percent %v", got)
	}
	b = lcovBundle("SF:a.py\nDA:2,1\nDA:3,0\nBRDA:8,0,0,1\nBRDA:8,0,1,0\nend_of_record\n")
	if got, _ := pct(t, b, fn("python", "a.py", 2, 3)); got != 50.0 {
		t.Fatalf("branchless span keeps lines: %v", got)
	}
}

func TestLcovWithoutEndOfRecordAndRepeatedSF(t *testing.T) {
	o := ParseLcov("SF:a.clj\nDA:1,1\nSF:b.clj\nDA:2,0\n")
	if !reflect.DeepEqual(o.Keys, []string{"a.clj", "b.clj"}) || o.Records["a.clj"].Lines[1] != (Hits{1, 1}) || o.Records["b.clj"].Lines[2] != (Hits{0, 1}) {
		t.Fatalf("parsed %+v", o)
	}
	o = ParseLcov("SF:a\nDA:1,1\nend_of_record\nSF:b\nend_of_record\nSF:a\nDA:9,0\nend_of_record\n")
	if !reflect.DeepEqual(o.Keys, []string{"a", "b"}) || len(o.Records["a"].Lines) != 1 || o.Records["a"].Lines[9] != (Hits{0, 1}) {
		t.Fatalf("repeated SF must replace in place: %+v", o.Records["a"])
	}
}

func TestSelectKey(t *testing.T) {
	cases := []struct {
		keys  []string
		src   string
		bound [][]string
		want  string
	}{
		{[]string{"other.go", "board.go"}, "board.go", nil, "board.go"},
		{[]string{"other/proj/src/a.go", "proj/src/a.go"}, "src/a.go", nil, "proj/src/a.go"},
		{[]string{"proj/a.go", "other/a.go"}, "a.go", nil, ""},
		{[]string{"a/b/main.go", "b/main.go"}, "b/main.go", nil, "b/main.go"},
		{[]string{"a/b/main.go"}, "b/main.go", [][]string{{"b", "main.go"}, {"a", "b", "main.go"}}, ""},
		{[]string{"a/b/main.go"}, "a/b/main.go", [][]string{{"b", "main.go"}, {"a", "b", "main.go"}}, "a/b/main.go"},
		{[]string{"board.go"}, "cmd/board.go", [][]string{{"cmd", "board.go"}}, "board.go"},
		{[]string{"board.go"}, "cmd/board.go", [][]string{{"cmd", "board.go"}, {"other", "board.go"}}, ""},
		{[]string{"proj/src/demo/core.clj"}, "src/demo/core.clj", nil, "proj/src/demo/core.clj"},
	}
	for _, c := range cases {
		got, _ := SelectKey(c.keys, c.src, c.bound)
		if got != c.want {
			t.Errorf("SelectKey(%v, %q) = %q, want %q", c.keys, c.src, got, c.want)
		}
	}
}

func TestNormalizePathAndSuffixMatch(t *testing.T) {
	if got := NormalizePath("file:./src//demo/core.clj"); got != "src/demo/core.clj" {
		t.Errorf("got %q", got)
	}
	if got := NormalizePath("%2E%2Fsrc%2Fcore.clj"); got != "src/core.clj" {
		t.Errorf("got %q", got)
	}
	if !SuffixMatch("src/demo/core.clj", "demo/core.clj") || SuffixMatch("core.clj", "src/demo/core.clj") || SuffixMatch("src/demo/core.clj", "") {
		t.Error("suffix_match")
	}
}

func TestPercentHelpers(t *testing.T) {
	if Percent(0, 0) != 0.0 || PercentForRange(map[int]Hits{}, 1, 3) != 0.0 || PercentForRange(map[int]Hits{1: {1, 1}}, 4, 5) != 0.0 {
		t.Fatal("zero totals score 0.0")
	}
	if got := ParseFormCoverage(`<span class="pc" title="1 out of 2 forms covered">   4&nbsp;&nbsp;(if x 1)</span>`); !reflect.DeepEqual(got, FormMap{4: {1, 2}}) {
		t.Fatalf("forms %v", got)
	}
}

func TestGoProfile(t *testing.T) {
	prof, err := ParseGoProfile("mode: set\ngithub.com/acme/demo/board.go:4.1,6.2 2 1\ngithub.com/acme/demo/board.go:8.1,9.2 1 0\n")
	if err != nil {
		t.Fatal(err)
	}
	b := NewBundle()
	b.GoProfile = prof
	if got, _ := pct(t, b, fn("go", "board.go", 4, 6)); got != 100.0 {
		t.Errorf("4-6 = %v", got)
	}
	if got, _ := pct(t, b, fn("go", "board.go", 8, 9)); got != 0.0 {
		t.Errorf("8-9 = %v", got)
	}
	if got, ok := pct(t, b, fn("go", "board.go", 10, 12)); !ok || got != 0.0 {
		t.Errorf("no overlap = %v %v", got, ok)
	}
	if _, ok := pct(t, b, fn("go", "other.go", 1, 2)); ok {
		t.Error("other.go must be nil")
	}
	if _, err := ParseGoProfile("not a segment"); err == nil || err.Error() != "invalid coverage segment on line 1: 'not a segment'" {
		t.Errorf("err = %v", err)
	}
}

func TestJacoco(t *testing.T) {
	xmlText := `<!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd"><report name="r"><package name="demo/pkg">
<class name="demo/pkg/Board"><method name="place" line="4"><counter type="INSTRUCTION" missed="1" covered="3"/></method>
<method name="lineonly" line="5"><counter type="LINE" missed="0" covered="1"/></method>
<method name="bad" line="nope"><counter type="INSTRUCTION" covered="2"/></method>
<counter type="INSTRUCTION" missed="9" covered="9"/></class>
<class name="demo/pkg/Board$Inner"><method name="tick" line="9"><counter type="INSTRUCTION" missed="0" covered="5"/></method></class>
</package></report>`
	idx, err := ParseJacocoIndex(xmlText)
	if err != nil {
		t.Fatal(err)
	}
	want := Jacoco{"demo.pkg.Board#place": {{1, 3, 4}}, "demo.pkg.Board#bad": {{0, 2, 0}}, "demo.pkg.Board$Inner#tick": {{0, 5, 9}}}
	if !reflect.DeepEqual(idx, want) {
		t.Fatalf("index %v", idx)
	}
	b := NewBundle()
	b.Jacoco = idx
	java := model.Function{Name: "place", Namespace: "demo.pkg.Board", Language: "java", StartLine: 4}
	if got, _ := b.PercentFor(java); got != 75.0 {
		t.Errorf("place = %v", got)
	}
	if _, err := ParseJacocoIndex("<report><class"); err == nil {
		t.Error("malformed XML must fail")
	}
}

func TestLoadBundleReadsLcovProfilesJacocoAndForms(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("target/coverage/lcov.info", "SF:src/demo/core.clj\nDA:3,1\nDA:4,0\nend_of_record\n")
	write("target/coverage/go/coverage.out", "mode: set\nx/a.go:1.1,2.2 1 1\n")
	write("target/site/jacoco/jacoco.xml", `<report><class name="a/B"><method name="m" line="1"><counter type="INSTRUCTION" missed="0" covered="1"/></method></class></report>`)
	write("target/coverage/src/demo/core.clj.html", `<span title="1 out of 2 forms covered">4&nbsp;</span>`)
	write("target/coverage/index.html", "<html>no forms here</html>")
	b, err := LoadBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	if b.GoProfile == nil || b.Jacoco == nil || !reflect.DeepEqual(b.FormKeys, []string{"src/demo/core.clj"}) {
		t.Fatalf("bundle %+v", b)
	}
	if got, _ := b.PercentFor(fn("clojure", "src/demo/core.clj", 3, 4)); got != 50.0 {
		t.Fatalf("form map wins: %v", got)
	}
	empty, _ := LoadBundle(t.TempDir())
	if empty.GoProfile != nil || empty.Jacoco != nil || len(empty.Lcov.Keys) != 0 {
		t.Fatal("empty tree")
	}
}

func TestSplitLinesMatchesPython(t *testing.T) {
	got := SplitLines("a\nb\r\nc\rd\x0be\x1cf g\n")
	want := []string{"a", "b", "c", "d", "e", "f", "g"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}
