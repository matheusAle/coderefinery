package discover

// IterFilesWith is IterFiles; with includeTests the walk also descends into
// TestDirs (dry --tests, docs/design-tests.md §7). keep still decides which
// files count, so the caller drops its test-file rule as well.
func IterFilesWith(roots []string, keep func(path string) bool, includeTests bool) []string {
	return iterFiles(roots, keep, !includeTests)
}
