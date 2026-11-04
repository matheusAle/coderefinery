package mutate

// Ported from upstream tests/test_sites.py and the site parts of
// tests/test_boundaries.py, plus raw-site goldens recorded from upstream
// sites.discover_raw (testdata/raw_sites.jsonl: [language, path, source,
// [[line, anchor, start, end, original, mutant], ...]]).

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRawSitesMatchUpstreamGoldens(t *testing.T) {
	f, err := os.Open("testdata/raw_sites.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	checked := map[string]int{}
	for n := 1; scanner.Scan(); n++ {
		var rec []json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatal(err)
		}
		var language, path, source string
		var want [][]any
		_ = json.Unmarshal(rec[0], &language)
		_ = json.Unmarshal(rec[1], &path)
		_ = json.Unmarshal(rec[2], &source)
		_ = json.Unmarshal(rec[3], &want)
		grammar := grammarFor(language, path)
		t.Run(fmt.Sprintf("%d-%s", n, grammar), func(t *testing.T) {
			if language != "clojure" {
				needGrammar(t, grammar)
			}
			var got [][]any
			for _, s := range discoverRaw(source, language, path, false) {
				got = append(got, []any{float64(s.Line), float64(s.AnchorLine), float64(s.Start), float64(s.End), s.Original, s.Mutant})
			}
			if len(want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%q\n got %v\nwant %v", source, got, want)
			}
		})
		checked[grammar]++
	}
	if checked["python"] < 10 || checked["clojure"] < 2 {
		t.Fatalf("golden file too small: %v", checked)
	}
}

func TestSitesFollowJavaDecisionsInPython(t *testing.T) {
	root := tempRoot(t)
	sites := sitesOf(t, root, "src/demo/app.py",
		"def place(x):\n    if x > 0 and ready:\n        return x + 1\n    if not ok:\n        return 0\n")
	pairs := pairsOf(sites)
	for _, p := range []pairT{{"and", "or"}, {"not", ""}, {">", ">="}, {"+", "-"}, {"0", "1"}, {"1", "0"}} {
		if !pairs[p] {
			t.Errorf("missing %v in %v", p, pairs)
		}
	}
	if pairs[pairT{"True", "False"}] {
		t.Error("no True literal in the source")
	}
	if ns := namespacesOf(sites); !reflect.DeepEqual(ns, []string{"demo.app"}) {
		t.Errorf("namespaces %v", ns)
	}
	first := sites[0]
	if first.ID != `["src/demo/app.py","demo.app","defn/place",23,24,">",">="]` || first.Line != 2 || first.Description() != "> -> >=" {
		t.Errorf("first site %+v", first)
	}
}

func TestPythonPrivateAndClassNamespace(t *testing.T) {
	root := tempRoot(t)
	sites := sitesOf(t, root, "src/demo/board.py",
		"class Board:\n    def place(self, x):\n        return x > 0\n    def _hide(self):\n        return False\n")
	forms := map[string]string{}
	for _, s := range sites {
		forms[s.Name] = s.FormID
	}
	if !reflect.DeepEqual(forms, map[string]string{"place": "defn/place", "_hide": "defn-/_hide"}) {
		t.Fatalf("forms %v", forms)
	}
	if ns := namespacesOf(sites); !reflect.DeepEqual(ns, []string{"demo.board.Board"}) {
		t.Fatalf("namespaces %v", ns)
	}
}

