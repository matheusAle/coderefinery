# Self-audit: coderefinery's gates run on coderefinery

- Date: 2026-11-04
- HEAD: `07edbc3` ("Document the test gates and teach the skill to act on them")
- Scope: the root module (`cmd/`, `internal/`, `tools/`). `fixtures/`, `**/testdata/`, `harness/` and `bench/` are not ours to judge and were left out of the findings.
- Identification only. No code was changed. The only repo file written is this one.

## How it was run

Everything ran on a scratch copy of HEAD with a binary built from that copy, so no `.metrics/` or `target/` reached the repo and concurrent edits in the main tree (the `react` work) did not leak in.

```sh
source harness/env.sh
mkdir -p .local/scratch/self-audit
git archive HEAD | tar -x -C .local/scratch/self-audit
cd .local/scratch/self-audit
go build -o bin/coderefinery ./cmd/coderefinery            # binary pinned to 07edbc3
B=bin/coderefinery

# 1. CRAP + untested arms (Go per-package coverage: go test ./... -coverprofile)
$B crap --branches cmd internal tools
#    second view, discarded (see caveat C3):
$B crap --coverage-command 'go test -count=1 -coverpkg=./... -coverprofile=target/coverage/go/coverage.out ./...' cmd internal tools/treediff tools/dryoracle/main.go

# 2. Duplicates
$B dry --threshold 0.8 cmd internal tools
$B dry --tests --threshold 0.8 cmd internal tools
$B dry --threshold 0.5 <the CLI files>                      # to size the file-selection cluster

# 3. Dead code (deadcode v0.38.0 installed into .local/go/bin via go install)
deadcode -test ./...        # 0 findings
deadcode ./...              # 4 findings (reachable only from tests)
go vet ./...                # clean
gofmt -l cmd internal tools # clean
#    + a grep pass for exported identifiers with no production reference
#    + a grep pass for package testdata files no test names

# 4. Test relevance (sampled)
$B test cmd internal tools/treediff tools/dryoracle/main.go          # Go only; 473 passed, 3 skipped
$B mutate --scan cmd internal tools/treediff tools/dryoracle/main.go # 8,899 sites
$B mutate --attribute --fix-go-coverage --use-existing-coverage --max-workers 4 internal/edn internal/pyjson internal/source internal/testplan
$B mutate --attribute --fix-go-coverage --use-existing-coverage --max-workers 4 internal/pyfmt internal/discover internal/branch internal/testreport internal/testresult
$B mutate --attribute --fix-go-coverage --use-existing-coverage --max-workers 4 internal/dry internal/syntax internal/testrun internal/lang/golang
$B tests
```

`coderefinery test` with no paths exits 1 here (`cannot add a JUnit reporter to npm test ("node --test")`, from `fixtures/`). With `cmd internal tools` it picks up `tools/oracle/*.py` as a pytest project with no tests and exits 2. Passing only the Go paths gives a clean run.

### Environment and load

- macOS, 10 cores, Go 1.26.1 (asdf, `GOTOOLCHAIN=local`), every cache under `.local/`.
- Load average was 6.4 to 9 for crap, dry and the first two mutate batches. It rose to 14.9 (1 min) and 17.4 (5 min) during the third batch, and reached 68 (1 min) at the end, which came from other agents.
- Wall times: crap with coverage took 16 s (5 s with a warm cache). dry took 0.1 s. `test` took 9 s. The three mutate batches took 16 s, 1 min 58 s and 7 min 1 s.
- Timeouts: there were 17 `<timeout>` kills (dry 8, golang 4, syntax 3, source 2), all in the batch that ran under load 15 to 17. A timeout counts as a kill, so load can flip a SURVIVED into a KILLED but never the reverse. Survivor counts are therefore a lower bound. The 17 cases are under 1% of executed mutants, so no conclusion below rests on them.
- The parity tests (`internal/lang/*/parity_test.go`) resolve the repo root through `REPO_ROOT`, which `harness/env.sh` exports. In the scratch run they therefore read `testdata/` and `tools/oracle/oracle.py` from the main tree, not from the scratch copy. Those files were unchanged at that point.

