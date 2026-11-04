package mutate

// --fix-overlay for TypeScript/JavaScript (DEVIATIONS.md M9): the modules a
// test runner loads by path become real files in the worker.
//
// Vite resolves a module id to its real path. Vitest asks for each test file
// and setup file by path, so a symlinked one resolves outside the worker,
// and under `environment: "jsdom"` (Vite's client environment) the load is
// checked against server.fs.allow: the config's directory, the workspace root
// found from the worker (the worker itself, which holds a copy of
// package.json) and Vitest's own package. The suite fails with "Failed to
// load url ... Does the file exist?", the run exits 1 and every mutant is
// KILLED. Modules reached through imports are not affected (Vite allows
// every id it resolved for an import), nor is the node environment (no fs
// check), nor a project inside a workspace (its root is allowed). Jest's
// crawler skips symlinked files: a linked test file does not run at all.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
)

// copySources replaces the worker's links into root by real directories, and
// links to TS/JS files by copies. Other files stay links. node_modules and
// dot-directories stay linked: packages are reached through bare imports,
// which Vite allows, and dot-directories hold tool caches (.next, .turbo,
// .yarn). A link out of root (a linked sibling package) stays a link.
func copySources(worker, root string) error { return copyLinked(worker, discover.Resolve(root)) }

func copyLinked(dir, realRoot string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.Type()&os.ModeSymlink == 0 {
			if e.IsDir() {
				if err := copyLinked(p, realRoot); err != nil {
					return err
				}
			}
			continue
		}
		target, err := os.Readlink(p) // symlink() writes resolved absolute paths
		if err != nil {
			return err
		}
		if _, inside := discover.RelativeTo(target, realRoot); !inside {
			continue
		}
		info, err := os.Stat(target)
		switch {
		case err != nil:
			continue
		case info.IsDir():
			if e.Name() == "node_modules" || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if err := expandDirectory(p, target); err != nil {
				return err
			}
			if err := copyLinked(p, realRoot); err != nil {
				return err
			}
		case info.Mode().IsRegular() && isNodeExt(discover.Suffix(e.Name())):
			data, err := os.ReadFile(target)
			if err != nil {
				return err
			}
			if err := os.Remove(p); err != nil {
				return err
			}
			if err := os.WriteFile(p, data, 0o666); err != nil {
				return err
			}
		}
	}
	return nil
}
