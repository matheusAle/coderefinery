package source

import "testing"

func TestDecodeReplaceMatchesCPython(t *testing.T) {
	// Expected values come from CPython 3.12 bytes.decode("utf-8", "replace").
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte("plain"), "plain"},
		{[]byte("caf\xc3\xa9"), "café"},
		{[]byte("a\xffb"), "a�b"},
		{[]byte("\xe2\x82x"), "�x"},                // truncated 3-byte: one U+FFFD
		{[]byte("\xf0\x9f\x98"), "�"},              // truncated 4-byte at EOF
		{[]byte("\xed\xa0\x80"), "���"},            // surrogate: 3 replacements
		{[]byte("\xc0\xaf"), "��"},                 // overlong lead
		{[]byte("\xef\xbb\xbfx"), "\xef\xbb\xbfx"}, // BOM kept
	}
	for _, c := range cases {
		if got := DecodeReplace(c.in); got != c.want {
			t.Errorf("DecodeReplace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTextAppliesUniversalNewlines(t *testing.T) {
	got := Text([]byte("a\r\nb\rc\n"))
	if got != "a\nb\nc\n" {
		t.Fatalf("Text = %q", got)
	}
}
