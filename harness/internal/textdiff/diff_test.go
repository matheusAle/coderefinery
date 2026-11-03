package textdiff

import (
	"strings"
	"testing"
)

func TestUnifiedEqualIsEmpty(t *testing.T) {
	if d := Unified("a", "b", "x\ny\n", "x\ny\n", 2, 0); d != "" {
		t.Fatalf("diff = %q", d)
	}
}

func TestUnifiedShowsChangedLineWithContext(t *testing.T) {
	want := "1\n2\n3\n4\n5\n6\n7\n"
	got := "1\n2\n3\nFOUR\n5\n6\n7\n"
	d := Unified("golden", "ours", want, got, 1, 0)
	expected := "--- golden\n+++ ours\n@@ -3,3 +3,3 @@\n 3\n-4\n+FOUR\n 5\n"
	if d != expected {
		t.Fatalf("diff =\n%s\nwant\n%s", d, expected)
	}
}

func TestUnifiedTruncates(t *testing.T) {
	d := Unified("a", "b", strings.Repeat("x\n", 50), strings.Repeat("y\n", 50), 0, 10)
	if !strings.Contains(d, "more diff lines") || strings.Count(d, "\n") != 11 {
		t.Fatalf("diff =\n%s", d)
	}
}

func TestUnifiedMissingTrailingNewline(t *testing.T) {
	d := Unified("a", "b", "x\n", "x", 0, 0)
	if !strings.Contains(d, "-x") || !strings.Contains(d, "+x  \\ (no newline at end)") {
		t.Fatalf("diff =\n%s", d)
	}
}
