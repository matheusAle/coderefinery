package pricing

// Checkout runs the whole pricing pipeline.
func Checkout(items []float64, points int, coupon float64) float64 {
	var total float64
	for _, item := range items {
		total += item
	}
	return ApplyCoupon(Price(total, TierRate(points)), coupon)
}
