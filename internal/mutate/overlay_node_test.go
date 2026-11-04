package mutate

// --fix-overlay on TS/JS (DEVIATIONS.md M9): test files and setup files are
// real files in the worker, so Vitest under jsdom can load them and Jest
// finds them.

import (
	"os"
	"path/filepath"
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
