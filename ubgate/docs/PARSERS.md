# Parsers: parity workflow and node inventory

Every ubgate parser must build the tree upstream tree-sitter builds
(tree-sitter 0.26.0 + tree-sitter-language-pack 1.23.1, the versions in
`../upstream/*/.venv`). Parity is checked differentially, never by eye.

## The oracle

`tools/oracle/oracle.py` prints upstream tree-sitter's tree in exactly the
format of `syntax.Dump`:

```
<indent>[<field>: ]<kind> [<start>,<end>) <sr>:<sc>-<er>:<ec>[ MISSING][ EXTRA][ <text>]
```

- one line per node, preorder, two spaces per depth level;
- `kind` = bare type for named nodes, quoted type for anonymous ones (`"("`, `"not in"`);
- byte range half-open; points are 0-based row and **byte** column;
- `<text>` (quoted source slice) only for leaves (`child_count == 0`);
- quoting: `\\ \" \n \r \t`, other control chars `\xNN`, everything else literal UTF-8.

The file is read the way the tools read it (`read_text(utf-8, errors=replace)`,
universal newlines), then re-encoded, so offsets agree with `internal/source`.

```
# always use an upstream venv interpreter, isolated (-I)
../upstream/crapper/.venv/bin/python -I tools/oracle/oracle.py FILE...          # parity dump
../upstream/crapper/.venv/bin/python -I tools/oracle/oracle.py --sexp FILE      # str(root_node)
../upstream/crapper/.venv/bin/python -I tools/oracle/oracle.py --lang tsx FILE  # force grammar
```

Grammar by extension (same rule as the three tools): `.py` python, `.go` go,
`.java` java, `.rs` rust, `.tsx` tsx, `.js .jsx .mjs .cjs` javascript,
`.ts .mts .cts` typescript. With several files each dump is preceded by
`=== FILE <path>`.

To see the grammar's full inventory (named/anonymous types, fields):

```
../upstream/crapper/.venv/bin/python -I -c "
from tree_sitter_language_pack import get_language
L = get_language('python')
print(sorted({L.node_kind_for_id(i) for i in range(L.node_kind_count) if L.node_kind_is_visible(i) and L.node_kind_is_named(i)}))
print([L.field_name_for_id(i) for i in range(1, L.field_count + 1)])"
```

## treediff

```
go run ./tools/treediff [-lang L] [-q] PATH...      # files or directories
make treediff                                       # default Python corpus
```

It collects files with a registered parser (skipping `.venv`, `node_modules`,
`.git`, `__pycache__`), runs the oracle once per 400 files, parses each file
with `lang.ParseText` and compares dumps. Per file: `PASS`, or `FAIL` with the
first differing dump line plus four lines of shared context (`ours:` /
`theirs:`), or the parse error. Exit 0 only when every file is identical.

The same logic is a library (`internal/treediff`) for parity tests; see
`internal/lang/python/parity_test.go` (skips when the venv is absent).

Probe a construct quickly:

```
printf 'x = [*a.b]\n' > /tmp/p.py
../upstream/crapper/.venv/bin/python -I tools/oracle/oracle.py /tmp/p.py
```

Beware: the agent shell tool rewrites `\uXXXX` in command text into literal
characters. Generate test sources containing backslash-u escapes from a
script (e.g. Python `chr(92)`), not from heredocs.

## Status

| grammar | package | parity |
|---|---|---|
| python | `internal/lang/python` | 23 213 / 23 253 files identical over CPython 3.12's stdlib + test suite, pip, pygments, pytest, coverage and the upstream tools; 100% on `testdata/python`, `fixtures/python`, `upstream/*/{src,tests}` |
| java | `internal/lang/java` | 100% on `testdata/java` (41 files: upstream test snippets, spec cases, a small project) and on the 15 759 `.java` files of the JDK 21 `src.zip` |
| go | `internal/lang/golang` | 7 647 / 7 702 `.go` files identical over Go 1.26's `$GOROOT/src` (the 55 others: 53 under `testdata/` — deliberately invalid sources — and 2 where tree-sitter itself builds ERROR nodes); 100% on `testdata/go`, `fixtures/go`, the ubgate and harness sources and the Go snippets of `upstream/*/tests` (except one truncated, invalid snippet) |
| rust | `internal/lang/rust` | 10 162 / 10 216 `.rs` files identical over the 410 crates of the local cargo registry (`~/.cargo/registry/src`); all 54 others are files where tree-sitter itself builds ERROR/MISSING nodes (DEVIATIONS R1). 100% on `testdata/rust` (a workspace exercising the grammar plus the upstream test snippets) |
| typescript, tsx, javascript | `internal/lang/typescript` | 100% of the files tree-sitter parses cleanly in every corpus tried: 547/547 `.ts`/`.d.ts` and 1 064/1 064 `.js`/`.mjs`/`.cjs` from `fixtures/*/node_modules`, a 12 243-file private TS/TSX application monorepo (8 322 ts, 3 845 tsx, 76 js), `testdata/typescript` (upstream test snippets, spec §8.2 cases, tricky TS/TSX/JSX) and `fixtures/{typescript,javascript}`. Files with tree-sitter ERROR/MISSING nodes are excluded (DEVIATIONS T1) |

