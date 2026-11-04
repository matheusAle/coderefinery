package testrun

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/discover"
)

func write(t *testing.T, path, content string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func sample(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "testresult", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeTools puts executables first on PATH (script bodies are /bin/sh).
func fakeTools(t *testing.T, tools map[string]string) {
	t.Helper()
	bin := filepath.Join(discover.Resolve(t.TempDir()), "bin")
	for name, body := range tools {
		write(t, filepath.Join(bin, name), "#!/bin/sh\n"+body+"\n", 0o755)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func runTest(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestParseArgs(t *testing.T) {
	o := ParseArgs([]string{"--root", "r", "--repeat", "3", "--no-coverage", "--changed", "--test-command", "make t", "src"})
	if o.ProjectRoot != "r" || o.Repeat != 3 || !o.NoCoverage || !o.Changed || o.TestCommand != "make t" || !reflect.DeepEqual(o.Positionals, []string{"src"}) {
		t.Fatalf("%+v", o)
	}
	for msg, args := range map[string][]string{
		"--repeat requires a positive integer": {"--repeat", "0"},
		"--root requires a value":              {"--root"},
		"Unknown option: --nope":               {"--nope"},
	} {
		if o := ParseArgs(args); o.ExitCode != 1 || o.Message != msg+"\n\n"+Help {
			t.Errorf("%v: %q", args, o.Message[:min(40, len(o.Message))])
		}
	}
	if code, out, _ := runTest(t, "--help"); code != 0 || out != Help {
		t.Error("help on stdout, exit 0")
	}
}

func TestDirSlugsTheModule(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	if got := Dir(root, "python", root); got != filepath.Join(root, ".metrics", "tests", "python") {
		t.Errorf("root module: %s", got)
	}
	if got := Dir(root, "go", filepath.Join(root, "svc", "api")); got != filepath.Join(root, ".metrics", "tests", "go", "svc__api") {
		t.Errorf("sub module: %s", got)
	}
	if ModuleRel(root, filepath.Join(root, "svc", "api")) != "svc/api" || ModuleRel(root, root) != "." {
		t.Error("ModuleRel")
	}
}

func TestMissingToolsPrintOneLineAndExit1(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		hint  string
	}{
		"node-test": {map[string]string{"package.json": `{"scripts":{"test":"node --test"}}`, "lib/a.js": "x\n"},
			`cannot add a JUnit reporter to npm test ("node --test"): pass --test-command`},
		"jest": {map[string]string{"package.json": `{"scripts":{"test":"jest --ci"}}`, "lib/a.js": "x\n"},
			"install jest-junit: npm i -D jest-junit"},
		"mocha": {map[string]string{"package.json": `{"scripts":{"test":"mocha"}}`, "lib/a.js": "x\n", "node_modules/mocha-junit-reporter/package.json": "{}"},
			"install c8: npm i -D c8"},
		"vitest": {map[string]string{"package.json": `{"scripts":{"test":"vitest run"}}`, "src/a.ts": "x\n", "node_modules/vitest/package.json": `{"version":"3.2.7"}`},
			"install the Vitest coverage provider: npm i -D @vitest/coverage-v8@3.2.7"},
		"unittest": {map[string]string{".venv/bin/python": "#!/bin/sh\nexit 1\n", "setup.cfg": "[metadata]\n", "pkg/a.py": "x = 1\n"},
			"unittest has no JUnit reporter: install pytest (it runs unittest suites) or pass --test-command"},
		"clojure": {map[string]string{"deps.edn": "{:aliases {:test {:main-opts [\"-m\" \"cognitect.test-runner\"]}}}", "src/a.clj": "(ns a)\n"},
			"only Kaocha (deps.edn :test alias) is supported for Clojure: pass --test-command"},
		"kaocha": {map[string]string{"deps.edn": "{:aliases {:test {:extra-deps {lambdaisland/kaocha {}}}}}", "src/a.clj": "(ns a)\n"},
			"add lambdaisland/kaocha-junit-xml (and lambdaisland/kaocha-cloverage) to the :test alias"},
		"gradle": {map[string]string{"build.gradle": "plugins { id 'java' }\n", "gradlew": "#!/bin/sh\n", "src/main/java/A.java": "class A {}\n"},
			"apply the jacoco plugin to get coverage, or pass --no-coverage"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := discover.Resolve(t.TempDir())
			for p, text := range c.files {
				mode := os.FileMode(0o666)
				if strings.HasPrefix(text, "#!") {
					mode = 0o755
				}
				write(t, filepath.Join(root, p), text, mode)
			}
			code, _, stderr := runTest(t, "--root", root)
			if code != 1 || stderr != c.hint+"\n" {
				t.Fatalf("exit %d stderr %q", code, stderr)
			}
			if discover.Exists(filepath.Join(root, ".metrics", "tests", "runs.edn")) {
				t.Error("nothing runs when a plugin is missing")
			}
		})
	}
	t.Run("rust", func(t *testing.T) {
		root := discover.Resolve(t.TempDir())
		write(t, filepath.Join(root, "Cargo.toml"), "[package]\nname='calc'\n", 0o666)
		write(t, filepath.Join(root, "src", "lib.rs"), "fn a() {}\n", 0o666)
		t.Setenv("PATH", "/usr/bin:/bin")
		if code, _, stderr := runTest(t, "--root", root); code != 1 || stderr != "install cargo-nextest: cargo install cargo-nextest --locked\n" {
			t.Fatalf("exit %d %q", code, stderr)
		}
	})
}

// Maven is not installed on the build machine: a fake mvn writes the
// checked-in Surefire sample where Surefire would, and a JaCoCo stub.
func TestMavenPlanCollectsSurefireReports(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	write(t, filepath.Join(root, "pom.xml"), "<project/>\n", 0o666)
	write(t, filepath.Join(root, "src", "main", "java", "com", "example", "Calc.java"), "class Calc {}\n", 0o666)
	write(t, filepath.Join(root, "src", "test", "java", "com", "example", "CalcTest.java"), "class CalcTest {}\n", 0o666)
	write(t, filepath.Join(root, "target", "surefire-reports", "TEST-stale.Old.xml"), "<testsuite name='stale'><testcase classname='stale.Old' name='gone'/></testsuite>", 0o666)
	fakeTools(t, map[string]string{"mvn": `echo "mvn $*" > mvn.args
mkdir -p target/surefire-reports target/site/jacoco
cp "` + sample(t, "surefire") + `"/TEST-*.xml target/surefire-reports/
echo '<report name="x"/>' > target/site/jacoco/jacoco.xml`})
	code, _, stderr := runTest(t, "--root", root)
	if code != 2 {
		t.Fatalf("the sample has a failure and an error: exit %d\n%s", code, stderr)
	}
	args, _ := os.ReadFile(filepath.Join(root, "mvn.args"))
	if string(args) != "mvn -q org.jacoco:jacoco-maven-plugin:0.8.12:prepare-agent test org.jacoco:jacoco-maven-plugin:0.8.12:report -Dmaven.test.failure.ignore=true -Dsurefire.skipAfterFailureCount=0\n" {
		t.Errorf("mvn args %q", args)
	}
	if !strings.Contains(stderr, "java .: surefire 6 passed, 2 failed, 1 skipped (branch-count coverage)\n") {
		t.Errorf("summary:\n%s", stderr)
	}
	copied, _ := filepath.Glob(filepath.Join(root, ".metrics", "tests", "java", "results", "*.xml"))
	if len(copied) != 3 || !discover.IsFile(filepath.Join(root, ".metrics", "tests", "java", "coverage", "jacoco.xml")) {
		t.Errorf("collected %v (the stale report was cleared first)", copied)
	}
	runs, err := ReadRuns(root)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs %v %v", runs, err)
	}
	if r := runs[0]; r.Tool != "surefire" || r.Results != ".metrics/tests/java/results" || r.Coverage != ".metrics/tests/java/coverage/jacoco.xml" ||
		r.Pass != 6 || r.Fail != 1 || r.Error != 1 || r.Skip != 1 || r.Granularity != "branch-count" {
		t.Errorf("record %+v", r)
	}
	run, err := LoadModuleResults(root, runs[0].Results, runs[0].Module, runs[0].Tool)
	if err != nil || len(run.Failing()) != 2 || run.Failing()[0] != "src/test/java/com/example/CalcTest.java::CalcTest::explodes" {
		t.Errorf("results %v %v", run.Failing(), err)
	}
}

// cargo-nextest / cargo-llvm-cov are not installed: fakes write the
// checked-in nextest sample where the coderefinery profile puts it.
func TestRustPlanRunsNextestWithTheGeneratedProfile(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	write(t, filepath.Join(root, "Cargo.toml"), "[package]\nname='calc'\n", 0o666)
	write(t, filepath.Join(root, "src", "lib.rs"), "fn a() {}\n", 0o666)
	write(t, filepath.Join(root, "tests", "integration.rs"), "#[test] fn checkout_works() {}\n", 0o666)
	fakeTools(t, map[string]string{
		"cargo-nextest":  "exit 0",
		"cargo-llvm-cov": "exit 0",
		"cargo": `echo "cargo $*" >> cargo.args
case "$*" in
  *"llvm-cov nextest"*) mkdir -p target/nextest/coderefinery && cp "` + sample(t, "nextest-junit.xml") + `" target/nextest/coderefinery/junit.xml; exit 100 ;;
  *--json*) echo '{"data":[]}' > "$5" ;;
  *--lcov*) echo 'TN:' > "$5" ;;
esac`,
	})
	t.Setenv("CARGO_TARGET_DIR", "")
	code, _, stderr := runTest(t, "--root", root)
	if code != 2 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	cov := filepath.Join(root, ".metrics", "tests", "rust", "coverage")
	args, _ := os.ReadFile(filepath.Join(root, "cargo.args"))
	want := "cargo llvm-cov nextest --no-report --no-fail-fast --config-file " + cov + "/nextest.toml --profile coderefinery\n" +
		"cargo llvm-cov report --json --output-path " + cov + "/llvm-cov.json\n" +
		"cargo llvm-cov report --lcov --output-path " + cov + "/lcov.info\n"
	if string(args) != want {
		t.Errorf("cargo calls:\n%s\nwant\n%s", args, want)
	}
	if cfg, _ := os.ReadFile(filepath.Join(cov, "nextest.toml")); string(cfg) != NextestConfig {
		t.Errorf("profile %q", cfg)
	}
	if !strings.Contains(stderr, "rust .: nextest 4 passed, 2 failed, 0 skipped (region coverage)\n") {
		t.Errorf("summary:\n%s", stderr)
	}
}

// No Clojure CLI on the build machine: a fake clj writes the Kaocha sample.
func TestClojurePlanAddsTheKaochaJunitPlugin(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	write(t, filepath.Join(root, "deps.edn"), "{:aliases {:test {:extra-deps {lambdaisland/kaocha {} lambdaisland/kaocha-junit-xml {} lambdaisland/kaocha-cloverage {}}}}}\n", 0o666)
	write(t, filepath.Join(root, "src", "billing", "pricing.clj"), "(ns billing.pricing)\n", 0o666)
	write(t, filepath.Join(root, "test", "billing", "pricing_test.clj"), "(ns billing.pricing-test)\n", 0o666)
	fakeTools(t, map[string]string{"clj": `echo "clj $*" > clj.args
cp "` + sample(t, "kaocha-junit.xml") + `" "$5"
mkdir -p "${9}" && echo 'TN:' > "${9}/lcov.info"
exit 1`})
	code, _, stderr := runTest(t, "--root", root)
	out := filepath.Join(root, ".metrics", "tests", "clojure")
	args, _ := os.ReadFile(filepath.Join(root, "clj.args"))
	if code != 2 || string(args) != "clj -M:test --plugin kaocha.plugin/junit-xml --junit-xml-file "+out+"/results.xml --plugin cloverage --cov-output "+out+"/coverage --lcov\n" {
		t.Fatalf("exit %d args %q\n%s", code, args, stderr)
	}
	if !strings.Contains(stderr, "clojure .: kaocha 2 passed, 2 failed, 0 skipped (form coverage)\n") {
		t.Errorf("summary:\n%s", stderr)
	}
}

func TestTestCommandWithRepeatRecordsFlakyTests(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	write(t, filepath.Join(root, "calc.go"), "package calc\n", 0o666)
	script := write(t, filepath.Join(root, "flaky.sh"), `#!/bin/sh
n=$(cat count 2>/dev/null || echo 0); n=$((n+1)); echo $n > count
status=pass; [ $n -eq 1 ] && status=fail
printf '{"Action":"pass","Package":"m/p","Test":"TestStable"}\n{"Action":"%s","Package":"m/p","Test":"TestFlaky"}\n' $status > "$CODEREFINERY_RESULTS"
[ -n "$CODEREFINERY_COVERAGE_DIR" ] && [ -d "$CODEREFINERY_COVERAGE_DIR" ] && touch "$CODEREFINERY_COVERAGE_DIR/seen"
exit 0
`, 0o755)
	code, _, stderr := runTest(t, "--root", root, "--test-command", script, "--repeat", "3")
	if code != 0 {
		t.Fatalf("a flaky failure alone exits 0: %d\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "command .: go 2 passed, 0 failed, 0 skipped, 1 flaky (no coverage)\n") {
		t.Errorf("summary:\n%s", stderr)
	}
	runs, err := ReadRuns(root)
	if err != nil || len(runs) != 1 || !reflect.DeepEqual(runs[0].Flaky, []string{"m/p::TestFlaky"}) || runs[0].Results != ".metrics/tests/command/results" {
		t.Fatalf("runs %+v %v", runs, err)
	}
	if n, _ := os.ReadFile(filepath.Join(root, "count")); strings.TrimSpace(string(n)) != "3" || !discover.IsFile(filepath.Join(root, ".metrics", "tests", "command", "coverage", "seen")) {
		t.Errorf("ran %q times; coverage dir exported", n)
	}
	// without --repeat the first run's failure fails the command
	_ = os.Remove(filepath.Join(root, "count"))
	if code, _, _ := runTest(t, "--root", root, "--test-command", script); code != 2 {
		t.Errorf("a failing case exits 2, got %d", code)
	}
	if code, _, stderr := runTest(t, "--root", root, "--test-command", "true"); code != 2 || !strings.Contains(stderr, "no results") {
		t.Errorf("no results exits 2: %d %q", code, stderr)
	}
}

func TestRunsEDNRoundTrips(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	TestedAt = func() string { return "2026-11-03T12:00:00-03:00" }
	recs := []RunRecord{
		{Language: "python", Module: ".", Tool: "pytest", Command: `py -m pytest "-k" a\b`, Results: ".metrics/tests/python/results.xml",
			Coverage: ".metrics/tests/python/coverage/coverage.json", Granularity: "branch", Contexts: true, Pass: 12, Skip: 1, Flaky: []string{`t.py::x["a b"]`}},
		{Language: "go", Module: "svc", Tool: "go", Exit: 1, Results: "", Granularity: "none", Fail: 2, BuildFailed: true},
	}
	text := RenderRuns(recs)
	if !strings.HasPrefix(text, "{:version 1 :tested-at \"2026-11-03T12:00:00-03:00\" :runs [\n  {:language \"python\" :module \".\" :tool \"pytest\"") ||
		!strings.Contains(text, ` :results nil :coverage nil :granularity "none" :contexts false :pass 0 :fail 2 :skip 0 :error 0 :build-failed true :flaky []}`) {
		t.Fatalf("rendered:\n%s", text)
	}
	write(t, filepath.Join(root, ".metrics", "tests", "runs.edn"), text, 0o666)
	got, err := ReadRuns(root)
	if err != nil || !reflect.DeepEqual(got[0], recs[0]) || got[1].Exit != 1 || !got[1].BuildFailed || got[1].Results != "" || got[1].Fail != 2 {
		t.Fatalf("read back %+v %v", got, err)
	}
	if none, err := ReadRuns(t.TempDir()); none != nil || err != nil {
		t.Error("a missing runs.edn is no runs")
	}
}