## Summary

| Gate | Findings | Top 3 |
|---|---|---|
| crap (CRAP ≥ 10) | 292 functions (279 Go + 6 Python oracle rows + 7 Go tool rows). 154 are in `internal/lang/*` parsers and 48 are at 0% unit coverage. | `testreport.readContexts` 272.0 (CC16, 0%) · `typescript.parser.defaultDeclaration` 167.5 (CC28, 43.8%) · `mutate.cljReader.dispatch` 167.4 (CC19, 25.6%) |
| crap --branches | 1,091 untested-arm lines (Go `block` granularity), 0 in testdata | `rust/items.go` 50 · `typescript/types.go` 43 · `rust/expr.go` 42 (`mutate/metrics.go` 41 is the top non-parser file) |
| dry 0.8 (source) | 59 pairs in about 10 clusters | CLI plumbing across crap/dry/mutate/testrun/testreport · grammar tree helpers copied into 5 parsers · crap/dry/mutate node predicates |
| dry --tests 0.8 | 28 test pairs | 5 near-identical `parity_test.go` · `writeFile`/`readFile` helpers in 5 packages · intra-file copies in `dry/clojure_test.go` |
| deadcode / vet | `-test`: 0. Without tests: 4 functions reachable only from tests. vet and gofmt clean. 0 unreferenced testdata. | `coverage.SuffixMatch` (no production caller) · `dryoracle.Parsable` (test-only) · `syntax.SExp`/`writeSExp` (cross-package test helper) |
| tests (sampled, 9 valid packages, 137 tests) | 3 zero-kill · 80 shadowed (77 in dry) · 74 same-function rows (30 "push a unit test down") | `dry::TestLanguageDetection`, `edn::TestStringEscapesOnlyBackslashAndQuote`, `pyfmt::TestRoundMatchesPythonTiesToEvenOnBinaryValue` (all three are mutator blind spots, not dead tests) |
| tests (4 invalid packages) | 39 zero-kill, all artifacts of caveat C1 | branch, testplan, testreport, testrun |

Top 5 actions by value. Line counts are estimated savings.

1. **Compact CLI plumbing** (file selection, `joinRoot`, `take`, `sourcePath`, `Main` wrappers): about 170 to 200 lines.
2. **Compact the 5 `parity_test.go` and the shared test helpers** into `internal/treediff` and `internal/testutil`: about 220 lines.
3. **Move grammar tree helpers** (`enclosingChild`, `insertSorted`, `removeHidden`, `field`, `attempt`) into `internal/syntax`: about 90 lines.
4. **Test the 0%-unit-coverage functions in their own packages** (`readContexts`, `ChangedFiles`, `treediff.*`, `pyfmt.ParseInt/PyRepr`, `GrammarFor`, `changedRows`): 0 lines saved, but 38 CRAP rows drop to CRAP = CC.
5. **Cover the untested arms of the worst partially covered functions** (`defaultDeclaration`, `cljReader.dispatch`, the `escapeEnd`/`escape` lexers, `pyInt`/`truthy`, `branch.mapper.llvm`): 0 lines saved, about 12 rows fall under 10.

---

## Caveats that change how to read the gates

