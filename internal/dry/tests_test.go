package dry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDryTestsReportsTestFilePairs: --tests adds the duplicate pair of test
// functions; the default run (and its dry.edn) is what upstream dryer
// prints for this project (checked against upstream/dryer once).
func TestDryTestsReportsTestFilePairs(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"src/calc.py", "tests/test_calc.py"} {
		data, err := os.ReadFile(filepath.Join("testdata", "tests_dup", rel))
		if err != nil {
			t.Fatal(err)
		}
		_ = os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
		if err := os.WriteFile(filepath.Join(root, rel), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sourcePair := "DUPLICATE score=1.00\n  src/calc.py:1-6\n  src/calc.py:9-14\n"
	testPair := "DUPLICATE score=1.00\n  tests/test_calc.py:4-8\n  tests/test_calc.py:11-15\n"

	code, out, errs := runDry("--root", root)
	if code != 0 || out != sourcePair {
		t.Fatalf("default: exit %d\n%s%s", code, out, errs)
	}
	plainEDN, _ := os.ReadFile(filepath.Join(root, ".metrics", "dry.edn"))
	if strings.Contains(string(plainEDN), "tests/") {
		t.Fatalf("default dry.edn names a test file:\n%s", plainEDN)
	}

	code, out, errs = runDry("--root", root, "--tests")
	if code != 0 || out != sourcePair+"\n"+testPair {
		t.Fatalf("--tests: exit %d\n%s%s", code, out, errs)
	}
	if errs != "Wrote "+filepath.Join(root, ".metrics", "dry.edn")+"\n" {
		t.Fatalf("stderr %q", errs)
	}
	edn, _ := os.ReadFile(filepath.Join(root, ".metrics", "dry.edn"))
	wantRow := ` :left {:file "tests/test_calc.py", :start-line 4, :end-line 8}`
	if !strings.Contains(string(edn), wantRow) || !strings.HasPrefix(string(edn), strings.TrimSuffix(string(plainEDN), "]}\n")) {
		t.Fatalf("--tests dry.edn:\n%s", edn)
	}

	// a named test directory is walked with its test files only under --tests
	if _, out, _ := runDry("--root", root, "--tests", "tests"); out != testPair {
		t.Fatalf("named test dir:\n%s", out)
	}
	if _, out, _ := runDry("--root", root, "tests"); out != "No source files to analyze.\n" {
		t.Fatalf("named test dir without --tests (upstream prints nothing found):\n%s", out)
	}
	if strings.Contains(Help, "--tests") {
		t.Fatal("--tests is listed in --help")
	}
	if opts := ParseArgs([]string{"--tests"}); !opts.Tests || opts.Help {
		t.Fatalf("opts %+v", opts)
	}
}

func TestChangedTestFilesAreTrackedUnderTests(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	for path, want := range map[string]bool{
		"tests/test_a.py": true, "src/a.py": true, "src/a_test.go": true, "target/a.py": false, "README.md": false,
	} {
		if got := trackedSource(filepath.Join(root, path), root, hasLanguage); got != want {
			t.Errorf("%s: %v", path, got)
		}
	}
}
