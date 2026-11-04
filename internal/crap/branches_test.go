package crap

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/branch"
)

// copyInto copies the tree src into dst.
func copyInto(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o777)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o666)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o666); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func stubGeneratedAt(t *testing.T) {
	old := branch.GeneratedAt
	branch.GeneratedAt = func() string { return "2026-11-03T12:00:00-03:00" }
	t.Cleanup(func() { branch.GeneratedAt = old })
}

// section returns stdout from the "Untested branches" header on.
func section(t *testing.T, stdout string) string {
	t.Helper()
	i := strings.Index(stdout, "\nUntested branches")
	if i < 0 {
		t.Fatalf("no branch section in\n%s", stdout)
	}
	return stdout[i+1:]
}

// TestCrapBranchesPythonEndToEnd runs coverage.py --branch the way
// `coderefinery test` does (its coverage.json under .metrics/tests/python),
// then crap --branches on the project.
func TestCrapBranchesPythonEndToEnd(t *testing.T) {
	if err := exec.Command("python3", "-c", "import coverage, pytest").Run(); err != nil {
		t.Skip("python3 with coverage and pytest is not available")
	}
	stubGeneratedAt(t)
	root := scratchDir(t)
	copyInto(t, "testdata/branches_python", root)
	data := filepath.Join(root, ".metrics", "tests", "python", "coverage")
	if err := os.MkdirAll(data, 0o777); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-m", "coverage", "run", "--branch", "--data-file=" + filepath.Join(data, ".coverage"), "-m", "pytest", "-q"},
		{"-m", "coverage", "json", "--data-file=" + filepath.Join(data, ".coverage"), "-o", filepath.Join(data, "coverage.json")},
	} {
		cmd := exec.Command("python3", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("python3 %v: %v\n%s", args, err, out)
		}
	}

	// crap --branches runs the coverage tools as usual, then adds the section
	code, out, errs := runCrap("--root", root, "--branches")
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	wantSection := "Untested branches (python branch)\n" +
		"billing/pricing.py:7 elif tier == \"silver\" · true arm never taken\n" +
		"billing/pricing.py:16 if total <= 0 · true arm never taken\n" +
		"billing/pricing.py:21 for item in items · body never entered\n" +
		"billing/pricing.py:30 case \"CA\" | \"MX\" · case 2 never matched\n"
	if got := section(t, out); got != wantSection {
		t.Fatalf("section\n%s\nwant\n%s", got, wantSection)
	}
	wantEDN := "{:version 1 :generated-at \"2026-11-03T12:00:00-03:00\" :branches [\n" +
		"  {:file \"billing/pricing.py\" :line 7 :decision \"elif tier == \\\"silver\\\"\" :arm \"true\" :granularity \"branch\" :text \"true arm never taken\"}\n" +
		"  {:file \"billing/pricing.py\" :line 16 :decision \"if total <= 0\" :arm \"true\" :granularity \"branch\" :text \"true arm never taken\"}\n" +
		"  {:file \"billing/pricing.py\" :line 21 :decision \"for item in items\" :arm \"body\" :granularity \"branch\" :text \"body never entered\"}\n" +
		"  {:file \"billing/pricing.py\" :line 30 :decision \"case \\\"CA\\\" | \\\"MX\\\"\" :arm \"case 2\" :granularity \"branch\" :text \"case 2 never matched\"}\n" +
		"]}\n"
	metrics := filepath.Join(root, ".metrics")
	if got := readFile(t, filepath.Join(metrics, "branches.edn")); got != wantEDN {
		t.Fatalf("branches.edn\n%s", got)
	}
	resolved, _ := filepath.EvalSymlinks(metrics)
	if !strings.HasSuffix(errs, "Wrote "+filepath.Join(resolved, "branches.edn")+"\n") {
		t.Fatalf("stderr does not end with the branches.edn line:\n%s", errs)
	}

	// the CRAP report and crap.edn do not change by a byte
	_, plain, _ := runCrap("--root", root, "--use-existing-coverage")
	plainEDN := readFile(t, filepath.Join(metrics, "crap.edn"))
	_, withBranches, _ := runCrap("--root", root, "--use-existing-coverage", "--branches")
	if withBranches != plain+"\n"+wantSection {
		t.Fatalf("--branches changed the report:\n%s\nplain:\n%s", withBranches, plain)
	}
	if got := readFile(t, filepath.Join(metrics, "crap.edn")); got != plainEDN {
		t.Fatalf("crap.edn changed:\n%s\nwant\n%s", got, plainEDN)
	}
	// smoke: a real report with the scored functions came first
	if !strings.HasPrefix(plain, "CRAP Report\n") || !strings.Contains(plain, "tier_rate") || !strings.Contains(plainEDN, ":name \"price\"") {
		t.Fatalf("unexpected report:\n%s", plain)
	}
}

