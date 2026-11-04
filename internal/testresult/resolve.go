package testresult

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// resolver maps report names (paths, dotted modules, FQCNs, namespaces,
// nextest binary ids) to files relative to root, the tool's cwd.
type resolver struct {
	root  string
	index map[string][]string // base name -> sorted rel paths (lazy)
}

func newResolver(root string) *resolver { return &resolver{root: root} }

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

// rel is a report path made relative to root, POSIX; absolute paths outside
// root stay absolute. "" stays "".
func (r *resolver) rel(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	if filepath.IsAbs(p) && r.root != "" {
		for _, base := range []string{r.root, resolved(r.root)} {
			if rel, err := filepath.Rel(base, p); err == nil && !strings.HasPrefix(rel, "..") {
				return filepath.ToSlash(rel)
			}
		}
		if rp := resolved(p); rp != p {
			if rel, err := filepath.Rel(resolved(r.root), rp); err == nil && !strings.HasPrefix(rel, "..") {
				return filepath.ToSlash(rel)
			}
		}
		return p
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(p)), "./")
}

func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "target": true, "build": true, ".venv": true, "venv": true,
	".metrics": true, "out": true, "dist": true, "__pycache__": true, ".gradle": true,
}

// byName is every file under root named name (outside build output),
// lexically sorted.
func (r *resolver) byName(name string) []string {
	if r.root == "" {
		return nil
	}
	if r.index == nil {
		r.index = map[string][]string{}
		_ = filepath.WalkDir(r.root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p != r.root && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(r.root, p)
			r.index[d.Name()] = append(r.index[d.Name()], filepath.ToSlash(rel))
			return nil
		})
		for _, v := range r.index {
			sort.Strings(v)
		}
	}
	return r.index[name]
}

// pythonModule resolves a dotted classname (pytest xunit2, no file attr) to
// the longest prefix that is a .py file under root.
func (r *resolver) pythonModule(classname string) string {
	parts := strings.Split(classname, ".")
	for k := len(parts); k > 0; k-- {
		rel := strings.Join(parts[:k], "/") + ".py"
		if r.root != "" && isFile(filepath.Join(r.root, rel)) {
			return rel
		}
	}
	return ""
}

// javaFile resolves an outer FQCN: src/test/{java,kotlin}/<pkg>/<Outer>.{java,kt}
// (also inside sub-modules), else the first lexical **/<Outer>.java outside
// build output, else "".
func (r *resolver) javaFile(outer string) string {
	if outer == "" || r.root == "" {
		return ""
	}
	rel := strings.ReplaceAll(outer, ".", "/")
	simple := outer[strings.LastIndexByte(outer, '.')+1:]
	for _, cand := range []string{"src/test/java/" + rel + ".java", "src/test/kotlin/" + rel + ".kt", "src/test/kotlin/" + rel + ".java"} {
		if isFile(filepath.Join(r.root, cand)) {
			return cand
		}
	}
	for _, ext := range []string{".java", ".kt"} {
		hits := r.byName(simple + ext)
		for _, h := range hits {
			if strings.HasSuffix(h, "/"+rel+ext) {
				return h
			}
		}
		if len(hits) > 0 {
			return hits[0]
		}
	}
	return ""
}

// rustTestFile is tests/<x>.rs for a nextest integration-test binary id
// (crate::x); "" for lib/bin binaries.
func (r *resolver) rustTestFile(binary string) string {
	_, kind, ok := strings.Cut(binary, "::")
	if !ok || strings.Contains(kind, "/") || r.root == "" {
		return ""
	}
	if cand := "tests/" + kind + ".rs"; isFile(filepath.Join(r.root, cand)) {
		return cand
	}
	return ""
}

// clojureFile resolves a test namespace (- becomes _) under the usual test
// paths, else the first lexical match anywhere.
func (r *resolver) clojureFile(ns string) string {
	if ns == "" || r.root == "" {
		return ""
	}
	rel := strings.ReplaceAll(strings.ReplaceAll(ns, "-", "_"), ".", "/")
	for _, dir := range []string{"test", "src/test/clojure", "spec", "src", "."} {
		for _, ext := range []string{".clj", ".cljc", ".cljs"} {
			cand := filepath.ToSlash(filepath.Join(dir, rel+ext))
			if isFile(filepath.Join(r.root, cand)) {
				return cand
			}
		}
	}
	simple := rel[strings.LastIndexByte(rel, '/')+1:]
	for _, ext := range []string{".clj", ".cljc", ".cljs"} {
		for _, h := range r.byName(simple + ext) {
			if strings.HasSuffix(h, rel+ext) {
				return h
			}
		}
	}
	return ""
}
