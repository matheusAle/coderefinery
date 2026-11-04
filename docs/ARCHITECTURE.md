# coderefinery architecture

`coderefinery` is one stdlib-only Go binary (no third-party modules, no cgo) that
re-implements Uncle Bob's crapper, dryer and mutator:

```
coderefinery crap   [flags] [paths]   # crapper
coderefinery dry    [flags] [paths]   # dryer
coderefinery mutate [flags] [paths]   # mutator
coderefinery test   [flags] [paths]   # addition: run the tests, JUnit results + branch coverage
coderefinery tests  [flags]           # addition: test relevance from mutate --attribute kills
coderefinery react  [flags] [paths]   # addition: React component/hook metrics (docs/design-react.md)
```

Same flags, stdout, exit codes and `.metrics/` files as the Python originals.
Specs: `docs/spec-{crapper,dryer,mutator}.md`. Upstream source:
`upstream/<tool>/src`.

## Build

```
make build      # bin/coderefinery
make ci         # gofmt check + go vet + go test ./... + build
make treediff   # parser parity over the default Python corpus
```

The Makefile sources `harness/env.sh`, which keeps GOCACHE, GOPATH and
GOMODCACHE under `.local/`. Acceptance: `harness/run.sh check -tool crap -fixture python`.

## The key decision: one tree shape for all three tools

All three upstream tools walk tree-sitter CSTs. coderefinery has one hand-written
parser per language that emits a tree with the **tree-sitter shape** of the
pinned grammar (tree-sitter 0.26.0, tree-sitter-language-pack 1.23.1): same
node type names, named/anonymous flag, field names, byte offsets, row/column
points and nesting. The tools are then 1:1 ports of the Python logic over that
tree. Parser parity is proven differentially (`tools/treediff`, see
`PARSERS.md`).

## Packages

| package | upstream counterpart | role |
|---|---|---|
| `cmd/coderefinery` | the three entry points | subcommand dispatch |
| `internal/syntax` | py-tree-sitter `Node` | the shared `Node`/`Tree` types, walkers, `Dump` (parity format), `SExp` (`str(node)`), builders (`Leaf`, `Inner`, `Build`, `SetField`, `Finish`), extra placement shared by the parsers (`ExtraParent`, `InsertExtra`, `RemoveHidden`) |
| `internal/lang` | `tree_sitter_language_pack.get_parser` | registry: grammar name -> parser; `GrammarFor(path)` |
| `internal/lang/python` | tree-sitter-python | hand-written Python parser |
| `internal/lang/rust` | tree-sitter-rust | hand-written Rust parser (lazy lexer, token trees, macro_rules!) |
| `internal/source` | `Path.read_text(errors="replace")` | CPython-exact UTF-8 replace decoding + universal newlines |
| `internal/discover` | `crapper/discover.py` | languages, test-file rules, walk, `Resolve` (pathlib), `git status` changed files, `Select` (the `cli.select_files` skeleton crap, dry and mutate share, with per-tool hooks) |
| `internal/model` | `crapper/model.py` | `Function`, `Entry` |
| `internal/coverage` | `crapper/coverage.py`, `runners.py` | LCOV / Go profile / JaCoCo / Cloverage loaders, report-key matching, per-function percent, coverage runners |
| `internal/crap` | crapper | complexity readers per language, CRAP, sort, report, EDN, CLI |
| `internal/dry` | dryer | selection, per-language candidates, normalization, fingerprints, Jaccard pairing, text/EDN reports, Clojure reader (`clojure.go`) |
| `internal/mutate` | mutator | complete port: sites (tree rules + Clojure reader), coverage gating, differential selection, worker overlays, snapshots; `--fix-go-coverage` (DEVIATIONS M1) |
| `internal/edn` | the tools' EDN writers | `String`, `Number4` |
| `internal/pyjson` | `json.dumps(ensure_ascii=True)` | mutator ids |
| `internal/pyfmt` | Python float semantics | `Round`, `Fixed`, `Repr`, `ParseFloat`, `ParseInt` |
| `internal/cliutil` | the tools' argparse/`main` tails | `Value` (an option's value), `Main` (help, run, git error and `<prefix>: <error>` to exit codes) for every subcommand |
| `internal/treediff` | — | oracle runner + dump comparison (used by the tool and by tests); `Parity`, the body of every parser's `TestParityWithTreeSitter` |
| `internal/testutil` | — | stdlib-only test helpers (`WriteFile`, `ReadFile`, `Git`, `InitRepo`) any package's own tests can import |
| `tools/oracle/oracle.py` | — | upstream tree-sitter dumper (run with an upstream venv python) |
| `tools/treediff` | — | CLI over `internal/treediff` |
| `tools/dryoracle` | — | dryer parity per candidate: `dryer_oracle.py` (upstream dryer venv) vs `internal/dry` on spans, node counts and printed normalized forms; `go run ./tools/dryoracle PATH...` |

