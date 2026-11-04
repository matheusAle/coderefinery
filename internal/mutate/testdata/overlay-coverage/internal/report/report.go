// Package report imports internal/coverage: its mutants only build when the
// worker overlay links that directory.
package report

import "example.com/overlay/internal/coverage"

// Grade is "pass" when at least min percent of the lines ran.
func Grade(hit, total, min int) string {
	if coverage.Percent(hit, total) >= min {
		return "pass"
	}
	return "fail"
}

// Missing is the number of lines that never ran.
func Missing(hit, total int) int {
	return total - hit
}
