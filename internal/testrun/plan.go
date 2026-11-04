package testrun

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/testplan"
)

// Dir is .metrics/tests/<language>[/<slug>] for a module (§5): slug is the
// module dir relative to root with / -> __, omitted at the root.
func Dir(root, language, module string) string {
	base := filepath.Join(root, ".metrics", "tests", language)
	rel, ok := discover.RelativeTo(discover.Resolve(module), discover.Resolve(root))
	if !ok || rel == "." || rel == "" {
		return base
	}
	return filepath.Join(base, strings.ReplaceAll(rel, "/", "__"))
}

// ModuleRel is the module dir relative to root, "." at the root.
func ModuleRel(root, module string) string {
	rel, ok := discover.RelativeTo(discover.Resolve(module), discover.Resolve(root))
	if !ok || rel == "" {
		return "."
	}
	return rel
}

// module is one project to test.
type module struct {
	language string
	dir      string   // absolute
	files    []string // selected files inside it
}

// markers extends the mutate test-plan markers with Gradle builds.
func markers(language string) []string {
	if language == "java" {
		return []string{"pom.xml", "build.gradle", "build.gradle.kts"}
	}
	return testplan.Markers[language]
}

// modules groups files by (language, nearest-marker dir), sorted.
func modules(root string, files []string) []module {
	byKey := map[string]*module{}
	var keys []string
	for _, f := range files {
		lang := discover.LanguageOf(f)
		if lang == "" {
			continue
		}
		dir := testplan.NearestMarker(f, root, markers(lang)...)
		k := lang + "\x00" + dir
		m := byKey[k]
		if m == nil {
			m = &module{language: lang, dir: dir}
			byKey[k] = m
			keys = append(keys, k)
		}
		m.files = append(m.files, f)
	}
	order := map[string]int{"python": 0, "go": 1, "typescript": 2, "java": 3, "rust": 4, "clojure": 5}
	sort.Slice(keys, func(i, j int) bool {
		a, b := byKey[keys[i]], byKey[keys[j]]
		if a.language != b.language {
			return order[a.language] < order[b.language]
		}
		return a.dir < b.dir
	})
	out := make([]module, len(keys))
	for i, k := range keys {
		out[i] = *byKey[k]
	}
	return out
}

// step is one command of a plan.
type step struct {
	argv     []string
	shell    string
	env      []string // extra KEY=VALUE
	goJSON   string   // capture stdout (go test -json) into this file, echo the test output
	optional bool     // a report step: its failure does not fail the run
}

func (s step) display() string {
	if s.argv == nil {
		return s.shell
	}
	return testplan.Command{Argv: s.argv}.Display()
}

// plan is how one module runs.
type plan struct {
	module
	tool        string
	out         string // .metrics/tests/<lang>[/<slug>]
	results     string // the results file or dir under out
	coverageRep string // main coverage report under out, "" without
	granularity string
	contexts    bool
	steps       []step
	collect     func() error // copies reports the tool wrote elsewhere into out
	hint        string       // a missing tool/plugin: print and exit 1
	cleanup     []string     // files under out removed before the run (globs)
}

func (p *plan) covDir() string { return filepath.Join(p.out, "coverage") }

// pyImports is whether py can import module.
func pyImports(py, dir, module string) bool {
	cmd := exec.Command(py, "-c", "import "+module)
	cmd.Dir = dir
	return cmd.Run() == nil
}

func onPath(name string) bool { _, err := exec.LookPath(name); return err == nil }

// nodeModule finds node_modules/<name> from dir up to root.
func nodeModule(dir, root, name string) string {
	stop := discover.Resolve(root)
	for cur := discover.Resolve(dir); ; cur = filepath.Dir(cur) {
		cand := filepath.Join(cur, "node_modules", name)
		if discover.IsFile(filepath.Join(cand, "package.json")) {
			return cand
		}
		if cur == stop || filepath.Dir(cur) == cur {
			return ""
		}
	}
}

func packageVersion(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct{ Version string }
	_ = json.Unmarshal(raw, &pkg)
	return pkg.Version
}

// atLeast compares a semver-ish "x.y.z" with major.minor.
func atLeast(version string, major, minor int) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	ma, err1 := strconv.Atoi(parts[0])
	mi, err2 := strconv.Atoi(strings.TrimLeftFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' }))
	if err1 != nil || err2 != nil {
		return false
	}
	return ma > major || ma == major && mi >= minor
}

