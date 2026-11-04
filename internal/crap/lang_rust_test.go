package crap

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// rustRow is one function as (namespace, name, complexity, start, end).
type rustRow struct {
	Namespace, Name string
	CC, Start, End  int
}

func rustRows(t *testing.T, source, path, root string) []rustRow {
	t.Helper()
	fns, err := RustFunctions(source, path, root)
	if err != nil {
		t.Fatalf("RustFunctions: %v", err)
	}
	var rows []rustRow
	for _, f := range fns {
		if f.Language != "rust" || f.Path != path || f.StartByte != -1 || f.EndByte != -1 {
			t.Fatalf("unexpected metadata %+v", f)
		}
		rows = append(rows, rustRow{f.Namespace, f.Name, f.Complexity, f.StartLine, f.EndLine})
	}
	return rows
}

func expectRust(t *testing.T, got []rustRow, want ...rustRow) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions:\n got  %+v\n want %+v", got, want)
	}
}

// rustCrate writes a Cargo.toml with [package] name into a fresh dir.
func rustCrate(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "Cargo.toml"), "[package]\nname = \""+name+"\"\nversion = \"0.1.0\"\n")
	return root
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

const rustUpstreamSource = `
pub fn choose(x: i32) -> i32 {
    if x > 0 && x < 10 {
        return 1;
    }
    match x {
        1 => 1,
        2 => 2,
        _ => 0,
    }
}

impl Sample {
    pub fn run(&self) -> i32 {
        if self.ok { 1 } else { 0 }
    }
}

impl Trait for Sample {
    fn draw(&mut self) {}
}

mod tests {
    fn hidden() {
        if true {}
    }
}

fn outer() {
    fn inner() {
        if true {}
    }
}
`

// Ported from upstream tests/test_rust.py.
func TestRustFunctionsMethodsAndSkippedTests(t *testing.T) {
	root := rustCrate(t, "demo-game")
	writeFile(t, filepath.Join(root, "src", "lib.rs"), rustUpstreamSource)
	expectRust(t, rustRows(t, rustUpstreamSource, "src/lib.rs", root),
		rustRow{"demo_game", "choose", 6, 2, 11},
		rustRow{"demo_game::Sample", "run", 2, 14, 16},
		rustRow{"demo_game::Sample", "draw", 1, 20, 20},
		rustRow{"demo_game", "outer", 2, 29, 33},
	)
}

func TestRustImplForANestedGenericType(t *testing.T) {
	root := rustCrate(t, "demo")
	source := "impl Trait for Vec<crate::Sample> {\n    fn draw(&self) {}\n}\n"
	expectRust(t, rustRows(t, source, filepath.Join(root, "src", "lib.rs"), root),
		rustRow{"demo::Vec", "draw", 1, 2, 2})
}

func TestRustModulePathsOutsideTheUsualLayout(t *testing.T) {
	root := rustCrate(t, "demo")
	expectRust(t, rustRows(t, "pub fn open() {}\n", filepath.Join(root, "src", "game", "mod.rs"), root),
		rustRow{"demo::game", "open", 1, 1, 1})
	expectRust(t, rustRows(t, "pub fn main_ok() {}\n", filepath.Join(root, "src", "bin", "tool.rs"), root),
		rustRow{"tool", "main_ok", 1, 1, 1})
	expectRust(t, rustRows(t, "pub fn bare() {}\n", filepath.Join(root, "notes.rs"), ""),
		rustRow{"demo::notes", "bare", 1, 1, 1})
	outside := rustRows(t, "pub fn loose() {}\n", "/tmp/loose.rs", root)
	if len(outside) != 1 || outside[0].Name != "loose" || !strings.HasSuffix(outside[0].Namespace, "loose") {
		t.Fatalf("outside the crate: %+v", outside)
	}
}

