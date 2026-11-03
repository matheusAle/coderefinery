package crap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matheusAle/ub-mode/ubgate/internal/coverage"
	"github.com/matheusAle/ub-mode/ubgate/internal/model"
)

func tsFunctions(t *testing.T, source, path, root string) []model.Function {
	t.Helper()
	fns, err := TypeScriptFunctions(source, path, root)
	if err != nil {
		t.Fatalf("TypeScriptFunctions(%s): %v", path, err)
	}
	for _, f := range fns {
		if f.Language != "typescript" || f.Path != path || f.StartByte < 0 || f.EndByte <= f.StartByte {
			t.Fatalf("unexpected metadata %+v", f)
		}
		if got := source[f.StartByte:f.EndByte]; !strings.Contains(got, "=>") && !strings.Contains(got, "(") {
			t.Fatalf("byte span %d-%d does not cover a function: %q", f.StartByte, f.EndByte, got)
		}
	}
	return fns
}

func tsRows(t *testing.T, source, path, root string) []fnRow {
	t.Helper()
	var rows []fnRow
	for _, f := range tsFunctions(t, source, path, root) {
		rows = append(rows, fnRow{f.Namespace, f.Name, f.Complexity})
	}
	return rows
}

// nameCC drops namespaces for the tests that only compare (name, cc).
func nameCC(rows []fnRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%s:%d", r.Name, r.CC))
	}
	return out
}

func expectNames(t *testing.T, rows []fnRow, want ...string) {
	t.Helper()
	if got := nameCC(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("functions:\n got  %q\n want %q", got, want)
	}
}

// Ported from upstream tests/test_typescript.py.
func TestTypeScriptFunctionClassMethodAndArrowAreSeparateEntries(t *testing.T) {
	source := `
export function choose(x: number): number {
  if (x > 0 && x < 10) return 1;
  for (let i = 0; i < x; i++) {
    if (i === 2) return i;
  }
  try {
    return x;
  } catch (e) {
    return 0;
  }
  return x > 0 ? 1 : 0;
}

export class Box {
  open(flag?: boolean): number {
    return flag ? 1 : 0;
  }
}

const arrow = (n: number) => (n > 0 ? n : 0);

function outer() {
  function inner(n: number) {
    if (n) return 1;
    return 0;
  }
  return inner(1);
}
`
	expectRows(t, tsRows(t, source, "src/demo/box.ts", "/proj"),
		fnRow{"demo.box", "choose", 7},
		fnRow{"demo.box.Box", "open", 2},
		fnRow{"demo.box", "arrow", 2},
		fnRow{"demo.box", "outer", 2})
}

func TestTypeScriptTSXCountsJSXLogic(t *testing.T) {
	source := "\nexport function View(ok: boolean, ready: boolean) {\n  return ok && ready ? 1 : 0;\n}\n"
	expectRows(t, tsRows(t, source, "src/ui/view.tsx", "/proj"), fnRow{"ui.view", "View", 3})
}

func TestTypeScriptNullishAndOptionalChainsAreBranches(t *testing.T) {
	source := `
export function choose(a, b, c) {
  return a ?? b?.c ?? c?.() ?? c?.[0];
}
export function text() {
  return "a ?? b?.c";
}
`
	expectNames(t, tsRows(t, source, "src/demo/box.ts", "/proj"), "choose:7", "text:1")
}

func TestTypeScriptExpressHandlersAreEntriesAndLeaveTheParent(t *testing.T) {
	source := `
export function mount(app) {
  const extra = [1].map((n) => (n ? 1 : 0));
  app.get("/users", (req, res) => {
    if (req.query.q) return 1;
    return 0;
  });
  app.post("/users", function (req, res) {
    return req.body ?? {};
  });
  app.use((req, res, next) => next());
  app.route("/items").get((req, res) => res.send(1)).post((req, res) => res.send(2));
  return extra;
}
app.get("/health", async (req, res) => res.send(req.query.ok ?? "no"));
app.get("/users", (req, res, next) => next(), (req, res) => res.send(req.id ?? 0));
app.delete(` + "`/users/${id}`" + `, ((req, res) => res.send(1)));
`
	rows := tsRows(t, source, "src/demo/routes.ts", "/proj")
	expectNames(t, rows, "mount:2", "GET /users:2", "POST /users:2", "USE:1", "GET /items:1",
		"POST /items:1", "GET /health:2", "GET /users#2:1", "GET /users#3:2", "DELETE /users/${id}:1")
	for _, r := range rows {
		if r.Namespace != "demo.routes" {
			t.Fatalf("namespace %q", r.Namespace)
		}
	}
}

