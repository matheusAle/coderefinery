package dry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/lang"
)

// Ports of upstream dryer tests/test_languages.py plus the golden
// normalized forms of spec-dryer §10.1. Cases for a language whose hand
// parser has not landed yet are skipped.

func writeSource(t *testing.T, root, name, text string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o666); err != nil {
		t.Fatal(err)
	}
}

func needGrammars(t *testing.T, grammars ...string) {
	t.Helper()
	for _, g := range grammars {
		if !lang.Supported(g) {
			t.Skipf("no %s parser registered yet", g)
		}
	}
}

// allFiles is sorted(root.rglob("*")) restricted to files.
func allFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func scanAll(t *testing.T, root string, threshold float64) []Duplicate {
	t.Helper()
	entries, warnings, err := ScanFiles(allFiles(t, root), root, 1, 1)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("scan: %v %v", err, warnings)
	}
	return FindDuplicates(entries, threshold)
}

func pairs(t *testing.T, root string) []Duplicate { return scanAll(t, root, 0.99) }

func byLanguage(found []Duplicate) map[string][]Duplicate {
	out := map[string][]Duplicate{}
	for _, d := range found {
		out[d.Language] = append(out[d.Language], d)
	}
	return out
}

func languages(m map[string][]Duplicate) string {
	var names []string
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

const (
	javaLeft  = "class Left {\n  int alpha(int[] xs) {\n    int[] ys = filter(xs, odd);\n    return map(ys, inc);\n  }\n}\n"
	javaRight = "class Right {\n  int beta(int[] items) {\n    int[] kept = filter(items, even);\n    return map(kept, dec);\n  }\n}\n"
	goLeft    = "package demo\n\nfunc (b *Board) alpha(xs []int) []int {\n\tys := filter(xs, odd)\n\treturn mapFn(ys, inc)\n}\n"
	goRight   = "package demo\n\nfunc (p *Panel) beta(items []int) []int {\n\tkept := filter(items, even)\n\treturn mapFn(kept, dec)\n}\n"
	pyLeft    = "class Board:\n    def alpha(self, xs):\n        ys = filter(xs, odd)\n        return map(ys, inc)\n"
	pyRight   = "class Panel:\n    def beta(self, items):\n        kept = filter(items, even)\n        return map(kept, dec)\n"
	tsLeft    = "export function alpha(xs: number[]): number[] {\n  const ys = filter(xs, odd);\n  return map(ys, inc);\n}\n"
	tsRight   = "export function beta(items: number[]): number[] {\n  const kept = filter(items, even);\n  return map(kept, dec);\n}\n"
	tsxLeft   = "export const alpha = (xs: number[]): number[] => {\n  const ys = filter(xs, odd);\n  return map(ys, inc);\n};\n"
	tsxRight  = "export const beta = (items: number[]): number[] => {\n  const kept = filter(items, even);\n  return map(kept, dec);\n};\n"
	rustLeft  = "pub fn alpha(xs: Vec<i32>) -> Vec<i32> {\n    let ys = filter(xs, odd);\n    map(ys, inc)\n}\n"
	rustRight = "pub fn beta(items: Vec<i32>) -> Vec<i32> {\n    let kept = filter(items, even);\n    map(kept, dec)\n}\n"
)

func TestEachLanguageMatchesRenamedLocals(t *testing.T) {
	samples := []struct{ grammar, left, right, ext string }{
		{"java", javaLeft, javaRight, "java"}, {"go", goLeft, goRight, "go"}, {"python", pyLeft, pyRight, "py"},
		{"typescript", tsLeft, tsRight, "ts"}, {"tsx", tsxLeft, tsxRight, "tsx"}, {"rust", rustLeft, rustRight, "rs"},
	}
	root := t.TempDir()
	want := map[string]bool{}
	for _, s := range samples {
		if lang.Supported(s.grammar) { // languages whose parser has not landed are left out
			writeSource(t, root, "left."+s.ext, s.left)
			writeSource(t, root, "right."+s.ext, s.right)
			want[LanguageOf("x."+s.ext)] = true
		}
	}
	groups := byLanguage(pairs(t, root))
	if len(groups) != len(want) || !want["python"] {
		t.Fatalf("languages %q, want %v", languages(groups), want)
	}
	wantNodes := map[string]int{"go": 61, "java": 52, "python": 35, "rust": 62}
	wantSpan := map[string]Span{"go": {"left.go", 3, 6}, "java": {"left.java", 2, 5}, "python": {"left.py", 2, 4}, "rust": {"left.rs", 1, 4}}
	for language, group := range groups {
		for _, d := range group {
			if d.Score != 1.0 {
				t.Errorf("%s score %v", language, d.Score)
			}
		}
		if n, ok := wantNodes[language]; ok && (len(group) != 1 || group[0].LeftNodes != n || group[0].Left != wantSpan[language]) {
			t.Errorf("%s: %+v, want nodes %d span %v", language, group, n, wantSpan[language])
		}
	}
	if lang.Supported("typescript") && lang.Supported("tsx") {
		ts := groups["typescript"]
		if len(ts) != 2 || ts[0].Left.File != "left.ts" || ts[1].Left.File != "left.tsx" || ts[0].LeftNodes != 50 || ts[1].LeftNodes != 58 {
			t.Errorf("typescript pairs %+v", ts)
		}
	}
}

func TestDifferentCalleeIsNotTheSameStructure(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "a.py", "def alpha(xs):\n    return filter(xs, odd)\n")
	writeSource(t, root, "b.py", "def beta(xs):\n    return select(xs, odd)\n")
	found := scanAll(t, root, 0.0)
	if len(found) != 1 || ednFloat(found[0].Score) != "0.428571428571" || found[0].LeftNodes != 19 || found[0].RightNodes != 19 {
		t.Fatalf("found %+v", found)
	}
}