func TestRustFileOutsideTheCrateRootUsesTheCrateName(t *testing.T) {
	if got := RustFileModules("/tmp/loose.rs", "demo", "/projects/demo"); !reflect.DeepEqual(got, []string{"demo"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRustNestedModulePath(t *testing.T) {
	root := rustCrate(t, "demo")
	source := "pub fn place() -> i32 { if ready { 1 } else { 0 } }\n"
	writeFile(t, filepath.Join(root, "src", "game", "board.rs"), source)
	expectRust(t, rustRows(t, source, "src/game/board.rs", root),
		rustRow{"demo::game::board", "place", 2, 1, 1})
}

// Ported from upstream tests/test_edges.py.
func TestRustRelativePathAndATraitMethod(t *testing.T) {
	root := rustCrate(t, "demo")
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	expectRust(t, rustRows(t, "pub fn bare() {}\n", "notes.rs", ""), rustRow{"crate::notes", "bare", 1, 1, 1})
	source := "trait Foo {\n    fn draw(&self) {}\n}\n"
	expectRust(t, rustRows(t, source, filepath.Join(root, "src", "lib.rs"), root), rustRow{"demo", "draw", 1, 2, 2})
}

// Observed on upstream crapper (spec-crapper §8.5, re-run with the pinned
// tree-sitter): a file x.rs without any Cargo.toml above it.
func TestRustDecisionsMatchUpstream(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.rs")
	cases := []struct {
		source string
		want   []rustRow
	}{
		{`fn f(a: bool) { println!("{}", if a { 1 } else { 2 }); assert!(a && b); vec![x?]; }`,
			[]rustRow{{"crate::x", "f", 1, 1, 1}}},
		{"fn f() { if let Some(x) = y && x > 0 && let Ok(z) = w { } while let Some(v) = it.next() {} }",
			[]rustRow{{"crate::x", "f", 3, 1, 1}}},
		{"fn f() { let c = |x| if x { 1 } else { 2 }; let d = async { a? }; match x { 1 | 2 if g => 1, _ => 2 } }",
			[]rustRow{{"crate::x", "f", 5, 1, 1}}},
		{"fn f() { struct S; impl S { fn m(&self) { if a {} } } }",
			[]rustRow{{"crate::x", "f", 2, 1, 1}, {"crate::x::S", "m", 2, 1, 1}}},
		{"mod a { mod tests { fn t() {} } mod b { fn g() { if a {} else if b {} else {} } } }\n#[cfg(test)]\nmod test { fn h() {} }",
			[]rustRow{{"crate::x::a::b", "g", 3, 1, 1}, {"crate::x::test", "h", 1, 3, 3}}},
		{"impl<T: Clone> fmt::Display for Wrapper<T> where T: X { fn fmt(&self) {} }\nimpl Foo for &Bar { fn a() {} }\nimpl dyn Tr { fn b() {} }\nimpl<T> Foo<T> { fn c() {} }\nimpl crate::a::B { fn d() {} }",
			[]rustRow{{"crate::x::Wrapper", "fmt", 1, 1, 1}, {"crate::x", "a", 1, 2, 2}, {"crate::x", "b", 1, 3, 3}, {"crate::x::Foo", "c", 1, 4, 4}, {"crate::x::B", "d", 1, 5, 5}}},
		{"fn f() { if (a && b) && let Some(x) = y {} while let P = a && b || c {} if let P = (a && b) {} }",
			[]rustRow{{"crate::x", "f", 7, 1, 1}}},
		{"fn f() { let a = || a && b; let b = move || x?; let r = &&x; 'outer: loop { break 'outer; } let t = async { a? }; }",
			[]rustRow{{"crate::x", "f", 5, 1, 1}}},
		{"fn f() { macro_rules! m { ($x:expr) => { if $x { 1 } else { 2 } }; } m!(true); }",
			[]rustRow{{"crate::x", "f", 1, 1, 1}}},
		{"trait T { fn sig(&self); fn body(&self) { if a {} } }\nextern \"C\" { fn ext(); }",
			[]rustRow{{"crate::x", "body", 2, 1, 1}}},
		{"fn f() -> Result<(), E> { let v = g()?.h()?; match v { Ok(_) => {}, Err(e) => return Err(e) } Ok(()) }",
			[]rustRow{{"crate::x", "f", 5, 1, 1}}},
	}
	for _, c := range cases {
		t.Run(c.source[:min(len(c.source), 40)], func(t *testing.T) {
			expectRust(t, rustRows(t, c.source, path, ""), c.want...)
		})
	}
}

func TestRustLinesStartAtTheQualifierNotTheAttribute(t *testing.T) {
	source := "/// doc\n#[inline]\npub(crate) const unsafe fn q(\n    n: u32,\n) -> u32 {\n    n\n}\n"
	expectRust(t, rustRows(t, source, filepath.Join(t.TempDir(), "lib.rs"), ""), rustRow{"crate", "q", 1, 3, 7})
}

func TestRustCrateNameWalksPastWorkspacesAndFallsBackToTheDirectory(t *testing.T) {
	ws, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ws, "Cargo.toml"), "[workspace]\nmembers = [\"app-cli\", \"core\"]\n")
	writeFile(t, filepath.Join(ws, "app-cli", "Cargo.toml"), "[package]\nversion = \"1\"\n\n[dependencies]\nname = \"not-this\"\n")
	writeFile(t, filepath.Join(ws, "core", "Cargo.toml"), "[package]\r\nname\t=  \"ub-core\"\r\n")

	for _, c := range []struct{ file, crate, root string }{
		{filepath.Join(ws, "app-cli", "src", "main.rs"), "app_cli", filepath.Join(ws, "app-cli")},
		{filepath.Join(ws, "core", "src", "deep", "x.rs"), "ub_core", filepath.Join(ws, "core")},
		{filepath.Join(ws, "loose.rs"), "crate", ""},
	} {
		name, root, err := RustCrateName(c.file)
		if err != nil || name != c.crate || root != c.root {
			t.Errorf("%s: got (%q, %q, %v), want (%q, %q)", c.file, name, root, err, c.crate, c.root)
		}
	}
}

func TestRustFileModules(t *testing.T) {
	root := "/projects/demo"
	for _, c := range []struct {
		path string
		want []string
	}{
		{root + "/src/lib.rs", []string{"demo"}},
		{root + "/src/main.rs", []string{"demo"}},
		{root + "/src/game/mod.rs", []string{"demo", "game"}},
		{root + "/src/game/board.rs", []string{"demo", "game", "board"}},
		{root + "/src/bin/tool.rs", []string{"tool"}},
		{root + "/src/bin/tool/main.rs", []string{"tool", "main"}},
		{root + "/benches/x.rs", []string{"demo", "benches", "x"}},
		{root + "/src/mod.rs", []string{"demo"}},
	} {
		if got := RustFileModules(c.path, "demo", root); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.path, got, c.want)
		}
	}
	if got := RustFileModules("/elsewhere/notes.rs", "crate", ""); !reflect.DeepEqual(got, []string{"crate", "notes"}) {
		t.Errorf("no crate root: got %v", got)
	}
	if got := RustFileModules("/elsewhere/main.rs", "crate", ""); !reflect.DeepEqual(got, []string{"crate"}) {
		t.Errorf("no crate root, main.rs: got %v", got)
	}
}