func TestTypeScriptNestedNonRouteCallbackStaysInsideItsFunction(t *testing.T) {
	source := "\nfunction outer() {\n  const values = [1].map((n) => (n ? 1 : 0));\n  return values;\n}\n"
	expectNames(t, tsRows(t, source, "src/demo/box.ts", "/proj"), "outer:2")
}

func TestJavaScriptFilesUseTheSameRules(t *testing.T) {
	source := "\nexport function choose(a, b, c) {\n  return a ?? b?.c ?? c?.() ?? c?.[0];\n}\napp.get(\"/users\", (req, res) => req.id ?? 0);\n"
	expectRows(t, tsRows(t, source, "src/demo/app.mjs", "/proj"),
		fnRow{"demo.app", "choose", 7}, fnRow{"demo.app", "GET /users", 2})
	expectNames(t, tsRows(t, source, "src/demo/app.js", "/proj"), "choose:7", "GET /users:2")
}

// Ported from upstream tests/test_edges.py.
func TestTypeScriptNamespacesAndDeclarationsWithoutBodies(t *testing.T) {
	body := "export function choose(): number { return 1 }\n"
	for _, c := range []struct{ path, root string }{
		{"/app/src/demo/box.ts", "/app"}, {"./demo/box.ts", ""}, {"/app/pkg/src/demo/box.mts", ""},
	} {
		if ns := tsFunctions(t, body, c.path, c.root)[0].Namespace; ns != "demo.box" {
			t.Fatalf("%s: namespace %q", c.path, ns)
		}
	}
	source := "\n    function choose(): void;\n    class Box { open(): void; }\n    const plain = 1;\n    const arrow = (n: number) => n;\n    "
	expectRows(t, tsRows(t, source, "src/demo/box.ts", "/proj"), fnRow{"demo.box", "arrow", 1})
	if rows := tsRows(t, "const obj = { open() { return 1 } }\n", "src/demo/box.ts", "/proj"); rows != nil {
		t.Fatalf("object method: %v", rows)
	}
	if rows := tsRows(t, "class Box { [name]() { return 1 } }\n", "src/demo/box.ts", "/proj"); rows != nil {
		t.Fatalf("computed method: %v", rows)
	}
}