func testScript(dir string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", false
	}
	var pkg struct{ Scripts map[string]string }
	if json.Unmarshal(raw, &pkg) != nil {
		return "", false
	}
	s, ok := pkg.Scripts["test"]
	return s, ok
}

// buildPlan decides the commands for one module (§2.1).
func buildPlan(root string, m module, opts Options) *plan {
	p := &plan{module: m, out: Dir(root, m.language, m.dir)}
	noCov := opts.NoCoverage
	switch m.language {
	case "python":
		pythonPlan(p, noCov)
	case "go":
		goPlan(p, noCov)
	case "typescript":
		nodePlan(p, root, noCov)
	case "java":
		javaPlan(p, noCov)
	case "rust":
		rustPlan(p, noCov)
	case "clojure":
		clojurePlan(p, noCov)
	default:
		p.hint = "no test runner for " + m.language + ": pass --test-command"
	}
	if noCov {
		p.coverageRep, p.granularity, p.contexts = "", "none", false
	}
	return p
}

func pythonPlan(p *plan, noCov bool) {
	py := testplan.ProjectInterpreter(p.dir)
	p.tool = "pytest"
	if !testplan.UsesPytest(p.dir) && !pyImports(py, p.dir, "pytest") {
		p.hint = "unittest has no JUnit reporter: install pytest (it runs unittest suites) or pass --test-command"
		return
	}
	if !pyImports(py, p.dir, "pytest") {
		p.hint = fmt.Sprintf("pytest is not installed for %s: run %s -m pip install pytest", py, py)
		return
	}
	p.results = filepath.Join(p.out, "results.xml")
	reporter := PytestReporter(p.results)
	if noCov {
		p.steps = []step{{argv: append([]string{py, "-m", "pytest"}, reporter...)}}
		return
	}
	if !pyImports(py, p.dir, "coverage") {
		p.hint = fmt.Sprintf("coverage is not installed for %s: run %s -m pip install coverage", py, py)
		return
	}
	data := filepath.Join(p.covDir(), ".coverage")
	jsonRep := filepath.Join(p.covDir(), "coverage.json")
	lcov := filepath.Join(p.covDir(), "lcov.info")
	src := coverage.PythonSources(p.dir, p.files)
	p.coverageRep, p.granularity = jsonRep, "branch"
	p.cleanup = []string{data, jsonRep, lcov}
	if pyImports(py, p.dir, "pytest_cov") {
		p.contexts = true
		cov := "--cov"
		if src != "" {
			cov = "--cov=" + src
		}
		args := append([]string{py, "-m", "pytest", cov, "--cov-branch", "--cov-context=test", "--cov-report="}, reporter...)
		p.steps = []step{{argv: args, env: []string{"COVERAGE_FILE=" + data}}}
	} else {
		args := []string{py, "-m", "coverage", "run", "--branch", "--data-file=" + data}
		if src != "" {
			args = append(args, "--source="+src)
		}
		args = append(append(args, "-m", "pytest"), reporter...)
		p.steps = []step{{argv: args}}
	}
	p.steps = append(p.steps,
		step{argv: []string{py, "-m", "coverage", "json", "--show-contexts", "--data-file=" + data, "-o", jsonRep}, optional: true},
		step{argv: []string{py, "-m", "coverage", "lcov", "--data-file=" + data, "-o", lcov}, optional: true})
}

func goPlan(p *plan, noCov bool) {
	p.tool = "go"
	if !onPath("go") {
		p.hint = "go is not installed: install Go or pass --test-command"
		return
	}
	p.results = filepath.Join(p.out, "results.json")
	args := []string{"go", "test", "-json", "-count=1"}
	if !noCov {
		profile := filepath.Join(p.covDir(), "coverage.out")
		args = append(args, "-covermode=count", "-coverprofile="+profile)
		p.coverageRep, p.granularity = profile, "block"
		p.cleanup = []string{profile}
	}
	p.steps = []step{{argv: append(args, "./..."), goJSON: p.results}}
}

