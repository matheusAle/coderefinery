package dry

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/discover"
)

func TestHelpIsByteExact(t *testing.T) {
	var out bytes.Buffer
	if code := Main([]string{"--nope", "--help"}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if sum := fmt.Sprintf("%x", md5.Sum(out.Bytes())); out.Len() != 2349 || sum != "740bd09b788b09a72ecfc61a09992bb7" {
		t.Fatalf("help: %d bytes, md5 %s", out.Len(), sum)
	}
}

func TestParseArgs(t *testing.T) {
	o := ParseArgs([]string{"--root", "r", "-s", "a", "--threshold", " 0.5 ", "--min-lines", "+5", "--min-nodes", "1_0", "--edn", "p"})
	if o.ProjectRoot != "r" || o.SourceRoots[0] != "a" || o.Threshold != 0.5 || o.MinLines != 5 || o.MinNodes != 10 || o.Format != "edn" || o.Positionals[0] != "p" {
		t.Fatalf("parsed %+v", o)
	}
	errs := map[string][]string{
		"--threshold requires a number":   {"--threshold", "x"},
		"--min-lines requires an integer": {"--min-lines", "1.5"},
		"--min-nodes requires a value":    {"--min-nodes", "-1"},
		"Unknown option: --x":             {"--x"},
		"--format requires a value":       {"--format"},
	}
	for msg, args := range errs {
		if o := ParseArgs(args); o.ExitCode != 1 || !strings.HasPrefix(o.Message, msg+"\n\nUsage: dryer") {
			t.Errorf("%v -> %q", args, o.Message[:min(40, len(o.Message))])
		}
	}
	var stderr bytes.Buffer
	if code := Main([]string{"--format", "xml"}, &bytes.Buffer{}, &stderr); code != 2 || stderr.String() != "Unknown format: xml\n" {
		t.Fatalf("format check: %d %q", code, stderr.String())
	}
}

// Ports of upstream dryer tests/test_cli.py.

const (
	cliBodyA = "def alpha(xs):\n    ys = filter(xs, odd)\n    zs = map(ys, inc)\n    return list(zs)\n"
	cliBodyB = "def beta(items):\n    kept = filter(items, even)\n    out = map(kept, dec)\n    return list(out)\n"
)

func runDry(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestLanguageDetection(t *testing.T) {
	for path, want := range map[string]string{
		"src/app/core.clj": "clojure", "src/app/core.cljd": "clojure", "Widget.java": "java", "board.go": "go",
		"ui/view.tsx": "typescript", "src/lib.rs": "rust", "src/dryer/cli.py": "python",
		"types.d.ts": "", "notes.md": "", "lib/a.js": "", "x.d.mts": "typescript",
	} {
		if got := LanguageOf(path); got != want {
			t.Errorf("LanguageOf(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	for _, c := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{"--nope"}, 1, "Unknown option: --nope\n\nUsage: dryer"},
		{[]string{"--threshold"}, 1, "--threshold requires a value\n\n"},
		{[]string{"file.py", "--min-lines", ""}, 1, "--min-lines requires a value\n\n"},
		{[]string{"--min-lines", "--min-nodes", "1"}, 1, "--min-lines requires a value\n\n"},
		{[]string{"--min-lines", "nope"}, 1, "--min-lines requires an integer\n\n"},
		{[]string{"--min-lines", "-1"}, 1, "--min-lines requires a value\n\n"},
		{[]string{"--min-lines", " -1"}, 1, "--min-lines requires a non-negative integer\n\n"},
		{[]string{"--threshold", "nope"}, 1, "--threshold requires a number\n\n"},
		{[]string{"--format", "csv"}, 2, "Unknown format: csv\n"},
		{[]string{"--format", "csv", "--nope"}, 1, "Unknown option: --nope\n"},
	} {
		code, out, errs := runDry(c.args...)
		if code != c.code || out != "" || !strings.HasPrefix(errs, c.msg) {
			t.Errorf("%q: exit %d, stdout %q, stderr %.60q", c.args, code, out, errs)
		}
	}
	if code, out, _ := runDry("--help"); code != 0 || !strings.Contains(out, "__pycache__") || !strings.Contains(out, ".test.cts") {
		t.Fatalf("help: %d", code)
	}
}

func TestParseDefaultsAndFlags(t *testing.T) {
	o := ParseArgs([]string{"--threshold", "0.9", "--min-lines", "5", "--min-nodes", "30", "--edn", "spec"})
	if o.Threshold != 0.9 || o.MinLines != 5 || o.MinNodes != 30 || o.Format != "edn" || strings.Join(o.Positionals, ",") != "spec" {
		t.Fatalf("parsed %+v", o)
	}
	if z := ParseArgs([]string{"--min-lines", "0", "--min-nodes", "0"}); z.Help || z.MinLines != 0 || z.MinNodes != 0 {
		t.Fatalf("zero counts: %+v", z)
	}
	if d := ParseArgs(nil); d.Threshold != 0.82 || d.MinLines != 4 || d.MinNodes != 20 || d.Format != "text" || d.ProjectRoot != "." {
		t.Fatalf("defaults %+v", d)
	}
}

func TestProjectReportAndSnapshot(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "src/left.py", cliBodyA)
	writeSource(t, root, "src/right.py", cliBodyB)
	writeSource(t, root, "tests/test_left.py", cliBodyA)
	code, out, errs := runDry("--root", root, "--min-lines", "3", "--min-nodes", "1")
	if code != 0 || out != "DUPLICATE score=1.00\n  src/left.py:1-4\n  src/right.py:1-4\n" {
		t.Fatalf("exit %d stdout %q", code, out)
	}
	metrics := filepath.Join(discover.Resolve(root), ".metrics", "dry.edn")
	if errs != "Wrote "+metrics+"\n" {
		t.Fatalf("stderr %q", errs)
	}
	text, err := os.ReadFile(metrics)
	want := "{:candidates [\n {:score 1.0\n  :language \"python\"\n  :left {:file \"src/left.py\", :start-line 1, :end-line 4}\n  :right {:file \"src/right.py\", :start-line 1, :end-line 4}\n  :left-nodes 48\n  :right-nodes 48}\n]}\n"
	if err != nil || string(text) != want {
		t.Fatalf("metrics %q %v", text, err)
	}
	if _, edn, _ := runDry("--root", root, "--min-lines", "3", "--min-nodes", "1", "--edn"); edn != want {
		t.Fatalf("--edn stdout %q", edn)
	}
}

func TestEDNStdoutAndTextWinsWhenLast(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "one.py", "def alpha(xs):\n    return xs\n")
	if _, out, _ := runDry("--root", root, "--edn", "--min-lines", "1", "--min-nodes", "1"); out != "{:candidates []}\n" {
		t.Fatalf("edn %q", out)
	}
	if _, out, _ := runDry("--root", root, "--edn", "--text", "--min-lines", "1", "--min-nodes", "1"); out != "No duplicate candidates found.\n" {
		t.Fatalf("text %q", out)
	}
}

func TestEmptyProjectWritesNoMetrics(t *testing.T) {
	root := t.TempDir()
	code, out, errs := runDry("--root", root)
	if code != 0 || out != "No source files to analyze.\n" || errs != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(root, ".metrics")); err == nil {
		t.Fatal(".metrics was created")
	}
	writeSource(t, root, "lib/format.mjs", "export const a = (x) => x;\n")
	if _, out, _ := runDry("--root", root); out != "No source files to analyze.\n" {
		t.Fatalf("javascript is not supported: %q", out)
	}
}

func TestPathFilter(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"board", "other"} {
		writeSource(t, root, "src/"+dir+"/a.py", cliBodyA)
		writeSource(t, root, "src/"+dir+"/b.py", cliBodyB)
	}
	_, out, _ := runDry("--root", root, "--min-lines", "3", "--min-nodes", "1", "board")
	if out != "DUPLICATE score=1.00\n  src/board/a.py:1-4\n  src/board/b.py:1-4\n" {
		t.Fatalf("stdout %q", out)
	}
}

