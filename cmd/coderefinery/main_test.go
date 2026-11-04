package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func runCapture(args []string, version string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, version, &out, &errb)
	return code, out.String(), errb.String()
}

func TestVersionFlagPrintsTheVersionOnStdout(t *testing.T) {
	for _, arg := range []string{"--version", "version"} {
		code, stdout, stderr := runCapture([]string{arg}, "v1.2.3")
		if code != 0 || stdout != "coderefinery v1.2.3\n" || stderr != "" {
			t.Fatalf("%s: got code=%d stdout=%q stderr=%q", arg, code, stdout, stderr)
		}
	}
}

func TestHelpMentionsVersionAndNoArgsIsAUsageError(t *testing.T) {
	code, stdout, _ := runCapture([]string{"--help"}, "v1")
	if code != 0 || stdout != usage || !strings.Contains(stdout, `Run "coderefinery --version" to print the version.`) {
		t.Fatalf("help: code=%d stdout=%q", code, stdout)
	}
	code, stdout, stderr := runCapture(nil, "v1")
	if code != 2 || stdout != "" || stderr != usage {
		t.Fatalf("no args: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestUnknownCommandNamesItAndExits2(t *testing.T) {
	code, stdout, stderr := runCapture([]string{"--versions"}, "v1")
	if code != 2 || stdout != "" || stderr != "coderefinery: unknown command \"--versions\"\n\n"+usage {
		t.Fatalf("got code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestSubcommandHelpIsDispatchedUnchanged(t *testing.T) {
	code, stdout, _ := runCapture([]string{"dry", "--help"}, "v1")
	if code != 0 || !strings.HasPrefix(stdout, "Usage: dryer [options] [path-or-filter ...]\n") {
		t.Fatalf("got code=%d stdout=%q", code, stdout)
	}
}

func TestResolveVersionPrefersStampThenBuildInfoThenDevel(t *testing.T) {
	installed := &debug.BuildInfo{Main: debug.Module{Version: "v0.4.0"}}
	cases := []struct {
		stamped string
		info    *debug.BuildInfo
		want    string
	}{
		{"v9.9.9", installed, "v9.9.9"},
		{"", installed, "v0.4.0"},
		{"", &debug.BuildInfo{}, "(devel)"},
		{"", nil, "(devel)"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.stamped, c.info); got != c.want {
			t.Errorf("resolveVersion(%q, %+v) = %q, want %q", c.stamped, c.info, got, c.want)
		}
	}
}

// Smoke: the release recipe's -ldflags stamp reaches `coderefinery --version`.
func TestReleaseLdflagsStampTheBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	bin := filepath.Join(t.TempDir(), "coderefinery")
	build := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w -X main.version=v7.8.9", "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "--version").Output()
	if err != nil || string(out) != "coderefinery v7.8.9\n" {
		t.Fatalf("got %q, %v", out, err)
	}
}
