# coderefinery verification harness

This harness runs coderefinery and the upstream Python tools (crapper, dryer,
mutator) on the same fixture projects and compares what they produce: exit
code, stdout, every file under `.metrics/`, and (for information) stderr.
Everything lives inside `/Users/matheusale/workspaces/ub-mode`.

```
harness/
  env.sh            source first: isolates every cache, venv and toolchain path
  run.sh            matrix runner (wraps cmd/compare)
  matrix.txt        the variant matrix
  cmd/compare/      the comparison program (stdlib-only Go, module coderefinery/harness)
  internal/edn/     EDN parser + structural comparison
  internal/textdiff unified line diff
  golden/<tool>/<fixture>/<variant>/   recorded upstream outputs
  UPSTREAM.md       upstream commits, test-suite results, quirks, nondeterminism
  logs/             upstream pytest logs, record/selftest logs
fixtures/           pristine fixture projects (never run in place) + setup.sh
.local/             venvs, caches, GOPATH, scratch copies, go wrapper
```

## Quick start

```bash
cd /Users/matheusale/workspaces/ub-mode
fixtures/setup.sh                # once: fixture venv, .venv symlink, node_modules
harness/run.sh test              # harness unit tests
harness/run.sh selftest          # upstream vs golden: must be 36/36 PASS
harness/run.sh check             # coderefinery vs golden (bin/coderefinery)
harness/run.sh live              # coderefinery vs upstream, both running at the same time
```

`run.sh` modes:

| mode       | what runs                                                               |
|------------|-------------------------------------------------------------------------|
| `record`   | upstream on every case. Rewrites `golden/`. Do this only after changing fixtures, the matrix or upstream. |
| `check`    | ours, diffed against golden (default)                                   |
| `live`     | upstream and ours at the same time on separate fixture copies           |
| `selftest` | `-ours upstream` against golden: checks the harness and nondeterminism  |
| `selflive` | upstream against upstream, live                                         |
| `test`     | `go test ./...` for the harness                                         |

Extra arguments go to `cmd/compare`. The useful ones:

```
-tool crap,dry,mutate     -fixture python,go     -variant default,full     (filters; default all)
-ours '<template>'        default: /Users/matheusale/workspaces/ub-mode/bin/coderefinery {tool} {args}
                          'upstream' = the upstream tool itself
-upstream '<template>'    default: {upstream}/{upstream_tool}/{upstream_tool} {args}
-ours-mutate-args '...'   appended to ours on mutate cases (default --min-timeout 300; see "Mutant timeouts")
-upstream-mutate-args '...'  same for upstream (default --timeout-factor 1000)
-live [-serial]           live mode. -serial runs upstream, then ours (no CPU contention)
-float-tol 1e-4           absolute tolerance for EDN floats (default 0 = exact)
-ignore-keys tested-at    EDN map keys ignored at any depth (comma list)
-strict-stderr            a stderr mismatch fails the case (default: shown in the table only)
-show-stderr              print stderr diffs too
-max-diffs 5 -diff-lines 60   how much detail to print
-no-normalize             turn off the duration/clock masking (paths are always masked)
-keep                     keep the scratch copies under .local/scratch/<run-id>/
-list                     print the selected cases and exit
-v                        print each command
```

Template placeholders: `{tool}` (crap|dry|mutate), `{upstream_tool}`
(crapper|dryer|mutator), `{upstream}`, `{repo_root}`, `{fixture}`, `{variant}`,
and `{args}`, which must be a word of its own and expands to the case's
arguments. Examples:

```bash
harness/run.sh check -ours 'go run ./cmd/coderefinery {tool} {args}' -tool crap
harness/run.sh live -fixture go -variant full -max-diffs 20
```

The exit status is 0 when every selected case passes, 1 when any fails or
errors, and 2 on a usage error.

## What a case compares

