package edn

import (
	"math"
	"testing"
)

func TestNumber4MatchesCrapperEdnNumber(t *testing.T) {
	cases := map[float64]string{100.0: "100.0", 0.0: "0.0", 66.6667: "66.6667", 4.5926: "4.5926", 3.1: "3.1",
		30.0: "30.0", 32.244: "32.244", math.Inf(1): "inf.0"}
	for in, want := range cases {
		if got := Number4(in); got != want {
			t.Errorf("Number4(%v) = %q, want %q", in, got, want)
		}
	}
	if OptNumber4(nil) != "nil" {
		t.Error("nil should print nil")
	}
}

func TestStringEscapesOnlyBackslashAndQuote(t *testing.T) {
	if got := String("a\\b\"c\nd\té"); got != "\"a\\\\b\\\"c\nd\té\"" {
		t.Errorf("String = %q", got)
	}
}
