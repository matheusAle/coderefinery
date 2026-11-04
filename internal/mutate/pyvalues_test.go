package mutate

import "testing"

// truthy and pyInt port Python's bool(v) and int(v or 0) over read EDN
// values; the expectations are CPython's.

func TestTruthyIsPythonBool(t *testing.T) {
	cases := []struct {
		v    any
		want bool
	}{
		{nil, false}, {false, false}, {true, true},
		{int64(0), false}, {int64(-2), true}, {0, false}, {3, true},
		{0.0, false}, {0.5, true}, {"", false}, {"x", true},
		{[]any{}, false}, {[]any{nil}, true}, {ednSet{}, false}, {ednSet{1}, true},
		{&ordMap{}, false}, {&ordMap{pairs: []pair{{kw("a"), 1}}}, true},
		{kw("k"), true}, // keywords, like any other object, are true
	}
	for _, c := range cases {
		if got := truthy(c.v); got != c.want {
			t.Errorf("truthy(%#v) = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestPyIntIsPythonIntOfValueOrZero(t *testing.T) {
	ok := []struct {
		v    any
		want int64
	}{
		{nil, 0}, {false, 0}, {true, 1}, {7, 7}, {int64(-4), -4}, {3.9, 3}, {-3.9, -3},
		{"", 0}, {" 1_000\n", 1000}, {"-12", -12}, {[]any{}, 0}, {&ordMap{}, 0},
	}
	for _, c := range ok {
		if got, err := pyInt(c.v); got != c.want || err != nil {
			t.Errorf("pyInt(%#v) = %d, %v; want %d", c.v, got, err, c.want)
		}
	}
	bad := []struct {
		v   any
		err string
	}{
		{"1.5", "invalid literal for int() with base 10: '1.5'"},
		{"it's", `invalid literal for int() with base 10: "it's"`},
		{[]any{1}, "int() argument must be a string, a bytes-like object or a real number"},
		{&ordMap{pairs: []pair{{kw("a"), 1}}}, "int() argument must be a string, a bytes-like object or a real number"},
		{kw("line"), "int() argument must be a string, a bytes-like object or a real number"},
	}
	for _, c := range bad {
		if got, err := pyInt(c.v); got != 0 || err == nil || err.Error() != c.err {
			t.Errorf("pyInt(%#v) = %d, %v; want error %q", c.v, got, err, c.err)
		}
	}
}
