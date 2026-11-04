package treediff

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/testutil"
)

func TestFindRootPrefersREPO_ROOTThenWalksUpToTheUpstreamVenv(t *testing.T) {
	t.Setenv("REPO_ROOT", "/somewhere")
	if got, err := FindRoot("/elsewhere"); got != "/somewhere" || err != nil {
		t.Fatalf("FindRoot with REPO_ROOT = %q, %v", got, err)
	}
	t.Setenv("REPO_ROOT", "")
	root := t.TempDir()
	testutil.WriteFile(t, filepath.Join(root, "upstream/crapper/.venv/bin/python"), "")
	deep := filepath.Join(root, "internal", "lang")
	if got, err := FindRoot(deep); got != root || err != nil {
		t.Fatalf("FindRoot(%s) = %q, %v; want %q", deep, got, err, root)
	}
	if got, err := FindRoot(filepath.Dir(root)); got != "" || err == nil || err.Error() != "cannot find the repo root (set REPO_ROOT)" {
		t.Fatalf("FindRoot without a venv = %q, %v", got, err)
	}
	if cfg := DefaultConfig(root); cfg.Python != filepath.Join(root, "upstream/crapper/.venv/bin/python") || cfg.Oracle != filepath.Join(root, "tools/oracle/oracle.py") {
		t.Fatalf("DefaultConfig = %+v", cfg)
	}
}

func TestCollectKeepsParsableFilesAndSkipsToolDirs(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"b.py", "a/c.go", "a/notes.txt", "node_modules/x.js", ".venv/y.py", "z.rb"} {
		testutil.WriteFile(t, filepath.Join(root, rel), "")
	}
	got, err := Collect([]string{root, filepath.Join(root, "z.rb")}, "")
	want := []string{filepath.Join(root, "a/c.go"), filepath.Join(root, "b.py"), filepath.Join(root, "z.rb")}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect = %v, %v; want %v (a named file is kept as is)", got, err, want)
	}
	forced, _ := Collect([]string{filepath.Join(root, "a")}, "go")
	if want := []string{filepath.Join(root, "a/c.go"), filepath.Join(root, "a/notes.txt")}; !reflect.DeepEqual(forced, want) {
		t.Fatalf("forced grammar takes every file: %v", forced)
	}
	if _, err := Collect([]string{filepath.Join(root, "missing")}, ""); err == nil {
		t.Fatal("a missing path is an error")
	}
}

func TestOursDumpsOurParse(t *testing.T) {
	path := testutil.WriteFile(t, filepath.Join(t.TempDir(), "m.py"), "x = 1\n")
	got, err := Ours(path, "")
	if err != nil || !strings.HasPrefix(got, "module [0,6) 0:0-1:0\n") || !strings.Contains(got, `identifier [0,1) 0:0-0:1 "x"`) {
		t.Fatalf("Ours = %q, %v", got, err)
	}
	if _, err := Ours(path, "nosuchgrammar"); err == nil {
		t.Fatal("an unknown grammar is an error")
	}
}

func TestCompareReportsTheFirstDifferingLineWithContext(t *testing.T) {
	if line, report := Compare("a\nb\n", "a\nb\n"); line != 0 || report != "" {
		t.Fatalf("equal dumps: %d %q", line, report)
	}
	line, report := Compare("1\n2\n3\n4\n5\nours\n", "1\n2\n3\n4\n5\ntheirs\n")
	if want := "    2\n    3\n    4\n    5\n  ours:   ours\n  theirs: theirs\n"; line != 6 || report != want {
		t.Fatalf("Compare = %d %q, want 6 %q", line, report, want)
	}
	if line, report := Compare("a", "a\nmore"); line != 2 || report != "    a\n  ours:   <end>\n  theirs: more\n" {
		t.Fatalf("shorter ours: %d %q", line, report)
	}
}

// skipT records Skip and stops Parity the way testing.T does (Goexit there,
// a panic here).
type skipT struct{ skipped string }

func (s *skipT) Helper()                      {}
func (s *skipT) Skip(args ...any)             { s.skipped = fmt.Sprint(args...); panic(s) }
func (s *skipT) Fatal(args ...any)            { panic("Fatal: " + fmt.Sprint(args...)) }
func (s *skipT) Fatalf(f string, args ...any) { panic("Fatalf: " + fmt.Sprintf(f, args...)) }
func (s *skipT) Errorf(f string, args ...any) { panic("Errorf: " + fmt.Sprintf(f, args...)) }

func TestParitySkipsWithoutTheUpstreamVenv(t *testing.T) {
	t.Setenv("REPO_ROOT", t.TempDir())
	st := &skipT{}
	func() {
		defer func() {
			if r := recover(); r != st {
				t.Fatalf("want a Skip, got %v", r)
			}
		}()
		Parity(st, Corpus{Dirs: []string{"testdata/python"}, Min: 1})
	}()
	if st.skipped != "upstream venv not found" {
		t.Fatalf("skip reason %q", st.skipped)
	}
	if _, err := os.Stat(DefaultConfig(os.Getenv("REPO_ROOT")).Python); err == nil {
		t.Fatal("the temp root must have no venv")
	}
}
