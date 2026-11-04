package mutate

// The mutator pipeline (upstream mutator/cli.py after parse_args).

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
)

func skippedPart(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if containsString(discover.SkipDirs, part) || containsString(discover.TestDirs, part) {
			return true
		}
	}
	return false
}

// selectFiles is cli.select_files.
func selectFiles(opts Options, stderr io.Writer) ([]string, error) {
	return discover.Select(discover.Selection{
		ProjectRoot: opts.ProjectRoot,
		SourceRoots: opts.SourceRoots,
		Positionals: opts.Positionals,
		Changed:     opts.Changed,
		Walk:        discover.IterSourceFiles,
		KeepChanged: func(p, root string) bool {
			rel, _ := discover.RelativeTo(p, root)
			return !skippedPart(rel) && discover.IsSource(p)
		},
		KeepFile: func(p string) bool {
			if discover.IsSource(p) {
				return true
			}
			if discover.IsTestFile(p) {
				fmt.Fprintf(stderr, "Skipping test file %s\n", p)
			}
			return false
		},
		Outside: func(p, root string) { fmt.Fprintf(stderr, "Skipping %s; it is outside %s\n", p, root) },
	})
}

// Run executes a parsed invocation; err is an internal failure (upstream
// would crash with a traceback).
func Run(opts Options, stdout, stderr io.Writer) (int, error) {
	root := discover.Resolve(opts.ProjectRoot)
	files, err := selectFiles(opts, stderr)
	if err != nil {
		var gerr *discover.GitError
		if errors.As(err, &gerr) {
			fmt.Fprintln(stderr, gerr.Message)
			return gerr.Code, nil
		}
		return 1, err
	}
	if len(files) == 0 {
		fmt.Fprintln(stdout, "No source files to mutate.")
		return 0, nil
	}
	if !opts.Scan {
		restored, err := restoreBackups(root)
		for _, p := range restored {
			fmt.Fprintf(stderr, "Restored %s from an interrupted mutation.\n", p)
		}
		if err != nil {
			return 1, err
		}
	}
	runner := &CommandRunner{Verbose: opts.Verbose, Stderr: stderr}
	if code, err := prepareCoverage(opts, root, files, runner, stdout, stderr); err != nil || code != 0 {
		return code, err
	}
	if opts.Scan {
		for _, p := range files {
			lines, err := coverageFor(opts, root, p)
			if err != nil {
				return 1, err
			}
			text, err := scanFile(p, root, lines, opts.NoCoverage, opts.Lines, opts.React)
			if err != nil {
				return 1, err
			}
			fmt.Fprint(stdout, text)
		}
		return 0, nil
	}
	return mutateFiles(opts, root, files, runner, stdout, stderr)
}

// prepareCoverage is cli._prepare_coverage: a failed --coverage-command's
// code is returned as-is.
func prepareCoverage(opts Options, root string, files []string, runner Runner, stdout, stderr io.Writer) (int, error) {
	if opts.NoCoverage || opts.UseExistingCoverage || opts.Scan {
		return 0, nil
	}
	if opts.CoverageCommand != "" {
		res := runner.Run(Command{Shell: opts.CoverageCommand}, root, 0)
		if res.Code != 0 {
			fmt.Fprintf(stderr, "Coverage command exited %d. Reports already on disk will not be read.\n", res.Code)
			if tail := lastLines(res.Output, 20); tail != "" {
				fmt.Fprintln(stderr, tail)
			}
			return res.Code, nil
		}
		return 0, nil
	}
	cov := &coverage.Runner{Stdout: stdout, Stderr: stderr}
	_, err := cov.RunCoverage(root, files, "")
	return 0, err
}

func coverageFor(opts Options, root, path string) (lineSet, error) {
	if opts.NoCoverage {
		return nil, nil
	}
	language := discover.LanguageOf(path)
	if language == "" {
		return nil, nil
	}
	return coveredLines(root, path, language, opts.FixGoCoverage)
}

// siteAnnotator chains the opt-in site-log suffixes: --attribute
// (attribute.go), then --branches (branches.go). nil when neither is on, so
// the default log keeps upstream's bytes.
func siteAnnotator(opts Options, root string, result RunResult, lines lineSet) func(Site, string) string {
	var chain []func(Site, string) string
	if opts.Attribute {
		if f := attributeAnnotator(result); f != nil {
			chain = append(chain, f)
		}
	}
	if opts.Branches {
		if f := branchAnnotator(root, result, lines); f != nil {
			chain = append(chain, f)
		}
	}
	if len(chain) == 0 {
		return nil
	}
	return func(s Site, status string) string {
		var out string
		for _, f := range chain {
			out += f(s, status)
		}
		return out
	}
}

func mutateFiles(opts Options, root string, files []string, runner Runner, stdout, stderr io.Writer) (int, error) {
	baselines := map[string]baseline{}
	var forms []FormResult
	var written []string
	baselineFailed, survived := false, false
	for _, p := range files {
		lines, err := coverageFor(opts, root, p)
		if err != nil {
			return 1, err
		}
		result, err := mutateFile(p, root, fileOptions{
			runner: runner, stderr: stderr, coveredLines: lines, ignoreCoverage: opts.NoCoverage,
			mutateAll: opts.MutateAll, lines: opts.Lines, testCommand: opts.TestCommand,
			timeoutFactor: opts.TimeoutFactor, minTimeout: opts.MinTimeout, mutationWarning: opts.MutationWarning,
			baselines: baselines, maxWorkers: opts.MaxWorkers, attribute: opts.Attribute, branches: opts.Branches,
			react: opts.React,
			// --attribute output is coderefinery's own: credit kills to tests, not to an overlay
			// that hides a package (M8)
			fixOverlay: opts.FixOverlay || opts.Attribute,
		})
		if err != nil {
			return 1, err
		}
		switch {
		case result.Hint != "":
			fmt.Fprintln(stderr, result.Hint)
			return 1, nil
		case result.Skipped != "":
			fmt.Fprintf(stderr, "Skipping %s: %s\n", result.Path, result.Skipped)
		case result.BaselineFailed:
			fmt.Fprintln(stderr, result.BaselineMessage)
			baselineFailed = true
		default:
			fmt.Fprint(stdout, formatSiteLog(result.Sites, result.Statuses, siteAnnotator(opts, root, result, lines)))
			forms = append(forms, result.Forms...)
			written = append(written, result.Written...)
			for _, f := range result.Forms {
				if f.Survived > 0 {
					survived = true
				}
			}
		}
	}
	if len(forms) > 0 {
		fmt.Fprint(stdout, formatResults(forms))
	}
	for _, p := range written {
		fmt.Fprintf(stderr, "Wrote %s\n", p)
	}
	switch {
	case baselineFailed:
		return 2, nil
	case survived:
		return 3, nil
	}
	return 0, nil
}