Known Python divergences (all rare): files tree-sitter itself fails on
(ERROR/MISSING nodes, Python 2 backticks, a leading BOM, bad encodings,
PEP 701 comments inside f-string interpolations); `print(..., *a.b())`, where
tree-sitter's GLR picks a different splat binding inside `print` calls only.

### Rust parser notes (tree-sitter-rust quirks reproduced)

- The lexer is lazy (the parser asks for the token at an offset), so
  compound tokens split where only their first character is valid: `>>`
  closes two generic lists, `||` is an empty closure, `&&` two references,
  and a prefix `..=` is `..` then `=` (`f(..=5)` is an assignment).
- A block-like expression at statement start ends the statement unless the
  next token cannot start a statement (`if a {} - 1` is two statements,
  `match x {}.f()` one); a following `;` is an `empty_statement`. The last
  block-like statement of a block is an `expression_statement`, never the
  tail expression.
- After a bare `..` or `break` (prec.left), a token that is both prefix and
  infix continues the outer expression: `a..-b` is `(a..) - b`, `break (x)`
  a call. Inside a closure body `x..(y)` calls the closure.
- Let chains: the prefix before the first `let` is one binary expression;
  after it every `&&` element is a separate child; `let` values stop at a
  top-level `&&`.
- `+` in types makes a left-associative `bounded_type` around the whole
  preceding type (`&dyn A + B`), except in trait bounds; `impl 'a + T` puts
  the bounded type inside the `abstract_type`.
- Raw strings: `string_content` starts after leading whitespace. Floats
  follow the external scanner (`1.` is a float, `1..2` and `1.max()` are
  not); tuple indices after `.` are integers, but inside token trees
  `x.0.1` lexes a float `0.1`.
- Token trees: `mut` is `mutable_specifier`, `self`/`super`/`crate` named,
  primitive types `primitive_type`, a fixed keyword set anonymous, other
  words `identifier`; `$x` is a metavariable only in `macro_rules!` bodies;
  the repetition separator is a hidden token.

### Python parser notes (tree-sitter-python quirks reproduced)

- No `expression_statement` node: statement-level expressions are direct
  children of `module`/`block`; a bare tuple statement is `tuple_expression`.
- Precedence follows grammar.js: `^` (16) binds tighter than `&` (15).
- `type` annotations: `NAME[...]` is `generic_type`; `|` after a generic is
  `union_type` (left-assoc); after a plain expression it stays `binary_operator`.
  `type(a).b = 1` parses as `type_alias_statement`.
- `*x` in list/set/expression_list binds to the atom (`[*a.b]` =
  `attribute(list_splat(a), b)`), except a list/set with more elements
  (`[*a.b, c]` = `list_splat(attribute)`); in calls and tuples it is loose.
- `(x) = 1`, `(x): int` -> `tuple_pattern`.
- Strings: `string_content` holds `escape_sequence`, `escape_interpolation`
  and anonymous `"\\"` (non-escape backslash). Raw strings whose content is
  only backslash pairs fold them into `string_end`; before `{`/`}` in raw
  f-strings they are re-read as escapes.
