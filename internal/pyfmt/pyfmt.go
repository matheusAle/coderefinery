// Package pyfmt reproduces the Python float formatting and parsing the
// upstream tools rely on: round(x, n), "%.Nf"/format(x, ".Nf"), repr(float)
// and float(str).
package pyfmt

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// Round is Python round(x, digits): correctly rounded, ties-to-even on the
// exact binary value (strconv's 'f' formatting is exact, so the
// format-then-parse round trip matches CPython).
func Round(x float64, digits int) float64 {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return x
	}
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', digits, 64), 64)
	return v
}

// Fixed is Python format(x, ".Nf") / "%.Nf" % x, including inf, -inf, nan.
func Fixed(x float64, digits int) string {
	switch {
	case math.IsNaN(x):
		return "nan"
	case math.IsInf(x, 1):
		return "inf"
	case math.IsInf(x, -1):
		return "-inf"
	}
	return strconv.FormatFloat(x, 'f', digits, 64)
}

// FixedWidth is Python f"{x:W.Nf}": Fixed right-aligned in width W.
func FixedWidth(x float64, width, digits int) string {
	s := Fixed(x, digits)
	if n := width - len(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// Repr is Python repr(float): the shortest round-tripping digits, fixed
// notation when the decimal exponent is in [-4, 16), else scientific with a
// signed two-digit (or longer) exponent.
func Repr(x float64) string {
	switch {
	case math.IsNaN(x):
		return "nan"
	case math.IsInf(x, 1):
		return "inf"
	case math.IsInf(x, -1):
		return "-inf"
	}
	e := strconv.FormatFloat(x, 'e', -1, 64) // e.g. -1.2345e+06
	sign := ""
	if e[0] == '-' {
		sign, e = "-", e[1:]
	}
	mant, expStr, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expStr)
	digits := strings.Replace(mant, ".", "", 1)
	if exp >= -4 && exp < 16 {
		var s string
		if exp >= 0 {
			if len(digits) <= exp+1 {
				s = digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
			} else {
				s = digits[:exp+1] + "." + digits[exp+1:]
			}
		} else {
			s = "0." + strings.Repeat("0", -exp-1) + digits
		}
		return sign + s
	}
	m := digits[:1]
	if len(digits) > 1 {
		m += "." + digits[1:]
	}
	es := "+"
	if exp < 0 {
		es, exp = "-", -exp
	}
	ex := strconv.Itoa(exp)
	if len(ex) < 2 {
		ex = "0" + ex
	}
	return sign + m + "e" + es + ex
}

var errFloat = errors.New("could not convert string to float")

// ParseFloat is Python float(str): surrounding whitespace, inf/nan
// spellings and digit-group underscores accepted; hex rejected.
func ParseFloat(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, errFloat
	}
	body := strings.TrimLeft(s, "+-")
	if len(s)-len(body) > 1 {
		return 0, errFloat
	}
	switch strings.ToLower(body) {
	case "inf", "infinity", "nan":
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, errFloat
		}
		return v, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			if i == 0 || i+1 >= len(s) || !isDigit(s[i-1]) || !isDigit(s[i+1]) {
				return 0, errFloat
			}
			continue
		}
		if !(isDigit(c) || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
			return 0, errFloat
		}
		b.WriteByte(c)
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		var ne *strconv.NumError
		if errors.As(err, &ne) && errors.Is(ne.Err, strconv.ErrRange) {
			return v, nil // Python overflows to inf / underflows to 0 too
		}
		return 0, errFloat
	}
	return v, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// ParseInt is Python int(str) for base 10: surrounding whitespace, one
// sign, digit-group underscores. The error text is Python's ValueError.
func ParseInt(raw string) (int, error) {
	s := strings.TrimSpace(raw)
	body := strings.TrimLeft(s, "+-")
	ok := body != "" && len(s)-len(body) <= 1 && isDigit(body[0]) && isDigit(body[len(body)-1])
	for i := 0; ok && i < len(body); i++ {
		if body[i] == '_' {
			ok = isDigit(body[i-1]) && isDigit(body[i+1])
		} else {
			ok = isDigit(body[i])
		}
	}
	if !ok {
		return 0, errors.New("invalid literal for int() with base 10: " + PyRepr(raw))
	}
	v, err := strconv.Atoi(strings.ReplaceAll(s, "_", ""))
	if err != nil {
		return 0, errors.New("invalid literal for int() with base 10: " + PyRepr(raw))
	}
	return v, nil
}

// FloatError is Python's float() ValueError text for raw.
func FloatError(raw string) error {
	return errors.New("could not convert string to float: " + PyRepr(raw))
}

// PyRepr is repr(str) for the common cases (quote choice and escapes of
// backslash, quotes, \n \r \t).
func PyRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		q = `"`
	}
	var b strings.Builder
	b.WriteString(q)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case string(r) == q:
			b.WriteString(`\` + q)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString(q)
	return b.String()
}
