// Package edn writes the EDN fragments the upstream tools print. Each
// upstream tool has its own tiny writer; the helpers here are named after
// the upstream function they reproduce byte for byte.
package edn

import (
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/pyfmt"
)

// String is crapper metrics._edn_string (also dryer/mutator strings):
// only backslash and double quote are escaped; newlines, tabs and every
// other character pass through raw.
func String(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// Number4 is crapper metrics._edn_number: format(v, ".4f") with trailing
// zeros and dot stripped, then ".0" re-added when no dot is left
// (100.0, 66.6667, 3.1, inf.0).
func Number4(v float64) string {
	t := strings.TrimRight(strings.TrimRight(pyfmt.Fixed(v, 4), "0"), ".")
	if !strings.Contains(t, ".") {
		t += ".0"
	}
	return t
}

// OptNumber4 is metrics._edn_value: nil -> "nil".
func OptNumber4(v *float64) string {
	if v == nil {
		return "nil"
	}
	return Number4(*v)
}
