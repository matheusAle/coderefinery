package crap

import (
	"strings"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/model"
)

func f(v float64) *float64 { return &v }

func entry(name string, crap *float64, cc int) model.Entry {
	cov := f(100.0)
	if crap == nil {
		cov = nil
	}
	return model.Entry{Name: name, Namespace: "demo.core", Complexity: cc, Coverage: cov, Crap: crap,
		Path: "src/demo/core.clj", Language: "clojure"}
}

func names(rows []model.Entry) string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return strings.Join(out, ",")
}

// Ported from upstream tests/test_crap.py.
func TestScoreMatchesCrap4clj(t *testing.T) {
	cases := []struct {
		cc   int
		cov  float64
		want float64
	}{{5, 100, 5.0}, {5, 0, 30.0}, {1, 100, 1.0}}
	for _, c := range cases {
		if got := *Score(c.cc, f(c.cov)); got != c.want {
			t.Errorf("Score(%d, %v) = %v, want %v", c.cc, c.cov, got, c.want)
		}
	}
	if got := *Score(8, f(45)); got < 18.638 || got > 18.658 {
		t.Errorf("Score(8, 45) = %v, want ≈18.648", got)
	}
	if Score(3, nil) != nil {
		t.Error("Score with nil coverage must be nil")
	}
}

func TestMakeEntryUsesRoundedCoverage(t *testing.T) {
	e := MakeEntry(model.Function{Name: "choose", Namespace: "demo.box", Complexity: 4, Language: "python"}, f(200.0/3.0))
	if *e.Coverage != 66.6667 || *e.Crap != 4.5926 {
		t.Fatalf("coverage %v crap %v, want 66.6667 and 4.5926", *e.Coverage, *e.Crap)
	}
}

func TestSortEntries(t *testing.T) {
	if got := names(SortEntries([]model.Entry{entry("missing", nil, 1), entry("mid", f(5), 1), entry("worst", f(30), 1)})); got != "worst,mid,missing" {
		t.Errorf("scored before unscored: %s", got)
	}
	if got := names(SortEntries([]model.Entry{entry("plain", f(1), 1), entry("large", nil, 8)})); got != "plain,large" {
		t.Errorf("low score before unscored: %s", got)
	}
	if got := names(SortEntries([]model.Entry{entry("small", nil, 1), entry("large", nil, 8)})); got != "large,small" {
		t.Errorf("unscored by complexity: %s", got)
	}
}

func TestFormatReportShowsNAAndExactLayout(t *testing.T) {
	text := FormatReport([]model.Entry{entry("score", nil, 3)})
	want := "CRAP Report\n===========\nLanguages: clojure\n\n" +
		"Function                       Namespace                             CC    Cov%     CRAP\n" +
		strings.Repeat("-", 88) + "\n" +
		"score                          demo.core                              3   N/A       N/A\n"
	if text != want {
		t.Fatalf("report:\n%q\nwant\n%q", text, want)
	}
	empty := FormatReport(nil)
	if empty != "CRAP Report\n===========\n"+"Function                       Namespace                             CC    Cov%     CRAP\n"+strings.Repeat("-", 88)+"\n" {
		t.Fatalf("empty report: %q", empty)
	}
}

func TestRenderEDN(t *testing.T) {
	got := RenderEDN([]model.Entry{
		{Name: "layout", Namespace: "uml-viewer.layout", Complexity: 8, Coverage: f(82.0), Crap: f(3.4)},
		{Name: `q"\`, Namespace: "n", Complexity: 1},
	})
	want := "{:entries [\n" +
		"  {:name \"layout\", :namespace \"uml-viewer.layout\", :complexity 8, :coverage 82.0, :crap 3.4}\n" +
		"  {:name \"q\\\"\\\\\", :namespace \"n\", :complexity 1, :coverage nil, :crap nil}\n]}\n"
	if got != want {
		t.Fatalf("edn:\n%s\nwant\n%s", got, want)
	}
	if RenderEDN(nil) != "{:entries []}\n" {
		t.Fatal("empty edn")
	}
}