func TestStringsAndNamesAreNotSites(t *testing.T) {
	root := tempRoot(t)
	if s := sitesOf(t, root, "src/demo/app.py", "def place():\n    return \"x > 0 and true\"\n"); len(s) != 0 {
		t.Errorf("string contents gave %v", s)
	}
	if p := pairsOf(sitesOf(t, root, "src/demo/name.py", "def place(true):\n    return true\n")); p[pairT{"true", "false"}] {
		t.Error("an identifier named true is not a boolean")
	}
	if s := sitesOf(t, root, "src/demo/two.py", "def place():\n    return 2\n"); len(s) != 0 {
		t.Errorf("2 is not a site: %v", s)
	}
	p := pairsOf(sitesOf(t, root, "src/demo/sub.py", "def place(a, b):\n    return a - b\n"))
	if !p[pairT{"-", "+"}] || p[pairT{"-", ""}] {
		t.Errorf("binary minus: %v", p)
	}
	if s := sitesOf(t, root, "top.py", "x = 1 > 0\n"); len(s) != 0 {
		t.Errorf("top-level code has no owner: %v", s)
	}
}

func TestADeepExpressionIsWalked(t *testing.T) {
	expr := strings.Repeat("1 + ", 1199) + "1"
	if sites := discoverRaw("def place():\n    return "+expr+"\n", "python", "app.py", false); len(sites) != 2399 {
		t.Fatalf("python: %d sites, want 2399", len(sites))
	}
	if !testing.Short() {
		t.Run("go", func(t *testing.T) {
			needGrammar(t, "go")
			if sites := discoverRaw("package p\nfunc place() int { return "+expr+" }\n", "go", "app.go", false); len(sites) == 0 {
				t.Fatal("go: no sites")
			}
		})
	}
}

func TestClojureRulesFollowCljMutateAndMarkPrivateDefns(t *testing.T) {
	root := tempRoot(t)
	src := "(ns demo.core)\n(defn place [x]\n  (if (> x 0)\n    (+ x 1)\n    0))\n(defn- hide []\n  false)\n"
	sites := sitesOf(t, root, "src/demo/core.clj", src)
	got := map[[3]string]bool{}
	for _, s := range sites {
		got[[3]string{s.FormID, s.Original, s.Mutant}] = true
	}
	for _, w := range [][3]string{{"defn/place", "if", "if-not"}, {"defn/place", ">", ">="}, {"defn/place", "+", "-"},
		{"defn/place", "0", "1"}, {"defn/place", "1", "0"}, {"defn-/hide", "false", "true"}} {
		if !got[w] {
			t.Errorf("missing %v", w)
		}
	}
	if ns := namespacesOf(sites); !reflect.DeepEqual(ns, []string{"demo.core"}) {
		t.Errorf("namespaces %v", ns)
	}
	quiet := "(ns demo.core)\n(defn place []\n  (comment (+ 1 0))\n  (quote (+ 1 0))\n  (str \"true + 1\"))\n"
	if s := sitesOf(t, root, "src/demo/quiet.clj", quiet); len(s) != 0 {
		t.Errorf("comment/quote/string gave %v", s)
	}
	if s := sitesOf(t, root, "src/demo/vec.clj", "(ns demo.core)\n(defn place []\n  [inc])\n"); len(s) != 0 {
		t.Errorf("a vector element is not a head: %v", s)
	}
	plus := 0
	for _, s := range sitesOf(t, root, "src/demo/add.clj", "(ns demo.core)\n(defn place []\n  (+ + 1))\n") {
		if s.Original == "+" {
			plus++
		}
	}
	if plus != 1 {
		t.Errorf("(+ + 1): %d + sites, want 1", plus)
	}
}

func TestGrammarForTypescriptFamily(t *testing.T) {
	for path, want := range map[string]string{"src/app.tsx": "tsx", "src/app.ts": "typescript", "src/app.mts": "typescript",
		"src/app.js": "javascript", "src/app.mjs": "javascript", "src/app.cjs": "javascript", "src/app.jsx": "javascript"} {
		if got := grammarFor("typescript", path); got != want {
			t.Errorf("%s: %s want %s", path, got, want)
		}
	}
	if grammarFor("python", "src/app.py") != "python" {
		t.Error("python")
	}
}

// ---- languages whose parsers land separately ----

