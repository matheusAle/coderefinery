package mutate

// Run mutants in symlink overlays (upstream mutator/workers.py). Each worker
// is a directory under target/mutation-workers with the project linked in
// and a private copy of the file under test.

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/discover"
)

// skipLink: build output and the worker tree itself are never shared.
var skipLink = map[string]bool{
	".git": true, ".hg": true, ".svn": true, ".idea": true, ".venv": true, "venv": true, "target": true,
	"dist": true, "build": true, "out": true, "coverage": true, ".metrics": true, "__pycache__": true,
	".pytest_cache": true, ".mypy_cache": true, ".clj-kondo": true, ".gradle": true,
}

var configFiles = []string{
	"deps.edn", "bb.edn", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "go.mod", "go.sum",
	"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "tsconfig.json", "Cargo.toml",
	"Cargo.lock", "pyproject.toml", "pytest.ini", "setup.cfg", "setup.py", "conftest.py", "requirements.txt",
	"Pipfile", "poetry.lock",
}

var nodeExtensions = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}

// workerCount is workers.worker_count. requested 0 = None.
func workerCount(siteCount, requested int) int {
	processors := runtime.NumCPU()
	limit := processors
	if requested != 0 {
		limit = requested
	}
	return max(1, min(siteCount, processors, limit))
}

func uuid4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func newRunDir(root string) (string, error) {
	parent := filepath.Join(root, "target", "mutation-workers")
	if err := os.MkdirAll(parent, 0o777); err != nil {
		return "", err
	}
	dir := filepath.Join(parent, "run-"+uuid4())
	return dir, os.Mkdir(dir, 0o777)
}

func isSymlink(p string) bool {
	info, err := os.Lstat(p)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func lexists(p string) bool { _, err := os.Lstat(p); return err == nil }

// symlink is workers.symlink: link -> target.resolve().
func symlink(link, target string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o777); err != nil {
		return err
	}
	if lexists(link) {
		if err := os.Remove(link); err != nil {
			return err
		}
	}
	return os.Symlink(discover.Resolve(target), link)
}

// deleteTree removes a worker tree without following links.
func deleteTree(p string) {
	info, err := os.Lstat(p)
	if err != nil {
		return
	}
	if info.IsDir() {
		entries, _ := os.ReadDir(p)
		for _, e := range entries {
			deleteTree(filepath.Join(p, e.Name()))
		}
	}
	_ = os.Remove(p)
}

func copyConfigs(worker, root, relative string) error {
	for _, name := range configFiles {
		src := filepath.Join(root, name)
		if !discover.IsFile(src) || name == relative {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(worker, name), data, 0o666); err != nil {
			return err
		}
	}
	return nil
}

// packageDirNames are the skipLink names that are also ordinary package
// names (Go internal/coverage, a Python build package, a Java ...build
// package). Upstream hides them at every depth of the overlay, so a mutant
// in a sibling package fails to build and counts as killed (self-audit C1).
var packageDirNames = map[string]bool{"target": true, "dist": true, "build": true, "out": true, "coverage": true}

// packageDirs is --fix-overlay (DEVIATIONS.md M8): it keeps a skip-listed
// directory in the overlay when it directly holds a source file of the
// mutated file's language, which build output and coverage reports do not
// (Maven target/, Cargo target/, Gradle build/ hold only subdirectories and
// artifacts). root/target is never linked: the worker tree lives in it.
// typescript (TS and JS) is left alone: dist/ and coverage/ HTML reports
// hold .js files there. nil = upstream behaviour.
func packageDirs(root, language string) func(dir, name string) bool {
	if language == "" || language == "typescript" {
		return nil
	}
	workerHome := filepath.Join(root, "target")
	return func(dir, name string) bool {
		p := filepath.Join(dir, name)
		if !packageDirNames[name] || p == workerHome {
			return false
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if e.Type().IsRegular() && discover.LanguageOf(e.Name()) == language {
				return true
			}
		}
		return false
	}
}

