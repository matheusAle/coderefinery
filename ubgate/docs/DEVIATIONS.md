# Deviations from upstream

Default policy: byte-for-byte parity with the upstream Python tools, quirks
included (see ARCHITECTURE.md). Every known difference is listed here. Add
an entry whenever you introduce one.

## All tools

| # | upstream | ubgate | why |
|---|---|---|---|
| A1 | Uncaught exceptions print a Python traceback, exit 1 (malformed Go profile or JaCoCo XML, unreadable files, missing `git`). | One line `crapper: <message>` on stderr, exit 1. | Traceback parity is not a goal (spec-crapper §1.3). |
| A2 | tree-sitter always returns a tree, recovering from syntax errors with ERROR/MISSING nodes. | The hand parsers return an error for input they cannot parse; the file then contributes no functions (crap) / candidates / sites. | ERROR-recovery emulation is out of scope for now. Python parity on valid code: see PARSERS.md. |
| A3 | Coverage globs (`target/coverage/**/lcov.info`, `*/target/...`) come back in filesystem order. | Each glob's matches are sorted lexically. | Determinism; only affects precedence between duplicate report keys (spec-crapper §5.4). |

## Python parser (affects all tools on Python files)

| # | case | upstream tree | ubgate |
|---|---|---|---|
| P1 | `print(…, *a.b())` (a splat inside a call to the soft keyword `print`) | `call(function: list_splat(attribute))` (GLR artefact) | `list_splat(call(…))`, as for every other callee |
| P2 | Files tree-sitter cannot parse cleanly (BOM at offset 0, Python 2 backticks, comments inside f-string interpolations, invalid code) | partial tree with ERROR nodes | parse error (A2) |

## Java parser (affects all tools on Java files)

| # | case | upstream tree | ubgate |
|---|---|---|---|
| J1 | Files tree-sitter cannot parse cleanly (invalid code, `0_7`/`0x_1` literals, annotations before `...`) | partial tree with ERROR nodes | parse error (A2) |
| J2 | `(X) + y` / `(X) - y` with a reference type `X` (a parenthesized name is also a valid type) | GLR keeps both the binary and the cast reading and picks one by merge timing / subtree order | rule reproducing the observed picks: binary unless `y` is followed by `*`, `/` or `%` and `(X)` is not the right operand of `+`/`-` (`internal/lang/java/expr.go`, `ambiguousSignCast`). Identical on the JDK 21 sources and `testdata/java/cases/cast_ambiguity.java`; exotic shapes (e.g. a cast as the left operand of the enclosing chain) may still differ |

## Go parser (affects all tools on Go files)

| # | case | upstream tree | ubgate |
|---|---|---|---|
| G1 | Files tree-sitter cannot parse cleanly (invalid Go, a leading BOM, `make(n - 1)`/`new(f(x))` where tree-sitter-go needs a type) | partial tree with ERROR/MISSING nodes | parse error (A2) |
| G2 | `x = m[k](y)` (an ambiguous `X[i](arg)`) as the last tokens of a file without a trailing newline | `call_expression(index_expression)` (GLR picks differently at EOF) | `type_conversion_expression`, as everywhere else in a file |

## Rust parser (affects all tools on Rust files)

| # | case | upstream tree | ubgate |
|---|---|---|---|
| R1 | Files tree-sitter-rust itself cannot parse cleanly: `try!(…)` (`try` is a keyword), `$ x` (space after `$`) in `macro_rules!`, `~` in token trees, `\u{4_e}` escapes, `str![…]`, `Option<raw>`, attributes inside struct patterns, negative const arguments `T<-5>`, `S::<T> { .. }` patterns, `where (): Tr`, a bare top-level expression | partial tree with ERROR/MISSING nodes | the construct parsed as Rust means it (a `~` token, a macro, a `negative_literal`, ...) where that is unambiguous, otherwise a parse error (A2); crapper complexity can differ only inside such files |

## TypeScript / TSX / JavaScript parser (affects all tools on those files)

