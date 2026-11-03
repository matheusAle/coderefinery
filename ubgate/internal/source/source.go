// Package source reads files the way every upstream tool does:
// Path.read_text(encoding="utf-8", errors="replace"), i.e. CPython's UTF-8
// decoder with one U+FFFD per maximal invalid subpart, then universal
// newlines (\r\n and lone \r become \n). A BOM is kept as U+FEFF. Parsers get
// the UTF-8 re-encoding of that text and byte offsets refer to it.
package source

import (
	"os"
	"strings"
)

// Read returns the decoded, newline-normalized text of path.
func Read(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return Text(raw), nil
}

// Text decodes raw bytes with errors="replace" and applies universal newlines.
func Text(raw []byte) string {
	return UniversalNewlines(DecodeReplace(raw))
}

// UniversalNewlines turns \r\n and lone \r into \n.
func UniversalNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// DecodeReplace decodes UTF-8 like CPython's bytes.decode("utf-8", "replace"):
// each maximal subpart of an ill-formed sequence (Unicode 3.9, table 3-7)
// becomes one U+FFFD.
func DecodeReplace(raw []byte) string {
	var b strings.Builder
	b.Grow(len(raw))
	i := 0
	for i < len(raw) {
		c := raw[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		size, lo, hi := 0, byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			size = 2
		case c == 0xE0:
			size, lo = 3, 0xA0
		case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
			size = 3
		case c == 0xED:
			size, hi = 3, 0x9F
		case c == 0xF0:
			size, lo = 4, 0x90
		case c >= 0xF1 && c <= 0xF3:
			size = 4
		case c == 0xF4:
			size, hi = 4, 0x8F
		default:
			b.WriteString("�")
			i++
			continue
		}
		j := i + 1
		ok := true
		for k := 1; k < size; k++ {
			if j >= len(raw) {
				ok = false
				break
			}
			d := raw[j]
			l, h := byte(0x80), byte(0xBF)
			if k == 1 {
				l, h = lo, hi
			}
			if d < l || d > h {
				ok = false
				break
			}
			j++
		}
		if ok {
			b.Write(raw[i:j])
		} else {
			b.WriteString("�")
		}
		i = j
	}
	return b.String()
}
