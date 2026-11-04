package mutate

// --fix-overlay on TS/JS (DEVIATIONS.md M9): test files and setup files are
// real files in the worker, so Vitest under jsdom can load them and Jest
// finds them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestFixOverlayCopiesNodeSourcesButLinksPackagesAndCaches(t *testing.T) {
	root := tempRoot(t)
	outside := tempRoot(t)
	writeFile(t, filepath.Join(root, "package.json"), "{}\n")
	original := writeFile(t, filepath.Join(root, "src", "grade.ts"), "export const x = 1;\n")
	writeFile(t, filepath.Join(root, "src", "title.tsx"), "export const t = 1;\n")
	writeFile(t, filepath.Join(root, "src", "logo.svg"), "<svg/>")
	writeFile(t, filepath.Join(root, "tests", "title.test.tsx"), "import '../src/title';\n")
	writeFile(t, filepath.Join(root, "tests", "deep", "setup.js"), "globalThis.x = 1;\n")
	writeFile(t, filepath.Join(root, "node_modules", "react", "index.js"), "module.exports = {};\n")
	writeFile(t, filepath.Join(root, "packages", "ui", "node_modules", "dep", "index.js"), "")
	writeFile(t, filepath.Join(root, "packages", "ui", "button.jsx"), "export const b = 1;\n")
	writeFile(t, filepath.Join(root, ".next", "server", "page.js"), "")
	writeFile(t, filepath.Join(root, "dist", "grade.js"), "")
	writeFile(t, filepath.Join(outside, "shared", "util.ts"), "export const u = 1;\n")
	if err := os.Symlink(filepath.Join(outside, "shared"), filepath.Join(root, "shared")); err != nil {
		t.Fatal(err)
	}
	created := func(copied bool) string {
		t.Helper()
		base, err := newRunDir(root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { deleteTree(base) })
		workers, err := createWorkers(base, root, "src/grade.ts", []byte(readFile(t, original)), 1, nil)
		if err != nil || len(workers) != 1 {
			t.Fatalf("workers %v %v", workers, err)
		}
		if copied {
			if err := copySources(workers[0], root); err != nil {
				t.Fatal(err)
			}
		}
		return workers[0]
	}
	kinds := func(w string) string {
		var out []string
		for _, rel := range []string{"tests", "tests/title.test.tsx", "tests/deep/setup.js", "src/title.tsx", "src/logo.svg",
			"node_modules", "packages/ui/button.jsx", "packages/ui/node_modules", ".next", "shared", "dist"} {
			info, err := os.Lstat(filepath.Join(w, rel))
			switch {
			case err != nil:
				out = append(out, rel+"=missing")
			case viaLink(w, rel):
				out = append(out, rel+"=link")
			case info.IsDir():
				out = append(out, rel+"=dir")
			default:
				out = append(out, rel+"=file")
			}
		}
		return strings.Join(out, " ")
	}

	if got, want := kinds(created(false)), "tests=link tests/title.test.tsx=link tests/deep/setup.js=link src/title.tsx=link "+
		"src/logo.svg=link node_modules=link packages/ui/button.jsx=link packages/ui/node_modules=link .next=link shared=link dist=missing"; got != want {
		t.Fatalf("upstream overlay:\n got %s\nwant %s", got, want)
	}
	w := created(true)
	if got, want := kinds(w), "tests=dir tests/title.test.tsx=file tests/deep/setup.js=file src/title.tsx=file "+
		"src/logo.svg=link node_modules=link packages/ui/button.jsx=file packages/ui/node_modules=link .next=link shared=link dist=missing"; got != want {
		t.Fatalf("--fix-overlay:\n got %s\nwant %s", got, want)
	}
	if readFile(t, filepath.Join(w, "tests", "deep", "setup.js")) != "globalThis.x = 1;\n" ||
		readFile(t, filepath.Join(w, "src", "grade.ts")) != "export const x = 1;\n" {
		t.Fatal("copies hold the real bytes; the mutated file stays the private copy")
	}
	if resolved, _ := filepath.EvalSymlinks(filepath.Join(w, "tests", "title.test.tsx")); resolved != filepath.Join(w, "tests", "title.test.tsx") {
		t.Fatalf("a test file's real path stays inside the worker, got %s", resolved)
	}
	writeFile(t, filepath.Join(w, "tests", "title.test.tsx"), "changed")
	if readFile(t, filepath.Join(root, "tests", "title.test.tsx")) != "import '../src/title';\n" {
		t.Fatal("writing a worker copy leaves the project file alone")
	}
}

// viaLink: rel or one of its parent directories is a symlink.
func viaLink(worker, rel string) bool {
	for p := rel; p != "."; p = filepath.Dir(p) {
		if isSymlink(filepath.Join(worker, p)) {
			return true
		}
	}
	return false
}

