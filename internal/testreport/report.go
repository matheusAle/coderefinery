package testreport

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/edn"
)

// Help is the `coderefinery tests --help` text.
const Help = `Usage: coderefinery tests [options]

Report how much each test contributes, from the kills ` + "`" + `coderefinery mutate
--attribute` + "`" + ` recorded in ` + "`" + `.metrics/mutate/` + "`" + ` and the results
` + "`" + `coderefinery test` + "`" + ` wrote to ` + "`" + `.metrics/tests/` + "`" + `. Writes ` + "`" + `.metrics/tests.edn` + "`" + `.

Kills are the mutants a test fails on; Unique are the kills no other test
has. A greedy covering set keeps every killed mutant killed; a test outside
it is shadowed by the listed tests, and a test that kills nothing is listed
with "(no kills)". Both are offenders.

Options:
  -h, --help        Print this help and exit.
  --root <path>     Project root. Default: the current directory.
  --group           One row per parametrized test (cases collapse onto
                    their base name) instead of one per case.
  --changed         Show only tests in changed test files and tests that
                    kill a mutant in a changed source file (git status).
                    The math still runs over every test.

Exit status: 0 no offenders; 1 usage error, or no attribution data; 2 the
latest test results have failures (fix the tests first); 3 a test kills
nothing or is shadowed.
`

// Options is the parsed command line.
type Options struct {
	Help        bool
	Message     string
	ExitCode    int
	ProjectRoot string
	Group       bool
	Changed     bool
}

// ParseArgs parses `coderefinery tests` arguments.
func ParseArgs(args []string) Options {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return Options{Help: true, Message: Help}
		}
	}
	opts := Options{ProjectRoot: "."}
	usage := func(msg string) Options { return Options{Help: true, Message: msg + "\n\n" + Help, ExitCode: 1} }
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--root":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return usage("--root requires a value")
			}
			opts.ProjectRoot = args[i+1]
			i++
		case "--group":
			opts.Group = true
		case "--changed":
			opts.Changed = true
		default:
			if strings.HasPrefix(a, "-") {
				return usage("Unknown option: " + a)
			}
			return usage("Unexpected argument: " + a)
		}
	}
	return opts
}

// Main runs `coderefinery tests` and returns the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	opts := ParseArgs(args)
	if opts.Help {
		w := stdout
		if opts.ExitCode != 0 {
			w = stderr
		}
		fmt.Fprint(w, opts.Message)
		return opts.ExitCode
	}
	code, err := Run(opts, stdout, stderr)
	if err != nil {
		var gerr *discover.GitError
		if errors.As(err, &gerr) {
			fmt.Fprintln(stderr, gerr.Message)
			return gerr.Code
		}
		fmt.Fprintf(stderr, "coderefinery tests: %v\n", err)
		return 1
	}
	return code
}

// GeneratedAt stamps tests.edn (swappable in tests).
var GeneratedAt = func() string { return time.Now().Format("2006-01-02T15:04:05-07:00") }

// Run executes a parsed invocation.
func Run(opts Options, stdout, stderr io.Writer) (int, error) {
	root := discover.Resolve(opts.ProjectRoot)
	m, err := Load(root, stderr)
	if err != nil {
		return 1, err
	}
	if !m.Attributed {
		fmt.Fprintln(stderr, "no attribution data: run coderefinery mutate --attribute")
		return 1, nil
	}
	if len(m.Failing) > 0 || m.BuildFailed {
		msg := "the latest test results have failures"
		if len(m.Failing) > 0 {
			msg += ": " + strings.Join(m.Failing, " ")
		} else {
			msg += ": the build or collection failed"
		}
		fmt.Fprintln(stderr, msg)
		fmt.Fprintln(stderr, "fix the tests first")
		return 2, nil
	}
	view := m
	if opts.Group {
		view = m.ByBase()
	}
	durations := map[string]float64{}
	for _, c := range view.Tests {
		if c.Duration >= 0 { // synthesized cases have none (+Inf in the tie-break)
			durations[c.ID] = c.Duration
		}
	}
	rows := Analyze(view, durations)
	shown := rows
	if opts.Changed {
		if shown, err = changedRows(root, view, rows); err != nil {
			return 1, err
		}
	}
	sortRows(shown)
	levels := sameFunction(view, rows)
	fmt.Fprint(stdout, FormatReport(view, shown, levels))
	out := filepath.Join(root, ".metrics", "tests.edn")
	if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
		return 1, err
	}
	rowsKind := "case"
	if opts.Group {
		rowsKind = "test"
	}
	if err := os.WriteFile(out, []byte(RenderEDN(view, shown, levels, rowsKind)), 0o666); err != nil {
		return 1, err
	}
	fmt.Fprintf(stderr, "Wrote %s\n", out)
	for _, r := range shown {
		if r.Offender() {
			return 3, nil
		}
	}
	return 0, nil
}

