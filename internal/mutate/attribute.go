package mutate

// mutate --attribute: which tests kill each mutant (docs/design-tests.md §3).
// The test plan gets a JUnit reporter (go: -json); a killed mutant is
// credited to the report's failing cases, "<timeout>" or "<build>".

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/testresult"
	"github.com/matheusAle/coderefinery/internal/testrun"
)

// Sentinels credited when no test can be.
const (
	byTimeout = "<timeout>"
	byBuild   = "<build>"
)

// attribution is how one plan reports per-test results.
type attribution struct {
	root, cwd  string // project root and the real plan cwd
	moduleRel  string // cwd relative to root: the prefix that makes IDs repo-relative
	tool       string // testresult tool hint ("" = sniff)
	goOutput   bool   // go test -json: parse the captured output
	reportDir  string // Surefire: the reports dir, relative to cwd
	reportFile string // nextest: the JUnit file, relative to cwd
	noReport   string // the hint when the baseline writes no parseable report
	build      func(results string, tests []string) Command
	narrow     map[string]map[int][]string // python: module-relative file -> line -> node ids
}

// cacheKey separates attributed baselines from plain ones.
func (a *attribution) cacheKey() string {
	if a == nil {
		return ""
	}
	return "\x03attribute"
}

// newAttribution decides the reporter for a plan; hint != "" means the
// project cannot be attributed (exit 1 with that line).
func newAttribution(root, path, language string, plan Command, cwd, testCommand string) (*attribution, string) {
	a := &attribution{root: root, cwd: cwd, moduleRel: testrun.ModuleRel(root, cwd)}
	if testCommand != "" {
		a.noReport = "--test-command must write JUnit XML or go test -json to $CODEREFINERY_RESULTS"
		a.build = func(results string, _ []string) Command {
			return Command{Shell: testCommand, Env: []string{"CODEREFINERY_RESULTS=" + results}}
		}
		return a, ""
	}
	argv := plan.Argv
	switch language {
	case "python":
		if len(argv) < 3 || argv[len(argv)-1] != "pytest" {
			return nil, "unittest has no JUnit reporter: install pytest (it runs unittest suites) or pass --test-command"
		}
		a.tool, a.noReport = "pytest", "pytest wrote no JUnit report: is pytest installed for "+argv[0]+"?"
		a.build = func(results string, tests []string) Command {
			args := append(append(append([]string(nil), argv...), testrun.PytestReporter(results)...), tests...)
			return Command{Argv: args}
		}
		a.narrow = pythonContexts(root, cwd)
	case "go":
		a.tool, a.goOutput, a.noReport = "go", true, "go test -json wrote no test events"
		pkg := argv[len(argv)-1]
		a.build = func(string, []string) Command { return Command{Argv: []string{"go", "test", "-json", "-count=1", pkg}} }
	case "typescript":
		runner, hint := testrun.NodeRunner(cwd)
		if hint == "" {
			hint = testrun.NodePluginHint(runner, cwd, root)
		}
		if hint != "" {
			return nil, hint
		}
		a.tool, a.noReport = runner, runner+" wrote no JUnit report"
		a.build = func(results string, _ []string) Command {
			args, env := testrun.NodeReporter(runner, results)
			return Command{Argv: append(append(append([]string(nil), argv...), "--"), args...), Env: env}
		}
	case "java":
		a.tool, a.reportDir, a.noReport = "surefire", filepath.Join("target", "surefire-reports"), "Maven wrote no Surefire reports"
		a.build = func(string, []string) Command {
			return Command{Argv: append(append([]string(nil), argv...), "-Dsurefire.skipAfterFailureCount=0")}
		}
	case "rust":
		if _, err := exec.LookPath("cargo-nextest"); err != nil {
			return nil, "install cargo-nextest: cargo install cargo-nextest --locked"
		}
		config := filepath.Join(testrun.Dir(root, "rust", cwd), "coverage", "nextest.toml")
		if err := os.MkdirAll(filepath.Dir(config), 0o777); err == nil {
			_ = os.WriteFile(config, []byte(testrun.NextestConfig), 0o666)
		}
		a.tool, a.reportFile, a.noReport = "nextest", filepath.Join("target", "nextest", "coderefinery", "junit.xml"), "cargo nextest wrote no JUnit report"
		a.build = func(string, []string) Command {
			return Command{Argv: append([]string{"cargo", "nextest", "run"}, testrun.NextestArgs(config)...)}
		}
	case "clojure":
		if len(argv) < 2 || argv[0] != "clj" || argv[1] != "-M:test" {
			return nil, "speclj, bb and clj -M:spec have no JUnit reporter: pass --test-command"
		}
		if _, hint := testrun.KaochaReporter(cwd, "x"); hint != "" {
			return nil, hint
		}
		a.tool, a.noReport = "kaocha", "Kaocha wrote no JUnit report"
		a.build = func(results string, _ []string) Command {
			reporter, _ := testrun.KaochaReporter(cwd, results)
			return Command{Argv: append(append([]string(nil), argv...), reporter...)}
		}
	default:
		return nil, "no test runner for " + language + ": pass --test-command"
	}
	return a, ""
}

