package crap

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
	"github.com/matheusAle/coderefinery/internal/source"
)

func helpText() string {
	dirs := append(append([]string{}, discover.SkipDirs...), discover.TestDirs...)
	sort.Strings(dirs)
	patterns := []string{"*" + discover.GoTestSuffix}
	for _, s := range discover.ClojureTestSuffixes {
		patterns = append(patterns, "*"+s)
	}
	for _, s := range discover.JSTestSuffixes {
		patterns = append(patterns, "*"+s)
	}
	patterns = append(patterns, "test_*.py", "*_test.py", "conftest.py")
	return `Usage: crapper [options] [path-or-filter ...]

Detect the language of each source file and score it with the CRAP metric
CRAP = CC² × (1 − coverage)³ + CC. Writes .metrics/crap.edn for uml-viewer
and prints a report sorted worst first.

Languages: Clojure (.clj .cljc .cljs .bb), Java (.java), Go (.go),
TypeScript and JavaScript (.ts .tsx .mts .cts .js .jsx .mjs .cjs),
Rust (.rs), Python (.py).

Options:
  -h, --help                    Print this help and exit.
  --root <path>                 Project root. Metrics are written here.
                                Default: the current directory.
  -s, --source-root <path>      Walk this tree instead of the project root.
                                May be repeated.
  --changed                     Analyze added and modified source files from
                                git status.
  --no-coverage                 Score complexity only. Coverage and CRAP are N/A.
  --use-existing-coverage       Read coverage already on disk. Do not rerun tests.
  --coverage-command <cmd>      Run this command instead of the per-language
                                coverage tools, then read the reports it wrote.
  --threshold <number>          Exit 2 when the worst CRAP score is above this.

Arguments:
  path              File or directory to analyze. Test paths are included when
                    you name them explicitly.
  filter            When the argument is not a path, only source files whose
                    path contains this text are analyzed.

With no paths, source files under the project root are analyzed. Directories
named ` + strings.Join(dirs, ", ") + ` are skipped, as are ` + strings.Join(patterns, ", ") + `.

Coverage, when it is produced:
  Clojure      clj -M:cov --lcov, then Cloverage form counts or LCOV
  Java         Maven JaCoCo instruction coverage, for each pom.xml module
  Go           go test ./... -coverprofile=...
  TypeScript   npm run coverage, Vitest's own LCOV, or c8 around npm test
  Rust         cargo llvm-cov, or cargo tarpaulin, in the nearest Cargo.toml
  Python       coverage.py LCOV, via pytest or unittest, per project

uml-viewer joins .metrics/crap.edn on :namespace and :name.
  Clojure      the ns, and the defn name
  Java         package.Class, and the method name
  Go           import path, or import.path.Receiver for methods
  TypeScript   dotted module path, or module.Class for methods
  Rust         crate::module, or crate::module::Type for methods
  Python       dotted module path, or module.Class for methods
`
}

// Options is cli.Options.
type Options struct {
	Help                bool
	Message             string
	ExitCode            int
	ProjectRoot         string
	SourceRoots         []string
	Positionals         []string
	NoCoverage          bool
	UseExistingCoverage bool
	CoverageCommand     string
	Threshold           *float64
	Changed             bool
	Branches            bool // --branches (hidden, design-tests §6)
}

func take(args []string, i int, opt string) (string, error) {
	if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
		return "", fmt.Errorf("%s requires a value", opt)
	}
	return args[i+1], nil
}

// ParseArgs is cli.parse_args.
func ParseArgs(args []string) Options {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return Options{Help: true, Message: helpText()}
		}
	}
	opts := Options{ProjectRoot: "."}
	usage := func(msg string) Options {
		return Options{Help: true, Message: msg + "\n\n" + helpText(), ExitCode: 1}
	}
	for i := 0; i < len(args); {
		arg := args[i]
		switch {
		case arg == "-s" || arg == "--source-root" || arg == "--root" || arg == "--coverage-command" || arg == "--threshold":
			v, err := take(args, i, arg)
			if err != nil {
				return usage(err.Error())
			}
			switch arg {
			case "--root":
				opts.ProjectRoot = v
			case "--coverage-command":
				opts.CoverageCommand = v
			case "--threshold":
				t, err := pyfmt.ParseFloat(v)
				if err != nil {
					return usage("--threshold requires a number")
				}
				opts.Threshold = &t
			default:
				opts.SourceRoots = append(opts.SourceRoots, v)
			}
			i += 2
			continue
		case arg == "--no-coverage":
			opts.NoCoverage = true
		case arg == "--use-existing-coverage":
			opts.UseExistingCoverage = true
		case arg == "--changed":
			opts.Changed = true
		case arg == "--branches":
			opts.Branches = true
		case strings.HasPrefix(arg, "-"):
			return usage("Unknown option: " + arg)
		default:
			opts.Positionals = append(opts.Positionals, arg)
		}
		i++
	}
	if opts.NoCoverage && opts.CoverageCommand != "" {
		return usage("--no-coverage cannot be combined with --coverage-command")
	}
	if opts.NoCoverage && opts.Branches {
		return usage("--no-coverage cannot be combined with --branches")
	}
	return opts
}

