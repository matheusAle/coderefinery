package main

// End to end: `test`, `mutate --attribute`, `tests` on the test-ingestion
// fixtures (fixtures/python-tests, fixtures/go-tests), through the
// dispatcher, on a scratch copy.

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/testreport"
)

// copyFixture copies fixtures/<name> (symlinks kept as links) to a temp dir.
// Top-level entries named in linked (e.g. node_modules) are symlinked instead.
func copyFixture(t *testing.T, name string, linked ...string) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("..", "..", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(discover.Resolve(t.TempDir()), name)
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		for _, l := range linked {
			if rel == l {
				if err := os.Symlink(p, target); err != nil {
					return err
				}
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.IsDir():
			if d.Name() == ".metrics" || d.Name() == "__pycache__" || d.Name() == ".pytest_cache" || d.Name() == "target" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o777)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

type relevance struct {
	code           int
	stdout, stderr string
	rows           map[string]testreport.Row
}

// pipeline runs test, mutate --attribute and tests; extra goes to tests.
func pipeline(t *testing.T, root string, mutateArgs []string, extra ...string) relevance {
	t.Helper()
	if code, _, stderr := runCapture([]string{"test", "--root", root}, "v"); code != 0 {
		t.Fatalf("test exit %d:\n%s", code, stderr)
	}
	args := append([]string{"mutate", "--root", root, "--attribute"}, mutateArgs...)
	if code, stdout, stderr := runCapture(args, "v"); code != 3 {
		t.Fatalf("mutate --attribute exit %d (survivors expected: 3):\n%s\n%s", code, stdout, stderr)
	} else if !strings.Contains(stdout, " · by ") {
		t.Fatalf("KILLED lines carry their killer:\n%s", stdout)
	}
	code, stdout, stderr := runCapture(append([]string{"tests", "--root", root}, extra...), "v")
	m, err := testreport.Load(root, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) > 0 && extra[0] == "--group" {
		m = m.ByBase()
	}
	durations := map[string]float64{}
	for _, c := range m.Tests {
		if c.Duration >= 0 {
			durations[c.ID] = c.Duration
		}
	}
	rows := map[string]testreport.Row{}
	for _, r := range testreport.Analyze(m, durations) {
		rows[r.ID] = r
	}
	return relevance{code, stdout, stderr, rows}
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func expectRow(t *testing.T, rows map[string]testreport.Row, id string, kills, unique int, level testreport.Level, inCover bool, shadowedBy ...string) {
	t.Helper()
	r, ok := rows[id]
	if !ok {
		ids := make([]string, 0, len(rows))
		for k := range rows {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		t.Fatalf("no row %s in %v", id, ids)
	}
	sort.Strings(shadowedBy)
	if r.Kills != kills || r.Unique != unique || r.Level != level || r.InCover != inCover || !reflect.DeepEqual(sortedCopy(r.ShadowedBy), shadowedBy) {
		t.Errorf("%s: kills=%d unique=%d level=%s cover=%v shadowed=%v; want kills=%d unique=%d level=%s cover=%v shadowed=%v",
			id, r.Kills, r.Unique, r.Level, r.InCover, r.ShadowedBy, kills, unique, level, inCover, shadowedBy)
	}
}

func TestTestsReportOnThePythonFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("runs pytest and mutants")
	}
	py, _ := filepath.Abs("../../fixtures/python-tests/.venv/bin/python")
	if err := exec.Command(py, "-c", "import pytest, coverage, pytest_cov").Run(); err != nil {
		t.Skip("fixtures/python-tests/.venv missing: run fixtures/setup-tests.sh")
	}
	root := copyFixture(t, "python-tests")
	res := pipeline(t, root, nil)
	const unit = "tests/unit/test_pricing.py::"
	const irrelevant = "tests/test_smoke.py::test_price_returns_a_number"
	const shadowed = "tests/integration/test_checkout.py::test_checkout_gold_member"
	if res.code != 3 {
		t.Fatalf("tests exit %d, want 3:\n%s\n%s", res.code, res.stdout, res.stderr)
	}
	expectRow(t, res.rows, irrelevant, 0, 0, testreport.Unit, false)
	expectRow(t, res.rows, shadowed, 2, 0, testreport.Integration, false, unit+"test_tier_rate[gold]", unit+"test_price_applies_the_rate")
	expectRow(t, res.rows, unit+"test_tier_rate[silver]", 1, 1, testreport.Unit, true)
	expectRow(t, res.rows, unit+"test_tier_rate[gold]", 1, 0, testreport.Unit, true)
	expectRow(t, res.rows, unit+"test_price_applies_the_rate", 2, 1, testreport.Unit, true)
	expectRow(t, res.rows, unit+"test_coupon_subtracts", 2, 1, testreport.Unit, true)
	if r := res.rows[irrelevant]; !r.LevelInferred || r.Runs != 1 {
		t.Errorf("irrelevant test: inferred=%v runs=%d (contexts say it runs price)", r.LevelInferred, r.Runs)
	}
	if r := res.rows[shadowed]; r.Runs != 4 || r.Fns != 2 {
		t.Errorf("integration test runs %d fns, kills in %d forms", r.Runs, r.Fns)
	}
	// smoke: the text report and tests.edn say the same
	for _, want := range []string{
		"Test relevance: 8 tests, 8 killed mutants (8 attributed)\n",
		irrelevant + "                  0      0    0 unit~         (no kills; runs 1 fn)\n",
		"billing.pricing defn/price: unit 3 (test_price_applies_the_rate, test_price_of_nothing_is_zero) · integration 1 (test_checkout_gold_member) · keep unit; integration adds no kills here\n",
		"Offenders: 1 test kills nothing, 1 test is shadowed\n",
	} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("report lacks %q:\n%s", want, res.stdout)
		}
	}
	lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
	if !strings.HasPrefix(lines[2], "------") || !strings.HasPrefix(lines[len(lines)-1], "Offenders:") {
		t.Errorf("layout:\n%s", res.stdout)
	}
	edn, err := os.ReadFile(filepath.Join(root, ".metrics", "tests.edn"))
	if err != nil || !strings.Contains(string(edn), `:rows "case"`) ||
		!strings.Contains(string(edn), `{:id "`+shadowed+`" :file "tests/integration/test_checkout.py" :level "integration" :level-inferred false :kills 2 :unique 0 :fns 2 :runs 4 :cases 1 :in-cover false :shadowed-by ["`) ||
		!strings.Contains(string(edn), `"`+unit+`test_tier_rate[gold]"`) {
		t.Errorf("tests.edn:\n%s", edn)
	}
	snapshot, _ := os.ReadFile(filepath.Join(root, ".metrics", "mutate", "billing.pricing.edn"))
	if !strings.Contains(string(snapshot), `:killed-by {"[\"src/billing/pricing.py\",\"billing.pricing\",\"defn/apply_coupon\",319,320,\"-\",\"+\"]" ["`+unit+`test_coupon_never_goes_negative" "`+unit+`test_coupon_subtracts"]`) {
		t.Errorf("snapshot :killed-by:\n%s", snapshot)
	}

	// --group: the parametrized cases collapse onto test_tier_rate
	code, stdout, _ := runCapture([]string{"tests", "--root", root, "--group"}, "v")
	if code != 3 || !strings.Contains(stdout, unit+"test_tier_rate ") || strings.Contains(stdout, "[gold]") {
		t.Errorf("--group exit %d:\n%s", code, stdout)
	}
	grouped, _ := os.ReadFile(filepath.Join(root, ".metrics", "tests.edn"))
	if !strings.Contains(string(grouped), `{:id "`+unit+`test_tier_rate" :file "tests/unit/test_pricing.py" :level "unit" :level-inferred false :kills 2 :unique 1 :fns 1 :runs 1 :cases 2 :in-cover true`) {
		t.Errorf("grouped tests.edn:\n%s", grouped)
	}
}

func TestTestsReportOnTheGoFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test and mutants")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}
	root := copyFixture(t, "go-tests")
	// --fix-go-coverage: upstream's suffix-only profile lookup misses Go files
	// whose import path does not end in their path (DEVIATIONS M1)
	res := pipeline(t, root, []string{"--fix-go-coverage"})
	const p = "example.com/gotests/pricing::"
	if res.code != 3 {
		t.Fatalf("tests exit %d, want 3:\n%s\n%s", res.code, res.stdout, res.stderr)
	}
	expectRow(t, res.rows, p+"TestPriceReturnsANumber", 0, 0, testreport.Unit, false)
	expectRow(t, res.rows, p+"TestCheckoutGoldMember", 2, 0, testreport.Integration, false, p+"TestTierRate/gold", p+"TestPriceAppliesTheRate")
	expectRow(t, res.rows, p+"TestTierRate/silver", 1, 1, testreport.Unit, true)
	expectRow(t, res.rows, p+"TestCouponNeverGoesNegative", 2, 1, testreport.Unit, true)
	if _, ok := res.rows[p+"TestTierRate"]; ok {
		t.Error("a parent test is never a row")
	}
	if r := res.rows[p+"TestPriceReturnsANumber"]; !r.LevelInferred || r.Runs != -1 || r.File != "pricing/pricing_test.go" {
		t.Errorf("irrelevant go test: %+v", r)
	}
	for _, want := range []string{
		"Test relevance: 8 tests, 8 killed mutants (8 attributed)\n",
		"Offenders: 1 test kills nothing, 1 test is shadowed\n",
		"(no kills)\n",
		"pricing defn/TierRate: unit 2 (TestTierRate/gold, TestTierRate/silver) · integration 1 (TestCheckoutGoldMember) · keep unit; integration adds no kills here\n",
	} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("report lacks %q:\n%s", want, res.stdout)
		}
	}
}

