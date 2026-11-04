package testrun

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/edn"
	"github.com/matheusAle/coderefinery/internal/testresult"
)

// selectFiles is crap's selection; --changed also keeps test files, so a
// changed test selects its project.
func selectFiles(opts Options, root string) ([]string, error) {
	if !opts.Changed {
		return crap.SelectFiles(crap.Options{ProjectRoot: root, Positionals: opts.Positionals})
	}
	changed, err := discover.ChangedFiles(root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range changed {
		if discover.LanguageOf(p) == "" {
			continue
		}
		if len(opts.Positionals) > 0 && !matchesAny(p, opts.Positionals) {
			continue
		}
		out = append(out, discover.Resolve(p))
	}
	sort.Strings(out)
	return out, nil
}

func matchesAny(p string, filters []string) bool {
	for _, f := range filters {
		if strings.Contains(p, f) {
			return true
		}
	}
	return false
}

// TestedAt is the timestamp written into runs.edn (swappable in tests).
var TestedAt = func() string { return time.Now().Format("2006-01-02T15:04:05-07:00") }

// RunRecord is one module's entry in runs.edn.
type RunRecord struct {
	Language, Module, Tool, Command string
	Exit                            int
	Results, Coverage               string // root-relative
	Granularity                     string
	Contexts                        bool
	Pass, Fail, Skip, Error         int
	Flaky                           []string
	BuildFailed                     bool
}

// Run executes a parsed invocation.
func Run(opts Options, stdout, stderr io.Writer) (int, error) {
	root := discover.Resolve(opts.ProjectRoot)
	if !discover.IsDir(root) {
		fmt.Fprintf(stderr, "--root %s is not a directory\n\n%s", opts.ProjectRoot, Help)
		return 1, nil
	}
	files, err := selectFiles(opts, root)
	if err != nil {
		return 1, err
	}
	var plans []*plan
	if opts.TestCommand != "" {
		plans = []*plan{commandPlan(root, opts, files)}
	} else {
		if len(files) == 0 {
			fmt.Fprintln(stdout, "No source files to test.")
			return 0, nil
		}
		for _, m := range modules(root, files) {
			plans = append(plans, buildPlan(root, m, opts))
		}
	}
	for _, p := range plans {
		if p.hint != "" {
			fmt.Fprintln(stderr, p.hint)
			return 1, nil
		}
	}
	code := 0
	var records []RunRecord
	for _, p := range plans {
		rec, c, err := execute(root, p, opts, stdout, stderr)
		if err != nil {
			return 1, err
		}
		code = max(code, c)
		records = append(records, rec)
		fmt.Fprintln(stderr, summaryLine(rec))
	}
	index := filepath.Join(root, ".metrics", "tests", "runs.edn")
	if err := os.MkdirAll(filepath.Dir(index), 0o777); err != nil {
		return 1, err
	}
	if err := os.WriteFile(index, []byte(RenderRuns(records)), 0o666); err != nil {
		return 1, err
	}
	fmt.Fprintf(stderr, "Wrote %s\n", index)
	return code, nil
}

func summaryLine(r RunRecord) string {
	tool := r.Tool
	if tool == "" {
		tool = "tests"
	}
	cov := "no coverage"
	if r.Granularity != "" && r.Granularity != "none" {
		cov = r.Granularity + " coverage"
	}
	if r.Contexts {
		cov += ", contexts"
	}
	line := fmt.Sprintf("%s %s: %s %d passed, %d failed, %d skipped", r.Language, r.Module, tool, r.Pass, r.Fail+r.Error, r.Skip)
	if len(r.Flaky) > 0 {
		line += fmt.Sprintf(", %d flaky", len(r.Flaky))
	}
	if r.BuildFailed {
		line += ", build failed"
	}
	if r.Results == "" {
		line += ", no results"
	}
	return line + " (" + cov + ")"
}

func removeGlobs(patterns []string) {
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		for _, m := range matches {
			if discover.IsFile(m) {
				_ = os.Remove(m)
			}
		}
	}
}

// clearResults removes the files a previous run left in the module dir
// (results.xml/.json, results/TEST-*.xml, a command's results), file by file.
func clearResults(p *plan) {
	removeGlobs([]string{filepath.Join(p.out, "results.xml"), filepath.Join(p.out, "results.json"),
		filepath.Join(p.out, "results"), filepath.Join(p.out, "results", "*.xml")})
	removeGlobs(p.cleanup)
}

