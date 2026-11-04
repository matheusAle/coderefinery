package mutate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matheusAle/coderefinery/internal/discover"
)

func TestHiddenFlagsParseButStayOutOfHelp(t *testing.T) {
	o := ParseArgs([]string{"--attribute", "--branches", "src"})
	if !o.Attribute || !o.Branches || o.Help || len(o.Positionals) != 1 {
		t.Fatalf("parsed %+v", o)
	}
	if strings.Contains(Help, "--attribute") || strings.Contains(Help, "--branches") || len(Help) != 4236 {
		t.Fatal("--help must stay byte-identical to upstream")
	}
	if d := ParseArgs([]string{"src"}); d.Attribute || d.Branches {
		t.Fatal("both flags default off")
	}
}

func TestSiteLogWithoutAnnotatorKeepsUpstreamBytes(t *testing.T) {
	sites := []Site{{ID: "a", File: "f.py", Line: 3, Original: "+", Mutant: "-"}, {ID: "b", File: "f.py", Line: 4, Original: "x"}}
	status := map[string]string{"a": "killed", "b": "uncovered"}
	plain := formatSiteLog(sites, status, nil)
	if plain != "KILLED    f.py:3 + -> -\nUNCOVERED f.py:4 delete x\n" {
		t.Fatalf("plain %q", plain)
	}
	tagged := formatSiteLog(sites, status, func(s Site, st string) string { return " <" + st + ">" })
	if tagged != "KILLED    f.py:3 + -> - <killed>\nUNCOVERED f.py:4 delete x <uncovered>\n" {
		t.Fatalf("annotated %q", tagged)
	}
	if siteAnnotator(Options{}, "/", RunResult{}, nil) != nil {
		t.Fatal("no flag, no annotator")
	}
}

// A snapshot recorded by upstream mutator (harness golden, python/full),
// rewritten because another file's results land in its namespace dir, keeps
// every byte when there is no :killed-by anywhere.
func TestSnapshotWithoutKilledByRoundTripsByteIdentically(t *testing.T) {
	root := tempRoot(t)
	golden := map[string][]byte{}
	stamp := regexp.MustCompile(`:tested-at "([^"]+)"`)
	for _, name := range []string{"shop.pricing.edn", "shop.pricing.Cart.edn"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "snapshots", name))
		if err != nil {
			t.Fatal(err)
		}
		golden[name] = raw
		writeFile(t, filepath.Join(root, ".metrics", "mutate", name), string(raw))
	}
	at := stamp.FindSubmatch(golden["shop.pricing.edn"])[1]
	defer func(prev func() string) { testedAt = prev }(testedAt)
	testedAt = func() string { return string(at) }
	for _, killedBy := range []map[string][]string{nil, {}} {
		attributed := killedBy != nil
		written, err := writeResults(root, "src/other.py", nil, map[string]string{}, killedBy)
		if err != nil {
			t.Fatal(err)
		}
		if len(written) != 2 {
			t.Fatalf("attributed=%v wrote %v", attributed, written)
		}
		for name, want := range golden {
			got := []byte(readFile(t, filepath.Join(root, ".metrics", "mutate", name)))
			if name == "shop.pricing.Cart.edn" {
				// recorded a second later than shop.pricing.edn; only the stamp differs
				want = stamp.ReplaceAll(want, []byte(`:tested-at "`+string(at)+`"`))
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("attributed=%v %s changed:\n%s\nwant\n%s", attributed, name, got, want)
			}
			if bytes.Contains(got, []byte(":killed-by")) {
				t.Fatalf("%s gained :killed-by", name)
			}
		}
	}
}

// junitRunner writes a pytest-style JUnit report to $CODEREFINERY_RESULTS
// (the --test-command contract) from what the mutated file looks like.
type junitRunner struct {
	mu       sync.Mutex
	calls    int
	argv     [][]string
	baseline func(src string) (code int, failing []string, report bool)
	mutant   func(src string) (code int, timedOut bool, failing []string, report bool)
}

func (r *junitRunner) IsVerbose() bool { return false }