func TestJavaRulesFollowMutate4java(t *testing.T) {
	needGrammar(t, "java")
	needReader(t, "java")
	root := tempRoot(t)
	sites := sitesOf(t, root, "src/demo/Board.java",
		"package demo;\nclass Board {\n  int place(int x) {\n    if (x > 0 && ready)\n      return x + 1;\n    return !ok ? -x : 0;\n  }\n  private int hide() { return 1; }\n}\n")
	var public, private []Site
	for _, s := range sites {
		switch s.FormID {
		case "defn/place":
			public = append(public, s)
		case "defn-/hide":
			private = append(private, s)
		}
	}
	p := pairsOf(public)
	for _, w := range []pairT{{"+", "-"}, {">", ">="}, {"&&", "||"}, {"!", ""}, {"-", ""}, {"0", "1"}} {
		if !p[w] {
			t.Errorf("missing %v", w)
		}
	}
	if p[pairT{"/", "*"}] || p[pairT{"*", "/"}] {
		t.Error("no division in place")
	}
	if len(private) != 1 || private[0].Original != "1" {
		t.Errorf("hide sites %v", private)
	}
	if ns := namespacesOf(sites); !reflect.DeepEqual(ns, []string{"demo.Board"}) {
		t.Errorf("namespaces %v", ns)
	}
	p = pairsOf(sitesOf(t, root, "src/demo/Math.java", "package demo;\nclass Math {\n  int place(int a, int b) { return a * b + a / b; }\n}\n"))
	if !p[pairT{"*", "/"}] || !p[pairT{"/", "*"}] || !p[pairT{"+", "-"}] {
		t.Errorf("division both ways: %v", p)
	}
}

func TestGoMatchesMutate4goAndSplitsReceivers(t *testing.T) {
	needGrammar(t, "go")
	needReader(t, "go")
	root := tempRoot(t)
	sites := sitesOf(t, root, "demo.go",
		"package demo\nfunc Run(a int, b int) int {\n    if a > 0 && b == 1 {\n        return a * b\n    }\n    return a / b\n}\ntype Widget struct{}\nfunc (w Widget) Run() bool { return true }\n")
	var free, method []Site
	for _, s := range sites {
		if s.Namespace == "demo" {
			free = append(free, s)
		} else if s.Namespace == "demo.Widget" {
			method = append(method, s)
		}
	}
	p := pairsOf(free)
	for _, w := range []pairT{{"*", "/"}, {"&&", "||"}, {"==", "!="}, {">", ">="}, {"0", "1"}, {"1", "0"}} {
		if !p[w] {
			t.Errorf("missing %v", w)
		}
	}
	if p[pairT{"/", "*"}] {
		t.Error("Go never mutates /")
	}
	if len(method) != 1 || method[0].FormID != "defn/Run" || method[0].Original != "true" {
		t.Errorf("method sites %v", method)
	}
	hidden := sitesOf(t, root, "hide.go", "package demo\nfunc hide() bool { return false }\n")
	if len(hidden) == 0 || hidden[0].FormID != "defn-/hide" {
		t.Errorf("unexported function: %v", hidden)
	}
}

func TestTypescriptAndRustUseTheJavaDecisions(t *testing.T) {
	root := tempRoot(t)
	t.Run("typescript", func(t *testing.T) {
		needGrammar(t, "typescript")
		needReader(t, "typescript")
		sites := sitesOf(t, root, "src/demo/app.ts",
			"export function place(x: number) {\n  if (x > 0 && ready) return x + 1;\n  return true === false;\n}\n")
		p := pairsOf(sites)
		if !p[pairT{"===", "!=="}] || !p[pairT{"&&", "||"}] {
			t.Errorf("pairs %v", p)
		}
		if ns := namespacesOf(sites); !reflect.DeepEqual(ns, []string{"demo.app"}) {
			t.Errorf("namespaces %v", ns)
		}
	})
	t.Run("rust", func(t *testing.T) {
		needGrammar(t, "rust")
		needReader(t, "rust")
		p := pairsOf(sitesOf(t, root, "src/lib.rs",
			"fn place(x: i32) -> i32 {\n    if x > 0 && ready { return x + 1; }\n    if !ok { return -x; }\n    0\n}\n"))
		if !p[pairT{"&&", "||"}] || !p[pairT{"!", ""}] || !p[pairT{"-", ""}] {
			t.Errorf("pairs %v", p)
		}
		trues := 0
		for _, s := range sitesOf(t, root, "src/one.rs", "fn place() -> bool { true }\n") {
			if s.Original == "true" {
				trues++
			}
		}
		if trues != 1 {
			t.Errorf("%d true sites, want 1", trues)
		}
	})
}

