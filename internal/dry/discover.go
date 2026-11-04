package dry

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
)

// File selection: upstream dryer/discover.py and cli.select_files
// (spec-dryer §2). Dryer's language table differs from crapper's: .cljd is
// clojure and JavaScript is not supported. The walk, path resolution and
// git plumbing are shared with crapper (internal/discover).

var extensions = map[string]string{
	".clj": "clojure", ".cljc": "clojure", ".cljs": "clojure", ".cljd": "clojure", ".bb": "clojure",
	".java": "java",
	".go":   "go",
	".ts":   "typescript", ".tsx": "typescript", ".mts": "typescript", ".cts": "typescript",
	".rs": "rust",
	".py": "python",
}

var testSuffixes = []string{
	"_test.go",
	"_test.clj", "_test.cljc", "_test.cljs", "_test.cljd", "_test.bb",
	".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx", ".test.mts", ".spec.mts", ".test.cts", ".spec.cts",
	"_test.py",
}

// LanguageOf is dryer discover.language_of ("" for none).
func LanguageOf(path string) string {
	if strings.HasSuffix(filepath.Base(path), ".d.ts") {
		return ""
	}
	return extensions[discover.Suffix(path)]
}

// IsTestFile is dryer discover.is_test_file. Any component of the path as
// given that is a test directory counts, ancestors of the root included.
func IsTestFile(path string) bool {
	name := filepath.Base(path)
	for _, s := range testSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	if name == "conftest.py" || strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py") {
		return true
	}
	return anyIn(discover.Parts(path), discover.TestDirs)
}

func anyIn(parts, names []string) bool {
	for _, p := range parts {
		for _, n := range names {
			if p == n {
				return true
			}
		}
	}
	return false
}

func isSource(path string) bool { return LanguageOf(path) != "" && !IsTestFile(path) }

// IterSourceFiles is dryer discover.iter_source_files.
func IterSourceFiles(roots []string) []string { return discover.IterFiles(roots, isSource) }

func joinRoot(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

// trackedSource is cli._tracked_source(path, root) for a resolved path:
// its components below root must not meet SKIP_DIRS (test directories are
// caught by the test-file check on the absolute path). keep is isSource
// (hasLanguage under --tests).
func trackedSource(path, root string, keep func(string) bool) bool {
	rel, ok := discover.RelativeTo(path, discover.Resolve(root))
	if !ok || anyIn(discover.Parts(rel), discover.SkipDirs) {
		return false
	}
	return keep(path)
}

// SelectFiles is cli.select_files: sorted, deduplicated, resolved paths.
func SelectFiles(opts Options) ([]string, error) {
	root := discover.Resolve(opts.ProjectRoot)
	var existing, filters []string
	for _, arg := range opts.Positionals {
		candidate := joinRoot(root, arg)
		if discover.Exists(candidate) {
			existing = append(existing, discover.Resolve(candidate))
		} else {
			filters = append(filters, arg)
		}
	}
	iter, keep := IterSourceFiles, isSource
	if opts.Tests {
		iter, keep = iterWithTests, hasLanguage
	}
	var files []string
	switch {
	case opts.Changed:
		changed, err := discover.ChangedFiles(root)
		if err != nil {
			return nil, err
		}
		for _, p := range changed {
			if trackedSource(p, root, keep) {
				files = append(files, p)
			}
		}
	case len(opts.SourceRoots) > 0:
		var roots []string
		for _, p := range opts.SourceRoots {
			roots = append(roots, discover.Resolve(joinRoot(root, p)))
		}
		files = iter(roots)
	case len(existing) > 0:
		for _, p := range existing {
			if discover.IsDir(p) {
				files = append(files, iter([]string{p})...)
			} else if LanguageOf(p) != "" {
				files = append(files, p)
			}
		}
	default:
		files = iter([]string{root})
	}
	if len(filters) > 0 {
		var kept []string
		for _, p := range files {
			for _, f := range filters {
				if strings.Contains(filepath.ToSlash(p), f) {
					kept = append(kept, p)
					break
				}
			}
		}
		files = kept
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range files {
		if r := discover.Resolve(p); !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out, nil
}