// SelectFiles is cli.select_files.
func SelectFiles(opts Options) ([]string, error) {
	return discover.Select(discover.Selection{
		ProjectRoot: opts.ProjectRoot,
		SourceRoots: opts.SourceRoots,
		Positionals: opts.Positionals,
		Changed:     opts.Changed,
		Walk:        discover.IterSourceFiles,
		KeepChanged: func(p, _ string) bool { return discover.IsSource(p) },
		KeepFile:    func(p string) bool { return discover.LanguageOf(p) != "" },
	})
}

func sourcePath(file, root string) string {
	if rel, ok := discover.RelativeTo(file, root); ok {
		return rel
	}
	return file
}

// AnalyzeFiles is analyze.analyze_files. A nil bundle means --no-coverage.
func AnalyzeFiles(files []string, projectRoot string, bundle *coverage.Bundle) ([]model.Entry, error) {
	root := discover.Resolve(projectRoot)
	if bundle != nil {
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = sourcePath(discover.Resolve(f), root)
		}
		bundle.BindSources(paths)
	}
	var entries []model.Entry
	for _, f := range files {
		f = discover.Resolve(f)
		language := discover.LanguageOf(f)
		if language == "" {
			continue
		}
		text, err := source.Read(f)
		if err != nil {
			return nil, err
		}
		fns, err := FunctionsInFile(language, text, sourcePath(f, root), root)
		if err != nil {
			// Upstream tree-sitter always yields a tree; a hand parser that
			// cannot handle the file contributes no functions (DEVIATIONS.md).
			continue
		}
		for _, fn := range fns {
			var cov *float64
			if bundle != nil {
				pct, ok := bundle.PercentFor(fn)
				if !ok {
					pct = 0.0
				}
				cov = &pct
			}
			entries = append(entries, MakeEntry(fn, cov))
		}
	}
	return SortEntries(entries), nil
}

// Main runs `coderefinery crap` and returns the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	opts := ParseArgs(args)
	if opts.Help {
		w := stdout
		if opts.ExitCode != 0 {
			w = stderr
		}
		msg := opts.Message
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(w, msg)
		return opts.ExitCode
	}
	code, err := run(opts, stdout, stderr)
	if err != nil {
		var gerr *discover.GitError
		if errors.As(err, &gerr) {
			fmt.Fprintln(stderr, gerr.Message)
			return gerr.Code
		}
		fmt.Fprintf(stderr, "crapper: %v\n", err)
		return 1
	}
	return code
}

func run(opts Options, stdout, stderr io.Writer) (int, error) {
	root := discover.Resolve(opts.ProjectRoot)
	files, err := SelectFiles(opts)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		fmt.Fprintln(stdout, "No source files to analyze.")
		return 0, nil
	}
	bundle, err := coverageBundle(opts, root, files, stdout, stderr)
	if err != nil {
		return 0, err
	}
	entries, err := AnalyzeFiles(files, root, bundle)
	if err != nil {
		return 0, err
	}
	metrics := filepath.Join(root, ".metrics", "crap.edn")
	if err := os.MkdirAll(filepath.Dir(metrics), 0o777); err != nil {
		return 0, err
	}
	if err := os.WriteFile(metrics, []byte(RenderEDN(entries)), 0o666); err != nil {
		return 0, err
	}
	fmt.Fprint(stdout, FormatReport(entries))
	fmt.Fprintf(stderr, "Wrote %s\n", metrics)
	if opts.Branches {
		if err := branchesReport(root, files, stdout, stderr); err != nil {
			return 0, err
		}
	}
	return thresholdStatus(opts, entries, stderr), nil
}

func coverageBundle(opts Options, root string, files []string, stdout, stderr io.Writer) (*coverage.Bundle, error) {
	if opts.NoCoverage {
		return nil, nil
	}
	if opts.UseExistingCoverage {
		return coverage.LoadBundle(root)
	}
	runner := &coverage.Runner{Stdout: stdout, Stderr: stderr}
	status, err := runner.RunCoverage(root, files, opts.CoverageCommand)
	if err != nil {
		return nil, err
	}
	if opts.CoverageCommand != "" && status != 0 {
		return coverage.NewBundle(), nil
	}
	return coverage.LoadBundle(root)
}

func thresholdStatus(opts Options, entries []model.Entry, stderr io.Writer) int {
	if opts.Threshold == nil {
		return 0
	}
	worst, scored := math.Inf(-1), false
	for _, e := range entries {
		if e.Crap != nil {
			scored = true
			worst = math.Max(worst, *e.Crap)
		}
	}
	if scored && worst > *opts.Threshold {
		fmt.Fprintf(stderr, "CRAP threshold exceeded: %s > %s\n", pyfmt.Fixed(worst, 1), pyfmt.Fixed(*opts.Threshold, 1))
		return 2
	}
	return 0
}