**C1. The Go mutation overlay hides every package directory named `coverage`. This is upstream behaviour and invalidates 4 of the 13 sampled packages.**
`internal/mutate/workers.go:25` `skipLink` contains `"coverage"`. It mirrors `SKIP_LINK` in `upstream/mutator/src/mutator/workers.py:26-46`, and `linkChildren` applies it at every level of the overlay, not only at the root. When a mutant sits in `internal/branch`, the worker's `internal/` therefore has no `coverage/` directory. Every package that imports `internal/coverage` fails to build, and every mutant is reported `KILLED · by <build>`.
- Evidence: `internal/branch` had 210/210, `testplan` 39/39, `testreport` 222/222 and `testrun` 247/247 kills credited to `<build>`, with 0 survivors. Reproduced by hand: in a plain copy, `branch.go:39 > -> >=` compiles and fails `TestArcsPickTheArmHoldingTheTarget`, so it is a real test kill. `go list -deps -test` confirms that branch, testplan, testreport, testrun, crap and mutate all import `internal/coverage`.
- Consequence: mutation results for those packages (about 3,000 of the 8,899 sites, including crap at 904 and mutate at 1,214) mean nothing on this repo. Their 39 "(no kills)" rows are artifacts.
- **Addressed** (DEVIATIONS.md M8): `mutate --fix-overlay` links a skip-listed directory that holds the mutated language's sources, and `mutate --attribute` (this audit's flow) turns it on, so a re-run credits these kills to tests. Plain `mutate` keeps the upstream overlay.
- Decision for the owner: this is upstream parity, not our bug. It hits any Go, Java or Rust project with a package directory named `coverage`, `build`, `out`, `dist` or `target` below the root. Fix it or record it in `docs/DEVIATIONS.md`. Fixing it would change mutate output, so treat it as a product decision. It is not a compaction.

**C2. 818 of the 2,031 kills are `<build>`.** Apart from C1 (718), the remaining 100 are genuine non-compiling mutants, such as `edn.go:18 + -> -` on strings. The mutator counts them as kills, as upstream does. They inflate scores and are credited to no test.

**C3. CRAP coverage for Go is per package.** `go test ./... -coverprofile` credits a function only to tests in its own package, so 38 internal functions show 0% even though other packages or the harness exercise them (`treediff.*` is driven by the 5 parity tests, `discover.ChangedFiles` by the crap and dry CLI tests). Do not try to fix this with `-coverpkg=./...`. That profile repeats every block once per test binary (11,154 duplicate block keys), and `coverage.ParseGoProfile` appends rather than merges them, as upstream does. The result was 838 functions at CRAP ≥ 10 instead of 292. That run was discarded.

**C4. Shadowing in dry comes from an equivalence oracle.** 72 of the 77 shadowed dry tests list `TestScanningByStructureEqualsScanningByString` (`internal/dry/fingerprint_test.go:151`). That test scans a corpus that includes `fixtures/` and `internal/syntax`, then compares the structural and printed fingerprint paths with each other and with a nested-loop reference. It kills mutants because the two paths diverge, not because it checks a specific behaviour. Applying the skill's "delete the shadowed test" rule there would leave dry guarded mostly by one self-referential oracle. See Leave as is L5.

---

## Eliminate

| # | Where | Evidence | Proposal | Lines | Risk | Verify |
|---|---|---|---|---|---|---|
| E1 | `internal/coverage/coverage.go:153-157` `SuffixMatch` and its assertions in `internal/coverage/coverage_test.go:90-99` | `deadcode ./...`: unreachable func. The only references are the definition and one test. | Delete the function. Drop `SuffixMatch` from the test and keep the `NormalizePath` half. It ports upstream `coverage.suffix_match`, but no Go caller uses it, so output cannot change. | ~8 | none | `make ci`, harness 36/36 |
| E2 | `tools/dryoracle/main.go:62-71` `Parsable` | `deadcode ./...`: reachable only from `main_test.go:38` | Move it into `main_test.go`. It is not deleted, but it stops being production surface. | 0 (moved) | none | `go test ./tools/dryoracle` |
| E3 | Shadowed same-level unit tests (low confidence): `pyjson::TestQuoteEscapes` (1 kill, shadowed by `TestDumpsMatchesMutatorProbe`) and `testresult::TestLoadRejectsGarbage` (2 kills, shadowed by `TestKaochaAndTest2junitResolveTheNamespace`) | `coderefinery tests` | Read each test first. Delete it only if no assertion is unique beyond the mutation operators: `TestQuoteEscapes` checks escape tables, which the operators cannot express. Most likely outcome is to keep both. | ≤20 | low | Skill proof: `test`, then `mutate --attribute --mutate-all` on the covered files, then `tests` |

