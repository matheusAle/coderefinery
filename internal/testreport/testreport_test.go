package testreport

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/testresult"
)

func TestLevelOfPathsAndNames(t *testing.T) {
	cases := map[string]Level{
		"tests/e2e/test_checkout.py":              E2E,
		"cypress/checkout.cy.ts":                  E2E,
		"src/cart.e2e.test.ts":                    E2E,
		"tests/test_cart_e2e.py":                  E2E,
		"tests/api/test_users.py":                 API,
		"tests/test_api_users.py":                 API,
		"pkg/users_api_test.go":                   API,
		"src/users.api.spec.ts":                   API,
		"tests/integration/test_checkout.py":      Integration,
		"src/test/java/com/x/CheckoutIT.java":     Integration,
		"src/test/java/com/x/CheckoutITCase.java": Integration,
		"pricing/checkout_integration_test.go":    Integration,
		"tests/test_integration_db.py":            Integration,
		"crate/tests/checkout.rs":                 Integration,
		"test/unit/pricing.test.ts":               Unit,
		"src/pricing.unit.test.ts":                Unit,
		"Tests/Unit/PricingTest.php":              Unit,
	}
	for file, want := range cases {
		if got, ok := LevelOf(file); !ok || got != want {
			t.Errorf("%s: %q %v, want %q", file, got, ok, want)
		}
	}
	for _, file := range []string{"tests/test_pricing.py", "pricing/pricing_test.go", "src/test/java/com/x/SplitTest.java", "", "example.com/m/p"} {
		if got, ok := LevelOf(file); ok {
			t.Errorf("%s is inferred, got %q", file, got)
		}
	}
}

// matrix: mutants 0..n-1, kills per test id.
func matrix(n int, kills map[string][]int, levels map[string]string) *Matrix {
	m := &Matrix{Kills: map[string][]int{}, Unattributed: map[string]int{}, Flaky: map[string]bool{}, Attributed: true}
	for i := 0; i < n; i++ {
		m.Mutants = append(m.Mutants, Mutant{ID: string(rune('a' + i)), Namespace: "ns", Form: "defn/f", Killed: true})
	}
	for id, ks := range kills {
		m.Kills[id] = ks
		for _, k := range ks {
			m.Mutants[k].KilledBy = append(m.Mutants[k].KilledBy, id)
		}
		file := levels[id]
		m.Tests = append(m.Tests, testresult.Case{ID: id, Base: id, File: file, Locator: file, Name: id})
	}
	return m
}

