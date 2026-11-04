// Package pyjson encodes values exactly like Python's json.dumps with the
// default ensure_ascii=True (mutator ids depend on it). Go's encoding/json
// is NOT equivalent: it escapes < > & and does not escape non-ASCII.
package pyjson

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

// Quote is json.dumps(s) for a str.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r >= 0x20 && r <= 0x7e:
				b.WriteRune(r)
			case r > 0xffff:
				r1, r2 := utf16.EncodeRune(r)
				fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Dumps encodes nil, bool, int, int64, float64, string and []any (nested)
// with the given separators, e.g. Dumps(v, ",", ":") for
// separators=(",", ":") or Dumps(v, ", ", ": ") for the defaults.
func Dumps(v any, itemSep, keySep string) string {
	_ = keySep // reserved for maps (no caller needs them yet)
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		switch r := pyfmt.Repr(x); r {
		case "inf":
			return "Infinity"
		case "-inf":
			return "-Infinity"
		case "nan":
			return "NaN"
		default:
			return r
		}
	case string:
		return Quote(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = Dumps(e, itemSep, keySep)
		}
		return "[" + strings.Join(parts, itemSep) + "]"
	}
	panic(fmt.Sprintf("pyjson: unsupported type %T", v))
}