Additions (opt-in, no upstream counterpart; contract in `docs/design-tests.md`):

| package | role |
|---|---|
| `internal/testplan` | the mutator test plan (command + cwd per language, upstream `runner.py test_plan`), moved out of `internal/mutate` so `test` and `mutate` share it |
| `internal/testresult` | JUnit XML (pytest, Vitest, jest-junit, Mocha, Surefire, Gradle, nextest, Kaocha, test2junit) and `go test -json` parsed into one `Run` of `Case`s with canonical, tool-stable test IDs |
| `internal/testrun` | `coderefinery test`: runs each project once with a JUnit reporter and branch coverage, writes `.metrics/tests/` (`runs.edn`, results, coverage); the reporter injection `mutate --attribute` reuses |
| `internal/testreport` | `coderefinery tests`: kill matrix from `:killed-by`, unique kills, greedy covering set, shadowed and zero-kill tests, levels, `.metrics/tests.edn` |
| `internal/branch` | maps tool branch data onto the decisions crap counts and renders the `crap --branches` section and `.metrics/branches.edn` |
| `internal/coverage/branch*.go` | branch loaders: coverage.py arcs (branch), Istanbul JSON (branch) / V8 / LCOV `BRDA` (branch-count), JaCoCo `mb`/`cb` (branch-count), Go profile (block), llvm-cov (region, or branch with `--branch`), Cloverage (form) |
| `internal/react` | `coderefinery react` (contract in `docs/design-react.md`): function components and custom hooks from the TSX/JSX tree, hook callbacks and handlers split into their own CRAP entries, render/logic complexity, props, hooks and render-cost metrics, thresholds, report and `.metrics/react.edn` |

Inside existing packages: `internal/crap/decisions.go` (decision nodes with
their arms) and `branches.go` (`--branches`); `internal/mutate/attribute.go`
(`--attribute`, the `:killed-by` snapshot key) and `branches.go` (UNCOVERED
reasons); `internal/discover/tests.go` (`IterFilesWith`, for `dry --tests`).

Dependency direction: `cmd -> crap/dry/mutate -> coverage, discover, lang -> syntax`.
`coverage` imports `model`, `discover`, `source`; it never imports `crap`.
Mutator must reuse `crap.FunctionsInFile`, `discover.*` and
`coverage.LoadBundle/NormalizePath/ParseJacocoIndex` (upstream mutator
imports these crapper internals).

Additions: `testplan -> coverage, discover`; `testrun -> testplan, testresult,
crap, coverage`; `mutate -> testplan, testresult, testrun, branch`;
`testreport -> mutate, testresult, testrun`; `crap -> branch -> coverage`;
`react -> crap, coverage, lang`.
`testresult` imports nothing internal.

## Test-relevance data flow (additions)

```
coderefinery test          -> .metrics/tests/runs.edn
                              .metrics/tests/<lang>[/<module>]/results.{xml,json}, coverage/
coderefinery mutate --attribute
                           -> each mutant run gets a JUnit reporter; the failing
                              case IDs of a killed mutant -> :killed-by in
                              .metrics/mutate/<ns>.edn (or "<timeout>" / "<build>")
                              (Python: per-test contexts from test's coverage narrow the run)
coderefinery tests         <- :killed-by + .metrics/tests/ results (the test universe,
                              durations, flaky)
                           -> report on stdout, .metrics/tests.edn
coderefinery crap --branches
                           <- .metrics/tests/<lang>/ coverage, else the CRAP reports on disk
                           -> "Untested branches" section, .metrics/branches.edn
```

Default crap and mutate runs never read `.metrics/tests/`, so these
files cannot change default output.

## The Node contract

```go
// internal/syntax
type Point struct{ Row, Column int }          // 0-based, Column in BYTES
type Node struct {
    Type       string   // node.type: "identifier", "if", "+", "ERROR"
    Named      bool     // node.is_named
    Missing    bool     // node.is_missing
    Extra      bool     // node.is_extra (comments, line continuations)
    Field      string   // field name in the parent ("" = none)
    StartByte, EndByte int
    StartPoint, EndPoint Point
    Parent     *Node
    Children   []*Node  // node.children: ALL children incl. anonymous and extras
}
type Tree struct{ Root *Node; Source []byte; Language string }

// internal/lang
type ParseFunc func(src []byte) (*syntax.Node, error)
func Parse(grammar string, src []byte) (*syntax.Tree, error)
func ParseText(grammar, text string) (*syntax.Tree, error)
func GrammarFor(path string) string   // python go java rust tsx javascript typescript
func Supported(grammar string) bool
```

