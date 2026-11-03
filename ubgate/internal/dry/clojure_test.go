package dry

import (
	"strings"
	"testing"
)

// Ports of upstream dryer tests/test_clojure.py.

func duplicatesIn(t *testing.T, root string, minLines, minNodes int, threshold float64) []Duplicate {
	t.Helper()
	entries, warnings, err := ScanFiles(allFiles(t, root), root, minLines, minNodes)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("scan: %v %v", err, warnings)
	}
	return FindDuplicates(entries, threshold)
}

func read(t *testing.T, src string) []*Form {
	t.Helper()
	forms, warning := ReadClojure(src)
	if warning != "" {
		t.Fatalf("warning %q", warning)
	}
	return forms
}

func heads(forms []*Form) string {
	var names []string
	for _, f := range forms {
		names = append(names, f.Items[0].Name)
	}
	return strings.Join(names, " ")
}

func names(forms []*Form) string {
	var out []string
	for _, f := range forms {
		out = append(out, f.Name)
	}
	return strings.Join(out, " ")
}

var sample = Duplicate{Score: 0.875, Language: "clojure", Left: Span{"a.clj", 10, 14}, Right: Span{"b.clj", 20, 24}, LeftNodes: 30, RightNodes: 31}

func TestEDNReportMatchesTheTextFields(t *testing.T) {
	want := "{:candidates [\n {:score 0.875\n  :language \"clojure\"\n  :left {:file \"a.clj\", :start-line 10, :end-line 14}\n  :right {:file \"b.clj\", :start-line 20, :end-line 24}\n  :left-nodes 30\n  :right-nodes 31}\n]}\n"
	if got := RenderEDN([]Duplicate{sample}); got != want {
		t.Fatalf("got %q", got)
	}
	if got := RenderEDN(nil); got != "{:candidates []}\n" {
		t.Fatalf("empty: %q", got)
	}
}

func TestTextReportMatchesDry4clj(t *testing.T) {
	if got := FormatText([]Duplicate{sample}); got != "DUPLICATE score=0.88\n  a.clj:10-14\n  b.clj:20-24\n" {
		t.Fatalf("got %q", got)
	}
	if got := FormatText(nil); got != "No duplicate candidates found.\n" {
		t.Fatalf("empty: %q", got)
	}
	two := FormatText([]Duplicate{sample, sample})
	if strings.Count(two, "\n\nDUPLICATE") != 1 || !strings.HasSuffix(two, "24\n") {
		t.Fatalf("two blocks: %q", two)
	}
}

func TestEDNFloat(t *testing.T) {
	for v, want := range map[float64]string{1.0: "1.0", 0.875: "0.875", 49.0 / 55: "0.890909090909",
		23.0 / 28: "0.821428571429", 3.0 / 7: "0.428571428571", 0.525: "0.525"} {
		if got := ednFloat(v); got != want {
			t.Errorf("ednFloat(%v) = %s, want %s", v, got, want)
		}
	}
}

func TestReportsStructuralDuplicatesWithLineRanges(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "left.cljc", "(ns sample.left)\n\n(defn alpha [xs]\n  (let [ys (filter odd? xs)]\n    (map inc ys)))\n")
	writeSource(t, root, "right.cljc", "(ns sample.right)\n\n(defn beta [items]\n  (let [kept (filter even? items)]\n    (map dec kept)))\n")
	found := duplicatesIn(t, root, 3, 8, 0.80)
	want := Duplicate{Score: 1.0, Language: "clojure", Left: Span{"left.cljc", 3, 5}, Right: Span{"right.cljc", 3, 5}, LeftNodes: 31, RightNodes: 31}
	if len(found) != 1 || found[0] != want {
		t.Fatalf("found %+v", found)
	}
}

func TestMatchesMapsSetsAndKeywordCalls(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "left.cljc", "(ns sample.left)\n\n(defn gamma [m]\n  (when (#{:a :b} (:kind m))\n    {:left (:a m) :right (:b m)}))\n")
	writeSource(t, root, "right.cljc", "(ns sample.right)\n\n(defn delta [row]\n  (when (#{:c :d} (:kind row))\n    {:left (:c row) :right (:d row)}))\n")
	if found := duplicatesIn(t, root, 3, 8, 0.80); len(found) != 1 || found[0].Score != 1.0 {
		t.Fatalf("found %+v", found)
	}
}