func TestTypescriptNullishOptionalChainingAndTypes(t *testing.T) {
	needGrammar(t, "typescript")
	needReader(t, "typescript")
	root := tempRoot(t)
	sites := sitesOf(t, root, "src/demo/app.ts", "\nexport function place(a, b) {\n  return a ?? b?.c ?? b?.[0] ?? b?.();\n}\n")
	nullish, optional := 0, map[string]bool{}
	for _, s := range sites {
		if s.Original == "??" && s.Mutant == "||" {
			nullish++
		}
		if s.Original == "?." {
			optional[s.Mutant] = true
		}
	}
	if nullish != 3 || !reflect.DeepEqual(optional, map[string]bool{".": true, "": true}) {
		t.Errorf("nullish %d optional %v", nullish, optional)
	}
	if s := sitesOf(t, root, "src/demo/q.ts", "export function place() {\n  return \"a ?? b?.c\";\n}\n"); len(s) != 0 {
		t.Errorf("quoted operators: %v", s)
	}
	typed := sitesOf(t, root, "src/demo/check.ts", "type Flag = { on: true };\ninterface Box { open: false }\nexport function check(n: number): { ok: true } | { ok: false; used?: true } {\n  const seen: Promise<{ done: false }> | null = null;\n  return { ok: n > 0 };\n}\n")
	got := map[[3]string]bool{}
	for _, s := range typed {
		got[[3]string{fmt.Sprint(s.Line), s.Original, s.Mutant}] = true
	}
	if !reflect.DeepEqual(got, map[[3]string]bool{{"5", ">", ">="}: true, {"5", "0", "1"}: true}) {
		t.Errorf("type positions leaked: %v", got)
	}
	saved := sitesOf(t, root, "src/demo/save.ts", "export function save(db: Db, now: number) {\n  db\n    .prepare(\"x\")\n    .bind(now + 1)\n    .run();\n  return {\n    ok: true,\n  };\n}\n")
	anchors := map[[3]string]bool{}
	for _, s := range saved {
		anchors[[3]string{fmt.Sprint(s.Line), s.Original, fmt.Sprint(s.AnchorLine)}] = true
	}
	for _, w := range [][3]string{{"4", "+", "2"}, {"4", "1", "2"}, {"7", "true", "6"}} {
		if !anchors[w] {
			t.Errorf("anchor %v missing from %v", w, anchors)
		}
	}
}

func TestJavascriptExpressHandlerOwnsItsNullishSite(t *testing.T) {
	needGrammar(t, "javascript")
	needReader(t, "typescript")
	root := tempRoot(t)
	sites := sitesOf(t, root, "src/demo/routes.js", "\nexport function mount(app) {\n  app.get(\"/users\", (req, res) => {\n    return req.body ?? false;\n  });\n}\n")
	found := false
	for _, s := range sites {
		if s.Name == "mount" {
			t.Errorf("mount owns %v", s)
		}
		if s.Name == "GET /users" && s.Original == "??" && s.Mutant == "||" {
			found = true
		}
	}
	if !found || !reflect.DeepEqual(namespacesOf(sites), []string{"demo.routes"}) {
		t.Errorf("sites %v", sites)
	}
}
