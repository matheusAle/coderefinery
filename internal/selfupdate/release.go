// Package selfupdate implements `coderefinery update` and the passive "a newer
// version is available" notice.
//
// A release lives under one base URL (docs/release.md):
//
//	<base>/latest.json                     Manifest of the newest release
//	<base>/<version>/coderefinery_<v>_<os>_<arch>.tar.gz
//	<base>/<version>/SHA256SUMS            `shasum -a 256` of the archives
//	<base>/<version>/SHA256SUMS.sig        base64 ed25519 signature of SHA256SUMS
//
// latest.json is not signed: it only names the version. What gets installed is
// checked against SHA256SUMS, and SHA256SUMS against the public key built in
// here, so a tampered manifest or archive is refused.
package selfupdate

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// DefaultBaseURL is where releases are served (an R2 bucket behind a Pages
// Function on the coderefinery.dev site). BaseURLEnv overrides it.
const (
	DefaultBaseURL = "https://coderefinery.dev/dl"
	BaseURLEnv     = "CODEREFINERY_DOWNLOAD_URL"
	InstallCommand = "curl -fsSL https://coderefinery.dev/install.sh | sh"
	GoInstallPath  = "github.com/matheusAle/coderefinery/cmd/coderefinery"
	ManifestName   = "latest.json"
	SumsName       = "SHA256SUMS"
	SigName        = "SHA256SUMS.sig"
)

// publicKey verifies SHA256SUMS.sig. Its private half exists only as the
// CODEREFINERY_SIGNING_KEY GitHub Actions secret; rotating it means a new key
// here and a new release (docs/release.md).
const publicKey = "2rtdSya+hRi/tZ7YsTFSaUnDx83ffmyB8mcytRMSEJg="

// PublicKey returns the release signing key built into this binary.
func PublicKey() ed25519.PublicKey {
	key, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		panic("selfupdate: malformed built-in public key")
	}
	return key
}

// Asset is one platform's archive in a Manifest.
type Asset struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// Manifest is latest.json. Assets is keyed by "<GOOS>-<GOARCH>".
type Manifest struct {
	Version   string           `json:"version"`
	Published string           `json:"published"`
	Assets    map[string]Asset `json:"assets"`
}

// ParseSums reads a SHA256SUMS file ("<hex>  <file>" or "<hex> *<file>" per
// line) into file -> lowercase hex digest.
func ParseSums(data []byte) (map[string]string, error) {
	sums := map[string]string{}
	for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != 64 {
			return nil, fmt.Errorf("%s line %d is not \"<sha256>  <file>\": %q", SumsName, i+1, line)
		}
		sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	return sums, nil
}

// VerifySums checks sig (the contents of SHA256SUMS.sig) against sums.
func VerifySums(pub ed25519.PublicKey, sums, sig []byte) error {
	raw, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(sig)))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return errors.New("SHA256SUMS.sig is not a base64 ed25519 signature")
	}
	if !ed25519.Verify(pub, sums, raw) {
		return errors.New("SHA256SUMS signature does not match the release key")
	}
	return nil
}

// Sign returns the SHA256SUMS.sig contents for sums.
func Sign(key ed25519.PrivateKey, sums []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, sums)) + "\n")
}
