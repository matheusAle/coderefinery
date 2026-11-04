# Judging the tests: test, mutate --attribute, tests, dry --tests

Open this before you run these commands, and before you delete or merge a test.

## Contents

- Order and files
- coderefinery test
- mutate --attribute (`<timeout>`, `<build>`, `--fix-overlay`, `--min-timeout`)
- coderefinery tests: columns and math
- Per-row actions
- Proving a deletion
- dry --tests
- crap --branches and mutate --branches

## Order and files

`coderefinery test` → `mutate --changed --use-existing-coverage --attribute` →
`coderefinery tests`. Each step reads what the previous one wrote:

```
.metrics/tests/runs.edn                       last test run (index)
.metrics/tests/<lang>[/<module>]/results.xml  JUnit (Go: results.json, the go test -json stream)
.metrics/tests/<lang>[/<module>]/coverage/    branch coverage of that run
.metrics/mutate/<namespace>.edn               :killed-by {"<mutation id>" ["<test id>" ...]}
.metrics/tests.edn                            the tests report
.metrics/branches.edn                         crap --branches
```

`<module>` is the module directory relative to the root with `/` turned into
`__`, omitted at the root. Test ids: per language, see the language reference.

## coderefinery test

- Runs each project's tests once (projects found like mutate finds them), with a
  JUnit reporter (`go test -json` for Go) and branch coverage. Test output is
  streamed. stderr then has one line per project, e.g.
  `python .: pytest 12 passed, 0 failed, 1 skipped (branch coverage, contexts)`,
  then `Wrote <path>/runs.edn`.
- Exit 0: every test passed or was skipped. Exit 2: a test failed or errored,
  the build or collection failed, or the runner wrote no results: fix the tests
  first. Exit 1: usage error, or a missing tool or plugin; the one line names
  what to install. coderefinery never installs it.
- `--repeat <n>`: run n times; a test whose status changes is recorded as flaky.
- `--changed`: only the projects of added and modified files, test files included.
- `--test-command <cmd>`: run your command (`/bin/sh -c`, in the root) instead.
  It must write JUnit XML or `go test -json` to `$CODEREFINERY_RESULTS`, and may
  write coverage to `$CODEREFINERY_COVERAGE_DIR`. Results land in
  `.metrics/tests/command/`.
- `--no-coverage`: results only. `--verbose`: more detail. `--root <path>`.

## mutate --attribute

- Run after `coderefinery test`. Every baseline and mutant run gets a JUnit
  reporter (`go test -json` for Go) and fail-fast is turned off. The tests that
  fail on a killed mutant go into `:killed-by` in `.metrics/mutate/<namespace>.edn`.
- KILLED lines end with ` · by <test id>`, plus ` (+N)` when more tests killed
  it. The table does not change.
- A kill credited to no test is `<timeout>` (the mutant timed out) or `<build>`
  (compile or collection error, no report).
- `--attribute` turns on `--fix-overlay`. Upstream's worker copy of the project
  leaves out every directory named `coverage`, `build`, `out`, `dist` or
  `target`, so a package that imports such a package directory (Go
  `internal/coverage`, a Python `build` package) cannot build in the worker and
  each of its mutants is a false `<build>` kill. `--fix-overlay` links such a
  directory when it holds source files of the mutated file's language. Plain
  `mutate` keeps upstream's copy unless you pass `--fix-overlay`.
  TypeScript/JavaScript keeps the skip list (`dist/` and coverage reports hold
  `.js`); there `--fix-overlay` copies the JS/TS sources into the worker instead,
  so Vitest + jsdom and Jest load the tests inside it (see
  `typescript-javascript.md`).
- `<timeout>` kills that come and go between runs are load, not tests. A
  mutant's timeout is `max(2 s, 10 × baseline)`, and a cached baseline can be a
  few milliseconds. `--min-timeout <seconds>` (for example 300) raises the 2 s
  floor; it changes nothing else in the output.
- The first attributed run re-runs each carried kill once (no `:killed-by` yet).
- Exit codes as plain mutate (0 all killed, 3 survivors, 2 baseline failed,
  1 usage or error). On exit 2 stderr adds `failing tests: <ids>`. A missing
  reporter or plugin exits 1 with one line naming what to install;
  `--test-command must write JUnit XML or go test -json to $CODEREFINERY_RESULTS`
  when your command wrote no report.
- Cost is about plain mutate (reporter I/O only), more where the project
  configured fail-fast (pytest `-x`, Jest/Vitest `bail`, nextest default). Python
  with pytest-cov runs only the covering tests per mutant; the others run the
  full suite.

## coderefinery tests: columns and math

Reads every `:killed-by` and the latest results. One row per test case
(`--group`: one per parametrized test, cases collapsed onto their base name):

```
Test relevance: 5 tests, 12 killed mutants (11 attributed)
Test                                              Kills Unique  Fns Level         Shadowed by
tests/test_pricing.py::test_tier_rate                 4      4    1 unit          -
tests/e2e/test_checkout.py::test_pays                 3      0    1 e2e           tests/test_pricing.py::test_tier_rate
tests/test_pricing.py::test_price_returns_a_number    0      0    0 unit          (no kills; runs 1 fn)

Same function, different levels
billing.pricing defn/price: unit 4 (test_tier_rate) · e2e 3 (test_pays) · keep unit; e2e adds no kills here

Unattributed kills: 1 <timeout>, 0 <build>
Offenders: 1 test kills nothing, 1 test is shadowed
```

- **Kills**: mutants the test fails on. **Unique**: kills no other row has.
  **Fns**: distinct functions among the kills.