Each case copies `fixtures/<fixture>` to a fresh scratch directory
`.local/scratch/<run-id>/<side>/<tool>__<fixture>__<variant>/<fixture>`. On
macOS the copy is an APFS clone, so node_modules costs nothing. The command
runs there with the environment from env.sh. The last path component is
always the fixture name, so tool output that includes the root's basename
matches. The pristine fixtures are never written to.

| column  | rule |
|---------|------|
| EXIT    | exit codes equal |
| STDOUT  | equal after normalisation (below) |
| STDERR  | equal after normalisation. Informational unless `-strict-stderr` |
| METRICS | every file under `.metrics/` exists on both sides, and each `.edn` is structurally equal: maps compared without regard to key order, vectors and lists by position, ints and floats are different types (`1` ≠ `1.0`), floats exact unless `-float-tol` is set, `-ignore-keys` skipped. Non-EDN files are compared byte for byte. `ok n/m` counts the equal files. |
| RAW     | byte-identical `.metrics` files out of the total, after masking only `:tested-at "..."` |
| RESULT  | PASS = EXIT, STDOUT and METRICS all ok (and STDERR with `-strict-stderr`). ERROR = something could not run, e.g. coderefinery is missing |

After the table, the first `-max-diffs` failing cases are printed: exit codes,
a unified diff of stdout, and EDN differences with paths such as
`$ :entries[0] :crap: want 32.244, got 32.2441`. A summary line comes last:

```
SUMMARY mode=golden cases=36 PASS=36 FAIL=0 ERROR=0 | metrics files=55 semantic-equal=55 raw-equal=55 | stderr-equal=36/36
```

### Normalisation (applied to both sides)

* The scratch root (plus its `/private` and symlink-resolved forms) becomes
  `<ROOT>`, and the repo root becomes `<REPO_ROOT>`.
* Test-runner timings become `<DUR>` / `<TIME>`: vitest `Start at` and
  `Duration`, node `--test` `(1.2ms)` and `duration_ms`, `go test`
  `ok pkg 0.8s` / `(cached)`, pytest ` in 0.02s`, and any other `<n>ms` or
  `<n.n>s`.
* vitest prints each test file's result line in completion order, which
  varies run to run, so each contiguous block of `✓ <file>.test.ts (...)`
  lines is sorted.
* `.metrics`: the `:tested-at` value is ignored for semantic equality and
  masked for raw equality.

`UPSTREAM.md` explains why each rule is needed. Mutator's timeouts are not
masked; they are kept out of play instead (next section).

### Mutant timeouts

A mutant's timeout is `max(2 s, 10 × baseline)`, from one baseline run. Under
heavy CPU load a slow mutant used to flip from survived to killed. Every
mutate case therefore gets extra arguments on top of its matrix row:

| side | flag | default |
|---|---|---|
| ours (`check`, `live`) | `-ours-mutate-args` | `--min-timeout 300` (hidden coderefinery flag, DEVIATIONS.md "mutate --min-timeout") |
| upstream (`record`, `live`, `selftest`, `selflive`) | `-upstream-mutate-args` | `--timeout-factor 1000` |

Neither value reaches stdout, stderr or `.metrics`: they only decide when a
run is cut off, and no fixture mutant hangs (the longest case runs under a
minute at load 170). Pass `-ours-mutate-args ''` to run the bare matrix row.
`argv.txt` records the arguments the next `record` actually used.

## Variant matrix (`matrix.txt`)

`*` means every fixture: python, typescript, javascript, go. That makes 36
cases.

