package report

import "testing"

// No boundary case: Grade's >= -> > mutant survives.
func TestGrade(t *testing.T) {
	if Grade(9, 10, 80) != "pass" || Grade(1, 10, 80) != "fail" {
		t.Fatal("grade")
	}
}

func TestMissing(t *testing.T) {
	if Missing(3, 10) != 7 {
		t.Fatal("missing")
	}
}
