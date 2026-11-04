// Package dry is `coderefinery dry`, the port of upstream dryer
// (docs/spec-dryer.md): file selection, per-language candidate extraction,
// normalization, structural fingerprints, Jaccard pairing and the text/EDN
// reports.
package dry

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

// Options is dryer cli.Options.
type Options struct {
	Help        bool
	Message     string
	ExitCode    int
	ProjectRoot string
	SourceRoots []string
	Positionals []string
	Changed     bool
	Threshold   float64
	MinLines    int
	MinNodes    int
	Format      string
}

func take(args []string, i int, opt string) (string, error) {
	if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
		return "", fmt.Errorf("%s requires a value", opt)
	}
	return args[i+1], nil
}

// ParseArgs is dryer cli.parse_args (spec-dryer §1.1).
func ParseArgs(args []string) Options {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return Options{Help: true, Message: Help}
		}
	}
	opts := Options{ProjectRoot: ".", Threshold: 0.82, MinLines: 4, MinNodes: 20, Format: "text"}
	usage := func(msg string) Options {
		return Options{Help: true, Message: msg + "\n\n" + Help, ExitCode: 1}
	}
	for i := 0; i < len(args); {
		arg := args[i]
		switch arg {
		case "-s", "--source-root", "--root", "--threshold", "--min-lines", "--min-nodes", "--format":
			v, err := take(args, i, arg)
			if err != nil {
				return usage(err.Error())
			}
			switch arg {
			case "--root":
				opts.ProjectRoot = v
			case "--threshold":
				t, err := pyfmt.ParseFloat(v)
				if err != nil {
					return usage(arg + " requires a number")
				}
				opts.Threshold = t
			case "--min-lines", "--min-nodes":
				n, err := pyfmt.ParseInt(v)
				if err != nil {
					return usage(arg + " requires an integer")
				}
				if n < 0 {
					return usage(arg + " requires a non-negative integer")
				}
				if arg == "--min-lines" {
					opts.MinLines = n
				} else {
					opts.MinNodes = n
				}
			case "--format":
				opts.Format = v
			default:
				opts.SourceRoots = append(opts.SourceRoots, v)
			}
			i += 2
			continue
		case "--changed":
			opts.Changed = true
		case "--edn":
			opts.Format = "edn"
		case "--text":
			opts.Format = "text"
		default:
			if strings.HasPrefix(arg, "-") {
				return usage("Unknown option: " + arg)
			}
			opts.Positionals = append(opts.Positionals, arg)
		}
		i++
	}
	return opts
}

// Run executes a parsed invocation: cli.run after the help/format checks.
func Run(opts Options, stdout, stderr io.Writer) (int, error) {
	root := discover.Resolve(opts.ProjectRoot)
	files, err := SelectFiles(opts)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		fmt.Fprintln(stdout, "No source files to analyze.")
		return 0, nil
	}
	entries, warnings, err := ScanFiles(files, root, opts.MinLines, opts.MinNodes)
	if err != nil {
		return 0, err
	}
	for _, w := range warnings {
		fmt.Fprintf(stderr, "dryer: %s\n", w)
	}
	candidates := FindDuplicates(entries, opts.Threshold)
	metrics, err := writeMetrics(candidates, root)
	if err != nil {
		return 0, err
	}
	if opts.Format == "edn" {
		fmt.Fprint(stdout, RenderEDN(candidates))
	} else {
		fmt.Fprint(stdout, FormatText(candidates))
	}
	fmt.Fprintf(stderr, "Wrote %s\n", metrics)
	return 0, nil
}

// Main runs `coderefinery dry` and returns the exit code.
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
	if opts.Format != "text" && opts.Format != "edn" {
		fmt.Fprintf(stderr, "Unknown format: %s\n", opts.Format)
		return 2
	}
	code, err := Run(opts, stdout, stderr)
	if err != nil {
		var gerr *discover.GitError
		if errors.As(err, &gerr) {
			fmt.Fprintln(stderr, gerr.Message)
			return gerr.Code
		}
		fmt.Fprintf(stderr, "dryer: %v\n", err)
		return 1
	}
	return code
}
