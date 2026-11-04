package pricing

import "testing"

func TestTierRate(t *testing.T) {
	for _, c := range []struct {
		name   string
		points int
		rate   float64
	}{{"gold", 100, 0.8}, {"silver", 50, 0.9}} {
		t.Run(c.name, func(t *testing.T) {
			if got := TierRate(c.points); got != c.rate {
				t.Fatalf("TierRate(%d) = %v, want %v", c.points, got, c.rate)
			}
		})
	}
}

func TestPriceOfNothingIsZero(t *testing.T) {
	if got := Price(0, 1.0); got != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestPriceAppliesTheRate(t *testing.T) {
	if got := Price(200, 0.8); got != 160 {
		t.Fatalf("got %v", got)
	}
	if got := Price(0.5, 1.0); got != 0.5 {
		t.Fatalf("got %v", got)
	}
}

func TestCouponSubtracts(t *testing.T) {
	if got := ApplyCoupon(10, 3); got != 7 {
		t.Fatalf("got %v", got)
	}
	if got := ApplyCoupon(10, 9.5); got != 0.5 {
		t.Fatalf("got %v", got)
	}
}

func TestCouponNeverGoesNegative(t *testing.T) {
	if got := ApplyCoupon(3, 5); got != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestPriceReturnsANumber(t *testing.T) {
	_ = Price(10, 1.0)
}
