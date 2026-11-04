// Package discover is upstream crapper/discover.py (also imported by
// mutator): extension -> language, test-file rules, the source walk, path
// resolution with Python pathlib semantics, and `git status` changed files.
package discover

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Extensions maps Python Path.suffix (case-sensitive) to a crapper language.
var Extensions = map[string]string{
	".clj": "clojure", ".cljc": "clojure", ".cljs": "clojure", ".bb": "clojure",
	".java": "java",
	".go":   "go",
	".ts":   "typescript", ".tsx": "typescript", ".mts": "typescript", ".cts": "typescript",
	".js": "typescript", ".jsx": "typescript", ".mjs": "typescript", ".cjs": "typescript",
	".rs": "rust",
	".py": "python",
}

// SkipDirs and TestDirs are pruned during the walk (exact names).
var SkipDirs = []string{".git", ".hg", ".svn", ".idea", ".venv", "venv", "node_modules", "vendor", "target",
	"dist", "build", "out", "coverage", ".metrics", "testdata", "__pycache__", ".clj-kondo"}

// TestDirs mark test files when they appear as any path component.
var TestDirs = []string{"test", "tests", "spec", "specs", "__tests__"}

const GoTestSuffix = "_test.go"

var ClojureTestSuffixes = []string{"_test.clj", "_test.cljc", "_test.cljs", "_test.bb"}

var JSTestSuffixes = []string{".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx", ".test.mts", ".spec.mts",
	".test.cts", ".spec.cts", ".test.js", ".spec.js", ".test.jsx", ".spec.jsx", ".test.mjs", ".spec.mjs",
	".test.cjs", ".spec.cjs"}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Suffix is Python's PurePath.suffix of the final component: the text from
// the last "." unless that dot is the first character ("" for ".py").
func Suffix(path string) string {
	name := filepath.Base(path)
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || i == len(name)-1 {
		return ""
	}
	return name[i:]
}

// LanguageOf is discover.language_of.
func LanguageOf(path string) string {
	name := filepath.Base(path)
	if strings.HasSuffix(name, ".d.ts") {
		return ""
	}
	return Extensions[Suffix(path)]
}

// Parts is Python PurePosixPath.parts: "/" first for absolute paths.
func Parts(path string) []string {
	var parts []string
	if strings.HasPrefix(path, "/") {
		parts = append(parts, "/")
	}
	for _, p := range strings.Split(path, "/") {
		if p != "" && p != "." {
			parts = append(parts, p)
		}
	}
	return parts
}

// IsTestFile is discover.is_test_file.
func IsTestFile(path string) bool {
	name := filepath.Base(path)
	if strings.HasSuffix(name, GoTestSuffix) {
		return true
	}
	for _, s := range ClojureTestSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	for _, s := range JSTestSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	if name == "conftest.py" || strings.HasSuffix(name, "_test.py") ||
		strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py") {
		return true
	}
	for _, part := range Parts(path) {
		if contains(TestDirs, part) {
			return true
		}
	}
	return false
}

// Resolve is Path.resolve(strict=False): absolute, symlinks resolved in the
// longest existing prefix, the rest appended.
func Resolve(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r
	}
	dir, rest := abs, ""
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
	}
}

// Exists, IsFile, IsDir follow symlinks like pathlib.
func Exists(path string) bool { _, err := os.Stat(path); return err == nil }

func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// walk is the os.walk-based source walk every tool shares: nothing for a
// missing root, the root itself when it is not a directory, otherwise every
// file under it for which keep is true, pruning SkipDirs (and TestDirs when
// pruneTests) below the root and never descending into symlinked directories.
func walk(root string, keep func(path string) bool, pruneTests bool) []string {
	if !Exists(root) {
		return nil
	}
	if !IsDir(root) {
		return []string{root}
	}
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && (contains(SkipDirs, d.Name()) || pruneTests && contains(TestDirs, d.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			// os.walk lists symlinked dirs under dirnames (not followed) and
			// symlinked files under filenames.
			if IsDir(path) {
				return nil
			}
		}
		if keep(path) {
			found = append(found, path)
		}
		return nil
	})
	return found
}

// IsSource is crapper's walk filter: a known language and not a test file.
func IsSource(path string) bool { return LanguageOf(path) != "" && !IsTestFile(path) }

// IterSourceFiles is discover.iter_source_files: walk each root, resolve,
// dedup, sort by posix string.
func IterSourceFiles(roots []string) []string { return IterFiles(roots, IsSource) }

// IterFiles walks each root with walk(root, keep, true), then resolves, dedups and
// sorts by posix string (iter_source_files with another file filter).
func IterFiles(roots []string, keep func(path string) bool) []string {
	return iterFiles(roots, keep, true)
}

func iterFiles(roots []string, keep func(path string) bool, pruneTests bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, root := range roots {
		for _, p := range walk(root, keep, pruneTests) {
			r := Resolve(p)
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	sort.Strings(out)
	return out
}

// GitError is git's failure: Code is its exit status.
type GitError struct {
	Code    int
	Message string
}

func (e *GitError) Error() string { return e.Message }

func git(root string, args ...string) (string, *GitError, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = "git status failed"
			}
			return "", &GitError{Code: ee.ExitCode(), Message: msg}, nil
		}
		return "", nil, err
	}
	return string(out), nil, nil
}

// ChangedFiles is cli._changed_files: added/modified regular files from
// `git status -z`, resolved from the repository top and limited to root.
func ChangedFiles(root string) ([]string, error) {
	top, gerr, err := git(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	if gerr != nil {
		return nil, gerr
	}
	status, gerr, err := git(root, "status", "--porcelain", "-z", "--no-renames", "--untracked-files=all", "--", ".")
	if err != nil {
		return nil, err
	}
	if gerr != nil {
		return nil, gerr
	}
	topDir := strings.TrimSpace(top)
	limit := Resolve(root)
	var found []string
	for _, entry := range strings.Split(status, "\x00") {
		if len(entry) < 4 {
			continue
		}
		path := Resolve(filepath.Join(topDir, entry[3:]))
		if IsFile(path) && IsRelativeTo(path, limit) {
			found = append(found, path)
		}
	}
	return found, nil
}

// IsRelativeTo is PurePath.is_relative_to.
func IsRelativeTo(path, base string) bool {
	return path == base || strings.HasPrefix(path, strings.TrimSuffix(base, "/")+"/")
}

// RelativeTo is PurePath.relative_to as a posix string; ok=false when path
// is not under base.
func RelativeTo(path, base string) (string, bool) {
	if path == base {
		return ".", true
	}
	prefix := strings.TrimSuffix(base, "/") + "/"
	if rel, ok := strings.CutPrefix(path, prefix); ok {
		return rel, true
	}
	return "", false
}
