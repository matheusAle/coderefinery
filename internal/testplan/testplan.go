// Package testplan decides how a project's tests run for one source file
// (upstream mutator/runner.py test_plan): the command and its working
// directory. mutate runs the plan; `coderefinery test` reuses its language
// markers and cwd rules.
package testplan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
)

// Command is a shell string (Shell != "", from --test-command or
// --coverage-command) or an argument vector.
type Command struct {
	Shell string
	Argv  []string
	Env   []string // extra KEY=VALUE for the child (mutate --attribute reporters); nil for upstream plans
}

// Display is runner.display_command.
func (c Command) Display() string {
	var prefix string
	for _, kv := range c.Env {
		prefix += coverage.ShellQuote(kv) + " "
	}
	if c.Argv == nil {
		return prefix + c.Shell
	}
	quoted := make([]string, len(c.Argv))
	for i, a := range c.Argv {
		quoted[i] = coverage.ShellQuote(a)
	}
	return prefix + strings.Join(quoted, " ")
}

// Key is engine._command_key (plus the cwd, as the baseline cache uses it).
func (c Command) Key(cwd string) string {
	env := ""
	if len(c.Env) > 0 {
		env = "\x02" + strings.Join(c.Env, "\x00")
	}
	if c.Argv == nil {
		return "sh\x00" + c.Shell + "\x01" + cwd + env
	}
	return "argv\x00" + strings.Join(c.Argv, "\x00") + "\x01" + cwd + env
}

// Nearest is runner.nearest: the first dir from start (a file's parent)
// up to stop that holds marker as a file; "" when none.
func Nearest(start, marker, stop string) string {
	current := start
	if !discover.IsDir(start) {
		current = filepath.Dir(start)
	}
	stop = discover.Resolve(stop)
	current = discover.Resolve(current)
	for {
		if discover.IsFile(filepath.Join(current, marker)) {
			return current
		}
		parent := filepath.Dir(current)
		if current == stop || parent == current {
			return ""
		}
		current = parent
	}
}

// ReadOrEmpty is a file's text, "" when it cannot be read.
func ReadOrEmpty(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

// ClojureCommand is runner._clojure_command.
func ClojureCommand(dir string) []string {
	deps := filepath.Join(dir, "deps.edn")
	bb := filepath.Join(dir, "bb.edn")
	if discover.IsFile(bb) && !discover.IsFile(deps) {
		if strings.Contains(ReadOrEmpty(bb), "spec") {
			return []string{"bb", "spec", "--tag", "~no-mutate"}
		}
		return []string{"bb", "test"}
	}
	text := ReadOrEmpty(deps)
	switch {
	case strings.Contains(text, ":spec") && strings.Contains(text, "speclj"):
		return []string{"clj", "-M:spec", "--tag", "~no-mutate"}
	case strings.Contains(text, ":spec"):
		return []string{"clj", "-M:spec"}
	}
	return []string{"clj", "-M:test"}
}

// SelfPython stands in for upstream's sys.executable: python3 on PATH.
func SelfPython() string {
	if p, err := exec.LookPath("python3"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	return "python3"
}

// ProjectInterpreter is runner._project_interpreter (absolute, not resolved).
func ProjectInterpreter(dir string) string {
	for _, name := range []string{".venv", "venv"} {
		cand := filepath.Join(dir, name, "bin", "python")
		if abs, err := filepath.Abs(cand); err == nil {
			cand = abs
		}
		if discover.IsFile(cand) && syscall.Access(cand, 1) == nil {
			return cand
		}
	}
	return SelfPython()
}

// UsesPytest is runner's pytest detection for a Python project dir.
func UsesPytest(dir string) bool {
	return discover.IsFile(filepath.Join(dir, "pytest.ini")) || discover.IsFile(filepath.Join(dir, "conftest.py")) ||
		strings.Contains(ReadOrEmpty(filepath.Join(dir, "pyproject.toml")), "pytest") ||
		discover.IsFile(filepath.Join(dir, "setup.cfg")) && strings.Contains(ReadOrEmpty(filepath.Join(dir, "setup.cfg")), "pytest")
}

// PythonCommand is runner._python_command.
func PythonCommand(dir string) []string {
	py := ProjectInterpreter(dir)
	if UsesPytest(dir) {
		return []string{py, "-m", "pytest"}
	}
	return []string{py, "-m", "unittest", "discover"}
}

// NearestMarker is the first marker's Nearest dir, root when none is found.
func NearestMarker(src, root string, markers ...string) string {
	for _, m := range markers {
		if found := Nearest(src, m, root); found != "" {
			return found
		}
	}
	return root
}

// GoPackage is the `go test` package argument for src under module dir.
func GoPackage(dir, src string) string {
	rel, ok := discover.RelativeTo(filepath.Dir(discover.Resolve(src)), discover.Resolve(dir))
	if !ok || rel == "." {
		return "."
	}
	return "./" + rel
}

// Markers are the project markers per language, in lookup order.
var Markers = map[string][]string{
	"clojure":    {"deps.edn", "bb.edn"},
	"java":       {"pom.xml"},
	"go":         {"go.mod"},
	"typescript": {"package.json"},
	"rust":       {"Cargo.toml"},
	"python":     {"pyproject.toml", "pytest.ini", "setup.cfg"},
}

// Plan is runner.test_plan: the command and its working directory.
func Plan(root, src, language, override string) (Command, string) {
	root = discover.Resolve(root)
	if override != "" {
		return Command{Shell: override}, root
	}
	markers, ok := Markers[language]
	if !ok {
		return Command{Argv: []string{"false"}}, root
	}
	dir := NearestMarker(src, root, markers...)
	switch language {
	case "clojure":
		return Command{Argv: ClojureCommand(dir)}, dir
	case "java":
		return Command{Argv: []string{"mvn", "-q", "test", "-DexcludeTags=no-mutate"}}, dir
	case "go":
		return Command{Argv: []string{"go", "test", "-count=1", GoPackage(dir, src)}}, dir
	case "typescript":
		return Command{Argv: []string{"npm", "test"}}, dir
	case "rust":
		return Command{Argv: []string{"cargo", "test"}}, dir
	}
	return Command{Argv: PythonCommand(dir)}, dir
}
