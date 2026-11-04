package mutate

// Ported from upstream tests/test_engine.py and the engine/worker parts of
// tests/test_boundaries.py.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func mutateOK(t *testing.T, path, root string, o fileOptions) RunResult {
	t.Helper()
	r, err := mutateFile(path, root, o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func filesUnder(t *testing.T, dir string) []string {
	var out []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestMutantsAreKilledOrKeptAndTheSnapshotIsDifferential(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	runner := &fakeRunner{decide: addTest}
	first := mutateOK(t, path, root, baseOptions(runner))
	if readFile(t, path) != addSource || first.BaselineFailed {
		t.Fatal("source must be unchanged and the baseline green")
	}
	form := first.Forms[0]
	if form.ID != "defn/add" || form.Namespace != "demo" || form.Killed != 1 || form.Survived != 3 || form.Sites != 4 ||
		form.Killed+form.Survived+form.Uncovered != form.Sites {
		t.Fatalf("form %+v", form)
	}
	snap := formsOf(loadSnapshot(t, root, "demo"))[0]
	if snap.Get("id") != "defn/add" || snap.Get("killed") != int64(1) || snap.Get("survived") != int64(3) {
		t.Fatalf("snapshot form %s", pyRepr(snap))
	}
	if first.Statuses[first.Sites[0].ID] != "survived" {
		t.Fatalf("statuses %v", first.Statuses)
	}
	firstCalls := runner.calls
	o := baseOptions(runner)
	o.mutateAll = false
	second := mutateOK(t, path, root, o)
	if second.Forms[0].Killed != 1 || second.Forms[0].Survived != 3 {
		t.Fatalf("second %+v", second.Forms[0])
	}
	// Baseline plus the three survivors; the killed mutant is carried.
	if got := runner.calls - firstCalls; got != 4 {
		t.Fatalf("second run made %d calls, want 4", got)
	}
	third := mutateOK(t, path, root, baseOptions(runner))
	if third.Forms[0].Killed != 1 || runner.calls-firstCalls != 4+5 {
		t.Fatalf("--mutate-all reruns everything: %d calls", runner.calls-firstCalls)
	}
	if files := filesUnder(t, filepath.Join(root, "target")); len(files) != 0 {
		t.Fatalf("workers left files: %v", files)
	}
}

func TestAChangedFunctionReRunsItsKilledMutants(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	runner := &fakeRunner{decide: addTest}
	mutateOK(t, path, root, baseOptions(runner))
	writeFile(t, path, strings.Replace(addSource, "return a + b\n", "return a + b  # edited\n", 1))
	before := runner.calls
	o := baseOptions(runner)
	o.mutateAll = false
	mutateOK(t, path, root, o)
	if got := runner.calls - before; got != 5 {
		t.Fatalf("a changed digest reruns all 4 sites plus the baseline, got %d calls", got)
	}
}

func TestUncoveredSitesAreNotExecuted(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	runner := &fakeRunner{decide: addTest}
	o := baseOptions(runner)
	o.ignoreCoverage, o.coveredLines = false, lineSet{}
	r := mutateOK(t, path, root, o)
	if runner.calls != 0 || r.Forms[0].Uncovered != 4 || r.Forms[0].Killed != 0 || r.Forms[0].Sites != 4 {
		t.Fatalf("calls %d form %+v", runner.calls, r.Forms[0])
	}
	if !strings.Contains(formatSiteLog(r.Sites, r.Statuses, nil), "UNCOVERED src/demo.py:2 > -> >=") {
		t.Fatal("site log must show UNCOVERED")
	}
}

func TestASiteInsideACoveredMultiLineStatementIsRun(t *testing.T) {
	root := tempRoot(t)
	src := "def add(a, b):\n    return (a\n            + b)\n"
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), src)
	runner := &fakeRunner{decide: func(string, time.Duration) int { return 1 }}
	o := baseOptions(runner)
	o.ignoreCoverage, o.coveredLines = false, lines(2)
	runner.decide = func(_ string, timeout time.Duration) int {
		if timeout == 0 {
			return 0
		}
		return 1
	}
	r := mutateOK(t, path, root, o)
	if r.Forms[0].Uncovered != 0 || r.Forms[0].Killed != 1 {
		t.Fatalf("form %+v", r.Forms[0])
	}
	text, err := scanFile(path, root, lines(2), false, nil)
	if err != nil || strings.Contains(text, "uncovered") || !strings.Contains(text, "+ -> -") {
		t.Fatalf("scan %q %v", text, err)
	}
}

