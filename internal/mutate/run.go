package mutate

// The mutator pipeline (upstream mutator/cli.py after parse_args).

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
)

func joinRoot(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func skippedPart(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if containsString(discover.SkipDirs, part) || containsString(discover.TestDirs, part) {
			return true
		}
	}
	return false
}

func isSource(path string) bool {
	return discover.LanguageOf(path) != "" && !discover.IsTestFile(path)
}

// selectFiles is cli.select_files.
func selectFiles(opts Options, stderr io.Writer) ([]string, error) {
	root := discover.Resolve(opts.ProjectRoot)
	var existing, filters []string
	for _, arg := range opts.Positionals {
		cand := joinRoot(root, arg)
		if discover.Exists(cand) {
			existing = append(existing, discover.Resolve(cand))
		} else {
			filters = append(filters, arg)
		}
	}
	var files []string
	switch {
	case opts.Changed:
		changed, err := discover.ChangedFiles(root)
		if err != nil {
			return nil, err
		}
		for _, p := range changed {
			rel, _ := discover.RelativeTo(p, root)
			if !skippedPart(rel) && isSource(p) {
				files = append(files, p)
			}
		}
	case len(opts.SourceRoots) > 0:
		var roots []string
		for _, p := range opts.SourceRoots {
			roots = append(roots, discover.Resolve(joinRoot(root, p)))
		}
		files = discover.IterSourceFiles(roots)
	case len(existing) > 0:
		for _, p := range existing {
			switch {
			case discover.IsDir(p):
				files = append(files, discover.IterSourceFiles([]string{p})...)
			case isSource(p):
				files = append(files, p)
			case discover.IsTestFile(p):
				fmt.Fprintf(stderr, "Skipping test file %s\n", p)
			}
		}
	default:
		files = discover.IterSourceFiles([]string{root})
	}
	if len(filters) > 0 {
		var kept []string
		for _, p := range files {
			for _, f := range filters {
				if strings.Contains(p, f) {
					kept = append(kept, p)
					break
				}
			}
		}
		files = kept
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range files {
		r := discover.Resolve(p)
		if _, ok := discover.RelativeTo(r, root); !ok {
			fmt.Fprintf(stderr, "Skipping %s; it is outside %s\n", p, root)
			continue
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sortStable(out, func(a, b string) bool { return a < b })
	return out, nil
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
			text, err := scanFile(p, root, lines, opts.NoCoverage, opts.Lines)
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
			timeoutFactor: opts.TimeoutFactor, mutationWarning: opts.MutationWarning,
			baselines: baselines, maxWorkers: opts.MaxWorkers,
		})
		if err != nil {
			return 1, err
		}
		switch {
		case result.Skipped != "":
			fmt.Fprintf(stderr, "Skipping %s: %s\n", result.Path, result.Skipped)
		case result.BaselineFailed:
			fmt.Fprintln(stderr, result.BaselineMessage)
			baselineFailed = true
		default:
			fmt.Fprint(stdout, formatSiteLog(result.Sites, result.Statuses))
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
