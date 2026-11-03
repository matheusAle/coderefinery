package crap

// Ported from upstream crapper tests/test_clojure.py and the Clojure part
// of tests/test_edges.py.

import (
	"reflect"
	"strings"
	"testing"
)

func TestClojureBaseAndBranchForms(t *testing.T) {
	cases := map[string]int{
		"(defn foo [])":                                  1,
		"(defn foo [x]\n  (+ x 1))":                      1,
		"(defn foo [x]\n  (if x 1 0))":                   2,
		"(defn foo [x]\n  (if-not x 1 0))":               2,
		"(defn foo [x]\n  (if-let [y x] y 0))":           2,
		"(defn foo [x]\n  (when x 1))":                   2,
		"(defn foo [x]\n  (when-first [y x] y))":         2,
		"(defn foo [x y]\n  (and x y))":                  2,
		"(defn foo [x y]\n  (or x y))":                   2,
		"(defn foo [x]\n  (loop [i 0] (recur (inc i))))": 2,
	}
	for src, want := range cases {
		if got := ClojureComplexity(src); got != want {
			t.Errorf("%q: complexity %d, want %d", src, got, want)
		}
	}
}

func TestClojureCondCaseAndThreading(t *testing.T) {
	cases := map[string]int{
		"(defn foo [x]\n  (cond\n    (= x 1) :one\n    :else :other))":        3,
		"(defn foo [x]\n  (condp = x\n    1 :one\n    2 :two))":               3,
		"(defn foo [x]\n  (case x\n    1 :one\n    :other))":                  3,
		"(defn foo [x]\n  (cond-> x\n    (pos? x) inc\n    (even? x) (* 2)))": 3,
		"(defn foo [x]\n  (some-> x inc dec))":                                3,
		"(defn foo [x]\n  (some->> x (map inc) (filter pos?)))":               3,
		"(defn foo [x]\n  \"if when cond and or\"\n  x)":                      1,
		"(defn foo [x]\n  \"(if true 1 0)\"\n  x)":                            1,
		"(defn foo [x]\n  ;; if when cond\n  x)":                              1,
		"(defn foo [x]\n  (if x :yes :no) ; and or when cond)":                2,
		`(defn semi [] \; (if true 1 2))`:                                     2,
		`(defn quoted [] \" (if x 1 0))`:                                      2,
	}
	for src, want := range cases {
		if got := ClojureComplexity(src); got != want {
			t.Errorf("%q: complexity %d, want %d", src, got, want)
		}
	}
}

func TestClojureMapLiteralsInsideCondAreNotExtraClauses(t *testing.T) {
	src := `(defn foo [cell]
  (let [contents (:contents cell)]
    (cond
      (pred-a? contents)
      {:type :army :mode :awake :owner (:owner contents) :aboard true}

      (pred-b? contents)
      {:type :fighter :mode :awake :owner (:owner contents) :fuel 20 :from-carrier true}

      (pred-c? contents) contents

      (pred-d? cell)
      {:type :fighter :mode :awake :owner :player :fuel 20 :from-airport true}

      :else nil)))`
	if got := ClojureComplexity(src); got != 6 {
		t.Fatalf("complexity %d, want 6", got)
	}
}

type cljEntry struct {
	Name       string
	Complexity int
	Start      int
}

func cljEntries(fns []ClojureDefn) []string {
	var names []string
	for _, f := range fns {
		names = append(names, f.Name)
	}
	return names
}