func TestAFailedBaselineDoesNotRewriteMetrics(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	out := []string{"START"}
	for i := 1; i < 24; i++ {
		out = append(out, "M"+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	out = append(out, "END")
	runner := &fakeRunner{decide: func(string, time.Duration) int { return 1 }, output: strings.Join(out, "\n")}
	r := mutateOK(t, path, root, baseOptions(runner))
	if !r.BaselineFailed || !strings.HasPrefix(r.BaselineMessage, "Baseline failed for src/demo.py: fake\n") {
		t.Fatalf("result %+v", r)
	}
	if !strings.Contains(r.BaselineMessage, "M05") || strings.Contains(r.BaselineMessage, "START") || !strings.HasSuffix(r.BaselineMessage, "END") {
		t.Fatalf("tail must be the last 20 lines: %q", r.BaselineMessage)
	}
	if _, err := os.Stat(filepath.Join(root, ".metrics", "mutate", "demo.edn")); err == nil {
		t.Fatal("no snapshot on a failed baseline")
	}
	if runner.calls != 1 {
		t.Fatalf("calls %d", runner.calls)
	}
}

func TestTimeoutIsAFactorOfTheBaselineWithATwoSecondFloor(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), "def place(x):\n    return x > 0\n")
	slow := &fakeRunner{seconds: 30, decide: func(_ string, timeout time.Duration) int {
		if timeout == 0 {
			return 0
		}
		return 1
	}}
	r := mutateOK(t, path, root, baseOptions(slow))
	if slow.timeouts[0] != 0 || slow.timeouts[1] != 300*time.Second || r.Forms[0].Killed != 2 {
		t.Fatalf("timeouts %v form %+v", slow.timeouts, r.Forms[0])
	}
	fast := &fakeRunner{seconds: 0.01}
	mutateOK(t, path, root, baseOptions(fast))
	if fast.timeouts[1] != 2*time.Second {
		t.Fatalf("floor: %v", fast.timeouts)
	}
	if !strings.Contains(slow.cwds[1], filepath.Join("target", "mutation-workers", "run-")) {
		t.Fatalf("mutants run in a worker: %s", slow.cwds[1])
	}
}

func TestWarningsFollowCoverageAndTheSelectionSize(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), "def place(x):\n    return x > 0\n")
	o := baseOptions(&fakeRunner{})
	var stderr strings.Builder
	o.stderr, o.ignoreCoverage = &stderr, false
	r := mutateOK(t, path, root, o)
	if r.Forms[0].Uncovered != 2 || !strings.Contains(stderr.String(),
		"No coverage data for src/demo.py; its sites are uncovered. Pass --no-coverage to run them anyway.\n") {
		t.Fatalf("stderr %q", stderr.String())
	}
	stderr.Reset()
	o = baseOptions(&fakeRunner{decide: func(_ string, timeout time.Duration) int {
		if timeout == 0 {
			return 0
		}
		return 1
	}})
	o.stderr, o.mutationWarning = &stderr, 2
	if r := mutateOK(t, path, root, o); r.Forms[0].Killed != 2 || stderr.Len() != 0 {
		t.Fatalf("2 sites at --mutation-warning 2: %q", stderr.String())
	}
	o.mutationWarning = 1
	mutateOK(t, path, root, o)
	if !strings.Contains(stderr.String(), "WARNING: 2 covered mutations selected in src/demo.py.\n") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestLinesRestrictSelectionAndCounts(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "demo.py"), addSource)
	runner := &fakeRunner{decide: addTest}
	o := baseOptions(runner)
	o.lines = map[int]bool{3: true}
	r := mutateOK(t, path, root, o)
	if f := r.Forms[0]; f.Killed != 1 || f.Survived != 0 || f.Sites != 4 || runner.calls != 2 {
		t.Fatalf("form %+v calls %d", f, runner.calls)
	}
	if log := formatSiteLog(r.Sites, r.Statuses, nil); log != "KILLED    src/demo.py:3 + -> -\n" {
		t.Fatalf("site log %q", log)
	}
}

