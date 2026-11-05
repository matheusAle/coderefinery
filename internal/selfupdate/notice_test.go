package selfupdate

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var noticeNow = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

const wantNotice = "coderefinery v0.2.0 is available (you have v0.1.0). Run: coderefinery update (silence: CODEREFINERY_NO_UPDATE_CHECK=1)\n"

// noticeCase is an interactive release run with its own cache directory.
type noticeCase struct {
	env    map[string]string
	cfg    NoticeConfig
	spawns int
}

func newNoticeCase(t *testing.T) *noticeCase {
	c := &noticeCase{env: map[string]string{"XDG_CACHE_HOME": t.TempDir()}}
	c.cfg = NoticeConfig{
		Binary:      Binary{Version: "v0.1.0", Build: Release},
		Command:     "crap",
		Getenv:      func(k string) string { return c.env[k] },
		StderrIsTTY: true,
		Now:         func() time.Time { return noticeNow },
		Spawn:       func() error { c.spawns++; return nil },
	}
	return c
}

func (c *noticeCase) cachePath() string {
	return filepath.Join(c.env["XDG_CACHE_HOME"], "coderefinery", cacheFile)
}

// start runs StartNotice then Finish, as main does; it returns what was printed.
func (c *noticeCase) start() string {
	var buf bytes.Buffer
	StartNotice(c.cfg).Finish(&buf)
	return buf.String()
}

func TestNoticeIsOffOutsideAnInteractiveReleaseRun(t *testing.T) {
	cases := map[string]func(c *noticeCase){
		"stderr is not a terminal": func(c *noticeCase) { c.cfg.StderrIsTTY = false },
		"CI is set":                func(c *noticeCase) { c.env["CI"] = "true" },
		"opted out":                func(c *noticeCase) { c.env[NoUpdateCheckEnv] = "1" },
		"development build":        func(c *noticeCase) { c.cfg.Binary.Build = Dev },
		"the update command":       func(c *noticeCase) { c.cfg.Command = "update" },
		"no cache directory":       func(c *noticeCase) { delete(c.env, "XDG_CACHE_HOME") },
	}
	for name, tweak := range cases {
		c := newNoticeCase(t)
		path := c.cachePath()
		writeCache(path, checkCache{Checked: noticeNow.Add(-48 * time.Hour), Latest: "v0.2.0"})
		tweak(c)
		if StartNotice(c.cfg) != nil {
			t.Errorf("%s: notice started", name)
		}
		if got := c.start(); got != "" || c.spawns != 0 {
			t.Errorf("%s: printed %q, spawned %d checks", name, got, c.spawns)
		}
		if cached, err := readCache(path); err != nil || !cached.Checked.Equal(noticeNow.Add(-48*time.Hour)) {
			t.Errorf("%s: cache touched: %+v, %v", name, cached, err)
		}
	}
}

func TestNoticeChecksAtMostOncePer24HoursAndPrintsFromTheCache(t *testing.T) {
	c := newNoticeCase(t)
	// No cache yet: a check starts, nothing is known, nothing printed.
	if got := c.start(); got != "" || c.spawns != 1 {
		t.Fatalf("first run: printed %q, %d checks", got, c.spawns)
	}
	cached, err := readCache(c.cachePath())
	if err != nil || !cached.Checked.Equal(noticeNow) || cached.Latest != "" {
		t.Errorf("claim = %+v, %v", cached, err)
	}
	// The detached check found v0.2.0; the next runs print it without a new check.
	writeCache(c.cachePath(), checkCache{Checked: noticeNow, Latest: "v0.2.0"})
	for _, later := range []time.Duration{time.Minute, 23 * time.Hour} {
		c.cfg.Now = func() time.Time { return noticeNow.Add(later) }
		if got := c.start(); got != wantNotice || c.spawns != 1 {
			t.Errorf("+%v: printed %q, %d checks", later, got, c.spawns)
		}
	}
	// A day later: one more check, and the known version is still printed.
	c.cfg.Now = func() time.Time { return noticeNow.Add(25 * time.Hour) }
	if got := c.start(); got != wantNotice || c.spawns != 2 {
		t.Errorf("+25h: printed %q, %d checks", got, c.spawns)
	}
	if got := c.start(); c.spawns != 2 {
		t.Errorf("right after: printed %q, %d checks", got, c.spawns)
	}
}

func TestNoticeCheckThatCannotStartIsHarmless(t *testing.T) {
	c := newNoticeCase(t)
	c.cfg.Spawn = func() error { return errors.New("no fork today") }
	if got := c.start(); got != "" {
		t.Errorf("printed %q", got)
	}
}

func TestNoticeIsSilentWhenUpToDateAndForGoInstallSaysGoInstall(t *testing.T) {
	c := newNoticeCase(t)
	writeCache(c.cachePath(), checkCache{Checked: noticeNow, Latest: "v0.1.0"})
	if got := c.start(); got != "" {
		t.Errorf("up to date printed %q", got)
	}
	writeCache(c.cachePath(), checkCache{Checked: noticeNow, Latest: "v0.2.0"})
	c.cfg.Binary.Build = GoInstall
	want := "coderefinery v0.2.0 is available (you have v0.1.0). Run: go install " + GoInstallPath + "@latest (silence: CODEREFINERY_NO_UPDATE_CHECK=1)\n"
	if got := c.start(); got != want {
		t.Errorf("go install build printed %q, want %q", got, want)
	}
}

func TestRunCheckRecordsTheLatestVersion(t *testing.T) {
	rel := newFakeRelease(t, "v0.2.0", script("v0.2.0"))
	c := newNoticeCase(t)
	c.env[BaseURLEnv] = rel.serve(t)
	RunCheck(c.cfg.Getenv, nil, noticeNow)
	cached, err := readCache(c.cachePath())
	if err != nil || !cached.Checked.Equal(noticeNow) || cached.Latest != "v0.2.0" {
		t.Errorf("cache = %+v, %v", cached, err)
	}
	if got := rel.requested(); strings.Join(got, " ") != "latest.json" {
		t.Errorf("requested %v", got)
	}
	// Smoke: the next interactive run prints the notice.
	if got := c.start(); got != wantNotice {
		t.Errorf("next run printed %q", got)
	}
	if names, _ := os.ReadDir(filepath.Dir(c.cachePath())); len(names) != 1 {
		t.Errorf("cache dir holds %d files, want 1 (no temp files)", len(names))
	}
}

func TestRunCheckKeepsTheKnownVersionWhenTheFetchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	c := newNoticeCase(t)
	c.env[BaseURLEnv] = srv.URL
	writeCache(c.cachePath(), checkCache{Checked: noticeNow.Add(-48 * time.Hour), Latest: "v0.2.0"})
	RunCheck(c.cfg.Getenv, nil, noticeNow)
	cached, err := readCache(c.cachePath())
	if err != nil || !cached.Checked.Equal(noticeNow) || cached.Latest != "v0.2.0" {
		t.Errorf("cache = %+v, %v", cached, err)
	}
}

func TestNoticeCacheFallsBackToHomeDotCache(t *testing.T) {
	env := map[string]string{"HOME": "/home/u"}
	if got := cachePath(func(k string) string { return env[k] }); got != "/home/u/.cache/coderefinery/update-check.json" {
		t.Errorf("got %q", got)
	}
	RunCheck(func(string) string { return "" }, nil, noticeNow) // no cache dir: nothing to do, no panic
}