// resultsPath is where a run's report lands: <worker>/.coderefinery/results.xml,
// the Surefire reports dir or nextest's JUnit file under the (mapped) cwd.
func (a *attribution) resultsPath(home, cwd string) string {
	switch {
	case a.goOutput:
		return ""
	case a.reportDir != "":
		return filepath.Join(cwd, a.reportDir)
	case a.reportFile != "":
		return filepath.Join(cwd, a.reportFile)
	}
	return filepath.Join(home, ".coderefinery", "results.xml")
}

// clear deletes a previous report, file by file.
func (a *attribution) clear(results string) {
	if results == "" {
		return
	}
	if a.reportDir != "" {
		matches, _ := filepath.Glob(filepath.Join(results, "TEST-*.xml"))
		for _, m := range matches {
			_ = os.Remove(m)
		}
		return
	}
	_ = os.Remove(results)
	_ = os.MkdirAll(filepath.Dir(results), 0o777)
}

// parse reads the run's report with repo-relative IDs.
func (a *attribution) parse(res CommandResult, results, cwd string) (*testresult.Run, error) {
	var run *testresult.Run
	var err error
	if a.goOutput {
		run, err = testresult.ParseGoJSON(strings.NewReader(res.Output), cwd)
		if err == nil && len(run.Cases) == 0 && !run.BuildFailed {
			return nil, fmt.Errorf("no test events")
		}
	} else {
		run, err = testresult.Load(results, cwd, a.tool)
	}
	if err != nil {
		return nil, err
	}
	run.Rebase(a.moduleRel)
	return run, nil
}

// baseline runs the attributed baseline in the real project; its report goes
// to target/mutation-workers/baseline/.
func (a *attribution) baseline(runner Runner) baseline {
	home := filepath.Join(a.root, "target", "mutation-workers", "baseline")
	results := a.resultsPath(home, a.cwd)
	a.clear(results)
	cmd := a.build(results, nil)
	res := runner.Run(cmd, a.cwd, 0)
	run, err := a.parse(res, results, a.cwd)
	if results != "" && a.reportDir == "" {
		_ = os.Remove(results)
		_ = os.Remove(filepath.Dir(results))
		_ = os.Remove(home)
	}
	b := baseline{ok: res.Code == 0, seconds: res.Seconds, display: cmd.Display()}
	if res.Code != 0 {
		b.tail = lastLines(res.Output, 20)
	}
	if err != nil {
		if res.Code == 0 {
			b.ok, b.hint = false, a.noReport
		}
		return b
	}
	b.failing = run.Failing()
	if len(b.failing) > 0 || run.BuildFailed {
		b.ok = false
		if b.tail == "" {
			b.tail = lastLines(res.Output, 20)
		}
	}
	return b
}

// narrowFor is the covering node ids of a site (§3.3); nil = full suite.
func (a *attribution) narrowFor(s Site) []string {
	if a.narrow == nil {
		return nil
	}
	file := s.File
	if a.moduleRel != "." {
		file = strings.TrimPrefix(file, a.moduleRel+"/")
	}
	lines := a.narrow[file]
	set := map[string]bool{}
	for _, l := range []int{s.Line, s.AnchorLine} {
		for _, id := range lines[l] {
			set[id] = true
		}
	}
	if len(set) == 0 || len(set) > 200 {
		return nil
	}
	ids := make([]string, 0, len(set))
	size := 0
	for id := range set {
		ids = append(ids, id)
		size += len(id) + 1
	}
	if size > 100*1024 {
		return nil
	}
	sort.Strings(ids)
	return ids
}

// once runs the tests against the mutant in place and attributes a kill.
func (a *attribution) once(j *mutantJob, home, cwd string, tests []string) (string, []string) {
	results := a.resultsPath(home, cwd)
	a.clear(results)
	res := j.runner.Run(a.build(results, tests), cwd, j.timeout)
	switch {
	case res.TimedOut:
		return "killed", []string{byTimeout}
	case res.Code == 0:
		return "survived", nil
	}
	if run, err := a.parse(res, results, cwd); err == nil {
		if failing := run.Failing(); len(failing) > 0 {
			return "killed", failing
		}
	}
	return "killed", []string{byBuild}
}