- **Covering set**: greedy; each step takes the row with the most uncovered
  kills, ties to the lower level, then the shorter duration, then the id. Rows
  with Unique > 0 are always in it.
- **Shadowed by**: for a row outside the covering set with kills, the cover rows
  that already kill all its mutants (up to 2 ids, then `+N`). Deleting every
  shadowed row keeps every killed mutant killed.
- **Level**: from the test file's path segments, then its name. e2e: `e2e`,
  `end2end`, `end-to-end`, `cypress`, `playwright`, `acceptance`, `*.e2e.*`,
  `*_e2e*`. api (directories only): `api`, `contract(s)`, `http`, `rest`,
  `graphql`, also `*.api.*`, `test_api_*`, `*_api_test.*`. integration:
  `integration`, `integration_tests`, `integration-tests`, `it`, `functional`,
  `*IT.java`, `*ITCase.java`, `*.int.*`, `*.integration.*`, `test_integration_*`,
  `*_integration_test.*`, Rust `<crate>/tests/*.rs`. unit: `unit`, `unit_tests`,
  `unittest`, `*.unit.*`. Otherwise guessed from fan-out (namespaces among the
  killed functions): ≤2 is `unit~`, ≥3 is `integration~`. A `~` is a guess.
- Offenders (exit 3): rows with no kills, and shadowed rows. A Unique-0 row in
  the covering set (`-` under Shadowed by) is not an offender.
- Flaky tests (from `test --repeat`) are left out of the math and listed apart.
- `stale test id <id>` on stderr: a recorded killer is not in the latest
  results; it is dropped. Re-run `mutate --attribute` on its files.
- `--changed`: rows only for tests in changed test files and tests that kill a
  mutant in a changed source file. Unique and the covering set still use every
  test.
- Exit 0: no offenders. Exit 3: offenders. Exit 2: the latest results have
  failures (`fix the tests first`). Exit 1: usage, or
  `no attribution data: run coderefinery mutate --attribute`.
- `.metrics/tests.edn`: `:rows` (`"case"` or `"test"`), `:tests` (one map per row:
  `:id :file :level :level-inferred :kills :unique :fns :runs :cases :in-cover
  :shadowed-by :flaky :duration`), `:levels` (`:recommendation` `"keep-lower"` or
  `"keep-both"`), `:unattributed {:timeout :build}`.

## Per-row actions

| row | do |
|---|---|
| Kills 0, `(no kills; runs N fns)` | The test notices no change to the code. Delete it, or rewrite it to assert the behaviour it claims to test. |
| Unique 0, tests under Shadowed by | Read those tests. If they are at the same or a lower level, delete this one. If this test guards a seam (an integration boundary the lower tests stub out), keep it and say why in the commit message. |
| Unique 0, `-` under Shadowed by | The covering set needs it. Not an offender; keep it. |
| Shadowed by a self-checking oracle | When most shadowing comes from one test that compares two implementations with each other (an equivalence oracle), deleting the shadowed tests leaves the code guarded by that one oracle. Keep the specific tests; note it. |
| `keep <lower>; <higher> adds no kills here` | Keep the lower-level test for that function. Keep the higher one only if it has unique kills elsewhere. |
| `keep both; <higher> kills N here that <lower> tests miss: push a <lower> test down` | Write that lower-level test, then apply the rule above again. |
| `Flaky` section | Fix those tests; do not delete them. |
| `Unattributed kills: N <timeout>, M <build>` | Credited to no test; not offenders. Many `<build>` kills mean mutants that do not compile, not good tests. |
| Exit 2 | Fix the failing tests first; the report is not computed on failing results. |

## Proving a deletion

Deleting every shadowed test keeps every killed mutant killed. Prove it after
deleting: re-run `coderefinery test`, then
`coderefinery mutate --attribute --mutate-all <files those tests covered>`,
then `coderefinery tests`. No mutant that was killed may now survive.

## dry --tests

`coderefinery dry --tests --threshold 0.8` also compares test files (test
directories and test file names are no longer skipped). Output, `.metrics/dry.edn`
and exit codes are the same as plain dry. A pair inside the tests is a copied
test: parametrize it (one test over a table of cases), or extract the shared
setup into a fixture or helper.

## crap --branches and mutate --branches

- `crap --branches` adds after the report:
  ```
  Untested branches (python branch, go block, java branch-count)
  billing/pricing.py:16 if total <= 0 · true arm never taken
  calc/calc.go:12 if x > 0 · block 13-15 never executed
  No branch data: rust (tarpaulin reports lines only)
  ```
  and writes `.metrics/branches.edn` (`{:file :line :decision :arm :granularity :text}`
  per line). Only decisions that ran are listed; a never-executed decision is
  plain uncovered code that CRAP and UNCOVERED already show. The report,
  `crap.edn` and exit codes are unchanged. `--no-coverage --branches` is a usage
  error (exit 1).
- Granularity: `branch` names the arm (`true arm never taken`, `body never entered`,
  `exit never taken`, `case 2 never matched`, `operand 2 (b) never evaluated`).
  `branch-count` gives a count (`1 of 4 branches missed`): read the line to find
  the arm. `block`, `region`, `form` name code inside an executed decision that
  never ran.
- `No branch data: <language> (run coderefinery test)`: run `coderefinery test`
  first (Python always needs it).
- Do: write the test that takes that arm. If no honest input reaches the arm,
  delete the dead arm.
- `mutate --branches` gives each UNCOVERED line a reason:
  ` · in the true arm of :16 (if total <= 0), never taken`, ` · line never executed`,
  or ` · no coverage data for file`. `--scan` is not affected.
