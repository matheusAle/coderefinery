package calc

import "testing"

func TestClamp(t *testing.T) {
	if Clamp(5, 0, 10) != 5 || Clamp(-1, 0, 10) != 0 {
		t.Fatal("clamp")
	}
}

func TestKind(t *testing.T) {
	if Kind(3) != "positive" || Kind(-2) != "negative" {
		t.Fatal("kind")
	}
}

func TestSum(t *testing.T) {
	if Sum(nil) != 0 {
		t.Fatal("sum")
	}
}