Not eliminated, with the reason:
- The 3 zero-kill tests are mutator blind spots. `edn.String` has only `+` string-concat sites, which become `<build>` kills. `pyfmt.Round`'s only sites are in the inf/nan guard (`pyfmt.go:17`), so the ties-to-even line 20 has no operator. `dry.LanguageOf` is a map lookup with no operator site. Each test asserts real upstream-parity output, so keep all three.
- `syntax.SExp`/`writeSExp` (`internal/syntax/dump.go:84-115`) are reachable only from the python and typescript parser tests. Go cannot share `_test.go` helpers across packages, so they must stay in a non-test file. An `internal/syntax/syntaxtest` package is the only alternative and buys nothing.
- No unreferenced testdata files were found. No exported identifier has zero references.
- `internal/treediff`, `tools/treediff` and `tools/oracle` show 0% coverage and no tests, but the 5 parity tests and `make treediff` drive them. They are dev tooling, not dead code.

## Compact

Ranked by lines saved and confidence. "Upstream-sensitive" means each copy currently mirrors a different upstream tool (crapper, dryer or mutator), so the merged version must keep each caller's exact messages and filters. The per-gate CLI tests and the harness goldens are the guard.

| # | Cluster (dry evidence) | Proposal | Lines | Risk | Verify |
|---|---|---|---|---|---|
| C1 | **File selection.** `internal/crap/cli.go:172-235` `SelectFiles`, `internal/dry/discover.go:91-157` `SelectFiles`, `internal/mutate/run.go:37-108` `selectFiles`. Scores 0.55 to 0.64 overall, with 1.00 sub-blocks: `joinRoot` at `crap/cli.go:164-169` = `dry/discover.go:71-76` = `mutate/run.go:16-21`. All three share the same skeleton: split positionals into existing paths and filters, then switch on changed / source roots / existing / default, then filter, then dedupe and sort. | One `discover.Select(opts, hooks)` with hooks for (a) the `--changed` keep predicate (crap: `LanguageOf && !IsTestFile`; dry: `trackedSource`; mutate: `!skippedPart && isSource`), (b) the iterator (dry `--tests` swaps in `iterWithTests`), (c) the explicit-file action (mutate prints `Skipping test file`), (d) the outside-root check (mutate only), (e) `ToSlash` in the filter (dry only). | ~120 | medium, upstream-sensitive | Per-gate `cli_test.go`, `dry --changed` and `--tests` tests, harness 36/36 |
| C2 | **`take` and `sourcePath`.** `take` at `crap/cli.go:98-103` = `dry/cli.go:33-38` = `mutate/cli.go:48-53` (1.00). `sourcePath` at `crap/cli.go:237-242` = `testrun/run.go:267-272` (1.00). | Move both into `discover` (or a small `internal/cliutil`) next to C1. | ~20 | low | `make ci` |
| C3 | **`Main` wrappers.** `dry/cli.go:140-165` ≈ `testreport/report.go:87-108` = `testrun/cli.go:110-131` (0.87 to 1.00). `crap/cli.go:287-312` and `mutate/cli.go:178-194` score 0.64 to 0.68. All five do: parse, help to stdout or stderr, run, `GitError` to its code, prefixed error to 1. | `cliutil.Main(prefix, parse, extraCheck, run)`. Keep each prefix (`dryer: `, `coderefinery tests: `, …) and dry's `Unknown format` check as an argument. | ~60 | low-medium (error prefixes are golden-tested) | `cmd/coderefinery` tests, harness |
| C4 | **Grammar tree helpers in 5 parsers.** `enclosingChild`/`insertSorted`/`removeHidden` at `lang/golang/extras.go:26-59` = `lang/java/extras.go:25-45` = `lang/typescript/extras.go:42-50` ≈ `lang/python/extras.go:31-64` ≈ `lang/rust/macro.go:222-242` (0.91 to 1.00). `field` at `lang/golang/parser.go:134-140` = `lang/python/parser.go:124-130`. `attempt` at `lang/python/parser.go:133-146` = `lang/rust/parser.go:137-150`. Also `lang/java/parser.go:151-167` = `lang/typescript/parser.go:194-210`. | Move the node-level helpers into `internal/syntax` (they touch only `*syntax.Node`). Diff the 0.91/0.93 rust and python variants first, because they may skip `Extra` nodes differently. `attempt` and `peek` depend on each parser's own `tok` and `parseError` types, so leave them. | ~90 | low | `TestParityWithTreeSitter` in all 5 lang packages, `make treediff` |
| C5 | **Report padding.** `padRight`/`padLeft` at `crap/report.go:15-27` = `mutate/report.go:13-25` = `testreport/report.go:280-285` (1.00). | Put `pyfmt.LJust`/`RJust` (Python `str.ljust`/`rjust`) in one place. | ~20 | low | Report golden tests |
| C6 | **Stdlib replacements.** `contains` at `discover/discover.go:41-48` = `containsString` at `mutate/coverage.go:142-149`. Also `discover/discover.go:139-142` = `testresult/resolve.go:20-23`. | Use `slices.Contains` (Go 1.26) and reuse the discover helper. | ~15 | none | `make ci` |
| C7 | **crap/dry/mutate node predicates.** `crap/lang_python.go:68-78` = `dry/extract.go:45-55` (nested-function test). `crap/lang_typescript.go:84-91` = `mutate/sites.go:124-131` ≈ `dry/extract.go:34-41` (0.87). `cljIsSpace` at `crap/lang_clojure.go:30-36` = `mutate/functions.go:22-28`. Inside crap: `lang_go.go:33-43` = `lang_rust.go:70-80`, and `lang_python.go:80-93` = `lang_rust.go:44-57` ≈ `lang_java.go:65-78` (0.84). | Share only what is the same by definition, such as the Clojure whitespace set and "walk parents to the first X". Keep tool-specific rules separate even when they are identical today, because crapper, dryer and mutator are three upstreams that can diverge. | ~40 | medium, upstream-sensitive | crap/dry/mutate language tests, harness |
| C8 | **dryoracle vs treediff.** `internal/treediff/treediff.go:42-54` = `tools/dryoracle/main.go:74-86` (`FindRoot`, 1.00). `Collect` is 0.61 (`treediff.go:67-104` vs `main.go:89-122`). | Have dryoracle call `treediff.FindRoot` and reuse `Collect` where the extension sets agree. | ~15 to 40 | none (dev tool) | `go test ./tools/dryoracle` |
| C9 | **Flag parsers.** `ParseArgs` in mutate (CC41), crap (CC26), dry (CC22), testrun (CC19) and testreport (CC13). Pairwise 0.53 to 0.67. | A table-driven option spec, with name, arity, value check and the upstream error text. This is the biggest CRAP win (5 rows ≥ 13) and also a compaction. Do it after C1 to C3. | ~100 | medium-high: each mirrors one upstream argparse's exact messages and ordering | Help and usage goldens (`TestHelpIsByteExact`, …), harness |
| T1 | **Parity tests.** The 5 `internal/lang/*/parity_test.go` (221 lines) are the same body with a different corpus dir and minimum (`java` ↔ `rust` 0.89, `python` ↔ `typescript` 0.80). | `treediff.RunParity(t, "java", 30)`, which keeps the skip-when-no-venv behaviour. Each file shrinks to 3 lines. | ~170 | low | Same tests pass, and still skip without the venv |
| T2 | **Test helpers.** `writeFile`/`readFile` at `mutate/helpers_test.go:17-35`, `testplan/testplan_test.go:16-25`, `crap/branches_test.go:51-58`, `crap/lang_go_test.go:194-202`, `crap/lang_rust_test.go:51-59`, plus `dry/languages_test.go:29-36` = `mutate/helpers_test.go:71-78`. The `parse` helper and the `TestInvalidInputIsAnError` shape appear in `lang/{golang,java,python,rust}/parser_test.go`. | Add an `internal/testutil` package with `WriteFile`, `ReadFile` and `MustParse`. | ~50 | none | `make test` |
| T3 | **Intra-file copied tests.** `dry/clojure_test.go:89-126` (3 identical bodies) and `:201-241`, `crap/lang_java_test.go:111-161`, `crap/lang_clojure_test.go:12-52` (0.81), `dry/cli_test.go:137-186` (0.81), `crap/cli_test.go:67-85` ≈ `crap/lang_rust_test.go:279-296` (0.93). | Make them table-driven, keeping the case names as subtest names so the tests report still groups them (`--group`). | ~60 | none | `make test` |