- A `\`-continuation directly before a string is swallowed (scanner padding);
  elsewhere it is a `line_continuation` extra.
- NEWLINE/INDENT/DEDENT positions and comment-held dedents follow
  `scanner.c` (a comment at or right of the block's indentation keeps the
  block open, so trailing comments land inside it).

### Go parser notes (`internal/lang/golang`)

Why hand-written: the tokens come from `go/scanner` (exact Go lexing,
comments, automatic semicolons), but the tree is built by a recursive
descent that follows tree-sitter-go's grammar.js directly. Translating
`go/ast` would have meant undoing its normalisations (no comma or `else`
positions, `ParenExpr`/`IndexExpr` resolved semantically, no
`expression_list`/`literal_element`/`statement_list` wrappers) and then
re-deciding every syntax-only choice tree-sitter makes anyway; building the
tree-sitter shape directly was faster to get exact.

tree-sitter-go behaviour reproduced:
- Terminators: an automatic semicolon is tree-sitter's hidden newline token,
  spanning the whole run of consecutive `\n` (so `statement_list` ends after
  blank lines but before indented ones), placed after a trailing comment. An
  explicit `;` is a visible `";"`; after an empty statement the newline is a
  terminator again although go/scanner inserts nothing. At EOF without a
  newline only an ungrouped `var`/`const` spec with a value gets the visible
  zero-width `""` (`\0`) terminator. An empty labeled statement ends a list.
- `source_file` spans from its first child to EOF; comments sit between two
  children of the deepest enclosing node, hidden terminators counted.
- `const_spec` and `type_case` put the field name on their commas too.
- `X[...]` ambiguity: tree-sitter keeps the GLR reading with the highest
  dynamic precedence; fitted values: `type_identifier` in a type position
  -1, `generic_type` +1, `type_arguments` +1, `type_instantiation_expression`
  -1, the rest 0, ties keep the expression reading (`types.go`: `tscore`,
  `escore`). Hence `m[k](x)` and `f[a.B](x)` are `type_conversion_expression`,
  `f[i](a, b)` and `f[i]()` call an `index_expression`, `f[a.B]()` is a call
  with `type_arguments`, `h[i][y[j]]` is a `type_instantiation_expression`,
  `(*Foo[R])(v)` a conversion, `(*T)(x)` a call of a parenthesized unary.
- `*` before a type-only operand binds first: `*(*[]T)(p)` converts to the
  pointer type. `new`/`make` take a type first (`new(nil)` is
  `type_identifier`). `chan<- chan T` is `chan (<-chan T)`.
- Parameter lists: at most 9 names in one `parameter_declaration` (GLR
  version limit: older bare names become type-only declarations); bare names
  before a variadic stay types; after a bare name, `func(T)` is read as the
  name `func` plus `parenthesized_type`.
- `type T[P C]` vs array `type T [N]E`: type parameters when the bracket has
  a top-level comma or `[` IDENT is followed by a type-ish token.
- Escapes split `interpreted_string_literal_content`; an empty raw string
  still has a zero-width `raw_string_literal_content`.

Known Go divergences: inputs tree-sitter recovers from with ERROR/MISSING
nodes (e.g. `make(n - 1)`, `new(f(x))`, `[...]T` outside literals, a leading
BOM) are parse errors here (A2); a conversion-like `m[k](x)` as the very last
token of a file without a trailing newline (tree-sitter flips to the call
reading there).

### TypeScript / JavaScript parser notes (tree-sitter quirks reproduced)

- One package, three entry points (`ParseTypeScript`, `ParseTSX`,
  `ParseJavaScript`). The JS grammar (0.25) differs: `field_definition`
  (`property:`), class members carry `member:`, `optional_chain` is a leaf
  (TS wraps an anonymous `?.`; a TS `a?.()` has a bare `?.`), `default` in
  export specifiers is anonymous (an `identifier` in TS).
- `??` sits at the ternary level (below `||`). TS `as`, `satisfies` and
  postfix `!` take the whole binary expression to their left
  (`a + b!` = `non_null(a + b)`), but `await a as T` = `await(as)`.
- TS type-argument calls/instantiations bind like a relational operator:
  `a + b<T>(c)` = `call(a + b)`, `a || b<T>(c)` keeps the call on `b`,
  `await a.get<T>(x)` = `call(await(a.get))`. A template after `>` makes it
  a comparison. An import type inside expression type arguments
  (`f<typeof import("m")>()`) is a comparison too; when that reading does
  not parse, the file is reparsed with the call reading.
- In the type after `as`/`satisfies`, a dotted name keeps two parts
  (`x as a.b.C` = member `C` of `x as a.b`), and an import type takes no
  `[...]`.
- JS-only: `await (x)` followed by a call/member/subscript is a call of an
  identifier `await`; `new new A()()` puts the call inside the constructor.
- Automatic semicolons are zero-width hidden tokens, but their position
  decides which node keeps a trailing same-line comment (`asi.go`): the TS
  scanner absorbs every same-line comment into the statement or block
  (`f() // c` keeps the comment inside the expression_statement); the newer
  JS scanner follows `scan_whitespace_and_comments` and absorbs into
  blocks only when they are empty and not try/catch/method/case/exported
  bodies.
- JSX text is the scanner's `jsx_text` (leading whitespace included,
  whitespace-only runs that start at a line break dropped); `//` in JSX text
  is text.

## Node-type inventory per tool (what must be exact first)

### crapper (`internal/crap`, spec-crapper §4)

Lines come from `start_point.row + 1` / `end_line()` of the entry node; only
start lines must be exact for coverage, but parity of end lines is cheap.

