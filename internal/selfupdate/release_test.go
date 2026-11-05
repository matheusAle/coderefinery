package selfupdate

import (
	"crypto/ed25519"
	"reflect"
	"strings"
	"testing"
)

func TestBuiltInPublicKeyIsAnEd25519Key(t *testing.T) {
	if got := len(PublicKey()); got != ed25519.PublicKeySize {
		t.Fatalf("len = %d", got)
	}
}

func TestParseSumsReadsBothShasumForms(t *testing.T) {
	a, b := strings.Repeat("a", 64), strings.Repeat("B", 64)
	got, err := ParseSums([]byte(a + "  one.tar.gz\n" + b + " *two.tar.gz\n"))
	want := map[string]string{"one.tar.gz": a, "two.tar.gz": strings.ToLower(b)}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
	for _, bad := range []string{"abc  short.tar.gz\n", a + "\n", a + "  x  y\n", ""} {
		if _, err := ParseSums([]byte(bad)); err == nil || !strings.Contains(err.Error(), `is not "<sha256>  <file>"`) {
			t.Errorf("ParseSums(%q) err = %v", bad, err)
		}
	}
}

func TestSignatureRoundTripAndRefusals(t *testing.T) {
	sums := []byte(strings.Repeat("c", 64) + "  a.tar.gz\n")
	pub := testKey.Public().(ed25519.PublicKey)
	sig := Sign(testKey, sums)
	if !strings.HasSuffix(string(sig), "\n") || len(sig) != 89 {
		t.Errorf("signature file = %q, want one base64 line", sig)
	}
	if err := VerifySums(pub, sums, sig); err != nil {
		t.Errorf("good signature: %v", err)
	}
	cases := map[string]struct {
		pub  ed25519.PublicKey
		sums []byte
		sig  []byte
		want string
	}{
		"other key":   {otherKey.Public().(ed25519.PublicKey), sums, sig, "SHA256SUMS signature does not match the release key"},
		"edited sums": {pub, append([]byte("x"), sums...), sig, "SHA256SUMS signature does not match the release key"},
		"truncated":   {pub, sums, sig[:40], "SHA256SUMS.sig is not a base64 ed25519 signature"},
		"empty":       {pub, sums, nil, "SHA256SUMS.sig is not a base64 ed25519 signature"},
	}
	for name, c := range cases {
		if err := VerifySums(c.pub, c.sums, c.sig); err == nil || err.Error() != c.want {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}