func (r *junitRunner) Run(c Command, cwd string, timeout time.Duration) CommandResult {
	r.mu.Lock()
	r.calls++
	r.argv = append(r.argv, c.Argv)
	r.mu.Unlock()
	results := ""
	for _, kv := range c.Env {
		if v, ok := strings.CutPrefix(kv, "CODEREFINERY_RESULTS="); ok {
			results = v
		}
	}
	src, _ := os.ReadFile(filepath.Join(cwd, "src", "demo.py"))
	var code int
	var timedOut, report bool
	var failing []string
	if timeout == 0 {
		code, failing, report = r.baseline(string(src))
	} else {
		code, timedOut, failing, report = r.mutant(string(src))
	}
	if report && results != "" {
		var b strings.Builder
		b.WriteString(`<testsuites name="pytest tests"><testsuite name="pytest">`)
		for _, name := range []string{"test_pos", "test_neg"} {
			b.WriteString(`<testcase classname="tests.test_demo" name="` + name + `" file="tests/test_demo.py" time="0.01">`)
			for _, f := range failing {
				if f == name {
					b.WriteString(`<failure message="assert">boom</failure>`)
				}
			}
			b.WriteString(`</testcase>`)
		}
		b.WriteString(`</testsuite></testsuites>`)
		_ = os.MkdirAll(filepath.Dir(results), 0o777)
		_ = os.WriteFile(results, []byte(b.String()), 0o666)
	}
	if timedOut {
		return CommandResult{Code: 124, TimedOut: true, Seconds: 0.01}
	}
	return CommandResult{Code: code, Seconds: 0.01, Output: "ran\n"}
}

func greenBaseline(string) (int, []string, bool) { return 0, nil, true }

// demoMutants: line 2 `>` times out, line 2 `0` breaks the build (no
// report), line 3 `+` fails test_pos, line 4 `+` survives.
func demoMutants(src string) (int, bool, []string, bool) {
	lines := strings.Split(src, "\n")
	switch {
	case strings.Contains(lines[1], ">="):
		return 0, true, nil, true
	case strings.Contains(lines[1], "> 1"):
		return 1, false, nil, false
	case strings.Contains(lines[2], "a - b"):
		return 1, false, []string{"test_pos"}, true
	}
	return 0, false, nil, true
}

func attributed(r Runner) fileOptions {
	o := baseOptions(r)
	o.attribute = true
	return o
}

func TestAttributeCreditsFailingTestsAndSentinels(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	r := mutateOK(t, path, root, attributed(&junitRunner{baseline: greenBaseline, mutant: demoMutants}))
	byLine := map[string][]string{}
	for _, s := range r.Sites {
		byLine[fmt.Sprintf("%d %s", s.Line, s.Description())] = r.KilledBy[s.ID]
	}
	want := map[string][]string{
		"2 > -> >=": {"<timeout>"},
		"2 0 -> 1":  {"<build>"},
		"3 + -> -":  {"tests/test_demo.py::test_pos"},
		"4 + -> -":  nil,
	}
	if !reflect.DeepEqual(byLine, want) {
		t.Fatalf("killed by %v", byLine)
	}
	if f := r.Forms[0]; f.Killed != 3 || f.Survived != 1 {
		t.Fatalf("classification is unchanged: %+v", f)
	}
	log := formatSiteLog(r.Sites, r.Statuses, attributeAnnotator(r))
	if !strings.Contains(log, "KILLED    src/demo.py:3 + -> - · by tests/test_demo.py::test_pos\n") ||
		!strings.Contains(log, "KILLED    src/demo.py:2 > -> >= · by <timeout>\n") || !strings.Contains(log, "SURVIVED  src/demo.py:4 + -> -\n") {
		t.Fatalf("site log:\n%s", log)
	}
	snap := readFile(t, filepath.Join(root, ".metrics", "mutate", "demo.edn"))
	tail := snap[strings.Index(snap, ":forms"):]
	if !strings.HasSuffix(snap, `"]" ["tests/test_demo.py::test_pos"]}}`+"\n") || !strings.Contains(tail, `}] :killed-by {"[\"src/demo.py\",\"demo\",\"defn/add\",`) ||
		strings.Count(tail, `["<`) != 2 {
		t.Fatalf("snapshot :killed-by is the last key:\n%s", snap)
	}
	if !strings.Contains(snap, `\"0\",\"1\"]" ["<build>"]`) {
		t.Fatalf("sentinel entries:\n%s", snap)
	}

	// a default run carries the kills and their credit; only the survivor re-runs
	plain := &fakeRunner{decide: func(_ string, _ time.Duration) int { return 0 }}
	o := baseOptions(plain)
	o.mutateAll = false
	again := mutateOK(t, path, root, o)
	if plain.calls != 2 || len(again.KilledBy) != 3 || again.Forms[0].Killed != 3 {
		t.Fatalf("calls %d killedBy %v", plain.calls, again.KilledBy)
	}
	if snap2 := readFile(t, filepath.Join(root, ".metrics", "mutate", "demo.edn")); strings.Count(snap2, `["`) != strings.Count(snap, `["`) {
		t.Fatalf("kept entries follow kept outcomes:\n%s", snap2)
	}

	// SnapshotOutcomes is what `coderefinery tests` reads
	outs := SnapshotOutcomes(root)
	killed := 0
	for _, o := range outs {
		if o.Killed {
			killed++
			if len(o.KilledBy) != 1 {
				t.Errorf("%s by %v", o.ID, o.KilledBy)
			}
		}
	}
	if len(outs) != 4 || killed != 3 {
		t.Fatalf("outcomes %+v", outs)
	}
}