| # | case | upstream tree | ubgate |
|---|---|---|---|
| T1 | Files tree-sitter cannot parse cleanly: `export type * from "m"`, a bare `&` in JSX text (`<a>Docs & Guides</a>`), `a ?.5 : 1`, `useState<import("m").T[]>([])` | partial tree with ERROR/MISSING nodes | parse error (A2), except `f<typeof import("m")>()` / `f<import("m").T>()` where the comparison reading fails: reparsed as a typed call |
| T2 | Trailing same-line comment after a non-empty block in a `.js` file in a context not probed (the JS scanner's block rule is reproduced from observed cases only) | comment inside or after the block | rule of `asi.go`; only the comment's parent can differ, never a node the tools count |

## crapper

| # | upstream | ubgate | why |
|---|---|---|---|
| C1 | `Coverage command failed to start: {OSError}` uses Python's exception text. | Emulated as `[Errno 2] No such file or directory: '<argv0 or cwd>'` / `[Errno 13] Permission denied: '<argv0>'`; other errors print Go's text. | Spec-crapper §5.2. |
| C3 | golang `package_namespace` reads the nearest go.mod with strict UTF-8; invalid UTF-8 raises a traceback. | The reader error makes that Go file contribute no entries (like a parse error). | `AnalyzeFiles` treats reader errors as A2; a go.mod that is not UTF-8 is not a valid go.mod. |
| C2 | — | All readers are 1:1 ports: Clojure (`lang_clojure.go`), Java (`lang_java.go`), Go (`lang_go.go`), Rust (`lang_rust.go`), TypeScript/TSX/JavaScript (`lang_typescript.go`). | No deviation left (row kept for numbering). |
| C4 | rust `_crate_name` reads each Cargo.toml with strict UTF-8; invalid UTF-8 raises a traceback. | The reader error makes that Rust file contribute no entries (like C3). | Same handling as C3. |

## dryer

| # | upstream | ubgate | why |
|---|---|---|---|
| D1 | A comment as the last named child before a call's argument list (`foo /* c */ (x)`) appends `None` and crashes the run in `pr()` with a traceback (spec-dryer §4.6, Q5). | The comment callee is skipped; the call keeps its other parts. | Spec recommendation; no test depends on the crash. |
| D2 | Unreadable files and a missing `git` raise a traceback, exit 1. | One line `dryer: <message>` on stderr, exit 1. | A1. |
| D3 | Every tree-sitter language always yields candidates. | A language whose hand parser is not registered yet, or that fails to parse a file, contributes no candidates from it (A2), silently. | Parsers land per language; see PARSERS.md for status. |

## mutator

| # | upstream | ubgate | why |
|---|---|---|---|
| M1 | Go coverprofile keys are import paths (`example.com/ubfix/calc.go`). `_lookup` matches them by path suffix only, so for a module whose path does not mirror its directories every Go site is `UNCOVERED` (`No coverage data for ...`) and nothing runs without `--no-coverage` (spec-mutator §4.2, harness UPSTREAM.md #3). | Reproduced by default. The opt-in flag `--fix-go-coverage` first maps the file to `<module path from the nearest go.mod>/<path below it>` and uses that profile key when present, then falls back to the upstream lookup. The flag is not listed in `--help`, which stays byte-identical. | Parity by default; the fix is behind a flag. |
| M2 | Python tracebacks (unsafe namespace, a non-UTF-8 file under `--scan`, malformed Go profile, a snapshot that has kept outcomes but no kept forms, a kept form holding a `#{}` set). | One line `mutator: <message>` on stderr, exit 1. Output already printed stays. | A1. |
| M3 | With no project `.venv`/`venv`, Python tests run with mutator's own `sys.executable`. | `python3` from `PATH`, as an absolute path. | ubgate has no interpreter of its own (spec-mutator §5.1). |
| M4 | Files tree-sitter parses with ERROR recovery still yield sites (and crapper functions). | A file the hand parser rejects has no functions and no sites (A2): its forms disappear from the snapshot on the next write. | A2. |
| M5 | The Clojure tree comes from tree-sitter-clojure. | A small reader in `internal/mutate/clojure.go` emits the node types the site rules read (`list_lit`, `sym_lit` with its metadata, `bool_lit`, `num_lit`, quoting/comment/string nodes, `dis_expr`, ...). It matches upstream on the probes in `testdata/raw_sites.jsonl`. It does not reproduce tree-sitter's ERROR recovery on malformed input. | No tree-sitter. |
| M6 | EDN integers are unbounded Python ints. | An integer outside int64 makes that snapshot unreadable (skipped, like any parse error). | Never written by the tools. |
| M7 | `json.loads` of an outcome key accepts `NaN`/`Infinity` and lone surrogates. | Decoded with `encoding/json`: such keys count as "not a mutation id". | Never written by the tools. |