func TestReadsCljcReaderConditionals(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "left.cljc", "(ns sample.left)\n\n(defn alpha [x]\n  #?(:clj (when (pos? x)\n            (inc x))))\n")
	writeSource(t, root, "right.cljc", "(ns sample.right)\n\n(defn beta [y]\n  #?(:clj (when (pos? y)\n            (inc y))))\n")
	if found := duplicatesIn(t, root, 1, 1, 0.50); len(found) != 1 || found[0].Score != 1.0 {
		t.Fatalf("found %+v", found)
	}
}

func TestFiltersFormsShorterThanTheMinimumLineCount(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "one.clj", "(ns one)\n(defn a [x] (+ x 1))\n")
	writeSource(t, root, "two.clj", "(ns two)\n(defn b [y] (+ y 2))\n")
	if found := duplicatesIn(t, root, 3, 1, 0.80); len(found) != 0 {
		t.Fatalf("found %+v", found)
	}
	if found := duplicatesIn(t, root, 1, 1, 0.80); len(found) != 1 {
		t.Fatalf("smoke: without the line minimum the pair is found, got %+v", found)
	}
}

func TestMetadataCommentsAndDiscardsAreNotStructure(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "left.clj", "(ns left)\n\n(defn alpha [xs] ; keep\n  #_(println xs)\n  (let [ys (filter odd? xs)]\n    (map inc ys)))\n")
	writeSource(t, root, "right.clj", "(ns right)\n\n(defn ^String beta [items]\n  (let [kept (filter even? items)]\n    (map dec kept)))\n")
	if found := duplicatesIn(t, root, 3, 8, 0.80); len(found) != 1 || found[0].Score != 1.0 {
		t.Fatalf("found %+v", found)
	}
}

const invoice = `(ns billing.invoice)

(defn invoice-summary [orders]
  (let [paid (filter paid? orders), domestic (filter domestic? paid)
        sorted (sort-by :date domestic), amounts (map :amount sorted)
        taxes (map tax amounts), ids (map :id sorted)
        customers (map :customer sorted), regions (group-by :region sorted)
        flagged (filter flagged? sorted)]
    {:count (count sorted)
     :first-id (first ids), :last-id (last ids)
     :customers (set customers), :regions (keys regions)
     :flagged (count flagged), :total (reduce + 0 amounts)
     :tax (reduce + 0 taxes)}))
`

const receipt = `(ns billing.receipt)

(defn receipt-summary [rows]
  (let [closed (filter closed? rows), local (filter local? closed)
        ordered (sort-by :date local), amounts (map :amount ordered)
        taxable (filter taxable? ordered), taxes (map tax amounts)
        ids (map :id ordered), customers (map :customer ordered)
        regions (group-by :region ordered)
        flagged (filter flagged? ordered)]
    {:count (count ordered)
     :first-id (first ids), :last-id (last ids)
     :customers (set customers), :regions (keys regions)
     :flagged (count flagged), :total (reduce + 0 amounts)
     :tax (reduce + 0 taxes)}))
`

func TestExtraBindingScoresBelowOne(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "invoice.clj", invoice)
	writeSource(t, root, "receipt.clj", receipt)
	found := duplicatesIn(t, root, 4, 20, 0.82)
	want := Duplicate{Score: 49.0 / 55, Language: "clojure", Left: Span{"invoice.clj", 3, 13}, Right: Span{"receipt.clj", 3, 14}, LeftNodes: 158, RightNodes: 166}
	if len(found) != 1 || found[0] != want || ednFloat(found[0].Score) != "0.890909090909" {
		t.Fatalf("found %+v", found)
	}
}

func TestThreeCopiesMakeThreePairs(t *testing.T) {
	body := "(let [ys (filter odd? xs)]\n    (map inc ys))"
	root := t.TempDir()
	writeSource(t, root, "same.clj", "(ns same)\n\n(defn a [xs]\n  "+body+")\n\n(defn b [xs]\n  "+body+")\n\n(defn c [xs]\n  "+body+")\n")
	found := duplicatesIn(t, root, 3, 8, 0.80)
	if len(found) != 3 {
		t.Fatalf("found %+v", found)
	}
	for _, d := range found {
		if d.Score != 1.0 {
			t.Fatalf("score %v", d.Score)
		}
	}
	if found[0].Left.StartLine != 3 || found[0].Right.StartLine != 7 || found[1].Right.StartLine != 11 || found[2].Left.StartLine != 7 || found[2].Right.StartLine != 11 {
		t.Fatalf("order %+v", found)
	}
}