func TestAttributeReselectsCarriedKillsWithoutCredit(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	mutateOK(t, path, root, baseOptions(&fakeRunner{decide: addTest})) // plain run: 1 kill, no :killed-by
	runner := &junitRunner{baseline: greenBaseline, mutant: demoMutants}
	o := attributed(runner)
	o.mutateAll = false
	r := mutateOK(t, path, root, o)
	// baseline + 3 survivors + the carried kill that has no credit yet
	if runner.calls != 5 || len(r.KilledBy) != 3 {
		t.Fatalf("calls %d killedBy %v", runner.calls, r.KilledBy)
	}
	runner.calls = 0
	mutateOK(t, path, root, o)
	if runner.calls != 1 { // the survivor (the baseline is cached in o.baselines); every kill is credited now
		t.Fatalf("second attributed run calls %d", runner.calls)
	}
}

func TestAttributedBaselineNamesFailingTestsOrTheMissingReport(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	failing := &junitRunner{baseline: func(string) (int, []string, bool) { return 1, []string{"test_neg"}, true }, mutant: demoMutants}
	r := mutateOK(t, path, root, attributed(failing))
	if !r.BaselineFailed || !strings.HasSuffix(r.BaselineMessage, "\nfailing tests: tests/test_demo.py::test_neg") ||
		!strings.HasPrefix(r.BaselineMessage, "Baseline failed for src/demo.py: CODEREFINERY_RESULTS=") {
		t.Fatalf("message %q", r.BaselineMessage)
	}
	silent := &junitRunner{baseline: func(string) (int, []string, bool) { return 0, nil, false }, mutant: demoMutants}
	r = mutateOK(t, path, root, attributed(silent))
	if r.Hint != "--test-command must write JUnit XML or go test -json to $CODEREFINERY_RESULTS" || silent.calls != 1 {
		t.Fatalf("hint %q calls %d", r.Hint, silent.calls)
	}
	if discover.Exists(filepath.Join(root, "target", "mutation-workers", "baseline")) {
		t.Error("the baseline report dir is removed")
	}
}

