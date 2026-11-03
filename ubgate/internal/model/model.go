// Package model holds crapper's data model (upstream crapper/model.py),
// shared by crap, coverage and mutate.
package model

// Function is one scored unit: a function or method.
type Function struct {
	Name        string
	Namespace   string
	Complexity  int
	StartLine   int // 1-based, inclusive
	EndLine     int // 1-based, inclusive
	Path        string
	Language    string // clojure|java|go|typescript|python|rust
	JacocoClass string // "" = None
	// UTF-8 byte span of the function; -1 when the language does not record
	// it (upstream fills it for TypeScript only). Mutator uses it.
	StartByte int
	EndByte   int
}

// Entry is one row of the CRAP report and of .metrics/crap.edn.
// Coverage and Crap are nil for --no-coverage.
type Entry struct {
	Name       string
	Namespace  string
	Complexity int
	Coverage   *float64
	Crap       *float64
	Path       string
	Language   string
}