// run is execute for an attributed job: a survivor of a narrowed run is
// re-run with the full suite, so killed/survived equals the full suite's.
func (a *attribution) run(j *mutantJob, dir string, s Site) (string, []string) {
	cwd := mappedCwd(dir, j.root, j.cwd)
	tests := a.narrowFor(s)
	status, by := a.once(j, dir, cwd, tests)
	if status == "survived" && tests != nil {
		status, by = a.once(j, dir, cwd, nil)
	}
	return status, by
}

// pythonContexts reads `coderefinery test`'s coverage.json contexts for the
// module when they are newer than every .py below it; nil otherwise.
func pythonContexts(root, cwd string) map[string]map[int][]string {
	report := filepath.Join(testrun.Dir(root, "python", cwd), "coverage", "coverage.json")
	info, err := os.Stat(report)
	if err != nil {
		return nil
	}
	stale := false
	_ = filepath.WalkDir(cwd, func(p string, d fs.DirEntry, err error) error {
		if err != nil || stale {
			return nil
		}
		if d.IsDir() {
			if p != cwd && (skipLink[d.Name()] || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".py") {
			if fi, err := d.Info(); err == nil && fi.ModTime().After(info.ModTime()) {
				stale = true
			}
		}
		return nil
	})
	if stale {
		return nil
	}
	raw, err := os.ReadFile(report)
	if err != nil {
		return nil
	}
	var doc struct {
		Files map[string]struct {
			Contexts map[string][]string `json:"contexts"`
		} `json:"files"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	out := map[string]map[int][]string{}
	found := false
	for file, f := range doc.Files {
		lines := map[int][]string{}
		for line, labels := range f.Contexts {
			var n int
			if _, err := fmt.Sscanf(line, "%d", &n); err != nil {
				continue
			}
			for _, label := range labels {
				if id := ContextNodeID(label); id != "" {
					lines[n] = append(lines[n], id)
					found = true
				}
			}
		}
		out[filepath.ToSlash(file)] = lines
	}
	if !found {
		return nil
	}
	return out
}

// ContextNodeID is a pytest-cov context label without its phase
// ("tests/t.py::test_x|run" -> "tests/t.py::test_x"); "" for the empty
// (non-test) context.
func ContextNodeID(label string) string {
	for _, phase := range []string{"|setup", "|run", "|teardown"} {
		label = strings.TrimSuffix(label, phase)
	}
	return label
}

// carriedKilledBy is the recorded :killed-by of the outcomes this run
// carries forward (kept entries follow kept outcomes).
func carriedKilledBy(root, fileKey string, carried map[string]string) map[string][]string {
	out := map[string][]string{}
	for k, v := range loadKilledBy(root, fileKey) {
		if carried[k] == "killed" {
			out[k] = v
		}
	}
	return out
}

// reselectUnattributed adds carried kills without a :killed-by entry to the
// selection (a one-time migration under --attribute), in site order.
func reselectUnattributed(found, selected []Site, carried map[string]string, killedBy map[string][]string, lines map[int]bool) []Site {
	chosen := map[string]bool{}
	for _, s := range selected {
		chosen[s.ID] = true
	}
	var out []Site
	for _, s := range found {
		redo := !chosen[s.ID] && carried[s.ID] == "killed" && len(killedBy[s.ID]) == 0 && (lines == nil || lines[s.Line])
		if chosen[s.ID] || redo {
			out = append(out, s)
		}
	}
	return out
}

// attributeAnnotator is the --attribute site-log suffix: KILLED lines get
// " · by <first id>" and " (+N)" when more tests killed the mutant.
func attributeAnnotator(result RunResult) func(Site, string) string {
	if len(result.KilledBy) == 0 {
		return nil
	}
	return func(s Site, status string) string {
		by := result.KilledBy[s.ID]
		if status != "killed" || len(by) == 0 {
			return ""
		}
		out := " · by " + by[0]
		if len(by) > 1 {
			out += fmt.Sprintf(" (+%d)", len(by)-1)
		}
		return out
	}
}

// SnapshotOutcome is one recorded mutant outcome with its :killed-by.
type SnapshotOutcome struct {
	ID       string   // the mutation id (JSON array text)
	Killed   bool     // killed (else survived)
	KilledBy []string // sorted test ids and sentinels; nil when unattributed
}

// SnapshotOutcomes reads every outcome of .metrics/mutate/**/*.edn (for
// `coderefinery tests`), sorted by ID.
func SnapshotOutcomes(root string) []SnapshotOutcome {
	byID := map[string]SnapshotOutcome{}
	for _, s := range readSnapshots(discover.Resolve(root)) {
		by := killedByOf(s.data)
		for _, o := range outcomesOf(s.data) {
			byID[o.id] = SnapshotOutcome{ID: o.id, Killed: o.status == "killed", KilledBy: by[o.id]}
		}
	}
	out := make([]SnapshotOutcome, 0, len(byID))
	for _, o := range byID {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