// TestCrapBranchesGoBlocks: Go data is block granularity, from crap's own
// coverprofile; it is never called a branch.
func TestCrapBranchesGoBlocks(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	stubGeneratedAt(t)
	root := scratchDir(t)
	copyInto(t, "testdata/branches_go", root)
	code, out, errs := runCrap("--root", root, "--branches")
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errs)
	}
	want := "Untested branches (go block)\n" +
		"calc.go:9 if v > hi · block 10-11 never executed\n" +
		"calc.go:20 case n == 0 · block 21 never executed\n" +
		"calc.go:30 for _, v := range values · block 31-32 never executed\n"
	if got := section(t, out); got != want {
		t.Fatalf("section\n%s\nwant\n%s", got, want)
	}
	edn := readFile(t, filepath.Join(root, ".metrics", "branches.edn"))
	if !strings.Contains(edn, `{:file "calc.go" :line 9 :decision "if v > hi" :arm "true" :granularity "block" :text "block 10-11 never executed"}`) ||
		strings.Contains(edn, `"branch"`) || strings.Count(edn, ":granularity \"block\"") != 3 {
		t.Fatalf("branches.edn\n%s", edn)
	}
}

// sampleRoot builds a project from checked-in sources and sample reports.
func sampleRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := scratchDir(t)
	for dst, src := range files {
		copyFile(t, src, filepath.Join(root, dst))
	}
	return root
}

const samples = "../coverage/testdata/branches/"

