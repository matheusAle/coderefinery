package testresult

import (
	"reflect"
	"strings"
	"testing"
)

// The samples under testdata/ were written by the real tools (pytest 9.2,
// vitest 3.2, jest 30 + jest-junit 16, mocha 11 + mocha-junit-reporter 2,
// go 1.26), except surefire/, gradle-*, nextest-*, kaocha-* and test2junit-*,
// which are hand-written from the documented formats (no mvn, gradle,
// cargo-nextest or clojure on the build machine).

const mochaRoot = "/Users/matheusale/workspaces/ub-mode/.local/scratch/samples/mochazoo"

func load(t *testing.T, name, root, hint string) *Run {
	t.Helper()
	r, err := Load("testdata/"+name, root, hint)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

func ids(r *Run) []string {
	var out []string
	for _, c := range r.Cases {
		out = append(out, c.ID)
	}
	return out
}

func get(t *testing.T, r *Run, id string) Case {
	t.Helper()
	for _, c := range r.Cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no case %q in %v", id, ids(r))
	return Case{}
}

func expectCase(t *testing.T, got Case, file string, suite []string, name, base string, status Status) {
	t.Helper()
	if got.File != file || !reflect.DeepEqual(got.Suite, suite) || got.Name != name || got.Base != base || got.Status != status {
		t.Errorf("%s:\n got file=%q suite=%q name=%q base=%q status=%s\nwant file=%q suite=%q name=%q base=%q status=%s",
			got.ID, got.File, got.Suite, got.Name, got.Base, got.Status, file, suite, name, base, status)
	}
}

func expectIDs(t *testing.T, r *Run, want ...string) {
	t.Helper()
	if got := ids(r); !reflect.DeepEqual(got, want) {
		t.Fatalf("ids:\n got %q\nwant %q", got, want)
	}
}

func TestPytestXunit1IDsAreNodeIDs(t *testing.T) {
	r := load(t, "pytest-xunit1.xml", "testdata/pyzoo", "")
	if r.Tool != "pytest" || r.BuildFailed {
		t.Fatalf("tool %s build %v", r.Tool, r.BuildFailed)
	}
	expectIDs(t, r,
		"tests/sub/test_deep.py::test_deep",
		"tests/test_zoo.py::TestGroup::TestNested::test_inner",
		"tests/test_zoo.py::TestGroup::test_method",
		"tests/test_zoo.py::test_error",
		"tests/test_zoo.py::test_fail",
		"tests/test_zoo.py::test_param[1]",
		"tests/test_zoo.py::test_param[2]",
		"tests/test_zoo.py::test_param[a b]",
		"tests/test_zoo.py::test_pass",
		"tests/test_zoo.py::test_skip")
	expectCase(t, get(t, r, "tests/test_zoo.py::TestGroup::TestNested::test_inner"), "tests/test_zoo.py", []string{"TestGroup", "TestNested"}, "test_inner", "tests/test_zoo.py::TestGroup::TestNested::test_inner", Pass)
	expectCase(t, get(t, r, "tests/test_zoo.py::test_param[a b]"), "tests/test_zoo.py", nil, "test_param[a b]", "tests/test_zoo.py::test_param", Pass)
	expectCase(t, get(t, r, "tests/test_zoo.py::test_skip"), "tests/test_zoo.py", nil, "test_skip", "tests/test_zoo.py::test_skip", Skip)
	fail := get(t, r, "tests/test_zoo.py::test_fail")
	if fail.Status != Fail || !strings.HasPrefix(fail.Failure, "AssertionError: one is not two") || !strings.Contains(fail.Failure, "tests/test_zoo.py:9: AssertionError") {
		t.Errorf("failure %q", fail.Failure)
	}
	if e := get(t, r, "tests/test_zoo.py::test_error"); e.Status != Error || !strings.Contains(e.Failure, "fixture exploded") {
		t.Errorf("error case %+v", e)
	}
	if got := r.Failing(); !reflect.DeepEqual(got, []string{"tests/test_zoo.py::test_error", "tests/test_zoo.py::test_fail"}) {
		t.Errorf("failing %v", got)
	}
	if p, f, s, e := r.Counts(); p != 7 || f != 1 || s != 1 || e != 1 {
		t.Errorf("counts %d %d %d %d", p, f, s, e)
	}
}

func TestPytestXunit2ResolvesTheModuleFromTheClassname(t *testing.T) {
	x1 := load(t, "pytest-xunit1.xml", "testdata/pyzoo", "")
	x2 := load(t, "pytest-xunit2.xml", "testdata/pyzoo", "")
	if !reflect.DeepEqual(ids(x1), ids(x2)) {
		t.Fatalf("xunit2 ids differ:\n%q\n%q", ids(x1), ids(x2))
	}
	if len(x2.Cases) != 10 {
		t.Fatalf("smoke: %d cases", len(x2.Cases))
	}
}

func TestPytestCollectionErrorIsABuildFailure(t *testing.T) {
	r := load(t, "pytest-collect.xml", "testdata/pyzoo", "pytest")
	if !r.BuildFailed || len(r.Cases) != 0 || len(r.Failing()) != 0 {
		t.Fatalf("collect %+v", r)
	}
}

func TestVitestSplitsDescribesAndDropsSuiteLoadFailures(t *testing.T) {
	r := load(t, "vitest.xml", "", "")
	if r.Tool != "vitest" || !r.BuildFailed {
		t.Fatalf("tool %s build %v (broken.test.ts failed to load)", r.Tool, r.BuildFailed)
	}
	expectIDs(t, r,
		"test/deep/err.test.ts::throws",
		"test/math.test.ts::add::adds",
		"test/math.test.ts::add::fails",
		"test/math.test.ts::add::nested::deeper::each 1",
		"test/math.test.ts::add::nested::deeper::each 2",
		"test/math.test.ts::add::skipped",
		"test/math.test.ts::top level")
	expectCase(t, get(t, r, "test/math.test.ts::add::nested::deeper::each 2"), "test/math.test.ts", []string{"add", "nested", "deeper"}, "each 2", "test/math.test.ts::add::nested::deeper::each 2", Pass)
	if c := get(t, r, "test/math.test.ts::add::fails"); c.Failure == "" || !strings.HasPrefix(c.Failure, "expected 3 to be 4") || c.Duration <= 0 {
		t.Errorf("fails %+v", c)
	}
	if got := r.Failing(); !reflect.DeepEqual(got, []string{"test/deep/err.test.ts::throws", "test/math.test.ts::add::fails"}) {
		t.Errorf("failing %v", got)
	}
}

func TestJestJunitWithTheWrapperTemplates(t *testing.T) {
	r := load(t, "jest-junit.xml", "", "")
	if r.Tool != "jest" || !r.BuildFailed {
		t.Fatalf("tool %s build %v", r.Tool, r.BuildFailed)
	}
	expectIDs(t, r,
		"__tests__/math.test.js::add::adds",
		"__tests__/math.test.js::add::fails",
		"__tests__/math.test.js::add::nested::each 1",
		"__tests__/math.test.js::add::nested::each 2",
		"__tests__/math.test.js::add::skipped",
		"__tests__/math.test.js::top level")
	expectCase(t, get(t, r, "__tests__/math.test.js::add::nested::each 1"), "__tests__/math.test.js", []string{"add", "nested"}, "each 1", "__tests__/math.test.js::add::nested::each 1", Pass)
	expectCase(t, get(t, r, "__tests__/math.test.js::add::skipped"), "__tests__/math.test.js", []string{"add"}, "skipped", "__tests__/math.test.js::add::skipped", Skip)
}

func TestJestJunitDefaultTemplatesKeepTheWholeTitle(t *testing.T) {
	r := load(t, "jest-junit-default.xml", "", "")
	if r.BuildFailed || len(r.Cases) != 6 {
		t.Fatalf("smoke %d cases", len(r.Cases))
	}
	expectCase(t, get(t, r, "__tests__/math.test.js::add nested each 2"), "__tests__/math.test.js", nil, "add nested each 2", "__tests__/math.test.js::add nested each 2", Pass)
}

func TestMochaJunitReporterFullSuiteTitles(t *testing.T) {
	r := load(t, "mocha-junit.xml", mochaRoot, "")
	if r.Tool != "mocha" {
		t.Fatalf("tool %s", r.Tool)
	}
	expectIDs(t, r,
		`test/deep/hook.test.js::hooks::"before all" hook for "never runs"`,
		"test/math.test.js::add::adds",
		"test/math.test.js::add::fails",
		"test/math.test.js::add::nested::each 1",
		"test/math.test.js::add::nested::each 2",
		"test/math.test.js::add::skipped",
		"top level")
	expectCase(t, get(t, r, "test/math.test.js::add::nested::each 1"), "test/math.test.js", []string{"add", "nested"}, "each 1", "test/math.test.js::add::nested::each 1", Pass)
	if c := get(t, r, "test/math.test.js::add::fails"); c.Status != Fail || !strings.Contains(c.Failure, "3 !== 4") {
		t.Errorf("fails %+v", c)
	}
	d := load(t, "mocha-junit-default.xml", mochaRoot, "")
	expectCase(t, get(t, d, "test/math.test.js::add nested::each 2"), "test/math.test.js", []string{"add nested"}, "each 2", "test/math.test.js::add nested::each 2", Pass)
}

func TestSurefireDirResolvesTheTestFile(t *testing.T) {
	r := load(t, "surefire", "testdata/javazoo", "")
	if r.Tool != "surefire" {
		t.Fatalf("tool %s", r.Tool)
	}
	const f = "src/test/java/com/example/CalcTest.java"
	expectIDs(t, r,
		"com.example.MissingTest::works",
		f+"::CalcTest::Nested::[1] gold",
		f+"::CalcTest::Nested::inner",
		f+"::CalcTest::adds",
		f+"::CalcTest::explodes",
		f+"::CalcTest::fails",
		f+"::CalcTest::rate(int, double)[1]",
		f+"::CalcTest::rate(int, double)[2]",
		f+"::CalcTest::skipped")
	expectCase(t, get(t, r, f+"::CalcTest::rate(int, double)[2]"), f, []string{"CalcTest"}, "rate(int, double)[2]", f+"::CalcTest::rate", Pass)
	expectCase(t, get(t, r, f+"::CalcTest::Nested::inner"), f, []string{"CalcTest", "Nested"}, "inner", f+"::CalcTest::Nested::inner", Pass)
	expectCase(t, get(t, r, "com.example.MissingTest::works"), "", nil, "works", "com.example.MissingTest::works", Pass)
	if e := get(t, r, f+"::CalcTest::explodes"); e.Status != Error || !strings.HasPrefix(e.Failure, "boom\njava.lang.IllegalStateException: boom") {
		t.Errorf("explodes %q", e.Failure)
	}
}

func TestGradleStripsParensAndFindsKotlin(t *testing.T) {
	r := load(t, "gradle-TEST-com.example.RateTest.xml", "testdata/javazoo/module", "")
	if r.Tool != "gradle" {
		t.Fatalf("tool %s", r.Tool)
	}
	const f = "src/test/kotlin/com/example/RateTest.kt"
	expectIDs(t, r, f+"::RateTest::gold", f+"::RateTest::later", f+"::RateTest::rate(int, double)[1]", f+"::RateTest::silver")
	expectCase(t, get(t, r, f+"::RateTest::silver"), f, []string{"RateTest"}, "silver", f+"::RateTest::silver", Fail)
	expectCase(t, get(t, r, f+"::RateTest::rate(int, double)[1]"), f, []string{"RateTest"}, "rate(int, double)[1]", f+"::RateTest::rate", Pass)
}

func TestNextestBinaryIDsAndRstestBases(t *testing.T) {
	r := load(t, "nextest-junit.xml", "testdata/rustzoo", "")
	if r.Tool != "nextest" {
		t.Fatalf("tool %s", r.Tool)
	}
	expectIDs(t, r, "calc::bin/calc-cli::tests::slow", "calc::tests::adds", "calc::tests::fails",
		"calc::tests::rate::case_1_gold", "calc::tests::rate::case_2_silver", "tests/integration.rs::checkout_works")
	expectCase(t, get(t, r, "calc::tests::rate::case_2_silver"), "", nil, "tests::rate::case_2_silver", "calc::tests::rate", Pass)
	expectCase(t, get(t, r, "tests/integration.rs::checkout_works"), "tests/integration.rs", nil, "checkout_works", "tests/integration.rs::checkout_works", Pass)
	if c := get(t, r, "calc::bin/calc-cli::tests::slow"); c.Status != Fail || c.Failure != "test timed out" || c.Duration != 60.001 {
		t.Errorf("timeout %+v", c)
	}
}

func TestKaochaAndTest2junitResolveTheNamespace(t *testing.T) {
	const f = "test/billing/pricing_test.clj"
	k := load(t, "kaocha-junit.xml", "testdata/cljzoo", "kaocha")
	expectIDs(t, k, "billing.missing-test::works-test", f+"::boom-test", f+"::price-test", f+"::tier-rate-test")
	expectCase(t, get(t, k, f+"::boom-test"), f, nil, "boom-test", f+"::boom-test", Error)
	if sniffed := load(t, "kaocha-junit.xml", "testdata/cljzoo", ""); sniffed.Tool != "kaocha" || !reflect.DeepEqual(ids(sniffed), ids(k)) {
		t.Errorf("sniffed %s %v", sniffed.Tool, ids(sniffed))
	}
	t2 := load(t, "test2junit-billing.pricing-test.xml", "testdata/cljzoo", "test2junit")
	expectIDs(t, t2, f+"::price-test", f+"::tier-rate-test")
	if c := get(t, t2, f+"::price-test"); c.Status != Fail || !strings.Contains(c.Failure, "actual: (not (= 160 250.0))") {
		t.Errorf("test2junit failure %q", c.Failure)
	}
}

func TestGoJSONSubtestsParentsAndBuildFailure(t *testing.T) {
	r := load(t, "go-zoo.json", "testdata/gozoo", "")
	if r.Tool != "go" || !r.BuildFailed {
		t.Fatalf("tool %s build %v (package broken does not compile)", r.Tool, r.BuildFailed)
	}
	const p = "example.com/zoo/ok"
	expectIDs(t, r, p+"::TestFail", p+"::TestPass", p+"::TestSkip", p+"::TestTable", p+"::TestTable/nested",
		p+"::TestTable/nested/deep", p+"::TestTable/one", p+"::TestTable/two_words")
	parent := get(t, r, p+"::TestTable")
	if !parent.Parent || parent.Status != Fail || parent.File != "ok/ok_test.go" || parent.Locator != p {
		t.Errorf("parent %+v", parent)
	}
	expectCase(t, get(t, r, p+"::TestTable/two_words"), "ok/ok_test.go", nil, "TestTable/two_words", p+"::TestTable", Fail)
	if !get(t, r, p+"::TestTable/nested").Parent || get(t, r, p+"::TestTable/one").Parent {
		t.Error("only tests with subtests are parents")
	}
	if got := r.Failing(); !reflect.DeepEqual(got, []string{p + "::TestFail", p + "::TestTable/two_words"}) {
		t.Errorf("failing %v (parents never count)", got)
	}
	if c := get(t, r, p+"::TestFail"); !strings.Contains(c.Failure, "Add(1, 2) = 3") || strings.Contains(c.Failure, "=== RUN") {
		t.Errorf("go failure %q", c.Failure)
	}
}

func TestGoJSONIgnoresNoiseAndPackageFailuresWithoutCases(t *testing.T) {
	stream := "warming up\n" +
		`{"Action":"run","Package":"m/p","Test":"TestA"}` + "\n" +
		`{"Action":"pass","Package":"m/p","Test":"TestA","Elapsed":0.25}` + "\n" +
		`{"Action":"fail","Package":"m/p","Elapsed":0.3}` + "\n" + "not json {\n"
	r, err := ParseGoJSON(strings.NewReader(stream), "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.BuildFailed || len(r.Cases) != 1 || r.Cases[0].ID != "m/p::TestA" || r.Cases[0].Duration != 0.25 || r.Cases[0].File != "" {
		t.Fatalf("run %+v", r)
	}
}

func TestGoTestsFixtureStream(t *testing.T) {
	r := load(t, "go-tests.json", "../../fixtures/go-tests", "")
	if r.BuildFailed || len(r.Failing()) != 0 {
		t.Fatalf("fixture run must be green: %+v", r.Failing())
	}
	const p = "example.com/gotests/pricing"
	if c := get(t, r, p+"::TestCheckoutGoldMember"); c.File != "pricing/checkout_integration_test.go" {
		t.Errorf("file %q", c.File)
	}
	if c := get(t, r, p+"::TestTierRate/gold"); c.Base != p+"::TestTierRate" || c.File != "pricing/pricing_test.go" {
		t.Errorf("subtest %+v", c)
	}
	if pass, _, _, _ := r.Counts(); pass != 8 {
		t.Errorf("pass %d", pass)
	}
}

func TestMergeDedupesWithStatusPrecedence(t *testing.T) {
	a := &Run{Tool: "pytest", Cases: []Case{{ID: "x", Status: Pass, Duration: 1}, {ID: "y", Status: Skip}}}
	b := &Run{Tool: "pytest", Cases: []Case{{ID: "x", Status: Fail, Duration: 2, Failure: "boom"}, {ID: "y", Status: Pass}}, BuildFailed: true}
	m := Merge(a, b)
	if m.Tool != "pytest" || !m.BuildFailed || len(m.Cases) != 2 {
		t.Fatalf("merge %+v", m)
	}
	if x := m.Cases[0]; x.Status != Fail || x.Duration != 3 || x.Failure != "boom" {
		t.Errorf("x %+v", x)
	}
	if m.Cases[1].Status != Pass {
		t.Errorf("pass beats skip: %+v", m.Cases[1])
	}
	if Merge(a, &Run{Tool: "go"}).Tool != "mixed" {
		t.Error("mixed tools")
	}
}

func TestRebaseMakesFileIDsRepoRelative(t *testing.T) {
	r := load(t, "pytest-xunit1.xml", "testdata/pyzoo", "")
	r.Rebase("svc/api")
	c := get(t, r, "svc/api/tests/test_zoo.py::test_param[2]")
	if c.File != "svc/api/tests/test_zoo.py" || c.Base != "svc/api/tests/test_zoo.py::test_param" {
		t.Fatalf("rebased %+v", c)
	}
	g := load(t, "go-zoo.json", "testdata/gozoo", "")
	g.Rebase("mod")
	if c := get(t, g, "example.com/zoo/ok::TestPass"); c.File != "mod/ok/ok_test.go" || c.Locator != "example.com/zoo/ok" {
		t.Fatalf("go locator stays the import path: %+v", c)
	}
}

func TestLoadRejectsGarbage(t *testing.T) {
	if _, err := ParseJUnit(strings.NewReader("<html></html>"), "", ""); err == nil {
		t.Error("an html root is not JUnit")
	}
	if _, err := Load("testdata/does-not-exist.xml", "", ""); err == nil {
		t.Error("missing file")
	}
}

func TestFailureIsCappedAtFourKiB(t *testing.T) {
	xml := `<testsuite name="s"><testcase classname="c" name="n"><failure message="m">` + strings.Repeat("é", 5000) + `</failure></testcase></testsuite>`
	r, err := ParseJUnit(strings.NewReader(xml), "", "junit")
	if err != nil {
		t.Fatal(err)
	}
	if f := r.Cases[0].Failure; len(f) > 4096 || len(f) < 4000 || !strings.HasPrefix(f, "m\n") || !strings.HasSuffix(f, "é") {
		t.Fatalf("len %d", len(f))
	}
	if r.Cases[0].ID != "c::n" {
		t.Errorf("generic id %q", r.Cases[0].ID)
	}
}
