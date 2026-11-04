package testresult

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type goEvent struct {
	Action      string
	Package     string
	Test        string
	Elapsed     float64
	Output      string
	FailedBuild string
	ImportPath  string
}

type goKey struct{ pkg, test string }

// ParseGoJSON reads a `go test -json` stream. Lines that are not JSON test
// events are ignored (runner output is often merged in). root is the
// module dir: Files are resolved by finding `func TestX(` in the package's
// _test.go files.
func ParseGoJSON(r io.Reader, root string) (*Run, error) {
	run := &Run{Tool: "go"}
	final := map[goKey]goEvent{}
	output := map[goKey]*strings.Builder{}
	var order []goKey
	pkgFailed := map[string]bool{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			if strings.Contains(line, "[build failed]") || strings.HasPrefix(line, "FAIL") && strings.Contains(line, "[setup failed]") {
				run.BuildFailed = true
			}
			continue
		}
		var ev goEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		switch ev.Action {
		case "build-fail":
			run.BuildFailed = true
			continue
		case "output":
			if ev.Test != "" {
				k := goKey{ev.Package, ev.Test}
				if output[k] == nil {
					output[k] = &strings.Builder{}
				}
				if output[k].Len() < 2*maxFailure {
					output[k].WriteString(ev.Output)
				}
			} else if strings.Contains(ev.Output, "[build failed]") || strings.Contains(ev.Output, "[setup failed]") {
				run.BuildFailed = true
			}
			continue
		case "pass", "fail", "skip":
		default:
			continue
		}
		if ev.Test == "" {
			if ev.Action == "fail" {
				if ev.FailedBuild != "" {
					run.BuildFailed = true
				}
				pkgFailed[ev.Package] = true
			}
			continue
		}
		k := goKey{ev.Package, ev.Test}
		if _, seen := final[k]; !seen {
			order = append(order, k)
		}
		final[k] = ev
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	finder := newGoFinder(root)
	failedCase := map[string]bool{}
	for _, k := range order {
		ev := final[k]
		c := Case{Locator: k.pkg, Name: k.test, Duration: ev.Elapsed}
		switch ev.Action {
		case "pass":
			c.Status = Pass
		case "fail":
			c.Status = Fail
			failedCase[k.pkg] = true
			if out := output[k]; out != nil {
				c.Failure = goFailure(out.String())
			}
		default:
			c.Status = Skip
		}
		for _, other := range order {
			if other.pkg == k.pkg && strings.HasPrefix(other.test, k.test+"/") {
				c.Parent = true
				break
			}
		}
		top, _, _ := strings.Cut(k.test, "/")
		c.File = finder.file(k.pkg, top)
		c.finish(top)
		run.Cases = append(run.Cases, c)
	}
	// A package that failed without a failing test (TestMain exit, panic in
	// init, timeout) is not tied to a case.
	for pkg := range pkgFailed {
		if !failedCase[pkg] {
			run.BuildFailed = true
		}
	}
	run.normalize()
	return run, nil
}

// goFinder resolves (import path, TestX) to the _test.go file declaring it.
type goFinder struct {
	root, module string
	files        map[string][]string // package dir -> _test.go files (rel)
}

var modulePattern = regexp.MustCompile(`(?m)^\s*module\s+"?([^\s"]+)"?`)

func newGoFinder(root string) *goFinder {
	f := &goFinder{root: root, files: map[string][]string{}}
	if root != "" {
		if raw, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
			if m := modulePattern.FindSubmatch(raw); m != nil {
				f.module = string(m[1])
			}
		}
	}
	return f
}

func (f *goFinder) file(pkg, test string) string {
	if f.module == "" || (pkg != f.module && !strings.HasPrefix(pkg, f.module+"/")) {
		return ""
	}
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg, f.module), "/")
	files, ok := f.files[dir]
	if !ok {
		matches, _ := filepath.Glob(filepath.Join(f.root, filepath.FromSlash(dir), "*_test.go"))
		sort.Strings(matches)
		for _, m := range matches {
			rel, _ := filepath.Rel(f.root, m)
			files = append(files, filepath.ToSlash(rel))
		}
		f.files[dir] = files
	}
	decl := regexp.MustCompile(`(?m)^func\s+` + regexp.QuoteMeta(test) + `\s*\(`)
	for _, rel := range files {
		if raw, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(rel))); err == nil && decl.Match(raw) {
			return rel
		}
	}
	return ""
}

// goFailure drops the runner's === RUN/PAUSE/CONT/NAME framing lines.
func goFailure(out string) string {
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "=== RUN") || strings.HasPrefix(t, "=== PAUSE") || strings.HasPrefix(t, "=== CONT") || strings.HasPrefix(t, "=== NAME") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
