# Upstream reference tools

Installed 2026-11-03, each into its own venv exactly where its wrapper script
expects it (`upstream/<tool>/.venv`, `pip install -e '.[dev]'`), with
`harness/env.sh` sourced so pip's cache lives in `.local/cache/pip`.

| Tool    | Commit                                     | Date                      | Version |
|---------|--------------------------------------------|---------------------------|---------|
| crapper | `6fd92c4c9bcc56efb64a2f8743d5a2993bcfd42d` | 2026-10-21 09:16:07 -0500 | 0.1.0   |
| dryer   | `66ff6d21a42c04afcad89c78a80066176d1294b0` | 2026-10-03 18:03:53 -0500 | 0.1.0   |
| mutator | `a5a0cb803b66e0cdb8bf471b3b08af701d424a41` | 2026-10-21 14:33:45 -0500 | 0.1.0   |

Python deps resolved: tree-sitter 0.26.0, tree-sitter-language-pack 1.23.1,
pytest 9.2.0 (Python 3.12.3). mutator imports crapper from the sibling
checkout `upstream/crapper/src` (see `mutator/crapper_link.py`); crapper is not
installed into mutator's venv.

Toolchain used for fixtures and golden runs: Go 1.26.1 (asdf), Node 24.13.0,
npm 11.6.2, vitest 3.2.7 + @vitest/coverage-v8 3.2.7, c8 10.1.3, coverage.py
7.17.1.

## Upstream test suites (`.venv/bin/pytest -q`, logs in `harness/logs/pytest-*.log`)

| Tool    | Passed | Failed | Notes |
|---------|--------|--------|-------|
| crapper | 145    | 1      | `tests/test_cli.py::test_project_snapshot_is_readable_as_crap4clj_edn` needs `bb` (Babashka) on PATH to read the EDN back. Environment only, not a code failure. Babashka was not installed (no global installs allowed). |
| dryer   | 80     | 0      | |
| mutator | 76     | 0      | 27 s; runs real pytest / node / go subprocesses. |

## Behaviour worth knowing (found while building fixtures and golden files)

These are upstream behaviours the golden files capture as-is. coderefinery has to
match them to pass the harness, or the spec has to choose to diverge on purpose.

1. **ECMAScript `#private` methods are not entries.** crapper drops `#record()`
   (typescript fixture) and `#compact()` (javascript fixture) entirely, and
   mutator therefore reports no sites in them. TypeScript `private restock()` is
   an entry. mutator's README says a `#` name gets `defn-/`, which can never
   happen because the function list comes from crapper.
2. **dryer ignores `.js/.jsx/.mjs/.cjs`.** Its `discover.py` docstring says the
   extensions match crapper and mutator, but its table only has
   `.ts/.tsx/.mts/.cts`. On the javascript fixture dryer prints
   `No source files to analyze.`, exits 0 and writes no `.metrics/dry.edn`.
   The `dry/javascript/*` cases pin that.
3. **mutator never finds Go coverage when the module path is not a suffix of
   the directory path.** `go test -coverprofile` writes keys like
   `example.com/ubfix/calc.go`. crapper maps them through `bind_sources`, but
   mutator's `_lookup` only matches by path suffix. Every site comes out
   `uncovered`, with the stderr line `No coverage data for calc.go; ...`, and
   the run exits 0 (`mutate/go/full`). This hits any normal module checked out
   outside GOPATH. `mutate/go/no-coverage` runs the Go mutants for real.
4. **mutator repeats `Wrote <file>` lines.** Every file it processes re-lists
   every snapshot written so far (in `mutate/typescript/full` stderr,
   `badge.edn` appears 6 times). Cosmetic, on stderr only.
5. **Coverage-tool output goes to the tool's own stdout.** crapper's (and
   mutator's) coverage commands inherit stdio, so pytest / vitest / node --test /
   go test output comes before the report on stdout, and the `+ (cwd) cmd`
   lines go to stderr. A byte-compatible coderefinery has to run the same commands
   with inherited stdio.
6. **Python interpreter choice differs between the tools.** crapper runs
   `<project>/.venv/bin/python`, otherwise `python3` from PATH, and
   `pip install`s coverage/pytest into it when they are missing. With no venv
   that is a global install. mutator runs `<project>/.venv/bin/python`,
   otherwise its *own* `sys.executable` (mutator's venv). The python fixture
   ships a `.venv` symlink to `.local/venvs/fixture-python` so both tools use
   the same interpreter, and `env.sh` sets `PIP_REQUIRE_VIRTUALENV=1` as a
   backstop.
7. **The Vitest config is analysed as source.** `vitest.config.ts` shows up in
   `mutator --scan` (`0 mutation sites`). It contributes no crap entries
   because it has no functions.

## Nondeterminism (what the harness masks, and what it cannot)

* `:tested-at` in every `.metrics/mutate/*.edn` is wall-clock time (with
  `TZ=UTC` from env.sh). Semantic comparison ignores it (`-ignore-keys`), and
  "raw equal" is judged after masking it.
* Test-runner durations and clock times on stdout: vitest `Start at` /
  `Duration`, node `(0.37ms)` / `duration_ms`, go `ok pkg 0.865s`, pytest
  `in 0.02s`. These are masked to `<DUR>` / `<TIME>`. `-no-normalize` shows them.
* Absolute scratch paths (in stderr `+ (<root>) ...`, `Wrote <root>/...`,
  pytest `rootdir:`, vitest `RUN v3.2.7 <root>`) are masked to `<ROOT>`.
* **mutator timeouts depend on load.** A mutant's timeout is
  `max(2 s, 10 × baseline)`. A Go mutant has to recompile, and when two
  mutators share the CPU (`-live`) it can exceed that and count as *killed*.
  Seen once: `geometry/shapes.go:13 <= -> <` in `mutate/go/no-coverage`. Golden
  mode and `-live -serial` were stable on every run.
