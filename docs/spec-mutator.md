# spec-mutator — implementation-ready spec for a stdlib-only Go port

Reference: `upstream/mutator` @ `a5a0cb8` (2026-10-21), `upstream/crapper` @ `6fd92c4`.
All 76 upstream tests pass. Facts marked **[probe]** were checked by running upstream against
tree-sitter 0.26.0 + tree-sitter-language-pack 1.23.1 in a throwaway venv (`/tmp/tsvenv`, outside the repo).
Function discovery, namespaces, coverage generation and LCOV/Go-profile/JaCoCo parsing belong to
crapper. Mutator only consumes them, so see `docs/spec-crapper.md` for those. This spec says exactly
which crapper outputs mutator reads.

---

## 0. Pipeline overview

```
parse args → select files → (no files: "No source files to mutate.", exit 0)
→ [not --scan] restore target/mutator-backup → prepare coverage (unless --scan/--no-coverage/--use-existing-coverage)
→ --scan: per file print scan inventory, exit 0
→ else per file (sequential): sites → coverage map → history → select → baseline (cached) → workers run mutants
  → forms → write snapshots → print site log
→ print report table (stdout), "Wrote <abs path>" lines (stderr) → exit code
```

---

## 1. CLI

### 1.1 Parsing (`cli.parse_args`)
- If **any** argv element is `-h` or `--help` (in any position, even as an option's value), print HELP to stdout and exit 0.
- Options are scanned left to right. A value option takes the next arg (`_take`). It fails with `<opt> requires a value` when there is no next arg, the next arg is `""`, or it **starts with `-`**. So `--timeout-factor -1` gives "requires a value", and `--test-command "-x"` is impossible.
- Unknown arg starting with `-` → `Unknown option: <arg>`. Everything else is a positional.
- A usage error prints `<message>\n\n<HELP>` to **stderr** and exits **1**.

| Flag | Value | Default | Notes |
|---|---|---|---|
| `-h`, `--help` | – | – | exit 0, stdout |
| `--root <path>` | path | `.` | project root, then `resolve()`d (symlinks resolved, e.g. `/tmp`→`/private/tmp`) |
| `-s`, `--source-root <path>` | path, repeatable | `[]` | joined to root (`root/path`; an absolute path stays absolute) |
| `--changed` | – | false | git-dirty sources |
| `--scan` | – | false | inventory only |
| `--mutate-all` | – | false | ignore history |
| `--since-last-run` | – | false | **no behaviour**. It only takes part in the conflict check (it is the default anyway) |
| `--lines <n,n,...>` | ints | None | split on `,`, strip, skip empty pieces. `int()` failure → that Python message, e.g. `invalid literal for int() with base 10: 'x'`. `<1` → `--lines requires positive line numbers`. No numbers → `--lines requires a line number` |
| `--no-coverage` | – | false | every site covered |
| `--use-existing-coverage`, `--reuse-coverage` | – | false | read reports on disk, do not generate |
| `--coverage-command <cmd>` | shell string | None | |
| `--test-command <cmd>` | shell string | None | replaces default test plan. cwd = root |
| `--timeout-factor <f>` | float | 10.0 | `<=0` → `--timeout-factor requires a positive number`. Parse error → Python float message |
| `--mutation-warning <n>` | int | 50 | warn threshold |
| `--max-workers <n>` | int ≥1 | None (=cores) | non-int or <1 → `--max-workers requires a positive integer` |
| `--verbose` | – | false | |

Post-parse conflicts (exit 1, message + HELP on stderr), checked in this order:
1. `--mutate-all` + `--since-last-run` → `--mutate-all cannot be combined with --since-last-run`
2. `--no-coverage` + `--coverage-command` → `--no-coverage cannot be combined with --coverage-command`
3. `--scan` + `--mutate-all` → `--scan cannot be combined with --mutate-all`
4. `--scan` + `--max-workers` → `--scan cannot be combined with --max-workers`

HELP text: reproduce `cli.py` lines 54–134 verbatim. The skipped-directory list is
`sorted(SKIP_DIRS ∪ TEST_DIRS)` joined `", "` with `", and "` before the last item:
`.clj-kondo, .git, .hg, .idea, .metrics, .svn, .venv, __pycache__, __tests__, build, coverage, dist, node_modules, out, spec, specs, target, test, testdata, tests, vendor, and venv`.

### 1.2 Positionals (`_positionals`)
For each positional: `candidate = arg` if absolute, else **`root/arg`** (relative to `--root`, *not* cwd).
- If it exists → "existing" (resolved).
- Otherwise it becomes a **filter**: a file is kept when `filter in abs_resolved_path.as_posix()` (substring of the absolute path, so it can match directory names above the project).

### 1.3 File selection (`select_files`)
The first matching rule wins:
1. `--changed` → changed sources (§1.4).
2. `--source-root`s → `iter_source_files([root/p ...])`.
3. existing positionals → for each: a dir gives `iter_source_files([dir])`. A file of a known language that is not a test file is kept. A test file prints `Skipping test file <abs path>` (stderr). Any other file is dropped silently.
4. else `iter_source_files([root])`.

Then: apply filters → drop files outside root (stderr `Skipping <path>; it is outside <root>`) → resolve → dedupe → sort by POSIX path string (codepoint order).

crapper `discover` (needed verbatim):
- `language_of`: `*.d.ts` → None. Extensions: `.clj .cljc .cljs .bb`→clojure, `.java`, `.go`, `.ts .tsx .mts .cts .js .jsx .mjs .cjs`→typescript, `.rs`→rust, `.py`→python.
- `is_test_file`: name ends `_test.go`, `_test.clj|.cljc|.cljs|.bb`, `.test.|.spec.` + `ts|tsx|mts|cts|js|jsx|mjs|cjs`. Also `conftest.py`, `*_test.py`, `test_*.py`, or **any path component** in `TEST_DIRS = {test, tests, spec, specs, __tests__}` (absolute path, so a project under `/x/tests/` has only test files).
- walk: prune dirs in `SKIP_DIRS = {.git,.hg,.svn,.idea,.venv,venv,node_modules,vendor,target,dist,build,out,coverage,.metrics,testdata,__pycache__,.clj-kondo}` ∪ TEST_DIRS. `iter_source_files` given a *file* returns it unchecked (quirk: `--source-root file.txt` passes).

### 1.4 `--changed`
- `git -C <root> rev-parse --show-toplevel`, then `git -C <root> status --porcelain -z --no-renames --untracked-files=all -- .`
- Non-zero git exit → stderr = git stderr stripped, or `git status failed` if that is empty. **Process exit code = git's code** (e.g. 128).
- Split stdout on NUL. Skip entries with `len ≤ 3`. Path = `toplevel / entry[3:]` resolved. Keep it if it is a file (deletions drop out) and inside root. A rename is just the new path (`--no-renames` emits D+A). The status letters are ignored.
- Filter: no component of `path.relative_to(root)` may be in SKIP_DIRS∪TEST_DIRS, the language must be known, and it must not be a test file.

### 1.5 Exit codes
| Code | When |
|---|---|
| 0 | every executed mutant killed, nothing to run, no files, `--scan`, `--help` |
| 1 | usage error (or an unhandled Python exception) |
| 2 | any file's baseline failed (beats 3). Also "crapper missing" (N/A in Go) |
| 3 | no baseline failure and some form in this run's results has `survived > 0`. This includes survivors carried forward / counted (see §7.3) |
| git rc | `--changed` git failure |
| cov rc | `--coverage-command` exited non-zero → its exit code is returned as-is (may be 3!) |

### 1.6 Output streams
stdout:
- `No source files to mutate.` (no files).
- Scan inventory (§1.7).
- Per file, right after it finishes, the **site log**: one line per site that has a status, in site order: `f"{STATUS:9} {file_key}:{line} {description}"`. STATUS is `KILLED`, `SURVIVED` or `UNCOVERED`, left-aligned and padded to 9, then a space. Uncovered sites are UNCOVERED. Covered sites with an outcome (newly run **or carried**) show it. Covered sites without an outcome (with `--lines`) are omitted.
- After all files, if any forms exist: the report table (§1.8).

stderr:
- `+ (<cwd>) <display_command>` before every command when `--verbose`. This covers baseline, mutants and `--coverage-command`, but not crapper's own coverage runs, which always print `+ (<cwd>) <cmd>` themselves.
- `--verbose`: before each mutant, `<file_key>:<line> <description>`.
- `No coverage data for <file_key>; its sites are uncovered. Pass --no-coverage to run them anyway.` when the coverage lookup returns None, `--no-coverage` is off and the file has ≥1 site.
- `WARNING: <n> covered mutations selected in <file_key>.` when `len(selected) > --mutation-warning`.
- `Baseline failed for <file_key>: <display_command>` + `\n<last 20 output lines>` (only if there is output).
- `Skipping <file_key>: unsupported file` / `Skipping <file_key>: file is not UTF-8`.
- `Skipped <file_key>:<line> <description>; source bytes moved` (can't trigger normally → status survived).
- `Restored <abs path> from an interrupted mutation.`
- `Coverage command exited <rc>. Reports already on disk will not be read.` + last 20 lines of its captured output.
- At the very end: `Wrote <abs posix path>` for every path returned by every `write_results` call, in order. Duplicates are possible because each write rewrites every non-empty snapshot (§7.5).

`description` = `"delete <orig>"` if the mutant is `""`, else `"<orig> -> <mutant>"`.

`display_command`: a string as-is. A list is `shlex.quote` of each part joined by `" "`. shlex.quote: `''` for empty; unchanged if every char is in `[A-Za-z0-9_@%+=:,./-]`; else `'` + s.replace(`'`, `'"'"'`) + `'`.

### 1.7 `--scan` format (`report.format_scan`)
Per file:
```
Scan: <N> mutation sites in <file_key>
<mark> <file_key>:<line> <description>[ uncovered]  [<form_id>]
...
[* marks a function whose text changed since the last snapshot.]
<empty line → the block ends with "\n">
```
- `N` = sites after the `--lines` filter (`site.line in lines`).
- `mark` = `*` if `(namespace, form_id)` ∈ changed, else a space. `changed` is computed only when history has ≥1 form for this file: the digest keys (all forms of the file, with or without sites) that are not in history, or whose digest differs. The footer line prints iff `changed` is non-empty, even when no listed site carries a `*`.
- ` uncovered` only when a coverage map exists (not `--no-coverage` **and** lookup ≠ None) and the site is uncovered. Two spaces come before `[`.
- Scan never generates coverage (it reads reports on disk unless `--no-coverage`). It never restores backups, never writes, never warns. A non-UTF-8 file crashes it (Python traceback).

Example **[probe]**:
```
Scan: 6 mutation sites in src/demo.py
  src/demo.py:2 > -> >=  [defn/add]
* src/demo.py:7 delete -  [defn-/_h]
* marks a function whose text changed since the last snapshot.
```

### 1.8 Report table (`report.format_results`), stdout
```
nw  = max(8, min(48, max(len(name))))       # len = code points; long names are NOT truncated
nsw = max(9, min(72, max(len(namespace))))
header = f"{'Function':<nw} {'Namespace':<nsw} {'Killed':>7} {'Survived':>9} {'Uncovered':>10} {'Sites':>6} {'Score':>7}"
rows sorted by (score is not None, score or 0, -survived, name)   # n/a first, then ascending score
row    = f"{name:<nw} {ns:<nsw} {k:7d} {s:9d} {u:10d} {sites:6d} {score6}"
score6 = "   n/a" | f"{score*100:5.1f}%"           # 6 chars (one narrower than the header column; keep it)
total  = f"{'Total':<nw} {'':<nsw} {K:7d} {S:9d} {U:10d} {SITES:6d} {tot:>7}"
tot    = "n/a" if K+S==0 else f"{K/(K+S)*100:.1f}%"
lines: header, '-'*len(header), rows..., '-'*len(header), total, ""  → joined "\n" (ends with "\n")
```
`name` = raw function name (e.g. `place`, or TS route `GET /users`), not the form id. With no forms it prints `No functions to mutate.\n`, but the CLI only prints the table when forms exist. Forms from all files are pooled into one table.

---

## 2. Site discovery (`sites.discover_raw`)

### 2.1 Algorithm (all non-Clojure languages)
Parse with tree-sitter. Grammar: `typescript` language → `tsx` for `.tsx`, `javascript` for `.js .jsx .mjs .cjs` (JSX enabled), else `typescript` (`.ts .mts .cts`). Others use their own grammar.
Visit every node depth-first in pre-order (explicit stack). For each node:
1. **TS only**: skip the node if any *ancestor* type ∈ `{type_annotation, type_alias_declaration, interface_declaration, type_arguments, type_parameters, literal_type}`.
2. Leaf node (`child_count == 0`), with `token = text`, `parent = parent.type`. All checks are independent; a token can match at most one in practice.
   - **binary**: `parent ∈ {binary_expression, binary_operator, comparison_operator, boolean_operator}` and `mutant = BINARY[token]` exists. Go `/` → none. `??` only for typescript.
   - **optional (TS only)**: token `?.`. Host = `parent.parent` if `parent.type == optional_chain` else `parent`. Host `member_expression` → mutant `.`. Host `call_expression` / `subscript_expression` → mutant `""`. Else none.
   - **not**: token ∈ `{!, not}` and `parent ∈ {unary_expression, unary_operator, not_operator, comparison_operator}` → mutant `""`.
   - **negation**: token `-` and `parent ∈ {unary_expression, unary_operator, not_operator}` → mutant `""`.
   - **boolean**: `node.type ∈ {true, false}` and token ∈ `{true,false,True,False}` → swap (`true↔false`, `True↔False`).
   - **constant**: `node.type ∈ {decimal_integer_literal, integer_literal, int_literal, integer, number, num_lit}` and the token text is **exactly** `0` or `1` → `0→1`, `1→0`.
3. Non-leaf with type `bool_lit` (Clojure only) → boolean swap.
4. A site is added only if `mutant != original`. Sort all sites by `(line, start_byte, end_byte)`.

`BINARY`: `+→-  -→+  *→/  /→*  >→>=  >=→>  <→<=  <=→<  ==→!=  !=→==  ===→!==  !==→===  &&→||  ||→&&  ??→||  and→or  or→and`.
Categories (internal only, never persisted): arithmetic `+ - * /`, comparison `> >= < <=`, equality `== != === !==`, logical `&& || ?? and or`, `optional`, `unary`, `boolean`, `constant`.

**Not mutated anywhere:** `% ** // << >> >>> & | ^ &^ @ ~`, unary `+`, `++ --`, compound assignments (`+= -=` …), `is`, `in`, `is not`, `not in` (tree-sitter-python makes `not in`/`is not` composite nodes, so `not`'s parent is not `comparison_operator`), `instanceof`, `typeof`, `None/null/nil/undefined`, floats, hex/octal/binary, suffixed ints (`1L 1n 1u8 1i32`), `1_0`, `01`, `1e0`, `1.`, `.5`, char/rune literals, any string/comment/regex/JSX-text content.

**Mutated even though often surprising** (consequences of the leaf rules) **[probe]**:
- Interpolations: Python f-string `{a > 1}` (not the format spec `:>10`, not `!r`), TS/JS template `${a > 1}`, JSX attribute/child expressions `{p.n - 1}`, `hidden={true}`.
- Go **and** README disagree: README omits `!`/unary `-` for Go, but the code **does** delete `!` and unary `-` in Go (unary_expression). Follow the code.
- `!true` gives two sites (`delete !`, `true -> false`). `-1` gives two (`delete -`, `1 -> 0`).
- Type positions in non-TS languages: Go array length `[1]int`, `[0]int`. Rust `[u8; 1]`, const generics. Python annotations `x: Literal[-1]` (delete `-` and `1`).
- Rust tuple index `t.0`, `t.1` (integer_literal). Rust literals inside macro/attribute token trees (`vec![1, 0]`, `assert!(a > 1 && true)` → only `1`, `true`). Operators/unary inside token trees are NOT sites.
- Pattern literals: Rust `match a { 0 => .., 1..=2 => .. }` (`0`, `1`), `Some(1)`. Rust `-1 =>` → `1` only (negative_literal, `-` not a site). Python `case -1:` → `1` only, `case {"k": 0}` → `0`.
- TS object numeric keys `{0: 1}` (both), computed `[1]: 0` (both). But `{ true: 1 }` key and `a.true` are property identifiers → not sites.
- TS default params `(x: number = 1)` → `1`. Enum initialisers are raw sites (dropped later, no owner).
- Rust `#[cfg(test)] let z = 0;` → `0`.

### 2.2 Per-language eligibility table (verified [probe])
| Lang | binary ops | unary delete | booleans | ints exactly 0/1 node |
|---|---|---|---|---|
| Python | `+ - * / < <= > >= == != and or` | `-` (unary_operator), `not` (not_operator) | `True False` (node types `true`/`false`) | `integer` |
| TS/TSX/JS | `+ - * / < <= > >= == != === !== && \|\| ??(→\|\|)` + `?.` | `-`, `!` (unary_expression) | `true false` | `number` |
| Go | `+ - * < <= > >= == != && \|\|` (`/` never; `*→/` only) | `-`, `!` | `true false` | `int_literal` |
| Java | `+ - * / < <= > >= == != && \|\|` | `-`, `!` | `true false` | `decimal_integer_literal` |
| Rust | `+ - * / < <= > >= == != && \|\|` | `-`, `!` (unary_expression) | `true false` (leaf in boolean_literal) | `integer_literal` |
| Clojure | head symbols (§2.5) | – | `bool_lit` true/false anywhere | `num_lit` 0/1 anywhere |

Disambiguations tree-sitter makes, which a port must reproduce **[probe]**:
- Python `*args`, `**kw`, `[*a]`, `print(*a)`, `def f(*, a)`, `lambda *a` → no site. `a ** 1` → no op site.
- TS: `foo<number>(1)`, `a<b>(c)` in `.ts/.tsx` = generic call (no `<`/`>` sites). In `.js`, `a<b>(c)` = two comparisons. `<number>a` type assertion: nothing. `a!` non-null: no site (`a! > b!` → only `>`). Regex `/a-1/` and `/a+1/g`: no sites. Division `a / b / 1`: both `/`.
- TS type exclusions (all ancestor-based): `let t: 0 | 1 = 1` → only the value `1`. `as 1`, `as {ok: true}`, `satisfies {ok:false}`, `as [0,1]`, `as Array<1>`, conditional types, `<T = 1>` defaults, return types `: {ok: true}`, `Promise<{done:false}>`, interfaces, type aliases → nothing.
- Java: `List<Integer>`, `new ArrayList<>()`, `<T extends Comparable<T>>` → nothing. `g(a < b, c > d)` → two comparisons. `(int) -y` → unary delete. **`(Foo) -y` → binary `- → +`** (tree-sitter can't tell a reference-type cast). `(y) - 1` → binary.
- Rust: `&&a`, `&&i32`, `&a`, `*b`, `&1` → nothing. `|| a > 0` closure: `||` not a site. `|x| x || false` → `||` site. `-> !`, `macro!` bang → nothing. `a as i64 - 1` → binary `-`.
- Go: `<-ch`, `&a`, `*ptr`, `ch <- 1` → no op sites (ints still count).
- Python PEP 701 nested same-quote f-strings (`f"{f"{x}"}"`) **misparse** in this grammar: later string contents (`rb"1"`, `"""a > 1"""`) leak `1` sites and real `+` ops are lost. Exact parity would mean reproducing a grammar bug. Recommend not doing so (document the divergence).
- Syntax errors: tree-sitter recovers with ERROR nodes. Leaves under ERROR still yield boolean/constant sites, and binary only if the parent is still a binary node. A port can't match this; skip or fall back.

### 2.3 Anchor line (multi-line statement rule)
`anchor_line` = start line of the **nearest strict ancestor** whose type ends with `_statement` or `_declaration`; if there is none, the site's own line. Used only for coverage (§4). Observed consequences **[probe]**:
- TS: object literal value on line 7 inside `return {` (line 6) → 6. Chained call `.bind(now + 1)` on line 4 of the `db…` expression_statement starting line 2 → 2. `switch (a) { case 1:` → the switch_statement line. `else if (` → the inner if line. Class field initialisers, and default params of methods → the **class_declaration** line (method_definition / public_field_definition are not statements). Default params of a top-level function → its function_declaration line. `const D = (a) => …` body → lexical_declaration line.
- Python: `elif` condition → the **`if`** line (elif_clause is not a statement). `case` patterns/guards → the `match` line. Multi-line `for … in range(0,\n 1)` → `for` line. Top-level/method def default params: no statement ancestor → own line.
- Java: `case 1:` label → the **method_declaration** line (switch_expression / switch_block_statement_group are not statements). `for (…;\n i < 1; …)` → `for` line. Method params → method line.
- Rust: the block tail of `if a > 0 {\n 1 } else {\n 0 }` at function tail → the `if` line (tree-sitter wraps a block-like tail expression in expression_statement). `match` arms → the match's statement line (`let x = match …` → let line). A plain tail expression (`x`) → own line.
- Go: statement nodes via tree-sitter-go (`*_statement`, `short_var_declaration`, `var/const/type_declaration`, `function/method_declaration`). Multi-line `if a > 1 ||\n b < 0 {` → `if` line.
- Clojure: no statement nodes → always its own line.

### 2.4 Site ownership and identity (`functions.sites_in_file`)
- `functions = crapper.languages.functions_in_file(language, source, abs_posix_path, abs_posix_root)`. Mutator reads `name, namespace, start_line, end_line, start_byte, end_byte`. Byte spans are set **only for TypeScript** (others `-1`).
- Owner of a raw site (`line`, `start_byte`): the functions with `start_line ≤ line ≤ end_line` and (if both have byte spans) `start_byte ≤ site.start < end_byte`. Pick the min by `(tightness, start_line)`, where tightness = `end_byte-start_byte` if there are byte spans, else `(end_line-start_line)*1_000_000`. Ties: first in crapper's order. **No owner → site dropped**: top-level code, class fields, enum values, constructors (Java), TS `#private()` methods (crapper looks for `property_identifier`/`identifier` only and skips them), `var f = function(){}` (only `lexical_declaration` arrows count), anonymous default exports, Rust `mod tests`, Python decorators of top-level defs.
- Nested functions that crapper does not list (Python inner defs, TS callbacks, Java anonymous/local classes, Rust fns in blocks) belong to the enclosing function. TS Express route callbacks are separate entries (`GET /users`).
- `form_id = "defn-/<name>"` if private else `"defn/<name>"` (§7.2).
- **mutation_id (outcomes key)** = Python `json.dumps([file_key, namespace, form_id, start_byte, end_byte, original, mutant], separators=(",",":"))`.
  - The numbers are **UTF-8 byte offsets into the whole file** (end exclusive), *not* line/col. README's `…,10,11,">",">="` = bytes 10..11. **[probe]**: a `-` after `"é"` sits at byte 101 (char 100).
  - `file_key` = path relative to root, POSIX.
  - JSON encoding must match Python's defaults exactly: `ensure_ascii=True`. Escape `"`→`\"`, `\`→`\\`, `\n \r \t \b \f` as two-char escapes, other `<0x20` and every char outside `0x20..0x7E` (including DEL) → `\uXXXX` with **lowercase** hex, non-BMP → UTF-16 surrogate pair. No HTML escaping (`< > &` stay literal; Go's `encoding/json` would turn them into `\u003c` — **do not use it for encoding**). **[probe]**: `["src/caf/\u00e9<&.py","caf.\u00e9<&","defn/g",23,24,">",">="]`.
  - Decoding keys (`mutation_file` = elem 0, `mutation_namespace` = elem 1) is a real JSON decode, so `\u00e9` compares equal to `é`. Elements that are not strings, or invalid JSON → None.
  - **Consequence:** ids include byte offsets, so any edit above a function shifts the ids of everything below it. Those sites lose their carried outcomes and get re-run, even though their function digest is unchanged.

### 2.5 Clojure (brief)
- Skip a node if it, or any ancestor, has a type containing `comment`, `quot` (quoting_lit, syn_quoting_lit, var_quoting_lit, unquoting_lit, unquote_splicing_lit), `string`, `str_lit` or `char_lit`, or if an ancestor is a `list_lit` whose head is `comment` or `quote`.
- Head = the **first `sym_lit` child** of a `list_lit` (not necessarily the first child). Head mutations (orig → mutant, category):
  `+ -`, `- +`, `* /` (one-way), `inc dec`, `dec inc`, `> >=`, `>= >`, `< <=`, `<= <`, `= not=`, `not= =`, `if if-not`, `if-not if`, `when when-not`, `when-not when`, `and or`, `or and`, `double int`, `int double`, `first second`, `second first`, `filter remove`, `remove filter`, `take drop`, `drop take`, `rest next`, `next rest`, `every? some`, `some every?`, `min max`, `max min`, `pos? neg?`, `neg? pos?`, `even? odd?`, `odd? even?`, `nil? some?`, `some? nil?`. Match on the full symbol text (`clojure.core/+` is not a match).
- `bool_lit` true/false and `num_lit` exactly `0`/`1` are sites anywhere (vectors, maps, metadata, sets).
- Quirks **[probe]**: `#(+ % 1)` → `+` NOT a site (anon_fn_lit isn't list_lit) but `1` is. `#_(+ 1 0)` discard IS mutated. `(1 + 2)` → `+` treated as head. `((comp f) inc)` → `inc` treated as head. `(-> x inc)` → `inc` not a head. `1N 1.0 0x1` are not sites.

---

## 3. Mutation application
- One mutant at a time per worker: `mutated = bytes[:start] + mutant.utf8 + bytes[end:]`, written over the worker's private copy of the file (UTF-8). After the run, the original bytes are written back (in `finally`).
- Before applying, check `original[start:end] == site.original`. If not → stderr "source bytes moved", status `survived`.
- No compile step. A mutant that doesn't compile makes the test command fail, so it is **killed**.
- Files that are not valid UTF-8 → `Skipping <key>: file is not UTF-8` (mutate mode).

---

## 4. Coverage gating

### 4.1 Generation (`cli._prepare_coverage`)
- Skipped when `--no-coverage`, `--use-existing-coverage` or `--scan`.
- `--coverage-command`: run it through the mutator CommandRunner (shell, cwd=root, no timeout, output **captured, not streamed**, prints `+ …` only with `--verbose`). If rc≠0: stderr `Coverage command exited <rc>. Reports already on disk will not be read.` + tail 20 lines, then **exit rc**.
- Otherwise `crapper.runners.run_coverage(root, selected_files, None)`: per-language tools for the languages present in the selected files, always returning 0 (see spec-crapper). Reports: LCOV in `target/coverage/<lang>[/<slug>]/lcov.info`, Go profile in `<module>/target/coverage/go/coverage.out`, JaCoCo in `<module>/target/site/jacoco/jacoco.xml`.

### 4.2 Lookup (`coverage.covered_lines(root, abs_path, language)`) → `set[int] | None`
- java: index all `root/target/site/jacoco/jacoco.xml`, `root/*/target/site/jacoco/jacoco.xml`, `root/*/*/target/…` (dedupe by real path; strip `<!DOCTYPE…>` case-insensitively; skip parse errors). Key = `<package name stripped of "/">/<sourcefile name>` (or just the filename). Later files overwrite. A line is hit if `ci>0 or cb>0` (non-int attr → line ignored).
- others: `crapper.coverage.load_bundle(root)`:
  - go: segments `(start_line, end_line, stmts, hits)`. Lines `start..end` inclusive of every segment with `hits>0`. A file in the profile with no hits → `set()` (not None). An invalid profile line raises → crash.
  - clojure: first try Cloverage HTML (`target/coverage/**/*.html` containing "forms covered", key = path relative to `target/coverage` minus `.html`, line → `(covered,total)` from `<span title="C out of T forms covered">L&nbsp;`, T>0). Hit if `covered>0`. If no HTML match, fall back to LCOV.
  - python/typescript/rust/(clojure fallback): LCOV from `root/target/coverage/**/lcov.info` + `root/coverage/**/lcov.info` (merged with dict.update, last wins). Only `DA:<line>,<hits>` records count; hit if hits>0. `BRDA` ignored by mutator.
- Path matching (mutator's own `_lookup`, **not** crapper's `_select_key`):
  1. `normalize`: URL-unquote, `\`→`/`, strip the `file:` prefix, strip leading `./` repeatedly, collapse `//`.
  2. Exact: a normalized key equals the normalized absolute source path.
  3. Suffix rank over `/`-split non-empty parts. If the key ends with the source parts → rank `(0, extra)`. Else, if the source is strictly longer and ends with the key → `(1, missing)`. Keep the minimum rank. If the keys at that rank (deduped by parts) number ≠1 → None. Effectively, the relative report key that is the longest suffix of the absolute path wins, and `b/app.py` never steals `a/b/app.py`.
- **UPSTREAM BUG [probe]**: Go profile keys are import paths (`example.com/ubfix/calc.go`). They are only a suffix of the absolute path when the directory tree mirrors the module path. For any ordinary module (`module example.com/x`) the lookup returns None, so every Go site is **UNCOVERED** and nothing runs unless `--no-coverage` is given. The repo's `fixtures/go` (`example.com/ubfix`) hits this. Decide: replicate (exact parity) or map the import path → file through `go.mod` (recommended, behind a parity flag).

### 4.3 Site coverage
- `--no-coverage` → every site covered.
- lookup None → every site uncovered + stderr warning (if the file has sites).
- else covered iff `site.line ∈ lines or site.anchor_line ∈ lines`.
- Uncovered sites are never run or selected. They count toward `:uncovered` and `:sites` and show as `UNCOVERED` in the site log.

---

## 5. Test execution

### 5.1 Test plan (`runner.test_plan(root, source, language, override)`)
`--test-command` → `(string, root)` run via `/bin/sh -c`. Otherwise argv lists. `nearest(marker)` walks from `source.parent` up to root inclusive. The markers are tried **in order**, each over the full walk. No marker → root.
| Lang | markers (cwd) | argv |
|---|---|---|
| clojure | `deps.edn`, then `bb.edn` | bb.edn and no deps.edn: `bb spec --tag ~no-mutate` if `"spec"` ⊂ bb.edn text, else `bb test`. deps.edn: `clj -M:spec --tag ~no-mutate` if `":spec"` and `"speclj"` ⊂ text. `clj -M:spec` if only `":spec"`. Else `clj -M:test` |
| java | `pom.xml` | `mvn -q test -DexcludeTags=no-mutate` |
| go | `go.mod` | `go test -count=1 <pkg>`, where pkg = `.` or `./<rel dir of source from the go.mod dir>` (one argv element even with spaces/`;`) |
| typescript | `package.json` | `npm test` |
| rust | `Cargo.toml` | `cargo test` |
| python | `pyproject.toml`, `pytest.ini`, `setup.cfg` | `<py> -m pytest` if `pytest.ini` or `conftest.py` exists in the dir, or `"pytest"` ⊂ pyproject.toml text, or setup.cfg exists with `"pytest"` ⊂ text. Else `<py> -m unittest discover`. `<py>` = `<dir>/.venv/bin/python`, then `<dir>/venv/bin/python` (absolute, symlink **not** resolved, must be a file and executable). Else mutator's own `sys.executable` (Go port: pick `python3` on PATH and document it) |
| other | – | `["false"]` |

### 5.2 CommandRunner.run(command, cwd, timeout)
- env = copy of os.environ + `PYTHONDONTWRITEBYTECODE=1`. If cwd is inside a worker (some ancestor-or-self is `worker-*` with parent `run-*` and grandparent `mutation-workers`): `PYTHONPATH = join(os.pathsep, existing dirs of [cwd/src, cwd, worker/src, worker] deduped) + existing PYTHONPATH`. Applies to every language.
- New session / process group. stdout+stderr merged, captured, text.
- Spawn OSError → `code=127, output=str(exc)`. returncode None → 1.
- Timeout → `killpg(SIGKILL)`, drain, `code=124, timed_out=True`.
- `seconds` = monotonic wall time.

### 5.3 Baseline and classification
- The baseline runs only when the file has ≥1 selected site, in the **real tree** at the plan cwd, without a timeout. It is cached across files by key `(("sh",cmd) | ("argv",*cmd), str(cwd))`. A cached failure fails every later file with the same key.
- Failure (rc≠0) → no mutants, no snapshot write for that file, stderr message (§1.6), exit 2 at the end.
- Mutant timeout = `max(2.0, baseline_seconds × timeout_factor)` (test: 30 s × 10 → 300).
- Outcome: `timed_out or rc≠0` → `killed`, else `survived`. **There is no separate timeout status** (no `:timeout` in snapshots).

---

## 6. Workers (`workers.py`)

- Count = `max(1, min(n_selected, cpu_count, max_workers or cpu_count))`. Files are processed one at a time. Only the mutants of one file run in parallel.
- `base = root/target/mutation-workers/run-<uuid4>/`. Workers `worker-0..N-1`. One thread per worker pulls sites from a FIFO queue (site order). Outcomes are written under a lock. After the file finishes, `delete_tree(base)` runs (unlink symlinks, never follow them). `target/mutation-workers/` itself is left behind (empty).
- `importers = importers_of(root, rel)` is computed once per file, **for every language**:
  - Walk root (prune `SKIP_LINK` ∪ `{node_modules}`; test dirs are included). For each file with ext ∈ `.ts .tsx .mts .cts .js .jsx .mjs .cjs`, match regex `(?:from|import|require)\s*\(?\s*['"](\.[^'"]+)['"]` and strip `?…` / `#…`.
  - Resolution candidates, in order: `dir/spec`, `stem+ext` for each ext, `stem/index+ext` for each ext (stem = spec minus a known ext). A candidate must resolve inside root and be a file.
  - Build the reverse graph. Take the transitive closure of importers of the target (excluding the target itself).
- Per worker:
  1. Copy (real bytes) each root-level file in `CONFIGS` that exists, unless it is the mutated file: `deps.edn bb.edn pom.xml build.gradle build.gradle.kts settings.gradle go.mod go.sum package.json package-lock.json pnpm-lock.yaml yarn.lock tsconfig.json Cargo.toml Cargo.lock pyproject.toml pytest.ini setup.cfg setup.py conftest.py requirements.txt Pipfile poetry.lock`.
  2. Symlink every root child except the first path segment of `rel`, names in `SKIP_LINK`, and names that already exist. Link target = the child's **resolved absolute** path. `SKIP_LINK = {.git .hg .svn .idea .venv venv target dist build out coverage .metrics __pycache__ .pytest_cache .mypy_cache .clj-kondo .gradle}`. `node_modules` IS linked.
  3. Overlay: every directory along `rel` is a real dir. Its children are symlinked except the next segment and SKIP_LINK names (**at every depth**: e.g. a `src/build/` sibling is not linked). The file itself is written as a private copy.
  4. For each importer: walk its dir segments. A symlinked dir is "expanded" (unlink, mkdir, symlink its children except SKIP_LINK). A missing dir is created. If the destination is a symlink it is replaced by a real copy. An existing real file is left alone.
- Mutant cwd = `worker / relpath(plan_cwd, root)`, then `resolve()` (or the worker itself if the cwd is outside root). The test command is the same as the baseline's.
- Implications: `.venv` is absent in workers (hence the absolute interpreter). Rust/Java `target` is absent, so each worker builds from scratch.

### 6.1 Backups (legacy)
- `restore_backups(root)`: runs before coverage, only in non-scan runs that have ≥1 selected file. For every file under `root/target/mutator-backup/` (sorted): write its bytes to `root/<rel>` (mkdir parents), unlink the backup, stderr `Restored <abs> from an interrupted mutation.` Empty dirs remain.
- The current code **never creates backups**. `engine._backup` and `engine._drop_bytecode` are dead code (only tests call them). `_restore_if_dirty(path, original)` rewrites the source if it changed after the run (defensive). The port needs only restore + restore-if-dirty.

---

## 7. Selection, forms, snapshot

### 7.1 Differential selection (`engine._select` / `_carry_forward`)
Digest per form key `(namespace, form_id)` (`functions.form_digests`):
```
lines   = source.splitlines()        # Python semantics: splits on \n \r\n \r \v \f \x1c \x1d \x1e \x85 \u2028 \u2029; no trailing empty line
chunk_i = "\n".join(l.rstrip() for l in lines[start-1:end])   # rstrip = Python str.isspace set (Go unicode.IsSpace PLUS \x1c-\x1f)
d_i     = sha256(chunk_i.utf8).hexdigest()
fns of the same key sorted by (start_line, end_line)
:hash   = sha256("".join(d_i).encode()).hexdigest()           # double-hashed even for a single function
```
(Line numbers come from tree-sitter, which counts `\n` only. A lone `\r`, `\f`, etc. desynchronises the window. Replicate it.) **[probe]**: `def add…` (lines 1–4) → `1a13fb93…5b89` = sha256(sha256hex(chunk)).

History (`metrics.load_history(root, file_key)`): read every `*.edn` under `.metrics/mutate` (recursive, sorted, unreadable/unparseable/non-map skipped, `:namespace` must be a string).
- forms: `(ns, :id) → :hash` for forms whose `:file` (or the snapshot's `:source` if `:file` is missing or empty) == file_key, with string id and hash.
- outcomes: keys whose JSON elem 0 == file_key and value ∈ {killed, survived} (keyword, or string with or without `:`).

For each site, in order:
1. not covered → skip.
2. `--lines` given and `site.line ∉ lines` → skip.
3. `unchanged = history has the form key and prior digest == current digest`.
4. `--mutate-all or not unchanged` → select.
5. else if history outcome for this exact mutation_id == `killed` → skip (carried). Otherwise select (survivors and unseen ids are re-run).

Carried outcomes = history outcomes (killed **and** survived) whose id matches a current covered site in an unchanged form. Re-run results overwrite them.

### 7.2 Privacy (`functions.is_private`)
`header` = function text from start_line to `min(end_line, start_line+6)` (≤7 lines), lstripped, with leading `#[…]` groups removed (each up to the first `]`).
| Lang | private iff |
|---|---|
| clojure | header matches `^\(defn-` |
| java | `\bprivate\b` anywhere in header (quirk: also in body text, comments or strings within the first 7 lines) |
| go | `name[0].islower()` (Unicode lower. `_x` → public, `init`/`main` → private) |
| python | starts with `_` and is not a dunder (`__x__`). `__x` → private |
| rust | header does NOT match `^pub\s` (quirk: `pub(crate) fn` → private) |
| typescript | name starts with `#` (never happens, crapper drops those) or `\bprivate\b` in header (same body quirk) |
`form_spans`: per key `(start=min, end=max, name, private of the first fn)`.

### 7.3 Form results (`engine._forms`)
Forms are ordered by span start line (stable; ties keep crapper order). One form per key, **including forms with zero sites**. For each site owned by the key:
- uncovered → `uncovered+=1`
- outcome killed → `killed+=1`
- outcome survived → `survived+=1`
- no outcome: `survived+=1` if `--lines` was not given, else **not counted**
`sites` = number of owned sites, counting uncovered ones. So `killed+survived+uncovered == sites` except with `--lines`.
Exit 3 triggers on any `survived>0` in the result forms (so carried survivors under `--lines` count).

### 7.4 Snapshot serialisation (`edn.dumps`), path `.metrics/mutate/<ns>.edn`
- Path: `ns.replace("\\","/").replace("::","/").split("/")`. Any part ∈ {"", ".", ".."} → ValueError (crash). Last part + `.edn`, earlier parts are dirs. Examples: `demo.board.Board` → `demo.board.Board.edn`; `example.com/demo.Widget` → `example.com/demo.Widget.edn`; `crate::foo::Bar` → `crate/foo/Bar.edn`.
- One line + `"\n"`. Top-level key order: `:version 1 :tested-at "<iso>" :source "<file_key>" :namespace "<ns>" :outcomes {…} :forms […]`.
- `:tested-at` = local time `YYYY-MM-DDTHH:MM:SS±HH:MM` (UTC → `+00:00`; Go layout `2006-01-02T15:04:05-07:00`).
- Outcomes map: entries sorted by key string. Key = EDN string of the mutation_id. Value = keyword `:killed` / `:survived`.
- Fresh form map key order: `:id :kind :file :line :end-line :hash :killed :survived :uncovered :sites` (`:kind` = `"defn-"` if private else `"defn"`).
- Encoding: map = `{` + `k v` pairs joined by single spaces + `}`. Vector = `[` items joined by space `]` (empty `[]`, empty map `{}`). A dict key matching `[A-Za-z*+!_?][A-Za-z0-9*+!_?\-./]*` → `:key`, otherwise an EDN string. Strings `"…"` escape only `\`→`\\` and `"`→`\"` (newlines stay raw). Ints are decimal. Bool `true/false`. None `nil`. Float `%.4f` with trailing zeros and `.` stripped, `.0` added if no dot. Sets are not renderable (TypeError).
- Real example **[probe]**:
```
{:version 1 :tested-at "2026-11-03T12:59:57-03:00" :source "src/demo.py" :namespace "demo" :outcomes {"[\"src/demo.py\",\"demo\",\"defn-/_h\",101,102,\"-\",\"\"]" :survived "[\"src/demo.py\",\"demo\",\"defn/add\",24,25,\">\",\">=\"]" :survived …} :forms [{:id "defn/add" :kind "defn" :file "src/demo.py" :line 1 :end-line 4 :hash "1a13fb93…" :killed 1 :survived 3 :uncovered 0 :sites 4} {:id "defn-/_h" :kind "defn-" …}]}
```
- The EDN reader (needed for history/merge) accepts: whitespace and `,`, `;` comments, strings (escapes `\n \t \r \" \\`; any other `\x` → `x`, so `\\u00e9` in a file reads back as `\u00e9`), keywords → bare name string, maps, vectors, lists, `#{}` sets (any other `#` → error), atoms `nil true false`, `-?\d+` int, `-?\d+\.\d+` float, otherwise a symbol string. Errors: `empty EDN`, `unexpected end of EDN`, `unterminated string`, `unterminated string escape`, `empty keyword`, `trailing EDN at N`, `unsupported EDN dispatch`. On any error the file is skipped.

### 7.5 Writing and merging (`metrics.write_results(root, file_key, forms, outcomes)`)
1. `incoming` = fresh forms grouped by namespace. `incoming_outcomes` = outcomes with elem0 == file_key and status ∈ {killed, survived}.
2. `existing` = every readable map snapshot under `.metrics/mutate`.
3. For each existing snapshot with a string `:namespace`:
   - "mentions file" (any form whose file == file_key) → `previous += ns`.
   - kept_forms = forms whose file ≠ file_key. kept_outcomes = outcomes whose elem0 ≠ file_key.
   - If either is non-empty, accumulate into `retained[ns]` (several files with the same ns merge: forms are appended, outcomes dict-updated).
4. `touched = incoming ∪ previous ∪ retained` → so **every snapshot with content from other files is rewritten too**, with a new `:tested-at` and kept values normalised (keyword values become strings).
5. For each ns in `sorted(touched)`:
   - rendered = dedupe(kept_forms + fresh rendered) by `(file, id, line, hash)`, first wins. Then sort by `(str(file), int(line or 0), str(id))` (missing file → `"None"`).
   - merged outcomes = kept_outcomes, plus (only if ns has fresh forms) this file's outcomes whose elem1 == ns.
   - If both rendered and merged are empty → delete the canonical file.
   - Else write. `:source` = file_key if there are fresh forms, else `kept_forms[0]["file"] or file_key` (pre-sort order). Latent crash: kept outcomes but zero kept forms → IndexError.
6. Drop stale: for each existing snapshot whose path ∉ canonical paths of touched: delete if its ns ∈ touched (folded legacy/duplicate), or if all its forms' files == {file_key} (or it has no forms and `:source == file_key`).
7. Remove empty dirs under `.metrics/mutate` (deepest first).
- **Not rewritten when**: `--scan`, baseline failure (for that file), skipped file (unsupported or non-UTF-8). Untouched: snapshots with no forms and no outcomes from other files (e.g. `{:namespace "untouched" :forms [] :outcomes {}}`).
- Two files in one namespace (same Go package): the second write keeps the first file's forms and outcomes. Rerunning file A replaces only A's forms (test_metrics).

---

## 8. Feasibility for a stdlib-only Go port

Sites only need leaves plus (a) parent-kind classification and (b) the nearest statement-start ancestor. A token stream suffices wherever (a) can be decided from neighbouring tokens plus a small context stack. Function spans/namespaces need structural parsing anyway (crapper port), so share one lexer/parser per language with crapper.

| Language | Approach | Parity |
|---|---|---|
| **Go** | `go/parser` + `go/ast` (stdlib, allowed). BinaryExpr ops `ADD SUB MUL GTR GEQ LSS LEQ EQL NEQ LAND LOR` (not QUO). UnaryExpr `SUB`/`NOT` → delete. BasicLit INT `"0"`/`"1"`. Ident `true`/`false` in expression position (not selector `.Sel`, not decl names). Anchor = nearest enclosing `ast.Stmt` (except BlockStmt, CaseClause, CommClause) or Decl/GenDecl/FuncDecl start line. `DeclStmt` = GenDecl pos. `RangeStmt` = for line. | **exact** (except files with syntax errors, which go/parser rejects and tree-sitter recovers) |
| **Python** | Hand tokenizer (PEP 263/UTF-8; strings incl. prefixes `rbuf`, triple quotes, f-strings with nested replacement fields, stopping at top-level `!conv`/`:spec`; implicit line joining in brackets; backslash continuation; INDENT/DEDENT). Classify by previous significant token. `-`/`*`/`+` binary iff prev ends an operand (NAME that is not a keyword, NUMBER, STRING, `)` `]` `}`, `True/False/None`); else unary. `not` site iff not followed by `in` and not preceded by `is`. `match`/`case` soft keywords: inside a case pattern (`case` … top-level `:` or `if` guard) `-` is not a site, but ints/bools are. Anchors: statement stack (simple statement = logical-line start; compound header = keyword line; `elif/else/except/finally/case` → parent compound's line; def/class headers → enclosing statement or own line). | **near** (deviates on PEP 701 nested-quote f-strings, where tree-sitter is wrong, on Python-2 syntax, and on error recovery) |
| **TypeScript / TSX / JavaScript** | Needs a real recursive-descent parser for expressions plus a **type skipper**. Lexer: regex-vs-`/` by previous token (with `)`/`}` context), template literals with nested `${}`, JSX mode (`.tsx`, `.jsx`, **and `.js/.mjs/.cjs`**), hashbang, numeric separators/bigint. Type contexts to skip (no sites at all): `:` annotations on params/vars/fields/returns (vs ternary/object/label/case colons), `as`/`satisfies` + type, `type X = …`, `interface`, `implements`/`extends` type args, `<…>` type params on functions/classes/arrows, and **call-site type args** `f<T>(…)` (TS only; JS treats them as comparisons), `<T>expr` assertions (`.ts` only). `?.` → next token `(`/`[`/template → `""`, else `.` (`?.` followed by a digit is ternary + number). `true`/`false` after `.`/`?.` or as an object key (`{`/`,` before, `:` after, not in ternary) → not a site. `!` postfix (non-null: previous token ends an operand) → not a site. Anchors need statement/declaration nesting: switch-case → switch line, class fields & method params → class line. | **approximate → near**. Most risk is here |
| **Java** | Token classifier + generics heuristic: `<` after a type-ish identifier, closed by a matching `>`/`>>`/`>>>` with only type tokens between (`ident . , ? extends super & [] @ann` and nested `<>`), → not an operator. Unary vs binary by previous token. Cast rule: `(primitive) -x` → unary; `(Ident) -x` → **binary** (match tree-sitter). Text blocks, char literals. Anchors: tree-sitter-java `*_statement`/`*_declaration` (`local_variable_declaration`, `method_declaration`, `class_declaration`…); switch labels → method line. Note `switch` is `switch_expression` (not a statement). | **near** |
| **Rust** | Token classifier with modes: nested block comments, raw strings `r#".."#`, byte/char vs lifetime (`'a'` vs `'a`), macro token trees after `ident!` and attribute token trees `#[..]`/`#![..]` (inside them only int 0/1 and true/false are sites), `&&`/`&` reference vs logical (binary iff prev ends an operand), `\|\|` closure vs or (same rule), `*` deref, `!` macro/never-type/`impl !Trait` vs unary, negative-literal patterns (match arm patterns before `=>`, `let`/`if let`/`while let` patterns, param patterns: `-` not a site), generics `<…>` in type contexts (after `:`, `->`, `as`, `impl`, `fn name<`, `struct/enum/trait/type` headers, `where`) and turbofish `::<` vs comparisons, `1..2`/`1..=2` ranges, tuple index `.0`/`.1` **is** a site, `1.0` vs `1.`. Anchors: `expression_statement` (incl. tail block-like `if/match/loop/{}`), `let_declaration`, `use_declaration`; else own line. | **near** |
| **Clojure** | Hand reader (lists/vectors/maps/sets, `#()`, `#_`, `'`, `` ` ``, `~`, `~@`, `#'`, `^meta`, strings, chars, regex, comments). Replicate quirks (§2.5). | **exact** (small grammar) |

Is token-level enough? Go: use go/ast, not tokens. Python: yes (tokens + prev-token + case/statement stack). Java & Rust: tokens + bracket/context stacks (generic heuristics). TS/JS: **no**, type-position exclusion and JSX need structure.

Other parity hazards:
- JSON key encoder (§2.4) and EDN escaping are needed for byte-identical `:outcomes` keys. Without them, a snapshot written by the Python tool and read by the port mismatches ids for non-ASCII or `<>&` content and re-runs everything.
- `splitlines` / `rstrip` character sets (§7.1) for `:hash` parity.
- Width/len in the report use code points (`utf8.RuneCountInString`).
- Go coverage bug (§4.2) changes the whole Go outcome. The harness must either pass `--no-coverage` for Go or pin the decision.
- Every write rewrites all snapshots (`:tested-at` changes in unrelated files). Harness diffs should ignore `:tested-at`.

---

## 9. Test cases to port (from `tests/*.py`; plus [probe] goldens)

Notation: `file :: source → expected`. Pairs are `(orig, mutant)`. Line numbers are 1-based.

### 9.1 Sites (`test_sites.py`)
1. `src/demo/core.clj` :: `(ns demo.core)\n(defn place [x]\n  (if (> x 0)\n    (+ x 1)\n    0))\n(defn- hide []\n  false)\n` → includes `(defn/place,if,if-not)`, `(defn/place,>,>=)`, `(defn/place,+,-)`, `(defn/place,0,1)`, `(defn/place,1,0)`, `(defn-/hide,false,true)`. All ns `demo.core`. `is_private(clojure,"hide",src,6,7)` true.
2. clj :: `(ns demo.core)\n(defn place []\n  (comment (+ 1 0))\n  (quote (+ 1 0))\n  (str "true + 1"))\n` → **no sites**.
3. `src/demo/Board.java` :: `package demo;\nclass Board {\n  int place(int x) {\n    if (x > 0 && ready)\n      return x + 1;\n    return !ok ? -x : 0;\n  }\n  private int hide() { return 1; }\n}\n` → place has `(+,-)`, `(>,>=)`, `(&&,||)`, `(!,"")`, `(-,"")`, `(0,1)`, and no `*`/`/` pairs. hide = `defn-/hide` with only `1`. ns `demo.Board`.
4. `src/demo/Math.java` :: `…int place(int a, int b) { return a * b + a / b; }…` → `(*,/)`, `(/,*)`, `(+,-)`.
5. `demo.go` :: `package demo\nfunc Run(a int, b int) int {\n    if a > 0 && b == 1 {\n        return a * b\n    }\n    return a / b\n}\ntype Widget struct{}\nfunc (w Widget) Run() bool { return true }\n` → ns `demo`: `(*,/)`, no `(/,*)`, `(&&,||)`, `(==,!=)`, `(>,>=)`, `(0,1)`, `(1,0)`, form `defn/Run`. ns `demo.Widget`: form `defn/Run`, `(true,false)`.
6. `demo.go` :: `package demo\nfunc hide() bool { return false }\n` → first site form `defn-/hide`.
7. `src/demo/app.py` :: `def place(x):\n    if x > 0 and ready:\n        return x + 1\n    if not ok:\n        return 0\n` → has `(and,or)`, `(not,"")`. ns `demo.app`.
   `src/demo/app.ts` :: `export function place(x: number) {\n  if (x > 0 && ready) return x + 1;\n  return true === false;\n}\n` → `(===,!==)`, `(&&,||)`. ns `demo.app`.
   `src/lib.rs` :: `fn place(x: i32) -> i32 {\n    if x > 0 && ready { return x + 1; }\n    if !ok { return -x; }\n    0\n}\n` → `(&&,||)`, `(!,"")`, `(-,"")`.
8. `src/demo/board.py` :: `class Board:\n    def place(self, x):\n        return x > 0\n    def _hide(self):\n        return False\n` → place `defn/place`, `_hide` `defn-/_hide`, ns `demo.board.Board`.
9. py :: `def place():\n    return "x > 0 and true"\n` → none.
10. `src/demo/app.ts` :: `\nexport function place(a, b) {\n  return a ?? b?.c ?? b?.[0] ?? b?.();\n}\n` → exactly 3× `(??,||)`. The `?.` mutants are {`.`, `""`}.
11. `src/demo/routes.js` :: `\nexport function mount(app) {\n  app.get("/users", (req, res) => {\n    return req.body ?? false;\n  });\n}\n` → `("GET /users", ??, ||)` present, no site named `mount`, ns `demo.routes`.
12. ts :: `export function place() {\n  return "a ?? b?.c";\n}\n` → none.
13. Deep: `def place():\n    return 1 + 1 + … (1200 terms)` and the Go equivalent → non-empty, no stack overflow.
14. `src/demo/check.ts` :: `type Flag = { on: true };\ninterface Box { open: false }\nexport function check(n: number): { ok: true } | { ok: false; used?: true } {\n  const seen: Promise<{ done: false }> | null = null;\n  return { ok: n > 0 };\n}\n` → **exactly** `{(5,>,>=), (5,0,1)}`.
15. `src/demo/save.ts` :: `export function save(db: Db, now: number) {\n  db\n    .prepare("x")\n    .bind(now + 1)\n    .run();\n  return {\n    ok: true,\n  };\n}\n` → anchors `(4,+,2)`, `(4,1,2)`, `(7,true,6)`.

### 9.2 Boundaries (`test_boundaries.py`, site/format parts)
- Clojure `[inc]` → no `inc`. `(+ + 1)` → exactly one `(+,-)`. `2` → none.
- py `return a - b` → `(-,+)`, not `(-,"")`. `def place(true): return true` → no `(true,false)`. `return 2` → none.
- rs `fn place() -> bool { true }` → exactly one `(true,false)`.
- Privacy: py `__init__`→F, `_hide`→T, `__x`→T. rust `fn place() {}`→T, `#[inline]\npub fn place() {}`→F. ts `#place() {}`→T, `private place() {}`→T, `function place() {}`→F. unknown lang → F.
- `digest("def place(x):\n    return x > 0\n",1,2) == sha256("def place(x):\n    return x > 0")`.
- `_owner([outer 1–10, inner 8–9], 8) == inner`.
- Mutation field parsing: `not-json`/`"hello"`/`[1]`→None for idx0. `["only"]` idx1→None.
- Description: `> -> >=`, `delete not`. Score: 1/2→0.5, 0/0→None.
- Report: total row `Total 1 1 0 2 50.0%`, `100.0%`, `n/a`. Empty → `No functions to mutate.\n`. Scan ` uncovered` only if the map has the id and it is False.
- Scan of `def add(a, b):\n    return (a\n            + b)\n` with coverage {2} → no `uncovered`, contains `+ -> -`.

### 9.3 Engine / workers (`test_engine.py`, boundaries)
Fake runner: exec `src/demo.py` in the cwd and check `add(1,2)==3`. Source `def add(a, b):\n    if a > 0:\n        return a + b\n    return a + b\n`:
- `--no-coverage --mutate-all` → form `defn/add` ns `demo`: killed 1, survived 3, sites 4. Source unchanged. Snapshot has the same values.
- Rerun without mutate-all → same counts. Runner calls = 1 baseline + 3 survivors.
- coverage `set()` → 0 runner calls, uncovered 4, sites 4.
- `def add(a, b):\n    return (a\n            + b)\n` with coverage {2} → uncovered 0, killed 1.
- Failing baseline → `baseline_failed`, no snapshot. Message has the last 20 output lines (`M5` present, `START` absent of 25).
- Timeout: baseline 30 s, factor 10 → mutant timeout 300. Baseline timeout None.
- worker_count(10,None)=min(10,cores). (10,3)=min(3,cores). (2,8)=min(2,cores).
- Overlay: private real copy of `src/demo.py`. `src/other.py` symlink, `tests/` symlink, `pyproject.toml` real copy. Workers are independent. The original is untouched and the base is removed.
- Node: mutating `src/find.mjs` copies `src/find.test.mjs`, `src/main.mjs`, `src/app.test.mjs` (transitive), `tests/use.mjs` (`../src/find.mjs`) as real files. `src/notes.mjs` stays a symlink. `tests/` becomes a real dir with `other.test.mjs` a symlink. `node script` in the worker prints `worker`.
- Mutant runs happen in a cwd containing `mutation-workers`. No files remain under `target/` afterwards.
- `--mutate-all` twice → the second run makes as many calls as the first.
- `restore_backups`: `target/mutator-backup/src/demo.py` restored to `src/demo.py` and the backup deleted.
- covered_lines=None & not ignore → all uncovered + "No coverage data". `mutation_warning=2` with 2 sites → no WARNING.

### 9.4 CLI (`test_cli.py`, boundaries)
- `--lines "2, 4"` → {2,4}. `--lines 0` → 1. `--timeout-factor 0`/`-1` → 1. `--max-workers 0`/`nope` → 1. `--scan --max-workers 2` → 1. `--not-a-flag` → 1. `--mutate-all --since-last-run` → 1 (stderr "cannot be combined", empty stdout). `--help` → 0 (stdout has `uml-viewer`, `__pycache__`, `testdata`; empty stderr).
- `--changed` outside git → exit 128, stderr `not a git repository`, no "No source files". Changed detection keeps `café.py`, `my file.py`, new nested untracked, renamed-to path. It drops deleted and old rename paths, and `target/skip.py`. `--root sub --changed` → only files under sub.
- `--scan --no-coverage --changed` on a freshly `git add`ed `src/demo.py` → stdout has `demo.py:2`.
- Dir arg + explicit test file → stderr `Skipping test file`, stdout `app.py:2`.
- `--scan` writes no `.metrics`. `--scan` doesn't restore backups (scan shows `0 -> 1` from the dirty file). A later failing-baseline run restores it (source == clean) → exit 2.
- Real Python run: `def add…if a > 0…return 0` with test `add(2,3)==5` → exit 3, stdout has SURVIVED and KILLED. Snapshot `.metrics/mutate/demo.edn` contains `:namespace "demo"` and `:id "defn/add"`.
- `def add(a, b):\n    return a + b\n` → exit 0, KILLED. Then a failing baseline → exit 2 and the snapshot is byte-identical.
- Empty tree → exit 0, `No source files`.
- `--coverage-command "exit 3"` → exit 3, stderr `will not be read`, no `Baseline failed`.
- `--root P src/demo.py` from cwd C resolves against P. `--source-root src` works.
- `--use-existing-coverage` / `--no-coverage` never call crapper run_coverage. The default calls it once.

### 9.5 Coverage (`test_coverage.py`)
- JaCoCo `<line nr=3 ci=2>`, `nr=4 mi=4 ci=0 cb=0`, `nr=5 cb=1` → {3,5}. Package/file key `demo/Board.java`.
- Go profile `demo/widget.go:2.1,4.2 1 1` + `5.1,6.2 1 0` → {2,3,4}. A missing file → None. No profile → None.
- LCOV `SF:src/demo/app.py DA:1,3 DA:2,0` → {1} (same for clojure via LCOV).
- Shorter path never steals: keys `b/app.py`{2}, `a/b/app.py`{1} → long {1}, short {2}. Go analog: short {2}, long `set()`. If only the `a/b/…` key exists → short None, long {2}.
- Clojure HTML `target/coverage/src/demo/core.clj.html` with span line 4 beats LCOV {9} → {4}.

### 9.6 Snapshot (`test_metrics.py`, `test_edn.py`, boundaries)
- `a.go` (Run) then `b.go` (Stop) in ns `demo` → one file `.metrics/mutate/demo.edn` with both forms. Rewriting a.go with killed=9 keeps Stop=4.
- Private round trip: outcome `["hide.clj","demo.core","defn-/hide",8,12,"false","true"]` killed → history has it, plus the digest.
- Paths: `example.com/demo.Widget` → `example.com/demo.Widget.edn`. `crate::foo::Bar` → `crate/foo/Bar.edn`.
- A legacy `legacy.edn` (ns demo, form from b.go) is folded into `demo.edn` and deleted.
- Writing a.go with [] after a.go forms → `demo.edn` deleted.
- An unrelated empty snapshot (`untouched`, no forms/outcomes) is left in place.
- Forms without line sort first: `defn/m` (no line) before `defn/a` line 1. A foreign outcome is kept. `other.edn` survives.
- A form with a non-string id is ignored by history.
- EDN: `""`/`"; hi"` → empty EDN. `{` → unexpected end. `"hi` → unterminated string. `"\` → unterminated string escape. `:` → empty keyword. `"; kept\n12"`→12. `#{1 2}`→set. `{nil 1}`→{None:1}. `1.50`→1.5. `foo`→"foo".

### 9.7 [probe] goldens worth adding as port tests
- Python `x = -a` → `(-,"")`. `a - -1` → `(-,+)`, `(-,"")`, `(1,0)`. `a += 1` → `(1,0)` only. `0x1 + 1.0 + 1j + 1` → three `+`, one `1`. `f"{a > 1}" + "a>1"` → `>`, `1`, `+`. `f"{x:>10}"` → none. `case -1:` → `1` only (anchor = match line). `Literal[-1]` → `-` and `1`.
- TS `a ** 2 % 3 >> 1 >>> 0 << 1` → only the ints `1`,`0`,`1`. `[1, 0.0, 0x0, 1n, 1e0, 01]` → only `1`. `a?.b()` → `.`. `a?.()` → `""`. `a?.[0]` → `""` + `0→1`. `` a?.b`x` `` → `.`. `a?.5:1` → `1` only. `{ true: 1, false: a.true, 0: 1, [1]: 0 }` → `1`, `0`, `1`, `1`, `0` (no booleans). `const o = a<b>(c)` → none in `.ts`, `<`/`>` in `.js`. `a! > b!` → `>` only.
- Go `[1]int`, `[0]int`, `[...]int{1}` → ints. `0x1 + 1.0 + 0 + 1i + 01 + 'a'` → five `+`, one `0`. `!true` → `delete !`, `true→false`. `-a` → `delete -`. `var g = 1 + 0` at top level → dropped (no owner).
- Rust `let rr = &&a;` → none. `|| a > 0` → `>`, `0`. `|x| x || false` → `||`, `false`. `match a { -1 => 0, _ => 1 }` → `1`,`0`,`1`. `t.0 + t.1` → `0`,`+`,`1`. `vec![1, 0]` → `1`,`0`. `assert!(a > 1 && true)` → `1`, `true`. `println!("{}", -1)` → `1`. `1u8`, `1_0`, `1i32`, `0x1` → none.
- Java `(Foo) -y` → `(-,+)`. `(int) -y` → `(-,"")`. `g(a < b, c > d)` → `<`, `>`. `1L`, `0b1`, `01` → none. Constructor body → dropped.
- Clojure §2.5 quirks.
