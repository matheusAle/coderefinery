// Command coderefinery bundles stdlib-only ports of Uncle Bob's crapper, dryer
// and mutator: `coderefinery crap|dry|mutate [flags] [paths]`, with the same
// flags, stdout, exit codes and .metrics files as the originals.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/dry"
	"github.com/matheusAle/coderefinery/internal/mutate"
	"github.com/matheusAle/coderefinery/internal/react"
	"github.com/matheusAle/coderefinery/internal/selfupdate"
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
  update   replace this binary with the latest release (--check: only report)

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
	bin := identify(version, info)
	args := os.Args[1:]
	command := ""
	if len(args) > 0 {
		command = args[0]
	}
	if len(args) == 1 && command == selfupdate.CheckCommand {
		// The detached update check started by StartNotice.
		selfupdate.RunCheck(os.Getenv, nil, time.Now())
		return
	}
	// The update notice goes to stderr after the command, only on a terminal
	// and never under CI (docs/DEVIATIONS.md X11); it never changes the exit code.
	notice := selfupdate.StartNotice(selfupdate.NoticeConfig{
		Binary: bin, Command: command, Getenv: os.Getenv, StderrIsTTY: isTerminal(os.Stderr),
	})
	code := run(args, bin, os.Stdout, os.Stderr)
	notice.Finish(os.Stderr)
	os.Exit(code)
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func run(args []string, bin selfupdate.Binary, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "--version", "version":
		fmt.Fprintf(stdout, "coderefinery %s\n", bin.Version)
		return 0
	case "update":
		return selfupdate.Main(args[1:], stdout, stderr, selfupdate.Options{Binary: bin})
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "coderefinery: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	return cmd(args[1:], stdout, stderr)
}

// identify names the running binary's version and how it was built: the
// -ldflags stamp of a release build; else the module version in the build
// info, which is a go install build when the module came from the module
// cache (it has a checksum) and a development build otherwise (go build in a
// checkout records a VCS pseudo-version, or "(devel)").
func identify(stamped string, info *debug.BuildInfo) selfupdate.Binary {
	if stamped != "" {
		return selfupdate.Binary{Version: stamped, Build: selfupdate.Release}
	}
	if info == nil || info.Main.Version == "" {
		return selfupdate.Binary{Version: "(devel)", Build: selfupdate.Dev}
	}
	if info.Main.Sum != "" {
		return selfupdate.Binary{Version: info.Main.Version, Build: selfupdate.GoInstall}
	}
	return selfupdate.Binary{Version: info.Main.Version, Build: selfupdate.Dev}
}
