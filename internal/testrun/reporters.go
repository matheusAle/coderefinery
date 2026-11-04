package testrun

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/matheusAle/coderefinery/internal/testplan"
)

// The reporter pieces `test` and `mutate --attribute` share (§2.1).

// PytestReporter makes pytest write xunit1 JUnit (node ids) to results and
// cancels a fail-fast -x in addopts.
func PytestReporter(results string) []string {
	return []string{"--junitxml=" + results, "-o", "junit_family=xunit1", "--maxfail=0"}
}

// VitestReporter adds the built-in junit reporter.
func VitestReporter(results string) []string {
	return []string{"--reporter=default", "--reporter=junit", "--outputFile.junit=" + results}
}

// JestReporter is the jest-junit reporter args and env (file attribute,
// describe path in classname, the leaf title in name, suite errors kept).
func JestReporter(results string) (args, env []string) {
	return []string{"--ci", "--reporters=default", "--reporters=jest-junit"},
		[]string{"JEST_JUNIT_OUTPUT_FILE=" + results, "JEST_JUNIT_ADD_FILE_ATTRIBUTE=true",
			"JEST_JUNIT_CLASSNAME={classname}", "JEST_JUNIT_TITLE={title}", "JEST_JUNIT_ANCESTOR_SEPARATOR= › ",
			"JEST_JUNIT_REPORT_TEST_SUITE_ERRORS=true"}
}

// MochaReporter is mocha-junit-reporter with full suite titles.
func MochaReporter(results string) []string {
	return []string{"--reporter", "mocha-junit-reporter", "--reporter-options",
		"mochaFile=" + results + ",useFullSuiteTitle=true,suiteTitleSeparatedBy=›,includePending=true"}
}

// NodeRunner is the runner npm test starts (vitest|jest|mocha), or a hint
// when no JUnit reporter can be added.
func NodeRunner(dir string) (string, string) {
	script, ok := testScript(dir)
	first := ""
	if f := strings.Fields(script); len(f) > 0 {
		first = filepath.Base(f[0])
	}
	if !ok || first != "vitest" && first != "jest" && first != "mocha" {
		return "", fmt.Sprintf("cannot add a JUnit reporter to npm test (%q): pass --test-command", script)
	}
	return first, ""
}

// NodePluginHint names a missing reporter plugin for runner ("" = none).
func NodePluginHint(runner, dir, root string) string {
	switch runner {
	case "jest":
		if nodeModule(dir, root, "jest-junit") == "" {
			return "install jest-junit: npm i -D jest-junit"
		}
	case "mocha":
		if nodeModule(dir, root, "mocha-junit-reporter") == "" {
			return "install mocha-junit-reporter: npm i -D mocha-junit-reporter"
		}
	}
	return ""
}

// NodeReporter is the npm test argv/env tail for runner.
func NodeReporter(runner, results string) (args, env []string) {
	switch runner {
	case "vitest":
		return VitestReporter(results), []string{"CI=true"}
	case "jest":
		a, e := JestReporter(results)
		return a, append([]string{"CI=true"}, e...)
	}
	return MochaReporter(results), nil
}

// KaochaReporter is the kaocha-junit-xml plugin args, or a hint.
func KaochaReporter(dir, results string) ([]string, string) {
	deps := testplan.ReadOrEmpty(filepath.Join(dir, "deps.edn"))
	if !strings.Contains(deps, "kaocha") {
		return nil, "only Kaocha (deps.edn :test alias) is supported for Clojure: pass --test-command"
	}
	if !strings.Contains(deps, "kaocha-junit-xml") {
		return nil, "add lambdaisland/kaocha-junit-xml (and lambdaisland/kaocha-cloverage) to the :test alias"
	}
	return []string{"--plugin", "kaocha.plugin/junit-xml", "--junit-xml-file", results}, ""
}

// NextestArgs runs nextest with the generated coderefinery profile; the
// JUnit file lands in <target>/nextest/coderefinery/junit.xml.
func NextestArgs(config string) []string {
	return []string{"--no-fail-fast", "--config-file", config, "--profile", "coderefinery"}
}
