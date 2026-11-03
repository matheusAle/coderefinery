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
)

func executable(t *testing.T, path string) string {
	t.Helper()
	writeFile(t, path, "#!/bin/sh\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNearestWalksUpToTheMarker(t *testing.T) {
	root := filepath.Join(tempRoot(t), "proj")
	writeFile(t, filepath.Join(root, "pyproject.toml"), "")
	src := writeFile(t, filepath.Join(root, "src", "demo", "app.py"), "x = 1\n")
	if nearest(src, "pyproject.toml", root) != root || nearest(filepath.Dir(src), "pyproject.toml", root) != root {
		t.Fatal("nearest")
	}
	if got := nearest(src, "missing.toml", root); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestClojureCommands(t *testing.T) {
	base := tempRoot(t)
	for dir, c := range map[string]struct {
		files map[string]string
		want  []string
	}{
		"bb-spec":   {map[string]string{"bb.edn": `{:tasks {spec "spec"}}`}, []string{"bb", "spec", "--tag", "~no-mutate"}},
		"bb-test":   {map[string]string{"bb.edn": `{:tasks {test (clojure "-M:test")}}`}, []string{"bb", "test"}},
		"spec":      {map[string]string{"deps.edn": "{:aliases {:spec {:extra-deps {speclj/speclj {}}}}}"}, []string{"clj", "-M:spec", "--tag", "~no-mutate"}},
		"spec-only": {map[string]string{"deps.edn": "{:aliases {:spec {}}}"}, []string{"clj", "-M:spec"}},
		"plain":     {map[string]string{"deps.edn": "{:deps {}}"}, []string{"clj", "-M:test"}},
		"both":      {map[string]string{"bb.edn": "{:tasks {test (clojure)}}", "deps.edn": "{:deps {}}"}, []string{"clj", "-M:test"}},
	} {
		for name, text := range c.files {
			writeFile(t, filepath.Join(base, dir, name), text)
		}
		if got := clojureCommand(filepath.Join(base, dir)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v want %v", dir, got, c.want)
		}
	}
}

func TestPythonCommandUsesAnAbsoluteProjectInterpreter(t *testing.T) {
	base := tempRoot(t)
	project := filepath.Join(base, "proj")
	dot := executable(t, filepath.Join(project, ".venv", "bin", "python"))
	writeFile(t, filepath.Join(project, "pytest.ini"), "[pytest]\n")
	if got := pythonCommand(project); !reflect.DeepEqual(got, []string{dot, "-m", "pytest"}) {
		t.Fatalf("got %v", got)
	}
	other := filepath.Join(base, "other")
	plain := executable(t, filepath.Join(other, "venv", "bin", "python"))
	if got := pythonCommand(other); !reflect.DeepEqual(got, []string{plain, "-m", "unittest", "discover"}) {
		t.Fatalf("got %v", got)
	}
	dot2 := executable(t, filepath.Join(other, ".venv", "bin", "python"))
	if got := pythonCommand(other); got[0] != dot2 {
		t.Fatalf(".venv wins over venv: %v", got)
	}

	linked := filepath.Join(base, "linked")
	target := executable(t, filepath.Join(linked, "real-python"))
	link := filepath.Join(linked, ".venv", "bin", "python")
	_ = os.MkdirAll(filepath.Dir(link), 0o777)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if got := pythonCommand(linked); got[0] != link {
		t.Fatalf("the venv symlink is kept: %v", got)
	}

	broken := filepath.Join(base, "broken")
	writeFile(t, filepath.Join(broken, ".venv", "bin", "python"), "")
	if got := pythonCommand(broken); got[0] != selfPython() || got[len(got)-1] != "discover" {
		t.Fatalf("a non-executable venv falls back: %v", got)
	}
}

func TestPythonCommandsPickPytestOrUnittest(t *testing.T) {
	base := tempRoot(t)
	for dir, c := range map[string]struct {
		file, text string
		pytest     bool
	}{
		"py":   {"pytest.ini", "[pytest]\n", true},
		"unit": {"README", "", false},
		"cfg":  {"setup.cfg", "[tool:pytest]\n", true},
		"cfg2": {"setup.cfg", "[metadata]\n", false},
		"proj": {"pyproject.toml", "[project]\ndependencies=['pytest']\n", true},
		"conf": {"conftest.py", "", true},
	} {
		writeFile(t, filepath.Join(base, dir, c.file), c.text)
		got := pythonCommand(filepath.Join(base, dir))
		if (got[len(got)-1] == "pytest") != c.pytest {
			t.Errorf("%s: %v", dir, got)
		}
	}
}

func TestTestPlansFollowTheNearestProject(t *testing.T) {
	root := filepath.Join(tempRoot(t), "proj")
	sub := filepath.Join(root, "svc")
	writeFile(t, filepath.Join(sub, "bb.edn"), "{:tasks {test (clojure)}}\n")
	writeFile(t, filepath.Join(sub, "pom.xml"), "<project/>\n")
	writeFile(t, filepath.Join(sub, "package.json"), "{}\n")
	writeFile(t, filepath.Join(sub, "Cargo.toml"), "[package]\nname='demo'\n")
	writeFile(t, filepath.Join(sub, "go.mod"), "module example.com/demo\n")
	writeFile(t, filepath.Join(sub, "pyproject.toml"), "[project]\ndependencies=['pytest']\n")
	cases := []struct {
		rel, language string
		want          []string
	}{
		{"src/a.clj", "clojure", []string{"bb", "test"}},
		{"src/A.java", "java", []string{"mvn", "-q", "test", "-DexcludeTags=no-mutate"}},
		{"src/a.ts", "typescript", []string{"npm", "test"}},
		{"src/lib.rs", "rust", []string{"cargo", "test"}},
		{"pkg/widget.go", "go", []string{"go", "test", "-count=1", "./pkg"}},
		{"main.go", "go", []string{"go", "test", "-count=1", "."}},
		{"app.py", "python", nil},
	}
	for _, c := range cases {
		src := writeFile(t, filepath.Join(sub, c.rel), "x\n")
		command, dir := testPlan(root, src, c.language, "")
		if dir != sub {
			t.Errorf("%s: dir %s", c.rel, dir)
		}
		if c.want != nil && !reflect.DeepEqual(command.Argv, c.want) {
			t.Errorf("%s: %v", c.rel, command.Argv)
		}
		if c.want == nil && strings.Join(command.Argv[len(command.Argv)-2:], " ") != "-m pytest" {
			t.Errorf("%s: %v", c.rel, command.Argv)
		}
	}
	if command, dir := testPlan(root, filepath.Join(sub, "x.cobol"), "cobol", ""); !reflect.DeepEqual(command.Argv, []string{"false"}) || dir != root {
		t.Error("unknown language runs false at the root")
	}
	override, dir := testPlan(root, filepath.Join(sub, "app.py"), "python", "go test ./...")
	if override.Shell != "go test ./..." || override.Argv != nil || dir != root {
		t.Error("an override is a shell string at the root")
	}
}

func TestAGoPackagePathIsOneArgument(t *testing.T) {
	root := filepath.Join(tempRoot(t), "proj")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	src := writeFile(t, filepath.Join(root, "a;echo no", "widget.go"), "package main\n")
	command, dir := testPlan(root, src, "go", "")
	if !reflect.DeepEqual(command.Argv, []string{"go", "test", "-count=1", "./a;echo no"}) || dir != root {
		t.Fatalf("plan %v %s", command.Argv, dir)
	}
	if command.display() != "go test -count=1 './a;echo no'" {
		t.Fatalf("display %q", command.display())
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
	if res := r.Run(Command{Argv: []string{"definitely-not-a-command-ubgate"}}, dir, 0); res.Code != 127 ||
		res.Output != "[Errno 2] No such file or directory: 'definitely-not-a-command-ubgate'" {
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
		res := (&CommandRunner{}).Run(Command{Shell: py + " -c 'import demo; print(demo.VALUE)'"}, worker, 5*time.Second)
		if res.Code != 0 || strings.TrimSpace(res.Output) != "worker" {
			t.Fatalf("python import %+v", res)
		}
	}
}
