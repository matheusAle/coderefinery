// Package ubfix holds package-level functions with branches.
package ubfix

import "errors"

// ErrEmpty is returned for empty input.
var ErrEmpty = errors.New("empty")

// Mean returns the mean of the positive values.
func Mean(values []int) (float64, error) {
	if len(values) == 0 {
		return 0, ErrEmpty
	}
	sum, count := 0, 0
	for _, v := range values {
		if v > 0 {
			sum += v
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	return float64(sum) / float64(count), nil
}

// Grade maps a score to a letter.
func Grade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75 && score < 90:
		return "B"
	case score < 0 || score > 100:
		return "?"
	default:
		return "C"
	}
}

func clampIndex(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// SumPositive adds up positive values, a near-duplicate of SumWeights.
func SumPositive(values []int) (int, int) {
	total := 0
	seen := 0
	for _, v := range values {
		if v > 0 {
			total += v
			seen++
		}
	}
	return total, seen
}

// SumWeights adds up positive weights.
func SumWeights(weights []int) (int, int) {
	acc := 0
	n := 0
	for _, w := range weights {
		if w > 0 {
			acc += w
			n++
		}
	}
	return acc, n
}
