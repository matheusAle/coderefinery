package pyjson

import "testing"

// Backslash-u escapes are spelled with bs ("\\") so no tool in the chain can
// turn them into literal characters.
const bs = "\\"

func TestDumpsMatchesMutatorProbe(t *testing.T) {
	// spec-mutator §2.4 [probe]
	got := Dumps([]any{"src/caf/\xc3\xa9<&.py", "caf.\xc3\xa9<&", "defn/g", 23, 24, ">", ">="}, ",", ":")
	want := `["src/caf/` + bs + `u00e9<&.py","caf.` + bs + `u00e9<&","defn/g",23,24,">",">="]`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestQuoteEscapes(t *testing.T) {
	cases := map[string]string{
		"a\"b\\c":          `"a\"b\\c"`,
		"\n\t\x01":         `"\n\t` + bs + `u0001"`,
		"\x7f":             `"` + bs + `u007f"`,
		"\xf0\x9f\x98\x80": `"` + bs + `ud83d` + bs + `ude00"`,
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}