func execute(root string, p *plan, opts Options, stdout, stderr io.Writer) (RunRecord, int, error) {
	rec := RunRecord{Language: p.language, Module: ModuleRel(root, p.dir), Tool: p.tool, Granularity: p.granularity, Contexts: p.contexts}
	var shown []string
	for _, s := range p.steps {
		shown = append(shown, s.display())
	}
	rec.Command = strings.Join(shown, " && ")
	if err := os.MkdirAll(p.out, 0o777); err != nil {
		return rec, 1, err
	}
	if p.coverageRep != "" || p.contexts {
		_ = os.MkdirAll(p.covDir(), 0o777)
	}
	var runs []*testresult.Run
	exit := 0
	for i := 0; i < opts.Repeat; i++ {
		clearResults(p)
		exit = 0
		for _, s := range p.steps {
			c := runStep(s, p.dir, stdout, stderr)
			if !s.optional && c != 0 && exit == 0 {
				exit = c
			}
		}
		if p.collect != nil {
			_ = p.collect()
		}
		run, err := loadResults(root, p)
		if err != nil {
			if opts.Verbose {
				fmt.Fprintf(stderr, "%s %s: %v\n", p.language, rec.Module, err)
			}
			run = nil
		}
		runs = append(runs, run)
	}
	rec.Exit = exit
	last := runs[len(runs)-1]
	if last == nil {
		return rec, 2, nil
	}
	if p.tool == "" {
		rec.Tool = last.Tool
	}
	rec.Results = relTo(root, p.results)
	if p.coverageRep != "" && discover.Exists(p.coverageRep) {
		rec.Coverage = relTo(root, p.coverageRep)
	} else if p.coverageRep != "" {
		rec.Granularity, rec.Contexts = "none", false
	}
	rec.Flaky = flaky(runs)
	flakySet := map[string]bool{}
	for _, id := range rec.Flaky {
		flakySet[id] = true
	}
	rec.Pass, rec.Fail, rec.Skip, rec.Error = last.Counts()
	for _, r := range runs {
		if r == nil || r.BuildFailed {
			rec.BuildFailed = true
		}
	}
	code := 0
	for _, id := range last.Failing() {
		if !flakySet[id] {
			code = 2
		}
	}
	if rec.BuildFailed {
		code = 2
	}
	return rec, code, nil
}

// flaky is the IDs whose status differs between runs.
func flaky(runs []*testresult.Run) []string {
	if len(runs) < 2 {
		return nil
	}
	seen := map[string]map[testresult.Status]bool{}
	for _, r := range runs {
		if r == nil {
			continue
		}
		for _, c := range r.Cases {
			if c.Parent {
				continue
			}
			if seen[c.ID] == nil {
				seen[c.ID] = map[testresult.Status]bool{}
			}
			seen[c.ID][c.Status] = true
		}
	}
	var out []string
	for id, statuses := range seen {
		if len(statuses) > 1 {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func relTo(root, p string) string {
	if rel, ok := discover.RelativeTo(p, root); ok {
		return rel
	}
	return p
}

// loadResults parses the module's report with IDs relative to the repo.
func loadResults(root string, p *plan) (*testresult.Run, error) {
	if !discover.Exists(p.results) {
		return nil, fmt.Errorf("no results at %s", p.results)
	}
	hint := p.tool
	run, err := testresult.Load(p.results, p.dir, hint)
	if err != nil {
		return nil, err
	}
	run.Rebase(ModuleRel(root, p.dir))
	return run, nil
}

// LoadModuleResults reads a results file written by `coderefinery test` for
// the module at moduleRel (as recorded in runs.edn).
func LoadModuleResults(root, results, moduleRel, tool string) (*testresult.Run, error) {
	dir := filepath.Join(root, filepath.FromSlash(moduleRel))
	if tool == "mixed" {
		tool = ""
	}
	run, err := testresult.Load(filepath.Join(root, filepath.FromSlash(results)), dir, tool)
	if err != nil {
		return nil, err
	}
	run.Rebase(moduleRel)
	return run, nil
}

// runStep runs one command with inherited output. A go -json step's stdout
// goes to its file and the test output is echoed.
func runStep(s step, dir string, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "+ (%s) %s\n", dir, s.display())
	var cmd *exec.Cmd
	if s.argv == nil {
		cmd = exec.Command("/bin/sh", "-c", s.shell)
	} else {
		cmd = exec.Command(s.argv[0], s.argv[1:]...)
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), s.env...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = stderr
	if s.goJSON == "" {
		cmd.Stdout = stdout
	}
	var sink *os.File
	var pipe io.ReadCloser
	if s.goJSON != "" {
		f, err := os.Create(s.goJSON)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 127
		}
		sink = f
		defer sink.Close()
		pipe, err = cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 127
		}
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "failed to start: %v\n", err)
		return 127
	}
	if pipe != nil {
		sc := bufio.NewScanner(pipe)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			line := sc.Bytes()
			_, _ = sink.Write(append(line, '\n'))
			var ev struct{ Action, Output string }
			if len(line) > 0 && line[0] == '{' && json.Unmarshal(line, &ev) == nil {
				if ev.Action == "output" || ev.Action == "build-output" {
					fmt.Fprint(stdout, ev.Output)
				}
				continue
			}
			fmt.Fprintf(stdout, "%s\n", line)
		}
	}
	err := cmd.Wait()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

// RenderRuns is runs.edn (§5).
func RenderRuns(records []RunRecord) string {
	var b strings.Builder
	b.WriteString("{:version 1 :tested-at " + edn.String(TestedAt()) + " :runs [")
	for _, r := range records {
		b.WriteString("\n  {:language " + edn.String(r.Language) + " :module " + edn.String(r.Module) +
			" :tool " + edn.String(r.Tool) + " :command " + edn.String(r.Command) + fmt.Sprintf(" :exit %d", r.Exit) +
			" :results " + optString(r.Results) + " :coverage " + optString(r.Coverage) +
			" :granularity " + edn.String(r.Granularity) + fmt.Sprintf(" :contexts %v", r.Contexts) +
			fmt.Sprintf(" :pass %d :fail %d :skip %d :error %d :build-failed %v", r.Pass, r.Fail, r.Skip, r.Error, r.BuildFailed) +
			" :flaky [" + joinStrings(r.Flaky) + "]}")
	}
	b.WriteString("\n]}\n")
	return b.String()
}

func optString(s string) string {
	if s == "" {
		return "nil"
	}
	return edn.String(s)
}

func joinStrings(list []string) string {
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = edn.String(s)
	}
	return strings.Join(parts, " ")
}
