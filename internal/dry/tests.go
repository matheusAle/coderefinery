package dry

import "github.com/matheusAle/coderefinery/internal/discover"

// dry --tests (docs/design-tests.md §7): the selection walk keeps test
// directories and test files. Pairing, threshold, report and dry.edn are
// unchanged; test spans just appear as path:start-end.

func hasLanguage(path string) bool { return LanguageOf(path) != "" }

func iterWithTests(roots []string) []string { return discover.IterFilesWith(roots, hasLanguage, true) }
