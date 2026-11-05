package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/matheusAle/coderefinery/internal/selfupdate"
)

var published = time.Date(2026, 11, 5, 12, 30, 0, 0, time.FixedZone("x", 3600))

func distDir(t *testing.T, sums string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runTool(args []string, env map[string]string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, func(k string) string { return env[k] }, func() time.Time { return published }, &out, &errb)
	return code, out.String(), errb.String()
}

func TestKeygenSignVerifyRoundTrip(t *testing.T) {
	code, stdout, stderr := runTool([]string{"keygen"}, nil)
	if code != 0 {
		t.Fatalf("keygen: %d %q", code, stderr)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stdout))
	if err != nil || len(seed) != ed25519.SeedSize {
		t.Fatalf("private key on stdout = %q", stdout)
	}
	pubText, ok := strings.CutPrefix(strings.TrimSpace(stderr), "public key: ")
	pub, err := base64.StdEncoding.DecodeString(pubText)
	if !ok || err != nil || !bytes.Equal(pub, ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)) {
		t.Fatalf("public key on stderr = %q", stderr)
	}

	dir := distDir(t, strings.Repeat("a", 64)+"  coderefinery_1.0.0_linux_amd64.tar.gz\n")
	if code, _, stderr := runTool([]string{"sign", dir}, map[string]string{KeyEnv: strings.TrimSpace(stdout)}); code != 0 {
		t.Fatalf("sign: %d %q", code, stderr)
	}
	if err := verify(dir, pub); err != nil {
		t.Errorf("verify with the generated key: %v", err)
	}
	// The built-in key is not this throwaway one.
	code, _, stderr = runTool([]string{"verify", dir}, nil)
	if code != 1 || stderr != "release: SHA256SUMS signature does not match the release key\n" {
		t.Errorf("verify with the built-in key: %d %q", code, stderr)
	}
}

func TestSignRefusesAMissingOrMalformedKey(t *testing.T) {
	dir := distDir(t, strings.Repeat("a", 64)+"  x.tar.gz\n")
	cases := map[string]string{
		"":               "release: CODEREFINERY_SIGNING_KEY is not set (the release signing key; docs/release.md)\n",
		"bm90IGEgc2VlZA": "release: CODEREFINERY_SIGNING_KEY is not a base64 ed25519 seed\n",
	}
	for key, want := range cases {
		code, _, stderr := runTool([]string{"sign", dir}, map[string]string{KeyEnv: key})
		if code != 1 || stderr != want {
			t.Errorf("key %q: %d %q, want %q", key, code, stderr, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "SHA256SUMS.sig")); !os.IsNotExist(err) {
		t.Errorf("a signature was written: %v", err)
	}
}

func TestManifestListsEveryPlatformOfTheVersion(t *testing.T) {
	a, b := strings.Repeat("a", 64), strings.Repeat("b", 64)
	dir := distDir(t, a+"  coderefinery_1.2.3_darwin_arm64.tar.gz\n"+b+"  coderefinery_1.2.3_linux_amd64.tar.gz\n")
	code, stdout, stderr := runTool([]string{"manifest", "v1.2.3", dir}, nil)
	if code != 0 {
		t.Fatalf("manifest: %d %q", code, stderr)
	}
	var got selfupdate.Manifest
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	want := selfupdate.Manifest{Version: "v1.2.3", Published: "2026-11-05T11:30:00Z", Assets: map[string]selfupdate.Asset{
		"darwin-arm64": {File: "coderefinery_1.2.3_darwin_arm64.tar.gz", SHA256: a},
		"linux-amd64":  {File: "coderefinery_1.2.3_linux_amd64.tar.gz", SHA256: b},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("manifest = %+v, want %+v", got, want)
	}
	// Smoke: the installer reads the version with a line-based sed.
	if !strings.Contains(stdout, "\n  \"version\": \"v1.2.3\",\n") {
		t.Errorf("version is not on its own line:\n%s", stdout)
	}
}

func TestManifestRefusesForeignFilesAndBadVersions(t *testing.T) {
	a := strings.Repeat("a", 64)
	cases := []struct {
		version, sums, want string
	}{
		{"v1.2.3", a + "  coderefinery_1.2.2_linux_amd64.tar.gz\n", "release: SHA256SUMS lists coderefinery_1.2.2_linux_amd64.tar.gz, which is not a v1.2.3 archive\n"},
		{"v1.2.3", a + "  notes.txt\n", "release: SHA256SUMS lists notes.txt, which is not a v1.2.3 archive\n"},
		{"1.2.3", a + "  coderefinery_1.2.3_linux_amd64.tar.gz\n", "release: version \"1.2.3\" is not vX.Y.Z\n"},
	}
	for _, c := range cases {
		code, stdout, stderr := runTool([]string{"manifest", c.version, distDir(t, c.sums)}, nil)
		if code != 1 || stdout != "" || stderr != c.want {
			t.Errorf("%s / %q: %d %q %q, want %q", c.version, c.sums, code, stdout, stderr, c.want)
		}
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"sign"}, {"manifest", "v1.0.0"}, {"nope"}} {
		if code, stdout, stderr := runTool(args, nil); code != 2 || stdout != "" || stderr != usage {
			t.Errorf("%v: %d %q %q", args, code, stdout, stderr)
		}
	}
}
