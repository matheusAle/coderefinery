package coverage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

// Runner runs coverage tools (upstream crapper/runners.py). Stderr receives
// the "+ (cwd) cmd" echo and warnings; child processes inherit Stdout/Stderr.
// Exec is swappable for tests (argv-level mocking, like upstream's
// monkeypatched run_shell).
type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	// Exec runs a command; nil means the real implementation (Shell).
	Exec func(r *Runner, command Command, cwd string) int
	// LookPath is shutil.which; nil means exec.LookPath.
	LookPath func(name string) (string, error)
	// Home is Path.home(); "" means os.UserHomeDir().
	Home string
	// Failed counts per-language TypeScript coverage runs that exited
	// non-zero. Upstream only warns; `coderefinery react` (an addition)
	// reads it to report a failing baseline.
	Failed int
}

// Command is either a shell string (Shell != "") or an argv.
type Command struct {
	Shell string
	Argv  []string
}

func Argv(args ...string) Command { return Command{Argv: args} }

func (r *Runner) warn(msg string) { fmt.Fprintln(r.Stderr, msg) }

func (r *Runner) which(name string) bool {
	look := r.LookPath
	if look == nil {
		look = exec.LookPath
	}
	_, err := look(name)
	return err == nil
}

func (r *Runner) home() string {
	if r.Home != "" {
		return r.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

// ShellQuote is shlex.quote.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_@%+=:,./-", c)) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func (c Command) shown() string {
	if c.Argv == nil {
		return c.Shell
	}
	parts := make([]string, len(c.Argv))
	for i, a := range c.Argv {
		parts[i] = ShellQuote(a)
	}
	return strings.Join(parts, " ")
}

// RunShell is runners.run_shell.
func (r *Runner) RunShell(command Command, cwd string) int {
	fmt.Fprintf(r.Stderr, "+ (%s) %s\n", cwd, command.shown())
	if r.Exec != nil {
		return r.Exec(r, command, cwd)
	}
	return Shell(r, command, cwd)
}

// Shell executes for real: a string through /bin/sh -c, an argv directly,
// stdio inherited, no timeout. Start failures print Python's OSError text
// and return 127; a signal N returns -N.
func Shell(r *Runner, command Command, cwd string) int {
	var cmd *exec.Cmd
	argv0 := ""
	if command.Argv == nil {
		cmd = exec.Command("/bin/sh", "-c", command.Shell)
		argv0 = "/bin/sh"
	} else {
		argv0 = command.Argv[0]
		cmd = exec.Command(command.Argv[0], command.Argv[1:]...)
	}
	cmd.Dir = cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		r.warn(fmt.Sprintf("Coverage command failed to start: [Errno 2] No such file or directory: %s", pyfmt.PyRepr(cwd)))
		return 127
	}
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return -int(ws.Signal())
		}
		return ee.ExitCode()
	}
	switch {
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		r.warn(fmt.Sprintf("Coverage command failed to start: [Errno 2] No such file or directory: %s", pyfmt.PyRepr(argv0)))
	case errors.Is(err, fs.ErrPermission):
		r.warn(fmt.Sprintf("Coverage command failed to start: [Errno 13] Permission denied: %s", pyfmt.PyRepr(argv0)))
	default:
		r.warn(fmt.Sprintf("Coverage command failed to start: %v", err))
	}
	return 127
}

// nearest is runners._nearest.
func nearest(start, marker, stop string) string {
	cur := start
	if !discover.IsDir(start) {
		cur = filepath.Dir(start)
	}
	stop = discover.Resolve(stop)
	for {
		if discover.IsFile(filepath.Join(cur, marker)) {
			return cur
		}
		if cur == stop || filepath.Dir(cur) == cur {
			return ""
		}
		cur = filepath.Dir(cur)
	}
}

// sortPaths orders like Python 3.12 Path sorting: component-wise.
func sortPaths(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		a, b := strings.Split(paths[i], "/"), strings.Split(paths[j], "/")
		for k := 0; k < len(a) && k < len(b); k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})
}

func uniqueSorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sortPaths(out)
	return out
}

func modulesWith(files []string, suffix, marker, root string) []string {
	set := map[string]bool{}
	for _, f := range files {
		if discover.Suffix(f) != suffix {
			continue
		}
		if m := nearest(f, marker, root); m != "" {
			set[m] = true
		}
	}
	return uniqueSorted(set)
}

// CoverageReport is runners._coverage_report.
func CoverageReport(root, module, language string) (string, error) {
	root = discover.Resolve(root)
	module = discover.Resolve(module)
	if module == root {
		return filepath.Join(root, "target", "coverage", language, "lcov.info"), nil
	}
	rel, ok := discover.RelativeTo(module, root)
	if !ok {
		return "", fmt.Errorf("%s is not in the subpath of %s", pyfmt.PyRepr(module), pyfmt.PyRepr(root))
	}
	return filepath.Join(root, "target", "coverage", language, strings.ReplaceAll(rel, "/", "__"), "lcov.info"), nil
}

var coverageDirs = map[string]bool{"typescript": true, "rust": true, "go": true, "python": true}

func outsideLanguageDirs(coverage, path string) bool {
	if coverageDirs[filepath.Base(path)] {
		return false
	}
	rel, _ := filepath.Rel(coverage, path)
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if coverageDirs[part] {
			return false
		}
	}
	return true
}

func cleanClojure(root string) {
	coverage := filepath.Join(root, "target", "coverage")
	if !discover.IsDir(coverage) {
		return
	}
	if lcov := filepath.Join(coverage, "lcov.info"); discover.IsFile(lcov) {
		_ = os.Remove(lcov)
	}
	for _, p := range recursiveGlobSuffix(coverage, ".html") {
		if outsideLanguageDirs(coverage, p) {
			_ = os.Remove(p)
		}
	}
	var dirs []string
	_ = filepath.WalkDir(coverage, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && path != coverage {
			dirs = append(dirs, path)
		}
		return nil
	})
	sortPaths(dirs)
	for i := len(dirs) - 1; i >= 0; i-- {
		if outsideLanguageDirs(coverage, dirs[i]) {
			_ = os.Remove(dirs[i])
		}
	}
}

func (r *Runner) coverClojure(root string) {
	if !discover.IsFile(filepath.Join(root, "deps.edn")) && !discover.IsFile(filepath.Join(root, "bb.edn")) {
		r.warn("No deps.edn or bb.edn; skipping Clojure coverage.")
		return
	}
	cleanClojure(root)
	code := r.RunShell(Argv("clj", "-M:cov", "--lcov"), root)
	if code != 0 {
		r.warn("clj -M:cov --lcov failed; retrying without --lcov.")
		code = r.RunShell(Argv("clj", "-M:cov"), root)
	}
	if code != 0 {
		r.warn(fmt.Sprintf("Clojure coverage exited %d. Clojure coverage will score 0%%.", code))
	}
}

var maven = []string{"mvn", "-q", "org.jacoco:jacoco-maven-plugin:0.8.12:prepare-agent", "test",
	"org.jacoco:jacoco-maven-plugin:0.8.12:report"}

func (r *Runner) coverJava(root string, files []string) {
	modules := modulesWith(files, ".java", "pom.xml", root)
	if len(modules) == 0 {
		r.warn("No pom.xml; skipping Java coverage.")
		return
	}
	for _, m := range modules {
		_ = os.RemoveAll(filepath.Join(m, "target", "site", "jacoco"))
		_ = os.Remove(filepath.Join(m, "target", "jacoco.exec"))
		if code := r.RunShell(Argv(maven...), m); code != 0 {
			r.warn(fmt.Sprintf("Java coverage exited %d in %s. Java coverage will score 0%%.", code, m))
		}
	}
}

