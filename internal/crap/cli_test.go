package crap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyTree copies testdata/<name> into a fresh, symlink-resolved temp dir
// (macOS /var -> /private/var, like upstream's Path.resolve()).
func copyTree(t *testing.T, name string) string {
	t.Helper()
	dst, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("testdata", name)
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func runCrap(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

const e2eReport = `CRAP Report
===========
Languages: python

Function                       Namespace                             CC    Cov%     CRAP
----------------------------------------------------------------------------------------
brew                           demo.café                              3   0.0%     12.0
choose                         demo.box                               4  66.7%      4.6
open                           demo.box.Box                           2  50.0%      2.5
`

const e2eEDN = `{:entries [
  {:name "brew", :namespace "demo.café", :complexity 3, :coverage 0.0, :crap 12.0}
  {:name "choose", :namespace "demo.box", :complexity 4, :coverage 66.6667, :crap 4.5926}
  {:name "open", :namespace "demo.box.Box", :complexity 2, :coverage 50.0, :crap 2.5}
]}
`

// Smoke: the whole pipeline against output recorded from upstream crapper
// on the same tree (testdata/e2e).
func TestSmokeExistingCoverageMatchesUpstream(t *testing.T) {
	root := copyTree(t, "e2e")
	code, stdout, stderr := runCrap("--root", root, "--use-existing-coverage", "--threshold", "3")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr %s", code, stderr)
	}
	if stdout != e2eReport {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, e2eReport)
	}
	metrics := filepath.Join(root, ".metrics", "crap.edn")
	wantErr := "Wrote " + metrics + "\nCRAP threshold exceeded: 12.0 > 3.0\n"
	if stderr != wantErr {
		t.Fatalf("stderr %q, want %q", stderr, wantErr)
	}
	edn, err := os.ReadFile(metrics)
	if err != nil || string(edn) != e2eEDN {
		t.Fatalf("crap.edn:\n%s\nwant:\n%s (%v)", edn, e2eEDN, err)
	}
}

func TestNoCoverageScoresComplexityOnly(t *testing.T) {
	root := copyTree(t, "e2e")
	code, stdout, _ := runCrap("--root", root, "--no-coverage", "--threshold", "1")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "choose                         demo.box                               4   N/A       N/A\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	edn, _ := os.ReadFile(filepath.Join(root, ".metrics", "crap.edn"))
	if !strings.Contains(string(edn), `{:name "choose", :namespace "demo.box", :complexity 4, :coverage nil, :crap nil}`) {
		t.Fatalf("edn:\n%s", edn)
	}
}

func TestFailingCoverageCommandIgnoresStaleReports(t *testing.T) {
	root := copyTree(t, "e2e")
	code, stdout, stderr := runCrap("--root", root, "--coverage-command", "exit 3", "src/demo/box.py")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "choose                         demo.box                               4   0.0%     20.0\n") {
		t.Fatalf("stale coverage was read:\n%s", stdout)
	}
	wantStart := "+ (" + root + ") exit 3\nCoverage command exited 3. Reports already on disk will not be read.\n"
	if !strings.HasPrefix(stderr, wantStart) {
		t.Fatalf("stderr %q", stderr)
	}
}

func TestSelectionPathsFiltersAndNoFiles(t *testing.T) {
	root := copyTree(t, "e2e")
	_, stdout, _ := runCrap("--root", root, "--no-coverage", "src/demo", "box")
	if strings.Contains(stdout, "brew") || !strings.Contains(stdout, "choose") {
		t.Fatalf("filter 'box' not applied:\n%s", stdout)
	}
	root = copyTree(t, "e2e")
	code, stdout, stderr := runCrap("--root", root, "--no-coverage", "notes.md")
	if code != 0 || stdout != "No source files to analyze.\n" || stderr != "" {
		t.Fatalf("no files: %d %q %q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".metrics")); !os.IsNotExist(err) {
		t.Fatal("no metrics must be written when nothing is analyzed")
	}
}

func TestParseArgsErrorsAndHelp(t *testing.T) {
	code, stdout, _ := runCrap("--threshold", "--help")
	if code != 0 || !strings.Contains(stdout, "uml-viewer") || !strings.Contains(stdout, ".clj-kondo") || !strings.Contains(stdout, ".test.cts") {
		t.Fatalf("help: %d", code)
	}
	cases := map[string][]string{
		"Unknown option: --not-an-option":                          {"--not-an-option"},
		"--root requires a value":                                  {"--root"},
		"--threshold requires a number":                            {"--threshold", "nope"},
		"--no-coverage cannot be combined with --coverage-command": {"--no-coverage", "--coverage-command", "true"},
	}
	for msg, args := range cases {
		code, stdout, stderr := runCrap(args...)
		if code != 1 || stdout != "" || !strings.HasPrefix(stderr, msg+"\n\nUsage: crapper") {
			t.Errorf("%v: exit %d stderr %.60q", args, code, stderr)
		}
	}
	for _, args := range [][]string{{"ok", "--threshold", ""}, {"--threshold", "-1"}} {
		if o := ParseArgs(args); o.ExitCode != 1 || !strings.HasPrefix(o.Message, "--threshold requires a value") {
			t.Errorf("%q: %q", args, o.Message[:40])
		}
	}
	o := ParseArgs([]string{"-s", "src", "--source-root", "lib", "--threshold", " 1_0 ", "x"})
	if strings.Join(o.SourceRoots, ",") != "src,lib" || *o.Threshold != 10 || strings.Join(o.Positionals, ",") != "x" {
		t.Fatalf("parsed %+v", o)
	}
}
