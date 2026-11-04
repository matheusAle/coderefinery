// Command coderefinery bundles stdlib-only ports of Uncle Bob's crapper, dryer
// and mutator: `coderefinery crap|dry|mutate [flags] [paths]`, with the same
// flags, stdout, exit codes and .metrics files as the originals.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/dry"
	"github.com/matheusAle/coderefinery/internal/mutate"
	"github.com/matheusAle/coderefinery/internal/react"
	"github.com/matheusAle/coderefinery/internal/testreport"
	"github.com/matheusAle/coderefinery/internal/testrun"
)

const usage = `Usage: coderefinery <command> [options] [path-or-filter ...]

Commands:
  crap     CRAP metric (crapper): complexity x coverage, .metrics/crap.edn
  dry      structural duplicates (dryer), .metrics/dry.edn
  mutate   mutation testing (mutator), .metrics/mutate/
  test     run the tests with JUnit results and coverage, .metrics/tests/
  tests    test relevance: kills, unique kills, shadowed tests, .metrics/tests.edn
  react    React components and hooks: per-callback CRAP, props, hooks, render cost, .metrics/react.edn

Run "coderefinery <command> --help" for the command's options.
Run "coderefinery --version" to print the version.
`

// version is stamped by release builds:
// go build -ldflags "-X main.version=v1.2.3" ./cmd/coderefinery
var version string

var commands = map[string]func(args []string, stdout, stderr io.Writer) int{
	"crap":   crap.Main,
	"dry":    dry.Main,
	"mutate": mutate.Main,
	"test":   testrun.Main,
	"tests":  testreport.Main,
	"react":  react.Main,
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
		fmt.Fprintf(stdout, "coderefinery %s\n", version)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "coderefinery: unknown command %q\n\n%s", args[0], usage)
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