func (r *Runner) coverGo(root string, files []string) {
	modules := modulesWith(files, ".go", "go.mod", root)
	if len(modules) == 0 {
		r.warn("No go.mod; skipping Go coverage.")
		return
	}
	for _, m := range modules {
		profile := filepath.Join(m, "target", "coverage", "go", "coverage.out")
		_ = os.MkdirAll(filepath.Dir(profile), 0o777)
		_ = os.Remove(profile)
		if code := r.RunShell(Argv("go", "test", "./...", "-coverprofile="+profile), m); code != 0 {
			r.warn(fmt.Sprintf("Go coverage exited %d in %s. Go coverage will score 0%%.", code, m))
		}
	}
}

// TypescriptPackages is runners.typescript_packages.
func TypescriptPackages(root string, files []string) []string {
	root = discover.Resolve(root)
	set := map[string]bool{}
	for _, f := range files {
		if discover.LanguageOf(f) != "typescript" {
			continue
		}
		if p := nearest(f, "package.json", root); p != "" {
			set[discover.Resolve(p)] = true
		}
	}
	return uniqueSorted(set)
}

type packageJSONError struct{ msg string }

func (e *packageJSONError) Error() string { return e.msg }

// jsonMessage approximates Python json.JSONDecodeError.msg.
func jsonMessage(err error) string {
	var se *json.SyntaxError
	if errors.As(err, &se) {
		if strings.Contains(se.Error(), "looking for beginning of object key string") {
			return "Expecting property name enclosed in double quotes"
		}
		return "Expecting value"
	}
	return err.Error()
}

func readPackageJSON(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, &packageJSONError{fmt.Sprintf("%s: %s", path, jsonMessage(err))}
	}
	return data, nil
}

var vitestConfigs = []string{"vitest.config.ts", "vitest.config.mts", "vitest.config.cts", "vitest.config.js",
	"vitest.config.mjs", "vitest.config.cjs"}

// TypescriptCommand is runners.typescript_command (nil = no test script).
func TypescriptCommand(pkg string, files []string, reportDir string) ([]string, error) {
	path := filepath.Join(pkg, "package.json")
	if !discover.IsFile(path) {
		return nil, nil
	}
	raw, err := readPackageJSON(path)
	if err != nil {
		return nil, err
	}
	data, ok := raw.(map[string]any)
	if !ok {
		return nil, &packageJSONError{fmt.Sprintf("%s: expected a JSON object", path)}
	}
	scripts, _ := data["scripts"].(map[string]any)
	if scripts == nil {
		scripts = map[string]any{}
	}
	if _, ok := scripts["coverage"]; ok {
		return []string{"npm", "run", "coverage"}, nil
	}
	if usesVitest(pkg, scripts) {
		return vitestCommand(pkg, files, reportDir), nil
	}
	if _, ok := scripts["test"]; !ok {
		return nil, nil
	}
	return []string{"npx", "--yes", "c8", "--reporter=lcov", "--reports-dir", reportDir, "npm", "test"}, nil
}

func usesVitest(pkg string, scripts map[string]any) bool {
	if t, ok := scripts["test"]; ok && strings.Contains(pyStr(t), "vitest") {
		return true
	}
	for _, name := range vitestConfigs {
		if discover.IsFile(filepath.Join(pkg, name)) {
			return true
		}
	}
	return false
}

func pyStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func vitestCommand(pkg string, files []string, reportDir string) []string {
	var cmd []string
	if bin := filepath.Join(pkg, "node_modules", ".bin", "vitest"); discover.IsFile(bin) {
		cmd = []string{bin}
	} else {
		cmd = []string{"npx", "vitest"}
	}
	cmd = append(cmd, "run", "--coverage", "--coverage.reporter=lcov", "--coverage.reportsDirectory="+reportDir,
		"--coverage.reportOnFailure=true")
	pkgResolved := discover.Resolve(pkg)
	for _, f := range files {
		if discover.LanguageOf(f) != "typescript" || discover.IsTestFile(f) {
			continue
		}
		if rel, ok := discover.RelativeTo(discover.Resolve(f), pkgResolved); ok {
			cmd = append(cmd, "--coverage.include="+rel)
		}
	}
	return cmd
}

