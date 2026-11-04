package cliutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/discover"
)

func TestValueTakesOnlyAWordThatIsNotAnOption(t *testing.T) {
	args := []string{"--root", "src", "--threshold", "-1", "--format", "", "--last"}
	if v, err := Value(args, 0, "--root"); v != "src" || err != nil {
		t.Errorf("Value(--root) = %q, %v", v, err)
	}
	for _, i := range []int{2, 4, 6} {
		v, err := Value(args, i, args[i])
		if want := args[i] + " requires a value"; v != "" || err == nil || err.Error() != want {
			t.Errorf("Value(%s) = %q, %v; want error %q", args[i], v, err, want)
		}
	}
}

type outcome struct {
	code           int
	stdout, stderr string
	ran            bool
}

func runMain(help bool, message string, exitCode int, run func() (int, error)) outcome {
	var stdout, stderr strings.Builder
	var o outcome
	o.code = Main(help, message, exitCode, "tool", &stdout, &stderr, func() (int, error) { o.ran = true; return run() })
	o.stdout, o.stderr = stdout.String(), stderr.String()
	return o
}

func TestMainPrintsHelpToStdoutAndUsageErrorsToStderr(t *testing.T) {
	never := func() (int, error) { return 9, nil }
	if got, want := runMain(true, "Usage: x\n", 0, never), (outcome{0, "Usage: x\n", "", false}); got != want {
		t.Errorf("help: %+v, want %+v", got, want)
	}
	if got, want := runMain(true, "bad\n\nUsage: x\n", 1, never), (outcome{1, "", "bad\n\nUsage: x\n", false}); got != want {
		t.Errorf("usage error: %+v, want %+v", got, want)
	}
}

func TestMainMapsRunResultsToExitCodes(t *testing.T) {
	cases := []struct {
		name string
		run  func() (int, error)
		want outcome
	}{
		{"code passes through", func() (int, error) { return 3, nil }, outcome{3, "", "", true}},
		{"git error keeps git's status", func() (int, error) {
			return 0, fmt.Errorf("selecting: %w", &discover.GitError{Code: 128, Message: "fatal: not a git repository"})
		}, outcome{128, "", "fatal: not a git repository\n", true}},
		{"other errors are prefixed", func() (int, error) { return 2, errors.New("boom") }, outcome{1, "", "tool: boom\n", true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := runMain(false, "", 0, c.run); got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