func TestExplicitTestFileIsIncluded(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "src/a.py", cliBodyA)
	writeSource(t, root, "tests/test_a.py", cliBodyB)
	_, out, _ := runDry("--root", root, "--min-lines", "3", "--min-nodes", "1", "src/a.py", "tests/test_a.py")
	if out != "DUPLICATE score=1.00\n  src/a.py:1-4\n  tests/test_a.py:1-4\n" {
		t.Fatalf("stdout %q", out)
	}
	if _, out, _ := runDry("--root", root, "--min-lines", "3", "--min-nodes", "1", "tests"); out != "No source files to analyze.\n" {
		t.Fatalf("an explicit tests dir is walked and finds nothing: %q", out)
	}
}

func TestNamesThatAreTests(t *testing.T) {
	for _, name := range []string{"pkg/foo_test.go", "src/app_test.clj", "src/app_test.cljc", "src/app_test.cljs",
		"src/app_test.cljd", "src/app_test.bb", "ui/a.test.ts", "ui/a.spec.ts", "ui/a.test.tsx", "ui/a.spec.tsx",
		"ui/a.test.mts", "ui/a.spec.mts", "ui/a.test.cts", "ui/a.spec.cts", "tests/conftest.py", "pkg/foo_test.py",
		"pkg/test_foo.py", "tests/app.py", "/x/specs/proj/src/a.py"} {
		if !IsTestFile(name) {
			t.Errorf("%s is not a test file", name)
		}
	}
	for _, name := range []string{"src/app.py", "lib/a.test.js", "src/testing.py"} {
		if IsTestFile(name) {
			t.Errorf("%s is a test file", name)
		}
	}
}

