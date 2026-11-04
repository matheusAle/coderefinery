package discover

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/matheusAle/coderefinery/internal/testutil"
)

func TestChangedFilesListsAddedAndModifiedFilesBelowTheRoot(t *testing.T) {
	repo := Resolve(t.TempDir())
	testutil.InitRepo(t, repo)
	testutil.WriteFile(t, filepath.Join(repo, "sub/kept.py"), "x = 1\n")
	testutil.WriteFile(t, filepath.Join(repo, "sub/gone.py"), "x = 1\n")
	testutil.WriteFile(t, filepath.Join(repo, "same.py"), "x = 1\n")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-q", "-m", "base")
	testutil.WriteFile(t, filepath.Join(repo, "sub/kept.py"), "x = 2\n")      // modified
	testutil.WriteFile(t, filepath.Join(repo, "sub/new/added.py"), "y = 1\n") // untracked, in a new dir
	testutil.WriteFile(t, filepath.Join(repo, "outside.py"), "z = 1\n")       // above the root
	testutil.Git(t, repo, "rm", "-q", "sub/gone.py")                          // deleted: not a file
	got, err := ChangedFiles(filepath.Join(repo, "sub"))
	want := []string{filepath.Join(repo, "sub/kept.py"), filepath.Join(repo, "sub/new/added.py")}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedFiles = %v, %v; want %v", got, err, want)
	}
	if got, _ := ChangedFiles(repo); len(got) != 3 {
		t.Fatalf("from the top: %v", got)
	}
}

func TestChangedFilesOutsideARepoIsAGitError(t *testing.T) {
	testutil.InitRepo(t, t.TempDir()) // skips without git
	root := Resolve(t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	_, err := ChangedFiles(root)
	var gerr *GitError
	if !errors.As(err, &gerr) || gerr.Code != 128 || gerr.Error() != gerr.Message || gerr.Message == "" {
		t.Fatalf("ChangedFiles outside a repo = %v (%#v)", err, gerr)
	}
}