Total compaction potential is about 600 to 750 lines: about 480 in source and about 280 in tests. Against 31,363 non-test and 9,785 test lines in `cmd/`, `internal/` and `tools/`, that is roughly 1.5%.

## Split / test (CRAP ≥ 10, untested arms)

For each finding, write the test that takes the arm, or delete the arm if no honest input reaches it. The parser rows are in Leave as is.

**A. 0% in their own package (per-package coverage, C3). Add a direct unit test; CRAP falls to CC.** There are 38 such rows. The worst:

| Function | File:line | CC | CRAP | Note |
|---|---|---|---|---|
| `readContexts` | `internal/testreport/matrix.go:184` | 16 | 272.0 | Reads coverage.py contexts (Python `tests` path). No Go unit test feeds it a coverage.json with contexts; only end-to-end Python runs reach it. |
| `Collect`, `Compare`, `FindRoot`, `Oracle`, `Ours`, `Run`, `oracleBatch` | `internal/treediff/treediff.go:42-` | 4-11 | 20-132 | Only the parity tests in other packages drive these. |
| `pyfmt.ParseInt`, `pyfmt.PyRepr` | `internal/pyfmt/pyfmt.go:144`, `:172` | 10 | 110.0 | Used by dry and mutate. Port CPython `int()` and `repr()` cases. |
| `discover.ChangedFiles`, `git` | `internal/discover/discover.go:246` | 9, 4 | 90.0, 20.0 | Covered by the crap and dry CLI tests, not by discover's own tests. |
| `lang.GrammarFor` | `internal/lang/lang.go:63` | 8 | 72.0 | |
| `testreport.changedRows` | `internal/testreport/report.go:177` | 8 | 72.0 | `tests --changed` |
| `testrun.atLeast` | `internal/testrun/plan.go:154` | 7 | 56.0 | Version compare. Easy table test. |
| `testresult.looksLikeGoJSON` | `internal/testresult/…` | 6 | 42.0 | |

