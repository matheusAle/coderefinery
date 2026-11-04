package mutate

// Apply each selected mutant, run tests, and record the snapshot (upstream
// mutator/engine.py).

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/source"
	"github.com/matheusAle/coderefinery/internal/testplan"
)

// restoreBackups puts back sources a killed legacy run left mutated.
func restoreBackups(root string) ([]string, error) {
	base := filepath.Join(root, "target", "mutator-backup")
	if !discover.IsDir(base) {
		return nil, nil
	}
	var backups []string
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err == nil && discover.IsFile(p) {
			backups = append(backups, p)
		}
		return nil
	})
	sortPaths(backups)
	var restored []string
	for _, b := range backups {
		rel, _ := filepath.Rel(base, b)
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
			return restored, err
		}
		data, err := os.ReadFile(b)
		if err != nil {
			return restored, err
		}
		if err := os.WriteFile(target, data, 0o666); err != nil {
			return restored, err
		}
		if err := os.Remove(b); err != nil {
			return restored, err
		}
		restored = append(restored, target)
	}
	return restored, nil
}

func isCovered(s Site, lines lineSet, ignoreCoverage bool) bool {
	if ignoreCoverage {
		return true
	}
	if lines == nil {
		return false
	}
	return lines[s.Line] || lines[s.AnchorLine]
}

func coverageMap(found []Site, lines lineSet, ignoreCoverage bool) map[string]bool {
	out := make(map[string]bool, len(found))
	for _, s := range found {
		out[s.ID] = isCovered(s, lines, ignoreCoverage)
	}
	return out
}

func unchanged(h History, digests map[formKey]string, k formKey) bool {
	prior, ok := h.Forms[k]
	d, has := digests[k]
	return ok && has && prior == d
}

// selectSites is engine._select.
func selectSites(sites []Site, covered map[string]bool, h History, digests map[formKey]string, mutateAll bool, lines map[int]bool) []Site {
	var chosen []Site
	for _, s := range sites {
		if !covered[s.ID] {
			continue
		}
		if lines != nil && !lines[s.Line] {
			continue
		}
		if mutateAll || !unchanged(h, digests, formKey{s.Namespace, s.FormID}) {
			chosen = append(chosen, s)
			continue
		}
		if h.Outcomes[s.ID] == "killed" {
			continue
		}
		chosen = append(chosen, s)
	}
	return chosen
}

// carryForward is engine._carry_forward: prior outcomes of covered sites
// in unchanged forms.
func carryForward(sites []Site, covered map[string]bool, h History, digests map[formKey]string) map[string]string {
	byID := map[string]Site{}
	for _, s := range sites {
		byID[s.ID] = s
	}
	carried := map[string]string{}
	for id, status := range h.Outcomes {
		s, ok := byID[id]
		if !ok || !covered[id] {
			continue
		}
		if unchanged(h, digests, formKey{s.Namespace, s.FormID}) {
			carried[id] = status
		}
	}
	return carried
}

// formResults is engine._forms.
func formResults(u *fileUnit, sites []Site, covered map[string]bool, outcomes map[string]string, lines map[int]bool) []FormResult {
	spans := u.spans()
	sortStable(spans, func(a, b formSpan) bool { return a.start < b.start })
	digests := u.digests()
	grouped := map[formKey][]Site{}
	for _, s := range sites {
		k := formKey{s.Namespace, s.FormID}
		grouped[k] = append(grouped[k], s)
	}
	var forms []FormResult
	for _, sp := range spans {
		owned := grouped[sp.key]
		f := FormResult{ID: sp.key.ID, Namespace: sp.key.Namespace, Name: sp.name, Private: sp.private, File: u.key,
			Line: sp.start, EndLine: sp.end, Digest: digests[sp.key], Sites: len(owned)}
		for _, s := range owned {
			if !covered[s.ID] {
				f.Uncovered++
				continue
			}
			switch outcomes[s.ID] {
			case "killed":
				f.Killed++
			case "survived":
				f.Survived++
			default:
				if lines == nil {
					f.Survived++
				}
			}
		}
		forms = append(forms, f)
	}
	return forms
}

func statuses(found []Site, covered map[string]bool, outcomes map[string]string) map[string]string {
	out := map[string]string{}
	for _, s := range found {
		if !covered[s.ID] {
			out[s.ID] = "uncovered"
		} else if o, ok := outcomes[s.ID]; ok {
			out[s.ID] = o
		}
	}
	return out
}

// baseline is one cached baseline run.
type baseline struct {
	ok      bool
	seconds float64
	tail    string
	display string   // --attribute: the command run (with its reporter)
	failing []string // --attribute: the failing test ids
	hint    string   // --attribute: no report; one line naming what to install
}