// TestCrapBranchesFromSampleReports covers the languages whose toolchain is
// not installed here (JaCoCo, cargo-llvm-cov, Cloverage) and the TS report
// variants, with --use-existing-coverage.
func TestCrapBranchesFromSampleReports(t *testing.T) {
	stubGeneratedAt(t)
	inventory := samples + "inventory.ts.txt"
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"typescript istanbul (AST-aware)", map[string]string{"src/inventory.ts": inventory,
			".metrics/tests/typescript/coverage/coverage-final.json": samples + "ast-coverage-final.json"},
			"Untested branches (typescript branch)\n" +
				"src/inventory.ts:55 item.tags?.length ? `${item.sku} [${item.tags.join(\",\")}]` :… · false arm never taken\n" +
				"src/inventory.ts:59 member ? price * 0.9 : price · false arm never taken\n" +
				"src/inventory.ts:65 case \"CA\" · case 2 never matched\n" +
				"src/inventory.ts:66 case \"MX\" · case 3 never matched\n"},
		{"typescript v8 ranges", map[string]string{"src/inventory.ts": inventory,
			".metrics/tests/typescript/coverage/coverage-final.json": samples + "v8-coverage-final.json"},
			"Untested branches (typescript branch-count)\n" +
				"src/inventory.ts:55 item.tags?.length ? `${item.sku} [${item.tags.join(\",\")}]` :… · 1 of 1 branch missed\n" +
				"src/inventory.ts:59 member ? price * 0.9 : price · 1 of 1 branch missed\n" +
				"src/inventory.ts:65 case \"CA\" · 1 of 1 branch missed\n" +
				"src/inventory.ts:66 case \"MX\" · 1 of 1 branch missed\n"},
		{"typescript crap LCOV BRDA, python without data", map[string]string{"src/inventory.ts": inventory,
			"target/coverage/typescript/lcov.info": samples + "lcov.info", "billing/pricing.py": "testdata/branches_python/billing/pricing.py"},
			"Untested branches (typescript branch-count)\n" +
				"src/inventory.ts:55 item.tags?.length ? `${item.sku} [${item.tags.join(\",\")}]` :… · 1 of 1 branch missed\n" +
				"src/inventory.ts:59 member ? price * 0.9 : price · 1 of 1 branch missed\n" +
				"src/inventory.ts:65 case \"CA\" · 1 of 1 branch missed\n" +
				"src/inventory.ts:66 case \"MX\" · 1 of 1 branch missed\n" +
				"No branch data: python (run coderefinery test)\n"},
		{"java jacoco", map[string]string{"src/main/java/demo/Board.java": "testdata/e2e_java/src/main/java/demo/Board.java",
			"target/site/jacoco/jacoco.xml": samples + "jacoco.xml"},
			"Untested branches (java branch-count)\n" +
				"src/main/java/demo/Board.java:26 if (x > 0 && x < cells.size()) · 1 of 4 branches missed\n" +
				"src/main/java/demo/Board.java:40 case 0 · 1 of 3 branches missed\n" +
				"src/main/java/demo/Board.java:42 cell > 10 ? 3 : 2 · 1 of 2 branches missed\n" +
				"src/main/java/demo/Board.java:52 value == 0 || value < 0 · 1 of 4 branches missed\n"},
		{"rust llvm-cov regions", map[string]string{"Cargo.toml": "testdata/e2e_rust/Cargo.toml", "src/lib.rs": "testdata/e2e_rust/src/lib.rs",
			".metrics/tests/rust/coverage/llvm-cov.json": samples + "llvm-cov-regions.json"},
			"Untested branches (rust region)\n" +
				"src/lib.rs:4 if x > 0 && x < 10 · region 4:24-6:6 never executed\n" +
				"src/lib.rs:8 1 => · region 8:14-8:15 never executed\n" +
				"src/lib.rs:19 if self.ok · region 19:31-19:36 never executed\n"},
		{"rust llvm-cov --branch", map[string]string{"Cargo.toml": "testdata/e2e_rust/Cargo.toml", "src/lib.rs": "testdata/e2e_rust/src/lib.rs",
			".metrics/tests/rust/coverage/llvm-cov.json": samples + "llvm-cov-branches.json"},
			"Untested branches (rust branch)\n" +
				"src/lib.rs:4 x > 0 && x < 10 · operand 1 (x > 0) never false\n" +
				"src/lib.rs:4 x > 0 && x < 10 · operand 2 (x < 10) never true\n" +
				"src/lib.rs:19 if self.ok · false arm never taken\n"},
		{"clojure cloverage", map[string]string{"src/demo/core.clj": samples + "clojure/core.clj",
			"target/coverage/demo/core.clj.html": samples + "clojure/core.clj.html"},
			"Untested branches (clojure form)\n" +
				"src/demo/core.clj:6 (when (neg? x) :negative))) · 2 of 4 forms not covered\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := sampleRoot(t, c.files)
			code, out, errs := runCrap("--root", root, "--use-existing-coverage", "--branches")
			if code != 0 {
				t.Fatalf("exit %d\n%s", code, errs)
			}
			if got := section(t, out); got != c.want {
				t.Fatalf("section\n%s\nwant\n%s", got, c.want)
			}
			edn := readFile(t, filepath.Join(root, ".metrics", "branches.edn"))
			if strings.Count(edn, "\n  {:file ") != strings.Count(c.want, " · ") {
				t.Fatalf("branches.edn rows do not match the section:\n%s", edn)
			}
		})
	}
}

func TestBranchesFlagIsHiddenAndExcludesNoCoverage(t *testing.T) {
	code, _, errs := runCrap("--no-coverage", "--branches")
	if code != 1 || !strings.HasPrefix(errs, "--no-coverage cannot be combined with --branches\n\n") || !strings.HasSuffix(errs, helpText()) {
		t.Fatalf("exit %d\n%s", code, errs)
	}
	code, out, _ := runCrap("--help")
	if code != 0 || out != helpText() || strings.Contains(out, "--branches") {
		t.Fatalf("help changed:\n%s", out)
	}
	if opts := ParseArgs([]string{"--branches", "--threshold", "5"}); !opts.Branches || opts.Help {
		t.Fatalf("opts %+v", opts)
	}
}