func (r *Runner) vitestVersion(pkg string) string {
	installed := filepath.Join(pkg, "node_modules", "vitest", "package.json")
	if !discover.IsFile(installed) {
		return ""
	}
	raw, err := readPackageJSON(installed)
	if err != nil {
		var pe *packageJSONError
		if errors.As(err, &pe) {
			r.warn(pe.msg)
		}
		return ""
	}
	if m, ok := raw.(map[string]any); ok {
		if v, ok := m["version"].(string); ok {
			return v
		}
	}
	return ""
}

func (r *Runner) ensureVitestCoverage(pkg string) bool {
	provider := filepath.Join(pkg, "node_modules", "@vitest", "coverage-v8")
	if discover.IsDir(provider) {
		return true
	}
	spec := "@vitest/coverage-v8"
	if v := r.vitestVersion(pkg); v != "" {
		spec += "@" + v
	}
	r.warn(fmt.Sprintf("Installing %s so Vitest can write LCOV. package.json and the lockfile are restored afterward.", spec))
	names := []string{"package.json", "package-lock.json", "npm-shrinkwrap.json"}
	snapshot := map[string][]byte{}
	for _, n := range names {
		p := filepath.Join(pkg, n)
		if discover.IsFile(p) {
			b, _ := os.ReadFile(p)
			snapshot[p] = b
		} else {
			snapshot[p] = nil
		}
	}
	code := r.RunShell(Argv("npm", "install", "--no-save", "--no-package-lock", spec), pkg)
	for _, n := range names {
		p := filepath.Join(pkg, n)
		content := snapshot[p]
		if content == nil {
			if discover.IsFile(p) {
				_ = os.Remove(p)
			}
			continue
		}
		cur, err := os.ReadFile(p)
		if err != nil || string(cur) != string(content) {
			_ = os.WriteFile(p, content, 0o666)
		}
	}
	ready := code == 0 && discover.IsDir(provider)
	if !ready {
		r.warn("Vitest's coverage provider is missing. TypeScript coverage will score 0%.")
	}
	return ready
}

func needsVitestProvider(cmd []string) bool {
	if len(cmd) >= 3 && cmd[0] == "npm" && cmd[1] == "run" && cmd[2] == "coverage" {
		return false
	}
	for _, part := range cmd {
		if filepath.Base(part) == "vitest" {
			return true
		}
	}
	return false
}

func (r *Runner) coverTypescript(root string, files []string) error {
	packages := TypescriptPackages(root, files)
	if len(packages) == 0 {
		r.warn("No package.json; skipping TypeScript coverage.")
		return nil
	}
	for _, pkg := range packages {
		report, err := CoverageReport(root, pkg, "typescript")
		if err != nil {
			return err
		}
		cmd, err := TypescriptCommand(pkg, files, filepath.Dir(report))
		if err != nil {
			var pe *packageJSONError
			if errors.As(err, &pe) {
				r.warn(pe.msg)
				continue
			}
			return err
		}
		if cmd == nil {
			r.warn(fmt.Sprintf("No package.json test script in %s; skipping TypeScript coverage.", pkg))
			continue
		}
		if needsVitestProvider(cmd) && !r.ensureVitestCoverage(pkg) {
			continue
		}
		_ = os.RemoveAll(filepath.Dir(report))
		_ = os.MkdirAll(filepath.Dir(report), 0o777)
		if code := r.RunShell(Argv(cmd...), pkg); code != 0 {
			r.Failed++
			r.warn(fmt.Sprintf("TypeScript coverage exited %d in %s. TypeScript coverage will score 0%%.", code, pkg))
		}
	}
	return nil
}

var pythonMarkers = []string{"pyproject.toml", "setup.cfg", "setup.py", "pytest.ini", "tox.ini"}

// PythonRoots is runners.python_roots.
func PythonRoots(root string, files []string) []string {
	root = discover.Resolve(root)
	set := map[string]bool{}
	for _, f := range files {
		if discover.Suffix(f) != ".py" {
			continue
		}
		module := ""
		for _, marker := range pythonMarkers {
			if module = nearest(f, marker, root); module != "" {
				break
			}
		}
		if module == "" {
			module = root
		}
		set[discover.Resolve(module)] = true
	}
	return uniqueSorted(set)
}