func TestNestedFunctionsStayInsideTheEnclosingFunction(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "a.py", "def alpha(xs):\n    def inner(ys):\n        return filter(ys, odd)\n    return inner(xs)\n")
	writeSource(t, root, "b.py", "def beta(items):\n    def inner(kept):\n        return filter(kept, even)\n    return inner(items)\n")
	found := pairs(t, root)
	if len(found) != 1 || found[0].Score != 1.0 || found[0].Left != (Span{"a.py", 1, 4}) {
		t.Fatalf("found %+v", found)
	}
}

func TestRustTestsModuleIsNotProductionCode(t *testing.T) {
	needGrammars(t, "rust")
	root := t.TempDir()
	writeSource(t, root, "a.rs", rustLeft+"\nmod tests {\n    pub fn helper(xs: Vec<i32>) -> Vec<i32> {\n        let ys = filter(xs, odd);\n        map(ys, inc)\n    }\n}\n")
	writeSource(t, root, "b.rs", rustRight)
	found := pairs(t, root)
	if len(found) != 1 || found[0].Left != (Span{"a.rs", 1, 4}) {
		t.Fatalf("found %+v", found)
	}
}

func TestAnExtraStatementLowersTheScore(t *testing.T) {
	needGrammars(t, "java")
	root := t.TempDir()
	writeSource(t, root, "a.java", javaLeft)
	writeSource(t, root, "b.java", "class Right {\n  int beta(int[] items) {\n    int[] kept = filter(items, even);\n    int[] extra = filter(kept, even);\n    return map(extra, dec);\n  }\n}\n")
	found := scanAll(t, root, 0.5)
	want := Duplicate{Score: 15.0 / 17.0, Language: "java", Left: Span{"a.java", 2, 5}, Right: Span{"b.java", 2, 6}, LeftNodes: 52, RightNodes: 75}
	if len(found) != 1 || found[0] != want {
		t.Fatalf("found %+v, want %+v", found, want)
	}
}

