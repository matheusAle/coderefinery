package main

import "testing"

func TestTimingSortsVitestFileLinesWithinTheirBlock(t *testing.T) {
	n := Normaliser{Timing: true}
	a := " RUN  v3.2.7\n\n ✓ tests/routes.test.ts (4 tests) 3ms\n ✓ tests/inventory.test.ts (5 tests) 4ms\n\n Test Files  2 passed (2)\n"
	b := " RUN  v3.2.7\n\n ✓ tests/inventory.test.ts (5 tests) 9ms\n ✓ tests/routes.test.ts (4 tests) 1ms\n\n Test Files  2 passed (2)\n"
	want := " RUN  v3.2.7\n\n ✓ tests/inventory.test.ts (5 tests) <DUR>\n ✓ tests/routes.test.ts (4 tests) <DUR>\n\n Test Files  2 passed (2)\n"
	if got := n.Text(a, ""); got != want {
		t.Fatalf("completion order a:\ngot  %q\nwant %q", got, want)
	}
	if got := n.Text(b, ""); got != want {
		t.Fatalf("completion order b:\ngot  %q\nwant %q", got, want)
	}
}

func TestVitestSortLeavesOtherLinesInPlace(t *testing.T) {
	in := "z line\n ✓ b.test.ts (1 test)\n ✓ a.test.ts (1 test)\na line"
	want := "z line\n ✓ a.test.ts (1 test)\n ✓ b.test.ts (1 test)\na line"
	if got := sortVitestFileLines(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