func pythonExecutable(pkg string) string {
	for _, rel := range []string{".venv/bin/python", "venv/bin/python"} {
		if c := filepath.Join(pkg, rel); discover.IsFile(c) {
			return c
		}
	}
	return "python3"
}

// UsesPytest is runners.uses_pytest.
func UsesPytest(pkg string) bool {
	if discover.IsFile(filepath.Join(pkg, "pytest.ini")) || discover.IsFile(filepath.Join(pkg, "conftest.py")) {
		return true
	}
	for _, name := range []string{"pyproject.toml", "setup.cfg", "tox.ini"} {
		if text, ok := readText(filepath.Join(pkg, name)); ok && strings.Contains(text, "pytest") {
			return true
		}
	}
	return false
}

// PythonSources is runners.python_sources.
func PythonSources(pkg string, files []string) string {
	pkg = discover.Resolve(pkg)
	set := map[string]bool{}
	for _, f := range files {
		if discover.Suffix(f) != ".py" || discover.IsTestFile(f) {
			continue
		}
		rel, ok := discover.RelativeTo(discover.Resolve(f), pkg)
		if !ok {
			continue
		}
		parts := strings.Split(rel, "/")
		if len(parts) == 1 {
			name := parts[0]
			if i := strings.LastIndexByte(name, '.'); i > 0 {
				name = name[:i]
			}
			set[name] = true
		} else {
			set[parts[0]] = true
		}
	}
	tops := make([]string, 0, len(set))
	for t := range set {
		tops = append(tops, t)
	}
	sort.Strings(tops)
	return strings.Join(tops, ",")
}

// PythonCoverageCommands is runners.python_coverage_commands.
func PythonCoverageCommands(py, kind, dataFile, report, src string) ([]string, []string) {
	run := []string{py, "-m", "coverage", "run", "--data-file=" + dataFile}
	if src != "" {
		run = append(run, "--source="+src)
	}
	if kind == "pytest" {
		run = append(run, "-m", "pytest")
	} else {
		run = append(run, "-m", "unittest", "discover", "-s", ".")
	}
	lcov := []string{py, "-m", "coverage", "lcov", "--data-file=" + dataFile, "-o", report}
	return run, lcov
}

func (r *Runner) ensurePythonModule(py, pkg, module string) bool {
	if r.RunShell(Argv(py, "-c", "import "+module), pkg) == 0 {
		return true
	}
	r.warn(fmt.Sprintf("Installing %s for Python coverage. The project requirements are left unchanged.", module))
	return r.RunShell(Argv(py, "-m", "pip", "install", "--disable-pip-version-check", module), pkg) == 0
}

func (r *Runner) pythonKind(py, pkg string) string {
	if !UsesPytest(pkg) {
		return "unittest"
	}
	if r.ensurePythonModule(py, pkg, "pytest") {
		return "pytest"
	}
	r.warn(fmt.Sprintf("pytest is missing in %s. Falling back to unittest.", pkg))
	return "unittest"
}

func (r *Runner) coverPython(root string, files []string) error {
	for _, pkg := range PythonRoots(root, files) {
		report, err := CoverageReport(root, pkg, "python")
		if err != nil {
			return err
		}
		py := pythonExecutable(pkg)
		if !r.ensurePythonModule(py, pkg, "coverage") {
			r.warn(fmt.Sprintf("coverage is missing in %s. Python coverage will score 0%%.", pkg))
			continue
		}
		_ = os.MkdirAll(filepath.Dir(report), 0o777)
		dataFile := filepath.Join(filepath.Dir(report), ".coverage")
		_ = os.Remove(report)
		_ = os.Remove(dataFile)
		kind := r.pythonKind(py, pkg)
		runCmd, lcovCmd := PythonCoverageCommands(py, kind, dataFile, report, PythonSources(pkg, files))
		code := r.RunShell(Argv(runCmd...), pkg)
		if !discover.Exists(dataFile) {
			if code != 0 {
				r.warn(fmt.Sprintf("Python coverage exited %d in %s. Python coverage will score 0%%.", code, pkg))
			}
			continue
		}
		if lcovCode := r.RunShell(Argv(lcovCmd...), pkg); lcovCode != 0 {
			r.warn(fmt.Sprintf("coverage lcov exited %d in %s. Python coverage will score 0%%.", lcovCode, pkg))
		} else if code != 0 {
			r.warn(fmt.Sprintf("Python tests exited %d in %s. Coverage was still recorded.", code, pkg))
		}
	}
	return nil
}