Accessors mirror py-tree-sitter: `ChildCount`, `Child(i)`, `NamedChildren`,
`ChildByFieldName`, `ChildrenByFieldName`, `ChildOfType(types...)` (upstream
`child_of_type`), `Text(src)`, `StartLine`/`EndLine` (upstream `start_line` /
`end_line`, including the column-0 rule), `Descendants` (upstream
`descendants`, iterative preorder), `Walk` (preorder with subtree skipping),
`Ancestors`, `NextSibling`, `PrevSibling`, `HasError`.

Rules every parser must honour:
- Input is the output of `source.Read`/`source.Text` re-encoded as UTF-8; all
  offsets refer to it. Rows split on `\n` only.
- Every node's span is `[first child start, last child end)` except where the
  grammar says otherwise (Python's `module` ends at EOF and starts at its first
  child; an empty module is `[len,len)`).
- Extras sit where tree-sitter puts them: strictly between two (non-extra)
  children of the deepest enclosing node, counting the grammar's hidden
  tokens (Python NEWLINE/INDENT/DEDENT) as children. See
  `internal/lang/python/extras.go`.
- Walks must be iterative (or deep-stack safe): the corpora include 1200-term
  expressions.
- Parsers return an error for input they cannot handle. They do not emulate
  tree-sitter ERROR recovery (yet). Tools treat a parse error as "no
  functions/candidates/sites in this file" (see DEVIATIONS.md).

Builders: parsers set only type/flags/field/byte spans (`syntax.Leaf`,
`syntax.Inner` fits a parent to its children), then call `syntax.Finish(root,
src)` once, which sets every `Parent` and derives every `Point`.

## How to add a language

1. Create `internal/lang/<grammar>/` exposing `func Parse(src []byte) (*syntax.Node, error)`.
2. Register it in `internal/lang/lang.go` (`registry`).
3. Build a corpus under `testdata/<grammar>/` and reach `treediff` parity
   (`go run ./tools/treediff testdata/<grammar> fixtures/<lang>`). Use the
   node inventory in `PARSERS.md` to prioritise: what the tools read must be
   exact first; dryer needs the full named structure.
4. Add a parity test like `internal/lang/python/parity_test.go` (one `treediff.Parity` call listing the corpora).
5. Write the tool readers over the tree: e.g. `internal/crap/lang_<lang>.go`
   registered in `crap.Readers` (signature `func(source, path, projectRoot string) ([]model.Function, error)`).

Go may instead be parsed with `go/parser` + `go/ast` (spec-crapper §4.3
recommends it); then convert to the tree-sitter-go shape if dryer/mutator
need the tree, or implement the Go readers on `go/ast` directly.

## How tools consume trees

Tools are line-by-line ports of the upstream Python over `*syntax.Node`.
Example (`internal/crap/lang_python.go` vs upstream `languages/python.py`):
`tree.Root.Descendants` ≙ `descendants(tree.root_node)`, `n.ChildOfType("identifier")`
≙ `child_of_type(node, "identifier")`, `crap.Complexity(n, isDecision, skip)` ≙
`complexity(node, _is_decision, _skip)`. Keep names and control flow close to
upstream so diffs against the Python stay reviewable.

## Parity policy

Default = match upstream byte for byte: stdout, stderr messages, exit codes,
`.metrics/*` files, including every documented upstream quirk (spec §9 lists
for each tool). Deviations are allowed only

- where exact parity is impossible or meaningless (Python tracebacks become a
  one-line `crapper: <error>` on stderr with exit 1; glob order is sorted), or
- behind an explicit flag,

and every deviation is listed in `DEVIATIONS.md`. When in doubt, run upstream
(`upstream/<tool>/<tool>` or the harness `live` mode) and match it.

## Tests

`go test ./...` — unit tests ported from upstream `tests/*.py` (Python-relevant
parts so far), spec goldens (§8), a smoke test that runs the whole crap
pipeline on `internal/crap/testdata/e2e` against output recorded from
upstream, and the parser parity test (skipped when the upstream venv is
missing). The harness (`harness/run.sh check`) is the end-to-end gate.
