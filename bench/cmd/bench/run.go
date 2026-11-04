package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Result is one timed execution (one JSON line in results.jsonl).
type Result struct {
	Workload string   `json:"workload"` // crap | dry | mutate-scan | help | ...
	Repo     string   `json:"repo"`     // corpus repo or fixture name
	Impl     string   `json:"impl"`     // upstream | ubgate
	Variant  string   `json:"variant,omitempty"`
	Batch    string   `json:"batch"` // one id per `bench run` invocation; report keeps the latest batch
	Run      int      `json:"run"`   // 0 = warm-up
	Warmup   bool     `json:"warmup"`
	Command  []string `json:"command"`
	Dir      string   `json:"dir"`
	Started  string   `json:"started"`

	WallSec float64 `json:"wall_s"`
	UserSec float64 `json:"user_s"`
	SysSec  float64 `json:"sys_s"`
	CPUSec  float64 `json:"cpu_s"` // user + sys, from wait4 rusage of the child
	// MaxRSS is the peak resident set of the single largest process in the
	// waited-for tree (rusage semantics), normalised to bytes.
	MaxRSS int64 `json:"maxrss_bytes"`
	// ChildCPUSec is the RUSAGE_CHILDREN delta of this runner around the run:
	// a cross-check of CPUSec. Both only see descendants that were waited for.
	ChildCPUSec float64 `json:"children_cpu_s"`

	Exit     int    `json:"exit"`
	TimedOut bool   `json:"timed_out"`
	Timeout  string `json:"timeout"`
	// Single is true when the warm-up exceeded -slow and was kept as the only
	// measured run.
	Single bool `json:"single,omitempty"`

	Load1      float64 `json:"load1"`
	Load5      float64 `json:"load5"`
	LoadWaited float64 `json:"load_waited_s"`
	StdoutPath string  `json:"stdout,omitempty"`
}

func runMain(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var (
		workload = fs.String("workload", "", "workload label (crap, dry, mutate-scan, help, ...)")
		repo     = fs.String("repo", "", "repo / fixture label")
		impl     = fs.String("impl", "", "implementation label (upstream, ubgate)")
		variant  = fs.String("variant", "", "optional variant label (e.g. cold, warm)")
		dir      = fs.String("dir", ".", "working directory of the command")
		runs     = fs.Int("runs", 3, "measured runs")
		warmup   = fs.Int("warmup", 1, "warm-up runs (recorded with warmup=true, excluded from stats)")
		timeout  = fs.Duration("timeout", 30*time.Minute, "per-run timeout; the process group is killed")
		slow     = fs.Duration("slow", 0, "if the first warm-up takes longer than this, keep it as the only measured run (0 = off)")
		pre      = fs.String("pre", "", "shell command run (untimed, in bench's own cwd, not -dir) before every run, e.g. 'rm -f DIR/.metrics/crap.edn'")
		fresh    = fs.String("fresh", "", "clone this directory before every run into "+freshRoot+"/<batch>/run-<n>/<basename> and run there (replaces -dir; nothing is deleted)")
		dirOut   = fs.String("dir-out", "", "write the last run's working directory to this file")
		out      = fs.String("out", "", "append JSON lines here (default stdout)")
		stdout   = fs.String("stdout", "", "save the last run's stdout here (default: discarded)")
		stderr   = fs.String("stderr", "", "save the last run's stderr here (default: discarded)")
		maxLoad  = fs.Float64("max-load", 4, "wait before each run while the 1-min load average is above this (0 = never wait)")
		loadWait = fs.Duration("load-wait", 3*time.Minute, "give up waiting for load after this long and run anyway")
		waitEach = fs.Bool("wait-each", false, "wait for load before every run (default: only before the first run of the case)")
		quiet    = fs.Bool("q", false, "do not print a line per run to stderr")
	)
	fs.Parse(args)
	argv := fs.Args()
	if len(argv) == 0 {
		return errors.New("run: no command (use: bench run [flags] -- cmd args...)")
	}
	absDir, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	var sink io.Writer = os.Stdout
	if *out != "" {
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(*out, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		sink = f
	}
	enc := json.NewEncoder(sink)
	batch := fmt.Sprintf("%s-%d", time.Now().UTC().Format("20060102T150405.000"), os.Getpid())

	total := *warmup + *runs
	for i := 0; i < total; i++ {
		isWarm := i < *warmup
		if *fresh != "" {
			d, err := freshCopy(*fresh, batch, i)
			if err != nil {
				return err
			}
			absDir = d
		}
		if *dirOut != "" {
			if err := os.WriteFile(*dirOut, []byte(absDir+"\n"), 0o644); err != nil {
				return err
			}
		}
		if *pre != "" {
			c := exec.Command("/bin/sh", "-c", *pre)
			c.Stdout, c.Stderr = os.Stderr, os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("pre command %q: %w", *pre, err)
			}
		}
		var waited time.Duration
		if i == 0 || *waitEach {
			waited = waitForLoad(*maxLoad, *loadWait)
		}
		l1, l5 := loadAvg()
		last := i == total-1
		// Every run writes its streams to the targets; the last run kept wins.
		r, err := timeOnce(argv, absDir, *timeout, *stdout, *stderr)
		if err != nil {
			return err
		}
		r.Workload, r.Repo, r.Impl, r.Variant, r.Batch = *workload, *repo, *impl, *variant, batch
		r.Run, r.Warmup = i-*warmup+1, isWarm
		r.Load1, r.Load5, r.LoadWaited = l1, l5, waited.Seconds()
		if isWarm && i == 0 && *slow > 0 && time.Duration(r.WallSec*float64(time.Second)) > *slow {
			// Too slow to repeat: keep this run as the single measurement.
			r.Warmup, r.Single, r.Run = false, true, 1
			r.StdoutPath = *stdout
			if err := enc.Encode(r); err != nil {
				return err
			}
			report(*quiet, r)
			break
		}
		if last && *stdout != "" {
			r.StdoutPath = *stdout
		}
		if err := enc.Encode(r); err != nil {
			return err
		}
		report(*quiet, r)
	}
	return nil
}

