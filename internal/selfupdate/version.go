package selfupdate

import (
	"strconv"
	"strings"
)

// Build says how the running binary was made, which decides how it updates.
type Build int

const (
	// Release: a `make dist` build, version stamped by -ldflags. Updates itself.
	Release Build = iota
	// GoInstall: `go install …@vX.Y.Z`; the module version comes from the
	// build info. Updated by go install, not by replacing the file.
	GoInstall
	// Dev: built from a checkout ("(devel)"). Never updated.
	Dev
)

// Binary is the running coderefinery: its version and how it was built.
type Binary struct {
	Version string
	Build   Build
}

// semver is vMAJOR.MINOR.PATCH[-PRERELEASE]; build metadata is ignored.
type semver struct {
	nums [3]int
	pre  string
}

func parseVersion(v string) (semver, bool) {
	s, ok := strings.CutPrefix(v, "v")
	if !ok {
		return semver{}, false
	}
	s, _, _ = strings.Cut(s, "+")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	var out semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" {
			return semver{}, false
		}
		out.nums[i] = n
	}
	out.pre = pre
	return out, true
}

// ValidVersion reports whether v is vMAJOR.MINOR.PATCH[-pre][+build].
func ValidVersion(v string) bool {
	_, ok := parseVersion(v)
	return ok
}

// Newer reports whether version latest is newer than current. Versions that
// are not vX.Y.Z[-pre] are never newer, and nothing is newer than them.
func Newer(latest, current string) bool {
	l, ok1 := parseVersion(latest)
	c, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l.nums {
		if l.nums[i] != c.nums[i] {
			return l.nums[i] > c.nums[i]
		}
	}
	// Same core: a release is newer than its pre-releases; pre-releases
	// compare as strings (enough for -rc.1 < -rc.2; releases are vX.Y.Z).
	if l.pre == "" || c.pre == "" {
		return l.pre == "" && c.pre != ""
	}
	return l.pre > c.pre
}
