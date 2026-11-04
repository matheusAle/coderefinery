# Deviations from upstream

Default policy: byte-for-byte parity with the upstream Python tools, quirks
included (see ARCHITECTURE.md). Every known difference is listed here. Add
an entry whenever you introduce one.

## All tools

| # | upstream | coderefinery | why |
|---|---|---|---|
| A1 | Uncaught exceptions print a Python traceback, exit 1 (malformed Go profile or JaCoCo XML, unreadable files, missing `git`). | One line `crapper: <message>` on stderr, exit 1. | Traceback parity is not a goal (spec-crapper §1.3). |
| A2 | tree-sitter always returns a tree, recovering from syntax errors with ERROR/MISSING nodes. | The hand parsers return an error for input they cannot parse; the file then contributes no functions (crap) / candidates / sites. | ERROR-recovery emulation is out of scope for now. Python parity on valid code: see PARSERS.md. |
| A3 | Coverage globs (`target/coverage/**/lcov.info`, `*/target/...`) come back in filesystem order. | Each glob's matches are sorted lexically. | Determinism; only affects precedence between duplicate report keys (spec-crapper §5.4). |

## Python parser (affects all tools on Python files)

| # | case | upstream tree | coderefinery |
|---|---|---|---|
| P1 | `print(…, *a.b())` (a splat inside a call to the soft keyword `print`) | `call(function: list_splat(attribute))` (GLR artefact) | `list_splat(call(…))`, as for every other callee |
| P2 | Files tree-sitter cannot parse cleanly (BOM at offset 0, Python 2 backticks, comments inside f-string interpolations, invalid code) | partial tree with ERROR nodes | parse error (A2) |

## Java parser (affects all tools on Java files)

| # | case | upstream tree | coderefinery |
|---|---|---|---|
| J1 | Files tree-sitter cannot parse cleanly (invalid code, `0_7`/`0x_1` literals, annotations before `...`) | partial tree with ERROR nodes | parse error (A2) |
| J2 | `(X) + y` / `(X) - y` with a reference type `X` (a parenthesized name is also a valid type) | GLR keeps both the binary and the cast reading and picks one by merge timing / subtree order | rule reproducing the observed picks: binary unless `y` is followed by `*`, `/` or `%` and `(X)` is not the right operand of `+`/`-` (`internal/lang/java/expr.go`, `ambiguousSignCast`). Identical on the JDK 21 sources and `testdata/java/cases/cast_ambiguity.java`; exotic shapes (e.g. a cast as the left operand of the enclosing chain) may still differ |

## Go parser (affects all tools on Go files)

| # | case | upstream tree | coderefinery |
|---|---|---|---|
| G1 | Files tree-sitter cannot parse cleanly (invalid Go, a leading BOM, `make(n - 1)`/`new(f(x))` where tree-sitter-go needs a type) | partial tree with ERROR/MISSING nodes | parse error (A2) |
| G2 | `x = m[k](y)` (an ambiguous `X[i](arg)`) as the last tokens of a file without a trailing newline | `call_expression(index_expression)` (GLR picks differently at EOF) | `type_conversion_expression`, as everywhere else in a file |

## Rust parser (affects all tools on Rust files)

| # | case | upstream tree | coderefinery |
|---|---|---|---|
| R1 | Files tree-sitter-rust itself cannot parse cleanly: `try!(…)` (`try` is a keyword), `$ x` (space after `$`) in `macro_rules!`, `~` in token trees, `\u{4_e}` escapes, `str![…]`, `Option<raw>`, attributes inside struct patterns, negative const arguments `T<-5>`, `S::<T> { .. }` patterns, `where (): Tr`, a bare top-level expression | partial tree with ERROR/MISSING nodes | the construct parsed as Rust means it (a `~` token, a macro, a `negative_literal`, ...) where that is unambiguous, otherwise a parse error (A2); crapper complexity can differ only inside such files |

## TypeScript / TSX / JavaScript parser (affects all tools on those files)

