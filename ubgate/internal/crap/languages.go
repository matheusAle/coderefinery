package crap

import "github.com/matheusAle/ub-mode/ubgate/internal/model"

// Reader extracts scored functions from one file's decoded source.
// path is relative to the project root (absolute when outside it);
// projectRoot is the absolute root (upstream languages.functions_in_file).
type Reader func(source, path, projectRoot string) ([]model.Function, error)

// Readers maps a crapper language to its reader. Languages without a reader
// yet contribute no functions. Parser/port agents add theirs here:
// clojure, java, go, typescript, rust.
var Readers = map[string]Reader{
	"go":      GoFunctions,
	"python":  PythonFunctions,
	"clojure": ClojureFunctions,
}

// FunctionsInFile is upstream languages.functions_in_file (mutator uses it).
func FunctionsInFile(language, source, path, projectRoot string) ([]model.Function, error) {
	reader, ok := Readers[language]
	if !ok {
		return nil, nil
	}
	return reader(source, path, projectRoot)
}