func byID(rows []Row) map[string]Row {
	out := map[string]Row{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

func TestCoverTieBreaksLevelThenDurationThenID(t *testing.T) {
	files := map[string]string{"u1": "tests/unit/a.py", "u2": "tests/unit/b.py", "i": "tests/integration/c.py", "e": "tests/e2e/d.py"}
	// u1, u2 and i kill the same {0,1}; e kills {0,1,2}: e has a unique kill
	m := matrix(3, map[string][]int{"u1": {0, 1}, "u2": {0, 1}, "i": {0, 1}, "e": {0, 1, 2}}, files)
	rows := byID(Analyze(m, map[string]float64{"u1": 0.5, "u2": 0.1, "i": 0.01}))
	if !rows["e"].InCover || rows["e"].Unique != 1 || rows["e"].Kills != 3 {
		t.Fatalf("e: %+v", rows["e"])
	}
	for _, id := range []string{"u1", "u2", "i"} {
		if rows[id].InCover || !reflect.DeepEqual(rows[id].ShadowedBy, []string{"e"}) || !rows[id].Offender() {
			t.Errorf("%s: %+v", id, rows[id])
		}
	}
	// without e the unit tests win over the faster integration test; then the shorter unit
	m = matrix(2, map[string][]int{"u1": {0, 1}, "u2": {0, 1}, "i": {0, 1}}, files)
	rows = byID(Analyze(m, map[string]float64{"u1": 0.5, "u2": 0.1, "i": 0.01}))
	if !rows["u2"].InCover || rows["u2"].Offender() || rows["u1"].InCover || rows["i"].InCover ||
		!reflect.DeepEqual(rows["i"].ShadowedBy, []string{"u2"}) {
		t.Fatalf("level then duration: %+v", rows)
	}
	// equal durations (and missing ones = +Inf) fall back to the ID
	rows = byID(Analyze(m, nil))
	if !rows["u1"].InCover || rows["u2"].InCover || !reflect.DeepEqual(rows["u2"].ShadowedBy, []string{"u1"}) || rows["u1"].Duration != math.Inf(1) {
		t.Fatalf("ID tie-break: %+v", rows)
	}
}

func TestShadowedRowsCanBeDeletedWithoutLosingKills(t *testing.T) {
	m := matrix(4, map[string][]int{"a": {0, 1}, "b": {2, 3}, "c": {1, 2}, "d": {}, "f": {3}}, nil)
	m.Flaky["f"] = true
	rows := byID(Analyze(m, nil))
	if !reflect.DeepEqual(rows["c"].ShadowedBy, []string{"a", "b"}) || rows["c"].Unique != 0 {
		t.Fatalf("c: %+v", rows["c"])
	}
	if rows["d"].Kills != 0 || !rows["d"].Offender() || rows["d"].LevelInferred != true || rows["d"].Level != Unit {
		t.Fatalf("d: %+v", rows["d"])
	}
	if r := rows["f"]; !r.Flaky || r.Offender() || r.InCover {
		t.Fatalf("flaky rows stay out of the math: %+v", r)
	}
	if rows["b"].Unique != 1 { // 2 is shared with c; 3 only with the flaky f, which does not count
		t.Fatalf("b unique %d", rows["b"].Unique)
	}
	kept := map[int]bool{}
	for _, r := range rows {
		if !r.Offender() && !r.Flaky {
			for _, k := range r.kills {
				kept[k] = true
			}
		}
	}
	if len(kept) != 4 {
		t.Fatalf("deleting the offenders lost kills: %v", kept)
	}
}

func TestByBaseCollapsesParametrizedCases(t *testing.T) {
	m := matrix(3, map[string][]int{"t.py::rate[gold]": {0}, "t.py::rate[silver]": {1}, "t.py::other": {1, 2}}, nil)
	for i := range m.Tests {
		c := &m.Tests[i]
		c.File, c.Locator = "t.py", "t.py"
		c.Name = strings.TrimPrefix(c.ID, "t.py::")
		c.Base = strings.Split(c.ID, "[")[0]
		c.Duration = 0.25
	}
	g := m.ByBase()
	if len(g.Tests) != 2 || g.Tests[0].ID != "t.py::other" || g.Tests[1].ID != "t.py::rate" || g.Tests[1].Duration != 0.5 || g.Tests[1].Name != "rate" {
		t.Fatalf("groups %+v", g.Tests)
	}
	rows := byID(Analyze(g, nil))
	if r := rows["t.py::rate"]; r.Kills != 2 || r.Unique != 1 || r.Cases != 2 || !r.InCover {
		t.Fatalf("group row %+v", r)
	}
}

func TestSynthesizedCasesForKillersWithoutResults(t *testing.T) {
	c := synthesize("tests/t.py::TestX::test_a[1]")
	if c.File != "tests/t.py" || c.Name != "test_a[1]" || c.Base != "tests/t.py::TestX::test_a" || !reflect.DeepEqual(c.Suite, []string{"TestX"}) || c.Duration != -1 {
		t.Fatalf("%+v", c)
	}
	g := synthesize("example.com/m/p::TestTier/gold")
	if g.File != "" || g.Locator != "example.com/m/p" || g.Base != "example.com/m/p::TestTier" {
		t.Fatalf("%+v", g)
	}
}

func snapshotWith(t *testing.T, root, body string) {
	t.Helper()
	p := filepath.Join(root, ".metrics", "mutate", "demo.edn")
	_ = os.MkdirAll(filepath.Dir(p), 0o777)
	if err := os.WriteFile(p, []byte(body), 0o666); err != nil {
		t.Fatal(err)
	}
}

const mid = `"[\"src/demo.py\",\"demo\",\"defn/add\",%d,%d,\"+\",\"-\"]"`

func id(n int) string {
	return strings.Replace(strings.Replace(mid, "%d", itoa(n), 1), "%d", itoa(n+1), 1)
}

func itoa(n int) string { return strings.TrimSpace(strings.Repeat(" ", 0) + string(rune('0'+n))) }

func runTests(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	code := Main(append([]string{"--root", root}, args...), &out, &errb)
	return code, out.String(), errb.String()
}

func TestExitCodesAndFooters(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	snapshotWith(t, root, `{:version 1 :tested-at "x" :source "src/demo.py" :namespace "demo" :outcomes {`+id(1)+` :killed}}`)
	if code, _, stderr := runTests(t, root); code != 1 || stderr != "no attribution data: run coderefinery mutate --attribute\n" {
		t.Fatalf("exit %d %q", code, stderr)
	}
	snapshotWith(t, root, `{:version 1 :tested-at "x" :source "src/demo.py" :namespace "demo" :outcomes {`+id(1)+` :killed `+id(3)+` :killed `+id(5)+` :killed `+id(7)+` :survived} :forms [] :killed-by {`+
		id(1)+` ["tests/t.py::test_a"] `+id(3)+` ["<timeout>"] `+id(5)+` ["tests/t.py::test_a" "tests/t.py::test_b"]}}`)
	code, stdout, stderr := runTests(t, root)
	if code != 3 {
		t.Fatalf("test_b is shadowed: exit %d\n%s%s", code, stdout, stderr)
	}
	want := "Test relevance: 2 tests, 3 killed mutants (2 attributed)\n" +
		"Test               Kills Unique  Fns Level         Shadowed by\n" +
		"--------------------------------------------------------------\n" +
		"tests/t.py::test_a     2      1    1 unit~         -\n" +
		"tests/t.py::test_b     1      0    1 unit~         tests/t.py::test_a\n" +
		"\nUnattributed kills: 1 <timeout>, 0 <build>\n" +
		"Offenders: 0 tests kill nothing, 1 test is shadowed\n"
	if stdout != want {
		t.Fatalf("report:\n%s\nwant\n%s", stdout, want)
	}
	if !strings.HasSuffix(stderr, "Wrote "+filepath.Join(root, ".metrics", "tests.edn")+"\n") {
		t.Fatalf("stderr %q", stderr)
	}
	edn, _ := os.ReadFile(filepath.Join(root, ".metrics", "tests.edn"))
	if !strings.Contains(string(edn), `{:id "tests/t.py::test_b" :file "tests/t.py" :level "unit" :level-inferred true :kills 1 :unique 0 :fns 1 :runs -1 :cases 1 :in-cover false :shadowed-by ["tests/t.py::test_a"] :flaky false :duration nil}`) ||
		!strings.HasSuffix(string(edn), "] :levels [\n] :unattributed {:timeout 1 :build 0}}\n") {
		t.Fatalf("tests.edn:\n%s", edn)
	}

	// results that fail: fix the tests first; killer ids missing from them are stale
	results := filepath.Join(root, ".metrics", "tests", "python", "results.xml")
	_ = os.MkdirAll(filepath.Dir(results), 0o777)
	_ = os.WriteFile(results, []byte(`<testsuites name="pytest tests"><testsuite name="pytest"><testcase classname="tests.t" name="test_a" file="tests/t.py" time="0.1"><failure message="x"/></testcase></testsuite></testsuites>`), 0o666)
	_ = os.WriteFile(filepath.Join(root, ".metrics", "tests", "runs.edn"), []byte("{:version 1 :tested-at \"x\" :runs [\n  {:language \"python\" :module \".\" :tool \"pytest\" :results \".metrics/tests/python/results.xml\" :flaky []}\n]}\n"), 0o666)
	if code, _, stderr := runTests(t, root); code != 2 || !strings.Contains(stderr, "the latest test results have failures: tests/t.py::test_a\nfix the tests first\n") {
		t.Fatalf("exit %d %q", code, stderr)
	}
	_ = os.WriteFile(filepath.Join(root, ".metrics", "tests", "runs.edn"), []byte("{:version 1 :tested-at \"x\" :runs [\n  {:language \"python\" :module \".\" :tool \"pytest\" :results \".metrics/tests/python/results.xml\" :flaky [\"tests/t.py::test_a\"]}\n]}\n"), 0o666)
	code, stdout, stderr = runTests(t, root)
	if code != 0 || !strings.Contains(stderr, "stale test id tests/t.py::test_b\n") || !strings.Contains(stdout, "Flaky (left out of the math; fix them, do not delete them)\ntests/t.py::test_a\n") {
		t.Fatalf("flaky failure is not a failure; test_b is stale: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, _, stderr := runTests(t, root, "extra"); code != 1 || !strings.HasPrefix(stderr, "Unexpected argument: extra\n\n") {
		t.Fatalf("usage %d %q", code, stderr)
	}
}