// linkChildren links realDir's children into workerDir, except skip and the
// skipLink names; keepDir (nil = upstream) rescues skipLink names.
func linkChildren(workerDir, realDir, skip string, keepDir func(dir, name string) bool) error {
	if !discover.IsDir(realDir) {
		return nil
	}
	entries, err := os.ReadDir(realDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == skip || skipLink[name] && (keepDir == nil || !keepDir(realDir, name)) {
			continue
		}
		dest := filepath.Join(workerDir, name)
		if lexists(dest) {
			continue
		}
		if err := symlink(dest, filepath.Join(realDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// specifierPattern: Node resolves a relative import from the module's real
// path, so a symlinked importer would load the original source. \s is
// Python's Unicode whitespace.
var specifierPattern = regexp.MustCompile(`(?:from|import|require)[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]*\(?[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]*['"](\.[^'"]+)['"]`)

func specifiers(text string) []string {
	var found []string
	for _, m := range specifierPattern.FindAllStringSubmatch(text, -1) {
		spec, _, _ := strings.Cut(m[1], "?")
		spec, _, _ = strings.Cut(spec, "#")
		if strings.HasPrefix(spec, ".") {
			found = append(found, spec)
		}
	}
	return found
}

func isNodeExt(ext string) bool { return containsString(nodeExtensions, ext) }

// pathJoin is pathlib `dir / spec`: "." components dropped, ".." kept.
func pathJoin(dir, spec string) string {
	out := dir
	for _, part := range strings.Split(spec, "/") {
		if part == "" || part == "." {
			continue
		}
		out = out + "/" + part
	}
	return out
}

func importCandidates(dir, spec string) []string {
	base := pathJoin(dir, spec)
	stem := base
	if ext := discover.Suffix(base); isNodeExt(ext) {
		stem = strings.TrimSuffix(base, ext)
	}
	names := []string{base}
	for _, ext := range nodeExtensions {
		names = append(names, stem+ext)
	}
	for _, ext := range nodeExtensions {
		names = append(names, stem+"/index"+ext)
	}
	return names
}

func importedFiles(importer, root string) []string {
	raw, err := os.ReadFile(importer)
	if err != nil || !utf8.Valid(raw) {
		return nil
	}
	rootResolved := discover.Resolve(root)
	var found []string
	for _, spec := range specifiers(string(raw)) {
		for _, cand := range importCandidates(filepath.Dir(importer), spec) {
			rel, ok := discover.RelativeTo(discover.Resolve(cand), rootResolved)
			if !ok {
				continue
			}
			local := filepath.Join(root, rel)
			if discover.IsFile(local) {
				found = append(found, discover.Resolve(local))
			}
		}
	}
	return found
}

// nodeSources is workers._source_files: os.walk without following links.
func nodeSources(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != root && (skipLink[d.Name()] || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 && discover.IsDir(p) {
			return nil // a linked dir is listed, never walked
		}
		if isNodeExt(discover.Suffix(d.Name())) {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// importersOf is workers.importers_of: modules that reach relative through
// relative imports (transitively), as posix paths under root.
func importersOf(root, relative string) []string {
	target := discover.Resolve(filepath.Join(root, relative))
	reverse := map[string]map[string]bool{}
	for _, src := range nodeSources(root) {
		for _, imported := range importedFiles(src, root) {
			if reverse[imported] == nil {
				reverse[imported] = map[string]bool{}
			}
			reverse[imported][discover.Resolve(src)] = true
		}
	}
	found := map[string]bool{}
	pending := []string{target}
	for len(pending) > 0 {
		cur := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for importer := range reverse[cur] {
			if found[importer] || importer == target {
				continue
			}
			found[importer] = true
			pending = append(pending, importer)
		}
	}
	rootResolved := discover.Resolve(root)
	var out []string
	for p := range found {
		if rel, ok := discover.RelativeTo(p, rootResolved); ok {
			out = append(out, rel)
		}
	}
	sortStable(out, func(a, b string) bool { return a < b })
	return out
}

// expandDirectory replaces a linked dir so a copied module is not written
// through it.
func expandDirectory(link, realDir string) error {
	if !discover.IsDir(realDir) {
		return nil
	}
	if err := os.Remove(link); err != nil {
		return err
	}
	if err := os.Mkdir(link, 0o777); err != nil {
		return err
	}
	entries, err := os.ReadDir(realDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if skipLink[e.Name()] {
			continue
		}
		if err := symlink(filepath.Join(link, e.Name()), filepath.Join(realDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyImporter(worker, root, relative string) error {
	real, current := root, worker
	segments := strings.Split(relative, "/")
	for _, seg := range segments[:len(segments)-1] {
		real = filepath.Join(real, seg)
		current = filepath.Join(current, seg)
		if isSymlink(current) {
			if err := expandDirectory(current, real); err != nil {
				return err
			}
		} else if !discover.Exists(current) {
			if err := os.MkdirAll(current, 0o777); err != nil {
				return err
			}
		}
	}
	dest := filepath.Join(worker, relative)
	if isSymlink(dest) {
		if err := os.Remove(dest); err != nil {
			return err
		}
	} else if discover.Exists(dest) {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o666)
}

// overlay: real directories along the source path, every sibling a link,
// the file itself a private copy.
func overlay(worker, root, relative string, original []byte, keepDir func(dir, name string) bool) error {
	segments := strings.Split(relative, "/")
	for i := range segments[:len(segments)-1] {
		relDir := filepath.Join(segments[:i+1]...)
		if err := linkChildren(filepath.Join(worker, relDir), filepath.Join(root, relDir), segments[i+1], keepDir); err != nil {
			return err
		}
	}
	dest := filepath.Join(worker, filepath.Join(segments...))
	if err := os.MkdirAll(filepath.Dir(dest), 0o777); err != nil {
		return err
	}
	return os.WriteFile(dest, original, 0o666)
}

func createWorkers(base, root, relative string, original []byte, count int, keepDir func(dir, name string) bool) ([]string, error) {
	first, _, _ := strings.Cut(relative, "/")
	importers := importersOf(root, relative)
	var created []string
	for i := 0; i < count; i++ {
		worker := filepath.Join(base, fmt.Sprintf("worker-%d", i))
		if err := os.MkdirAll(worker, 0o777); err != nil {
			return nil, err
		}
		if err := copyConfigs(worker, root, relative); err != nil {
			return nil, err
		}
		if err := linkChildren(worker, root, first, keepDir); err != nil {
			return nil, err
		}
		if err := overlay(worker, root, relative, original, keepDir); err != nil {
			return nil, err
		}
		for _, imp := range importers {
			if err := copyImporter(worker, root, imp); err != nil {
				return nil, err
			}
		}
		created = append(created, worker)
	}
	return created, nil
}

// mappedCwd is workers.mapped_cwd.
func mappedCwd(worker, root, cwd string) string {
	rel, ok := discover.RelativeTo(discover.Resolve(cwd), discover.Resolve(root))
	if !ok {
		return worker
	}
	return discover.Resolve(filepath.Join(worker, rel))
}

// mutantJob is everything a worker needs to run one file's mutants.
type mutantJob struct {
	root, relative, fileKey string
	original                []byte
	runner                  Runner
	command                 Command
	cwd                     string
	timeout                 time.Duration
	stderr                  io.Writer
	lock                    sync.Mutex
	outcomes                map[string]string
	attr                    *attribution                // --attribute; nil = upstream behaviour
	killedBy                map[string][]string         // :killed-by, updated for every mutant run
	keepDir                 func(dir, name string) bool // --fix-overlay; nil = upstream overlay
}

// execute runs one mutant; by is the --attribute credit of a kill.
func (j *mutantJob) execute(dir string, s Site) (status string, by []string) {
	if !bytes.Equal(j.original[s.Start:s.End], []byte(s.Original)) {
		fmt.Fprintf(j.stderr, "Skipped %s:%d %s; source bytes moved\n", j.fileKey, s.Line, s.Description())
		return "survived", nil
	}
	dest := filepath.Join(dir, j.relative)
	_ = os.WriteFile(dest, applySite(j.original, s.Start, s.End, s.Mutant), 0o666)
	defer func() { _ = os.WriteFile(dest, j.original, 0o666) }()
	if j.attr != nil {
		return j.attr.run(j, dir, s)
	}
	result := j.runner.Run(j.command, mappedCwd(dir, j.root, j.cwd), j.timeout)
	if result.TimedOut || result.Code != 0 {
		return "killed", nil
	}
	return "survived", nil
}

// runMutants is workers.run_mutants: sites in parallel overlays, outcomes
// recorded into outcomes. The source tree stays put.
func runMutants(job *mutantJob, sites []Site, maxWorkers int) error {
	if len(sites) == 0 {
		return nil
	}
	base, err := newRunDir(job.root)
	if err != nil {
		return err
	}
	defer deleteTree(base)
	dirs, err := createWorkers(base, job.root, job.relative, job.original, workerCount(len(sites), maxWorkers), job.keepDir)
	if err != nil {
		return err
	}
	pending := make(chan Site, len(sites))
	for _, s := range sites {
		pending <- s
	}
	close(pending)
	var wg sync.WaitGroup
	for _, dir := range dirs {
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			for s := range pending {
				if job.runner.IsVerbose() {
					job.lock.Lock()
					fmt.Fprintf(job.stderr, "%s:%d %s\n", job.fileKey, s.Line, s.Description())
					job.lock.Unlock()
				}
				status, by := job.execute(dir, s)
				job.lock.Lock()
				job.outcomes[s.ID] = status
				if job.killedBy != nil {
					if status == "killed" && by != nil {
						job.killedBy[s.ID] = by
					} else {
						delete(job.killedBy, s.ID) // a re-run mutant drops its old credit
					}
				}
				job.lock.Unlock()
			}
		}(dir)
	}
	wg.Wait()
	return nil
}
