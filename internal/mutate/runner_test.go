package mutate

// Ported from upstream tests/test_runner.py and the command parts of
// tests/test_boundaries.py.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/matheusAle/coderefinery/internal/testplan"
)

func TestAGoPackagePathIsOneArgument(t *testing.T) {
	root := filepath.Join(tempRoot(t), "proj")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	src := writeFile(t, filepath.Join(root, "a;echo no", "widget.go"), "package main\n")
	command, dir := testplan.Plan(root, src, "go", "")
	if !reflect.DeepEqual(command.Argv, []string{"go", "test", "-count=1", "./a;echo no"}) || dir != root {
		t.Fatalf("plan %v %s", command.Argv, dir)
	}
	if command.Display() != "go test -count=1 './a;echo no'" {
		t.Fatalf("display %q", command.Display())
	}
	echoed := (&CommandRunner{}).Run(Command{Argv: []string{"/bin/echo", command.Argv[3]}}, dir, 0)
	if echoed.Code != 0 || strings.TrimSpace(echoed.Output) != "./a;echo no" {
		t.Fatalf("echo %+v", echoed)
	}
}

func TestCommandRunnerCodesOutputAndTimeouts(t *testing.T) {
	dir := tempRoot(t)
	r := &CommandRunner{}
	if res := r.Run(Command{Shell: "printf 'a\\r\\nb'; echo err >&2; exit 3"}, dir, 0); res.Code != 3 || res.Output != "a\nberr\n" || res.TimedOut {
		t.Fatalf("shell %+v", res)
	}
	started := time.Now()
	res := r.Run(Command{Shell: "sleep 30"}, dir, 400*time.Millisecond)
	if !res.TimedOut || res.Code != 124 || time.Since(started) > 10*time.Second {
		t.Fatalf("timeout %+v", res)
	}
	if res := r.Run(Command{Argv: []string{"definitely-not-a-command-coderefinery"}}, dir, 0); res.Code != 127 ||
		res.Output != "[Errno 2] No such file or directory: 'definitely-not-a-command-coderefinery'" {
		t.Fatalf("spawn failure %+v", res)
	}
	if res := r.Run(Command{Shell: "kill -9 $$"}, dir, 0); res.Code != -9 {
		t.Fatalf("a signal is -N: %+v", res)
	}
	if res := r.Run(Command{Shell: "echo $PYTHONDONTWRITEBYTECODE"}, dir, 0); strings.TrimSpace(res.Output) != "1" {
		t.Fatalf("env %+v", res)
	}
	var stderr strings.Builder
	(&CommandRunner{Verbose: true, Stderr: &stderr}).Run(Command{Argv: []string{"true", "a b"}}, dir, 0)
	if stderr.String() != "+ ("+dir+") true 'a b'\n" {
		t.Fatalf("verbose %q", stderr.String())
	}
}

func TestAWorkerOverlayIsImportedAheadOfTheEnvironment(t *testing.T) {
	root := tempRoot(t)
	worker := filepath.Join(root, "target", "mutation-workers", "run-1", "worker-0")
	writeFile(t, filepath.Join(worker, "src", "demo.py"), "VALUE = 'worker'\n")
	if workerHome(filepath.Join(worker, "src")) != worker || workerHome(root) != "" {
		t.Fatal("workerHome")
	}
	env := commandEnv(worker)
	want := "PYTHONPATH=" + filepath.Join(worker, "src") + ":" + worker
	if os.Getenv("PYTHONPATH") != "" {
		want += ":" + os.Getenv("PYTHONPATH")
	}
	if !containsString(env, want) {
		t.Fatalf("env lacks %s", want)
	}
	if py, err := exec.LookPath("python3"); err == nil {
		res := (&CommandRunner{}).Run(Command{Shell: py + " -c 'import demo; print(demo.VALUE)'"}, worker, 120*time.Second) // a hang guard, not a speed check
		if res.Code != 0 || strings.TrimSpace(res.Output) != "worker" {
			t.Fatalf("python import %+v", res)
		}
	}
}
