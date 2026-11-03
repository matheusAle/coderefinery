package mutate

// Ported from upstream tests/test_boundaries.py
// (test_score_and_description_follow_the_counts,
// test_report_text_states_the_score_the_total_and_coverage).

import (
	"strings"
	"testing"
)

func form(name string, killed, survived, uncovered, sites int) FormResult {
	return FormResult{ID: "defn/" + name, Namespace: "demo", Name: name, Killed: killed, Survived: survived, Uncovered: uncovered, Sites: sites}
}

func TestScoreAndDescriptionFollowTheCounts(t *testing.T) {
	if s, ok := form("p", 1, 1, 0, 2).Score(); !ok || s != 0.5 {
		t.Fatal("1/2")
	}
	if _, ok := form("p", 0, 0, 0, 0).Score(); ok {
		t.Fatal("0/0 is None")
	}
	if (Site{Original: ">", Mutant: ">="}).Description() != "> -> >=" || (Site{Original: "not"}).Description() != "delete not" {
		t.Fatal("description")
	}
}

func TestReportTableIsByteExact(t *testing.T) {
	if formatResults(nil) != "No functions to mutate.\n" {
		t.Fatal("empty")
	}
	got := formatResults([]FormResult{form("place", 1, 1, 0, 2), form("idle", 0, 0, 2, 2), form("perfect", 3, 0, 0, 3),
		{Name: "café_long_name", Namespace: "demo.Zé", Killed: 0, Survived: 2, Sites: 2}})
	want := "" +
		"Function       Namespace  Killed  Survived  Uncovered  Sites   Score\n" +
		"--------------------------------------------------------------------\n" +
		"idle           demo            0         0          2      2    n/a\n" +
		"café_long_name demo.Zé         0         2          0      2   0.0%\n" +
		"place          demo            1         1          0      2  50.0%\n" +
		"perfect        demo            3         0          0      3 100.0%\n" +
		"--------------------------------------------------------------------\n" +
		"Total                          4         3          2      9   57.1%\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	idle := formatResults([]FormResult{form("place", 0, 0, 2, 2)})
	if !strings.Contains(idle, "Total                    0         0          2      2     n/a\n") {
		t.Fatalf("n/a total:\n%s", idle)
	}
}

func TestScanMarksUncoveredOnlyWhenTheMapSaysSo(t *testing.T) {
	s := Site{File: "src/demo.py", Namespace: "demo", FormID: "defn/place", Line: 2, Original: ">", Mutant: ">=", ID: "m"}
	if got := formatScan("src/demo.py", []Site{s}, nil, map[string]bool{"m": true}); got != "Scan: 1 mutation sites in src/demo.py\n  src/demo.py:2 > -> >=  [defn/place]\n" {
		t.Fatalf("covered %q", got)
	}
	if strings.Contains(formatScan("src/demo.py", []Site{s}, nil, map[string]bool{}), "uncovered") {
		t.Fatal("absent from the map is not uncovered")
	}
	bare := formatScan("src/demo.py", []Site{s}, map[formKey]bool{{"other", "defn/x"}: true}, map[string]bool{"m": false})
	if bare != "Scan: 1 mutation sites in src/demo.py\n  src/demo.py:2 > -> >= uncovered  [defn/place]\n* marks a function whose text changed since the last snapshot.\n" {
		t.Fatalf("bare %q", bare)
	}
}
