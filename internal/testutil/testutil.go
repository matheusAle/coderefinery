// Package testutil holds the file helpers the package tests share. It
// imports nothing internal, so any package's own tests can use it.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// WriteFile writes content to path, creating the parent directories, and
// returns path. Any failure stops the test.
func WriteFile(t testing.TB, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

// ReadFile returns the content of path. Any failure stops the test.
func ReadFile(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Git runs git in dir and returns its output. Any failure stops the test.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// InitRepo makes dir a git repository with a committer identity; the test
// is skipped when git is not installed.
func InitRepo(t testing.TB, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	Git(t, dir, "init", "-q")
	Git(t, dir, "config", "user.email", "dev@example.com")
	Git(t, dir, "config", "user.name", "Dev")
	Git(t, dir, "config", "commit.gpgsign", "false")
}