func nodePlan(p *plan, root string, noCov bool) {
	runner, hint := NodeRunner(p.dir)
	if hint == "" {
		hint = NodePluginHint(runner, p.dir, root)
	}
	if hint != "" {
		p.hint = hint
		return
	}
	p.tool = runner
	p.results = filepath.Join(p.out, "results.xml")
	cd := p.covDir()
	reporter, env := NodeReporter(runner, p.results)
	args := append([]string{"npm", "test", "--"}, reporter...)
	if !noCov {
		p.coverageRep, p.granularity = filepath.Join(cd, "coverage-final.json"), "branch"
		p.cleanup = []string{filepath.Join(cd, "coverage-final.json"), filepath.Join(cd, "lcov.info")}
		switch runner {
		case "vitest":
			vitest := nodeModule(p.dir, root, "vitest")
			if nodeModule(p.dir, root, "@vitest/coverage-v8") == "" && nodeModule(p.dir, root, "@vitest/coverage-istanbul") == "" {
				version := packageVersion(vitest)
				if version == "" {
					version = "latest"
				}
				p.hint = "install the Vitest coverage provider: npm i -D @vitest/coverage-v8@" + version
				return
			}
			args = append(args, "--coverage", "--coverage.reporter=json", "--coverage.reporter=lcov",
				"--coverage.reportsDirectory="+cd, "--coverage.reportOnFailure=true")
			if atLeast(packageVersion(vitest), 3, 2) {
				args = append(args, "--coverage.experimentalAstAwareRemapping=true")
			}
		case "jest":
			args = append(args, "--coverage", "--coverageReporters=json", "--coverageReporters=lcov", "--coverageDirectory="+cd)
		case "mocha":
			if nodeModule(p.dir, root, "c8") == "" {
				p.hint = "install c8: npm i -D c8"
				return
			}
			args = append([]string{"npx", "--no-install", "c8", "--reporter=json", "--reporter=lcov", "--reports-dir=" + cd}, args...)
		}
	}
	p.steps = []step{{argv: args, env: env}}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o666)
}

// copyGlob copies dir/pattern files into dest (a dir), returning how many.
func copyGlob(dir, pattern, dest string) int {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	n := 0
	for _, m := range matches {
		if copyFile(m, filepath.Join(dest, filepath.Base(m))) == nil {
			n++
		}
	}
	return n
}

func javaPlan(p *plan, noCov bool) {
	p.results = filepath.Join(p.out, "results")
	p.cleanup = []string{filepath.Join(p.results, "*.xml")}
	if discover.IsFile(filepath.Join(p.dir, "pom.xml")) {
		p.tool = "surefire"
		if !onPath("mvn") {
			p.hint = "mvn is not installed: install Maven or pass --test-command"
			return
		}
		reports := filepath.Join(p.dir, "target", "surefire-reports")
		p.cleanup = append(p.cleanup, filepath.Join(reports, "TEST-*.xml"))
		args := []string{"mvn", "-q"}
		if !noCov {
			args = append(args, "org.jacoco:jacoco-maven-plugin:0.8.12:prepare-agent")
		}
		args = append(args, "test")
		if !noCov {
			args = append(args, "org.jacoco:jacoco-maven-plugin:0.8.12:report")
			p.coverageRep, p.granularity = filepath.Join(p.covDir(), "jacoco.xml"), "branch-count"
			p.cleanup = append(p.cleanup, p.coverageRep)
		}
		args = append(args, "-Dmaven.test.failure.ignore=true", "-Dsurefire.skipAfterFailureCount=0")
		p.steps = []step{{argv: args}}
		p.collect = func() error {
			copyGlob(reports, "TEST-*.xml", p.results)
			if p.coverageRep != "" {
				_ = copyFile(filepath.Join(p.dir, "target", "site", "jacoco", "jacoco.xml"), p.coverageRep)
			}
			return nil
		}
		return
	}
	p.tool = "gradle"
	gradle := "gradle"
	if discover.IsFile(filepath.Join(p.dir, "gradlew")) {
		gradle = "./gradlew"
	} else if !onPath("gradle") {
		p.hint = "gradle is not installed: install Gradle or pass --test-command"
		return
	}
	p.cleanup = append(p.cleanup, filepath.Join(p.dir, "build", "test-results", "test", "TEST-*.xml"))
	args := []string{gradle, "test", "--continue"}
	if !noCov {
		build := testplan.ReadOrEmpty(filepath.Join(p.dir, "build.gradle")) + testplan.ReadOrEmpty(filepath.Join(p.dir, "build.gradle.kts"))
		if !strings.Contains(build, "jacoco") {
			p.hint = "apply the jacoco plugin to get coverage, or pass --no-coverage"
			return
		}
		args = []string{gradle, "test", "jacocoTestReport", "--continue"}
		p.coverageRep, p.granularity = filepath.Join(p.covDir(), "jacoco.xml"), "branch-count"
		p.cleanup = append(p.cleanup, p.coverageRep)
	}
	p.steps = []step{{argv: args}}
	p.collect = func() error {
		copyGlob(filepath.Join(p.dir, "build", "test-results", "test"), "TEST-*.xml", p.results)
		if p.coverageRep != "" {
			_ = copyFile(filepath.Join(p.dir, "build", "reports", "jacoco", "test", "jacocoTestReport.xml"), p.coverageRep)
		}
		return nil
	}
}

