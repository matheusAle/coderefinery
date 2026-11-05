// Command release signs and describes a coderefinery release for the R2
// bucket (docs/release.md). The release workflow runs it; keygen is run once
// by hand, piping the private key straight into the GitHub secret.
//
//	go run ./tools/release keygen | gh secret set CODEREFINERY_SIGNING_KEY
//	    private key (base64 ed25519 seed) on stdout, public key on stderr
//	go run ./tools/release sign DIR       key from $CODEREFINERY_SIGNING_KEY; writes DIR/SHA256SUMS.sig
//	go run ./tools/release verify DIR     checks DIR/SHA256SUMS.sig against the key built into coderefinery
//	go run ./tools/release manifest VERSION DIR   latest.json for DIR/SHA256SUMS on stdout
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/matheusAle/coderefinery/internal/selfupdate"
)

// KeyEnv holds the private signing key (base64 of the 32-byte ed25519 seed).
const KeyEnv = "CODEREFINERY_SIGNING_KEY"

const usage = `usage: release keygen | sign DIR | verify DIR | manifest VERSION DIR
`

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, time.Now, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, now func() time.Time, stdout, stderr io.Writer) int {
	var err error
	switch {
	case len(args) == 1 && args[0] == "keygen":
		err = keygen(stdout, stderr)
	case len(args) == 2 && args[0] == "sign":
		err = sign(args[1], getenv(KeyEnv))
	case len(args) == 2 && args[0] == "verify":
		err = verify(args[1], selfupdate.PublicKey())
	case len(args) == 3 && args[0] == "manifest":
		err = manifest(stdout, args[1], args[2], now())
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "release: %v\n", err)
		return 1
	}
	return 0
}

// keygen prints a new private key on stdout (for `gh secret set`) and its
// public key on stderr (for selfupdate.publicKey). Nothing is written to disk.
func keygen(stdout, stderr io.Writer) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, base64.StdEncoding.EncodeToString(priv.Seed()))
	fmt.Fprintf(stderr, "public key: %s\n", base64.StdEncoding.EncodeToString(pub))
	return nil
}

func privateKey(encoded string) (ed25519.PrivateKey, error) {
	if encoded == "" {
		return nil, fmt.Errorf("%s is not set (the release signing key; docs/release.md)", KeyEnv)
	}
	seed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s is not a base64 ed25519 seed", KeyEnv)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func sign(dir, encodedKey string) error {
	key, err := privateKey(encodedKey)
	if err != nil {
		return err
	}
	sums, err := os.ReadFile(filepath.Join(dir, selfupdate.SumsName))
	if err != nil {
		return err
	}
	if _, err := selfupdate.ParseSums(sums); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, selfupdate.SigName), selfupdate.Sign(key, sums), 0o644)
}

func verify(dir string, pub ed25519.PublicKey) error {
	sums, err := os.ReadFile(filepath.Join(dir, selfupdate.SumsName))
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(filepath.Join(dir, selfupdate.SigName))
	if err != nil {
		return err
	}
	return selfupdate.VerifySums(pub, sums, sig)
}

// archiveRE matches the archives `make dist` writes.
var archiveRE = regexp.MustCompile(`^coderefinery_(.+)_([a-z0-9]+)_([a-z0-9]+)\.tar\.gz$`)

func manifest(stdout io.Writer, version, dir string, now time.Time) error {
	if !selfupdate.ValidVersion(version) {
		return fmt.Errorf("version %q is not vX.Y.Z", version)
	}
	data, err := os.ReadFile(filepath.Join(dir, selfupdate.SumsName))
	if err != nil {
		return err
	}
	sums, err := selfupdate.ParseSums(data)
	if err != nil {
		return err
	}
	m := selfupdate.Manifest{Version: version, Published: now.UTC().Format(time.RFC3339), Assets: map[string]selfupdate.Asset{}}
	for file, sum := range sums {
		parts := archiveRE.FindStringSubmatch(file)
		if parts == nil || "v"+parts[1] != version {
			return fmt.Errorf("%s lists %s, which is not a %s archive", selfupdate.SumsName, file, version)
		}
		m.Assets[parts[2]+"-"+parts[3]] = selfupdate.Asset{File: file, SHA256: sum}
	}
	if len(m.Assets) == 0 {
		return errors.New(selfupdate.SumsName + " lists no archives")
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", out)
	return err
}