func TestPythonNarrowingRerunsSurvivorsWithTheFullSuite(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "pyproject.toml"), "[tool.pytest.ini_options]\n")
	writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	cov := filepath.Join(root, ".metrics", "tests", "python", "coverage", "coverage.json")
	writeFile(t, cov, `{"files":{"src/demo.py":{"contexts":{"2":["","tests/test_demo.py::test_pos|run"],"3":["tests/test_demo.py::test_pos|run","tests/test_demo.py::test_neg|setup"]}}}}`)
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(cov, future, future)
	a, hint := newAttribution(root, filepath.Join(root, "src", "demo.py"), "python", Command{Argv: []string{"/py", "-m", "pytest"}}, root, "")
	if hint != "" || a.narrow == nil {
		t.Fatalf("hint %q narrow %v", hint, a.narrow)
	}
	if got := a.narrowFor(Site{File: "src/demo.py", Line: 3, AnchorLine: 2}); !reflect.DeepEqual(got, []string{"tests/test_demo.py::test_neg", "tests/test_demo.py::test_pos"}) {
		t.Fatalf("narrowed %v", got)
	}
	if got := a.narrowFor(Site{File: "src/demo.py", Line: 9, AnchorLine: 9}); got != nil {
		t.Fatalf("no covering test runs the full suite: %v", got)
	}
	cmd := a.build("/w/r.xml", []string{"t::a"})
	if strings.Join(cmd.Argv, " ") != "/py -m pytest --junitxml=/w/r.xml -o junit_family=xunit1 --maxfail=0 t::a" {
		t.Fatalf("argv %v", cmd.Argv)
	}
	// narrowed run passes, the full suite kills: the survivor is re-run
	var argvs []string
	runner := runnerFunc(func(c Command, cwd string, _ time.Duration) CommandResult {
		argvs = append(argvs, strings.Join(c.Argv, " "))
		if strings.HasSuffix(c.Argv[len(c.Argv)-1], "test_pos") {
			return CommandResult{Code: 0}
		}
		return CommandResult{Code: 2}
	})
	j := &mutantJob{root: root, cwd: root, runner: runner, timeout: time.Second}
	status, by := a.run(j, root, Site{File: "src/demo.py", Line: 2, AnchorLine: 2})
	if status != "killed" || !reflect.DeepEqual(by, []string{"<build>"}) || len(argvs) != 2 || strings.Contains(argvs[1], "test_pos") {
		t.Fatalf("status %s by %v argv %q", status, by, argvs)
	}
	// a source newer than the contexts disables narrowing
	_ = os.Chtimes(cov, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	if pythonContexts(root, root) != nil {
		t.Fatal("stale contexts")
	}
	if ContextNodeID("t.py::x[a|b]|teardown") != "t.py::x[a|b]" || ContextNodeID("") != "" {
		t.Fatal("ContextNodeID")
	}
}

type runnerFunc func(Command, string, time.Duration) CommandResult

func (f runnerFunc) Run(c Command, cwd string, t time.Duration) CommandResult { return f(c, cwd, t) }
func (f runnerFunc) IsVerbose() bool                                          { return false }

func TestAttributionPlansPerLanguage(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "package.json"), `{"scripts":{"test":"vitest run"}}`)
	a, hint := newAttribution(root, filepath.Join(root, "a.ts"), "typescript", Command{Argv: []string{"npm", "test"}}, root, "")
	if hint != "" {
		t.Fatal(hint)
	}
	c := a.build("/w/.coderefinery/results.xml", nil)
	if strings.Join(c.Argv, " ") != "npm test -- --reporter=default --reporter=junit --outputFile.junit=/w/.coderefinery/results.xml" ||
		!reflect.DeepEqual(c.Env, []string{"CI=true"}) || a.resultsPath("/w", "/w/pkg") != "/w/.coderefinery/results.xml" {
		t.Fatalf("vitest %v %v", c.Argv, c.Env)
	}
	g, _ := newAttribution(root, filepath.Join(root, "p", "x.go"), "go", Command{Argv: []string{"go", "test", "-count=1", "./p"}}, root, "")
	if strings.Join(g.build("", nil).Argv, " ") != "go test -json -count=1 ./p" || !g.goOutput || g.resultsPath("/w", "/w") != "" {
		t.Fatal("go")
	}
	j, _ := newAttribution(root, filepath.Join(root, "A.java"), "java", Command{Argv: []string{"mvn", "-q", "test", "-DexcludeTags=no-mutate"}}, root, "")
	if strings.Join(j.build("", nil).Argv, " ") != "mvn -q test -DexcludeTags=no-mutate -Dsurefire.skipAfterFailureCount=0" ||
		j.resultsPath("/w", "/w/m") != "/w/m/target/surefire-reports" {
		t.Fatal("java")
	}
	for lang, plan := range map[string][]string{"python": {"/py", "-m", "unittest", "discover"}, "clojure": {"bb", "test"}} {
		if _, hint := newAttribution(root, "x", lang, Command{Argv: plan}, root, ""); hint == "" {
			t.Errorf("%s %v has no reporter", lang, plan)
		}
	}
	writeFile(t, filepath.Join(root, "jest", "package.json"), `{"scripts":{"test":"jest"}}`)
	if _, hint := newAttribution(root, "x", "typescript", Command{Argv: []string{"npm", "test"}}, filepath.Join(root, "jest"), ""); hint != "install jest-junit: npm i -D jest-junit" {
		t.Errorf("jest hint %q", hint)
	}
}
