package mutate

// Ported from upstream tests/test_cli.py and the CLI parts of
// tests/test_boundaries.py. The real-Python cases are the end-to-end smoke
// test: real subprocesses, real worker overlays, real snapshots.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type cliResult struct {
	code           int
	stdout, stderr string
}

func cli(args ...string) cliResult {
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return cliResult{code, out.String(), errb.String()}
}

func needPython(t *testing.T) string {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	return py
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Dev", "GIT_AUTHOR_EMAIL=dev@example.com",
		"GIT_COMMITTER_NAME=Dev", "GIT_COMMITTER_EMAIL=dev@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

const placeSource = "def place(x):\n    return x > 0\n"

func TestScanListsSitesWithoutWritingMetrics(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "src", "demo.py"), placeSource)
	r := cli("--scan", "--no-coverage", "--root", root, "src/demo.py")
	want := "Scan: 2 mutation sites in src/demo.py\n  src/demo.py:2 > -> >=  [defn/place]\n  src/demo.py:2 0 -> 1  [defn/place]\n"
	if r.code != 0 || r.stdout != want || r.stderr != "" {
		t.Fatalf("%+v", r)
	}
	if lexists(filepath.Join(root, ".metrics")) {
		t.Fatal("--scan must not write metrics")
	}
	if r := cli("--scan", "--no-coverage", "--lines", "3", "--root", root); r.stdout != "Scan: 0 mutation sites in src/demo.py\n" {
		t.Fatalf("--lines filters the scan: %q", r.stdout)
	}
}

func TestScanMarksChangedFunctionsAndUncoveredSites(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), placeSource+"\ndef other(y):\n    return y - 1\n")
	if r := mutateOK(t, path, root, baseOptions(&fakeRunner{})); len(r.Forms) != 2 {
		t.Fatalf("forms %+v", r.Forms)
	}
	writeFile(t, path, placeSource+"\ndef other(y):\n    return y - 2\n")
	writeFile(t, filepath.Join(root, "target", "coverage", "python", "lcov.info"), "SF:src/demo.py\nDA:2,1\nDA:5,0\nend_of_record\n")
	r := cli("--scan", "--root", root)
	want := "Scan: 3 mutation sites in src/demo.py\n" +
		"  src/demo.py:2 > -> >=  [defn/place]\n" +
		"  src/demo.py:2 0 -> 1  [defn/place]\n" +
		"* src/demo.py:5 - -> + uncovered  [defn/other]\n" +
		"* marks a function whose text changed since the last snapshot.\n"
	if r.code != 0 || r.stdout != want {
		t.Fatalf("got %q", r.stdout)
	}
}

func TestADirectoryArgumentSkipsANamedTestFile(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "pkg", "app.py"), placeSource)
	test := writeFile(t, filepath.Join(root, "pkg", "test_app.py"), "def test_place():\n    assert True\n")
	r := cli("--scan", "--no-coverage", "--root", root, filepath.Join(root, "pkg"), test)
	if r.code != 0 || r.stderr != "Skipping test file "+test+"\n" || !strings.Contains(r.stdout, "pkg/app.py:2") {
		t.Fatalf("%+v", r)
	}
}

func TestSelectionJoinsTheRootAndFilters(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "src", "demo.py"), placeSource)
	writeFile(t, filepath.Join(root, "src", "other.py"), placeSource)
	writeFile(t, filepath.Join(root, "note.txt"), "hello\n")
	got, err := selectFiles(ParseArgs([]string{"--root", root, "src/demo.py", "note.txt"}), &strings.Builder{})
	if err != nil || strings.Join(got, ",") != filepath.Join(root, "src", "demo.py") {
		t.Fatalf("explicit: %v %v", got, err)
	}
	got, _ = selectFiles(ParseArgs([]string{"--root", root, "other"}), &strings.Builder{})
	if strings.Join(got, ",") != filepath.Join(root, "src", "other.py") {
		t.Fatalf("a non-path argument is a filter: %v", got)
	}
	r := cli("--scan", "--no-coverage", "--root", root, "--source-root", "src")
	if r.code != 0 || !strings.Contains(r.stdout, "src/demo.py") || !strings.Contains(r.stdout, "src/other.py") {
		t.Fatalf("--source-root: %+v", r)
	}
	outside := writeFile(t, filepath.Join(tempRoot(t), "far.py"), placeSource)
	var stderr strings.Builder
	got, _ = selectFiles(ParseArgs([]string{"--root", root, outside}), &stderr)
	if len(got) != 0 || stderr.String() != "Skipping "+outside+"; it is outside "+root+"\n" {
		t.Fatalf("outside: %v %q", got, stderr.String())
	}
}

func TestAnEmptyTreeHasNothingToMutate(t *testing.T) {
	r := cli("--no-coverage", "--root", tempRoot(t))
	if r.code != 0 || r.stdout != "No source files to mutate.\n" {
		t.Fatalf("%+v", r)
	}
}

