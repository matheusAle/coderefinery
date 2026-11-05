package selfupdate

import "testing"

func TestNewerComparesSemverNumericallyAndRanksReleasesAbovePreReleases(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.2.0", "v0.1.0", true},
		{"v0.10.0", "v0.9.9", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.1.1", "v0.1.0", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"v0.2.0", "v0.2.0-rc.1", true},
		{"v0.2.0-rc.1", "v0.2.0", false},
		{"v0.2.0-rc.2", "v0.2.0-rc.1", true},
		{"v0.2.0-rc.1", "v0.1.0", true},
		{"v0.2.0+build.5", "v0.2.0", false},
		{"v0.2.0", "(devel)", false},
		{"latest", "v0.1.0", false},
		{"v1.2", "v0.1.0", false},
		{"v1.x.0", "v0.1.0", false},
		{"0.2.0", "v0.1.0", false},
		{"", "v0.1.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestValidVersion(t *testing.T) {
	for v, want := range map[string]bool{"v1.2.3": true, "v0.0.0-rc.1": true, "v1.2.3+meta": true, "1.2.3": false, "v1.2": false, "v1.2.-3": false, "": false} {
		if got := ValidVersion(v); got != want {
			t.Errorf("ValidVersion(%q) = %v, want %v", v, got, want)
		}
	}
}
