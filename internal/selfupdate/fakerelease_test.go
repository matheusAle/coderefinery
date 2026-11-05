package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

// testKey signs the fake releases; otherKey is a key the binary does not trust.
var (
	testKey  = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	otherKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
)

const testPlatform = "linux-amd64"

// fakeRelease is a release served the way the R2 bucket serves it.
type fakeRelease struct {
	files map[string][]byte // path below the base URL -> body
	mu    sync.Mutex
	hits  []string
}

// newFakeRelease builds release version whose archive holds bin (a shell
// script standing in for coderefinery), with a correct signed SHA256SUMS.
func newFakeRelease(t *testing.T, version string, bin []byte) *fakeRelease {
	t.Helper()
	archive := tarGz(t, "coderefinery", bin)
	file := "coderefinery_" + strings.TrimPrefix(version, "v") + "_linux_amd64.tar.gz" // as make dist names it
	sum := sha256.Sum256(archive)
	sums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), file)
	m := Manifest{Version: version, Published: "2026-11-05T00:00:00Z",
		Assets: map[string]Asset{testPlatform: {File: file, SHA256: hex.EncodeToString(sum[:])}}}
	manifest, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeRelease{files: map[string][]byte{
		ManifestName:             manifest,
		version + "/" + file:     archive,
		version + "/" + SumsName: []byte(sums),
		version + "/" + SigName:  Sign(testKey, []byte(sums)),
	}}
}

// serve starts an httptest server for the release; requests are recorded.
func (r *fakeRelease) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/dl/")
		r.mu.Lock()
		r.hits = append(r.hits, path)
		body, ok := r.files[path]
		r.mu.Unlock()
		if !ok {
			http.NotFound(w, req)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/dl"
}

func (r *fakeRelease) requested() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.hits...)
	sort.Strings(out)
	return out
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// script is a stand-in binary that prints what `coderefinery --version` would.
func script(version string) []byte {
	return []byte("#!/bin/sh\necho \"coderefinery " + version + "\"\n")
}
