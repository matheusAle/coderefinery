// Code generated from upstream mutator/cli.py HELP (do not edit by hand).
// sha256: 4c0ba54f147958930d75a4e05f2af3029d2e169ee64c31fea22bcb0126cf4ea7

package mutate

// Help is upstream mutator HELP, byte for byte.
const Help = `Usage: mutator [options] [path-or-filter ...]

Discover mutation sites in Clojure, Java, Go, TypeScript, Rust, and Python,
run the project's tests against each one, and write ` + "`" + `.metrics/mutate` + "`" + ` for
uml-viewer. Namespaces and function names are the ones crapper writes into
` + "`" + `.metrics/crap.edn` + "`" + `.

A form id is ` + "`" + `defn/<name>` + "`" + ` or, for a private operation, ` + "`" + `defn-/<name>` + "`" + `.
uml-viewer joins that name to the operation and joins ` + "`" + `:namespace` + "`" + ` to the
class.

Options:
  -h, --help                    Print this help and exit.
  --root <path>                 Project root. Metrics are written here.
                                Default: the current directory.
  -s, --source-root <path>      Walk this tree instead of the project root.
                                May be repeated.
  --changed                     Mutate added and modified source files from
                                git status.
  --scan                        List mutation sites. Do not run tests or
                                write metrics.
  --mutate-all                  Run every covered site, including ones the
                                last snapshot already killed.
  --since-last-run              Run survivors and sites in new or rewritten
                                functions. This is the default when a snapshot
                                exists.
  --lines <n,n,...>             Run only mutations on these source lines.
  --no-coverage                 Treat every site as covered.
  --use-existing-coverage       Read coverage already on disk. Same as
                                --reuse-coverage.
  --reuse-coverage              Read coverage already on disk.
  --coverage-command <cmd>      Run this command instead of the per-language
                                coverage tools, then read the reports it wrote.
  --test-command <cmd>          Run this command for the baseline and every
                                mutant. The working directory is the project
                                root.
  --timeout-factor <number>     Mutant timeout, as a multiple of the baseline
                                duration. Default: 10. The timeout is at least
                                2 seconds.
  --mutation-warning <number>   Warn when a file selects more covered sites
                                than this. Default: 50.
  --max-workers <number>        Run at most this many mutants of one file at
                                once. Default: one per core. The run uses the
                                smaller of this limit, the cores, and the
                                number of selected sites.
  --verbose                     Print each test command and each mutant.

Arguments:
  path              File or directory to mutate. Test paths are skipped.
  filter            When the argument is not a path, only source files whose
                    path contains this text are mutated.

With no paths, source files under the project root are mutated. Directories
named .clj-kondo, .git, .hg, .idea, .metrics, .svn, .venv, __pycache__, __tests__, build, coverage, dist, node_modules, out, spec, specs, target, test, testdata, tests, vendor, and venv are skipped.

The default, once a snapshot exists, reruns survivors and sites in functions
whose text changed. Killed mutants in unchanged functions are kept.

Selected mutants of one file run at the same time, one worker per core unless
--max-workers says otherwise. A worker is a symlink overlay under
target/mutation-workers with its own copy of the mutated file. The project
tree is left unchanged. A copy left under target/mutator-backup/ by an
interrupted older run is restored before a non-scan run.

Exit codes:
  0  every executed mutant was killed, or there was nothing to run
  1  usage error
  2  baseline tests failed
  3  at least one mutant survived

Coverage, when it is produced, uses the same commands as crapper:
  Clojure      clj -M:cov --lcov
  Java         Maven JaCoCo
  Go           go test ./... -coverprofile=...
  TypeScript   npm run coverage, Vitest, or c8
  Rust         cargo llvm-cov or cargo tarpaulin
  Python       coverage.py LCOV

A site on a line the report does not mark as hit is uncovered and is not run.
`
