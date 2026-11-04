package react

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
	"github.com/matheusAle/coderefinery/internal/source"
)

func helpText() string {
	var b strings.Builder
	b.WriteString(`Usage: coderefinery react [options] [path-or-filter ...]

React metrics for function components and custom hooks in .tsx .jsx .ts
.js files (static, from the syntax tree). Each hook callback
(Cart#useEffect#1) and handler (Cart.onSubmit) is its own CRAP entry; its
decisions are not charged to the component. Writes .metrics/react.edn and
prints the entries and the components worst first.

Options:
  -h, --help                Print this help and exit.
  --root <path>             Project root. Metrics are written here.
                            Default: the current directory.
  --changed                 Analyze added and modified source files from git status.
  --no-coverage             Complexity only. Coverage and CRAP are N/A; an entry
                            then offends at CC >= the CRAP limit (CRAP's floor).
  --use-existing-coverage   Read coverage already on disk. Do not rerun tests.
  --coverage-command <cmd>  Run this command instead of the TypeScript coverage
                            runner, then read the reports it wrote.
  --crap <number>           An entry offends at CRAP >= this. Default: 10.
`)
	b.WriteString("  --max-<metric> <n|off>    A component or hook offends when the metric is above n.\n" +
		"                            Metrics and defaults:\n")
	line := ""
	for i, l := range limits {
		item := l.flag + " " + itoa(l.def)
		if l.def < 0 {
			item = l.flag + " off"
		}
		if i < len(limits)-1 {
			item += ","
		}
		if line != "" && len(line)+1+len(item) > 50 {
			b.WriteString("                              " + line + "\n")
			line = ""
		}
		line = strings.TrimSpace(line + " " + item)
	}
	b.WriteString("                              " + line + "\n")
	b.WriteString(`
Exit codes: 0 no violations · 1 usage or error · 2 the coverage run failed
(failing tests; fix them or use --no-coverage) · 3 a threshold is exceeded.
`)
	return b.String()
}

// Options are the parsed arguments.
type Options struct {
	Help                bool
	Message             string
	ExitCode            int
	ProjectRoot         string
	Positionals         []string
	Changed             bool
	NoCoverage          bool
	UseExistingCoverage bool
	CoverageCommand     string
	Limits              Limits
}

// ParseArgs parses `coderefinery react` arguments.
func ParseArgs(args []string) Options {
	opts := Options{ProjectRoot: ".", Limits: DefaultLimits()}
	usage := func(msg string) Options {
		return Options{Help: true, Message: msg + "\n\n" + helpText(), ExitCode: 1}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return Options{Help: true, Message: helpText()}
		case arg == "--changed":
			opts.Changed = true
		case arg == "--no-coverage":
			opts.NoCoverage = true
		case arg == "--use-existing-coverage":
			opts.UseExistingCoverage = true
		case strings.HasPrefix(arg, "-"):
			if !takesValue(arg) {
				return usage("Unknown option: " + arg)
			}
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return usage(arg + " requires a value")
			}
			if msg := opts.set(arg, args[i+1]); msg != "" {
				return usage(msg)
			}
			i++
		default:
			opts.Positionals = append(opts.Positionals, arg)
		}
	}
	if opts.NoCoverage && (opts.CoverageCommand != "" || opts.UseExistingCoverage) {
		return usage("--no-coverage cannot be combined with --coverage-command or --use-existing-coverage")
	}
	return opts
}

func takesValue(arg string) bool {
	if arg == "--root" || arg == "--coverage-command" || arg == "--crap" {
		return true
	}
	_, ok := limitFor(arg)
	return ok
}

func limitFor(arg string) (limit, bool) {
	for _, l := range limits {
		if arg == "--max-"+l.flag {
			return l, true
		}
	}
	return limit{}, false
}

func (o *Options) set(arg, value string) string {
	switch arg {
	case "--root":
		o.ProjectRoot = value
	case "--coverage-command":
		o.CoverageCommand = value
	case "--crap":
		v, err := pyfmt.ParseFloat(value)
		if err != nil {
			return "--crap requires a number"
		}
		o.Limits.Crap = v
	default:
		l, _ := limitFor(arg)
		if value == "off" {
			o.Limits.Max[l.flag] = -1
			return ""
		}
		v, err := pyfmt.ParseInt(value)
		if err != nil || v < 0 {
			return arg + " requires a non-negative integer or off"
		}
		o.Limits.Max[l.flag] = v
	}
	return ""
}