func TestClojureExtractsDefnNamesLinesAndNamespace(t *testing.T) {
	src := "(ns demo.core)\n\n(defn choose [x]\n  (if x 1 0))\n\n(defn- hidden [y]\n  y)\n"
	fns, _ := ClojureFunctions(src, "src/demo/core.clj", "/proj")
	var got []cljEntry
	for _, f := range fns {
		got = append(got, cljEntry{f.Name, f.Complexity, f.StartLine})
		if f.Namespace != "demo.core" || f.Language != "clojure" || f.StartByte != -1 {
			t.Errorf("%s: namespace %q language %q byte %d", f.Name, f.Namespace, f.Language, f.StartByte)
		}
	}
	want := []cljEntry{{"choose", 2, 3}, {"hidden", 1, 6}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if fns[1].EndLine != 7 {
		t.Fatalf("hidden ends on %d, want 7", fns[1].EndLine)
	}
}

func TestClojureTopLevelDefDoesNotAttachToThePreviousFunction(t *testing.T) {
	src := "(defn alpha [x]\n  (if x 1 0))\n\n(def rules\n  [{:pred (fn [v]\n            (if v (when (pos? v) true) false))}])\n\n(defn omega [y]\n  y)"
	got := map[string]int{}
	for _, d := range ClojureTopLevelDefns(src) {
		got[d.Name] = ClojureComplexity(d.Text)
	}
	if !reflect.DeepEqual(got, map[string]int{"alpha": 2, "omega": 1}) {
		t.Fatalf("got %v", got)
	}
}

func TestClojureNamespaceFallsBackToThePath(t *testing.T) {
	for path, want := range map[[2]string]string{
		{"src/demo/core_extra.clj", ""}:       "demo.core-extra",
		{"/proj/src/demo/core.cljc", "/proj"}: "demo.core",
		{"pkg/src/demo/core.bb", ""}:          "demo.core",
	} {
		if got := ClojureNamespaceFromPath(path[0], path[1]); got != want {
			t.Errorf("%v: %q, want %q", path, got, want)
		}
	}
	fns, _ := ClojureFunctions("(defn foo [] 1)\n", "src/demo/core_extra.clj", "/proj")
	if len(fns) != 1 || fns[0].Namespace != "demo.core-extra" {
		t.Fatalf("got %+v", fns)
	}
}

func TestClojureReaderFormsStayInsideOneForm(t *testing.T) {
	src := "(defn foo [x]\n  \"say \\\"hi\\\"\n  (if x 1)\"\n  \\space\n  ;; (defn hidden [])\n  x)\n"
	if got := cljEntries(ClojureTopLevelDefns(src)); !reflect.DeepEqual(got, []string{"foo"}) {
		t.Fatalf("got %v", got)
	}
	discards := `
#_
; gone
(defn ghost [] (if x 1 0))
#_ "string"
#_ \x
#_ symbol
#_ #_(defn inner [] (if x 1 0))
#_(defn boxed []
  "str"
  \;
  ; comment
  (if x 1 0))
(defn ^{:doc "hi" :private true} kept [] 1)
(defn ^{:flag \; :private true} also [] 1)
(defn ^{:private true ; note
  } third [] 1)
( defn spaced [] 1)
(defn)
(defn ^)
`
	if got := cljEntries(ClojureTopLevelDefns(discards)); !reflect.DeepEqual(got, []string{"kept", "also", "third", "spaced"}) {
		t.Fatalf("got %v", got)
	}
	if got := ClojureTopLevelDefns("(defn ^{:private kept [] 1)"); len(got) != 0 {
		t.Fatalf("unbalanced metadata gave %v", got)
	}
	if got := cljEntries(ClojureTopLevelDefns("(def x #{\\\"})\n(defn foo [] 1)")); !reflect.DeepEqual(got, []string{"foo"}) {
		t.Fatalf("char literal swallowed the defn: %v", got)
	}
}

func TestClojureReaderKeepsTheRealNamespaceAndDecisions(t *testing.T) {
	src := `;; (ns demo.wrong)
(ns demo.core)
#_(defn dropped [x]
  (if x 1 0))
(defn ^:private kept [x]
  #_(if x 1 0)
  1)
(defn ^{:private true} also [] 1)
(defn quoted []
  \"
  (if x 1 0))
(defn semi []
  \;
  (if true 1 2))
`
	fns, _ := ClojureFunctions(src, "src/demo/core.clj", "/proj")
	var got []cljEntry
	for _, f := range fns {
		got = append(got, cljEntry{f.Name, f.Complexity, 0})
		if f.Namespace != "demo.core" {
			t.Errorf("%s namespace %q", f.Name, f.Namespace)
		}
	}
	want := []cljEntry{{"kept", 1, 0}, {"also", 1, 0}, {"quoted", 2, 0}, {"semi", 2, 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	only := ClojureTopLevelDefns("#_(defn dropped [] (if x 1 0))\n(defn kept [] 1)\n")
	if len(only) != 1 || only[0].Name != "kept" || only[0].StartLine != 2 || only[0].EndLine != 2 {
		t.Fatalf("got %+v", only)
	}
}

func TestClojureScannerSurvivesEndOfFile(t *testing.T) {
	for _, src := range []string{`(defn foo [] \`, `(defn foo [] \abc`, "(defn foo [] ;; comment", `(defn foo [] "hello`} {
		if got := ClojureTopLevelDefns(src); len(got) != 0 {
			t.Errorf("%q: %v", src, got)
		}
	}
	found := ClojureTopLevelDefns("(defn foo []\n  ;; comment\n  \"hello\")")
	if len(found) != 1 || found[0].Name != "foo" || !strings.HasSuffix(found[0].Text, ")") {
		t.Fatalf("got %+v", found)
	}
}

func TestClojureIsARegisteredReader(t *testing.T) {
	fns, err := FunctionsInFile("clojure", "(defn choose [x] x)\n", "src/demo/core.clj", "/proj")
	if err != nil || len(fns) != 1 || fns[0].Name != "choose" || fns[0].Namespace != "demo.core" {
		t.Fatalf("got %+v, %v", fns, err)
	}
}