func TestWorkerCountFollowsSitesCoresAndTheCap(t *testing.T) {
	cores := runtime.NumCPU()
	if workerCount(10, 0) != min(10, cores) || workerCount(10, 3) != min(3, cores) || workerCount(2, 8) != min(2, cores) {
		t.Fatal("worker count")
	}
}

func TestWorkersKeepAPrivateCopyAndLinkTheRest(t *testing.T) {
	root := tempRoot(t)
	src := writeFile(t, filepath.Join(root, "src", "demo.py"), "def place(x):\n    return x > 0\n")
	sibling := writeFile(t, filepath.Join(root, "src", "other.py"), "kept = True\n")
	marker := writeFile(t, filepath.Join(root, "tests", "keep.txt"), "keep")
	writeFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname = 'demo'\n")
	writeFile(t, filepath.Join(root, "src", "build", "x.txt"), "build output")
	original := []byte(readFile(t, src))
	base, err := newRunDir(root)
	if err != nil {
		t.Fatal(err)
	}
	workers, err := createWorkers(base, root, "src/demo.py", original, 2)
	if err != nil || len(workers) != 2 {
		t.Fatalf("workers %v %v", workers, err)
	}
	for _, w := range workers {
		private := filepath.Join(w, "src", "demo.py")
		if isSymlink(private) || readFile(t, private) != string(original) {
			t.Fatal("private copy")
		}
		if !isSymlink(filepath.Join(w, "src", "other.py")) || !isSymlink(filepath.Join(w, "tests")) {
			t.Fatal("siblings must be links")
		}
		if readFile(t, filepath.Join(w, "tests", "keep.txt")) != "keep" {
			t.Fatal("linked content")
		}
		if isSymlink(filepath.Join(w, "pyproject.toml")) || !lexists(filepath.Join(w, "pyproject.toml")) {
			t.Fatal("configs are real copies")
		}
		if lexists(filepath.Join(w, "src", "build")) || lexists(filepath.Join(w, "target")) {
			t.Fatal("SKIP_LINK names are never linked, at any depth")
		}
	}
	writeFile(t, filepath.Join(workers[0], "src", "demo.py"), "changed\n")
	if readFile(t, filepath.Join(workers[1], "src", "demo.py")) != string(original) || readFile(t, src) != string(original) {
		t.Fatal("workers must be independent")
	}
	deleteTree(base)
	if readFile(t, src) != string(original) || readFile(t, sibling) != "kept = True\n" || readFile(t, marker) != "keep" {
		t.Fatal("deleteTree followed a link")
	}
	if lexists(base) {
		t.Fatal("base must be removed")
	}
}