// freshRoot is where -fresh clones go, relative to bench's cwd (run.sh runs
// from the ub-mode root, so .local/bench/runs, which run.sh clears with one
// literal rm). The runner itself never deletes anything.
const freshRoot = ".local/bench/runs"

// freshCopy clones src into a new directory freshRoot/<batch>/run-<i>/<base>
// (an APFS clone on macOS, a plain copy elsewhere) and returns its absolute
// path. The last component stays the source's name, so outputs that mention
// the project directory name match across implementations.
func freshCopy(src, batch string, i int) (string, error) {
	parent, err := filepath.Abs(filepath.Join(freshRoot, batch, fmt.Sprintf("run-%d", i)))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(parent, filepath.Base(src))
	if _, err := os.Lstat(dst); err == nil {
		return "", fmt.Errorf("fresh copy target %s already exists", dst)
	}
	if err := exec.Command("cp", "-c", "-R", src, dst).Run(); err != nil {
		if _, statErr := os.Lstat(dst); statErr == nil {
			return "", fmt.Errorf("clone %s -> %s failed half-way: %w", src, dst, err)
		}
		// no clonefile support (not APFS): plain copy
		if err := exec.Command("cp", "-R", src, dst).Run(); err != nil {
			return "", fmt.Errorf("copy %s -> %s: %w", src, dst, err)
		}
	}
	return dst, nil
}

func report(quiet bool, r Result) {
	if quiet {
		return
	}
	tag := fmt.Sprintf("run %d", r.Run)
	if r.Warmup {
		tag = "warm-up"
	}
	if r.Single {
		tag = "single (slow)"
	}
	extra := ""
	if r.TimedOut {
		extra = " TIMEOUT"
	}
	fmt.Fprintf(os.Stderr, "  %-12s %-11s %-8s %-13s wall %8.2fs  cpu %8.2fs  rss %7.1f MiB  exit %d  load %.1f%s\n",
		r.Workload, r.Repo, r.Impl, tag, r.WallSec, r.CPUSec, float64(r.MaxRSS)/(1<<20), r.Exit, r.Load1, extra)
}

func timeOnce(argv []string, dir string, timeout time.Duration, stdoutPath, stderrPath string) (Result, error) {
	r := Result{Command: argv, Dir: dir, Timeout: timeout.String()}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	so, err := openSink(stdoutPath)
	if err != nil {
		return r, err
	}
	defer so.Close()
	se, err := openSink(stderrPath)
	if err != nil {
		return r, err
	}
	defer se.Close()
	cmd.Stdout, cmd.Stderr = so, se

	var before syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_CHILDREN, &before)
	start := time.Now()
	r.Started = start.UTC().Format(time.RFC3339)
	if err := cmd.Start(); err != nil {
		return r, fmt.Errorf("start %v: %w", argv, err)
	}
	var killed atomic.Bool
	timer := time.AfterFunc(timeout, func() {
		killed.Store(true)
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	waitErr := cmd.Wait()
	timer.Stop()
	r.TimedOut = killed.Load()
	r.WallSec = time.Since(start).Seconds()
	var after syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_CHILDREN, &after)

	if ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok && ru != nil {
		r.UserSec = tv(ru.Utime)
		r.SysSec = tv(ru.Stime)
		r.CPUSec = r.UserSec + r.SysSec
		r.MaxRSS = rssBytes(int64(ru.Maxrss))
	}
	r.ChildCPUSec = tv(after.Utime) + tv(after.Stime) - tv(before.Utime) - tv(before.Stime)
	r.Exit = cmd.ProcessState.ExitCode()
	if waitErr != nil {
		var ee *exec.ExitError
		if !errors.As(waitErr, &ee) {
			return r, waitErr
		}
	}
	return r, nil
}

func openSink(p string) (io.WriteCloser, error) {
	if p == "" {
		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		return f, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return os.Create(p)
}

func tv(t syscall.Timeval) float64 { return float64(t.Sec) + float64(t.Usec)/1e6 }

// rssBytes normalises ru_maxrss: bytes on macOS (and iOS), KiB on Linux and
// the BSDs.
func rssBytes(v int64) int64 {
	switch runtime.GOOS {
	case "darwin", "ios":
		return v
	default:
		return v * 1024
	}
}

// loadAvg returns the 1- and 5-minute load averages (0, 0 if unknown).
func loadAvg() (float64, float64) {
	var text string
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		text = string(b)
	} else if b, err := exec.Command("sysctl", "-n", "vm.loadavg").Output(); err == nil {
		text = strings.Trim(strings.TrimSpace(string(b)), "{}")
	}
	f := strings.Fields(text)
	if len(f) < 2 {
		return 0, 0
	}
	l1, _ := strconv.ParseFloat(f[0], 64)
	l5, _ := strconv.ParseFloat(f[1], 64)
	return l1, l5
}

func waitForLoad(max float64, budget time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	start := time.Now()
	for {
		l1, _ := loadAvg()
		if l1 <= max || time.Since(start) >= budget {
			return time.Since(start)
		}
		time.Sleep(10 * time.Second)
	}
}
