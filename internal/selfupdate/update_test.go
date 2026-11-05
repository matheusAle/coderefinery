package selfupdate

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installed writes an "old" coderefinery into a fresh directory and returns its path.
func installed(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir()) // macOS: /var -> /private/var
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "coderefinery")
	if err := os.WriteFile(exe, script("v0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func opts(base, exe string, b Build) Options {
	return Options{Binary: Binary{Version: "v0.1.0", Build: b}, Exe: exe, BaseURL: base,
		PublicKey: testKey.Public().(ed25519.PublicKey), Platform: testPlatform}.withDefaults()
}

// dirEntries lists the names in exe's directory (no temp file may be left).
func dirEntries(t *testing.T, exe string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(exe))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func assertUntouched(t *testing.T, exe string) {
	t.Helper()
	got, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(got, script("v0.1.0")) {
		t.Errorf("binary changed: %q, %v", got, err)
	}
	if names := dirEntries(t, exe); strings.Join(names, ",") != "coderefinery" {
		t.Errorf("directory holds %v, want only coderefinery", names)
	}
}

func TestUpdateReplacesTheBinaryWithTheVerifiedRelease(t *testing.T) {
	rel := newFakeRelease(t, "v0.2.0", script("v0.2.0"))
	exe := installed(t)
	var out bytes.Buffer
	err := Update(context.Background(), &out, opts(rel.serve(t), exe, Release))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if want := "updated coderefinery v0.1.0 -> v0.2.0 (" + exe + ")\n"; out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
	got, _ := os.ReadFile(exe)
	if !bytes.Equal(got, script("v0.2.0")) {
		t.Errorf("binary = %q, want the release's", got)
	}
	if names := dirEntries(t, exe); strings.Join(names, ",") != "coderefinery" {
		t.Errorf("directory holds %v, want only coderefinery", names)
	}
	want := []string{"latest.json", "v0.2.0/SHA256SUMS", "v0.2.0/SHA256SUMS.sig", "v0.2.0/coderefinery_0.2.0_linux_amd64.tar.gz"}
	if got := rel.requested(); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("requested %v, want %v", got, want)
	}
	// Smoke: the replaced file is what runs now.
	if v, err := exec.Command(exe, "--version").Output(); err != nil || string(v) != "coderefinery v0.2.0\n" {
		t.Errorf("new binary --version = %q, %v", v, err)
	}
}

func TestUpdateThroughASymlinkReplacesTheTarget(t *testing.T) {
	rel := newFakeRelease(t, "v0.2.0", script("v0.2.0"))
	exe := installed(t)
	link := filepath.Join(t.TempDir(), "coderefinery")
	if err := os.Symlink(exe, link); err != nil {
		t.Fatal(err)
	}
	if err := Update(context.Background(), &bytes.Buffer{}, opts(rel.serve(t), link, Release)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if target, err := os.Readlink(link); err != nil || target != exe {
		t.Errorf("link now points to %q, %v", target, err)
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, script("v0.2.0")) {
		t.Errorf("target = %q, want the release's", got)
	}
}

func TestUpdateRefusesAndKeepsTheBinary(t *testing.T) {
	cases := []struct {
		name    string
		tamper  func(r *fakeRelease)
		wantErr string
		// fetched: whether the archive is downloaded before the refusal.
		fetched bool
	}{
		{"signature by another key", func(r *fakeRelease) {
			r.files["v0.2.0/"+SigName] = Sign(otherKey, r.files["v0.2.0/"+SumsName])
		}, "release v0.2.0: SHA256SUMS signature does not match the release key; not updated", false},
		{"SHA256SUMS edited after signing", func(r *fakeRelease) {
			r.files["v0.2.0/"+SumsName] = append(r.files["v0.2.0/"+SumsName], "0000000000000000000000000000000000000000000000000000000000000000  extra\n"...)
		}, "SHA256SUMS signature does not match the release key", false},
		{"signature file is garbage", func(r *fakeRelease) {
			r.files["v0.2.0/"+SigName] = []byte("not base64!\n")
		}, "SHA256SUMS.sig is not a base64 ed25519 signature", false},
		{"archive does not match the signed sum", func(r *fakeRelease) {
			r.files["v0.2.0/coderefinery_0.2.0_linux_amd64.tar.gz"] = tarGz(t, "coderefinery", script("v6.6.6"))
		}, "checksum mismatch for coderefinery_0.2.0_linux_amd64.tar.gz (SHA256SUMS: ", true},
		{"no build for this platform", func(r *fakeRelease) {
			r.files[ManifestName] = []byte(`{"version":"v0.2.0","assets":{"plan9-mips":{"file":"x.tar.gz"}}}`)
		}, "release v0.2.0 has no build for linux-amd64", false},
		{"binary that does not run as the release", func(r *fakeRelease) {
			*r = *newFakeRelease(t, "v0.2.0", script("v0.1.9"))
		}, `the downloaded binary does not run here (--version: "coderefinery v0.1.9", <nil>); not updated`, true},
		{"manifest without a version", func(r *fakeRelease) {
			r.files[ManifestName] = []byte(`{"version":"latest"}`)
		}, `latest.json has no valid version ("latest")`, false},
		{"missing SHA256SUMS", func(r *fakeRelease) {
			delete(r.files, "v0.2.0/"+SumsName)
		}, "v0.2.0/SHA256SUMS: 404 Not Found", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel := newFakeRelease(t, "v0.2.0", script("v0.2.0"))
			c.tamper(rel)
			exe := installed(t)
			var out bytes.Buffer
			err := Update(context.Background(), &out, opts(rel.serve(t), exe, Release))
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, c.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", out.String())
			}
			assertUntouched(t, exe)
			fetched := false
			for _, p := range rel.requested() {
				fetched = fetched || strings.HasSuffix(p, ".tar.gz")
			}
			if fetched != c.fetched {
				t.Errorf("archive fetched = %v, want %v (requests %v)", fetched, c.fetched, rel.requested())
			}
		})
	}
}

