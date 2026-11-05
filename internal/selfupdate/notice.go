package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// NoUpdateCheckEnv turns the passive notice off when set to anything.
	NoUpdateCheckEnv = "CODEREFINERY_NO_UPDATE_CHECK"
	// CheckCommand is the hidden argument of the detached process that
	// fetches the manifest for the notice (main runs RunCheck for it).
	CheckCommand = "__update-check"

	checkInterval = 24 * time.Hour
	checkTimeout  = 3 * time.Second
	cacheFile     = "update-check.json"
)

// NoticeConfig is what StartNotice needs to know about this run.
type NoticeConfig struct {
	Binary      Binary
	Command     string // the first argument: "crap", "update", ...
	Getenv      func(string) string
	StderrIsTTY bool
	Now         func() time.Time // nil: time.Now
	Spawn       func() error     // nil: start `<this executable> __update-check`, detached
}

// Notice is the passive "a newer version is available" line. It is decided
// from the cache when the command starts, so printing it never waits.
type Notice struct {
	cur    Binary
	latest string
}

type checkCache struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest,omitempty"`
}

// StartNotice reads the update-check cache and, when the last check is 24
// hours old or more, starts a detached process that fetches the manifest and
// refreshes the cache for the next run; this one never waits for it. It
// returns nil when the notice is off: stderr is not a terminal, CI or
// CODEREFINERY_NO_UPDATE_CHECK is set, the binary is a development build, the
// command is update itself, or there is no cache directory. The cache is
// ${XDG_CACHE_HOME:-$HOME/.cache}/coderefinery/update-check.json.
func StartNotice(cfg NoticeConfig) *Notice {
	if !cfg.StderrIsTTY || cfg.Getenv("CI") != "" || cfg.Getenv(NoUpdateCheckEnv) != "" ||
		cfg.Binary.Build == Dev || cfg.Command == "update" {
		return nil
	}
	path := cachePath(cfg.Getenv)
	if path == "" {
		return nil
	}
	now := time.Now
	if cfg.Now != nil {
		now = cfg.Now
	}
	cached, _ := readCache(path)
	if age := now().Sub(cached.Checked); age < 0 || age >= checkInterval {
		// Claim the slot first, so commands run in the next seconds do not
		// start checks of their own: at most one fetch per 24 hours.
		writeCache(path, checkCache{Checked: now(), Latest: cached.Latest})
		spawn := cfg.Spawn
		if spawn == nil {
			spawn = spawnCheck
		}
		_ = spawn() // best effort: no check today if it cannot start
	}
	return &Notice{cur: cfg.Binary, latest: cached.Latest}
}

// Finish prints the notice on w when the cache knows a newer release.
func (n *Notice) Finish(w io.Writer) {
	if n == nil || !Newer(n.latest, n.cur.Version) {
		return
	}
	fmt.Fprintf(w, "coderefinery %s is available (you have %s). Run: %s (silence: %s=1)\n",
		n.latest, n.cur.Version, howToUpdate(n.cur.Build), NoUpdateCheckEnv)
}

// spawnCheck starts `<this executable> __update-check` in its own process
// group with no stdio, and does not wait for it.
func spawnCheck() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, CheckCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// RunCheck is the detached check: fetch the manifest (3 s timeout) and record
// the latest version in the cache. A failed fetch keeps the version already
// known. It prints nothing.
func RunCheck(getenv func(string) string, client *http.Client, now time.Time) {
	path := cachePath(getenv)
	if path == "" {
		return
	}
	if client == nil {
		client = &http.Client{Timeout: checkTimeout}
	}
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cached, _ := readCache(path)
	next := checkCache{Checked: now, Latest: cached.Latest}
	if m, err := FetchManifest(ctx, client, BaseURL(getenv)); err == nil {
		next.Latest = m.Version
	}
	writeCache(path, next)
}

func cachePath(getenv func(string) string) string {
	dir := getenv("XDG_CACHE_HOME")
	if dir == "" {
		home := getenv("HOME")
		if home == "" {
			return ""
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "coderefinery", cacheFile)
}

func readCache(path string) (checkCache, error) {
	var c checkCache
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &c)
	}
	return c, err
}

// writeCache replaces the cache file atomically; errors are ignored (the
// notice is best effort).
func writeCache(path string, c checkCache) {
	data, _ := json.Marshal(c) // a time and a string: cannot fail
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".update-check-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name()) // the temp file only
	}
}
