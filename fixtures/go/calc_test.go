package ubfix

import "testing"

func TestMean(t *testing.T) {
	got, err := Mean([]int{2, 4, -1})
	if err != nil || got != 3 {
		t.Fatalf("Mean = %v, %v", got, err)
	}
	if _, err := Mean(nil); err != ErrEmpty {
		t.Fatalf("want ErrEmpty, got %v", err)
	}
}

func TestGrade(t *testing.T) {
	if Grade(95) != "A" || Grade(80) != "B" {
		t.Fatal("grade")
	}
}

func TestSumPositive(t *testing.T) {
	total, seen := SumPositive([]int{1, 2, -3})
	if total != 3 || seen != 2 {
		t.Fatalf("got %d %d", total, seen)
	}
}
