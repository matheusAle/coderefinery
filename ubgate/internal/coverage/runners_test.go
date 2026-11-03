package coverage

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func tempRoot(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func touch(t *testing.T, path, text string) string {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

type recorder struct {
	calls [][]string
	codes map[string]int
}

func (r *recorder) exec(_ *Runner, c Command, cwd string) int {
	argv := c.Argv
	if argv == nil {
		argv = []string{"sh", c.Shell}
	}
	r.calls = append(r.calls, argv)
	return r.codes[strings.Join(argv, " ")]
}

// Ported from upstream tests/test_runners.py (Python parts).
func TestPythonRunnerArgvWithPytestProject(t *testing.T) {
	root := tempRoot(t)
	touch(t, filepath.Join(root, "pyproject.toml"), "[tool.pytest.ini_options]\n")
	src := touch(t, filepath.Join(root, "src", "pkg", "mod.py"), "x = 1\n")
	if got := PythonRoots(root, []string{src}); !reflect.DeepEqual(got, []string{root}) {
		t.Fatalf("roots %v", got)
	}
	if !UsesPytest(root) || PythonSources(root, []string{src}) != "src" {
		t.Fatal("pytest / sources")
	}
	rec := &recorder{codes: map[string]int{}}
	var stderr bytes.Buffer
	r := &Runner{Stdout: &bytes.Buffer{}, Stderr: &stderr, Exec: rec.exec}
	if _, err := r.RunCoverage(root, []string{src}, ""); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "target", "coverage", "python", ".coverage")
	want := [][]string{
		{"python3", "-c", "import coverage"},
		{"python3", "-c", "import pytest"},
		{"python3", "-m", "coverage", "run", "--data-file=" + data, "--source=src", "-m", "pytest"},
	}
	if !reflect.DeepEqual(rec.calls, want) {
		t.Fatalf("calls:\n%v\nwant\n%v", rec.calls, want)
	}
	if !strings.HasPrefix(stderr.String(), "+ ("+root+") python3 -c 'import coverage'\n") {
		t.Fatalf("echo: %q", stderr.String())
	}
	// The mock wrote no .coverage file, so `coverage lcov` never runs.
}

func TestPythonUnittestFallbackSourcesAndOutsideFiles(t *testing.T) {
	root := tempRoot(t)
	touch(t, filepath.Join(root, "setup.py"), "from setuptools import setup\n")
	top := touch(t, filepath.Join(root, "carddav.py"), "")
	outside := touch(t, filepath.Join(tempRoot(t), "elsewhere.py"), "")
	if UsesPytest(root) {
		t.Fatal("setup.py without pytest is unittest")
	}
	if got := PythonSources(root, []string{top, outside}); got != "carddav" {
		t.Fatalf("sources %q", got)
	}
	run, lcov := PythonCoverageCommands("py", "unittest", "/d", "/r", "")
	if !reflect.DeepEqual(run, []string{"py", "-m", "coverage", "run", "--data-file=/d", "-m", "unittest", "discover", "-s", "."}) ||
		!reflect.DeepEqual(lcov, []string{"py", "-m", "coverage", "lcov", "--data-file=/d", "-o", "/r"}) {
		t.Fatalf("argv %v %v", run, lcov)
	}
}

func TestPythonRunnerWarnings(t *testing.T) {
	root := tempRoot(t)
	src := touch(t, filepath.Join(root, "a.py"), "")
	rec := &recorder{codes: map[string]int{"python3 -c import coverage": 1, "python3 -m pip install --disable-pip-version-check coverage": 1}}
	var stderr bytes.Buffer
	r := &Runner{Stdout: &bytes.Buffer{}, Stderr: &stderr, Exec: rec.exec}
	_, _ = r.RunCoverage(root, []string{src}, "")
	if !strings.Contains(stderr.String(), "coverage is missing in "+root+". Python coverage will score 0%.\n") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestRunShellRealExitCodesAndStartFailure(t *testing.T) {
	var stderr bytes.Buffer
	r := &Runner{Stdout: &bytes.Buffer{}, Stderr: &stderr}
	dir := tempRoot(t)
	if code := r.RunShell(Command{Shell: "exit 3"}, dir); code != 3 {
		t.Fatalf("exit 3 -> %d", code)
	}
	if code := r.RunShell(Argv("definitely-not-a-binary-ubgate"), dir); code != 127 {
		t.Fatalf("missing binary -> %d", code)
	}
	if !strings.Contains(stderr.String(), "Coverage command failed to start: [Errno 2] No such file or directory: 'definitely-not-a-binary-ubgate'\n") {
		t.Fatalf("stderr %q", stderr.String())
	}
	// An argv is never shell-interpreted.
	r.RunShell(Argv("echo", "a;touch pwned"), dir)
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("argv went through a shell")
	}
}

func TestShellQuoteAndCoverageReportSlug(t *testing.T) {
	cases := map[string]string{"": "''", "plain/path-1.py": "plain/path-1.py", "a b": "'a b'", "it's": `'it'"'"'s'`}
	for in, want := range cases {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
	root := tempRoot(t)
	_ = os.MkdirAll(filepath.Join(root, "src-tauri", "x"), 0o755)
	got, err := CoverageReport(root, filepath.Join(root, "src-tauri", "x"), "rust")
	if err != nil || got != filepath.Join(root, "target", "coverage", "rust", "src-tauri__x", "lcov.info") {
		t.Fatalf("report %q %v", got, err)
	}
}

func TestCustomCommandFailureStatus(t *testing.T) {
	var stderr bytes.Buffer
	r := &Runner{Stdout: &bytes.Buffer{}, Stderr: &stderr}
	code, err := r.RunCoverage(tempRoot(t), nil, "exit 9")
	if err != nil || code != 9 || !strings.Contains(stderr.String(), "Coverage command exited 9. Reports already on disk will not be read.\n") {
		t.Fatalf("code %d err %v stderr %q", code, err, stderr.String())
	}
}