func TestTestFilesAndSkippedDirectoriesAreLeftOut(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"src/app.py", "src/foo_test.py", "target/app.py", "tests/app.py"} {
		writeSource(t, root, name, "x = 1\n")
	}
	for _, name := range []string{"src/app.test.cts", "src/app.spec.cts", "src/widget.cts"} {
		writeSource(t, root, name, "export {}\n")
	}
	var rel []string
	for _, p := range IterSourceFiles([]string{discover.Resolve(root)}) {
		r, _ := discover.RelativeTo(p, discover.Resolve(root))
		rel = append(rel, r)
	}
	if strings.Join(rel, " ") != "src/app.py src/widget.cts" {
		t.Fatalf("walk %v", rel)
	}
}

func TestSourceRootLimitsTheWalk(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "src/a.py", "x = 1\n")
	writeSource(t, root, "extra/b.py", "x = 1\n")
	files, err := SelectFiles(ParseArgs([]string{"--root", root, "--source-root", "src"}))
	if err != nil || len(files) != 1 || files[0] != filepath.Join(discover.Resolve(root), "src", "a.py") {
		t.Fatalf("files %v %v", files, err)
	}
}

func TestASecondReportReplacesTheSnapshot(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "a.py", cliBodyA)
	writeSource(t, root, "b.py", cliBodyB)
	runDry("--root", root, "--min-lines", "1", "--min-nodes", "1")
	os.Remove(filepath.Join(root, "b.py"))
	runDry("--root", root, "--min-lines", "1", "--min-nodes", "1")
	if text, _ := os.ReadFile(filepath.Join(root, ".metrics", "dry.edn")); string(text) != "{:candidates []}\n" {
		t.Fatalf("snapshot %q", text)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "dev@example.com")
	gitRun(t, dir, "config", "user.name", "Dev")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
}

func TestChangedLimitsTheReport(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeSource(t, root, "src/c.py", cliBodyA)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base")
	writeSource(t, root, "src/a.py", cliBodyA)
	writeSource(t, root, "src/b.py", cliBodyB)
	writeSource(t, root, "tests/test_a.py", cliBodyB)
	writeSource(t, root, "target/x.py", cliBodyB)
	writeSource(t, root, "README.md", "notes\n")
	code, out, _ := runDry("--root", root, "--changed", "--min-lines", "3", "--min-nodes", "1", "src/c.py")
	if code != 0 || out != "DUPLICATE score=1.00\n  src/a.py:1-4\n  src/b.py:1-4\n" {
		t.Fatalf("exit %d stdout %q", code, out)
	}
}

func TestChangedFromASubdirectoryStaysInsideIt(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeSource(t, repo, "src/above.py", "x = 1\n")
	writeSource(t, repo, "sub/src/below.py", "x = 1\n")
	files, err := SelectFiles(ParseArgs([]string{"--root", filepath.Join(repo, "sub"), "--changed"}))
	if err != nil || len(files) != 1 || files[0] != filepath.Join(discover.Resolve(repo), "sub", "src", "below.py") {
		t.Fatalf("files %v %v", files, err)
	}
}

func TestChangedOutsideARepoExitsWithGitsCode(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(discover.Resolve(root)))
	code, out, errs := runDry("--changed", "--root", root)
	if code != 128 || out != "" || !strings.Contains(errs, "not a git repository") || strings.Count(errs, "\n") != 1 {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errs)
	}
}

func TestTrackedSourceSkipsUnknownTestAndSkippedFiles(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	for path, want := range map[string]bool{
		"README.md": false, "tests/test_a.py": false, "src/a.py": true, "target/app.py": false,
	} {
		if got := trackedSource(filepath.Join(root, path), root); got != want {
			t.Errorf("%s: %v", path, got)
		}
	}
}

func TestClojureWarningsGoToStderrBeforeWrote(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "left.clj", "(ns left)\n(defn broken [x]\n")
	code, out, errs := runDry("--root", root)
	want := "dryer: left.clj:2: unterminated list\nWrote " + filepath.Join(discover.Resolve(root), ".metrics", "dry.edn") + "\n"
	if code != 0 || out != "No duplicate candidates found.\n" || errs != want {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errs)
	}
}