| # | case | upstream tree | coderefinery |
|---|---|---|---|
| T1 | Files tree-sitter cannot parse cleanly: `export type * from "m"`, a bare `&` in JSX text (`<a>Docs & Guides</a>`), `a ?.5 : 1`, `useState<import("m").T[]>([])` | partial tree with ERROR/MISSING nodes | parse error (A2), except `f<typeof import("m")>()` / `f<import("m").T>()` where the comparison reading fails: reparsed as a typed call |
| T2 | Trailing same-line comment after a non-empty block in a `.js` file in a context not probed (the JS scanner's block rule is reproduced from observed cases only) | comment inside or after the block | rule of `asi.go`; only the comment's parent can differ, never a node the tools count |
| T3 | An incomplete `\u` / `\x` escape in a string or template: `"\u004"`, `"\x4"`, `` `\u004` ``, `"\u{}"` (found by the compaction tests of `escapeEnd`) | the escape does not match tree-sitter's `escape_sequence` token: `string(ERROR (UNEXPECTED '"'))` (`template_string(ERROR ...)`) | accepted: `escape_sequence` `\u` / `\x` (the backslash and the letter), the rest a `string_fragment` (`escapeEnd`, `internal/lang/typescript/lexer.go`). Only the literal's children differ; the file parses where tree-sitter recovers with ERROR (the reverse of A2). Repro: `go run ./tools/treediff` on a file holding `x = "\u004";` |

## crapper

| # | upstream | coderefinery | why |
|---|---|---|---|
| C1 | `Coverage command failed to start: {OSError}` uses Python's exception text. | Emulated as `[Errno 2] No such file or directory: '<argv0 or cwd>'` / `[Errno 13] Permission denied: '<argv0>'`; other errors print Go's text. | Spec-crapper §5.2. |
| C3 | golang `package_namespace` reads the nearest go.mod with strict UTF-8; invalid UTF-8 raises a traceback. | The reader error makes that Go file contribute no entries (like a parse error). | `AnalyzeFiles` treats reader errors as A2; a go.mod that is not UTF-8 is not a valid go.mod. |
| C2 | — | All readers are 1:1 ports: Clojure (`lang_clojure.go`), Java (`lang_java.go`), Go (`lang_go.go`), Rust (`lang_rust.go`), TypeScript/TSX/JavaScript (`lang_typescript.go`). | No deviation left (row kept for numbering). |
| C4 | rust `_crate_name` reads each Cargo.toml with strict UTF-8; invalid UTF-8 raises a traceback. | The reader error makes that Rust file contribute no entries (like C3). | Same handling as C3. |

## dryer

| # | upstream | coderefinery | why |
|---|---|---|---|
| D1 | A comment as the last named child before a call's argument list (`foo /* c */ (x)`) appends `None` and crashes the run in `pr()` with a traceback (spec-dryer §4.6, Q5). | The comment callee is skipped; the call keeps its other parts. | Spec recommendation; no test depends on the crash. |
| D2 | Unreadable files and a missing `git` raise a traceback, exit 1. | One line `dryer: <message>` on stderr, exit 1. | A1. |
| D3 | Every tree-sitter language always yields candidates. | A language whose hand parser is not registered yet, or that fails to parse a file, contributes no candidates from it (A2), silently. | Parsers land per language; see PARSERS.md for status. |

## mutator

| # | upstream | coderefinery | why |
|---|---|---|---|
| M1 | Go coverprofile keys are import paths (`example.com/ubfix/calc.go`). `_lookup` matches them by path suffix only, so for a module whose path does not mirror its directories every Go site is `UNCOVERED` (`No coverage data for ...`) and nothing runs without `--no-coverage` (spec-mutator §4.2, harness UPSTREAM.md #3). | Reproduced by default. The opt-in flag `--fix-go-coverage` first maps the file to `<module path from the nearest go.mod>/<path below it>` and uses that profile key when present, then falls back to the upstream lookup. The flag is not listed in `--help`, which stays byte-identical. | Parity by default; the fix is behind a flag. |
| M2 | Python tracebacks (unsafe namespace, a non-UTF-8 file under `--scan`, malformed Go profile, a snapshot that has kept outcomes but no kept forms, a kept form holding a `#{}` set). | One line `mutator: <message>` on stderr, exit 1. Output already printed stays. | A1. |
| M3 | With no project `.venv`/`venv`, Python tests run with mutator's own `sys.executable`. | `python3` from `PATH`, as an absolute path. | coderefinery has no interpreter of its own (spec-mutator §5.1). |
| M4 | Files tree-sitter parses with ERROR recovery still yield sites (and crapper functions). | A file the hand parser rejects has no functions and no sites (A2): its forms disappear from the snapshot on the next write. | A2. |
| M5 | The Clojure tree comes from tree-sitter-clojure. | A small reader in `internal/mutate/clojure.go` emits the node types the site rules read (`list_lit`, `sym_lit` with its metadata, `bool_lit`, `num_lit`, quoting/comment/string nodes, `dis_expr`, ...). It matches upstream on the probes in `testdata/raw_sites.jsonl`. It does not reproduce tree-sitter's ERROR recovery on malformed input. | No tree-sitter. |
| M6 | EDN integers are unbounded Python ints. | An integer outside int64 makes that snapshot unreadable (skipped, like any parse error). | Never written by the tools. |
| M7 | `json.loads` of an outcome key accepts `NaN`/`Infinity` and lone surrogates. | Decoded with `encoding/json`: such keys count as "not a mutation id". | Never written by the tools. |
| M8 | The worker overlay never links a child named in `SKIP_LINK` (`workers.py:26-46`), at every depth. A package directory named `coverage`, `build`, `out`, `dist` or `target` (Go `internal/coverage`, a Python `build` package, a Java `...build` package) is missing from the worker, so every mutant in a package that imports it fails to build and is reported KILLED (`· by <build>` with `--attribute`; self-audit C1). | Reproduced by default. The opt-in flag `--fix-overlay` links such a directory when it directly holds a source file of the mutated file's language, which build output and coverage reports do not. `target` at the project root (the worker home) is never linked. TypeScript/JavaScript projects keep the upstream skip list, since `dist/` and HTML coverage reports hold `.js` files there (their overlay fix is M9); a directory holding only subpackages also stays hidden. `--attribute` turns the fix on: its output is already coderefinery's own, and a kill credited to `<build>` because of the overlay is wrong data for `coderefinery tests`. The flag is not listed in `--help`. Test: `internal/mutate/overlay_test.go` on `internal/mutate/testdata/overlay-coverage`. | Parity by default; the fix is behind a flag, and on in the attribution flow. |
| M9 | The worker overlay links every file that is not the mutated file or a relative importer of it (`workers.py`), with the link pointing at the real path. Vite resolves a module to its real path, and Vitest requests each test file and setup file by path: a linked one resolves outside the worker. Under `environment: "jsdom"` (Vite's client environment) that path is checked against `server.fs.allow` (the config's directory, the workspace root found from the worker, which is the worker since it holds a copy of `package.json`, and Vitest's package), the load fails with `Failed to load url ... Does the file exist?`, every run exits 1 and **every mutant is reported KILLED**. Not affected: modules reached through imports (Vite allows every id it resolved for an import, `node_modules` included), the `node` environment (no fs check), a project whose root or an ancestor is a workspace root (`workspaces` in package.json, `pnpm-workspace.yaml`, `lerna.json`). Jest's crawler skips symlinks: a linked test file does not run, so a test that reaches the mutated file through a non-relative import (`moduleDirectories`, an alias) never kills it and the mutant is reported SURVIVED; Jest's jsdom environment adds nothing to that. | Reproduced by default. With `--fix-overlay` (and so `--attribute`) on a TypeScript/JavaScript file, after the upstream overlay is built, every linked directory inside the project becomes a real directory and every linked `.ts .tsx .mts .cts .js .jsx .mjs .cjs` file a copy, so their real paths stay in the worker. Other files stay links (imports reach them), so do `node_modules` and dot-directories (`.next`, `.turbo`, `.yarn`: caches; a setup file under one, e.g. `.storybook/`, still fails under jsdom), and links that leave the project. The project's files and config are not touched: no `server.fs` override is needed. Cost: one copy of the project's JS/TS sources per worker, per mutated file. Tests: `internal/mutate/overlay_node_test.go` on `testdata/overlay-jsdom` (Vitest 3 + jsdom, `node_modules` from `fixtures/react`: all 3 mutants KILLED without the flag, the `>`→`>=` survivor survives with it) and `testdata/overlay-jest` (Jest 30, `fixtures/js-jest-tests`: the boundary mutant SURVIVED without, KILLED with). | Parity by default; the fix is behind the M8 flag. |

## Additions (opt-in)

Not upstream behaviour: none of these exist in crapper, dryer or mutator.
Each is opt-in. The hidden flags are left out of the upstream `--help` text,
which stays byte-identical (the `mutate --fix-go-coverage` precedent, M1);
`test`, `tests` and `react` are listed only in the top-level `coderefinery --help`.
Without them, crap, dry and mutate keep their stdout, stderr, exit codes and
`.metrics` bytes (harness 36/36). Contract: `docs/design-tests.md` (X1-X6),
`docs/design-react.md` (X7-X10).

| # | addition | kind | what it does | writes |
|---|---|---|---|---|
| X1 | `coderefinery test` (`--root`, `--test-command`, `--no-coverage`, `--repeat <n>`, `--changed`, `--verbose`) | new subcommand | runs each project's tests once with a JUnit reporter (`go test -json` for Go) and branch coverage; exit 0 pass · 1 usage / missing tool · 2 failures | `.metrics/tests/runs.edn`, `.metrics/tests/<lang>[/<module>]/results.xml` or `results.json`, `coverage/`; `.metrics/tests/command/` for `--test-command` |
| X2 | `coderefinery tests` (`--root`, `--group`, `--changed`) | new subcommand | test relevance: kills, unique kills, covering set, shadowed and zero-kill tests, levels; exit 0 clean · 1 usage / no attribution data · 2 failing results · 3 offenders | `.metrics/tests.edn` |
| X3 | `mutate --attribute` | hidden flag | runs each mutant with a JUnit reporter and credits the kill to the failing tests (`<timeout>` / `<build>` when none); KILLED lines get ` · by <id>`; exit 2 adds `failing tests: <ids>` | `:killed-by`, a new last key in `.metrics/mutate/<ns>.edn` (only when non-empty; upstream ignores it) |
| X4 | `mutate --branches` | hidden flag | UNCOVERED lines get a reason (` · in the <arm> arm of :<line> (<decision>), never taken`, ` · line never executed`, ` · no coverage data for file`) | nothing new |
| X5 | `crap --branches` | hidden flag | adds an `Untested branches` section after the report; the report, `crap.edn` and exit codes are unchanged; `--no-coverage --branches` is a usage error | `.metrics/branches.edn` |
| X6 | `dry --tests` | hidden flag | also compares test files (test directories and test file names are not skipped); the report format is unchanged | nothing new (`.metrics/dry.edn` as usual) |
| X7 | `coderefinery react` (`--root`, `--changed`, `--no-coverage`, `--use-existing-coverage`, `--coverage-command`, `--crap <n>`, `--max-<metric> <n\|off>`) | new subcommand | React function components and custom hooks: each hook callback (`Cart#useEffect#1`) and handler (`Cart.onSubmit`) is its own CRAP entry, not charged to the component (the Express-route rule); render vs logic complexity; props, hooks and render-cost metrics; exit 0 clean · 1 usage · 2 coverage run failed · 3 a threshold exceeded. Contract: `docs/design-react.md` | `.metrics/react.edn` |
| X8 | `react --profile` | flag of an addition | also runs the Vitest (else Jest) tests with a render profiler (the React DevTools global hook, installed by a generated setup file; the project is not edited) and appends `Runtime renders` (renders, mounts, updates, actualDuration total/max per component; components with updates >= 3x mounts and inline props flagged) to the report; without the flag `react`'s output is unchanged; exit 1 when the run cannot be instrumented, 2 when it fails. Contract: `docs/design-react.md` §11 | `:renders` in `.metrics/react.edn`; generated files and `profile-*.json` under `.metrics/react-profile/` |
| X9 | `dry --jsx` (`--jsx-threshold N`, `--jsx-min-elements N`, `--jsx-min-nodes N`; each implies `--jsx`) | hidden flag | also pairs JSX element subtrees of `.tsx`/`.jsx`/`.js` files (tag and attribute names kept; text, attribute values, `key` and SVG-only subtrees dropped), with dryer's fingerprints and Jaccard; the largest enclosing pair is reported. A `Duplicate JSX` section follows the unchanged function report (with `--edn`, a second `{:jsx-candidates [...]}` map); `dry.edn` is byte-identical with or without the flag. Contract: `docs/design-react.md` §12 | `.metrics/dry-jsx.edn` |
| X10 | `mutate --react` | hidden flag | adds React operators to the usual sites in TS/TSX/JS files: `drop render` / `always render` (`cond && <X/>` → `false` / `<X/>`), `swap ternary` (JSX branches), `drop prop` (one JSX attribute; not `key`, `className`, `style`, handlers, spreads), `noop onX` (`onX={() => {}}`), `drop dep` (one React hook dependency); site log and scan print the operator label (`noop onClick`); sites keep crapper's form ids and the upstream mutation-id shape. Contract: `docs/design-react.md` §13 | nothing new (React outcomes in `.metrics/mutate/<ns>.edn` as usual) |

### mutate --min-timeout (test stability)

A mutant's timeout is `max(2 s, --timeout-factor × baseline)`, with the
baseline measured once, in the real tree, often warm or cached. Under a load
spike a mutant run in a fresh overlay can outlast it and is reported KILLED
(or `· by <timeout>` with `--attribute`) although it survives on an idle
machine. `--min-timeout <seconds>` replaces the fixed 2 s floor; the default
(2) is upstream's. It only decides when a mutant is cut off, so stdout,
stderr, exit codes and `.metrics` are those of an untimed run. It is hidden
from `--help`. The e2e tests in `cmd/coderefinery` and the harness
(`-ours-mutate-args`, harness/README.md "Mutant timeouts") pass
`--min-timeout 300`, so a real hang still ends, after 300 s. A larger
`--timeout-factor` alone is not enough: it multiplies a baseline that can be a
cached `go test` run of a few milliseconds.
