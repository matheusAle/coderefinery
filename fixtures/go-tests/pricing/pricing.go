// Package pricing prices orders with loyalty tiers and coupons.
package pricing

// TierRate is the price factor for a loyalty balance.
func TierRate(points int) float64 {
	if points >= 100 {
		return 0.8
	}
	if points >= 50 {
		return 0.9
	}
	return 1.0
}

// Price applies rate to total; nothing costs nothing.
func Price(total, rate float64) float64 {
	if total <= 0 {
		return 0
	}
	return total * rate
}

// ApplyCoupon takes coupon off total, never below zero.
func ApplyCoupon(total, coupon float64) float64 {
	remaining := total - coupon
	if remaining < 0 {
		return 0
	}
	return remaining
}
