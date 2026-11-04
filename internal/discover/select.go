package discover

import (
	"path/filepath"
	"sort"
	"strings"
)

// Selection is one tool's cli.select_files. crapper, dryer and mutator copy
// the same skeleton: split the positionals into existing paths and filters;
// take --changed, else the source roots, else the existing paths, else the
// project root; keep the paths containing a filter; resolve, dedupe, sort.
// They differ only in the hooks below.
type Selection struct {
	ProjectRoot string
	SourceRoots []string
	Positionals []string
	Changed     bool
	// Walk lists the source files under roots (IterSourceFiles, or the
	// tool's own language table).
	Walk func(roots []string) []string
	// KeepChanged filters the `git status` paths of --changed; root is resolved.
	KeepChanged func(path, root string) bool
	// KeepFile decides whether a file named on the command line is analyzed.
	KeepFile func(path string) bool
	// Outside, when set, drops every selected file outside the root after
	// reporting it (mutator); nil keeps them (crapper, dryer).
	Outside func(path, root string)
}

// JoinRoot is pathlib's root / p: an absolute p wins.
func JoinRoot(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

// Select is cli.select_files: sorted, deduplicated, resolved paths.
func Select(s Selection) ([]string, error) {
	root := Resolve(s.ProjectRoot)
	var existing, filters []string
	for _, arg := range s.Positionals {
		if candidate := JoinRoot(root, arg); Exists(candidate) {
			existing = append(existing, Resolve(candidate))
		} else {
			filters = append(filters, arg)
		}
	}
	files, err := s.candidates(root, existing)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range containingAny(files, filters) {
		r := Resolve(p)
		if s.Outside != nil && !IsRelativeTo(r, root) {
			s.Outside(p, root)
			continue
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (s Selection) candidates(root string, existing []string) ([]string, error) {
	var files []string
	switch {
	case s.Changed:
		changed, err := ChangedFiles(root)
		if err != nil {
			return nil, err
		}
		for _, p := range changed {
			if s.KeepChanged(p, root) {
				files = append(files, p)
			}
		}
	case len(s.SourceRoots) > 0:
		var roots []string
		for _, p := range s.SourceRoots {
			roots = append(roots, Resolve(JoinRoot(root, p)))
		}
		files = s.Walk(roots)
	case len(existing) > 0:
		for _, p := range existing {
			if IsDir(p) {
				files = append(files, s.Walk([]string{p})...)
			} else if s.KeepFile(p) {
				files = append(files, p)
			}
		}
	default:
		files = s.Walk([]string{root})
	}
	return files, nil
}

// containingAny keeps the paths that contain one of filters (all of them
// when there is none). Paths compare in slash form, as dryer does; that is
// the identity on the supported (Unix) platforms.
func containingAny(files, filters []string) []string {
	if len(filters) == 0 {
		return files
	}
	var kept []string
	for _, p := range files {
		for _, f := range filters {
			if strings.Contains(filepath.ToSlash(p), f) {
				kept = append(kept, p)
				break
			}
		}
	}
	return kept
}