func TestCalleeNamesStayAndMemberNamesDoNot(t *testing.T) {
	samples := map[string]string{
		"left.py":    "def alpha(xs):\n    ys = xs.filter(odd)\n    return (len)(xs.total)\n",
		"right.py":   "def beta(items):\n    kept = items.filter(even)\n    return (len)(items.count)\n",
		"left.go":    "package demo\n\nfunc alpha(xs []int) {\n\tfmt.Println(xs)\n}\n",
		"right.go":   "package demo\n\nfunc beta(items []int) {\n\tlog.Println(items)\n}\n",
		"left.rs":    "pub fn alpha(xs: Vec<i32>) -> Vec<i32> {\n    let ys = Vec::new();\n    let zs = HashMap::<String, i32>::new();\n    xs.iter()\n}\n",
		"right.rs":   "pub fn beta(items: Vec<i32>) -> Vec<i32> {\n    let kept = Vec::new();\n    let rows = HashMap::<Integer, i32>::new();\n    items.iter()\n}\n",
		"left.java":  "class Left {\n  int alpha(List xs) {\n    List ys = new ArrayList<String>();\n    ys.add(xs);\n    return xs.total;\n  }\n}\n",
		"right.java": "class Right {\n  int beta(List items) {\n    List kept = new ArrayList<Integer>();\n    kept.add(items);\n    return items.count;\n  }\n}\n",
		"left.ts":    "export function alpha(xs: number[]): number {\n  return foo<string>(xs.bar);\n}\n",
		"right.ts":   "export function beta(items: number[]): number {\n  return foo<number>(items.baz);\n}\n",
	}
	suffix := map[string]string{"python": "py", "go": "go", "rust": "rs", "java": "java", "typescript": "ts"}
	grammar := map[string]string{"py": "python", "go": "go", "rs": "rust", "java": "java", "ts": "typescript"}
	root := t.TempDir()
	want := map[string]bool{}
	for name, src := range samples {
		ext := strings.TrimPrefix(filepath.Ext(name), ".")
		if lang.Supported(grammar[ext]) {
			writeSource(t, root, name, src)
			want[LanguageOf(name)] = true
		}
	}
	groups := byLanguage(pairs(t, root))
	if len(groups) != len(want) {
		t.Fatalf("languages %q, want %v", languages(groups), want)
	}
	for language, group := range groups {
		s := suffix[language]
		if len(group) != 1 || group[0].Left.File != "left."+s || group[0].Right.File != "right."+s || group[0].Score != 1.0 {
			t.Errorf("%s: %+v", language, group)
		}
	}
}

func TestADifferentCalleeNameLowersTheScore(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "a.py", "def alpha(xs):\n    return xs.filter(odd)\n")
	writeSource(t, root, "b.py", "def beta(items):\n    return items.select(even)\n")
	scores := []string{"0.416666666667"}
	if lang.Supported("rust") {
		writeSource(t, root, "a.rs", "fn alpha() {\n    Vec::new()\n}\n")
		writeSource(t, root, "b.rs", "fn beta() {\n    Vec::with_capacity(1)\n}\n")
		scores = []string{"0.416666666667", "0.4"}
	}
	found := scanAll(t, root, 0.0)
	var got []string
	for _, d := range found {
		got = append(got, ednFloat(d.Score))
	}
	if strings.Join(got, " ") != strings.Join(scores, " ") {
		t.Fatalf("scores %v, want %v", got, scores)
	}
}

// forms is the upstream test helper: entries of one source, no warning.
func forms(t *testing.T, language, src, file string) []Entry {
	t.Helper()
	entries, warning := EntriesInSource(testFingerprints, language, src, file, file, 0, 0)
	if warning != "" {
		t.Fatalf("warning %q", warning)
	}
	return entries
}

func spans(t *testing.T, language, src, file string) string {
	t.Helper()
	var out []string
	for _, e := range forms(t, language, src, file) {
		out = append(out, fmt.Sprintf("%d-%d", e.Span.StartLine, e.Span.EndLine))
	}
	return strings.Join(out, " ")
}

func hasFingerprint(entries []Entry, pred func(string) bool) bool {
	for _, e := range entries {
		for _, fp := range printedFingerprints(e) {
			if pred(fp) {
				return true
			}
		}
	}
	return false
}

func TestAParenthesizedCalleeMatchesTheBareCall(t *testing.T) {
	wrapped := forms(t, "python", "def alpha(xs):\n    return (len)(xs)\n", "a.py")
	bare := forms(t, "python", "def beta(items):\n    return len(items)\n", "b.py")
	if !hasFingerprint(wrapped, func(s string) bool { return strings.Contains(s, "len") }) {
		t.Fatal("len is not in the fingerprints")
	}
	found := FindDuplicates(append(wrapped, bare...), 0.99)
	if len(found) != 1 || found[0].Score != 1.0 || found[0].LeftNodes != 18 {
		t.Fatalf("found %+v", found)
	}
}

func TestAStringAndANumberNormalizeToTheSameLiteral(t *testing.T) {
	str := forms(t, "python", "def alpha(xs):\n    return \"xs\"\n", "a.py")
	num := forms(t, "python", "def beta(items):\n    return 1\n", "b.py")
	if !hasFingerprint(num, func(s string) bool { return s == ":literal" }) {
		t.Fatal(":literal is not in the fingerprints")
	}
	if found := FindDuplicates(append(str, num...), 0.99); len(found) != 1 || found[0].Score != 1.0 {
		t.Fatalf("found %+v", found)
	}
}

