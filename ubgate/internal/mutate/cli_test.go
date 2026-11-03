package mutate

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpGoesToStdout(t *testing.T) {
	var out bytes.Buffer
	if code := Main([]string{"--lines", "--help"}, &out, &bytes.Buffer{}); code != 0 || out.String() != Help || out.Len() != 4236 {
		t.Fatalf("exit %d, %d bytes", code, out.Len())
	}
}

func TestParseArgs(t *testing.T) {
	o := ParseArgs([]string{"--lines", "3, 1,,2", "--timeout-factor", "2.5", "--max-workers", "4", "--reuse-coverage", "x"})
	if len(o.Lines) != 3 || !o.Lines[1] || !o.Lines[3] || o.TimeoutFactor != 2.5 || o.MaxWorkers != 4 || !o.UseExistingCoverage || o.Positionals[0] != "x" {
		t.Fatalf("parsed %+v", o)
	}
	errs := map[string][]string{
		"invalid literal for int() with base 10: 'x'":              {"--lines", "x"},
		"--lines requires positive line numbers":                   {"--lines", "0"},
		"--lines requires a line number":                           {"--lines", ","},
		"--timeout-factor requires a positive number":              {"--timeout-factor", "0"},
		"could not convert string to float: 'z'":                   {"--timeout-factor", "z"},
		"--max-workers requires a positive integer":                {"--max-workers", "0"},
		"--mutate-all cannot be combined with --since-last-run":    {"--mutate-all", "--since-last-run"},
		"--no-coverage cannot be combined with --coverage-command": {"--no-coverage", "--coverage-command", "c"},
		"--scan cannot be combined with --mutate-all":              {"--scan", "--mutate-all"},
		"--scan cannot be combined with --max-workers":             {"--scan", "--max-workers", "2"},
	}
	for msg, args := range errs {
		if o := ParseArgs(args); o.ExitCode != 1 || !strings.HasPrefix(o.Message, msg+"\n\n") {
			t.Errorf("%v -> %q", args, o.Message[:min(50, len(o.Message))])
		}
	}
}
