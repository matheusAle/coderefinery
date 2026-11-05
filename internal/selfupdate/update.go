package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/matheusAle/coderefinery/internal/cliutil"
)

// Help is `coderefinery update --help`.
const Help = `Usage: coderefinery update [--check]

Replaces this coderefinery with the latest release, after checking the
archive against the release's SHA256SUMS and SHA256SUMS against the
signing key built into coderefinery.

Options:
  --check    only report whether a newer release exists
  -h, --help show this help

Release downloads come from $CODEREFINERY_DOWNLOAD_URL (default
https://coderefinery.dev/dl). Builds made with go install update with
go install; development builds do not update.
`

// Options configures Main. Zero values mean the real thing: the running
// executable, $CODEREFINERY_DOWNLOAD_URL or DefaultBaseURL, the built-in key.
type Options struct {
	Binary    Binary
	Exe       string
	BaseURL   string
	Client    *http.Client
	PublicKey ed25519.PublicKey
	Platform  string // "<GOOS>-<GOARCH>"
}

func (o Options) withDefaults() Options {
	if o.BaseURL == "" {
		o.BaseURL = BaseURL(os.Getenv)
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 5 * time.Minute}
	}
	if o.PublicKey == nil {
		o.PublicKey = PublicKey()
	}
	if o.Platform == "" {
		o.Platform = runtime.GOOS + "-" + runtime.GOARCH
	}
	return o
}

// BaseURL is the release base URL: $CODEREFINERY_DOWNLOAD_URL, else DefaultBaseURL.
func BaseURL(getenv func(string) string) string {
	if u := strings.TrimRight(getenv(BaseURLEnv), "/"); u != "" {
		return u
	}
	return DefaultBaseURL
}

// parseArgs returns --check, or help with the message to print and its exit
// code (0 for --help, 1 for a usage error), as the other subcommands do.
func parseArgs(args []string) (check, help bool, message string, exitCode int) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return false, true, Help, 0
		}
	}
	for _, a := range args {
		switch {
		case a == "--check":
			check = true
		case strings.HasPrefix(a, "-"):
			return false, true, "Unknown option: " + a + "\n\n" + Help, 1
		default:
			return false, true, "Unexpected argument: " + a + "\n\n" + Help, 1
		}
	}
	return check, false, "", 0
}

// Main runs `coderefinery update [--check]`: exit 0 when updated or up to
// date, 1 on a failure or a usage error. Errors leave the binary untouched.
func Main(args []string, stdout, stderr io.Writer, opts Options) int {
	check, help, message, exitCode := parseArgs(args)
	return cliutil.Main(help, message, exitCode, "coderefinery update", stdout, stderr, func() (int, error) {
		opts = opts.withDefaults()
		if check {
			return 0, Check(context.Background(), stdout, opts)
		}
		return 0, Update(context.Background(), stdout, opts)
	})
}

// howToUpdate is the command that updates a Release or GoInstall build.
func howToUpdate(b Build) string {
	if b == GoInstall {
		return "go install " + GoInstallPath + "@latest"
	}
	return "coderefinery update"
}

// Check prints whether a release newer than the running binary exists.
func Check(ctx context.Context, stdout io.Writer, opts Options) error {
	m, err := FetchManifest(ctx, opts.Client, opts.BaseURL)
	if err != nil {
		return err
	}
	cur := opts.Binary.Version
	switch {
	case opts.Binary.Build == Dev:
		fmt.Fprintf(stdout, "the latest release is %s; this is a development build (%s). Install a release: %s\n", m.Version, cur, InstallCommand)
	case Newer(m.Version, cur):
		fmt.Fprintf(stdout, "coderefinery %s is available (you have %s). Run: %s\n", m.Version, cur, howToUpdate(opts.Binary.Build))
	default:
		fmt.Fprintf(stdout, "coderefinery %s is up to date\n", cur)
	}
	return nil
}

// Update replaces the running binary with the latest release when it is newer.
func Update(ctx context.Context, stdout io.Writer, opts Options) error {
	switch opts.Binary.Build {
	case GoInstall:
		return fmt.Errorf("this coderefinery (%s) was built by go install; update it with: go install %s@latest", opts.Binary.Version, GoInstallPath)
	case Dev:
		return fmt.Errorf("this is a development build (%s); rebuild it from your checkout, or install a release: %s", opts.Binary.Version, InstallCommand)
	}
	exe, err := resolveExe(opts.Exe)
	if err != nil {
		return err
	}
	m, err := FetchManifest(ctx, opts.Client, opts.BaseURL)
	if err != nil {
		return err
	}
	if !Newer(m.Version, opts.Binary.Version) {
		fmt.Fprintf(stdout, "coderefinery %s is up to date\n", opts.Binary.Version)
		return nil
	}
	bin, err := download(ctx, opts, m)
	if err != nil {
		return err
	}
	if err := replaceExecutable(exe, bin, m.Version); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "updated coderefinery %s -> %s (%s)\n", opts.Binary.Version, m.Version, exe)
	return nil
}