// Spec-crapper §8.2 observed cases (namespace `a`, file a.ts unless noted).
func TestTypeScriptObservedCases(t *testing.T) {
	cases := []struct {
		path, source string
		want         []string
	}{
		{"a.ts", "export default function () { if (a) {} }\nexport default function named() { if (a) {} }\n", []string{"named:2"}},
		{"a.js", "export default function () { if (a) {} }\nexport default function named() { if (a) {} }\n", []string{"named:2"}},
		{"a.ts", "function* gen() { if (a) {} }\nasync function af() { if (a) {} }\nvar v = () => { if (a) {} };\nlet l = function () { if (a) {} }, m = async (x) => x ? 1 : 2;\n", []string{"af:2", "l:2", "m:2"}},
		{"a.ts", "function f(a) { return a?.b.c?.d; }\nfunction g(a) { a ??= 1; a ||= 2; a &&= 3; return a; }\n", []string{"f:3", "g:1"}},
		{"a.ts", `app.get("/a\nb", (r) => 1); app.get('/q', (r) => 1); app.get(path, handler); app.get(handler, "/late", function () {}); app.GET("/up", () => 1); cache.get("k", () => 1);`, []string{"GET /ab:1", "GET /q:1", "GET /late:1", "GET /up:1", "GET k:1"}},
		{"a.ts", "function f(xs) { for (const x of xs) {} for (const k in xs) {} for await (const y of xs) {} label: while (1) { break label; } do {} while (0); switch (a) { case 1: case 2: break; default: } }", []string{"f:9"}},
		{"a.ts", `class A { get x() { return a ? 1 : 2; } set x(v) { if (v) {} } static s() {} #p() { if (a) {} } "q"() {} 1() {} }`, []string{"x:2", "x:2", "s:1"}},
		{"a.ts", "const {a, b} = obj;\nconst f = (() => 1);\nconst g = memo(() => a ? 1 : 2);\nif (x) { const h = () => a ? 1 : 2; }\n", []string{"h:2"}},
		{"a.ts", "function o() { function i() { if (a) {} } const k = () => b ? 1 : 2; return class { m() { if (c) {} } }; }", []string{"o:4"}},
		{"a.ts", "namespace N { export function f() { if (a) {} } }\nabstract class B { abstract m(): void; n(): void { if (a) {} } }\ninterface I { m(): void }\nenum E { A }\ntype T<X> = X extends string ? 1 : 2;\nfunction g<T>(x: T): x is string { return (x as any)!.y ?? 1; }\nclass P { handler = () => { if (a) {} }; constructor(private x: number) { if (a) {} } }\ndeclare function d(): void;\nfunction over(a: string): void;\nfunction over(a: any) { if (a) {} }\n", []string{"f:2", "n:2", "g:2", "constructor:2", "over:2"}},
		{"a.tsx", "export function V({ok}) { return <div>{ok && <span>a b c</span>}{ok ? <A/> : null}</div>; }\nconst W = <T,>(x: T) => x ? 1 : 0;\n", []string{"V:3", "W:2"}},
		{"a.jsx", "export function V({ok}) { return <div>{ok && <span>a b</span>}</div>; }", []string{"V:2"}},
		{"a.ts", "const a = 1 / 2 / 3;\nfunction r(s) { return s.replace(/if|&&|\\?\\./g, \"x\") / 2 > 1 ? `t ${a ? `n${b ?? c}` : \"\"}` : 0; }", []string{"r:4"}},
		{"a.ts", "export const x = { a: () => { if (q) {} } };\nexport function f() { const o = { m() { if (q) {} } }; return o; }", []string{"f:2"}},
		{"a.ts", "app.route(\"/items\").get((req, res) => 1).post((req, res) => 2);\napp.route(`/t`).all(() => 1);\napp.use(\"/static\", express.static(\"p\"), (req,res) => 1);\napp.get(\"\", () => 1);", []string{"GET /items:1", "POST /items:1", "ALL /t:1", "USE /static:1", "GET:1"}},
		{"a.ts", `app.get("/x", async function named(req) { if (a) { app.post("/y", () => 1); } });`, []string{"GET /x:2", "POST /y:1"}},
		{"a.ts", "function f(x?: number, y: string = a ? \"1\" : \"2\"): (a?: number) => void { type C<T> = T extends string ? 1 : 2; let q: C<number> | undefined; const o: { a?: number } = {}; return (a?: number) => {}; }", []string{"f:2"}},
		{"a.ts", "function g() { return <any>x && y; }\nfunction h() { return f<string>(x) || g<T, U>(y); }\nfunction k() { return a < b && c > d; }", []string{"g:2", "h:2", "k:2"}},
	}
	for _, c := range cases {
		expectNames(t, tsRows(t, c.source, c.path, ""), c.want...)
	}
}

func TestTypeScriptMethodNamespacesAndDoubleCount(t *testing.T) {
	expectRows(t, tsRows(t, "const K = class { m() { if (a) {} } };\nclass A { m() { return { n() { if (a) {} } }; } }\n", "a.ts", ""),
		fnRow{"a.A", "m", 2}, fnRow{"a.A", "n", 2})
	expectRows(t, tsRows(t, `class S { async run() { router.get("/x", (req) => { if (a) {} }); } }`, "a.ts", ""),
		fnRow{"a.S", "run", 1}, fnRow{"a", "GET /x", 2})
	expectRows(t, tsRows(t, "export default class { m() { if (a) {} } }\nexport default class Named { m() { if (a) {} } }\n@dec class D { m() { if (a) {} } }", "a.ts", ""),
		fnRow{"a.Named", "m", 2}, fnRow{"a.D", "m", 2})
}

func TestTypeScriptLinesAndBytes(t *testing.T) {
	source := "function f() { return a\n  ? b\n  : c; }\nconst g = () =>\n  1;\n"
	fns := tsFunctions(t, source, "a.ts", "")
	if len(fns) != 2 {
		t.Fatalf("functions %+v", fns)
	}
	if f := fns[0]; f.Name != "f" || f.Complexity != 2 || f.StartLine != 1 || f.EndLine != 3 || f.StartByte != 0 || f.EndByte != 38 {
		t.Fatalf("f %+v", f)
	}
	// declarator span: from the identifier to the end of the value
	if g := fns[1]; g.Name != "g" || g.StartLine != 4 || g.EndLine != 5 || source[g.StartByte:g.EndByte] != "g = () =>\n  1" {
		t.Fatalf("g %+v", g)
	}
}

