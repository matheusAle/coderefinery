// Package testutil holds the file helpers the package tests share. It
// imports nothing internal, so any package's own tests can use it.
package testutil

import (
	"os"
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
