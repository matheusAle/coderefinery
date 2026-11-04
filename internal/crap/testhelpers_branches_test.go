package crap

import (
	"os"
	"path/filepath"
	"testing"
)

// scratchDir is t.TempDir() under the repo's .local/scratch when
// harness/env.sh is sourced (REPO_LOCAL), so test runs stay in the repo.
// Child processes (pytest, go test) inherit the TMPDIR too.
func scratchDir(t *testing.T) string {
	t.Helper()
	if local := os.Getenv("REPO_LOCAL"); local != "" {
		base := filepath.Join(local, "scratch", "gotest")
		if err := os.MkdirAll(base, 0o777); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TMPDIR", base)
	}
	return t.TempDir()
}