func TestAnInterpolatedStringIsNotAPlainLiteral(t *testing.T) {
	plain := forms(t, "python", "def alpha(xs):\n    return \"xs\"\n", "a.py")
	interp := forms(t, "python", "def beta(items):\n    return f\"{items}\"\n", "b.py")
	found := FindDuplicates(append(plain, interp...), 0.0)
	if len(found) != 1 || ednFloat(found[0].Score) != "0.285714285714" || found[0].LeftNodes != 11 || found[0].RightNodes != 19 {
		t.Fatalf("found %+v", found)
	}
}

func TestConstructedNamesStayInTheFingerprint(t *testing.T) {
	needGrammars(t, "rust", "java")
	rust := forms(t, "rust", "pub fn alpha() {\n    let ys = Vec::new();\n    let zs = HashMap::<String, i32>::new();\n}\n", "a.rs")
	java := forms(t, "java", "class Left {\n  int alpha() {\n    return new ArrayList<String>();\n  }\n}\n", "a.java")
	joined := func(e []Entry) string { return strings.Join(printedFingerprints(e[0]), " ") }
	for _, name := range []string{"Vec", "new", "HashMap"} {
		if !strings.Contains(joined(rust), name) {
			t.Errorf("rust fingerprints lack %s", name)
		}
	}
	if !strings.Contains(joined(java), "ArrayList") {
		t.Error("java fingerprints lack ArrayList")
	}
}

func TestCandidateSpans(t *testing.T) {
	cases := []struct {
		name, language, grammar, file, src, want string
	}{
		{"class contributes its method only", "python", "python", "a.py",
			"class Board:\n    def alpha(self, xs):\n        return xs\n", "2-3"},
		{"end line uses the last occupied row", "python", "python", "a.py",
			"def alpha(xs):\n    return xs\n", "1-2"},
		{"trailing indented comment stays in the def", "python", "python", "a.py",
			"def a(x):\n    # lead\n    return x\n    # trailing comment\n", "1-4"},
		{"java keeps the method that has its own body", "java", "java", "a.java",
			"abstract class Left {\n  abstract int declared(int[] xs);\n  int alpha(int[] xs) {\n    class Inner {\n      int run(int[] ys) {\n        return ys.length;\n      }\n    }\n    return xs.length;\n  }\n}\n", "3-10"},
		{"java field-initializer anonymous class method is kept", "java", "java", "a.java",
			"class A {\n\n\n\n\n\n\n\n\n\n  Runnable r = new Runnable(){ public void run(){ go(); } };\n}\n", "11-11"},
		{"java constructor anonymous class method is dropped", "java", "java", "a.java",
			"class A {\n  A() {\n    Runnable r = new Runnable(){ public void run(){ go(); } };\n  }\n}\n", ""},
		{"go function needs a body", "go", "go", "a.go",
			"package demo\n\nfunc declared(xs []int)\n\nfunc alpha(xs []int) int {\n\treturn len(xs)\n}\n", "5-7"},
		{"rust closure in a let stays inside", "rust", "rust", "a.rs",
			"fn alpha() {\n    let f = || { let x = 1; };\n}\n", "1-3"},
		{"ts method nested in a function", "typescript", "typescript", "a.ts",
			"export function outer(xs: number[]): number[] {\n  class Hidden {\n    run(ys: number[]): number[] {\n      return ys;\n    }\n  }\n  return xs;\n}\n", "1-8"},
		{"rust keeps helpers, skips tests and signatures", "rust", "rust", "a.rs",
			"fn declared();\n\nfn alpha(xs: Vec<i32>) -> usize {\n    xs.len()\n}\n\nmod helpers {\n    fn beta(xs: Vec<i32>) -> usize {\n        xs.len()\n    }\n}\n\nmod tests {\n    fn gamma(xs: Vec<i32>) -> usize {\n        xs.len()\n    }\n}\n", "3-5 8-10"},
		{"ts keeps methods, skips nested functions", "typescript", "typescript", "a.ts",
			"export class Box {\n  run(xs: number[]): number[] {\n    return xs;\n  }\n}\nexport function outer(xs: number[]): number[] {\n  function inner(ys: number[]): number[] {\n    return ys;\n  }\n  const cb = (zs: number[]) => zs;\n  return inner(xs);\n}\nxs.map((item: number) => item + 1);\n", "2-4 6-12"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			needGrammars(t, c.grammar)
			if got := spans(t, c.language, c.src, c.file); got != c.want {
				t.Fatalf("spans %q, want %q", got, c.want)
			}
		})
	}
}

