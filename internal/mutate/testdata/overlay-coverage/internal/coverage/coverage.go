// Package coverage is a package, not a report directory, although upstream
// mutator's overlay treats every directory named coverage as one.
package coverage

// Percent is hit out of total in whole percent, 0 when total is 0.
func Percent(hit, total int) int {
	if total == 0 {
		return 0
	}
	return hit * 100 / total
}
