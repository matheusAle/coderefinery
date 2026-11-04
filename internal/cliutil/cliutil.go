// Package cliutil is the command-line plumbing every subcommand shares: the
// option-value check and the tail of Main (help, run, error to exit code).
package cliutil

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/matheusAle/coderefinery/internal/discover"
)

// Value returns the value of the option at args[i]. The upstream parsers
// accept only a next word that is non-empty and does not start with "-".
func Value(args []string, i int, opt string) (string, error) {
	if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
		return "", fmt.Errorf("%s requires a value", opt)
	}
	return args[i+1], nil
}

// Main finishes a subcommand's Main once its arguments are parsed. With
// help set it prints message (stdout when exitCode is 0, else stderr) and
// returns exitCode. Otherwise it returns run's code; a *discover.GitError
// prints git's message and returns git's status, any other error prints
// "<prefix>: <error>" and returns 1.
func Main(help bool, message string, exitCode int, prefix string, stdout, stderr io.Writer, run func() (int, error)) int {
	if help {
		w := stdout
		if exitCode != 0 {
			w = stderr
		}
		fmt.Fprint(w, message)
		return exitCode
	}
	code, err := run()
	if err != nil {
		var gerr *discover.GitError
		if errors.As(err, &gerr) {
			fmt.Fprintln(stderr, gerr.Message)
			return gerr.Code
		}
		fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
		return 1
	}
	return code
}
