# design-tests: test results, per-test kills, untested branches

Status: Track A (`internal/testresult`, `testrun`, `testreport`, `mutate --attribute`) and Track B are built; the owner decisions are in §9. Source: the owner's feature request and `designs/motion-spec.md` §8b
(S7 dedup, S8 relevance). Everything here is **opt-in**. Default `crap`, `dry` and `mutate` keep their
stdout, stderr, exit codes, `--help` and `.metrics` bytes; `harness/run.sh check` stays at 36/36 after
every merge. The new crap/dry/mutate flags follow the `--fix-go-coverage` precedent: they are left out of
the upstream `--help` text and are documented in README, SKILL.md and DEVIATIONS.md ("Additions").
The code stays stdlib-only Go. `coderefinery` never installs anything for these features. When something
is missing it prints one line naming what to install and exits 1.

## 1. Data model: `internal/testresult` (Track A owns it; everyone reads it)

```go
package testresult

type Status string
const (Pass Status = "pass"; Fail Status = "fail"; Skip Status = "skip"; Error Status = "error")

type Case struct {
    ID       string   // canonical id, §1.1. Stable across runs and tools; the kill-matrix join key
    Locator  string   // repo-relative POSIX test file; else the tool's container (Go import path, nextest binary id)
    File     string   // repo-relative POSIX test file, "" when it cannot be resolved
    Suite    []string // class / describe path, outermost first
    Name     string   // leaf name incl. parameters: "test_rate[gold]", "TestX/sub"
    Base     string   // ID with the trailing parameter or subtest part removed (== ID if there is none)
    Parent   bool     // Go: a test with subtests. Its status is derived from the subtests and never credited with a kill
    Status   Status
    Duration float64  // seconds, 0 when absent
    Failure  string   // <failure>/<error> message + text, or the test's go output; trimmed, capped at 4 KiB
}

type Run struct {
    Tool        string // pytest|vitest|jest|mocha|surefire|gradle|nextest|kaocha|test2junit|go|junit
    Cases       []Case // sorted by ID. Duplicate IDs merge: error>fail>pass>skip, durations summed
    BuildFailed bool   // a collection/compile/package failure that is not tied to a case
}

func Load(path, root, toolHint string) (*Run, error) // file or dir (Surefire dirs); sniffs '<' JUnit, '{' go -json
func ParseJUnit(r io.Reader, root, toolHint string) (*Run, error)
func ParseGoJSON(r io.Reader, root string) (*Run, error) // ignores non-JSON lines (runner output is merged)
func Merge(runs ...*Run) *Run
func (r *Run) Failing() []string                         // IDs with fail|error and !Parent, sorted
```

JUnit status: a `<failure>` child is fail, `<error>` is error, `<skipped>` is skip, anything else is pass.
Duration comes from `time`. `toolHint` (the wrapper knows which tool it ran) beats sniffing the root
element name (`pytest`, `vitest tests`, `jest tests`, `Mocha Tests`, `nextest-run`, Surefire schema).
Go: the last `pass|fail|skip` action per (Package, Test) wins. A `fail` event with a Package but no Test,
or `[build failed]`, sets BuildFailed.

### 1.1 Canonical IDs: `ID = Locator + "::" + join(Suite, "::") + "::" + Name` (empty parts dropped)