func TestUnreadableTailDoesNotDropAnEarlierForm(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "left.clj", "(ns left)\n\n(defn alpha [xs]\n  (let [ys (filter odd? xs)]\n    (map inc ys)))\n(defn broken [x]\n")
	writeSource(t, root, "right.clj", "(ns right)\n\n(defn beta [items]\n  (let [kept (filter even? items)]\n    (map dec kept)))\n")
	entries, warnings, err := ScanFiles(allFiles(t, root), root, 3, 8)
	if err != nil || strings.Join(warnings, "|") != "left.clj:6: unterminated list" {
		t.Fatalf("warnings %q, err %v", warnings, err)
	}
	if found := FindDuplicates(entries, 0.80); len(found) != 1 {
		t.Fatalf("found %+v", found)
	}
}

func TestReadsEscapedStringsAndCharacters(t *testing.T) {
	if got := heads(read(t, "(defn alpha [x]\n  (str \"a\\\\nb\" \\newline \\a))\n")); got != "defn" {
		t.Fatalf("heads %q", got)
	}
}

func TestReaderWarnings(t *testing.T) {
	for src, want := range map[string]string{
		"(defn alpha [x]\n  \"abc)\n":                     "2: unterminated string",
		"(defn alpha [x] \"abc\\":                         "1: unterminated string",
		"(defn alpha [x] \\":                              "1: incomplete character",
		"#?[:clj (inc x)]\n":                              "1: reader conditional body must be a list",
		"(defn alpha [x]\n  (inc x))\n#?@(:clj {:a 1})\n": "3: splicing reader conditional needs a collection",
		"{:a}\n":    "1: map literal has an odd number of forms",
		"(a\n[b\n":  "2: unterminated vector",
		"#{a":       "1: unterminated set",
		"#(a":       "1: unterminated list",
		"'":         "1: unexpected end of file",
		"#":         "1: incomplete dispatch",
		"(f #'x)":   "1: expected a symbol",
		"(f ##Inf)": "1: expected a symbol",
		"(a x#)":    "1: expected a symbol",
		"(a c')":    "1: expected a symbol",
		")":         "1: expected a symbol",
		"^m":        "1: unexpected end of file",
	} {
		if _, got := ReadClojure(src); got != want {
			t.Errorf("%q: warning %q, want %q", src, got, want)
		}
	}
	forms, warning := ReadClojure("(defn alpha [x]\n  (inc x))\n#?@(:clj {:a 1})\n")
	if heads(forms) != "defn" || warning == "" {
		t.Fatalf("forms before the error are kept: %q %q", heads(forms), warning)
	}
}

func TestBOMAndShebangAreNotForms(t *testing.T) {
	if got := heads(read(t, "\uFEFF#!/usr/bin/env bb\n(defn alpha [x]\n  (inc x))\n")); got != "defn" {
		t.Fatalf("heads %q", got)
	}
}

func TestReaderConditionalKeepsCljAndSplicesACollection(t *testing.T) {
	if got := heads(read(t, "#?(:cljs (inc x) :default (dec x))\n")); got != "dec" {
		t.Fatalf("default branch: %q", got)
	}
	if got := read(t, "#?((inc x) (dec x))\n"); len(got) != 0 {
		t.Fatalf("no feature: %d forms", len(got))
	}
	spliced := read(t, "(defn alpha []\n  [#?@(:clj [:a :b])])\n")
	if got := names(spliced[0].Items[3].Items); got != "a b" {
		t.Fatalf("spliced %q", got)
	}
}

func TestEmptyListNormalizesToALiteralList(t *testing.T) {
	if got := Pr(NormalizeClojure(read(t, "(defn alpha [x]\n  ())\n")[0])); !strings.Contains(got, "[:list :literal]") {
		t.Fatalf("got %s", got)
	}
}

func TestReaderMacrosWrapTheFollowingForm(t *testing.T) {
	forms := read(t, "'x\n@x\n`x\n~x\n~@x\n")
	if got := heads(forms); got != "quote deref syntax-quote unquote unquote-splicing" {
		t.Fatalf("heads %q", got)
	}
	if forms[0].Items[1].Name != "x" || forms[4].Items[1].Name != "x" {
		t.Fatal("wrapped form lost")
	}
	if got := names(read(t, "'#_x\n")[0].Items); got != "quote" {
		t.Fatalf("quote of nothing: %q", got)
	}
}

func TestACharacterNameIsConsumed(t *testing.T) {
	items := read(t, "(defn alpha []\n  (str \\newline))\n")[0].Items[3].Items
	if len(items) != 2 || items[0].Kind != FormSym || items[1].Kind != FormLit {
		t.Fatalf("items %+v", items)
	}
}

