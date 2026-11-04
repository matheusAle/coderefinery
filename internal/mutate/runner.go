package mutate

// Run a project's tests, and decide the command for each language
// (upstream mutator/runner.py).

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
	"github.com/matheusAle/coderefinery/internal/source"
)

// Command is a shell string (Shell != "", from --test-command or
// --coverage-command) or an argument vector.
type Command struct {
	Shell string
	Argv  []string
}

// display is runner.display_command.
func (c Command) display() string {
	if c.Argv == nil {
		return c.Shell
	}
	quoted := make([]string, len(c.Argv))
	for i, a := range c.Argv {
		quoted[i] = coverage.ShellQuote(a)
	}
	return strings.Join(quoted, " ")
}

// key is engine._command_key (plus the cwd, as the baseline cache uses it).
func (c Command) key(cwd string) string {
	if c.Argv == nil {
		return "sh\x00" + c.Shell + "\x01" + cwd
	}
	return "argv\x00" + strings.Join(c.Argv, "\x00") + "\x01" + cwd
}

// CommandResult is runner.CommandResult.
type CommandResult struct {
	Code     int
	TimedOut bool
	Seconds  float64
	Output   string
}

// Runner runs a command; CommandRunner is the real one, tests use fakes.
type Runner interface {
	Run(command Command, cwd string, timeout time.Duration) CommandResult
	IsVerbose() bool
}

// CommandRunner is runner.CommandRunner. timeout 0 = none.
type CommandRunner struct {
	Verbose bool
	Stderr  io.Writer
	mu      sync.Mutex
}

func (r *CommandRunner) IsVerbose() bool { return r.Verbose }

// workerHome is runner._worker_home: the worker-N dir when cwd is inside
// target/mutation-workers/run-*/.
func workerHome(cwd string) string {
	for cand := cwd; ; {
		parent := filepath.Dir(cand)
		if strings.HasPrefix(filepath.Base(cand), "worker-") && strings.HasPrefix(filepath.Base(parent), "run-") &&
			filepath.Base(filepath.Dir(parent)) == "mutation-workers" {
			return cand
		}
		if parent == cand {
			return ""
		}
		cand = parent
	}
}

// commandEnv is os.environ + PYTHONDONTWRITEBYTECODE=1, with worker sources
// first on PYTHONPATH (runner._prefer_worker_sources).
func commandEnv(cwd string) []string {
	env := setEnv(os.Environ(), "PYTHONDONTWRITEBYTECODE", "1")
	worker := workerHome(cwd)
	if worker == "" {
		return env
	}
	var entries []string
	for _, dir := range []string{filepath.Join(cwd, "src"), cwd, filepath.Join(worker, "src"), worker} {
		if !containsString(entries, dir) && discover.IsDir(dir) {
			entries = append(entries, dir)
		}
	}
	if current := os.Getenv("PYTHONPATH"); current != "" {
		entries = append(entries, current)
	}
	if len(entries) == 0 {
		return env
	}
	return setEnv(env, "PYTHONPATH", strings.Join(entries, string(os.PathListSeparator)))
}

func setEnv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return append(out, key+"="+value)
}

// spawnError is Python's str(OSError) for a failed Popen.
func spawnError(err error, argv0, cwd string) string {
	if info, serr := os.Stat(cwd); serr != nil || !info.IsDir() {
		return "[Errno 2] No such file or directory: " + pyfmt.PyRepr(cwd)
	}
	switch {
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return "[Errno 2] No such file or directory: " + pyfmt.PyRepr(argv0)
	case errors.Is(err, fs.ErrPermission):
		return "[Errno 13] Permission denied: " + pyfmt.PyRepr(argv0)
	}
	return err.Error()
}

// Run is CommandRunner.run: merged stdout+stderr captured as text, its own
// session, SIGKILL to the process group on timeout (code 124).
func (r *CommandRunner) Run(command Command, cwd string, timeout time.Duration) CommandResult {
	if r.Verbose {
		r.mu.Lock()
		fmt.Fprintf(r.Stderr, "+ (%s) %s\n", cwd, command.display())
		r.mu.Unlock()
	}
	started := time.Now()
	var cmd *exec.Cmd
	argv0 := "/bin/sh"
	if command.Argv == nil {
		cmd = exec.Command("/bin/sh", "-c", command.Shell)
	} else {
		argv0 = command.Argv[0]
		cmd = exec.Command(command.Argv[0], command.Argv[1:]...)
	}
	cmd.Dir = cwd
	cmd.Env = commandEnv(cwd)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return CommandResult{Code: 127, Seconds: time.Since(started).Seconds(), Output: spawnError(err, argv0, cwd)}
	}
	var timedOut atomic.Bool
	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			timedOut.Store(true)
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		})
	}
	err := cmd.Wait()
	if timer != nil {
		timer.Stop()
	}
	result := CommandResult{Seconds: time.Since(started).Seconds(), Output: source.UniversalNewlines(source.DecodeReplace(out.Bytes()))}
	switch {
	case timedOut.Load():
		result.Code, result.TimedOut = 124, true
	case err != nil:
		result.Code = exitCode(err)
	}
	return result
}