| tool | Locator / Suite / Name from | Base strips |
|---|---|---|
| pytest (`-o junit_family=xunit1`) | `file` attr; classname minus the module's dotted path; `name`. The ID equals pytest's node id | trailing `[...]` |
| Vitest junit | classname (the file path, relative to the package → made relative to the repo); `name` split on ` > ` | none (`each` titles differ) |
| jest-junit (`JEST_JUNIT_ADD_FILE_ATTRIBUTE=true`) | `file` attr; ancestor describe titles; title | none |
| mocha-junit-reporter | testsuite `file` attr; suite titles; `name` | none |
| Surefire / Gradle | the FQCN resolved to `src/test/{java,kotlin}/<pkg>/<Outer>.java` (else the first lexical match of `**/<Outer>.java` outside target/build, else the FQCN); `Outer$Inner` gives Suite `[Outer, Inner]`; name without `()` | trailing `[n]` or `(...)[n]` |
| cargo nextest | the binary id (`crate`, `crate::bin/x`, `crate::tests_file`), and the file when it is a `tests/<x>.rs` binary; name = test path | rstest `::case_\d+.*` |
| Kaocha / test2junit | the ns resolved to a file under the test paths (`-`→`_`); var name | none |
| `go test -json` | the package import path (File is resolved by grepping the package's `_test.go` files for `func TestX(`); Name = `TestX/sub/...` | everything from the first `/` |

## 2. `coderefinery test` (Track A, `internal/testrun`)

Runs each detected project's tests once, with a JUnit reporter (go: `-json`) and branch coverage, and
writes §5's files. Detection reuses the mutator test plan (`internal/testplan`, extracted in step 0):
the language markers and the cwd of spec-mutator §5.1. The modules are the nearest-marker dirs of the
selected source files (crapper's §5.3 module rule). Each module runs once.

Flags: `--root <path>`, `--test-command <cmd>` (`/bin/sh -c`, cwd root; coderefinery exports
`CODEREFINERY_RESULTS=<abs file>`, which must receive JUnit XML or `go test -json`, and
`CODEREFINERY_COVERAGE_DIR=<abs dir>`, optional), `--no-coverage`, `--repeat <n>` (default 1; a test whose
status differs between runs is recorded as `:flaky`), `--changed`, `--verbose`, positional paths or
filters (as crap). Usage errors follow the upstream style (`<msg>\n\n<HELP>` on stderr, exit 1).
Exit: **0** every case passed or skipped · **1** usage, or a missing tool/plugin (the one-line hint) ·
**2** a case failed or errored, BuildFailed, or the runner exited non-zero without writing results.
Test output is streamed (inherited), as crap does. Then one line per module on stderr:
`python .: pytest 12 passed, 0 failed, 1 skipped (branch coverage, contexts)`, and finally `Wrote <abs runs.edn>`.

### 2.1 Per language (implementation order: Python, Go, TS/JS, Java, Rust, Clojure)

| lang | command (cwd = plan cwd) | reporter, how it is enabled | missing → one-line hint (exit 1) | branch source → granularity | `mutate --attribute` | approximate / unsupported |
|---|---|---|---|---|---|---|
| Python (pytest) | `<py> -m coverage run --branch --data-file=D [--source=S] -m pytest --junitxml=R -o junit_family=xunit1 --maxfail=0`, then `coverage json --show-contexts -o C.json` and `coverage lcov -o C.lcov`. If `pytest_cov` imports: `<py> -m pytest --cov=S --cov-branch --cov-context=test --cov-report= ...` with `COVERAGE_FILE=D` instead of `coverage run` | built in. `--maxfail=0` cancels an `-x` in addopts | `coverage is not installed for <py>: run <py> -m pip install coverage` | coverage.py JSON `missing_branches` → **branch** (statement-level `if/elif/for/while/match/try` arcs; no short-circuit `and/or`, no ternaries) | narrowed to the covering tests (§3.3), else the full suite | `unittest discover` projects: `unittest has no JUnit reporter: install pytest (it runs unittest suites) or pass --test-command`. Contexts need pytest-cov; without it there is no narrowing, only a note |
| Go | `go test -json -count=1 -covermode=count -coverprofile=C.out ./...` per module; stdout → R.json | `-json` (stdlib) | none | coverprofile blocks → **block**, never called branch | `go test -json -count=1 <pkg>`: package-scoped, already narrow | subtests are separate cases; a parent with failed subtests is not credited |
| TS/JS Vitest | `npm test -- --reporter=default --reporter=junit --outputFile.junit=R --coverage --coverage.reporter=json --coverage.reporter=lcov --coverage.reportsDirectory=CD --coverage.reportOnFailure=true`, env `CI=true` | built-in junit reporter | provider missing: `install the Vitest coverage provider: npm i -D @vitest/coverage-v8@<vitest version>` | `coverage-final.json` branchMap → **branch** (arm-precise: if, cond-expr, binary-expr operands, switch cases, default args) | full suite (`npm test -- <reporter args>`) | only when `scripts.test`'s first token is `vitest`/`jest`/`mocha`; any other script: `cannot add a JUnit reporter to npm test ("<script>"): pass --test-command` |
| TS/JS Jest | `npm test -- --ci --reporters=default --reporters=jest-junit --coverage --coverageReporters=json --coverageReporters=lcov --coverageDirectory=CD`, env `JEST_JUNIT_OUTPUT_FILE=R JEST_JUNIT_ADD_FILE_ATTRIBUTE=true` | jest-junit plugin | `install jest-junit: npm i -D jest-junit` | same JSON → **branch** | full suite | as above |
| TS/JS Mocha | `npx --no-install c8 --reporter=json --reporter=lcov --reports-dir=CD npm test -- --reporter mocha-junit-reporter --reporter-options mochaFile=R` | mocha-junit-reporter | `install mocha-junit-reporter c8: npm i -D mocha-junit-reporter c8` | c8 JSON → **branch** | full suite | node:test and other runners: `--test-command` only |
| Java Maven | `mvn -q org.jacoco:jacoco-maven-plugin:0.8.12:prepare-agent test org.jacoco:jacoco-maven-plugin:0.8.12:report -Dmaven.test.failure.ignore=true -Dsurefire.skipAfterFailureCount=0`; copy `target/surefire-reports/TEST-*.xml` → R/ and `target/site/jacoco/jacoco.xml` → C.xml | Surefire always writes XML | none (Maven resolves the plugin, as crap already does) | JaCoCo `<line mb cb>` → **branch-count** (which line and how many arms, not which arm) | full module suite. The worker's `target/surefire-reports` is cleared before every run | Gradle: `test` only (`./gradlew test jacocoTestReport --continue`, `build/test-results/test/`); without the jacoco plugin: `apply the jacoco plugin to get coverage, or pass --no-coverage`. mutate has no Gradle plan (upstream), so attribution needs `--test-command` |
| Rust | `cargo llvm-cov nextest --no-report --no-fail-fast --config-file N --profile coderefinery`, then `cargo llvm-cov report --json --output-path C.json` and `--lcov --output-path C.lcov`. N (generated under §5) holds `[profile.coderefinery] fail-fast = false` and `[profile.coderefinery.junit] path = "junit.xml"`; the result is copied to R | nextest JUnit | `install cargo-nextest: cargo install cargo-nextest --locked` (and likewise `cargo-llvm-cov`) | llvm-cov JSON regions → **region**; real **branch** only with `--branch` (nightly), and then it says branch | `cargo nextest run` with the generated profile (it replaces `cargo test`) | plain `cargo test` has no stable JUnit (`--format junit` is nightly `-Z`): unsupported, hint above. nextest runs no doctests, so a mutant that only a doctest kills survives under `--attribute`. `--config-file` replaces the project's own `.config/nextest.toml`. tarpaulin has no branch data |
| Clojure | deps.edn `:test` alias running Kaocha: `clj -M:test --plugin kaocha.plugin/junit-xml --junit-xml-file R --plugin cloverage --cov-output CD --lcov` | kaocha-junit-xml | `add lambdaisland/kaocha-junit-xml (and lambdaisland/kaocha-cloverage) to the :test alias` | Cloverage HTML forms → **form** | full suite | speclj, bb and `clj -M:spec`: unsupported (`--test-command`). lein test2junit: parser only |

R = results path, D = coverage data file, C/CD = coverage outputs. All paths follow §5.

## 3. `mutate --attribute` (Track A)

- Baseline and mutants run the §5.1 plan of spec-mutator **plus** the column "mutate --attribute" injection
  above. The fail-fast overrides are in the commands (`--maxfail=0`, `skipAfterFailureCount=0`,
  `--no-fail-fast`, nextest `fail-fast=false`). Each worker writes its report to
  `<worker>/.coderefinery/results.{xml,json}`, deleted before every run (`.coderefinery` is never linked).
  `--test-command` runs get `CODEREFINERY_RESULTS` pointing there. The baseline cache key includes the
  attribute mode.
- The baseline must produce a parseable report with zero failing cases. No report → exit 1 with the
  §2.1 hint (or `--test-command must write JUnit XML or go test -json to $CODEREFINERY_RESULTS`). A failing
  baseline keeps exit 2, and stderr adds `failing tests: <ids...>`.
- Outcome classification does not change (`timed_out or rc≠0` → killed). Attribution for a killed mutant:
  - timeout → `["<timeout>"]`. A partial report is ignored, and the kill is credited to no test.
  - otherwise `Run.Failing()`. If that is empty (compile error, collection error, BuildFailed, no report)
    → `["<build>"]`.
- History: under `--attribute`, a carried `killed` outcome that has no `:killed-by` entry is **selected**
  (re-run once). This is a one-time migration cost. Without the flag, selection is upstream §7.1.
- stdout under the flag only: KILLED site-log lines get ` · by <first id>` plus ` (+N)` when there are more.
  The table is unchanged.

### 3.1 Snapshot: the `:killed-by` key

`.metrics/mutate/<ns>.edn` gains one **last** top-level key, after `:forms`, written only when non-empty:
```
{:version 1 :tested-at "…" :source "…" :namespace "…" :outcomes {…} :forms […] :killed-by {"<mutation id>" ["<test id>" …] …}}
```
Keys are the exact `:outcomes` key strings, sorted the same way. Values are sorted, deduped test IDs and
sentinels (`"<build>"`, `"<timeout>"`), rendered with the existing EDN string escaping. Merge rules extend
spec-mutator §7.5: kept entries follow kept outcomes; fresh entries come only from attributed runs. A
**default** run keeps entries for outcomes it carried, drops entries for mutants it re-ran, and keeps other
files' entries untouched. With no `:killed-by` in the input, the bytes match upstream (golden test). Upstream
mutator ignores the unknown key when reading.

### 3.2 Cost

Per file: `1 baseline + selected mutants × T(suite)`, divided by the workers, as with plain mutate. Upstream
plans never pass fail-fast flags, so the extra cost over plain mutate is reporter I/O (≈1.0–1.1×). It
reaches T(suite)/T(first failure) only where the project configured fail-fast (pytest `-x`, Vitest/Jest
`bail`, nextest's default `fail-fast=true`). The first attributed run also re-runs every carried kill.

### 3.3 Python narrowing (the only one in v1)

Used when `.metrics/tests/python[/<slug>]/coverage/coverage.json` has contexts and is newer than every `.py` under
the module. For each mutant, tests = the contexts on `site.line ∪ site.anchor_line` (pytest-cov label
`<nodeid>|run`, with `|setup|run|teardown` stripped). The mutant runs `<py> -m pytest <nodeids…>` + the
reporter. Fall back to the full suite when the set is empty, has more than 200 ids, or the argv would exceed
100 KB. **A survivor of a narrowed run is re-run with the full suite**, so killed/survived equals the full
suite; only the kill set can be a subset. Cost ≈ Σ T(covering tests) + survivors × T(suite).

## 4. `coderefinery tests`: the relevance report (Track A, `internal/testreport`)

Inputs: every `:killed-by` in `.metrics/mutate/**/*.edn`, every results file under `.metrics/tests/`
(the test universe, durations, statuses, `:flaky`), and Python contexts (optional). Flags: `--root`,
`--changed`, `--group`. Exit: **0** clean · **3** a zero-kill test or a shadowed test in scope ·
**2** the latest results contain failing cases or BuildFailed ("fix the tests first") · **1** usage, or no
`:killed-by` anywhere (`no attribution data: run coderefinery mutate --attribute`).

```go
package testreport
type Mutant struct {
    ID, File, Namespace, Form, Original, Mutant string // decoded from the mutation id JSON
    Start, End int
    Killed     bool
    KilledBy   []string // real test IDs only, sorted
    Sentinel   string   // "", "<build>", "<timeout>"
}
type Matrix struct {
    Tests        []testresult.Case // results ∪ every id in KilledBy (synthesised Case when unseen); sorted by ID
    Mutants      []Mutant          // sorted by ID
    Kills        map[string][]int  // test ID -> ascending indices into Mutants
    Unattributed map[string]int    // sentinel -> count
}
func Load(root string) (*Matrix, error)
func (m *Matrix) ByBase() *Matrix // --group rows: cases collapse onto Base (kills = union)
type Level string // "unit" | "integration" | "api" | "e2e"
type Row struct {
    ID, File      string
    Level         Level
    LevelInferred bool     // decided by fan-out, shown as "unit~"
    Kills, Unique, Fns int // Fns = distinct (namespace, form) among the kills
    Runs          int      // functions executed per contexts; -1 when unknown
    InCover       bool
    ShadowedBy    []string
    Flaky         bool
    Cases         int
}
func Analyze(m *Matrix, durations map[string]float64) []Row
```

Definitions. **kills** = mutants whose KilledBy contains the test. **unique** = kills that no other row has
(computed at the row granularity: cases by default, the S8 per-case table; parametrized groups with
`--group`). **Cover** = greedy set cover over U = mutants with ≥1 real killer. Each step picks the row with
the most uncovered kills. Ties go to the lower level (unit<integration<api<e2e), then the smaller duration
(missing = +∞), then the smaller ID. It stops when U is covered. Rows with unique>0 are always in the
cover. **shadowed by** (rows outside the cover with kills>0) = the same greedy algorithm run over that
row's kills using only cover rows. Up to 2 IDs are printed, then `+N`. Deleting every shadowed row keeps
the killed set. Exit 3 counts zero-kill and shadowed rows. A zero-unique row in the cover is reported but
is not an offender, because the cover needs it. Flaky rows (`test --repeat`) are left out of the math and
listed separately (fix them, don't delete them). Sentinel kills are counted only in the footer. A test id
in `:killed-by` that is missing from the current results gets a stderr note `stale test id <id>` and is
dropped.

**Level**, first match on the test file's lowercased path segments, then its name:
e2e `e2e end2end end-to-end cypress playwright acceptance`, `*.e2e.*`, `*_e2e*` · api `api contract
contracts http rest graphql` (dirs only), `*.api.*`, `test_api_*`, `*_api_test.*` · integration
`integration integration_tests integration-tests it functional`, `*IT.java`, `*ITCase.java`, `*.int.*`,
`*.integration.*`, `test_integration_*`, `*_integration_test.*` (Go keeps integration tests in the package), Rust `<crate>/tests/*.rs` · unit `unit unit_tests unittest`,
`*.unit.*`. Otherwise fan-out: the distinct namespaces among the killed forms, plus the functions executed
per contexts. ≤2 gives `unit~`, ≥3 gives `integration~`.

`--changed`: the rows are tests in changed test files plus tests that kill a mutant in a changed source file.
All math still runs over the whole matrix, so unique and cover never depend on the scope.

Text report (stdout). The ID column is `tw = max(4, min(72, max rune len))`, never truncated:
```
Test relevance: 5 tests, 12 killed mutants (11 attributed)
Test                                              Kills Unique  Fns Level         Shadowed by
-----------------------------------------------------------------------------------------------
tests/test_pricing.py::test_tier_rate                 4      4    1 unit          -
tests/test_pricing.py::test_coupon_never_goes_negative 1     1    1 unit          -
tests/test_pricing.py::test_unknown_tier_pays_full_price 1   1    1 unit          -
tests/e2e/test_checkout.py::test_pays                 3      0    1 e2e           tests/test_pricing.py::test_tier_rate
tests/test_pricing.py::test_price_returns_a_number    0      0    0 unit          (no kills; runs 1 fn)
tests/test_pricing.py::test_settle_called             0      0    0 unit          (no kills; runs 2 fns)

Same function, different levels
billing.pricing defn/price: unit 4 (test_tier_rate) · e2e 3 (test_pays) · keep unit; e2e adds no kills here

Unattributed kills: 1 <timeout>, 0 <build>
Offenders: 2 tests kill nothing, 1 test is shadowed
```
Row format: `f"{id:<tw} {kills:5d} {unique:6d} {fns:4d} {level:<13} {shadow}"`. Rows sort by
(offender last, -unique, -kills, ID). The "same function" section lists each form killed by rows at ≥2
levels. If the higher level's kills in that form ⊆ the lower levels' kills, it prints `keep <lower>; <higher>
adds no kills here`. Otherwise it prints `keep both; <higher> kills N here that <lower> tests miss: push a
<lower> test down`. Empty sections are omitted. stderr ends with `Wrote <abs>/.metrics/tests.edn`.

## 5. `.metrics` layout and EDN shapes

```
.metrics/tests/runs.edn                       last `coderefinery test` run (index)
.metrics/tests/<lang>[/<slug>]/results.xml    JUnit (Surefire/Gradle: results/ dir of TEST-*.xml)
.metrics/tests/<lang>[/<slug>]/results.json   go test -json stream (Go)
.metrics/tests/<lang>[/<slug>]/coverage/      python .coverage coverage.json lcov.info · go coverage.out ·
                                              ts coverage-final.json lcov.info · java jacoco.xml ·
                                              rust llvm-cov.json lcov.info nextest.toml · clojure html/ lcov.info
.metrics/tests.edn                            `tests` report
.metrics/branches.edn                         `crap --branches`
.metrics/mutate/<ns>.edn                      + :killed-by (§3.1)
```
`<slug>` = the module dir relative to root with `/`→`__`, omitted at the root (crapper §5.3). A module dir
holds one tool's output. `test` rewrites the dir of each module it ran and leaves the others alone. One
`.metrics/tests/results.xml` would collide in multi-language repos, hence the dirs. Crap and mutate never
glob `.metrics/`, so these files cannot change default numbers.

EDN uses the `internal/edn` string/number rules, one map per line inside a vector, like crap.edn:
```
{:version 1 :tested-at "2026-11-03T12:00:00-03:00" :runs [
  {:language "python" :module "." :tool "pytest" :command "…" :exit 0 :results ".metrics/tests/python/results.xml" :coverage ".metrics/tests/python/coverage/coverage.json" :granularity "branch" :contexts true :pass 12 :fail 0 :skip 1 :error 0 :flaky []}
]}
{:version 1 :generated-at "…" :rows "test" :tests [
  {:id "tests/test_pricing.py::test_tier_rate" :file "tests/test_pricing.py" :level "unit" :level-inferred false :kills 4 :unique 4 :fns 1 :runs 1 :cases 3 :in-cover true :shadowed-by [] :flaky false}
] :levels [
  {:namespace "billing.pricing" :form "defn/price" :by-level {:unit ["…"] :e2e ["…"]} :recommendation "keep-lower"}
] :unattributed {:timeout 1 :build 0}}
{:version 1 :generated-at "…" :branches [
  {:file "billing/pricing.py" :line 16 :decision "if total <= 0" :arm "true" :granularity "branch" :text "true arm never taken"}
]}
```
`:rows` is `"case"` (default) or `"test"` (`--group`). Each row also carries `:duration` (seconds, `nil` when
unknown). runs.edn rows also carry `:build-failed`. A `--test-command` run writes `.metrics/tests/command/results`
(JUnit or go -json, sniffed) and `command/coverage/` (`$CODEREFINERY_COVERAGE_DIR`). `:recommendation` is `"keep-lower"` or `"keep-both"`.

## 6. Untested branches (Track B)

**Raw data** (`internal/coverage`, new files `branch_*.go`; coverage still never imports crap):
```go
type ArmHit struct {
    Line, Column, EndLine, EndColumn int // where the tool locates the arm/block/region (columns may be 0)
    FromLine, ToLine int                 // coverage.py arcs only (ToLine < 0 = function exit)
    Index, Of        int                 // arm k of n (Istanbul, BRDA, JaCoCo mb+cb)
    Kind             string              // tool kind: if|cond-expr|binary-expr|switch|default-arg|arc|block|region|line-count|form
    Hits             int
}
type BranchFile struct{ Granularity string; Arms []ArmHit } // branch|branch-count|block|region|form
type BranchBundle struct{ Files map[string]*BranchFile; Keys []string; Missing map[string]string } // language -> why there is no data
func LoadBranches(root string) (*BranchBundle, error)
```
Per language, `.metrics/tests/<lang>/**` (from `test`) is read first. Otherwise the CRAP reports already on
disk: Go profile → block; JaCoCo line `mb/cb` → branch-count; LCOV `BRDA` (Vitest/c8, llvm-cov
`--branch`) → branch-count; Cloverage HTML → form. Python CRAP LCOV has no BRDA, so it gets `Missing =
"no branch data: run coderefinery test"`. tarpaulin gives `"tarpaulin reports lines only"`. Files are
matched with the existing `SelectKey`.

**Decisions** (`internal/crap/decisions.go`, new): the same nodes the complexity predicates count, with
their arms.
```go
type Arm struct{ Label string; StartLine, EndLine int } // "true" "false" "body" "exit" "case <n>" "handler" "operand <n>"
type Decision struct{ Path string; Line, Column, EndLine int; Kind, Header string; Arms []Arm }
func Decisions(source, path, projectRoot string) ([]Decision, error) // per-language, preorder; Clojure via its scanner (form lines)
```
**Mapping** (`internal/branch`, new; imports crap + coverage):
- An arc (from, to) belongs to the decision on line `from`, and to the arm whose span contains `to`.
  Otherwise it is the `false`/`exit` arm.
- An Istanbul or BRDA entry belongs to the decision on its line, ties broken by column. The arm is Index.
- A Go block or Rust region with Hits=0 belongs to the innermost decision arm that contains its start.
- A JaCoCo line-count belongs to the decision(s) on that line.
- Only arms of **executed** decisions are reported. A decision on a never-executed line is plain uncovered
  code, and CRAP/UNCOVERED already show it.
- Text per granularity (block/region/form are never called a branch):
  branch → `true arm never taken`, `body never entered`, `exit never taken`, `case 2 never matched`,
  `operand 2 (b) never evaluated` · branch-count → `1 of 4 branches missed` · block →
  `block 13-15 never executed` · region → `region 13:5-15:6 never executed` · form → `2 of 5 forms not covered`.
- `Header` = the decision's first line up to `:`/`{`, trimmed, at most 60 runes, then `…`.

**`crap --branches`**. The CRAP report and crap.edn are unchanged. Then on stdout:
```

Untested branches (python branch, go block, java branch-count)
billing/pricing.py:16 if total <= 0 · true arm never taken
billing/pricing.py:21 for item in items · body never entered
calc/calc.go:12 if x > 0 · block 13-15 never executed
No branch data: rust (tarpaulin reports lines only)
```
Lines sort by (path, line, column), restricted to the selected files (`--changed` too), and are written to
`.metrics/branches.edn`. `--no-coverage --branches` is a usage error (`--no-coverage cannot be combined with
--branches`). Exit codes are unchanged (missing branch data is not an error).

**`mutate --branches`**. An UNCOVERED site line gets a reason: ` · in the true arm of :16 (if total <= 0),
never taken` when the site lies in an untested arm, ` · line never executed` otherwise, and ` · no coverage
data for file` when the lookup is None. Nothing else changes, and `--scan` is not affected.

## 7. `dry --tests` (Track B)

The selection walk does not prune test dirs and does not apply the test-file rule (a new
`discover.IterFilesWith(roots, keep, includeTests)`, in a new file). `--changed` keeps changed test files.
The output format, threshold, pairing and `.metrics/dry.edn` shape are unchanged. Test spans simply appear
as `path:start-end`. Without the flag, the bytes match upstream.

## 8. Build plan

**Step 0: seams (one small PR, before the tracks fork, harness 36/36):**
1. Move `testPlan` and its helpers from `internal/mutate/runner.go` into `internal/testplan` (a pure move;
   mutate calls it; delete the dead originals).
2. Add `Attribute`, `Branches` to `mutate.Options` and `ParseArgs` (hidden), wire them into `fileOptions`
   as no-ops, and change `formatSiteLog(sites, status, annotate func(Site, string) string)` (nil keeps
   today's bytes).
3. Add `writeResults(..., killedBy map[string][]string, attributed bool)`, still writing nothing new.
4. Export any `internal/coverage` helper Track A needs (`PythonExecutable`, `PythonSources`).
5. Golden test: a snapshot without `:killed-by` round-trips byte-identically.

**Track A**, which owns `internal/testresult`, `internal/testrun`, `internal/testreport`,
`internal/mutate/attribute*.go`, the `:killed-by` code in `internal/mutate/metrics.go`, and the `test` and
`tests` entries in `cmd/coderefinery/main.go`:
- A1 testresult parsers + ID rules, with goldens recorded from real reporters per tool.
- A2 `test`.
- A3 `mutate --attribute`: injection, worker reports, sentinels, re-selection, Python narrowing.
- A4 `tests`: Matrix, cover, levels, text/EDN, `--changed`, exits.
Each of A2 and A3 goes Python → Go → TS/JS → Java → Rust → Clojure.

**Track B**, which owns `internal/coverage/branch_*.go`, `internal/crap/decisions*.go`, the `--branches`
code in `internal/crap/cli.go`, `internal/branch`, `internal/mutate/branches.go` (the annotator), the
`internal/dry` `--tests` code, and the new `internal/discover` function:
- B1 Decisions.
- B2 loaders.
- B3 mapping + `crap --branches` + branches.edn.
- B4 `mutate --branches`.
- B5 `dry --tests`.
B1–B4 go Python → Go → TS/JS → Java → Rust → Clojure.

**Shared seams and conflict rules.**
- `internal/mutate`: after step 0 each track edits only its own new files, plus A's hunk in metrics.go and
  each track's single call site in engine.go/workers.go, which sit in different functions.
- `internal/coverage`: B adds new files only. A only reads.
- The §5 paths and file names are the A→B contract: A writes them, B reads them. B tests against checked-in
  sample reports and never waits for A.
- Docs: both tracks append notes to their PR descriptions, and Track C writes the docs.
- Gate for every PR: `make ci`, then `harness/run.sh check` = 36/36.
- Builder rules: toolchains and plugins for fixtures install only under `.local/` (`harness/env.sh`). Never
  `rm -rf` a variable. No commits without the owner.

**Track C, after A and B:**
- SKILL.md, additive. The Quality Gates order becomes crap → dry (then `dry --tests --threshold 0.8`:
  merge into a parametrized test) → `mutate --changed --use-existing-coverage --attribute` → `tests`.
  Actions per row: `(no kills…)` → delete the test, or make it assert behaviour. `Shadowed by X` → delete
  it, unless it is the only test at its level that documents the behaviour. Keep-lower recommendation →
  move the case down a level. Untested-branch line → write the test for that arm. Exit 2 → fix the tests
  first.
- README: commands table rows and usage lines.
- ARCHITECTURE: package rows.
- DEVIATIONS: a new "Additions (opt-in)" table listing every flag and file above.

## 9. Owner decisions (were the open questions)

1. **Decided: yes.** Exit 3 counts zero-kill tests and **shadowed** tests (zero unique and not in the covering
   set). A zero-unique test that the cover needs is reported but is not an offender.
2. **Decided: hidden.** `--attribute` / `--branches` (mutate), `--branches` (crap), `--tests` (dry) stay out of
   the upstream `--help`, which stays byte-identical. The new `test` and `tests` subcommands are listed in the
   top-level `coderefinery` usage.
3. **Decided: per-case by default.** `tests` rows are cases (`test_tier_rate[gold]`); `--group` collapses them
   to parametrized groups (`:rows "test"`). `--cases` does not exist.
4. **Decided: nextest is acceptable** for Rust attribution (no doctests; the generated `--config-file` replaces
   the project's nextest.toml). Plain `cargo test` gets the one-line `install cargo-nextest` hint.
5. **Decided: no TS/JS narrowing in v1.** TS/JS mutants run the full suite.
6. **Decided: `test --repeat N` is enough** for flake detection in v1.
