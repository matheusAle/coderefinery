package mutate

// --fix-overlay (DEVIATIONS.md M8, self-audit C1): skip-listed directories
// that are packages of the mutated language stay in the worker overlay.

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixOverlayLinksPackageDirsButNotBuildOutput(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/m\n")
	original := writeFile(t, filepath.Join(root, "internal", "report", "report.go"), "package report\n")
	writeFile(t, filepath.Join(root, "internal", "coverage", "coverage.go"), "package coverage\n")
	writeFile(t, filepath.Join(root, "internal", "build", "notes.txt"), "not a package")
	writeFile(t, filepath.Join(root, "internal", "out", "gen", "gen.go"), "package gen\n") // sources only below
	writeFile(t, filepath.Join(root, "internal", "dist", "tool.py"), "x = 1\n")            // another language
	writeFile(t, filepath.Join(root, "internal", ".git", "x.go"), "package x\n")           // not a package name
	writeFile(t, filepath.Join(root, "coverage", "pct.go"), "package pct\n")               // a top-level package
	writeFile(t, filepath.Join(root, "target", "stray.go"), "package target\n")            // the worker home
	created := func(keep func(dir, name string) bool) string {
		t.Helper()
		base, err := newRunDir(root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { deleteTree(base) })
		workers, err := createWorkers(base, root, "internal/report/report.go", []byte(readFile(t, original)), 1, keep)
		if err != nil || len(workers) != 1 {
			t.Fatalf("workers %v %v", workers, err)
		}
		return workers[0]
	}
	linked := func(w string) []string {
		var names []string
		for _, rel := range []string{"internal/coverage", "internal/build", "internal/out", "internal/dist", "internal/.git", "coverage", "target"} {
			if isSymlink(filepath.Join(w, rel)) {
				names = append(names, rel)
			}
		}
		return names
	}

	if got := linked(created(nil)); len(got) != 0 {
		t.Fatalf("upstream overlay links no skip-listed name, got %v", got)
	}
	w := created(packageDirs(root, "go"))
	if got := strings.Join(linked(w), " "); got != "internal/coverage coverage" {
		t.Fatalf("--fix-overlay links %q, want the two Go packages named coverage", got)
	}
	if readFile(t, filepath.Join(w, "internal", "coverage", "coverage.go")) != "package coverage\n" {
		t.Fatal("the link reaches the real package")
	}
	if isSymlink(filepath.Join(w, "internal", "report", "report.go")) || lexists(filepath.Join(w, "target")) {
		t.Fatal("the mutated file stays private; root target/ (the worker home) is never linked, even holding .go files")
	}
	if packageDirs(root, "typescript") != nil || packageDirs(root, "") != nil {
		t.Fatal("TS/JS projects keep upstream's overlay: dist/ and coverage/ hold .js output there")
	}
	if py := packageDirs(root, "python"); py(filepath.Join(root, "internal"), "coverage") || !py(filepath.Join(root, "internal"), "dist") {
		t.Fatal("only directories holding the mutated file's language are kept")
	}
}

// copyTree copies a testdata module to a fresh temp root.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := tempRoot(t)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o777)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o666)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestFixOverlayLetsMutantsBesideAPackageNamedCoverageRun(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test on mutants")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}
	run := func(flags ...string) cliResult {
		root := copyTree(t, filepath.Join("testdata", "overlay-coverage"))
		args := append([]string{"--root", root, "--no-coverage", "--min-timeout", "300"}, flags...)
		return cli(append(args, "internal/report/report.go")...)
	}
	const grade, missing = "internal/report/report.go:9 >= -> >", "internal/report/report.go:17 - -> +"

	// upstream overlay: internal/coverage is hidden, nothing builds, all "killed"
	plain := run()
	if plain.code != 0 || !strings.HasPrefix(plain.stdout, "KILLED    "+grade+"\nKILLED    "+missing+"\n") {
		t.Fatalf("upstream overlay: exit %d\n%s%s", plain.code, plain.stdout, plain.stderr)
	}
	fixed := run("--fix-overlay")
	if fixed.code != 3 || !strings.HasPrefix(fixed.stdout, "SURVIVED  "+grade+"\nKILLED    "+missing+"\n") ||
		!strings.Contains(fixed.stdout, "Total                                              1         1          0      2   50.0%\n") {
		t.Fatalf("--fix-overlay: exit %d\n%s%s", fixed.code, fixed.stdout, fixed.stderr)
	}
	// --attribute implies the fix: the kill goes to the test, not to <build>
	attributed := run("--attribute")
	if attributed.code != 3 || !strings.HasPrefix(attributed.stdout, "SURVIVED  "+grade+"\nKILLED    "+missing+" · by example.com/overlay/internal/report::TestMissing\n") {
		t.Fatalf("--attribute: exit %d\n%s%s", attributed.code, attributed.stdout, attributed.stderr)
	}
}

func TestFixOverlayIsParsedAndHidden(t *testing.T) {
	if o := ParseArgs([]string{"--fix-overlay"}); !o.FixOverlay || o.ExitCode != 0 {
		t.Fatalf("--fix-overlay: %+v", o)
	}
	if o := ParseArgs(nil); o.FixOverlay {
		t.Fatal("off by default")
	}
	if strings.Contains(Help, "fix-overlay") || strings.Contains(Help, "min-timeout") {
		t.Fatal("hidden flags stay out of the upstream --help")
	}
}