func TestAPrivateFieldNameIsNotPartOfTheStructure(t *testing.T) {
	needGrammars(t, "typescript")
	entries := forms(t, "typescript", "export class Box {\n  value(): number {\n    return this.#total;\n  }\n  run(xs: number[]): number[] {\n    return this.#hidden(xs);\n  }\n}\n", "a.ts")
	if len(entries) != 2 || entries[0].Span != (Span{"a.ts", 2, 4}) || entries[1].Span != (Span{"a.ts", 5, 7}) {
		t.Fatalf("entries %+v", entries)
	}
	if hasFingerprint(entries, func(s string) bool { return strings.Contains(s, "#total") || strings.Contains(s, "#hidden") }) {
		t.Fatal("a private name leaked into the fingerprints")
	}
}

func entry(file string, start, end int, names []string, language string, nodes int) Entry {
	fps := map[string]struct{}{}
	for _, n := range names {
		fps[n] = struct{}{}
	}
	return Entry{Language: language, Span: Span{file, start, end}, Nodes: nodes, Fingerprints: testFingerprints.strings(fps)}
}

func TestTheSameSpanIsNotAPair(t *testing.T) {
	q := []string{"q"}
	cases := []struct {
		a, b      Entry
		threshold float64
		want      int
	}{
		{entry("a.py", 1, 4, q, "python", 20), entry("a.py", 1, 4, q, "python", 20), 0.0, 0},
		{entry("a.py", 1, 4, q, "python", 20), entry("b.py", 1, 4, q, "python", 20), 0.5, 1},
		{entry("a.py", 1, 5, q, "python", 20), entry("a.py", 3, 5, q, "python", 20), 0.5, 1},
		{entry("a.py", 1, 4, q, "python", 20), entry("a.py", 1, 8, q, "python", 20), 0.5, 1},
	}
	for i, c := range cases {
		if got := len(FindDuplicates([]Entry{c.a, c.b}, c.threshold)); got != c.want {
			t.Errorf("case %d: %d pairs, want %d", i, got, c.want)
		}
	}
}

func TestAScoreEqualToTheThresholdIsKept(t *testing.T) {
	left := entry("a.rs", 1, 4, []string{"x", "y"}, "rust", 20)
	right := entry("b.rs", 1, 4, []string{"x", "z"}, "rust", 20)
	if got := FindDuplicates([]Entry{left, right}, 1.0/3); len(got) != 1 || got[0].Score != 1.0/3 {
		t.Fatalf("at 1/3: %+v", got)
	}
	if got := FindDuplicates([]Entry{left, right}, 1.0/3+0.01); len(got) != 0 {
		t.Fatalf("above 1/3: %+v", got)
	}
}

func TestTheHigherScoreIsReportedFirst(t *testing.T) {
	found := FindDuplicates([]Entry{
		entry("a.rs", 1, 4, []string{"x", "y"}, "rust", 20),
		entry("b.rs", 10, 14, []string{"x", "z"}, "rust", 20),
		entry("a.py", 1, 4, []string{"only"}, "python", 20),
		entry("b.py", 1, 4, []string{"only"}, "python", 20),
	}, 0.3)
	if len(found) != 2 || found[0].Score != 1.0 || found[0].Language != "python" || found[1].Score != 1.0/3 {
		t.Fatalf("found %+v", found)
	}
}

func TestKeepUsesTheLineAndNodeMinimums(t *testing.T) {
	q := []string{"q"}
	if !keep(entry("a.py", 1, 4, q, "python", 20), 4, 20) {
		t.Error("1-4 with 20 nodes is dropped")
	}
	if keep(entry("a.py", 1, 3, q, "python", 20), 4, 20) {
		t.Error("1-3 is kept")
	}
	if keep(entry("a.py", 1, 4, q, "python", 19), 4, 20) {
		t.Error("19 nodes is kept")
	}
}