Seven tool `main` functions (`tools/treediff`, `tools/dryoracle`) and the 6 Python oracle rows (`tools/oracle/oracle.py`, `tools/dryoracle/dryer_oracle.py`) are dev harness code. Leave them.

**B. Partly covered and complex. These need arm tests (from `crap --branches`):**

| Function | File:line | CC / Cov | CRAP | Untested arms |
|---|---|---|---|---|
| `parser.defaultDeclaration` (TS) | `internal/lang/typescript/stmt.go:693` | 28 / 43.8% | 167.5 | `:696` `export default function` (blocks 697, 700), `:701` `export default async function`, `:707` named class heritage, `:711` `abstract class` |
| `cljReader.dispatch` (mutate) | `internal/mutate/clojure.go:213` | 19 / 25.6% | 167.4 | `:225` `#"regex"`, `:229` `#'var`, `:233` `#?@`, `:235` `#?`, `:237` `#^`, `:239` `#=` |
| `parser.escapeEnd` (TS lexer) | `internal/lang/typescript/lexer.go:328` | 19 / 30.8% | 138.8 | `:332` `\x`, `:337` `\u`/`\u{}` (blocks 338, 348, 353), `:354` octal |
| `pyInt` | `internal/mutate/metrics.go:340` | 14 / 21.1% | 110.4 | `:344` bool, `:353` float64, `:355` string (blank, underscore, invalid) |
| `lexer.escape` (Java) | `internal/lang/java/lexer.go:392` | 20 / 42.4% | 96.3 | `:394` EOF after `\`, `:406` `\x`, `:411` `\u` |
| `mapper.llvm` | `internal/branch/map.go:264` | 10 / 5.9% | 93.4 | The whole hits loop (`:265`, blocks 266-284). The Rust `region` path is effectively untested. |
| `parser.charRefEnd` (JSX) | `internal/lang/typescript/jsx.go:237` | 22 / 54.2% | 68.6 | `:243` `&#x…;`, `:251` `&#…;` |
| `truthy` | `internal/mutate/metrics.go:74` | 10 / 18.2% | 64.8 | `:76` nil, `:78` bool, `:80` int64, `:82` int, `:84` float64, `:88` `[]any` |
| `pythonPlan` | `internal/testrun/plan.go:206` | 9 / 14.3% | 60.0 | (decisions not executed) |
| `runStep` | `internal/testrun/run.go:305` | 16 / 46.7% | 54.8 | `:322` goJSON capture, `:336` `cmd.Start` failure, `:340` pipe |
| `pyjson.Dumps` | `internal/pyjson/pyjson.go:54` | 14 / 42.1% | 52.0 | `:57` nil, `:59` bool, `:66` int64, `:68` float64 |
| `sameFunction` | `internal/testreport/report.go:220` | 15 / 48.8% | 45.1 | `:242` loop, blocks 246-269 (the "keep both / push down" texts) |

