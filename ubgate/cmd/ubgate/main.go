// Command ubgate bundles stdlib-only ports of Uncle Bob's crapper, dryer
// and mutator: `ubgate crap|dry|mutate [flags] [paths]`, with the same
// flags, stdout, exit codes and .metrics files as the originals.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/matheusAle/ub-mode/ubgate/internal/crap"
	"github.com/matheusAle/ub-mode/ubgate/internal/dry"
	"github.com/matheusAle/ub-mode/ubgate/internal/mutate"
)

const usage = `Usage: ubgate <command> [options] [path-or-filter ...]

Commands:
  crap     CRAP metric (crapper): complexity x coverage, .metrics/crap.edn
  dry      structural duplicates (dryer), .metrics/dry.edn
  mutate   mutation testing (mutator), .metrics/mutate/

Run "ubgate <command> --help" for the command's options.
Run "ubgate --version" to print the version.
`

// version is stamped by release builds:
// go build -ldflags "-X main.version=v1.2.3" ./cmd/ubgate
var version string

var commands = map[string]func(args []string, stdout, stderr io.Writer) int{
	"crap":   crap.Main,
	"dry":    dry.Main,
	"mutate": mutate.Main,
}

func main() {
	info, _ := debug.ReadBuildInfo()
	os.Exit(run(os.Args[1:], resolveVersion(version, info), os.Stdout, os.Stderr))
}

func run(args []string, version string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "--version", "version":
		fmt.Fprintf(stdout, "ubgate %s\n", version)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "ubgate: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	return cmd(args[1:], stdout, stderr)
}

// resolveVersion prefers the -ldflags stamp, then the module version that
// `go install …@vX.Y.Z` records in the build info, then "(devel)".
func resolveVersion(stamped string, info *debug.BuildInfo) string {
	if stamped != "" {
		return stamped
	}
	if info != nil && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}
