// Package testrun is `coderefinery test` (docs/design-tests.md §2): run each
// detected project's tests once with a JUnit reporter (go: -json) and branch
// coverage, and write .metrics/tests/.
package testrun

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

// Help is the `coderefinery test --help` text.
const Help = `Usage: coderefinery test [options] [path-or-filter ...]

Run each detected project's tests once, with a JUnit reporter (go test -json
for Go) and branch coverage, and write ` + "`" + `.metrics/tests/` + "`" + `: the results, the
coverage reports, and ` + "`" + `runs.edn` + "`" + `. ` + "`" + `coderefinery tests` + "`" + ` and
` + "`" + `coderefinery mutate --attribute` + "`" + ` read them.

Projects are found like mutate finds them: the nearest pyproject.toml /
pytest.ini / setup.cfg, go.mod, package.json, pom.xml or build.gradle,
Cargo.toml, deps.edn above each selected source file. Each runs once.

Options:
  -h, --help                Print this help and exit.
  --root <path>             Project root. Metrics are written here.
                            Default: the current directory.
  --test-command <cmd>      Run this command (/bin/sh -c, in the root)
                            instead. It must write JUnit XML or go test -json
                            to $CODEREFINERY_RESULTS; coverage reports may go
                            to $CODEREFINERY_COVERAGE_DIR.
  --no-coverage             Run the tests without coverage.
  --repeat <n>              Run the tests n times. A test whose status
                            differs between runs is recorded as flaky.
                            Default: 1.
  --changed                 Run the projects of added and modified files
                            from git status (test files included).
  --verbose                 Print more detail.

Exit status: 0 every test passed or was skipped; 1 usage error or a missing
tool or plugin (one line names what to install); 2 a test failed or errored,
the build or collection failed, or the runner wrote no results.
`

// Options is the parsed command line.
type Options struct {
	Help        bool
	Message     string
	ExitCode    int
	ProjectRoot string
	Positionals []string
	TestCommand string
	NoCoverage  bool
	Repeat      int
	Changed     bool
	Verbose     bool
}

// ParseArgs parses `coderefinery test` arguments (upstream usage style).
func ParseArgs(args []string) Options {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return Options{Help: true, Message: Help}
		}
	}
	opts := Options{ProjectRoot: ".", Repeat: 1}
	usage := func(msg string) Options { return Options{Help: true, Message: msg + "\n\n" + Help, ExitCode: 1} }
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--root", "--test-command", "--repeat":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return usage(arg + " requires a value")
			}
			v := args[i+1]
			i++
			switch arg {
			case "--root":
				opts.ProjectRoot = v
			case "--test-command":
				opts.TestCommand = v
			default:
				n, err := pyfmt.ParseInt(v)
				if err != nil || n < 1 {
					return usage("--repeat requires a positive integer")
				}
				opts.Repeat = n
			}
		case "--no-coverage":
			opts.NoCoverage = true
		case "--changed":
			opts.Changed = true
		case "--verbose":
			opts.Verbose = true
		default:
			if strings.HasPrefix(arg, "-") {
				return usage("Unknown option: " + arg)
			}
			opts.Positionals = append(opts.Positionals, arg)
		}
	}
	return opts
}

// Main runs `coderefinery test` and returns the exit code.
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
		fmt.Fprintf(stderr, "coderefinery test: %v\n", err)
		return 1
	}
	return code
}
