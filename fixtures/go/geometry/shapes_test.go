package geometry

import "testing"

func TestArea(t *testing.T) {
	if got := (Rect{W: 2, H: 3}).Area(); got != 6 {
		t.Fatalf("Area = %v", got)
	}
	if got := (Rect{W: -1, H: 3}).Area(); got != 0 {
		t.Fatalf("Area = %v", got)
	}
}

func TestCollect(t *testing.T) {
	values := make(chan int, 3)
	values <- 1
	values <- 2
	close(values)
	got := Collect(values, make(chan struct{}), 5)
	if len(got) != 2 {
		t.Fatalf("Collect = %v", got)
	}
}
