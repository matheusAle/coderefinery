# Parser bugs found by the tools

Concise repros of hand-parser divergences from tree-sitter (pack 1.23.1)
that tools hit. Check each with `go run ./tools/treediff FILE`. Append new
ones under the language; remove an entry once its parser is fixed.

## python

Found by `tools/dryoracle` over CPython 3.12's stdlib + site-packages
(22 109 / 22 126 files identical in dryer's normalized forms; the rest are
these, P1/P2 in DEVIATIONS.md, and ERROR trees).

| repro | ours | tree-sitter |
|---|---|---|
| `x = [*[a] * b]\n` | `list(binary_operator(list_splat([a]), *, b))` | `list(list_splat(binary_operator([a] * b)))` |
| `x = [*{k: v for k in d}.values()]\n` | `call(function: attribute(list_splat(dict_comp), values))` | `call(function: list_splat(attribute(dict_comp, values)))` (yet `[*a.b()]` agrees) |
| `f"{a, b=}"\n` | `interpolation(expression: pattern_list)` | `interpolation(expression: expression_list)` |

## go

Status (lang-go): all six repros below and the long parameter list are
fixed and kept in `testdata/go/dryer_repros.go.txt` / `glr.go.txt`.
dryoracle rerun after the fix: 7 610 / 7 651 comparable files identical; 36
parse errors (invalid `testdata`); the 5 remaining differences are all
tree-sitter ERROR/MISSING trees (e.g. `&X{make(n - 1)}`, where tree-sitter
wants a type as make's first argument), which A2 does not emulate.

Found by `tools/dryoracle` over `$(go env GOROOT)/src`: 7 525 / 7 651
comparable files identical (51 more crash upstream dryer, spec §4.6); 43
parse errors (mostly invalid `testdata`), 83 tree differences, mainly these
(each body is `package p\n\nfunc f() {\n\t<stmt>\n}\n`):

| stmt | ours | tree-sitter |
|---|---|---|
| `_ = (*(*[10]byte)(q))[:]` | inner `(*[10]byte)(q)` as `unary_expression` | `type_conversion_expression` |
| `_ = (*entry[K, V])(q)` | `call_expression` | `type_conversion_expression` |
| `_ = reflect.TypeFor[abi.Type]()` | `call(function: index_expression)` | `call(function: selector_expression, type_arguments)` |
| `_ = &s.m[r.n][r.i]` | `index_expression(index_expression)` | `index_expression(type_instantiation_expression)` |
| `_ = new(par.Cache[K, R])` | `type_instantiation_expression` | `generic_type(qualified_type …)` |
| `switch {\n\tcase 4:\n\t\tfallthrough\n\t\t;\n\t\t;\n\t}` | `expression_case` ends after the last `;`, one `empty_statement` | case spans to the next line start, two `empty_statement`s |

Also: `cmd/compile/internal/ssa/_gen/simdAMD64ops.go` line 5 (a long
`func simdAMD64Ops(v11, v21, …, regInfo)` parameter list) groups names into
different `parameter_declaration`s.

## typescript

Status after lang-ts's fixes (dryoracle rerun): 12 370 / 12 398 files
identical. The other 28 are all input on which tree-sitter itself builds
ERROR/MISSING trees (A2; lang-ts's recovery is DEVIATIONS T1): 25 parse
errors (20 `export type * from`, 4 bare `&` in JSX text, 1 more) and 3
diffs (2 ERROR trees, plus `useState<\n  import('m').T[]\n>([])`, where
tree-sitter's tree has a MISSING identifier). No open TypeScript item.

Original report (all fixed or ERROR cases):

Found by `tools/dryoracle` over a 12 398-file TS/TSX application monorepo:
12 083 identical; 28 parse errors (mostly `bad export`, 4 `bad character
reference` in TSX); the dominant tree difference (283 files):

| repro | ours | tree-sitter |
|---|---|---|
| `async function f() {\n  const r = await a.get<T>(x);\n}\n` | `await_expression(call_expression(member, type_arguments, arguments))` | `call_expression(function: await_expression(member), type_arguments, arguments)` (only with type arguments; `await a.get(x)` agrees) |

Parse errors (synthetic repros of the 28 failing files, each confirmed with treediff):

| repro | ours | files |
|---|---|---|
| `export type * from './types';\n` | `bad export` at offset 7 | 20 (barrel `index.ts`/`index.tsx` files and one mixins module) |
| `type T = { export?: number };\n` (keyword as property name) | `bad export` | 2 generated GraphQL type files |
| `const a = <a>Docs & Guides</a>;\n` (.tsx, bare `&` in JSX text) | `bad character reference` | 4 |
| not minimized yet: an `export default function (plop: …)` file and a `.tsx` that fails with `unexpected ")"` | | 2 (paths sent to lang-ts) |

## rust

Found by `tools/dryoracle` over `~/.cargo/registry/src` (10 249 files):
10 211 identical. Nearly every remaining difference is a tree-sitter ERROR
node on valid code (macro-heavy files, attributes in patterns), which A2
does not emulate. One non-ERROR case: `unicode-segmentation-1.13.2/src/tables.rs`
line 546, a `map_or(` call whose arguments are preceded by comments and end in
`|r| (r[0] as usize)..((r[1] + 1) as usize)`: tree-sitter makes it
`call_expression(closure_expression(…), arguments)`, ours a plain closure
(a comment-free one-liner agrees).