func TestADeepExpressionIsRead(t *testing.T) {
	expr := strings.Repeat("1 + ", 1199) + "1"
	if entries, w := EntriesInSource(testFingerprints, "python", "def place():\n    return "+expr+"\n", "app.py", "app.py", 0, 0); w != "" || len(entries) != 1 || entries[0].Nodes < 2400 {
		t.Fatalf("python: %d entries, warning %q", len(entries), w)
	}
	nested := strings.Repeat("(inc ", 1200) + "x" + strings.Repeat(")", 1200)
	if entries, w := EntriesInSource(testFingerprints, "clojure", "(defn place [] "+nested+")\n", "app.clj", "app.clj", 0, 0); w != "" || len(entries) != 1 || entries[0].Nodes < 1200*4 {
		t.Fatalf("clojure: %d entries, warning %q", len(entries), w)
	}
}

// TestGoldenNormalizedForms string-compares printed normalized trees with
// spec-dryer §10.1 (recorded from upstream).
func TestGoldenNormalizedForms(t *testing.T) {
	tsBody := `[:statement_block [:lexical_declaration [:variable_declarator :symbol [:symbol "="] [:call_expression [:symbol "filter"] [:arguments :symbol :symbol]]]] [:return_statement [:call_expression [:symbol "map"] [:arguments :symbol :symbol]]]]`
	cases := []struct {
		name, language, file, src string
		index                     int // candidate index
		want                      string
		nodes                     int
	}{
		{"py filter", "python", "a.py", "def alpha(xs):\n    return filter(xs, odd)\n", 0,
			`[:function_definition :symbol [:parameters :symbol] [:block [:return_statement [:call [:symbol "filter"] [:argument_list :symbol :symbol]]]]]`, 19},
		{"py method", "python", "a.py", pyLeft, 0,
			`[:function_definition :symbol [:parameters :symbol :symbol] [:block [:assignment :symbol [:symbol "="] [:call [:symbol "filter"] [:argument_list :symbol :symbol]]] [:return_statement [:call [:symbol "map"] [:argument_list :symbol :symbol]]]]]`, 35},
		{"py f-string", "python", "a.py", "def beta(items):\n    return f\"{items}\"\n", 0,
			`[:function_definition :symbol [:parameters :symbol] [:block [:return_statement [:string [:string_start] [:interpolation :symbol] [:string_end]]]]]`, 19},
		{"go method", "go", "a.go", goLeft, 0,
			`[:method_declaration [:parameter_list [:parameter_declaration :symbol [:pointer_type [:symbol "*"] :symbol]]] :symbol [:parameter_list [:parameter_declaration :symbol [:slice_type :symbol]]] [:slice_type :symbol] [:block [:statement_list [:short_var_declaration [:expression_list :symbol] [:symbol ":="] [:expression_list [:call_expression [:symbol "filter"] [:argument_list :symbol :symbol]]]] [:return_statement [:expression_list [:call_expression [:symbol "mapFn"] [:argument_list :symbol :symbol]]]]]]]`, 61},
		{"go empty", "go", "a.go", "package p\n\nfunc a() {}\n", 0,
			`[:function_declaration :symbol [:parameter_list] [:block]]`, 7},
		{"java method", "java", "a.java", javaLeft, 0,
			`[:method_declaration [:integral_type] :symbol [:formal_parameters [:formal_parameter [:array_type [:integral_type] [:dimensions]] :symbol]] [:block [:local_variable_declaration [:array_type [:integral_type] [:dimensions]] [:variable_declarator :symbol [:symbol "="] [:method_invocation [:symbol "filter"] [:argument_list :symbol :symbol]]]] [:return_statement [:method_invocation [:symbol "map"] [:argument_list :symbol :symbol]]]]]`, 52},
		{"rust fn", "rust", "a.rs", rustLeft, 0,
			`[:function_item [:visibility_modifier] :symbol [:parameters [:parameter :symbol [:generic_type :symbol [:type_arguments [:symbol "<"] [:primitive_type] [:symbol ">"]]]]] [:generic_type :symbol [:type_arguments [:symbol "<"] [:primitive_type] [:symbol ">"]]] [:block [:let_declaration :symbol [:symbol "="] [:call_expression [:symbol "filter"] [:arguments :symbol :symbol]]] [:call_expression [:symbol "map"] [:arguments :symbol :symbol]]]]`, 62},
		{"ts function", "typescript", "a.ts", tsLeft, 0,
			`[:function_declaration :symbol [:formal_parameters [:required_parameter :symbol [:type_annotation [:array_type [:predefined_type]]]]] [:type_annotation [:array_type [:predefined_type]]] ` + tsBody + `]`, 50},
		{"tsx arrow in const", "typescript", "a.tsx", tsxLeft, 0,
			`[:variable_declarator :symbol [:symbol "="] [:arrow_function [:formal_parameters [:required_parameter :symbol [:type_annotation [:array_type [:predefined_type]]]]] [:type_annotation [:array_type [:predefined_type]]] [:symbol "=>"] ` + tsBody + `]]`, 58},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			needGrammars(t, grammarOf(c.language, c.file))
			cands, _, err := Candidates(c.language, c.src, c.file)
			if err != nil || len(cands) <= c.index {
				t.Fatalf("candidates %d, err %v", len(cands), err)
			}
			v := cands[c.index].Norm
			if got := Pr(v); got != c.want {
				t.Fatalf("normalized\n got: %s\nwant: %s", got, c.want)
			}
			if NodeCount(v) != c.nodes {
				t.Fatalf("nodes %d, want %d", NodeCount(v), c.nodes)
			}
		})
	}
}