// changedRows keeps tests in changed test files and tests killing a mutant
// in a changed source file.
func changedRows(root string, m *Matrix, rows []Row) ([]Row, error) {
	changed, err := discover.ChangedFiles(root)
	if err != nil {
		return nil, err
	}
	files := map[string]bool{}
	for _, p := range changed {
		if rel, ok := discover.RelativeTo(p, root); ok {
			files[rel] = true
		}
	}
	var out []Row
	for _, r := range rows {
		keep := files[r.File]
		for _, k := range r.kills {
			if files[m.Mutants[k].File] {
				keep = true
			}
		}
		if keep {
			out = append(out, r)
		}
	}
	return out, nil
}

// LevelGroup is one form killed at two or more levels.
type LevelGroup struct {
	Namespace, Form string
	ByLevel         map[Level][]string // level -> row IDs (sorted)
	KillsAt         map[Level]int      // level -> distinct kills in the form
	Higher, Lower   Level
	Extra           int // kills of the highest level the lower ones miss
}

// Recommendation is "keep-lower" or "keep-both".
func (g LevelGroup) Recommendation() string {
	if g.Extra == 0 {
		return "keep-lower"
	}
	return "keep-both"
}

func sameFunction(m *Matrix, rows []Row) []LevelGroup {
	type key struct{ ns, form string }
	byForm := map[key]map[Level]map[string]bool{}
	kills := map[key]map[Level]map[int]bool{}
	for _, r := range rows {
		if r.Flaky {
			continue
		}
		for _, k := range r.kills {
			mu := m.Mutants[k]
			fk := key{mu.Namespace, mu.Form}
			if byForm[fk] == nil {
				byForm[fk], kills[fk] = map[Level]map[string]bool{}, map[Level]map[int]bool{}
			}
			if byForm[fk][r.Level] == nil {
				byForm[fk][r.Level], kills[fk][r.Level] = map[string]bool{}, map[int]bool{}
			}
			byForm[fk][r.Level][r.ID] = true
			kills[fk][r.Level][k] = true
		}
	}
	var out []LevelGroup
	for fk, levels := range byForm {
		if len(levels) < 2 {
			continue
		}
		g := LevelGroup{Namespace: fk.ns, Form: fk.form, ByLevel: map[Level][]string{}, KillsAt: map[Level]int{}}
		var present []Level
		for l, ids := range levels {
			present = append(present, l)
			for id := range ids {
				g.ByLevel[l] = append(g.ByLevel[l], id)
			}
			sort.Strings(g.ByLevel[l])
			g.KillsAt[l] = len(kills[fk][l])
		}
		sort.Slice(present, func(i, j int) bool { return levelOrder[present[i]] < levelOrder[present[j]] })
		g.Lower, g.Higher = present[0], present[len(present)-1]
		lower := map[int]bool{}
		for _, l := range present[:len(present)-1] {
			for k := range kills[fk][l] {
				lower[k] = true
			}
		}
		for k := range kills[fk][g.Higher] {
			if !lower[k] {
				g.Extra++
			}
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Form < out[j].Form
	})
	return out
}