func resolveExe(exe string) (string, error) {
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return "", fmt.Errorf("cannot find the running executable: %w", err)
		}
	}
	// Replace the file a symlink points to, not the link.
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("cannot find the running executable: %w", err)
	}
	return real, nil
}

// FetchManifest downloads and checks <base>/latest.json.
func FetchManifest(ctx context.Context, client *http.Client, base string) (Manifest, error) {
	data, err := get(ctx, client, base+"/"+ManifestName, 1<<20)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s/%s is not a release manifest: %w", base, ManifestName, err)
	}
	if !ValidVersion(m.Version) {
		return Manifest{}, fmt.Errorf("%s/%s has no valid version (%q)", base, ManifestName, m.Version)
	}
	return m, nil
}

// download fetches this platform's archive of release m, checks it against
// the signed SHA256SUMS and returns the coderefinery binary inside it.
func download(ctx context.Context, opts Options, m Manifest) ([]byte, error) {
	asset, ok := m.Assets[opts.Platform]
	if !ok {
		return nil, fmt.Errorf("release %s has no build for %s", m.Version, opts.Platform)
	}
	dir := opts.BaseURL + "/" + m.Version + "/"
	sums, err := get(ctx, opts.Client, dir+SumsName, 1<<20)
	if err != nil {
		return nil, err
	}
	sig, err := get(ctx, opts.Client, dir+SigName, 1<<10)
	if err != nil {
		return nil, err
	}
	if err := VerifySums(opts.PublicKey, sums, sig); err != nil {
		return nil, fmt.Errorf("release %s: %w; not updated", m.Version, err)
	}
	table, err := ParseSums(sums)
	if err != nil {
		return nil, err
	}
	want, ok := table[asset.File]
	if !ok {
		return nil, fmt.Errorf("release %s: %s is not in %s; not updated", m.Version, asset.File, SumsName)
	}
	archive, err := get(ctx, opts.Client, dir+asset.File, 512<<20)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return nil, fmt.Errorf("checksum mismatch for %s (%s: %s, file: %s); not updated", asset.File, SumsName, want, got)
	}
	return extractBinary(archive, asset.File)
}

func get(ctx context.Context, client *http.Client, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, fmt.Errorf("download failed: %s: %w", url, err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("download failed: %s is larger than %d bytes", url, max)
	}
	return data, nil
}

// extractBinary returns the regular file "coderefinery" from a .tar.gz.
func extractBinary(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("cannot unpack %s: %w", name, err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s has no coderefinery binary", name)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot unpack %s: %w", name, err)
		}
		if h.Typeflag == tar.TypeReg && strings.TrimPrefix(h.Name, "./") == "coderefinery" {
			return io.ReadAll(tr)
		}
	}
}

// replaceExecutable writes bin next to exe, checks that it runs and reports
// version, then renames it over exe: the swap is atomic, and a failure at any
// step leaves exe untouched and no temp file behind.
func replaceExecutable(exe string, bin []byte, version string) error {
	mode := fs.FileMode(0o755)
	if st, err := os.Stat(exe); err == nil {
		mode = st.Mode().Perm() | 0o111
	}
	tmp, err := writeTemp(filepath.Dir(exe), bin, mode)
	if err != nil {
		return err
	}
	out, err := exec.Command(tmp, "--version").Output()
	if want := "coderefinery " + version + "\n"; err != nil || string(out) != want {
		os.Remove(tmp) // the temp file only, never a directory
		return fmt.Errorf("the downloaded binary does not run here (--version: %q, %v); not updated", strings.TrimSpace(string(out)), err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cannot replace %s: %w", exe, err)
	}
	return nil
}

// writeTemp writes bin to a new hidden file in dir with mode and returns its
// path. A directory the user cannot write gets a message naming the fix.
func writeTemp(dir string, bin []byte, mode fs.FileMode) (string, error) {
	f, err := os.CreateTemp(dir, ".coderefinery-update-*")
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) && (errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EROFS)) {
			return "", fmt.Errorf("cannot write to %s (%v). Run the update as a user who can write there, or reinstall into a directory you own: %s", dir, pe.Err, InstallCommand)
		}
		return "", fmt.Errorf("cannot write the new binary in %s: %w", dir, err)
	}
	_, err = f.Write(bin)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(f.Name(), mode)
	}
	if err != nil {
		os.Remove(f.Name()) // the temp file only
		return "", fmt.Errorf("cannot write the new binary in %s: %w", dir, err)
	}
	return f.Name(), nil
}
