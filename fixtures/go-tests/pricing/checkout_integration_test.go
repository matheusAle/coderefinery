package pricing

import "testing"

func TestCheckoutGoldMember(t *testing.T) {
	if got := Checkout([]float64{100, 100}, 100, 0); got != 160 {
		t.Fatalf("got %v", got)
	}
}
