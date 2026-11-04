// Package calc has arms its tests never take (crap --branches e2e).
package calc

// Clamp limits v to [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Kind names a number.
func Kind(n int) string {
	switch {
	case n < 0:
		return "negative"
	case n == 0:
		return "zero"
	default:
		return "positive"
	}
}

// Sum adds the values.
func Sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

func unused(n int) int {
	if n > 0 {
		return n
	}
	return 0
}