// Main runs `coderefinery react` and returns the exit code.
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
	code, err := run(opts, stdout, stderr)
	if err != nil {
		var gerr *discover.GitError
		if errors.As(err, &gerr) {
			fmt.Fprintln(stderr, gerr.Message)
			return gerr.Code
		}
		fmt.Fprintf(stderr, "react: %v\n", err)
		return 1
	}
	return code
}

// selectFiles reuses crap's selection (paths, filters, --changed, skipped
// dirs and test files) and keeps the TypeScript/JavaScript files.
func selectFiles(opts Options) ([]string, error) {
	files, err := crap.SelectFiles(crap.Options{ProjectRoot: opts.ProjectRoot, Positionals: opts.Positionals, Changed: opts.Changed})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range files {
		if discover.LanguageOf(f) == "typescript" {
			out = append(out, f)
		}
	}
	return out, nil
}

func run(opts Options, stdout, stderr io.Writer) (int, error) {
	root := discover.Resolve(opts.ProjectRoot)
	files, err := selectFiles(opts)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		fmt.Fprintln(stdout, "No TypeScript or JavaScript files to analyze.")
		return 0, nil
	}
	bundle, failed, err := coverageBundle(opts, root, files, stdout, stderr)
	if err != nil {
		return 0, err
	}
	if failed {
		fmt.Fprintln(stderr, "react: the coverage run failed; fix the failing tests or use --no-coverage")
		return 2, nil
	}
	units, entries, err := Analyze(files, root, bundle)
	if err != nil {
		return 0, err
	}
	entries = SortEntries(entries)
	units = SortUnits(units, Check(units, entries, opts.Limits))
	violations := Check(units, entries, opts.Limits)
	metrics := filepath.Join(root, ".metrics", "react.edn")
	if err := os.MkdirAll(filepath.Dir(metrics), 0o777); err != nil {
		return 0, err
	}
	if err := os.WriteFile(metrics, []byte(RenderEDN(entries, units, violations)), 0o666); err != nil {
		return 0, err
	}
	if len(units) == 0 {
		fmt.Fprintln(stdout, "No React components or hooks found.")
	} else {
		fmt.Fprint(stdout, FormatReport(entries, units, violations))
	}
	fmt.Fprintf(stderr, "Wrote %s\n", metrics)
	if len(violations) > 0 {
		fmt.Fprintf(stderr, "React thresholds exceeded: %d violation(s)\n", len(violations))
		return 3, nil
	}
	return 0, nil
}

// coverageBundle returns nil for --no-coverage; failed reports a coverage
// run that exited non-zero (the baseline is not trustworthy).
func coverageBundle(opts Options, root string, files []string, stdout, stderr io.Writer) (*coverage.Bundle, bool, error) {
	if opts.NoCoverage {
		return nil, false, nil
	}
	if !opts.UseExistingCoverage {
		runner := &coverage.Runner{Stdout: stdout, Stderr: stderr}
		status, err := runner.RunCoverage(root, files, opts.CoverageCommand)
		if err != nil {
			return nil, false, err
		}
		if status != 0 || runner.Failed > 0 {
			return nil, true, nil
		}
	}
	bundle, err := coverage.LoadBundle(root)
	return bundle, false, err
}

func sourcePath(file, root string) string {
	if rel, ok := discover.RelativeTo(file, root); ok {
		return rel
	}
	return file
}

// Analyze reads every file and scores its entries (bundle nil: no coverage).
// Files the parser rejects contribute nothing, as in crap.
func Analyze(files []string, root string, bundle *coverage.Bundle) ([]Unit, []Entry, error) {
	if bundle != nil {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = sourcePath(f, root)
		}
		bundle.BindSources(paths)
	}
	var units []Unit
	var entries []Entry
	for _, f := range files {
		text, err := source.Read(f)
		if err != nil {
			return nil, nil, err
		}
		file, err := AnalyzeSource(text, sourcePath(f, root), root)
		if err != nil {
			continue
		}
		for _, e := range file.Entries {
			if bundle != nil {
				pct, ok := bundle.PercentFor(model.Function{Name: e.Name, Namespace: e.Namespace, Path: e.Path,
					Language: "typescript", StartLine: e.StartLine, EndLine: e.EndLine})
				if !ok {
					pct = 0
				}
				e.Coverage = crap.RoundMetric(&pct)
				e.Crap = crap.RoundMetric(crap.Score(e.Complexity, e.Coverage))
			}
			entries = append(entries, e)
		}
		units = append(units, file.Units...)
	}
	return units, entries, nil
}
