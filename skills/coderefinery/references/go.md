# Go

Files: `.go`. Skipped: `*_test.go`, `testdata/`, `vendor/` and the other test
directories. Namespace: the import path; methods are `import/path.Receiver`.

## crap and mutate: coverage

- Per nearest `go.mod`: `go test ./... -coverprofile=<module>/target/coverage/go/coverage.out`.
- Coverage per function is statement coverage of the profile blocks inside its
  lines.
- Profiles are read only at the root and one or two directories below it. A
  module deeper than that is measured but never read: run from closer to it, or
  pass `--root`.
- A malformed profile stops the run (exit 1, one line on stderr).
- mutate runs `go test -count=1 ./<package dir>` in the module: package-scoped.
- stderr `Go coverage exited <n> in <module>` means the tests or the build
  failed. Fix them before trusting CRAP.

## --fix-go-coverage (mutate)

Profile keys are import paths (`example.com/x/calc.go`). Upstream matches them
to files by path suffix only, so when the module path does not mirror the
directories every site is `UNCOVERED` and stderr says
`No coverage data for <file>; its sites are uncovered`. Add `--fix-go-coverage`:
it maps each file through the nearest `go.mod` first. It is a coderefinery
addition, hidden from `mutate --help`. crap is not affected.

## --fix-overlay (mutate)

A package directory named `coverage`, `build`, `out`, `dist` or `target` (Go's
own `internal/coverage`, for example) is left out of upstream's worker copy, so
every mutant in a package that imports it fails to build and is reported KILLED
(`· by <build>` with `--attribute`). `--fix-overlay` links such a directory
when it directly holds `.go` files. `--attribute` turns it on. A coderefinery
addition, hidden from `mutate --help`.

## coderefinery test

- `go test -json -count=1 -covermode=count -coverprofile=<file> ./...` per
  module. Needs only Go (`go is not installed: install Go or pass --test-command`).
- Test ids are `<import path>::TestX/sub`. Subtests are separate cases; a parent
  test with subtests is never credited with a kill.
- `mutate --attribute` uses `go test -json`, package-scoped (already narrow).
- Status: live-tested.

## Branches

- Granularity `block`, from coverprofile blocks: `block 13-15 never executed`.
  It names unexecuted code inside an executed decision, never an arm, and is
  never called a branch. Read the decision to find which arm the block is in.
- crap's own profile is enough for `crap --branches`; `coderefinery test` is not
  required.

## Approximate or different

- The `module` line of `go.mod` is used verbatim; the first `go.mod` found wins
  even without a module line.
- `/` is never a mutation site (only `*` becomes `/`).
- Files the parser rejects contribute nothing: invalid Go, a leading BOM,
  `make(n - 1)` or `new(f(x))` where tree-sitter-go needs a type.
