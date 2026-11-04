package mutate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/lang"
)

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// tempRoot is a resolved temp dir (macOS /var -> /private/var).
func tempRoot(t *testing.T) string { return discover.Resolve(t.TempDir()) }

// sitesOf is the upstream test helper _sites: write, then sites_in_file.
func sitesOf(t *testing.T, root, relative, source string) []Site {
	t.Helper()
	path := writeFile(t, filepath.Join(root, relative), source)
	return newFileUnit(source, path, root, relative).sites()
}

type pairT struct{ orig, mutant string }

func pairsOf(sites []Site) map[pairT]bool {
	out := map[pairT]bool{}
	for _, s := range sites {
		out[pairT{s.Original, s.Mutant}] = true
	}
	return out
}

func namespacesOf(sites []Site) []string {
	set := map[string]bool{}
	for _, s := range sites {
		set[s.Namespace] = true
	}
	var out []string
	for ns := range set {
		out = append(out, ns)
	}
	sort.Strings(out)
	return out
}

// needGrammar skips a test until that language's parser has landed.
func needGrammar(t *testing.T, grammars ...string) {
	t.Helper()
	for _, g := range grammars {
		if !lang.Supported(g) {
			t.Skipf("no %s parser registered yet", g)
		}
	}
}

// needReader skips a test until crapper's function reader for the
// language is registered (sites need owners).
func needReader(t *testing.T, language string) {
	t.Helper()
	if _, ok := crap.Readers[language]; !ok {
		t.Skipf("no crapper %s reader registered yet", language)
	}
}

// fakeRunner records calls; decide returns the exit code for one run.
type fakeRunner struct {
	mu       sync.Mutex
	calls    int
	timeouts []time.Duration
	cwds     []string
	seconds  float64
	output   string
	decide   func(cwd string, timeout time.Duration) int
}

func (f *fakeRunner) IsVerbose() bool { return false }

func (f *fakeRunner) Run(_ Command, cwd string, timeout time.Duration) CommandResult {
	f.mu.Lock()
	f.calls++
	f.timeouts = append(f.timeouts, timeout)
	f.cwds = append(f.cwds, cwd)
	f.mu.Unlock()
	code := 0
	if f.decide != nil {
		code = f.decide(cwd, timeout)
	}
	seconds := f.seconds
	if seconds == 0 {
		seconds = 0.01
	}
	return CommandResult{Code: code, Seconds: seconds, Output: f.output}
}

const addSource = "def add(a, b):\n    if a > 0:\n        return a + b\n    return a + b\n"

// addTest stands in for upstream's FakeRunner (`add(1, 2) == 3`): with
// a = 1 only line 3 runs, so only a mutant of its `+` fails the test.
func addTest(cwd string, _ time.Duration) int {
	text, err := os.ReadFile(filepath.Join(cwd, "src", "demo.py"))
	if err != nil {
		return 1
	}
	lines := strings.Split(string(text), "\n")
	if len(lines) > 2 && strings.Contains(lines[2], "a + b") {
		return 0
	}
	return 1
}

func baseOptions(r Runner) fileOptions {
	return fileOptions{runner: r, stderr: &strings.Builder{}, ignoreCoverage: true, mutateAll: true,
		testCommand: "fake", timeoutFactor: 10, mutationWarning: 50, baselines: map[string]baseline{}}
}