| language | entries / names | decisions (+1 each) | skipped subtrees |
|---|---|---|---|
| python | `function_definition` (not nested in another def before a class), name = first `identifier` child; namespace from `class_definition` ancestors' `identifier` | `if_statement elif_clause for_statement while_statement except_clause conditional_expression case_clause if_clause`, `boolean_operator` with an `and`/`or` child | `class_definition` |
| typescript/tsx/javascript | `function_declaration` (+`statement_block` child, first `identifier`), `method_definition` (+`statement_block`, `property_identifier`/`identifier`) under `class_declaration`/`abstract_class_declaration` (`type_identifier`/`identifier`), `lexical_declaration` > `variable_declarator` > `identifier` + `arrow_function`/`function_expression`; route callbacks: `call_expression` > `member_expression` > `property_identifier`, `arguments`, `parenthesized_expression`, `string` > `string_fragment`, `template_string`; ancestors checked: `function_declaration method_definition arrow_function function_expression`; `generator_function_declaration` must exist (it is excluded) | `if_statement for_statement for_in_statement while_statement do_statement catch_clause ternary_expression switch_case switch_default`, `binary_expression` with `&&`/`\|\|`/`??`, every `optional_chain`, bare `?.` | `class_declaration abstract_class_declaration interface_declaration`, route callbacks |
| go | `function_declaration`/`method_declaration` with `block`; `identifier`, `field_identifier`, first `parameter_list` > `type_identifier`, `package_identifier` (or go/ast) | `if_statement for_statement expression_case type_case communication_case default_case`, `binary_expression` with `&&`/`\|\|` | none |
| java | `method_declaration` with `block` + `identifier`; ancestors `method_declaration constructor_declaration lambda_expression`; type chain `class_declaration interface_declaration enum_declaration record_declaration annotation_type_declaration` > `identifier` | `if_statement for_statement enhanced_for_statement while_statement do_statement catch_clause ternary_expression switch_label`, binary `&&`/`\|\|` | nested type declarations, `class_body` |
| rust | `function_item` with `block`, parent not `block`; `mod_item` > `identifier` (`tests`); `declaration_list` < `impl_item` children `for`, `type_identifier generic_type scoped_type_identifier`; `trait_item` | `if_expression for_expression while_expression loop_expression match_arm try_expression`, binary `&&`/`\|\|`; nothing inside `token_tree` | none |

### dryer (`internal/dry`, spec-dryer §3–§4)

Dryer normalizes the **whole candidate subtree**: every named node type,
nesting level and wrapper node changes node counts and Jaccard scores. Type
*names* never reach the output, but merging two types or adding/dropping a
wrapper does. Exact requirements:
- candidates: Python `function_definition`, `class_definition`; TS
  `function_declaration method_definition statement_block arrow_function
  function_expression variable_declarator public_field_definition
  property_definition`; Go `function_declaration method_declaration block
  func_literal`; Java `method_declaration constructor_declaration
  lambda_expression block`; Rust `function_item block mod_item identifier`.
- special sets (spec-dryer §4.1): `SKIP` comments; `IDENTIFIERS`; `LITERALS`
  (incl. `string` with/without `interpolation` child); every anonymous
  operator token in `OPERATORS` (spelling matters: `not in`/`is not` are
  dropped, `->`, `::` dropped); `CALLS` (`call call_expression
  method_invocation object_creation_expression new_expression`);
  `ATTRIBUTES` (`attribute member_expression selector_expression
  field_access field_expression`); `SCOPED`; `ARG_LISTS` (`argument_list
  arguments`); `TYPE_ARGS`; `parenthesized_expression`; `generic_type`.
- identifier *text* in callee positions (call targets) is kept.
- Appendix A of spec-dryer lists the operator tokens per grammar.

### mutator (`internal/mutate`, spec-mutator §2)

Mutator reads **leaves**, their **parent type**, and the nearest ancestor
whose type ends in `_statement` or `_declaration` (anchor line). Exact:
- leaf tokens `+ - * / < <= > >= == != === !== && || ?? and or not ! ?.` with
  parents `binary_expression binary_operator comparison_operator
  boolean_operator unary_expression unary_operator not_operator
  optional_chain member_expression call_expression subscript_expression`;
- literal leaves `true false` (Python `True`/`False` text), integer node types
  `integer number int_literal decimal_integer_literal integer_literal`;
- TS type contexts (sites skipped below them): `type_annotation
  type_alias_declaration interface_declaration type_arguments type_parameters
  literal_type`;
- every `*_statement` / `*_declaration` type and its start row (Python
  `elif_clause` / `case_clause` are *not* statements, so anchors move to the
  `if`/`match` line);
- byte offsets of every leaf (mutation ids embed them).
- plus crapper's function spans (owner lookup).