// NextestConfig is the generated nextest profile (§2.1).
const NextestConfig = "[profile.coderefinery]\nfail-fast = false\n\n[profile.coderefinery.junit]\npath = \"junit.xml\"\n"

func rustPlan(p *plan, noCov bool) {
	p.tool = "nextest"
	if !onPath("cargo-nextest") {
		p.hint = "install cargo-nextest: cargo install cargo-nextest --locked"
		return
	}
	if !noCov && !onPath("cargo-llvm-cov") {
		p.hint = "install cargo-llvm-cov: cargo install cargo-llvm-cov --locked"
		return
	}
	p.results = filepath.Join(p.out, "results.xml")
	config := filepath.Join(p.covDir(), "nextest.toml")
	if noCov {
		p.steps = []step{{argv: append([]string{"cargo", "nextest", "run"}, NextestArgs(config)...)}}
	} else {
		jsonRep := filepath.Join(p.covDir(), "llvm-cov.json")
		lcov := filepath.Join(p.covDir(), "lcov.info")
		p.coverageRep, p.granularity = jsonRep, "region"
		p.cleanup = []string{jsonRep, lcov}
		p.steps = []step{
			{argv: append([]string{"cargo", "llvm-cov", "nextest", "--no-report"}, NextestArgs(config)...)},
			{argv: []string{"cargo", "llvm-cov", "report", "--json", "--output-path", jsonRep}, optional: true},
			{argv: []string{"cargo", "llvm-cov", "report", "--lcov", "--output-path", lcov}, optional: true},
		}
	}
	if err := os.MkdirAll(filepath.Dir(config), 0o777); err == nil {
		_ = os.WriteFile(config, []byte(NextestConfig), 0o666)
	}
	target := os.Getenv("CARGO_TARGET_DIR")
	if target == "" {
		target = filepath.Join(p.dir, "target")
	}
	junit := filepath.Join(target, "nextest", "coderefinery", "junit.xml")
	p.cleanup = append(p.cleanup, junit)
	p.collect = func() error { return copyFile(junit, p.results) }
}

func clojurePlan(p *plan, noCov bool) {
	p.tool = "kaocha"
	p.results = filepath.Join(p.out, "results.xml")
	reporter, hint := KaochaReporter(p.dir, p.results)
	if hint == "" && !noCov && !strings.Contains(testplan.ReadOrEmpty(filepath.Join(p.dir, "deps.edn")), "kaocha-cloverage") {
		hint = "add lambdaisland/kaocha-junit-xml (and lambdaisland/kaocha-cloverage) to the :test alias"
	}
	if hint == "" && !onPath("clj") {
		hint = "clj is not installed: install the Clojure CLI or pass --test-command"
	}
	if hint != "" {
		p.hint = hint
		return
	}
	args := append([]string{"clj", "-M:test"}, reporter...)
	if !noCov {
		cd := p.covDir()
		args = append(args, "--plugin", "cloverage", "--cov-output", cd, "--lcov")
		p.coverageRep, p.granularity = filepath.Join(cd, "lcov.info"), "form"
		p.cleanup = []string{p.coverageRep}
	}
	p.steps = []step{{argv: args}}
}

// commandPlan is --test-command: one shell run at the root.
func commandPlan(root string, opts Options, files []string) *plan {
	lang := "command"
	p := &plan{module: module{language: lang, dir: root, files: files}, tool: "", out: filepath.Join(root, ".metrics", "tests", lang)}
	p.results = filepath.Join(p.out, "results")
	p.granularity = "none"
	env := []string{"CODEREFINERY_RESULTS=" + p.results}
	if !opts.NoCoverage {
		env = append(env, "CODEREFINERY_COVERAGE_DIR="+p.covDir())
		_ = os.MkdirAll(p.covDir(), 0o777)
	}
	p.steps = []step{{shell: opts.TestCommand, env: env}}
	return p
}
