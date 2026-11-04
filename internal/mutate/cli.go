// Package mutate is `coderefinery mutate`, the port of upstream mutator
// (docs/spec-mutator.md): site discovery per language, coverage gating,
// differential selection, parallel worker overlays and .metrics/mutate
// snapshots, byte for byte.
package mutate

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

// Options is mutator cli.Options.
type Options struct {
	Help                bool
	Message             string
	ExitCode            int
	ProjectRoot         string
	SourceRoots         []string
	Positionals         []string
	Scan                bool
	MutateAll           bool
	SinceLastRun        bool
	Lines               map[int]bool // nil = None
	NoCoverage          bool
	UseExistingCoverage bool
	CoverageCommand     string
	TestCommand         string
	TimeoutFactor       float64
	MutationWarning     int
	MaxWorkers          int // 0 = None (cores)
	Changed             bool
	Verbose             bool
	// FixGoCoverage maps Go coverprofile import paths to files through
	// go.mod (coderefinery-only, DEVIATIONS.md M1). Off = upstream behaviour.
	FixGoCoverage bool
	// Attribute records which tests kill each mutant (:killed-by,
	// docs/design-tests.md §3). Hidden from --help.
	Attribute bool
	// Branches adds an untested-branch reason to UNCOVERED site lines
	// (docs/design-tests.md §6). Hidden from --help.
	Branches bool
	// MinTimeout is the floor of a mutant's timeout in seconds (upstream: a
	// fixed 2). Hidden from --help; tests raise it so a load spike cannot
	// turn a survivor into a timeout kill (DEVIATIONS.md, "mutate --min-timeout").
	MinTimeout float64
	// FixOverlay links skip-listed directories that hold the mutated file's
	// language into worker overlays (DEVIATIONS.md M8). Hidden from --help;
	// --attribute turns it on too.
	FixOverlay bool
}

func take(args []string, i int, opt string) (string, error) {
	if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
		return "", fmt.Errorf("%s requires a value", opt)
	}
	return args[i+1], nil
}

func parseLines(value string) (map[int]bool, error) {
	found := map[int]bool{}
	for _, piece := range strings.Split(value, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		n, err := pyfmt.ParseInt(piece)
		if err != nil {
			return nil, err
		}
		if n < 1 {
			return nil, errors.New("--lines requires positive line numbers")
		}
		found[n] = true
	}
	if len(found) == 0 {
		return nil, errors.New("--lines requires a line number")
	}
	return found, nil
}

// ParseArgs is mutator cli.parse_args (spec-mutator §1.1).
func ParseArgs(args []string) Options {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return Options{Help: true, Message: Help}
		}
	}
	opts := Options{ProjectRoot: ".", TimeoutFactor: 10.0, MutationWarning: 50, MinTimeout: 2.0}
	usage := func(msg string) Options {
		return Options{Help: true, Message: msg + "\n\n" + Help, ExitCode: 1}
	}
	for i := 0; i < len(args); {
		arg := args[i]
		switch arg {
		case "-s", "--source-root", "--root", "--coverage-command", "--test-command", "--timeout-factor",
			"--mutation-warning", "--max-workers", "--lines", "--min-timeout":
			v, err := take(args, i, arg)
			if err != nil {
				return usage(err.Error())
			}
			switch arg {
			case "--root":
				opts.ProjectRoot = v
			case "--coverage-command":
				opts.CoverageCommand = v
			case "--test-command":
				opts.TestCommand = v
			case "--timeout-factor":
				f, err := pyfmt.ParseFloat(v)
				if err != nil {
					return usage(pyfmt.FloatError(v).Error())
				}
				if !(f > 0) {
					return usage("--timeout-factor requires a positive number")
				}
				opts.TimeoutFactor = f
			case "--min-timeout":
				f, err := pyfmt.ParseFloat(v)
				if err != nil || !(f > 0) || math.IsInf(f, 1) {
					return usage("--min-timeout requires a positive number of seconds")
				}
				opts.MinTimeout = f
			case "--mutation-warning":
				n, err := pyfmt.ParseInt(v)
				if err != nil {
					return usage(err.Error())
				}
				opts.MutationWarning = n
			case "--max-workers":
				n, err := pyfmt.ParseInt(v)
				if err != nil || n < 1 {
					return usage("--max-workers requires a positive integer")
				}
				opts.MaxWorkers = n
			case "--lines":
				lines, err := parseLines(v)
				if err != nil {
					return usage(err.Error())
				}
				opts.Lines = lines
			default:
				opts.SourceRoots = append(opts.SourceRoots, v)
			}
			i += 2
			continue
		case "--no-coverage":
			opts.NoCoverage = true
		case "--use-existing-coverage", "--reuse-coverage":
			opts.UseExistingCoverage = true
		case "--changed":
			opts.Changed = true
		case "--scan":
			opts.Scan = true
		case "--mutate-all":
			opts.MutateAll = true
		case "--since-last-run":
			opts.SinceLastRun = true
		case "--verbose":
			opts.Verbose = true
		case "--fix-go-coverage":
			opts.FixGoCoverage = true
		case "--attribute":
			opts.Attribute = true
		case "--branches":
			opts.Branches = true
		case "--fix-overlay":
			opts.FixOverlay = true
		default:
			if strings.HasPrefix(arg, "-") {
				return usage("Unknown option: " + arg)
			}
			opts.Positionals = append(opts.Positionals, arg)
		}
		i++
	}
	switch {
	case opts.MutateAll && opts.SinceLastRun:
		return usage("--mutate-all cannot be combined with --since-last-run")
	case opts.NoCoverage && opts.CoverageCommand != "":
		return usage("--no-coverage cannot be combined with --coverage-command")
	case opts.Scan && opts.MutateAll:
		return usage("--scan cannot be combined with --mutate-all")
	case opts.Scan && opts.MaxWorkers != 0:
		return usage("--scan cannot be combined with --max-workers")
	}
	return opts
}

// Main runs `coderefinery mutate` and returns the exit code.
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
		fmt.Fprintf(stderr, "mutator: %v\n", err)
		return 1
	}
	return code
}
