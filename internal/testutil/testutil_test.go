package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileCreatesParentsAndReadFileReturnsTheContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c.txt")
	if got := WriteFile(t, path, "héllo\n"); got != path {
		t.Fatalf("WriteFile returned %q, want %q", got, path)
	}
	if got := ReadFile(t, path); got != "héllo\n" {
		t.Fatalf("ReadFile = %q", got)
	}
	WriteFile(t, path, "")
	if info, err := os.Stat(path); err != nil || info.Size() != 0 {
		t.Fatalf("rewrite to empty: %v, %v", info, err)
	}
}
