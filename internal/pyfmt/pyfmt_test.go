package pyfmt

import (
	"math"
	"testing"
)

func TestRoundMatchesPythonTiesToEvenOnBinaryValue(t *testing.T) {
	cases := map[float64]float64{
		66.66666666666667: 66.6667,
		4.592592592592593: 4.5926,
		2.675:             2.675,  // round(2.675, 4)
		0.00005:           0.0001, // 5.000000000000000409e-05 rounds up
		1.00005:           1.0001, // CPython: round(1.00005, 4) == 1.0001
	}
	for in, want := range cases {
		if got := Round(in, 4); got != want {
			t.Errorf("Round(%v, 4) = %v, want %v", in, got, want)
		}
	}
}

func TestFixedHandlesSpecials(t *testing.T) {
	if got := Fixed(math.Inf(1), 1); got != "inf" {
		t.Errorf("inf -> %q", got)
	}
	if got := Fixed(32.244, 1); got != "32.2" {
		t.Errorf("32.244 -> %q", got)
	}
	if got := FixedWidth(5.0488, 8, 1); got != "     5.0" {
		t.Errorf("width -> %q", got)
	}
}

func TestReprMatchesPython(t *testing.T) {
	cases := map[float64]string{
		1.0: "1.0", 0.1: "0.1", 100.0: "100.0", 1e16: "1e+16", 1e-5: "1e-05", 0.0001: "0.0001",
		123456789012345.6: "123456789012345.6", -2.5: "-2.5", 1.5e300: "1.5e+300",
	}
	for in, want := range cases {
		if got := Repr(in); got != want {
			t.Errorf("Repr(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestParseFloatFollowsPythonFloat(t *testing.T) {
	ok := map[string]float64{" 5 ": 5, "1e3": 1000, "+5": 5, "1_0": 10, "-2.5": -2.5}
	for in, want := range ok {
		got, err := ParseFloat(in)
		if err != nil || got != want {
			t.Errorf("ParseFloat(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if v, err := ParseFloat("Infinity"); err != nil || !math.IsInf(v, 1) {
		t.Errorf("Infinity -> %v %v", v, err)
	}
	for _, bad := range []string{"nope", "0x1p3", "", "1__0", "_1", "--1"} {
		if _, err := ParseFloat(bad); err == nil {
			t.Errorf("ParseFloat(%q) accepted", bad)
		}
	}
}