func TestHelpAndConflictsUseDifferentStreams(t *testing.T) {
	if r := cli("--help"); r.code != 0 || !strings.Contains(r.stdout, "uml-viewer") || !strings.Contains(r.stdout, "__pycache__, __tests__") || r.stderr != "" {
		t.Fatalf("help %d %q", r.code, r.stderr)
	}
	if r := cli("--mutate-all", "--since-last-run"); r.code != 1 || r.stdout != "" || !strings.HasPrefix(r.stderr, "--mutate-all cannot be combined with --since-last-run\n\n") {
		t.Fatalf("conflict %+v", r.code)
	}
	if o := ParseArgs([]string{"--fix-go-coverage", "--root", "x"}); !o.FixGoCoverage || o.ExitCode != 0 || o.ProjectRoot != "x" {
		t.Fatalf("--fix-go-coverage: %+v", o)
	}
	if o := ParseArgs(nil); o.MinTimeout != 2 {
		t.Fatalf("upstream floor: %v", o.MinTimeout)
	}
	if o := ParseArgs([]string{"--min-timeout", "300", "--root", "x"}); o.MinTimeout != 300 || o.ExitCode != 0 || o.ProjectRoot != "x" {
		t.Fatalf("--min-timeout: %+v", o)
	}
	for _, bad := range []string{"0", "-1", "inf", "nan", "soon"} {
		if o := ParseArgs([]string{"--min-timeout", bad}); o.ExitCode != 1 || !strings.HasPrefix(o.Message, "--min-timeout requires a ") {
			t.Fatalf("--min-timeout %s: %d %q", bad, o.ExitCode, o.Message)
		}
	}
	if o := ParseArgs([]string{"--root"}); o.ExitCode != 1 || !strings.HasPrefix(o.Message, "--root requires a value") {
		t.Fatal("--root needs a value")
	}
}

func TestChangedFollowsGitStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	base := tempRoot(t)
	if r := cli("--changed", "--root", base); r.code != 128 || !strings.Contains(r.stderr, "not a git repository") || strings.Contains(r.stdout, "No source files") {
		t.Fatalf("outside git: %+v", r)
	}
	repo := filepath.Join(base, "repo")
	writeFile(t, filepath.Join(repo, "src", "old.py"), placeSource)
	writeFile(t, filepath.Join(repo, "gone.py"), placeSource)
	git(t, repo, "init", "-q")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "base")
	keep := writeFile(t, filepath.Join(repo, "src", "café.py"), placeSource)
	spaced := writeFile(t, filepath.Join(repo, "src", "my file.py"), placeSource)
	nested := writeFile(t, filepath.Join(repo, "fresh", "nested", "new.py"), placeSource)
	writeFile(t, filepath.Join(repo, "target", "skip.py"), placeSource)
	writeFile(t, filepath.Join(repo, "src", "test_x.py"), placeSource)
	_ = os.Remove(filepath.Join(repo, "gone.py"))
	git(t, repo, "mv", "src/old.py", "src/new.py")
	chosen, err := selectFiles(ParseArgs([]string{"--root", repo, "--changed"}), &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{nested, keep, spaced, filepath.Join(repo, "src", "new.py")}
	if strings.Join(chosen, "|") != strings.Join(want, "|") {
		t.Fatalf("chosen %v\nwant %v", chosen, want)
	}
	if r := cli("--scan", "--no-coverage", "--root", filepath.Join(repo, "src"), "--changed"); r.code != 0 || !strings.Contains(r.stdout, "café.py:2") || strings.Contains(r.stdout, "fresh/") {
		t.Fatalf("--changed under a subdirectory: %+v", r)
	}
}

func pythonTest(py string) string {
	return py + " -c \"import sys; sys.path.insert(0, 'src'); from demo import add; raise SystemExit(0 if add(2, 3) == 5 else 1)\""
}

