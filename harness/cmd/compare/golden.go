package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Golden directory layout, one per case under <golden>/<tool>/<fixture>/<variant>/:
//
//	argv.txt      the upstream argv, one word per line
//	exit_code     decimal exit status
//	stdout.txt    raw stdout
//	stderr.txt    raw stderr
//	root.txt      the scratch directory the run used (normalised to <ROOT>)
//	meta.txt      recorded-at and duration, informational only
//	metrics/...   every file the run left under .metrics/, same relative paths

func goldenDir(base string, c Case) string {
	return filepath.Join(base, c.Tool, c.Fixture, c.Variant)
}

func writeGolden(dir string, r Run) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"argv.txt":   strings.Join(r.Argv, "\n") + "\n",
		"exit_code":  strconv.Itoa(r.Exit) + "\n",
		"stdout.txt": r.Stdout,
		"stderr.txt": r.Stderr,
		"root.txt":   r.Root + "\n",
		"meta.txt":   fmt.Sprintf("recorded-at %s\nduration %s\n", time.Now().UTC().Format(time.RFC3339), r.Duration.Round(time.Millisecond)),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	for rel, data := range r.Metrics {
		target := filepath.Join(dir, "metrics", strings.TrimPrefix(rel, ".metrics/"))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readGolden(dir string) (Run, error) {
	var r Run
	read := func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		return string(b), err
	}
	exit, err := read("exit_code")
	if err != nil {
		return r, fmt.Errorf("no golden at %s (run with -record first): %w", dir, err)
	}
	if r.Exit, err = strconv.Atoi(strings.TrimSpace(exit)); err != nil {
		return r, fmt.Errorf("%s/exit_code: %w", dir, err)
	}
	if r.Stdout, err = read("stdout.txt"); err != nil {
		return r, err
	}
	if r.Stderr, err = read("stderr.txt"); err != nil {
		return r, err
	}
	root, err := read("root.txt")
	if err != nil {
		return r, err
	}
	r.Root = strings.TrimSpace(root)
	argv, _ := read("argv.txt")
	r.Argv = strings.Fields(argv)
	r.Metrics = map[string][]byte{}
	mdir := filepath.Join(dir, "metrics")
	filepath.Walk(mdir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(mdir, path)
		r.Metrics[".metrics/"+filepath.ToSlash(rel)] = data
		return nil
	})
	return r, nil
}