func padRight(s string, w int) string {
	if n := w - utf8.RuneCountInString(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func levelText(r Row) string {
	if r.LevelInferred {
		return string(r.Level) + "~"
	}
	return string(r.Level)
}

func shadowText(r Row) string {
	switch {
	case r.Flaky:
		return "(flaky; left out)"
	case r.Kills == 0 && r.Runs >= 0:
		return fmt.Sprintf("(no kills; runs %d %s)", r.Runs, plural(r.Runs, "fn", "fns"))
	case r.Kills == 0:
		return "(no kills)"
	case len(r.ShadowedBy) == 0:
		return "-"
	}
	shown := r.ShadowedBy
	if len(shown) > 2 {
		shown = shown[:2]
	}
	text := strings.Join(shown, ", ")
	if extra := len(r.ShadowedBy) - len(shown); extra > 0 {
		text += fmt.Sprintf(" +%d", extra)
	}
	return text
}

// shortName is a row ID's leaf (after the last "::").
func shortName(id string) string {
	if i := strings.LastIndex(id, "::"); i >= 0 {
		return id[i+2:]
	}
	return id
}

// FormatReport is the text report (§4).
func FormatReport(m *Matrix, rows []Row, levels []LevelGroup) string {
	killed, attributed := 0, 0
	for _, mu := range m.Mutants {
		if mu.Killed {
			killed++
			if len(mu.KilledBy) > 0 {
				attributed++
			}
		}
	}
	tw := 4
	for _, r := range rows {
		tw = max(tw, utf8.RuneCountInString(r.ID))
	}
	tw = min(tw, 72)
	var b strings.Builder
	fmt.Fprintf(&b, "Test relevance: %d %s, %d killed %s (%d attributed)\n", len(rows), plural(len(rows), "test", "tests"),
		killed, plural(killed, "mutant", "mutants"), attributed)
	header := fmt.Sprintf("%s %5s %6s %4s %-13s %s", padRight("Test", tw), "Kills", "Unique", "Fns", "Level", "Shadowed by")
	b.WriteString(header + "\n" + strings.Repeat("-", utf8.RuneCountInString(header)) + "\n")
	zero, shadowed := 0, 0
	var flaky []string
	for _, r := range rows {
		if r.Flaky {
			flaky = append(flaky, r.ID)
			continue
		}
		fmt.Fprintf(&b, "%s %5d %6d %4d %-13s %s\n", padRight(r.ID, tw), r.Kills, r.Unique, r.Fns, levelText(r), shadowText(r))
		switch {
		case r.Kills == 0:
			zero++
		case len(r.ShadowedBy) > 0:
			shadowed++
		}
	}
	if len(flaky) > 0 {
		b.WriteString("\nFlaky (left out of the math; fix them, do not delete them)\n")
		for _, id := range flaky {
			b.WriteString(id + "\n")
		}
	}
	if len(levels) > 0 {
		b.WriteString("\nSame function, different levels\n")
		for _, g := range levels {
			var parts []string
			for _, l := range []Level{Unit, Integration, API, E2E} {
				ids, ok := g.ByLevel[l]
				if !ok {
					continue
				}
				names := make([]string, 0, 2)
				for _, id := range ids[:min(2, len(ids))] {
					names = append(names, shortName(id))
				}
				text := strings.Join(names, ", ")
				if len(ids) > 2 {
					text += fmt.Sprintf(" +%d", len(ids)-2)
				}
				parts = append(parts, fmt.Sprintf("%s %d (%s)", l, g.KillsAt[l], text))
			}
			rec := fmt.Sprintf("keep %s; %s adds no kills here", g.Lower, g.Higher)
			if g.Extra > 0 {
				rec = fmt.Sprintf("keep both; %s kills %d here that %s tests miss: push a %s test down", g.Higher, g.Extra, g.Lower, g.Lower)
			}
			fmt.Fprintf(&b, "%s %s: %s · %s\n", g.Namespace, g.Form, strings.Join(parts, " · "), rec)
		}
	}
	if t, bd := m.Unattributed["<timeout>"], m.Unattributed["<build>"]; t+bd > 0 {
		fmt.Fprintf(&b, "\nUnattributed kills: %d <timeout>, %d <build>\n", t, bd)
	}
	if zero+shadowed > 0 {
		fmt.Fprintf(&b, "Offenders: %d %s nothing, %d %s shadowed\n", zero, plural(zero, "test kills", "tests kill"),
			shadowed, plural(shadowed, "test is", "tests are"))
	}
	return b.String()
}

func ednStrings(list []string) string {
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = edn.String(s)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// RenderEDN is .metrics/tests.edn (§5).
func RenderEDN(m *Matrix, rows []Row, levels []LevelGroup, rowsKind string) string {
	var b strings.Builder
	b.WriteString("{:version 1 :generated-at " + edn.String(GeneratedAt()) + " :rows " + edn.String(rowsKind) + " :tests [")
	for _, r := range rows {
		duration := "nil"
		if !math.IsInf(r.Duration, 1) {
			duration = edn.Number4(r.Duration)
		}
		fmt.Fprintf(&b, "\n  {:id %s :file %s :level %s :level-inferred %v :kills %d :unique %d :fns %d :runs %d :cases %d :in-cover %v :shadowed-by %s :flaky %v :duration %s}",
			edn.String(r.ID), edn.String(r.File), edn.String(string(r.Level)), r.LevelInferred, r.Kills, r.Unique, r.Fns, r.Runs,
			r.Cases, r.InCover, ednStrings(r.ShadowedBy), r.Flaky, duration)
	}
	b.WriteString("\n] :levels [")
	for _, g := range levels {
		var parts []string
		for _, l := range []Level{Unit, Integration, API, E2E} {
			if ids, ok := g.ByLevel[l]; ok {
				parts = append(parts, ":"+string(l)+" "+ednStrings(ids))
			}
		}
		fmt.Fprintf(&b, "\n  {:namespace %s :form %s :by-level {%s} :recommendation %s}", edn.String(g.Namespace), edn.String(g.Form),
			strings.Join(parts, " "), edn.String(g.Recommendation()))
	}
	fmt.Fprintf(&b, "\n] :unattributed {:timeout %d :build %d}}\n", m.Unattributed["<timeout>"], m.Unattributed["<build>"])
	return b.String()
}