// nodeProject copies a testdata project and links node_modules to a fixture's.
func nodeProject(t *testing.T, name, fixture string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("runs the project's tests on mutants")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("no npm on PATH")
	}
	modules, err := filepath.Abs(filepath.Join("..", "..", "fixtures", fixture, "node_modules"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(modules); err != nil {
		t.Skipf("fixtures/%s/node_modules missing: run fixtures/setup-tests.sh", fixture)
	}
	root := copyTree(t, filepath.Join("testdata", name))
	// an asdf shim finds no .tool-versions outside the repo (the go wrapper's
	// HOME is .local/home): pin the newest installed nodejs, as pinAsdfNode does
	if probe := exec.Command("node", "--version"); os.Getenv("ASDF_NODEJS_VERSION") == "" {
		if probe.Dir = root; probe.Run() != nil {
			shim, _ := exec.LookPath("node")
			versions, _ := filepath.Glob(filepath.Join(filepath.Dir(filepath.Dir(shim)), "installs", "nodejs", "*"))
			if len(versions) == 0 {
				t.Skip("node does not start outside the repo")
			}
			sort.Strings(versions)
			t.Setenv("ASDF_NODEJS_VERSION", filepath.Base(versions[len(versions)-1]))
		}
	}
	if err := os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFixOverlayLetsVitestJsdomLoadLinkedTestFiles(t *testing.T) {
	root := nodeProject(t, "overlay-jsdom", "react")
	run := func(flags ...string) cliResult {
		args := append([]string{"--root", root, "--no-coverage", "--mutate-all", "--min-timeout", "300"}, flags...)
		return cli(append(args, "src/Badge.tsx")...)
	}
	const sum, zero, capped = "src/Badge.tsx:7 + -> -", "src/Badge.tsx:7 0 -> 1", "src/Badge.tsx:14 > -> >="

	// upstream overlay: tests/setup.ts and tests/Title.test.tsx are links whose
	// real path is outside the worker; under jsdom Vite refuses them, every run
	// exits 1, and even the survivor is "killed"
	plain := run()
	if plain.code != 0 || !strings.HasPrefix(plain.stdout, "KILLED    "+sum+"\nKILLED    "+zero+"\nKILLED    "+capped+"\n") ||
		!strings.Contains(plain.stdout, "Total                    3         0          0      3  100.0%\n") {
		t.Fatalf("upstream overlay: exit %d\n%s%s", plain.code, plain.stdout, plain.stderr)
	}
	fixed := run("--fix-overlay")
	if fixed.code != 3 || !strings.HasPrefix(fixed.stdout, "KILLED    "+sum+"\nKILLED    "+zero+"\nSURVIVED  "+capped+"\n") ||
		!strings.Contains(fixed.stdout, "Total                    2         1          0      3   66.7%\n") {
		t.Fatalf("--fix-overlay: exit %d\n%s%s", fixed.code, fixed.stdout, fixed.stderr)
	}
	// --attribute implies the fix: kills are credited to the test that failed
	attributed := run("--attribute")
	if attributed.code != 3 || !strings.HasPrefix(attributed.stdout, "KILLED    "+sum+" · by tests/Badge.test.tsx::adds the counts (+1)\n"+
		"KILLED    "+zero+" · by tests/Badge.test.tsx::adds the counts\nSURVIVED  "+capped+"\n") {
		t.Fatalf("--attribute: exit %d\n%s%s", attributed.code, attributed.stdout, attributed.stderr)
	}
	if readFile(t, filepath.Join(root, "src", "Badge.tsx")) != readFile(t, filepath.Join("testdata", "overlay-jsdom", "src", "Badge.tsx")) {
		t.Fatal("the project tree is left unchanged")
	}
}

func TestFixOverlayLetsJestFindLinkedTestFiles(t *testing.T) {
	root := nodeProject(t, "overlay-jest", "js-jest-tests")
	run := func(flags ...string) cliResult {
		args := append([]string{"--root", root, "--no-coverage", "--mutate-all", "--min-timeout", "300"}, flags...)
		return cli(append(args, "lib/grade.js")...)
	}
	const boundary = "lib/grade.js:2 >= -> >"

	// upstream overlay: __tests__/grade.test.js imports "lib/grade", not a
	// relative path, so it stays a link; Jest's crawler skips links and the
	// test that kills the mutant never runs
	plain := run()
	if plain.code != 3 || !strings.HasPrefix(plain.stdout, "SURVIVED  "+boundary+"\n") {
		t.Fatalf("upstream overlay: exit %d\n%s%s", plain.code, plain.stdout, plain.stderr)
	}
	fixed := run("--fix-overlay")
	if fixed.code != 0 || !strings.HasPrefix(fixed.stdout, "KILLED    "+boundary+"\n") ||
		!strings.Contains(fixed.stdout, "Total                    1         0          0      1  100.0%\n") {
		t.Fatalf("--fix-overlay: exit %d\n%s%s", fixed.code, fixed.stdout, fixed.stderr)
	}
	attributed := run("--attribute")
	if attributed.code != 0 || !strings.HasPrefix(attributed.stdout, "KILLED    "+boundary+" · by __tests__/grade.test.js::50 passes\n") {
		t.Fatalf("--attribute: exit %d\n%s%s", attributed.code, attributed.stdout, attributed.stderr)
	}
}