// RustModules is runners.rust_modules.
func RustModules(root string, files []string) []string {
	root = discover.Resolve(root)
	var out []string
	for _, m := range modulesWith(files, ".rs", "Cargo.toml", root) {
		out = append(out, discover.Resolve(m))
	}
	return out
}

func (r *Runner) rustKind() string {
	if r.which("cargo-llvm-cov") {
		return "llvm-cov"
	}
	if r.which("cargo-tarpaulin") {
		return "tarpaulin"
	}
	r.warn("Neither cargo-llvm-cov nor cargo-tarpaulin is installed. Installing cargo-llvm-cov.")
	if r.which("rustup") {
		if code := r.RunShell(Argv("rustup", "component", "add", "llvm-tools-preview"), r.home()); code != 0 {
			r.warn("rustup component add llvm-tools-preview failed.")
		}
	}
	code := r.RunShell(Argv("cargo", "install", "cargo-llvm-cov", "--locked"), r.home())
	if code == 0 && r.which("cargo-llvm-cov") {
		return "llvm-cov"
	}
	r.warn("Rust coverage will score 0%.")
	return ""
}

// RustCoverageCommand is runners.rust_coverage_command.
func RustCoverageCommand(kind, report string) []string {
	if kind == "llvm-cov" {
		return []string{"cargo", "llvm-cov", "--lcov", "--output-path", report}
	}
	return []string{"cargo", "tarpaulin", "--out", "Lcov", "--output-dir", filepath.Dir(report)}
}

func (r *Runner) coverRust(root string, files []string) error {
	modules := RustModules(root, files)
	if len(modules) == 0 {
		r.warn("No Cargo.toml; skipping Rust coverage.")
		return nil
	}
	kind := r.rustKind()
	if kind == "" {
		return nil
	}
	for _, m := range modules {
		report, err := CoverageReport(root, m, "rust")
		if err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(report), 0o777)
		_ = os.Remove(report)
		if code := r.RunShell(Argv(RustCoverageCommand(kind, report)...), m); code != 0 {
			r.warn(fmt.Sprintf("Rust coverage exited %d in %s. Rust coverage will score 0%%.", code, m))
		}
	}
	return nil
}

// RunCoverage is runners.run_coverage. It returns the custom command's
// status (0 for the per-language runners). err is an internal failure
// (upstream would crash with a traceback).
func (r *Runner) RunCoverage(root string, files []string, command string) (int, error) {
	root = discover.Resolve(root)
	if command != "" {
		code := r.RunShell(Command{Shell: command}, root)
		if code != 0 {
			r.warn(fmt.Sprintf("Coverage command exited %d. Reports already on disk will not be read.", code))
		}
		return code, nil
	}
	languages := map[string]bool{}
	for _, f := range files {
		if l := discover.LanguageOf(f); l != "" {
			languages[l] = true
		}
	}
	if languages["clojure"] {
		r.coverClojure(root)
	}
	if languages["java"] {
		r.coverJava(root, files)
	}
	if languages["go"] {
		r.coverGo(root, files)
	}
	if languages["typescript"] {
		if err := r.coverTypescript(root, files); err != nil {
			return 0, err
		}
	}
	if languages["python"] {
		if err := r.coverPython(root, files); err != nil {
			return 0, err
		}
	}
	if languages["rust"] {
		if err := r.coverRust(root, files); err != nil {
			return 0, err
		}
	}
	return 0, nil
}