**C. High CC, well covered. Split rather than test:** `mutate.ParseArgs` (CC41, 91%) and the other `ParseArgs` (see C9), `mutate.writeResults` (CC35, 93%), `testresult.ParseGoJSON` (CC31, 95%), `testresult.junitParser.convert` (CC44, 93%), `testrun.execute` (CC25), `mutate.mutateFile` (CC25), `testreport.Analyze` (CC25), `crap.SelectFiles`/`dry.SelectFiles`/`mutate.selectFiles` (CC21-23; these fold into C1).

Mutation survivors in the valid sample are a second test to-do list: 280 survivors and 184 uncovered sites. Top files: `lang/golang/parser.go` 69, `lang/golang/expr.go` 41, `testresult/junit.go` 40, `lang/golang/types.go` 29, `pyfmt/pyfmt.go` 23, `testresult/resolve.go` 20. The "Same function, different levels" section has 30 "push a unit test down" rows in valid packages. The largest are `testresult.junitParser.convert` (37 kills only integration tests make), `golang.lex` (17), `dry.sharedNeeded` (11), `testresult.ParseGoJSON` (8) and `testresult.parseXML` (8).

## Leave as is

| # | Finding | Why |
|---|---|---|
| L1 | 154 CRAP ≥ 10 rows in `internal/lang/{typescript,rust,python,java,golang}`, e.g. `declaration` CC56, `str` CC46, `classMember` CC39, `statement` CC37, `call` CC30 | Each routine mirrors one tree-sitter grammar rule. Splitting them moves the rule out of the shape the grammar reads in, and parity is checked against upstream tree-sitter output (`TestParityWithTreeSitter`, `make treediff`). Add arm tests where an arm is reachable (B above), but do not restructure. |
| L2 | Intra-parser dry pairs: `java/decl.go:164-192`, `java/parser.go:110-120`, `java/decl.go:404-410` ≈ `java/expr.go:299-305`, `rust/parser.go:270-278`, `python/parser.go:645-684`, `python/expr.go:434-479` (0.85), `typescript/expr.go:412-564` (0.85), `typescript/parser.go:103-117`, `typescript/stmt.go:445-455` | These are sibling grammar productions (e.g. two statement kinds whose rules differ by one token). Merging couples rules that tree-sitter keeps apart. Saves under 40 lines in total. |
| L3 | Three Clojure readers: `internal/crap/lang_clojure.go` (456 lines), `internal/dry/clojure.go` (463), `internal/mutate/clojure.go` (399) | Structurally different (only `cljIsSpace` scores ≥ 0.8). Each reproduces a different upstream reader (crapper form counting, dryer fingerprinting, mutator site finding) with different outputs and error texts. |
| L4 | `coverage.ParseGoProfile` appends duplicate blocks; the overlay `skipLink` hides nested `coverage`/`build`/… dirs (C1, C3) | Upstream parity. Changing either changes crap or mutate output. Record it in DEVIATIONS or fix it as a product decision, not as cleanup. |
| L5 | 77 dry tests "shadowed" mainly by `TestScanningByStructureEqualsScanningByString` | See C4. That oracle checks the two fingerprint paths against each other. The shadowed tests pin specific dryer quirks (`TestABareColonIsAnEmptyKeyword`, `TestHelpIsByteExact`, `TestTextReportMatchesDry4clj`, …). Kills only measure what the operators can express. Keep them. |
| L6 | 3 zero-kill tests | Mutator blind spots (see Eliminate). |
| L7 | `syntax.SExp`, the `tools/treediff` and `tools/oracle` dev tooling | Test and dev infrastructure that other packages' tests and `make treediff` need. |