func TestRealPythonMutantsUpdateTheSnapshot(t *testing.T) {
	py := needPython(t)
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), "def add(a, b):\n    if a > 0:\n        return a + b\n    return 0\n")
	r := cli("--no-coverage", "--mutate-all", "--root", root, "--test-command", pythonTest(py), "src/demo.py")
	wantLog := "SURVIVED  src/demo.py:2 > -> >=\nSURVIVED  src/demo.py:2 0 -> 1\nKILLED    src/demo.py:3 + -> -\nSURVIVED  src/demo.py:4 0 -> 1\n"
	if r.code != 3 || !strings.HasPrefix(r.stdout, wantLog) {
		t.Fatalf("exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	table := "Function Namespace  Killed  Survived  Uncovered  Sites   Score\n" +
		"--------------------------------------------------------------\n" +
		"add      demo            1         3          0      4  25.0%\n" +
		"--------------------------------------------------------------\n" +
		"Total                    1         3          0      4   25.0%\n"
	if !strings.HasSuffix(r.stdout, table) {
		t.Fatalf("table:\n%s", r.stdout)
	}
	snapshot := filepath.Join(root, ".metrics", "mutate", "demo.edn")
	if r.stderr != "Wrote "+snapshot+"\n" {
		t.Fatalf("stderr %q", r.stderr)
	}
	text := readFile(t, snapshot)
	if !strings.Contains(text, `:namespace "demo"`) || !strings.Contains(text, `:id "defn/add"`) || !strings.Contains(text, `:killed 1 :survived 3`) {
		t.Fatalf("snapshot %s", text)
	}
	if !strings.HasPrefix(readFile(t, path), "def add") || len(filesUnder(t, filepath.Join(root, "target"))) != 0 {
		t.Fatal("tree must be left unchanged")
	}
	// The default run reruns only the survivors.
	again := cli("--no-coverage", "--root", root, "--test-command", pythonTest(py), "src/demo.py")
	if again.code != 3 || !strings.HasPrefix(again.stdout, wantLog) {
		t.Fatalf("rerun %+v", again)
	}
}

func TestCLIExitCodesFollowKillsAndBaselineFailure(t *testing.T) {
	py := needPython(t)
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), "def add(a, b):\n    return a + b\n")
	r := cli("--no-coverage", "--mutate-all", "--root", root, "--test-command", pythonTest(py), "src/demo.py")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "KILLED    src/demo.py:2 + -> -\n") {
		t.Fatalf("%+v", r)
	}
	snapshot := filepath.Join(root, ".metrics", "mutate", "demo.edn")
	recorded := readFile(t, snapshot)
	writeFile(t, path, placeSource)
	failing := py + " -c 'print(\"boom\"); raise SystemExit(1)'"
	r = cli("--no-coverage", "--mutate-all", "--root", root, "--test-command", failing, "src/demo.py")
	if r.code != 2 || r.stderr != "Baseline failed for src/demo.py: "+failing+"\nboom\n" || r.stdout != "" {
		t.Fatalf("%+v", r)
	}
	if readFile(t, snapshot) != recorded {
		t.Fatal("a failed baseline must not rewrite the snapshot")
	}
}

func TestAFailedCoverageCommandDoesNotReadTheReport(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "app.py"), placeSource)
	writeFile(t, filepath.Join(root, "target", "coverage", "python", "lcov.info"), "SF:src/app.py\nDA:1,1\nDA:2,1\nend_of_record\n")
	r := cli("--root", root, "--coverage-command", "echo partial; exit 3", "--test-command", "false", "src/app.py")
	if r.code != 3 || r.stderr != "Coverage command exited 3. Reports already on disk will not be read.\npartial\n" || r.stdout != "" {
		t.Fatalf("%+v", r)
	}
	if !strings.HasPrefix(readFile(t, path), "def place") {
		t.Fatal("source changed")
	}
	if r := cli("--root", root, "--coverage-command", "true", "--test-command", "false", "--use-existing-coverage", "src/app.py"); r.code != 2 {
		t.Fatalf("a covered site runs the (failing) baseline: %+v", r)
	}
}

func TestAScanDoesNotRestoreABackupAndARunDoes(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), placeSource)
	clean := "def place(x):\n    return x > 1\n"
	backup := writeFile(t, filepath.Join(root, "target", "mutator-backup", "src", "demo.py"), clean)
	r := cli("--scan", "--no-coverage", "--root", root, "src/demo.py")
	if r.code != 0 || readFile(t, path) != placeSource || !strings.Contains(r.stdout, "0 -> 1") || strings.Contains(r.stdout, "1 -> 0") {
		t.Fatalf("%+v", r)
	}
	r = cli("--no-coverage", "--mutate-all", "--root", root, "--test-command", "exit 1", "src/demo.py")
	if r.code != 2 || readFile(t, path) != clean || lexists(backup) ||
		!strings.HasPrefix(r.stderr, "Restored "+path+" from an interrupted mutation.\nBaseline failed for src/demo.py: exit 1\n") {
		t.Fatalf("%+v", r)
	}
}

func TestVerboseEchoesCommandsAndMutants(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "src", "demo.py"), "def place(x):\n    return x\n\ndef other():\n    return 1\n")
	r := cli("--no-coverage", "--verbose", "--root", root, "--test-command", "exit 0")
	if r.code != 3 || !strings.Contains(r.stderr, "+ ("+root+") exit 0\n") || !strings.Contains(r.stderr, "src/demo.py:5 1 -> 0\n") ||
		!strings.Contains(r.stderr, "/target/mutation-workers/run-") {
		t.Fatalf("%+v", r)
	}
}
