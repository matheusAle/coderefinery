// Code generated from upstream dryer/cli.py HELP (do not edit by hand).
// sha256: fbb911ea8bd875c0486380726f6f00fe2327c231f33173817245df64256b1916

package dry

// Help is upstream dryer HELP, byte for byte.
const Help = `Usage: dryer [options] [path-or-filter ...]

Find candidate duplicate code in Clojure, Java, Go, TypeScript, Rust, and
Python. Each form is normalized — call and method names, operators, and
tree shape stay; local names, field names, and literals do not — and compared
with the other forms of the same language. The score is Jaccard similarity of
those structural fingerprints, the same score as dry4clj.

Prints a report and writes .metrics/dry.edn.

Languages: Clojure (.clj .cljc .cljs .cljd .bb), Java (.java), Go (.go),
TypeScript (.ts .tsx .mts .cts), Rust (.rs), Python (.py).

Options:
  -h, --help                    Print this help and exit.
  --root <path>                 Project root. Metrics are written here.
                                Default: the current directory.
  -s, --source-root <path>      Walk this tree instead of the project root.
                                May be repeated.
  --changed                     Compare added and modified source files from
                                git status.
  --threshold N                 Minimum similarity score, default 0.82.
  --min-lines N                 Minimum source lines in a candidate form,
                                default 4.
  --min-nodes N                 Minimum normalized syntax nodes, default 20.
  --format F                    text or edn, default text.
  --edn                         Same as --format edn.
  --text                        Same as --format text.

Arguments:
  path              File or directory to compare. Test paths are included when
                    you name them explicitly.
  filter            When the argument is not a path, only source files whose
                    path contains this text are compared.

With no paths, source files under the project root are compared. Directories
named .clj-kondo, .git, .hg, .idea, .metrics, .svn, .venv, __pycache__, __tests__, build, coverage, dist, node_modules, out, spec, specs, target, test, testdata, tests, vendor, and venv are skipped, as are *_test.go, *.test.cts,
*.spec.cts, *.spec.ts, test_*.py, and *_test.py files.

Clojure compares every top-level form except ns. The other languages compare
functions and methods. A form is only compared with forms in the same language.

Exit codes:
  0  the report was written
  1  usage error
  2  unknown output format
`
