package edn

import (
	"strings"
	"testing"
)

const crapSample = `{:entries [
  {:name "_grade", :namespace "shop.reports", :complexity 6, :coverage 10.0, :crap 32.244}
  {:name "x", :namespace "a\"b", :complexity 1, :coverage nil, :crap nil}
]}
`

func TestParseCrapSnapshot(t *testing.T) {
	v, err := Parse(crapSample)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != Map || len(v.Keys) != 1 || v.Keys[0].Text != "entries" {
		t.Fatalf("top level = %s", v)
	}
	entries := v.Vals[0]
	if entries.Kind != Vector || len(entries.Items) != 2 {
		t.Fatalf("entries = %s", entries)
	}
	first := entries.Items[0]
	if got := first.String(); got != `{:name "_grade", :namespace "shop.reports", :complexity 6, :coverage 10.0, :crap 32.244}` {
		t.Fatalf("first = %s", got)
	}
	second := entries.Items[1]
	if second.Vals[1].Text != `a"b` || second.Vals[3].Kind != Nil {
		t.Fatalf("second = %s", second)
	}
}

func TestParseMutateSnapshot(t *testing.T) {
	src := `{:version 1 :tested-at "2026-11-03T16:01:04+00:00" :outcomes {"[\"src/a.py\",\"a\",\"defn/f\",1,2,\"or\",\"and\"]" :killed} :forms [{:id "defn-/_g" :sites 0}] :tags #{:a :b} #_ :skipped :chars [\a \newline] :inst #inst "2020"}`
	v, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Keys) != 7 {
		t.Fatalf("keys = %d: %s", len(v.Keys), v)
	}
	outcomes := v.Vals[2]
	if outcomes.Keys[0].Text != `["src/a.py","a","defn/f",1,2,"or","and"]` || outcomes.Vals[0].Text != "killed" {
		t.Fatalf("outcomes = %s", outcomes)
	}
	if v.Vals[4].Kind != Set || v.Vals[5].Items[1].Text != "\n" || v.Vals[6].Kind != Tagged {
		t.Fatalf("misc = %s", v)
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{"{:a}", "[1 2", `"abc`, "{} {}", ")"} {
		if _, err := Parse(src); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", src)
		}
	}
}

func TestCompareIgnoresMapOrderAndIgnoredKeys(t *testing.T) {
	a, _ := Parse(`{:a 1 :tested-at "x" :b [1 2.5 "s"]}`)
	b, _ := Parse(`{:b [1 2.5 "s"] :tested-at "y" :a 1}`)
	diffs := Compare(a, b, Options{IgnoreKeys: map[string]bool{"tested-at": true}})
	if len(diffs) != 0 {
		t.Fatalf("diffs = %v", diffs)
	}
	if diffs := Compare(a, b, Options{}); len(diffs) != 1 || diffs[0].Path != "$ :tested-at" {
		t.Fatalf("without ignore: %v", diffs)
	}
}

func TestCompareReportsTypedAndPositionalDifferences(t *testing.T) {
	a, _ := Parse(`{:entries [{:crap 9.321 :complexity 3} {:crap 1.0}]}`)
	b, _ := Parse(`{:entries [{:crap 9.32 :complexity 3.0}]}`)
	diffs := Compare(a, b, Options{})
	var got []string
	for _, d := range diffs {
		got = append(got, d.String())
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"$ :entries[0] :crap: want 9.321, got 9.32",
		"$ :entries[0] :complexity: want 3, got 3.0 (type int vs float)",
		"$ :entries[1]: want {:crap 1.0}, got <missing> (missing element)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if len(diffs) != 3 {
		t.Errorf("len(diffs) = %d, want 3", len(diffs))
	}
}

func TestCompareFloatTolerance(t *testing.T) {
	a, _ := Parse(`[9.321]`)
	b, _ := Parse(`[9.3211]`)
	if d := Compare(a, b, Options{}); len(d) != 1 {
		t.Fatalf("exact: %v", d)
	}
	if d := Compare(a, b, Options{FloatTolerance: 0.001}); len(d) != 0 {
		t.Fatalf("tolerant: %v", d)
	}
}