// exitCode is Popen.returncode: the status, or -N for signal N.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return -int(ws.Signal())
		}
		return ee.ExitCode()
	}
	return 1
}

// nearest is runner.nearest: the first dir from start (a file's parent)
// up to stop that holds marker as a file; "" when none.
func nearest(start, marker, stop string) string {
	current := start
	if !discover.IsDir(start) {
		current = filepath.Dir(start)
	}
	stop = discover.Resolve(stop)
	current = discover.Resolve(current)
	for {
		if discover.IsFile(filepath.Join(current, marker)) {
			return current
		}
		parent := filepath.Dir(current)
		if current == stop || parent == current {
			return ""
		}
		current = parent
	}
}

func readOrEmpty(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

func clojureCommand(dir string) []string {
	deps := filepath.Join(dir, "deps.edn")
	bb := filepath.Join(dir, "bb.edn")
	if discover.IsFile(bb) && !discover.IsFile(deps) {
		if strings.Contains(readOrEmpty(bb), "spec") {
			return []string{"bb", "spec", "--tag", "~no-mutate"}
		}
		return []string{"bb", "test"}
	}
	text := readOrEmpty(deps)
	switch {
	case strings.Contains(text, ":spec") && strings.Contains(text, "speclj"):
		return []string{"clj", "-M:spec", "--tag", "~no-mutate"}
	case strings.Contains(text, ":spec"):
		return []string{"clj", "-M:spec"}
	}
	return []string{"clj", "-M:test"}
}

// selfPython stands in for upstream's sys.executable: python3 on PATH.
func selfPython() string {
	if p, err := exec.LookPath("python3"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	return "python3"
}

// projectInterpreter is runner._project_interpreter (absolute, not resolved).
func projectInterpreter(dir string) string {
	for _, name := range []string{".venv", "venv"} {
		cand := filepath.Join(dir, name, "bin", "python")
		if abs, err := filepath.Abs(cand); err == nil {
			cand = abs
		}
		if discover.IsFile(cand) && syscall.Access(cand, 1) == nil {
			return cand
		}
	}
	return selfPython()
}

func pythonCommand(dir string) []string {
	py := projectInterpreter(dir)
	if discover.IsFile(filepath.Join(dir, "pytest.ini")) || discover.IsFile(filepath.Join(dir, "conftest.py")) ||
		strings.Contains(readOrEmpty(filepath.Join(dir, "pyproject.toml")), "pytest") ||
		discover.IsFile(filepath.Join(dir, "setup.cfg")) && strings.Contains(readOrEmpty(filepath.Join(dir, "setup.cfg")), "pytest") {
		return []string{py, "-m", "pytest"}
	}
	return []string{py, "-m", "unittest", "discover"}
}

func nearestMarker(src, root string, markers ...string) string {
	for _, m := range markers {
		if found := nearest(src, m, root); found != "" {
			return found
		}
	}
	return root
}

func goPackage(dir, src string) string {
	rel, ok := discover.RelativeTo(filepath.Dir(discover.Resolve(src)), discover.Resolve(dir))
	if !ok || rel == "." {
		return "."
	}
	return "./" + rel
}

// testPlan is runner.test_plan: the command and its working directory.
func testPlan(root, src, language, override string) (Command, string) {
	root = discover.Resolve(root)
	if override != "" {
		return Command{Shell: override}, root
	}
	switch language {
	case "clojure":
		dir := nearestMarker(src, root, "deps.edn", "bb.edn")
		return Command{Argv: clojureCommand(dir)}, dir
	case "java":
		dir := nearestMarker(src, root, "pom.xml")
		return Command{Argv: []string{"mvn", "-q", "test", "-DexcludeTags=no-mutate"}}, dir
	case "go":
		dir := nearestMarker(src, root, "go.mod")
		return Command{Argv: []string{"go", "test", "-count=1", goPackage(dir, src)}}, dir
	case "typescript":
		dir := nearestMarker(src, root, "package.json")
		return Command{Argv: []string{"npm", "test"}}, dir
	case "rust":
		dir := nearestMarker(src, root, "Cargo.toml")
		return Command{Argv: []string{"cargo", "test"}}, dir
	case "python":
		dir := nearestMarker(src, root, "pyproject.toml", "pytest.ini", "setup.cfg")
		return Command{Argv: pythonCommand(dir)}, dir
	}
	return Command{Argv: []string{"false"}}, root
}