| tool     | variant       | args |
|----------|---------------|------|
| crap     | `default`     | (none): runs the coverage tools |
| crap     | `no-coverage` | `--no-coverage` |
| crap     | `threshold`   | `--threshold 10`: exits 2 with `CRAP threshold exceeded` on stderr |
| dry      | `default`     | (none) |
| dry      | `loose`       | `--min-lines 3 --min-nodes 5` |
| dry      | `edn-loose`   | `--edn --min-lines 3 --min-nodes 5 --threshold 0.7` |
| mutate   | `scan`        | `--scan` (fresh copy, so no coverage report exists) |
| mutate   | `full`        | (none): coverage, baseline, every covered mutant |
| mutate   | `no-coverage` | `--no-coverage`: every site, which is the only way Go mutants run (see UPSTREAM.md #3) |

To add a case, add a line `<tool> <fixture|*> <variant> [shell-quoted args]`,
then run `harness/run.sh record -tool <tool> -variant <variant>` to record its
golden output.

## Fixtures

All four test suites pass. Each full mutator run takes under 30 s.

| fixture      | contents | tests |
|--------------|----------|-------|
| `python`     | `src/shop/` package: `Cart` class with methods and private `_discount`, a nested function, comprehensions with filters, conditional expressions, `match/case` with an or-pattern, `and`/`or` chains, `try/except` with two handlers, an untested `_grade` with `elif`/`while`, and the near-duplicates `summarize_orders`/`summarize_refunds`. `.venv` is a symlink to `.local/venvs/fixture-python` (pytest + coverage). | pytest, partial coverage |
| `typescript` | `Inventory` class with `private` and `#private` members, `??`, `?.`, a ternary, a top-level arrow function, `switch` with fallthrough, an Express-style `routes.ts` (two `GET /users` callbacks, `POST`, `DELETE /users/:id`) on a local `Router` stub (no express install needed, and the handlers are callable from tests), `badge.tsx` with JSX `&&`, and the near-duplicates in `stats.ts`. | vitest 3 with @vitest/coverage-v8 preinstalled (crapper uses vitest's own coverage) |
| `javascript` | `lib/queue.js` (CommonJS class, `#private` field and method, getter, `??`, `?.`) and `lib/format.mjs` (ESM, arrow function, `switch`, near-duplicates `totals`/`weights`) | `node --test`. crapper wraps it in `npx c8`, and c8 is a devDependency, so it never touches the network |
| `go`         | module `example.com/ubfix`: package functions, an unexported `clampIndex`, `switch` with `&&`/`\|\|` cases, near-duplicates `SumPositive`/`SumWeights`. `geometry/`: value and pointer receivers, an unexported method, `select` | `go test`, partial coverage |

`fixtures/setup.sh` recreates the dependencies: the fixture venv, the `.venv`
symlink, and `npm ci` for both node fixtures. It also removes caches (such as
`__pycache__` or the vitest results cache) that could change test order or
coverage. Re-record golden after editing any fixture.

## Environment isolation (`env.sh`)

Every script sources `harness/env.sh`. Nothing is installed or cached outside
`/Users/matheusale/workspaces/ub-mode`:

| what | where |
|------|-------|
| upstream tool venvs | `upstream/<tool>/.venv` (where the wrappers expect them) |
| fixture Python (pytest, coverage) | `.local/venvs/fixture-python`, first `python3` on PATH |
| pip cache | `PIP_CACHE_DIR=.local/cache/pip`. `PIP_REQUIRE_VIRTUALENV=1` blocks installs outside a venv, which matters because crapper pip-installs coverage into `python3` when it is missing |
| npm | `npm_config_cache=.local/cache/npm` (npx included), `npm_config_prefix=.local/npm-global` |
| Go | `GOPATH=.local/go`, `GOMODCACHE=.local/go/pkg/mod`, `GOCACHE=.local/cache/go-build`, `GOENV=.local/go/env`, `GOTOOLCHAIN=local`. `go` on PATH is `.local/bin/go`, a wrapper around asdf's Go 1.26.1 that sets `HOME=.local/home` for the go process only, because Go writes telemetry counters to `~/Library/Application Support/go` and no env var redirects that |
| other caches | `XDG_CACHE_HOME=.local/cache/xdg` |
| scratch copies | `.local/scratch/<run-id>/`, removed after each run unless `-keep` |
| determinism | `TZ=UTC`, `NO_COLOR=1`, `FORCE_COLOR=0`, `CI=1`, `LC_ALL=en_US.UTF-8` |
