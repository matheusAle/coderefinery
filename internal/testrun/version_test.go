package testrun

import "testing"

func TestAtLeastComparesMajorThenMinor(t *testing.T) {
	cases := map[string]bool{
		"3.2.0": true, "3.2": true, "3.10.1": true, "4.0.0": true, // at or above 3.2
		"3.1.9": false, "2.99.0": false, // below
		"3": false, "": false, "^3.2.0": false, "3.x": false, // not a plain version
	}
	for v, want := range cases {
		if got := atLeast(v, 3, 2); got != want {
			t.Errorf("atLeast(%q, 3, 2) = %v, want %v", v, got, want)
		}
	}
}

func TestMatchesAnyIsASubstringTest(t *testing.T) {
	if !matchesAny("/r/src/cart.py", []string{"nope", "src/c"}) || matchesAny("/r/src/cart.py", []string{"tests"}) || matchesAny("/r/a.py", nil) {
		t.Fatal("matchesAny")
	}
}