func lastLines(output string, n int) string {
	lines := coverage.SplitLines(output)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// fileOptions is the per-run configuration mutate_file receives.
type fileOptions struct {
	runner          Runner
	stderr          io.Writer
	coveredLines    lineSet
	ignoreCoverage  bool
	mutateAll       bool
	lines           map[int]bool
	testCommand     string
	timeoutFactor   float64
	minTimeout      float64 // seconds; upstream's floor is 2
	mutationWarning int
	baselines       map[string]baseline
	maxWorkers      int
	attribute       bool // --attribute (attribute.go)
	branches        bool // --branches (branches.go)
}

// mutateFile is engine.mutate_file.
func mutateFile(path, root string, o fileOptions) (RunResult, error) {
	root = discover.Resolve(root)
	path = discover.Resolve(path)
	language := discover.LanguageOf(path)
	key := sourceKey(path, root)
	if language == "" {
		return RunResult{Path: key, Skipped: "unsupported file"}, nil
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return RunResult{}, err
	}
	if !utf8.Valid(original) {
		return RunResult{Path: key, Skipped: "file is not UTF-8"}, nil
	}
	u := newFileUnit(string(original), path, root, key)
	found := u.sites()
	covered := coverageMap(found, o.coveredLines, o.ignoreCoverage)
	if o.coveredLines == nil && !o.ignoreCoverage && len(found) > 0 {
		fmt.Fprintf(o.stderr, "No coverage data for %s; its sites are uncovered. Pass --no-coverage to run them anyway.\n", key)
	}
	history := loadHistory(root, key)
	digests := u.digests()
	selected := selectSites(found, covered, history, digests, o.mutateAll, o.lines)
	outcomes := carryForward(found, covered, history, digests)
	killedBy := carriedKilledBy(root, key, outcomes)
	if o.attribute {
		selected = reselectUnattributed(found, selected, outcomes, killedBy, o.lines)
	}
	if len(selected) > o.mutationWarning {
		fmt.Fprintf(o.stderr, "WARNING: %d covered mutations selected in %s.\n", len(selected), key)
	}
	if len(selected) > 0 {
		command, cwd := testplan.Plan(root, path, language, o.testCommand)
		var attr *attribution
		if o.attribute {
			var hint string
			if attr, hint = newAttribution(root, path, language, command, cwd, o.testCommand); hint != "" {
				return RunResult{Path: key, Hint: hint}, nil
			}
		}
		cacheKey := command.Key(cwd) + attr.cacheKey()
		if _, ok := o.baselines[cacheKey]; !ok {
			switch {
			case attr != nil:
				o.baselines[cacheKey] = attr.baseline(o.runner)
			default:
				res := o.runner.Run(command, cwd, 0)
				if res.Code != 0 {
					o.baselines[cacheKey] = baseline{ok: false, seconds: res.Seconds, tail: lastLines(res.Output, 20)}
				} else {
					o.baselines[cacheKey] = baseline{ok: true, seconds: res.Seconds}
				}
			}
		}
		b := o.baselines[cacheKey]
		if b.hint != "" {
			return RunResult{Path: key, Hint: b.hint}, nil
		}
		if !b.ok {
			shown := command.Display()
			if b.display != "" {
				shown = b.display
			}
			msg := fmt.Sprintf("Baseline failed for %s: %s", key, shown)
			if b.tail != "" {
				msg += "\n" + b.tail
			}
			if len(b.failing) > 0 {
				msg += "\nfailing tests: " + strings.Join(b.failing, " ")
			}
			return RunResult{Path: key, BaselineFailed: true, BaselineMessage: msg}, nil
		}
		timeout := max(o.minTimeout, b.seconds*o.timeoutFactor)
		job := &mutantJob{root: root, relative: key, fileKey: key, original: original, runner: o.runner,
			command: command, cwd: cwd, timeout: time.Duration(timeout * float64(time.Second)), stderr: o.stderr,
			outcomes: outcomes, attr: attr, killedBy: killedBy}
		if err := runMutants(job, selected, o.maxWorkers); err != nil {
			return RunResult{}, err
		}
	}
	forms := formResults(u, found, covered, outcomes, o.lines)
	written, err := writeResults(root, key, forms, outcomes, killedBy)
	if err != nil {
		return RunResult{}, err
	}
	if now, err := os.ReadFile(path); err == nil && !bytes.Equal(now, original) {
		_ = os.WriteFile(path, original, 0o666)
	}
	return RunResult{Path: key, Forms: forms, Written: written, Sites: found, Statuses: statuses(found, covered, outcomes), KilledBy: killedBy}, nil
}

// scanFile is engine.scan_file. The text is read like Path.read_text:
// strict UTF-8 and universal newlines.
func scanFile(path, root string, coveredLines lineSet, ignoreCoverage bool, lines map[int]bool) (string, error) {
	root = discover.Resolve(root)
	path = discover.Resolve(path)
	key := sourceKey(path, root)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("'utf-8' codec can't decode %s", key)
	}
	u := newFileUnit(source.UniversalNewlines(string(raw)), path, root, key)
	found := u.sites()
	if lines != nil {
		var kept []Site
		for _, s := range found {
			if lines[s.Line] {
				kept = append(kept, s)
			}
		}
		found = kept
	}
	var covered map[string]bool
	if !ignoreCoverage && coveredLines != nil {
		covered = coverageMap(found, coveredLines, ignoreCoverage)
	}
	history := loadHistory(root, key)
	changed := map[formKey]bool{}
	if len(history.Forms) > 0 {
		for k, d := range u.digests() {
			if prior, ok := history.Forms[k]; !ok || prior != d {
				changed[k] = true
			}
		}
	}
	return formatScan(key, found, changed, covered), nil
}
