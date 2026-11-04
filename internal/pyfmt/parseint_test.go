package pyfmt

import "testing"

// Expected values come from CPython 3.12 int(s) and repr(s).

func TestParseIntFollowsPythonInt(t *testing.T) {
	ok := map[string]int{" -1_000 ": -1000, "+7": 7, "0": 0, "\t42\n": 42}
	for in, want := range ok {
		if got, err := ParseInt(in); got != want || err != nil {
			t.Errorf("ParseInt(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	bad := map[string]string{
		"1__0": `'1__0'`, "_1": `'_1'`, "1_": `'1_'`, "+-1": `'+-1'`, "": `''`, "1.5": `'1.5'`,
		"99999999999999999999": `'99999999999999999999'`, // beyond int: Python's text, not Go's
	}
	for in, repr := range bad {
		got, err := ParseInt(in)
		if want := "invalid literal for int() with base 10: " + repr; got != 0 || err == nil || err.Error() != want {
			t.Errorf("ParseInt(%q) = %d, %v; want error %q", in, got, err, want)
		}
	}
}

func TestPyReprPicksQuotesAndEscapesLikePython(t *testing.T) {
	cases := map[string]string{
		"plain":             `'plain'`,
		"it's":              `"it's"`,
		`say "hi"`:          `'say "hi"'`,
		`both ' and "`:      `'both \' and "'`,
		`a\b`:               `'a\\b'`,
		"tab\there\nnl\rcr": `'tab\there\nnl\rcr'`,
		"é":                 `'é'`,
	}
	for in, want := range cases {
		if got := PyRepr(in); got != want {
			t.Errorf("PyRepr(%q) = %s, want %s", in, got, want)
		}
	}
}