## Cost of a full-repo mutation run, from the sample

- Sites: 8,899 (`--scan`). The sample covered 13 packages (2,690 sites), of which 9 gave valid results (about 1,780 sites).
- Measured throughput at `--max-workers 4`: dry + syntax + lang/golang (about 1,210 sites, test runs of about 2 s) took 7 min under load 15 to 17, which is about 0.35 to 0.4 s of wall time per site. Packages whose mutants all failed to build (C1) ran several times faster, so batch 2 overstates throughput.
- Per-package `go test -count=1` time sets the per-mutant cost: `cmd/coderefinery` 10.8 s, `crap` 5.1 s, `testrun` 3.9 s, `mutate` 3.1 s, the five `lang/*` 2.0 to 2.5 s, the rest about 2 s.
- Estimate for all 8,899 sites at 4 workers on this machine: about 60 to 90 min. crap alone is about 20 min and mutate about 15 min. That is still not useful until C1 is resolved, because about 3,000 sites (crap, mutate, branch, testplan, testreport, testrun, coverage's importers) would all be reported killed-by-build. The tests report's zero-kill and shadowed lists would also be wrong for those packages.

## How to verify any change made from this list

- `make ci` (gofmt, vet, `go test ./...`, build), then the harness at 36/36 (`harness/`). Parity with upstream is the product.
- Compactions marked upstream-sensitive (C1, C3, C7, C9) must keep every message, filter and ordering byte for byte. Run the per-gate CLI tests and the harness goldens before and after.
- Grammar changes (C4): all five `TestParityWithTreeSitter` must pass with the upstream venv present (they skip silently without it), plus `make treediff`.
- Test deletions: follow the skill's proof (`test`, then `mutate --attribute --mutate-all` on the files those tests covered, then `tests`). Only do this in packages that do not import `internal/coverage`, until C1 is resolved.
