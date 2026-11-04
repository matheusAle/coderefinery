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

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
	"github.com/matheusAle/coderefinery/internal/source"
	"github.com/matheusAle/coderefinery/internal/testplan"
)

// Command is a test command: a shell string or an argument vector
// (internal/testplan).
type Command = testplan.Command

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
		fmt.Fprintf(r.Stderr, "+ (%s) %s\n", cwd, command.Display())
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
	cmd.Env = append(commandEnv(cwd), command.Env...)
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