func TestUpdateWhenNothingIsNewerDownloadsOnlyTheManifest(t *testing.T) {
	rel := newFakeRelease(t, "v0.1.0", script("v0.1.0"))
	exe := installed(t)
	var out bytes.Buffer
	if err := Update(context.Background(), &out, opts(rel.serve(t), exe, Release)); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if out.String() != "coderefinery v0.1.0 is up to date\n" {
		t.Errorf("stdout = %q", out.String())
	}
	if got := rel.requested(); strings.Join(got, " ") != "latest.json" {
		t.Errorf("requested %v, want only latest.json", got)
	}
	assertUntouched(t, exe)
}

func TestUpdateInAReadOnlyDirectorySaysHowToFixIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to read-only directories")
	}
	rel := newFakeRelease(t, "v0.2.0", script("v0.2.0"))
	exe := installed(t)
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	err := Update(context.Background(), &bytes.Buffer{}, opts(rel.serve(t), exe, Release))
	want := "cannot write to " + dir + " (permission denied). Run the update as a user who can write there, or reinstall into a directory you own: " + InstallCommand
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	assertUntouched(t, exe)
}

func TestUpdateRefusesGoInstallAndDevBuildsWithoutTouchingTheNetwork(t *testing.T) {
	cases := []struct {
		build Build
		want  string
	}{
		{GoInstall, "this coderefinery (v0.1.0) was built by go install; update it with: go install github.com/matheusAle/coderefinery/cmd/coderefinery@latest"},
		{Dev, "this is a development build (v0.1.0); rebuild it from your checkout, or install a release: " + InstallCommand},
	}
	for _, c := range cases {
		rel := newFakeRelease(t, "v0.2.0", script("v0.2.0"))
		exe := installed(t)
		err := Update(context.Background(), &bytes.Buffer{}, opts(rel.serve(t), exe, c.build))
		if err == nil || err.Error() != c.want {
			t.Errorf("build %d: err = %v, want %q", c.build, err, c.want)
		}
		if got := rel.requested(); len(got) != 0 {
			t.Errorf("build %d: requested %v, want nothing", c.build, got)
		}
		assertUntouched(t, exe)
	}
}

func TestCheckReportsWithoutChangingAnything(t *testing.T) {
	cases := []struct {
		name    string
		latest  string
		build   Build
		wantOut string
	}{
		{"newer release", "v0.2.0", Release, "coderefinery v0.2.0 is available (you have v0.1.0). Run: coderefinery update\n"},
		{"newer, go install build", "v0.2.0", GoInstall, "coderefinery v0.2.0 is available (you have v0.1.0). Run: go install github.com/matheusAle/coderefinery/cmd/coderefinery@latest\n"},
		{"same version", "v0.1.0", Release, "coderefinery v0.1.0 is up to date\n"},
		{"older release", "v0.0.9", Release, "coderefinery v0.1.0 is up to date\n"},
		{"development build", "v0.2.0", Dev, "the latest release is v0.2.0; this is a development build (v0.1.0). Install a release: " + InstallCommand + "\n"},
	}
	for _, c := range cases {
		rel := newFakeRelease(t, c.latest, script(c.latest))
		exe := installed(t)
		var out bytes.Buffer
		if err := Check(context.Background(), &out, opts(rel.serve(t), exe, c.build)); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if out.String() != c.wantOut {
			t.Errorf("%s: stdout = %q, want %q", c.name, out.String(), c.wantOut)
		}
		if got := rel.requested(); strings.Join(got, " ") != "latest.json" {
			t.Errorf("%s: requested %v", c.name, got)
		}
		assertUntouched(t, exe)
	}
}

func TestMainExitCodesAndStreams(t *testing.T) {
	rel := newFakeRelease(t, "v0.2.0", script("v0.2.0"))
	base := rel.serve(t)
	cases := []struct {
		name               string
		args               []string
		code               int
		stdout, stderrPart string
	}{
		{"help", []string{"--help"}, 0, Help, ""},
		{"help wins over a bad option", []string{"--bogus", "-h"}, 0, Help, ""},
		{"unknown option", []string{"--bogus"}, 1, "", "Unknown option: --bogus\n\n" + Help},
		{"stray argument", []string{"now"}, 1, "", "Unexpected argument: now\n\n" + Help},
		{"check", []string{"--check"}, 0, "coderefinery v0.2.0 is available (you have v0.1.0). Run: coderefinery update\n", ""},
		{"failure", nil, 1, "", "coderefinery update: release v0.2.0 has no build for plan9-mips\n"},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		o := Options{Binary: Binary{Version: "v0.1.0", Build: Release}, Exe: installed(t), BaseURL: base,
			PublicKey: testKey.Public().(ed25519.PublicKey), Platform: "plan9-mips"}
		code := Main(c.args, &out, &errb, o)
		if code != c.code || out.String() != c.stdout || errb.String() != c.stderrPart {
			t.Errorf("%s: code=%d stdout=%q stderr=%q, want %d %q %q", c.name, code, out.String(), errb.String(), c.code, c.stdout, c.stderrPart)
		}
	}
}

func TestBaseURLPrefersTheEnvironment(t *testing.T) {
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }
	if got := BaseURL(getenv); got != DefaultBaseURL {
		t.Errorf("default = %q", got)
	}
	env[BaseURLEnv] = "https://mirror.example/dl/"
	if got := BaseURL(getenv); got != "https://mirror.example/dl" {
		t.Errorf("override = %q", got)
	}
}