func TestRustUnparsableFileContributesNothing(t *testing.T) {
	fns, err := RustFunctions("fn f( {", filepath.Join(t.TempDir(), "x.rs"), "")
	if err == nil || fns != nil {
		t.Fatalf("got %v, %v; want a parse error and no functions", fns, err)
	}
}

const e2eRustReport = `CRAP Report
===========
Languages: rust

Function                       Namespace                             CC    Cov%     CRAP
----------------------------------------------------------------------------------------
choose                         ub_demo                                5  71.4%      5.6
main                           tool                                   2  33.3%      3.2
place                          ub_demo::game::board                   2  50.0%      2.5
run                            ub_demo::Sample                        2 100.0%      2.0
`

const e2eRustEDN = `{:entries [
  {:name "choose", :namespace "ub_demo", :complexity 5, :coverage 71.4286, :crap 5.5831}
  {:name "main", :namespace "tool", :complexity 2, :coverage 33.3333, :crap 3.1852}
  {:name "place", :namespace "ub_demo::game::board", :complexity 2, :coverage 50.0, :crap 2.5}
  {:name "run", :namespace "ub_demo::Sample", :complexity 2, :coverage 100.0, :crap 2.0}
]}
`

// TestSmokeRustLcovMatchesUpstream runs the whole pipeline on
// testdata/e2e_rust (relative and absolute-suffix SF keys, a branch record
// that wins over line hits, `mod tests` hidden) against output recorded
// from upstream crapper --use-existing-coverage on the same tree.
func TestSmokeRustLcovMatchesUpstream(t *testing.T) {
	root := copyTree(t, "e2e_rust")
	code, stdout, stderr := runCrap("--root", root, "--use-existing-coverage", "--threshold", "3")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr %s", code, stderr)
	}
	if stdout != e2eRustReport {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, e2eRustReport)
	}
	metrics := filepath.Join(root, ".metrics", "crap.edn")
	if want := "Wrote " + metrics + "\nCRAP threshold exceeded: 5.6 > 3.0\n"; stderr != want {
		t.Fatalf("stderr %q, want %q", stderr, want)
	}
	edn, err := os.ReadFile(metrics)
	if err != nil || string(edn) != e2eRustEDN {
		t.Fatalf("crap.edn:\n%s\nwant:\n%s (%v)", edn, e2eRustEDN, err)
	}
}
