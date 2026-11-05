package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/selfupdate"
)

func runCapture(args []string, version string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, selfupdate.Binary{Version: version, Build: selfupdate.Release}, &out, &errb)
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

func TestIdentifyPrefersStampThenGoInstallThenDevel(t *testing.T) {
	goInstalled := &debug.BuildInfo{Main: debug.Module{Version: "v0.4.0", Sum: "h1:abc="}}
	checkout := &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261105145933-cf1fb6a9d8ea+dirty"}}
	cases := []struct {
		name    string
		stamped string
		info    *debug.BuildInfo
		want    selfupdate.Binary
	}{
		{"release stamp wins", "v9.9.9", goInstalled, selfupdate.Binary{Version: "v9.9.9", Build: selfupdate.Release}},
		{"module from the module cache", "", goInstalled, selfupdate.Binary{Version: "v0.4.0", Build: selfupdate.GoInstall}},
		{"go build in a checkout", "", checkout, selfupdate.Binary{Version: "v0.0.0-20261105145933-cf1fb6a9d8ea+dirty", Build: selfupdate.Dev}},
		{"(devel)", "", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, selfupdate.Binary{Version: "(devel)", Build: selfupdate.Dev}},
		{"no version", "", &debug.BuildInfo{}, selfupdate.Binary{Version: "(devel)", Build: selfupdate.Dev}},
		{"no build info", "", nil, selfupdate.Binary{Version: "(devel)", Build: selfupdate.Dev}},
	}
	for _, c := range cases {
		if got := identify(c.stamped, c.info); got != c.want {
			t.Errorf("%s: identify = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestUpdateIsDispatchedAndListed(t *testing.T) {
	code, stdout, stderr := runCapture([]string{"update", "--help"}, "v1.0.0")
	if code != 0 || stdout != selfupdate.Help || stderr != "" {
		t.Fatalf("update --help: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(usage, "\n  update   ") {
		t.Errorf("usage does not list update:\n%s", usage)
	}
	code, stdout, stderr = runCapture([]string{"update", "--nope"}, "v1.0.0")
	if code != 1 || stdout != "" || stderr != "Unknown option: --nope\n\n"+selfupdate.Help {
		t.Fatalf("update --nope: code=%d stdout=%q stderr=%q", code, stdout, stderr)
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

func TestAdditionsAreDispatched(t *testing.T) {
	for _, c := range []struct{ cmd, prefix string }{
		{"test", "Usage: coderefinery test [options] [path-or-filter ...]\n"},
		{"tests", "Usage: coderefinery tests [options]\n"},
		{"react", "Usage: coderefinery react [options] [path-or-filter ...]\n"},
	} {
		code, stdout, _ := runCapture([]string{c.cmd, "--help"}, "v1")
		if code != 0 || !strings.HasPrefix(stdout, c.prefix) {
			t.Errorf("%s --help: code=%d stdout=%q", c.cmd, code, stdout[:min(60, len(stdout))])
		}
		if !strings.Contains(usage, "\n  "+c.cmd+" ") {
			t.Errorf("usage lists %s", c.cmd)
		}
	}
}