func TestTypeScriptModuleNamespace(t *testing.T) {
	for _, c := range []struct{ path, root, want string }{
		{"src/x/index.ts", "", "x.index"},
		{"src/ui/view.tsx", "/proj", "ui.view"},
		{"lib/queue.cjs", "", "lib.queue"},
		{`win\src\a.d.mts`, "", "a.d"},
	} {
		if got := TypeScriptModuleNamespace(c.path, c.root); got != c.want {
			t.Fatalf("%s: %q, want %q", c.path, got, c.want)
		}
	}
}

func TestTypeScriptParseErrorContributesNothing(t *testing.T) {
	fns, err := TypeScriptFunctions("function (", "a.ts", "")
	if err == nil || fns != nil {
		t.Fatalf("want a parse error, got %v %v", fns, err)
	}
}

func TestTypeScriptDispatchThroughFunctionsInFile(t *testing.T) {
	found, err := FunctionsInFile("typescript", "export function view() { return 1 }\n", "src/ui.ts", "/proj")
	if err != nil || len(found) != 1 || found[0].Name != "view" || found[0].Language != "typescript" {
		t.Fatalf("FunctionsInFile: %+v %v", found, err)
	}
}

// Ported from test_edges.py: missing coverage scores 0% and sorts ahead.
func TestTypeScriptMissingCoverageSortsAheadOfCovered(t *testing.T) {
	dir := t.TempDir()
	body := "export function %s(x: number) {\n  if (x) return 1;\n  return 0;\n}\n"
	low, high := filepath.Join(dir, "src", "low.ts"), filepath.Join(dir, "src", "high.ts")
	if err := os.MkdirAll(filepath.Dir(low), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ path, name string }{{low, "low"}, {high, "high"}} {
		if err := os.WriteFile(f.path, []byte(fmt.Sprintf(body, f.name)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bundle := coverage.NewBundle()
	bundle.Lcov = coverage.ParseLcov("SF:src/low.ts\nDA:1,1\nDA:2,1\nDA:3,1\nend_of_record\n")
	entries, err := AnalyzeFiles([]string{low, high}, dir, bundle)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, fmt.Sprintf("%s %v %v", e.Name, *e.Coverage, *e.Crap))
	}
	if want := []string{"high 0 6", "low 100 2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entries %q, want %q", got, want)
	}
	unscored, err := AnalyzeFiles([]string{high}, dir, nil)
	if err != nil || unscored[0].Coverage != nil || unscored[0].Crap != nil {
		t.Fatalf("unscored %+v %v", unscored, err)
	}
}

// Smoke: both node fixtures through AnalyzeFiles (no coverage), against
// upstream crapper's --no-coverage reports (harness golden).
func TestTypeScriptAndJavaScriptFixtureSmoke(t *testing.T) {
	for _, c := range []struct {
		fixture string
		files   []string
		want    []string
	}{
		{"typescript", []string{"src/badge.tsx", "src/inventory.ts", "src/router.ts", "src/routes.ts", "src/stats.ts"}, []string{
			"shippingZone inventory 5", "Badge badge 4", "restock inventory.Inventory 4", "POST /users routes 4",
			"label inventory 3", "add inventory.Inventory 3", "quantity inventory.Inventory 3", "value inventory.Inventory 3",
			"handle router.Router 3", "DELETE /users/:id routes 3", "sumPositive stats 3", "sumWeights stats 3",
			"discount inventory 2", "GET /users routes 2", "GET /users#2 routes 2", "find inventory.Inventory 1",
			"makeResponse router 1", "delete router.Router 1", "get router.Router 1", "post router.Router 1", "routeCount routes 1",
		}},
		{"javascript", []string{"lib/format.mjs", "lib/queue.js"}, []string{
			"describe lib.format 7", "priority lib.queue 7", "totals lib.format 4", "weights lib.format 4",
			"push lib.queue.Queue 4", "clamp lib.format 3", "plural lib.format 2", "constructor lib.queue.Queue 2",
			"pop lib.queue.Queue 2", "size lib.queue.Queue 1",
		}},
	} {
		root, err := filepath.Abs(filepath.Join("../../../fixtures", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(root); err != nil {
			t.Skip("fixtures not present")
		}
		var files []string
		for _, f := range c.files {
			files = append(files, filepath.Join(root, f))
		}
		entries, err := AnalyzeFiles(files, root, nil)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, e := range entries {
			if e.Language != "typescript" || e.Coverage != nil {
				t.Fatalf("unexpected entry %+v", e)
			}
			got = append(got, fmt.Sprintf("%s %s %d", e.Name, e.Namespace, e.Complexity))
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s entries:\n got  %q\n want %q", c.fixture, got, c.want)
		}
	}
}