// TestGoldenSubforms checks printed subtrees of spec §10.1 inside a
// candidate's normalized form.
func TestGoldenSubforms(t *testing.T) {
	cases := []struct {
		name, language, file, src, want string
	}{
		{"java generic method call", "java", "a.java", "class A {\n  void f() {\n    this.<T>foo(1);\n  }\n}\n",
			`[:method_invocation [:this] [:symbol "foo"] [:type_arguments [:symbol "<"] :symbol [:symbol ">"]] [:argument_list :literal]]`},
		{"rust turbofish path", "rust", "a.rs", "fn f() {\n    HashMap::<String, i32>::new();\n}\n",
			`[:call_expression [:scoped_identifier [:generic_type [:symbol "HashMap"] [:type_arguments [:symbol "<"] :symbol [:primitive_type] [:symbol ">"]]] [:symbol "new"]] [:arguments]]`},
		{"ts private member", "typescript", "a.ts", "class B {\n  v() {\n    return this.#total;\n  }\n}\n",
			`[:return_statement [:member_expression [:this] [:private_property_identifier]]]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			needGrammars(t, grammarOf(c.language, c.file))
			cands, _, err := Candidates(c.language, c.src, c.file)
			if err != nil || len(cands) != 1 {
				t.Fatalf("candidates %d, err %v", len(cands), err)
			}
			if _, ok := Fingerprints(cands[0].Norm)[c.want]; !ok {
				t.Fatalf("%s\nnot a subtree of %s", c.want, Pr(cands[0].Norm))
			}
		})
	}
}

// testFingerprints interns the fingerprints of the entries tests build by
// hand or with forms, by printed string so printedFingerprints can read
// them back.
var testFingerprints = NewFingerprinter(true)

// printedFingerprints lists an entry's testFingerprints ids as printed
// fingerprints, in byte order.
func printedFingerprints(e Entry) []string {
	printed := map[uint32]string{}
	for s, id := range testFingerprints.ids {
		printed[id] = s
	}
	set := map[string]struct{}{}
	for _, id := range e.Fingerprints {
		set[printed[id]] = struct{}{}
	}
	return sortedFingerprints(set)
}

// sortedFingerprints lists a fingerprint set in byte order.
func sortedFingerprints(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestShapePrimitives(t *testing.T) {
	if Pr(K("symbol")) != ":symbol" || NodeCount(K("symbol")) != 1 {
		t.Fatal("keyword")
	}
	if NodeCount(List(K("list"), K("symbol"), K("literal"))) != 4 || NodeCount(symbolOf("len")) != 3 {
		t.Fatal("node count")
	}
	if got := Pr(List(K("x"), Str(`a"b\c`))); got != `[:x "a\"b\\c"]` {
		t.Fatalf("escaping %s", got)
	}
	fps := sortedFingerprints(Fingerprints(List(K("call"), symbolOf("select"), List(K("argument_list"), K("symbol")))))
	want := []string{`"select"`, `:argument_list`, `:call`, `:symbol`, `[:argument_list :symbol]`,
		`[:call [:symbol "select"] [:argument_list :symbol]]`, `[:symbol "select"]`}
	if strings.Join(fps, "|") != strings.Join(want, "|") {
		t.Fatalf("fingerprints %q", fps)
	}
	if jaccard(0, 0, 0) != 0.0 {
		t.Fatal("empty jaccard")
	}
}