func TestTestsReportOnTheNodeFixtures(t *testing.T) {
	if testing.Short() {
		t.Skip("runs vitest/jest and mutants")
	}
	for _, c := range []struct{ fixture, dir, unit, ns, form string }{
		{"ts-tests", "test", "test/unit/pricing.test.ts::", "pricing", "tierRate"},
		{"js-jest-tests", "__tests__", "__tests__/unit/pricing.test.js::", "lib.pricing", "tierRate"},
	} {
		t.Run(c.fixture, func(t *testing.T) {
			if _, err := os.Stat(filepath.Join("..", "..", "fixtures", c.fixture, "node_modules")); err != nil {
				t.Skip("node_modules missing: run fixtures/setup-tests.sh")
			}
			pinAsdfNode(t)
			root := copyFixture(t, c.fixture)
			res := pipeline(t, root, nil)
			ext := filepath.Ext(strings.TrimSuffix(c.unit, "::"))
			irrelevant := c.dir + "/smoke.test" + ext + "::price returns a number"
			shadowed := c.dir + "/integration/checkout.test" + ext + "::checks out a gold member"
			if res.code != 3 {
				t.Fatalf("tests exit %d, want 3:\n%s\n%s", res.code, res.stdout, res.stderr)
			}
			expectRow(t, res.rows, irrelevant, 0, 0, testreport.Unit, false)
			expectRow(t, res.rows, shadowed, 2, 0, testreport.Integration, false, c.unit+"tierRate::rate for gold", c.unit+"price::applies the rate")
			expectRow(t, res.rows, c.unit+"tierRate::rate for silver", 1, 1, testreport.Unit, true)
			expectRow(t, res.rows, c.unit+"applyCoupon::subtracts", 2, 1, testreport.Unit, true)
			for _, want := range []string{
				"Test relevance: 8 tests, 8 killed mutants (8 attributed)\n",
				c.ns + " defn/" + c.form + ": unit 2 (rate for gold, rate for silver) · integration 1 (checks out a gold member) · keep unit; integration adds no kills here\n",
				"Offenders: 1 test kills nothing, 1 test is shadowed\n",
			} {
				if !strings.Contains(res.stdout, want) {
					t.Errorf("report lacks %q:\n%s", want, res.stdout)
				}
			}
		})
	}
}

// pinAsdfNode: an asdf node shim resolves its version from .tool-versions
// files above the cwd (and ~ — under the repo's go wrapper HOME points into
// .local). Outside the repo it may find none: pin the newest installed
// nodejs by env when node does not start there.
func pinAsdfNode(t *testing.T) {
	probe := exec.Command("node", "--version")
	probe.Dir = t.TempDir() // outside the repo, like the scratch copy
	if probe.Run() == nil || os.Getenv("ASDF_NODEJS_VERSION") != "" {
		return
	}
	shim, err := exec.LookPath("node")
	if err != nil {
		return
	}
	versions, _ := filepath.Glob(filepath.Join(filepath.Dir(filepath.Dir(shim)), "installs", "nodejs", "*"))
	sort.Strings(versions)
	if len(versions) > 0 {
		t.Setenv("ASDF_NODEJS_VERSION", filepath.Base(versions[len(versions)-1]))
	}
}
