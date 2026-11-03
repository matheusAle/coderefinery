// Command ubgate bundles stdlib-only ports of Uncle Bob's crapper, dryer
// and mutator: `ubgate crap|dry|mutate [flags] [paths]`, with the same
// flags, stdout, exit codes and .metrics files as the originals.
package main

import (
	"fmt"
	"io"
	"os"

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
`

var commands = map[string]func(args []string, stdout, stderr io.Writer) int{
	"crap":   crap.Main,
	"dry":    dry.Main,
	"mutate": mutate.Main,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, usage)
		return
	}
	cmd, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "ubgate: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	os.Exit(cmd(os.Args[2:], os.Stdout, os.Stderr))
}