func TestABareColonIsAnEmptyKeyword(t *testing.T) {
	if forms := read(t, ":\n"); len(forms) != 1 || forms[0].Kind != FormKw || forms[0].Name != "" {
		t.Fatalf("forms %+v", forms)
	}
	if forms := read(t, "#!only"); len(forms) != 0 {
		t.Fatalf("shebang only: %+v", forms)
	}
}

func TestNamespacedMapsKeepTheMap(t *testing.T) {
	for _, src := range []string{"#:foo{:a 1}\n", "#::{:a 1}\n", "#:{:a 1}\n"} {
		forms := read(t, src)
		if len(forms) != 1 || forms[0].Coll != "map" || forms[0].Pairs[0][0].Name != "a" {
			t.Errorf("%q: %+v", src, forms)
		}
	}
	if forms := read(t, "#::foo{:a 1}\n"); len(forms) != 2 || forms[0].Name != "foo" || forms[1].Coll != "map" {
		t.Errorf("#::foo quirk: %+v", forms)
	}
}

func TestMapPairsKeepSourceOrder(t *testing.T) {
	m := read(t, "{:a 1 :b 2}\n")[0]
	if m.Pairs[0][0].Name != "a" || m.Pairs[1][0].Name != "b" {
		t.Fatal("pair order")
	}
	left, right := Pr(NormalizeClojure(read(t, "{foo 1}\n")[0])), Pr(NormalizeClojure(read(t, "{bar 2}\n")[0]))
	if left != right || left != "[:map [:symbol :literal]]" {
		t.Fatalf("left %s right %s", left, right)
	}
	paired := read(t, "{:a 1\n :b 2}\n")[0]
	if paired.Pairs[0][1].Line != 1 || paired.Pairs[1][1].Line != 2 {
		t.Fatal("value lines")
	}
	if got := Pr(NormalizeClojure(read(t, "{foo bar}\n")[0])); strings.Contains(got, "foo") || strings.Contains(got, "bar") {
		t.Fatalf("names leaked: %s", got)
	}
}

func TestATrailingFeatureWithoutAnExpressionIsIgnored(t *testing.T) {
	if got := heads(read(t, "#?(:clj (inc x) :extra)\n")); got != "inc" {
		t.Fatalf("heads %q", got)
	}
	if got := read(t, "#?(:cljs (dec x) :extra)\n"); len(got) != 0 {
		t.Fatalf("forms %d", len(got))
	}
}

func TestNamesInsideASetAreErased(t *testing.T) {
	if got := Pr(NormalizeClojure(read(t, "#{foo bar}\n")[0])); got != "[:set :symbol :symbol]" {
		t.Fatalf("got %s", got)
	}
}

func TestOnlyANonEmptyListThatIsNotNsIsACandidate(t *testing.T) {
	sym := func(n string) *Form { return &Form{Kind: FormSym, Name: n, Line: 1} }
	coll := func(kind string, items ...*Form) *Form {
		return &Form{Kind: FormColl, Coll: kind, Line: 1, Items: items}
	}
	for _, c := range []struct {
		form *Form
		want bool
	}{
		{coll("vector", sym("a")), false},
		{coll("list"), false},
		{sym("a"), false},
		{coll("list", sym("ns"), sym("demo")), false},
		{coll("list", sym("defn")), true},
	} {
		if got := IsCandidateForm(c.form); got != c.want {
			t.Errorf("%+v: %v", c.form, got)
		}
	}
	if got := Pr(NormalizeClojure(coll("bag"))); got != "[:literal]" {
		t.Fatalf("unknown kind: %s", got)
	}
}

func TestMaxLineIgnoresTheClosingParen(t *testing.T) {
	forms := read(t, "(f\n  x\n)\n(g \"a\nb\")\n")
	if MaxLine(forms[0]) != 2 || MaxLine(forms[1]) != 4 {
		t.Fatalf("max lines %d %d", MaxLine(forms[0]), MaxLine(forms[1]))
	}
}

func TestClojureNumbersAreLiterals(t *testing.T) {
	forms := read(t, "(f 1 -2 3/4 1.5e3 .5 0x1F 7N 1. nil true \u0663)\n")
	var kinds []string
	for _, it := range forms[0].Items[1:] {
		kinds = append(kinds, map[FormKind]string{FormSym: "sym", FormLit: "lit"}[it.Kind])
	}
	if got := strings.Join(kinds, " "); got != "lit lit lit lit lit lit lit sym sym sym lit" {
		t.Fatalf("kinds %s", got)
	}
}