func TestNodeImportersAreRealCopiesInTheWorker(t *testing.T) {
	root := tempRoot(t)
	src := writeFile(t, filepath.Join(root, "src", "find.mjs"), "export const value = \"real\"\n")
	writeFile(t, filepath.Join(root, "src", "find.test.mjs"), "import { value } from \"./find.mjs\";\nconsole.log(value)\n")
	writeFile(t, filepath.Join(root, "src", "notes.mjs"), "export const notes = 1\n")
	writeFile(t, filepath.Join(root, "src", "main.mjs"), "import { value } from \"./find.mjs\";\nexport const main = value\n")
	writeFile(t, filepath.Join(root, "src", "app.test.mjs"), "import { main } from \"./main.mjs\";\n")
	writeFile(t, filepath.Join(root, "tests", "use.mjs"), "import { value } from \"../src/find.mjs\";\nconsole.log(value)\n")
	writeFile(t, filepath.Join(root, "tests", "other.test.mjs"), "export const n = 1\n")
	if got := importersOf(root, "src/find.mjs"); strings.Join(got, ",") != "src/app.test.mjs,src/find.test.mjs,src/main.mjs,tests/use.mjs" {
		t.Fatalf("importers %v", got)
	}
	base, _ := newRunDir(root)
	defer deleteTree(base)
	workers, err := createWorkers(base, root, "src/find.mjs", []byte(readFile(t, src)), 1)
	if err != nil {
		t.Fatal(err)
	}
	w := workers[0]
	writeFile(t, filepath.Join(w, "src", "find.mjs"), "export const value = \"worker\"\n")
	for _, rel := range []string{"src/find.test.mjs", "src/main.mjs", "src/app.test.mjs", "tests/use.mjs"} {
		p := filepath.Join(w, rel)
		if !lexists(p) || isSymlink(p) {
			t.Errorf("%s must be a real copy", rel)
		}
	}
	if !isSymlink(filepath.Join(w, "src", "notes.mjs")) || isSymlink(filepath.Join(w, "tests")) || !isSymlink(filepath.Join(w, "tests", "other.test.mjs")) {
		t.Fatal("non-importers stay links; an expanded dir is real")
	}
	node, err := exec.LookPath("node")
	if err == nil {
		probe := exec.Command(node, "--version")
		probe.Dir = w
		err = probe.Run() // fails for a version-manager shim with no version here
	}
	if err == nil {
		for _, script := range []string{"src/find.test.mjs", "tests/use.mjs"} {
			cmd := exec.Command(node, script)
			cmd.Dir = w
			out, err := cmd.Output()
			if err != nil || strings.TrimSpace(string(out)) != "worker" {
				t.Errorf("node %s: %q %v", script, out, err)
			}
		}
	}
	if readFile(t, src) != "export const value = \"real\"\n" {
		t.Fatal("original touched")
	}
}

func TestRestoreBackupsPutsAnInterruptedMutantBack(t *testing.T) {
	root := tempRoot(t)
	src := writeFile(t, filepath.Join(root, "src", "demo.py"), "mutated\n")
	backup := writeFile(t, filepath.Join(root, "target", "mutator-backup", "src", "demo.py"), "original\n")
	writeFile(t, filepath.Join(root, "target", "mutator-backup", "a", "b", "deep.py"), "deep\n")
	restored, err := restoreBackups(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(restored, ",") != filepath.Join(root, "a", "b", "deep.py")+","+src {
		t.Fatalf("restored %v", restored)
	}
	if readFile(t, src) != "original\n" || lexists(backup) || readFile(t, filepath.Join(root, "a", "b", "deep.py")) != "deep\n" {
		t.Fatal("restore")
	}
}

func TestSkippedFilesAreReported(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "latin.py"), "")
	if err := os.WriteFile(path, []byte("x = '\xe9'\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if r := mutateOK(t, path, root, baseOptions(&fakeRunner{})); r.Skipped != "file is not UTF-8" {
		t.Fatalf("got %+v", r)
	}
	note := writeFile(t, filepath.Join(root, "note.txt"), "x")
	if r := mutateOK(t, note, root, baseOptions(&fakeRunner{})); r.Skipped != "unsupported file" || r.Path != "note.txt" {
		t.Fatalf("got %+v", r)
	}
}

func TestCarryForwardNeedsAnUnchangedCoveredForm(t *testing.T) {
	s := Site{Namespace: "demo", FormID: "defn/place", ID: "m"}
	h := History{Forms: map[formKey]string{{"demo", "defn/place"}: "old"}, Outcomes: map[string]string{"missing": "killed", "m": "killed"}}
	if got := carryForward([]Site{s}, map[string]bool{"m": true}, h, map[formKey]string{{"demo", "defn/place"}: "new"}); len(got) != 0 {
		t.Fatalf("changed digest carried %v", got)
	}
	if got := carryForward([]Site{s}, map[string]bool{"m": false}, h, map[formKey]string{{"demo", "defn/place"}: "old"}); len(got) != 0 {
		t.Fatalf("uncovered carried %v", got)
	}
	if got := carryForward([]Site{s}, map[string]bool{"m": true}, h, map[formKey]string{{"demo", "defn/place"}: "old"}); got["m"] != "killed" || len(got) != 1 {
		t.Fatalf("unchanged form carries exactly its site: %v", got)
	}
}
