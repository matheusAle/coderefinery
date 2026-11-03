package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"
	"time"
)

// Run is everything one invocation produced.
type Run struct {
	Argv     []string
	Root     string // the scratch copy of the fixture the command ran in
	Exit     int
	Stdout   string
	Stderr   string
	Metrics  map[string][]byte // path relative to Root, e.g. ".metrics/crap.edn"
	Err      string            // start failure or timeout; empty on a normal exit
	Duration time.Duration
}

// copyFixture copies src to dst (dst must not exist), preserving symlinks.
// On macOS it uses APFS clones (cp -c), which is instant for node_modules.
func copyFixture(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		if err := exec.Command("cp", "-cR", src, dst).Run(); err == nil {
			return nil
		}
		os.RemoveAll(dst) // fall back to a plain copy

	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		default:
			return copyFile(path, target, info.Mode().Perm())
		}
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// execute copies the fixture to root and runs argv there.
func execute(argv []string, fixtureDir, root string, timeout time.Duration, verbose bool) Run {
	r := Run{Argv: argv, Root: root, Exit: -1}
	if err := copyFixture(fixtureDir, root); err != nil {
		r.Err = "copy fixture: " + err.Error()
		return r
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "+ (%s) %q\n", root, argv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	r.Duration = time.Since(start)
	r.Stdout = stdout.String()
	r.Stderr = stderr.String()
	if cmd.ProcessState != nil {
		r.Exit = cmd.ProcessState.ExitCode()
	}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		r.Err = fmt.Sprintf("timed out after %s", timeout)
	case err != nil && !errors.As(err, &exitErr):
		r.Err = err.Error()
	case r.Exit < 0:
		r.Err = "terminated by signal: " + err.Error()
	}
	r.Metrics = readMetrics(root)
	return r
}

// readMetrics collects every regular file under root/.metrics.
func readMetrics(root string) map[string][]byte {
	files := map[string][]byte{}
	base := filepath.Join(root, ".metrics")
	filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	return files
}

func sortedNames(m map[string][]byte) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
