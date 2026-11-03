# crapper — implementation spec for a stdlib-only Go port

Reference: `upstream/crapper` @ `6fd92c4` (Python ≥3.11, `tree-sitter` + `tree-sitter-language-pack`).
Behaviour marked **[obs]** was verified by running upstream with tree-sitter 0.26.0 and
tree-sitter-language-pack 1.23.1 (the versions pip installs today). Grammar versions change node shapes, so
treat **[obs]** rows as the parity target. Everything else comes from reading the source.

Upstream test status: 145/146 pass. The failure (`test_project_snapshot_is_readable_as_crap4clj_edn`) only
needs `bb` (babashka) on PATH.

Coupling note: `upstream/mutator` imports crapper internals: `discover.language_of`, `is_test_file`,
`iter_source_files`, `languages.functions_in_file`, `languages.treesitter`, `coverage.normalize_path`,
`coverage.load_bundle`, and the JaCoCo reader. Mutator also reads `Function.start_byte/end_byte`, which only
TypeScript fills. The Go packages should expose the same seams.

---

## 0. Data model

```
Function{name, namespace string; complexity int; start_line, end_line int (1-based, inclusive);
         path string; language string; jacoco_class string|nil; start_byte, end_byte int (=-1 unless TS)}
Entry{name, namespace string; complexity int; coverage *float64; crap *float64; path, language string}
```
`language` ∈ `clojure|java|go|typescript|python|rust`. JS files are also `"typescript"`.

### 0.1 Reading source (applies to every language)
`file.read_text(encoding="utf-8", errors="replace")`:
- Invalid UTF-8 bytes become U+FFFD. Python's `errors="replace"` follows the WHATWG-style maximal-subpart rule,
  one U+FFFD per maximal invalid subsequence. Go's `strings.ToValidUTF8` collapses runs, so it is not
  equivalent. Decode rune by rune with `utf8.DecodeRune`, emitting U+FFFD for each `RuneError` width-1 step.
  That matches CPython for almost all inputs. The edge cases are truncated multibyte sequences.
- **Universal newlines**: `\r\n` → `\n`, then a lone `\r` → `\n`. This changes line numbers for lone-CR files,
  and byte offsets everywhere. Do it before parsing.
- A BOM is kept as U+FEFF (the codec is `utf-8`, not `utf-8-sig`).
- The parser receives the UTF-8 re-encoding of that text. Byte offsets refer to it.

### 0.2 Line rules (tree-sitter `start_line` / `end_line`)
`start_line = start_row + 1`. `end_line = end_row + 1`, except when `end_col == 0`: then
`end_line = max(start_line, end_row)`. Rows are 0-based. This only fires for nodes ending right after a
newline. For a function's last token it effectively never applies.

### 0.3 Complexity walk (generic)
`complexity(root, is_decision, skip)`: `score = 1`. Walk root's subtree in preorder. If `skip(node)`, drop the
node **and its subtree**. The root is passed through `skip` too, so skip predicates must exclude the root
where noted. If `is_decision(node)`, `score += 1`. Iterative: a 1200-term `1 + 1 + …` expression must not
overflow.

`binary_logic(n)` = `n.type == "binary_expression"` with a direct child token `&&` or `||`.

---

## 1. CLI

### 1.1 Argument parsing (`parse_args`)
- `argv` = process args after the program name.
- If **any** arg equals `-h` or `--help` (anywhere, even as an option value), print HELP to **stdout** and
  exit 0.
- Scan left to right:

| arg | effect |
|---|---|
| `-s`, `--source-root` | takes value; appended to `source_roots` (repeatable) |
| `--root` | takes value; `project_root` (last wins). Default `.` |
| `--coverage-command` | takes value (string, run through the shell) |
| `--threshold` | takes value; Python `float(raw)`, or error `--threshold requires a number` |
| `--no-coverage` | bool |
| `--use-existing-coverage` | bool |
| `--changed` | bool |
| anything else starting with `-` (incl. `-`, `--x=1`, `--threshold=5`) | error `Unknown option: <arg>` |
| otherwise | positional |

- Taking a value: error `<opt> requires a value` (opt spelled as typed) when there is no next arg, the next
  arg is `""`, or the next arg starts with `-`. So negative thresholds cannot be given.
- `float()` semantics: surrounding whitespace allowed, `inf`/`nan`/`infinity` (any case), `1e3`, `+5`,
  digit-group underscores `1_0` are accepted. Go: `strings.TrimSpace`, remove `_` only between digits,
  `strconv.ParseFloat`. ParseFloat already accepts inf/nan forms. Reject hex (`0x1p3` is valid in Go and
  invalid in Python).
- Errors: write `"<msg>\n\n" + HELP` to **stderr**, exit **1**.
- After the scan, `--no-coverage` together with `--coverage-command` →
  `--no-coverage cannot be combined with --coverage-command\n\n` + HELP on stderr, exit 1.
- Precedence when several coverage flags are given: `--no-coverage` > `--use-existing-coverage` >
  `--coverage-command` > default runners. `--use-existing-coverage` silently ignores `--coverage-command`.

### 1.2 HELP (byte-exact, 2938 bytes, sha256 `19f6dcbf19d2c7ced9a432cbade105d159bacc93000dd28d6c2501a6ff0b3648`, ends with `\n`)
The skipped-dirs list is `sorted(SKIP_DIRS ∪ TEST_DIRS)` joined with `", "`. The patterns list is
`*_test.go`, the Clojure suffixes, the JS suffixes in tuple order, then `test_*.py, *_test.py, conftest.py`.
`²`, `×`, and `−` are non-ASCII. Two lines are long and unwrapped.
```
Usage: crapper [options] [path-or-filter ...]

Detect the language of each source file and score it with the CRAP metric
CRAP = CC² × (1 − coverage)³ + CC. Writes .metrics/crap.edn for uml-viewer
and prints a report sorted worst first.

Languages: Clojure (.clj .cljc .cljs .bb), Java (.java), Go (.go),
TypeScript and JavaScript (.ts .tsx .mts .cts .js .jsx .mjs .cjs),
Rust (.rs), Python (.py).

Options:
  -h, --help                    Print this help and exit.
  --root <path>                 Project root. Metrics are written here.
                                Default: the current directory.
  -s, --source-root <path>      Walk this tree instead of the project root.
                                May be repeated.
  --changed                     Analyze added and modified source files from
                                git status.
  --no-coverage                 Score complexity only. Coverage and CRAP are N/A.
  --use-existing-coverage       Read coverage already on disk. Do not rerun tests.
  --coverage-command <cmd>      Run this command instead of the per-language
                                coverage tools, then read the reports it wrote.
  --threshold <number>          Exit 2 when the worst CRAP score is above this.

Arguments:
  path              File or directory to analyze. Test paths are included when
                    you name them explicitly.
  filter            When the argument is not a path, only source files whose
                    path contains this text are analyzed.

With no paths, source files under the project root are analyzed. Directories
named .clj-kondo, .git, .hg, .idea, .metrics, .svn, .venv, __pycache__, __tests__, build, coverage, dist, node_modules, out, spec, specs, target, test, testdata, tests, vendor, venv are skipped, as are *_test.go, *_test.clj, *_test.cljc, *_test.cljs, *_test.bb, *.test.ts, *.spec.ts, *.test.tsx, *.spec.tsx, *.test.mts, *.spec.mts, *.test.cts, *.spec.cts, *.test.js, *.spec.js, *.test.jsx, *.spec.jsx, *.test.mjs, *.spec.mjs, *.test.cjs, *.spec.cjs, test_*.py, *_test.py, conftest.py.

Coverage, when it is produced:
  Clojure      clj -M:cov --lcov, then Cloverage form counts or LCOV
  Java         Maven JaCoCo instruction coverage, for each pom.xml module
  Go           go test ./... -coverprofile=...
  TypeScript   npm run coverage, Vitest's own LCOV, or c8 around npm test
  Rust         cargo llvm-cov, or cargo tarpaulin, in the nearest Cargo.toml
  Python       coverage.py LCOV, via pytest or unittest, per project

uml-viewer joins .metrics/crap.edn on :namespace and :name.
  Clojure      the ns, and the defn name
  Java         package.Class, and the method name
  Go           import path, or import.path.Receiver for methods
  TypeScript   dotted module path, or module.Class for methods
  Rust         crate::module, or crate::module::Type for methods
  Python       dotted module path, or module.Class for methods
```

### 1.3 Run flow (`run`)
```
opts = parse_args(argv); help → print, return code
root = abs(realpath-like resolve(opts.project_root))   # Path.resolve(strict=False): symlinks resolved (/tmp → /private/tmp on macOS)
files = select_files(opts)          # GitError → print str(err) to stderr, return err.code
if files empty: print "No source files to analyze." to STDOUT; return 0   # no coverage run, no metrics write
bundle = coverage_bundle(opts, root, files)    # §5
entries = analyze_files(files, root, bundle)   # §3, §6
write root/.metrics/crap.edn (mkdir -p)        # §6.4
stdout ← format_report(entries) (flushed)
stderr ← "Wrote " + str(root/".metrics"/"crap.edn") + "\n"     # absolute resolved path
return threshold_status
```
Threshold: if `threshold` is set and at least one entry has non-nil crap and `max(crap) > threshold`
(strictly greater, using the 4-dp rounded crap), print
`CRAP threshold exceeded: {max:.1f} > {threshold:.1f}` to stderr (after "Wrote …") and return **2**.
A NaN threshold never trips. `inf` formats as `inf`.

Exit codes: `0` ok/help/no files · `1` usage error, or any uncaught exception (Python traceback, e.g. a
malformed Go profile, malformed JaCoCo XML, `git` missing, an unreadable go.mod or Cargo.toml) ·
`2` threshold · `N` = git's exit status when `--changed` fails (e.g. 128).
For the Go port: on internal errors print a one-line message to stderr and exit 1. Byte-exact traceback
parity is not a goal.

Coverage subprocesses **inherit stdout/stderr**. Test-runner output therefore lands on crapper's stdout
**before** the report. Keep inheriting.

### 1.4 File selection (`select_files`)
1. `root` = resolved project root.
2. Positionals: `candidate = arg` if absolute, else `root/arg`. If it exists (file or dir) →
   `existing += resolve(candidate)`. Otherwise `filters += arg` (raw string).
3. Base set, first matching rule wins:
   - `--changed` → `changed_source(root)` (§1.5). Existing positionals are **ignored**.
   - `source_roots` non-empty → `iter_source_files([resolve(root/p) for p in source_roots])` (an absolute
     `p` stays absolute). Existing positionals are **ignored**, not even used as filters.
   - `existing` non-empty → for each: a dir → `iter_source_files([dir])`; a file → included iff
     `language_of(file) != nil`. **Test files are included when named explicitly; test dirs are not**:
     naming `tests/` yields nothing, because `is_test_file` sees `tests` in the absolute path parts.
   - else `iter_source_files([root])`.
4. Filters: keep a path iff any filter is a substring of the path's **absolute** posix string.
5. Result: unique resolved paths sorted by posix string (codepoint order = Go byte order on valid UTF-8).

### 1.5 `--changed`
- `git -C <root> rev-parse --show-toplevel` (capture, text, utf-8). Non-zero → GitError(code,
  `stderr.strip()` or `"git status failed"`).
- `git -C <root> status --porcelain -z --no-renames --untracked-files=all -- .`. Non-zero → GitError, same
  form.
- `top = Path(stdout.strip())`. Split stdout on `\0`. Skip entries with `len < 4`. `path = resolve(top /
  entry[3:])`. Keep it iff it is a regular file **and** lies under `resolve(root)`. Deleted files fail
  `is_file`. Non-ASCII names come through unquoted thanks to `-z`.
- Then keep paths with `language_of != nil` and `!is_test_file(path)`.

---

## 2. Discovery (`discover.py`)

`EXTENSIONS` (case-sensitive, applied to Python `Path.suffix` = the text from the last `.` of the name,
and `""` when the only dot is a leading one: `.py` → no suffix; Go's `filepath.Ext(".py")` returns `.py`,
so special-case it):
`.clj .cljc .cljs .bb`→clojure · `.java`→java · `.go`→go · `.ts .tsx .mts .cts .js .jsx .mjs .cjs`→typescript ·
`.rs`→rust · `.py`→python. A name ending in `.d.ts` → nil (but `.d.mts`/`.d.cts` are **not** excluded).

`SKIP_DIRS` = `.git .hg .svn .idea .venv venv node_modules vendor target dist build out coverage .metrics testdata __pycache__ .clj-kondo`.
`TEST_DIRS` = `test tests spec specs __tests__`.

`is_test_file(path)`: by name: ends with `_test.go`; ends with `_test.clj|_test.cljc|_test.cljs|_test.bb`;
ends with any of `.test|.spec` × `.ts .tsx .mts .cts .js .jsx .mjs .cjs`; equals `conftest.py`; ends with
`_test.py`; starts with `test_` and ends with `.py`. Otherwise true iff **any component of the path as
given** is in TEST_DIRS.
- Discovery passes **absolute** paths. If the project itself lives under a directory named
  `test|tests|spec|specs|__tests__` (e.g. `/home/u/tests/proj`), every file counts as a test →
  "No source files to analyze." Preserve this.

`iter_source_files(roots)`: for each root: missing → nothing; a file → `[root]` with **no**
language/test filter; a dir → `os.walk(root)` top-down, pruning subdirs whose name is in SKIP_DIRS ∪ TEST_DIRS
(exact, case-sensitive). The root itself is never pruned. Symlinked dirs are listed but not followed (Go:
`WalkDir` + stat symlink entries; skip if the target is a dir). Keep files with a language that are not
tests. Then `resolve()` each (symlinks → target path), dedup, and sort by posix string. Walk errors are
ignored.

Note: `analyze_files` re-resolves each file and calls `language_of` on the **target** name.

---

## 3. analyze_files

```
root = resolve(project_root)
if bundle: bundle.bind_sources([rel_or_abs(resolve(f), root) for f in files])
for f in files (in order): f = resolve(f); lang = language_of(f) or skip
  src = read (§0.1); rel = f relative to root as posix, or absolute posix if outside root
  for fn in functions_in_file(lang, src, rel, str(root)): entries += make_entry(fn, cov(bundle, fn))
return sort_entries(entries)
```
`cov`: bundle nil → nil. `bundle.percent_for(fn)` nil → `0.0`, else the value.

The language readers get `path` = `rel` (relative to root, or absolute when outside root) and
`project_root` = absolute root string.

---

## 4. Languages

### 4.1 Python (`grammar: python`)

**Entries**: every `function_definition` node (`def` or `async def`; decorators are outside the node) that
is not nested. Nested = walking up the ancestors, a `function_definition` is met **before** any
`class_definition`.
- Methods of classes at any depth are entries, including classes defined inside functions:
  `def factory(): class Hidden: def secret` → entry `module.Hidden.secret`.
- `name` = first `identifier` child (the def name).
- `namespace` = module, plus `.`-joined names of **all** `class_definition` ancestors, outermost first,
  crossing function boundaries. Example: `class A: def m(): class B: def n()` → `n` in `mod.A.B`; `m` in
  `mod.A`.
- `start_line` = the line of `def`, or `async` for async defs. Never the decorator line. `end_line` = the
  last line of the block. Trailing comments indented inside the block are included **[obs]**. Only DA lines
  affect coverage, so this does not matter.
- Order: preorder (document order; outer before inner).

**Decisions** (`is_decision`): node types `if_statement`, `elif_clause`, `for_statement`, `while_statement`,
`except_clause`, `conditional_expression`, `case_clause`, `if_clause`, plus `boolean_operator` with an
`and`/`or` child.
**Skip**: `class_definition` subtrees (the root is never a class).
Nested defs, lambdas, and comprehensions inside the function **are counted**.

Token-level equivalent (what a hand parser must count inside the function span, minus nested class spans):

| construct | counts | [obs] notes |
|---|---|---|
| `if` stmt, `elif` | 1 each | `else` 0 |
| `x if c else y` | 1 | also in lambdas, default args `def f(x=a if b else c)`, f-string interpolations |
| comprehension `if` | 1 per `if` clause | `[x for ys in xs if ys for x in ys if x if x>1]` → +3. Comprehension `for` = **0** |
| `for` statement (incl. `async for`) | 1 | `for…else` adds 0 |
| `while` | 1 | `while…else` adds 0 |
| `except`, `except*`, bare `except:` | 1 each | `finally`/`try-else` 0 |
| `match` | 0 | each `case` clause +1 (incl. `case _`) |
| case guard `case p if g:` | **+1 extra** (guard is an `if_clause`) | `case 1 if x>0` → +2 total |
| `and` / `or` | 1 per operator token | `a and b or c and not a` → +3. `not` 0. `assert a and b` counts |
| f-strings | interpolations are parsed: `f"{a if b else 0}"` +1, `f"{x!r:>{10 if x else 5}}"` +1, `rf"{x and y}"` +1 | |
| `with`, walrus, `assert`, `lambda`, `type X = …`, PEP 695 generics | 0 | |

**Namespace** (`_module_namespace(path, source_root)`), applied to `path` with `\` → `/`:
1. Strip root: `root = source_root.replace("\\","/").rstrip("/")`. If root not in {"", "."} and path starts
   with `root + "/"`, drop that prefix; **else** if path starts with `./`, drop `./` (once).
2. Strip src: starts with `src/` → drop 4 chars; elif it contains `/src/` → keep the text after the
   **first** `/src/`.
3. Drop a trailing `.py`. Then a trailing `/__init__` → drop; exactly `__init__` → `""`.
4. `""` → `"__init__"`. Else replace `/` with `.`.
Examples: `src/demo/box.py`→`demo.box` · `/proj/src/demo/app.py` (root `/proj`)→`demo.app` ·
`./src/demo/app.py` (root `.`)→`demo.app` · `pkg/src/demo/__init__.py`→`demo` · `__init__.py`→`__init__` ·
`src/demo/__init__.py`→`demo`. A file outside root (absolute, no `/src/`) such as `/other/x.py` →
`.other.x` (leading dot; preserve).
Class namespace = `module + "." + ".".join(classes)`. Module is never empty.

### 4.2 TypeScript / TSX / JavaScript

Grammar: `.tsx` → tsx; `.jsx .mjs .cjs .js` → javascript (JSX enabled); everything else (`.ts .mts .cts`) →
typescript (no JSX; `<T>x` is a type assertion).

**Decisions**: `if_statement`, `for_statement`, `for_in_statement` (covers for-in, for-of, for-await),
`while_statement`, `do_statement`, `catch_clause` (incl. `catch {}` without a binding), `ternary_expression`,
`switch_case`, `switch_default`; `binary_expression` with `&&`, `||`, or `??`; every `optional_chain` node;
and a bare `?.` token whose parent is not `optional_chain`.
⇒ Token rule: **each `?.` token = +1** (TS `a?.()` is a bare `?.` under call_expression; JS wraps it).
`a?.b.c?.d` → +2. `a ?.5 : 1` is a ternary (+1), not `?.` **[obs]**.
Not counted: `&&=`, `||=`, `??=`; TS conditional types `T extends U ? X : Y`; optional markers `x?:`,
`m?()`; `!` non-null; labels; `else`; `finally`. JSX text `a && b` is not counted. `&&` inside `{…}` JSX
expressions is.

**Skip during complexity** (never the root itself): `class_declaration`, `abstract_class_declaration`,
`interface_declaration`, and any **route callback** (below). Class *expressions* (`const K = class {…}`,
`return class {…}`) are **not** skipped: their methods' decisions count toward the enclosing function
**[obs]**. Nested `function_declaration`s, arrows, function expressions, and object-literal methods are
**counted** in their enclosing entry.

**Entries** — collected in one preorder pass, then all sorted by the recorded node's `start_byte` (stable):

1. `function_declaration` (plain or `async`; **not** `function*`, which is `generator_function_declaration`)
   - not inside any ancestor of type `function_declaration | method_definition | arrow_function | function_expression`
     (generators are not in this set, so a function inside a top-level `function*` **is** an entry **[obs]**);
   - has a `statement_block` child (overloads / `declare function` are `function_signature` → ignored);
   - `name` = first `identifier` child. `export default function () {}` has none → ignored.
   - namespace = module. Lines/bytes = the function_declaration node (starts at `async`/`function`; `export`
     is outside).
2. `method_definition` with a `statement_block` child (TS `m(): void;` / `abstract m()` are signatures →
   ignored)
   - `name` = first child of type `property_identifier` or `identifier`. Private `#p`, string `"q"()`,
     numeric `1()`, and computed `[k]()` names → ignored (their decisions vanish because the class is
     skipped). Keywords work as names: `if()`, `catch()`, `get`/`set` accessors (both named `x` → two
     entries `x`), `static s()`, `constructor` (**is** an entry, unlike Java).
   - Needs ≥1 ancestor `class_declaration` or `abstract_class_declaration` (any depth, crossing functions).
     Class name = first `type_identifier` or `identifier` child. Namespace = `module + "." + ".".join(names
     outermost→innermost)`. Anonymous `export default class {…}` and class expressions contribute no name.
     When no named class is above → ignored.
   - ⚠ An object-literal method (or class-expression method) **inside** a named class's method becomes an
     entry named after the outer class **and** its decisions are also counted in the outer method
     (double-count) **[obs]**: `class A { m() { return { n() { if (a) {} } }; } }` → `A.m` cc2, `A.n` cc2.
     Object methods at top level → no entry.
   - Decorators are siblings in `class_body`, so the start line is the method line **[obs]**.
3. `lexical_declaration` (`const`/`let`; **not** `var`) not inside any function-type ancestor (same set as
   in 1). Class static blocks and top-level `if {}` blocks don't count as functions **[obs]**. For each
   `variable_declarator` child: `ident` = first `identifier` child (destructuring → none → skip);
   `value` = first child of type `arrow_function` or `function_expression` (a direct child: `(() => 1)`,
   `memo(() => …)`, `x as F`, IIFEs → skip; `async function*` → generator_function → skip).
   - name = ident. Lines and bytes = the **declarator** (starts at the identifier, ends at the end of the
     value). Complexity = walk of the **value** node.
   - `let l = function(){…}, m = async (x) => …` → two entries.
   - Not entries: `var f = …`, `module.exports = function…`, `exports.h = async () => …`, class fields
     `handler = () => {…}` (and their decisions vanish).
4. **Express route callbacks**. For **every** `call_expression` anywhere in the tree (nested in functions,
   classes, other callbacks):
   - callee = first child that is not `arguments` (skips `type_arguments`; `app.get?.()` works). It must be
     a `member_expression` with a `property_identifier` child `p`, and `lower(p)` ∈
     {get, post, put, patch, delete, head, options, all, use}. Case-insensitive: `app.GET` → `GET`.
     ⚠ No receiver check: `cache.get("k", () => 1)` → entry `GET k` **[obs]**.
   - callbacks = argument children, each unwrapped through any `parenthesized_expression` layers, whose type
     is `arrow_function` or `function_expression`. None → no entries.
   - route = the first argument (unwrapped, in order, at **any** position) that is a literal:
     `string` → concatenation of its `string_fragment` children only, so **escape sequences are dropped**:
     `"/a\nb"` → `/ab`, `'it\'s'` → `its` **[obs]**. `template_string` → the raw text between the backticks,
     verbatim (`${id}` kept). If route is `""` or absent → look for a `.route(...)` chain: start at the
     callee; while the node is a member_expression → go to its first child; if it is a call_expression whose
     callee's property is `route` → use that call's first literal arg; else go to its callee and continue;
     anything else → none.
   - label = `METHOD` upper-cased, plus `" " + route` when route is non-empty. `app.get("", cb)` → `GET`.
   - Each callback becomes one entry. Within a file, route entries are sorted by callback `start_byte` and
     names are claimed in that order: first `L`, then `L#2`, `L#3`… (lowest free n ≥ 2). The claim set holds
     route labels only.
   - Lines and bytes = the callback node. Complexity = walk of the callback (the root itself is not
     skipped). Nested route callbacks inside it are skipped and become their own entries.
   - The namespace is always the module (never a class).
   - A non-route callback inside a function is counted in that function. At top level its decisions vanish.

**Namespace** (`_module_namespace`): steps 1–2 as in Python (root strip, else `./`; then `src/` /
first `/src/`). Then strip the first matching suffix in order `.tsx .mts .cts .jsx .mjs .cjs .ts .js`, then
`/` → `.`. No `index` collapsing (`src/x/index.ts` → `x.index`). Examples: `/app/src/demo/box.ts` (root
`/app`) → `demo.box`; `./demo/box.ts` → `demo.box`; `/app/pkg/src/demo/box.mts` (root None) → `demo.box`.

### 4.3 Go (`grammar: go`; port with `go/parser` + `go/ast`)

**Entries**: `function_declaration` and `method_declaration` nodes with a `block` body, in document order.
- Function: name = `identifier`; namespace = `package_ns`.
- Method: name = `field_identifier`; receiver = the text of the first `type_identifier` in preorder inside
  the **first** `parameter_list` (the receiver). go/ast: unwrap `StarExpr`/`ParenExpr`/`IndexExpr`/
  `IndexListExpr` down to an `Ident`. `*Widget[T]` → `Widget`. An empty receiver `func () Run() {}` or a
  missing name → skip. Namespace = `package_ns + "." + receiver`.
- Lines: `func` keyword line (doc comments excluded) → closing `}` line. go/ast:
  `fset.Position(fd.Pos()).Line`, `fset.Position(fd.End()).Line` (End is just past `}`, same line).

**Decisions** (no skips; closures counted): `if_statement`, `for_statement` (incl. range), `expression_case`,
`type_case`, `communication_case`, `default_case` (switch, type-switch, select defaults), `binary_expression`
with `&&`/`||`. go/ast: `IfStmt`, `ForStmt`, `RangeStmt`, every `CaseClause` (incl. default), every
`CommClause` (incl. default), `BinaryExpr` with `token.LAND`/`token.LOR`. `case 1, 2:` = 1. Walk
`ast.Inspect(fd)`, which includes params and results (they contain nothing countable).

**package_ns**:
- `package_name` = the first `package_identifier` in the file (the package clause), or `""`.
- abs = `path` if absolute, else `resolve(project_root/path)`. With no project_root → `resolve(path)` against
  the process CWD.
- From `dirname(resolve(abs))` walk up to `/`. The **first** directory containing a `go.mod` file stops the
  walk. Scan its lines (`read_text` utf-8 strict): the first line whose `strip()` starts with `"module "` →
  `module = line.split(None,1)[1].strip()`. Comments and quotes are kept verbatim: `module "x"` → `"x"`;
  `module x // c` → `x // c`. A tab after `module` doesn't match. If that go.mod has no module line →
  return package_name.
- No go.mod → `package_name`. Else rel = dir relative to module_dir (posix): `.` → `module`, else
  `module/rel`.

**go/parser parity shims**:
- tree-sitter accepts a file without a package clause (test: `func Orphan() int { return 1 }` → `("",
  "Orphan")`). go/parser returns no decls. If the error is `expected 'package'`, re-parse
  `"package _;" + src` (same line count) and use package_name `""`.
- Use `parser.SkipObjectResolution`. On other errors go/parser still returns partial decls. Keep FuncDecls
  with `Body != nil` and a name that is not `_`-from-error (`func (w *Widget) () {}` gives `Name=="_"`,
  `Body==nil` → skipped by the body rule). The behaviour on invalid files may diverge; that's accepted.

### 4.4 Java (`grammar: java`)

**package** = regex `(?m)^\s*package\s+([a-zA-Z_][\w.]*)\s*;` on the raw text, **first match** (it can hit a
`package x;` line inside a multi-line block comment; `/* package a; */` on one line does not match). Python
`\w` is Unicode. Go `regexp` `\w` is ASCII, so use `[\p{L}\p{N}_]`.

**Entries**: iterative preorder over the tree. On a `method_declaration` node: don't descend. It's an entry iff
- it has no ancestor `method_declaration | constructor_declaration | lambda_expression`,
- it has a `block` child (abstract/interface methods have none),
- name = `identifier` child,
- type chain = the `identifier`s of all ancestors of type `class_declaration interface_declaration
  enum_declaration record_declaration annotation_type_declaration`, outermost first, and the chain is
  non-empty.
- namespace = `pkg + "." + ".".join(chain)` (or just the chain when there's no package); `jacoco_class` =
  the same joined with `$`.
- Constructors, compact record constructors, static/instance initializers → not entries and not counted.
- ⚠ **[obs]** Methods of an anonymous class in a **field initializer** are entries under the enclosing
  type: `Runnable r = new Runnable(){ public void run(){…} };` → `a.b.S.run`. Enum-constant body methods →
  `pkg.S.E.f`. Interface `default` methods → `pkg.S.I.d`. Record methods → `pkg.S.R.y`. Methods in
  anonymous/local classes inside methods or lambdas → not entries.
- start_line = start of the method_declaration, **including annotations/modifiers** (`@Override` line).
  Javadoc is excluded. end = closing `}`.

**Decisions**: `if_statement`, `for_statement`, `enhanced_for_statement`, `while_statement`, `do_statement`,
`catch_clause`, `ternary_expression`, `switch_label`, and binary `&&`/`||`.
`switch_label` = each `case …` (one per label, even `case 1, 2 ->`) and each `default`. This works for
colon and arrow forms and for switch expressions. Pattern guards `when …` add 0 (their `&&` count).
`instanceof` patterns add 0. try-with-resources adds 0; multi-catch `catch (A | B e)` = 1.
**Skip**: subtrees of the nested type kinds above and **any `class_body`** (anonymous classes, local
classes). Lambdas inside the method are counted.

### 4.5 Rust (`grammar: rust`)

**Entries** (preorder): `function_item` with a `block` child such that
- its parent is not `block` (a fn directly inside a fn body is not an entry, but its decisions count in the
  enclosing function; a fn inside an `impl`/`trait` inside a fn body **is** an entry **[obs]**);
- no ancestor `mod_item` has identifier `tests` exactly (`mod test`, `#[cfg(test)] mod unit` are not skipped
  **[obs]**);
- name = `identifier` child.
- Lines: from the first qualifier token (`pub`, `pub(crate)`, `const`, `async`, `unsafe`, `extern "C"`) or
  `fn`. Attributes `#[…]` and doc comments are separate nodes, so they're excluded.

**Decisions** (no skips): `if_expression` (incl. `if let`, `else if`), `for_expression`,
`while_expression` (incl. `while let`), `loop_expression`, `match_arm` (every arm incl. the last), and
`try_expression` (`?`), plus binary `&&`/`||`.
**[obs]** details:
- Macro invocations are token trees: nothing inside `m!(…)`, `m![…]`, `m!{…}` counts (`println!("{}", if a
  {1} else {2})`, `assert!(a && b)`, `vec![x?]` → 0). `macro_rules!` bodies → 0. Attributes → 0.
- Match guards `p if g =>` add 0. The arm itself counts once.
- **Let chains**: in an `if`/`while` condition containing a top-level `let`, the top-level `&&` that join
  the chain elements add 0. Operators inside the elements count. `if let Some(x) = a && b` → +1 (if only);
  `if (a && b) && let …` → +2; `while let P = a && b || c` → +2 (while, `||`); `if let P = (a && b)` → +2.
- Closures: `|| a && b` → `&&` +1 (the empty-param `||` is not binary). `move || x?` → +1. `&&x` (double
  reference) and `&&T` add 0. `?Sized` adds 0. `for<'a>` and `impl T for X` add 0. Labels `'outer: loop`
  → +1. `async { a? }` → +1.

**Namespace** = `"::".join(nonempty(modules + inline_mods + [impl_type]))`.
- crate: from `dirname(resolve(abs_path))` walk up. At each dir with a `Cargo.toml` file, read it (utf-8),
  match `(?ms)^\[package\](.*?)(?:^\[|\Z)`. If it matches → name = `(?m)^name\s*=\s*"([^"]+)"` within the
  block, else that dir's basename; return `name.replace("-","_")` and crate_root = that dir. A Cargo.toml
  without `[package]` (a workspace) → keep walking. None found → `("crate", None)`.
- modules (`_file_modules`), with `file = resolve(abs_path)`:
  - crate_root None: stem ∈ {lib, main} → `[crate]`, else `[crate, stem]` (`notes.rs` → `crate::notes`).
  - rel = file relative to crate_root (posix). Not relative → `[crate]`. Strip a leading `src/`.
    `lib.rs`/`main.rs` → `[crate]`. Else split on `/`; last == `mod.rs` → drop it, else strip `.rs`; if the
    first part is `bin` and len ≥ 2 → parts[1:] **without** the crate (`src/bin/tool.rs` → `tool`;
    `src/bin/tool/main.rs` → `tool::main`); else `[crate, *parts]` (`src/game/board.rs` →
    `demo::game::board`; `src/game/mod.rs` → `demo::game`; `benches/x.rs` → `demo::benches::x`).
- inline_mods = the identifiers of all `mod_item` ancestors, outermost first.
- impl_type: only when the fn's parent is a `declaration_list` whose parent is `impl_item`. Iterate the
  impl's children: on the token `for` → saw_for = true, chosen = nil. On a child of type
  `type_identifier | generic_type | scoped_type_identifier` → chosen = the first `type_identifier` in
  preorder (itself included); if saw_for, return it immediately. At the end return chosen. Results
  **[obs]**: `impl<T> Foo<T>` → `Foo`; `impl fmt::Display for Wrapper<T>` → `Wrapper`;
  `impl Trait for Vec<crate::Sample>` → `Vec`; `impl crate::a::B` → `B`; `impl Foo for &Bar` → none;
  `impl dyn Tr` → none. Trait default methods (parent `trait_item`) → no type: `trait Foo { fn draw(&self)
  {} }` in lib.rs → `demo`.

### 4.6 Clojure (no tree-sitter; port the scanner verbatim)

`ws(c)` = Python `str.isspace` (Unicode, incl. `\x1c-\x1f`, `\x85`, `\xa0`, U+1680, U+2000–200A,
2028, 2029, 202F, 205F, 3000). Go's `unicode.IsSpace` lacks `\x1c-\x1f`, so add them. Regex `\s` in these
patterns is also Python-Unicode. Build the classes explicitly; Go's `\s` lacks `\v`.

**Reader helpers**
- `consume_string(i)`: skip the opening `"`; `\` escapes the next char; stops after the closing `"` or at EOF.
  Counts `\n` for line tracking.
- `char_literal_end(i)` (at `\`): if i+1 ≥ len → len. Else take char i+1 unconditionally, then advance while
  the char is not ws and not in `()[]{}";,` → end.
- `form_end(i)`: skip ignorables (ws, then one `;` comment to the newline, repeated until no progress).
  At EOF → i. Then: `([{` → `balanced_end` (same-bracket depth counting, skipping strings, char literals,
  and `;` comments); `"` → string end; `\` → char end; `#_` → `form_end(i+2)` (recursive, so `#_ #_ a b`
  drops two forms); else an atom: advance while not ws and not in `()[]{}";`.
- `without_strings_and_comments(src)`: left to right: `#_` → blank through `form_end(i+2)`; `"` → blank
  the string; `\` → blank the char literal; `;` → blank to the newline (newline kept). Blank = each char →
  `\n` if newline else space (length preserved).

**Defn extraction** (`_extract_top_level_defns`), line = 1, depth = 0:
- At each index, try in order: `;` → consume the comment **including** its newline (line+1 if one was
  consumed); `"` → consume the string (lines counted); `\` → char literal (no line counting, even if the
  char is a newline); `#_` → `form_end(i+2)`, line += count of `\n` in the span. Otherwise: `(` → if depth
  == 0, record start and start_line; depth++. `)` → if depth == 1 and a start is recorded → remember the
  form `src[start:i+1]` with end_line = the current line, reset depth=0/start=nil; else depth =
  max(0, depth-1). `\n` → line++. Brackets `[ ] { }` are ignored.
- `defn_name(form)`: the form starts with `(`; skip ASCII-or-Unicode ws (`isspace`); the keyword `defn-` or
  `defn` must be followed by EOF or one of ` \t\n\r()[]{}"`. Then repeatedly skip ws and metadata:
  `^{…}` → balanced_end; `^sym` → skip to the stop set. The name is read up to the stop set
  ` \t\n\r()[]{}"`. If the first char is in `()[]{}";` or EOF → nil. Note that `;` and `,` are **not** in
  the stop set.
- Unclosed forms at EOF are dropped. A top-level `def`/`defmethod`/`comment` form → not an entry.

**Complexity** of each form text: `clean = without_strings_and_comments(text)`.
`1 + count(DECISION) + Σ clauses(COND match)`:
- `DECISION = \((if-not|if-let|if-some|when-not|when-let|when-some|when-first|if|when|and|or|loop|catch)[\s\)]`
  (non-overlapping findall; the trailing char is consumed, but a match always starts with `(`, so count =
  number of match positions).
- `COND = \((some->>|some->|cond->>|cond->|cond|condp|case)[\s\)]` (Python alternation order: `(condp` matches
  via backtracking).
- clauses(form at match start m): `body = m+1` advanced over non-ws, non-`)` chars. Count the top-level forms
  from body with depth=1, in_form=false, forms=0, iterating until depth==0 or EOF:
  - open `({[`: if depth==1 && !in_form → forms++. Then depth++, in_form = true.
  - close `)}]`: if depth==1 → in_form = false. Then depth--.
  - ws: in_form = (depth != 1).
  - else (atom char, incl. `#`, `'`, `,`, `^`, `@`): if depth==1 && !in_form → forms++. in_form = true.
  - Strings are blanked to spaces, so **string clauses count 0**. A lone `,` counts as a form.
  - `remaining = forms - SKIP[type]`, with SKIP = {condp:2, case:1, cond->:1, cond->>:1, some->:1,
    some->>:1, cond:0}.
  - `some->`, `some->>` → `remaining` (can be negative: `(some->)` → −1).
  - else `base = floor(remaining / 2)` (**Python floor division**; `-1//2 == -1`). For `case` add 1 if
    `remaining % 2 == 1` (Python modulo: `-1 % 2 == 1`). Go: `floorDiv`/`floorMod` helpers.
- **Namespace**: from the cleaned whole source, the first match of `\(\s*ns\s+([A-Za-z0-9*+!_?.\-/]+)`, else
  `\(\s*in-ns\s+'([…same…]+)\s*\)`, else `\(\s*in-ns\s+\(quote\s+([…]+)\)\s*\)`. Otherwise from the path:
  strip root (root not in {".",""}), strip `src/` or the first `/src/`, strip `\.(?:clj[cs]?|bb)$`, then
  `/`→`.` and `_`→`-`. `(ns ^:meta foo)` fails the first regex (the search continues to later text).

---

## 5. Coverage

### 5.1 Bundle selection (`_coverage_bundle`)
- `--no-coverage` → nil bundle (coverage and crap are nil for every entry).
- `--use-existing-coverage` → `load_bundle(root)`.
- Otherwise `status = run_coverage(root, files, coverage_command)`. If `coverage_command` is set and
  `status != 0` → an **empty** bundle (every function 0%; stale reports are ignored). Else `load_bundle(root)`.

### 5.2 Running commands (`run_shell(cmd, cwd)`)
- First prints to stderr `+ ({cwd}) {shown}`. For a string, `shown` is the string itself. For an argv list,
  it's each element `shlex.quote`d and joined by spaces. `shlex.quote(s)`: `''` if empty; s unchanged if it
  only contains ASCII `[A-Za-z0-9_@%+=:,./-]`; else `'` + s.replace(`'`, `'"'"'`) + `'`.
- String → `/bin/sh -c <string>` (`shell=True`). List → exec directly (PATH lookup, no shell). cwd = the given
  dir. stdin/stdout/stderr are inherited. No timeout.
- Start failure (OSError: missing executable, missing cwd) → stderr
  `Coverage command failed to start: {exc}`, return 127. Python's text is
  `[Errno 2] No such file or directory: 'mvn'`. Emulate it as `[Errno 2] No such file or directory: '<argv0>'`
  for ENOENT.
- Return code: the exit status. Killed by signal N → `-N` (Python convention).

### 5.3 `run_coverage(root, files, command)`
`root` resolved. If `command` is set: `run_shell(command, root)`. Non-zero → stderr
`Coverage command exited {code}. Reports already on disk will not be read.` Return the code.
Otherwise, for the languages present in `files` (via `language_of`), in this **fixed order**: clojure, java,
go, typescript, python, rust. Return 0.

`nearest(start, marker, stop)`: `cur = start if dir else parent(start)`. Loop: if `cur/marker` is a file →
cur. If `cur == resolve(stop)` or cur is `/` → nil. `cur = parent`. (A file outside root walks up to `/`.)
Module lists are sets of nearest dirs, **sorted as Python `Path`s**. In 3.12 that is a component-wise
comparison of `str.split("/")`, not raw string order: `/a/b/c` < `/a/b-c`. Only run order depends on it.
`coverage_report(root, module, lang)` = `root/target/coverage/<lang>/lcov.info` when module == root, else
`root/target/coverage/<lang>/<relpath(module, root) with "/"→"__">/lcov.info`. A module outside root raises
ValueError → crash.

**Clojure** (root only):
- Needs `root/deps.edn` or `root/bb.edn`, else warn `No deps.edn or bb.edn; skipping Clojure coverage.`
- Clean `target/coverage` if it's a dir: unlink `target/coverage/lcov.info`. Unlink every `*.html` (recursive)
  whose relative path has no component in {typescript, rust, go, python} and whose own name is not one of
  those. Then remove empty dirs, deepest first (reverse-sorted path order), under the same exclusion; rmdir
  errors are ignored.
- `clj -M:cov --lcov` in root. Non-zero → warn `clj -M:cov --lcov failed; retrying without --lcov.` then
  `clj -M:cov`. Still non-zero → `Clojure coverage exited {code}. Clojure coverage will score 0%.`

**Java**: modules = nearest `pom.xml` of each `.java` file (suffix check). None → `No pom.xml; skipping Java coverage.`
Per module: `rm -rf module/target/site/jacoco`; `rm module/target/jacoco.exec`; run
`mvn -q org.jacoco:jacoco-maven-plugin:0.8.12:prepare-agent test org.jacoco:jacoco-maven-plugin:0.8.12:report`
in the module. Non-zero → `Java coverage exited {code} in {module}. Java coverage will score 0%.`

**Go**: modules = nearest `go.mod` of each `.go` file. None → `No go.mod; skipping Go coverage.` Per module:
`profile = module/target/coverage/go/coverage.out`; mkdir -p; unlink; argv `["go","test","./...","-coverprofile=<abs profile>"]`
(a single argv element, spaces included) in the module. Non-zero → `Go coverage exited {code} in {module}. Go coverage will score 0%.`
⚠ The loader only reads `root/…`, `root/*/…`, `root/*/*/…`. Deeper modules are measured but never read.

**TypeScript**: packages = nearest `package.json` of each typescript-language file (sorted set). None →
`No package.json; skipping TypeScript coverage.` Per package:
1. `report = coverage_report(root, package, "typescript")`. `report_dir = report.parent`.
2. Read `package/package.json` as JSON. Invalid → warn `{path}: {exc.msg}` (e.g. `Expecting value`,
   `Expecting property name enclosed in double quotes`) and skip the package. Not an object → warn
   `{path}: expected a JSON object` and skip. Python json accepts `NaN`/`Infinity` and duplicate keys (last
   wins).
3. `scripts = data.scripts or {}`; a non-dict → `{}`.
   - `"coverage" in scripts` → `["npm","run","coverage"]`.
   - vitest if `"vitest" in str(scripts.get("test",""))` or any of `vitest.config.{ts,mts,cts,js,mjs,cjs}`
     exists in the package →
     `[<package>/node_modules/.bin/vitest if it is a file, else "npx","vitest"] + ["run","--coverage","--coverage.reporter=lcov","--coverage.reportsDirectory=<report_dir>","--coverage.reportOnFailure=true"]`
     plus `--coverage.include=<rel>` for each file (in `files` order) with language typescript that is not
     a test file and lies under the package (posix rel path).
   - `"test"` not in scripts → nil → warn `No package.json test script in {package}; skipping TypeScript coverage.`
   - else `["npx","--yes","c8","--reporter=lcov","--reports-dir",<report_dir>,"npm","test"]`.
4. If the command is not `npm run coverage` and any element's basename == `vitest`, ensure the provider:
   `package/node_modules/@vitest/coverage-v8` is a dir → ok. Else version = `node_modules/vitest/package.json`
   `.version` (str). A JSON error warns `{that path}: {msg}`. spec = `@vitest/coverage-v8@<ver>` or the
   bare name. Warn `Installing {spec} so Vitest can write LCOV. package.json and the lockfile are restored afterward.`
   Snapshot the bytes of `package.json`, `package-lock.json`, `npm-shrinkwrap.json` (nil if absent). Run
   `npm install --no-save --no-package-lock <spec>` in the package. **Always** restore: absent before →
   delete if present now; else rewrite if missing or the bytes differ. ok = (code == 0 && the provider dir
   exists). Not ok → warn `Vitest's coverage provider is missing. TypeScript coverage will score 0%.` and
   skip the package.
5. `rm -rf report_dir; mkdir -p report_dir` (done for `npm run coverage` too). Run the command in the package.
   Non-zero → `TypeScript coverage exited {code} in {package}. TypeScript coverage will score 0%.`
⚠ `npm run coverage` in a sub-package writes `<pkg>/coverage/lcov.info`, which the loader does not read
(only `root/coverage/**` and `root/target/coverage/**`).

**Python**: roots = for each `.py` file (suffix check), the nearest dir containing a marker. The markers are
tried **one at a time**, each walked all the way up, in the order pyproject.toml, setup.cfg, setup.py,
pytest.ini, tox.ini. The first marker that resolves wins, even if another marker is nearer. Fallback: root.
Sorted set. Per root (= package):
1. `report = coverage_report(root, pkg, "python")`. `py` = `pkg/.venv/bin/python` if it is a file, else
   `pkg/venv/bin/python` if it is a file, else `python3`.
2. Ensure module `coverage`: run `[py,"-c","import coverage"]` in pkg. 0 → ok. Else warn
   `Installing coverage for Python coverage. The project requirements are left unchanged.` and run
   `[py,"-m","pip","install","--disable-pip-version-check","coverage"]`. ok iff 0. Not ok → warn
   `coverage is missing in {pkg}. Python coverage will score 0%.` and skip.
3. mkdir -p `report.parent`. `data = report.parent/.coverage`. Unlink report and data (missing ok).
4. kind: `uses_pytest` = `pkg/pytest.ini` or `pkg/conftest.py` exists, or the substring `pytest` appears in
   `pyproject.toml`/`setup.cfg`/`tox.ini` (read with errors=replace). False → `unittest`. True → ensure
   module `pytest` (same probe/install, warning text `Installing pytest for Python coverage. …`) → `pytest`;
   on failure warn `pytest is missing in {pkg}. Falling back to unittest.` → `unittest`.
   (Order: kind is computed before sources.)
5. sources = sorted unique tops over `.py` non-test files under pkg: a rel path with 1 component →
   stem (`carddav.py` → `carddav`), else the first component. Joined with `,`.
6. run = `[py,"-m","coverage","run","--data-file=<data>"] + (["--source=<sources>"] if sources) + (["-m","pytest"] | ["-m","unittest","discover","-s","."])`;
   lcov = `[py,"-m","coverage","lcov","--data-file=<data>","-o",<report>]`.
7. code = run(run). If `data` doesn't exist: if code != 0 warn `Python coverage exited {code} in {pkg}. Python coverage will score 0%.`
   Then stop. Else lcov_code = run(lcov). Non-zero → `coverage lcov exited {lcov_code} in {pkg}. Python coverage will score 0%.`;
   elif code != 0 → `Python tests exited {code} in {pkg}. Coverage was still recorded.`
   (There is no `--branch`, so Python LCOV has no BRDA and Python uses line coverage.)

**Rust**: modules = nearest `Cargo.toml` (any, workspace included) of each `.rs` file. None → `No Cargo.toml; skipping Rust coverage.`
kind: `which cargo-llvm-cov` → llvm-cov; `which cargo-tarpaulin` → tarpaulin. Else warn
`Neither cargo-llvm-cov nor cargo-tarpaulin is installed. Installing cargo-llvm-cov.` If `rustup` is on
PATH: run `rustup component add llvm-tools-preview` in `$HOME` (failure → warn
`rustup component add llvm-tools-preview failed.`). Run `cargo install cargo-llvm-cov --locked` in
`$HOME`. If 0 and it is now on PATH → llvm-cov. Else warn `Rust coverage will score 0%.` and skip Rust. The
kind is computed once, before the module loop. Per module: report; mkdir; unlink; llvm-cov →
`cargo llvm-cov --lcov --output-path <report>`; tarpaulin → `cargo tarpaulin --out Lcov --output-dir <report.parent>`.
Non-zero → `Rust coverage exited {code} in {module}. Rust coverage will score 0%.`

### 5.4 Loading (`load_bundle(root)`)
Reads text with errors=replace. An empty file is skipped. Paths are de-duplicated by resolved path. Glob
order is filesystem order upstream (os.scandir, unsorted). **Go port: sort each glob's matches
lexically.** That only affects precedence between duplicate keys (see LCOV).
- **LCOV**: `root.glob("target/coverage/**/lcov.info")` (`**` = zero or more dirs, so it includes
  `target/coverage/lcov.info`; pathlib matches dot-dirs), then `root.glob("coverage/**/lcov.info")`. Merged
  by `dict.update` per file: a later file **replaces** an earlier record with the same raw SF key but keeps
  the original key position. Use an ordered map.
- **Go profiles**: `root/target/coverage/coverage.out`, `root/target/coverage/go/coverage.out`,
  `root/coverage.out`, `root/*/target/coverage/go/coverage.out`, `root/*/*/target/coverage/go/coverage.out`.
  Segments of the same key are **concatenated**. Empty overall → `go_profile = nil`.
- **JaCoCo**: `root/target/site/jacoco/jacoco.xml`, `root/*/target/site/jacoco/jacoco.xml`,
  `root/*/*/target/site/jacoco/jacoco.xml`. Lists per key concatenated. Empty → nil.
- **Cloverage forms**: if `root/target/coverage` is a dir, every `*.html` under it (recursive, language dirs
  included) whose text contains `forms covered`. Key = its path relative to `target/coverage`, posix,
  without `.html`.

### 5.5 Parsers
**LCOV** (`parse_lcov`): `text.splitlines()`. Python splits on `\n \r \r\n \v \f \x1c \x1d \x1e \x85    `,
so implement the same set. Each line is `strip()`ped (Unicode ws). State: `current_file=nil`.
- `SF:<p>` → store the previous record (if any). `current_file = p` (raw, after the strip); new record.
- `end_of_record` → store; `current_file = nil`.
- Otherwise only when current_file is set: `^DA:(\d+),(\d+)` (prefix match, trailing text allowed) →
  `lines[n] = (hits>0 ? 1 : 0, 1)` (overwrites). `^BRDA:(\d+),([^,]*),([^,]*),(-|\d+)` →
  `branches[line] += (taken is digits && int > 0 ? 1 : 0, 1)`. `\d` is Unicode in Python. Use `[0-9]` and
  accept the tiny divergence.
- At EOF the last record is stored (a missing `end_of_record` is OK). Storing does `out[file] = record` (a
  repeated SF **replaces** the earlier record).
- FN/FNDA/LF/LH/BRF/BRH are ignored.

**Go profile** (`parse_go_profile`): `enumerate(splitlines, 1)`. Skip blank lines and lines starting with
`mode:` (after strip). Split the line on whitespace: it needs exactly 3 fields and a `:` in field 0, else
**fatal** `ValueError("invalid coverage segment on line {n}: {raw!r}")` (the run crashes, exit 1).
`file, rng = f0.split(":",1)`; `a, b = rng.split(",",1)`; `start = int(a.split(".")[0])`,
`end = int(b.split(".")[0])`, `stmts = int(f1)`, `hits = int(f2)`. Any int failure → fatal.
Append `(start, end, stmts, hits)` to `out[file]`.

**JaCoCo** (`parse_jacoco_index`): strip `<!DOCTYPE[^>]*>` (case-insensitive), parse the XML (a malformed
document is fatal). For every `class` element at any depth: `cname = name.replace("/",".")`. For each
**direct child** `method`: the first direct child `counter` with `type="INSTRUCTION"`, or skip.
`missed = int(attr or "0")`, `covered = int(attr or "0")` (a non-int is fatal). `line = int(attr or "0")`,
and a non-int → 0. Key `cname#methodname` → append. Class-level counters are ignored.

**Cloverage HTML**: regex `<span[^>]*title="(\d+) out of (\d+) forms covered"[^>]*>\s*(\d+)&nbsp;` (findall).
For total > 0: `lines[int(line)] = (covered, total)` (later wins).

### 5.6 Matching a source file to a report key (`_select_key(keys, source_path, source_parts)`)
`normalize(p)`: URL-percent-decode (Python `unquote`: UTF-8, errors=replace, `+` kept); `\`→`/`; drop
a leading `file:`; drop leading `./` repeatedly; replace `//`→`/` until none remain. `segments(p)` = the
non-empty parts of `normalize(p).split("/")`.

`candidates(src)` (ordered, deduped, empties dropped):
1. `rel = normalize(src)`
2. `abs = normalize(str(Path(src).resolve()))`. ⚠ Relative paths resolve against the **process CWD**, not
   the project root. Symlinks in existing prefixes are resolved. When src is empty → rel.
3. `no_src = rel` with **one leading** `src/` removed (`^src/`)
4. `abs_no_src = abs` with **every** `/src/` replaced by `/` (non-overlapping, left to right).

`bind_sources(paths)`: `source_parts` = ordered unique tuples of `segments(c)` for every candidate c of every
analysed file (paths as given to analyze: rel or abs).

Algorithm (keys in report insertion order; `items = [(key, normalize(key))]`):
1. **Exact**: for each candidate in order, for each item in order: `norm == candidate` → return the key.
2. **Forward** (the key is longer): for each candidate with parts P (non-empty), for each key with parts K
   where K ends with P: skip if a bound source O with `len(O) > len(P)`, `len(O) ≤ len(K)`, and K ending with O
   exists ("a longer source owns it"). Else collect `(len(K)-len(P), key)`. If anything was collected: keep the
   keys with the minimal extra (unique, ordered). Exactly 1 → return it. More than 1 → **nil** (tie, no
   reverse).
3. **Reverse** (the key is shorter): own = set of the candidates' parts. For each candidate P, each key K
   with `0 < len(K) < len(P)` and P ending with K: skip if some bound O ∉ own with `len(O) ≥ len(K)` ends
   with K. Collect the keys. Unique count == 1 → return it, else nil.

### 5.7 Percent for a function (`percent_for`) — nil = "not in the report"
`coverage_percent(c, t) = t <= 0 ? 0.0 : 100.0 * c / t` (float math: `(100.0*float(c))/float(t)`).
- **java**: `names = [jacoco_class]` (if set) `+ [namespace]` if it differs. pool = concatenation of
  `jacoco[n+"#"+fn.name]` over names (jacoco nil → {}). Empty → nil. If a method in the pool has
  `line == start_line` → the first such. Else the method minimizing `|line - start_line|`, first on ties.
  → `percent(covered, missed+covered)`. The file path is **not** used. Java start_line includes
  annotations, and JaCoCo's line is the first body line, so the nearest-line rule matters.
- **go**: segments = the profile for `select_key(profile keys, fn.path, source_parts)`. A nil profile or no
  key → nil. Sum over segments overlapping `[start_line, end_line]` (exclude when `seg_end < start` or
  `seg_start > end`; columns ignored): `total += stmts`; `covered += stmts if hits > 0`. total == 0 → 0.0.
  Duplicate segments (repeated blocks, merged files) are counted twice.
- **all others** (clojure, typescript, python, rust): first the Cloverage form map (`select_key` over its
  keys; an empty map → skip). Found → `range_pct(forms)`. Else LCOV (empty → nil, no key → nil): if
  `record.branches` is non-empty and the sum of branch totals over the lines in `[start,end]` is > 0 →
  branch % (Σcovered/Σtotal). Else `range_pct(lines)`.
- `range_pct(map)`: Σ over the lines `start..end` of `(covered, total)`. total == 0 → **0.0** (the file is in
  the report but no lines are in range).
So nil only arises when the file/class is absent, and analyze turns nil into 0.0 as well. The nil/0
distinction only matters for `--no-coverage` (nil bundle → nil).

---

## 6. CRAP, rounding, sorting, output

### 6.1 Formula
```
crap(cc, cov) = cov == nil ? nil : float64(cc*cc*u*u*u) + cc   where cc = float64(complexity), u = 1.0 - (cov/100.0)
```
Evaluate left to right: `((((cc*cc)*u)*u)*u) + cc`. **Go may fuse `x*y + z` into an FMA on arm64**. The
explicit `float64(...)` conversion around the product forbids fusion (Go spec). Required for bit parity.

### 6.2 Rounding (`round_metric`)
Python `round(x, 4)` = correctly rounded, ties-to-even on the exact binary value.
Go: `strconv.ParseFloat(strconv.FormatFloat(x, 'f', 4, 64), 64)`. Verified identical on 200k random values
against CPython 3.12, as were `%.1f` and `%.4f`.
`make_entry`: `coverage = round4(cov)`; `crap = round4(crap(cc, coverage_rounded))`. CRAP uses the
**rounded** coverage.

### 6.3 Sort (`sort_entries`) — stable
Key: crap nil → `(1, -complexity, namespace, name, path)`; else `(0, -crap, namespace, name, path)`.
Strings compare by codepoint (Go byte compare). Full ties (e.g. TS getter and setter `x`) keep input order:
files sorted by path, then per-language entry order.

### 6.4 `.metrics/crap.edn`
Path `root/.metrics/crap.edn` (mkdir -p), UTF-8, overwritten each run. Written before the report prints.
```
"{:entries [\n" + "\n".join(rows) + "\n]}\n"          # no entries → "{:entries []}\n"
row = "  {:name " + S(name) + ", :namespace " + S(ns) + ", :complexity " + str(int) + ", :coverage " + N(cov) + ", :crap " + N(crap) + "}"
S(s) = "\"" + s.replace("\\","\\\\").replace("\"","\\\"") + "\""      # nothing else is escaped (raw newlines/tabs pass through)
N(nil) = "nil"; N(v) = t = format(v, ".4f").rstrip("0").rstrip("."); t if "." in t else t + ".0"
```
Examples: `100.0`→`100.0`, `0.0`→`0.0`, `66.6667`, `4.5926`, `3.1`, `30.0`. inf → `inf.0`, nan →
`nan.0` (unreachable). Row order = sorted order.

### 6.5 Report (stdout)
```
nw  = max(30, min(48, max(runelen(name))))      # only when entries non-empty; else 30
nsw = max(35, min(72, max(runelen(namespace)))) # else 35
header = pad_right("Function", nw) + " " + pad_right("Namespace", nsw) + " " + pad_left("CC",4) + " " + pad_left("Cov%",7) + " " + pad_left("CRAP",8)
lines = ["CRAP Report", "==========="]
if entries: lines += ["Languages: " + ", ".join(sorted(set(languages))), ""]
lines += [header, "-" * runelen(header)]       # = nw + nsw + 23 (88 at defaults); rows are 87
row = pad_right(name, nw) + " " + pad_right(ns, nsw) + " " + "%4d"(cc) + " " + COV + " " + CRAP
COV  = nil ? "  N/A " : "%5.1f%%"(cov)          # 6 chars, so rows are 1 char narrower than the header
CRAP = nil ? "     N/A" : "%8.1f"(crap)
output = "\n".join(lines + rows + [""])        # ends with exactly one "\n"
```
Padding counts code points and never truncates (long names push the columns). Go `fmt` `%-*s` pads by
runes, which is correct. Languages are the `language` values (`typescript` for JS).

Golden (fixture §8.9):
```
CRAP Report
===========
Languages: python, typescript

Function                       Namespace                             CC    Cov%     CRAP
----------------------------------------------------------------------------------------
GET /users                     demo.app                               2   0.0%      6.0
choose                         demo.box                               4  66.7%      4.6
view                           demo.app                               2  50.0%      2.5
open                           demo.box.Box                           2  50.0%      2.5
```
stderr: `Wrote /private/tmp/…/.metrics/crap.edn`. With `--threshold 3`: also
`CRAP threshold exceeded: 6.0 > 3.0`, exit 2.

### 6.6 stderr message catalogue
`Wrote {abs}` · `CRAP threshold exceeded: {a:.1f} > {b:.1f}` · `{git stderr stripped | "git status failed"}` ·
`+ ({cwd}) {cmd}` · `Coverage command failed to start: {exc}` ·
`Coverage command exited {c}. Reports already on disk will not be read.` · plus the per-language warnings in §5.3.
stdout: the report, `No source files to analyze.`, HELP (exit 0), and child-process output.

---

## 7. Feasibility: stdlib-only Go parsers vs tree-sitter

Shared architecture: a per-language **lexer** that yields significant tokens with byte offsets and lines,
handling comments, strings, and nested interpolation. On top of it, a **structural scanner** that builds a
light region tree (functions, classes, route callbacks, macros, match blocks…) from bracket/indent
structure plus a handful of grammar rules. `complexity(entry)` = 1 + the decision markers inside the entry's
byte span, minus markers inside skipped sub-regions. That is equivalent to §0.3 because every decision kind
maps to a token or keyword occurrence. A full AST is not required except for Go (use `go/ast`).

Only `start_line`/`end_line` feed coverage, and only through lines that carry DA/BRDA/segments. Off-by-a-few
end lines over blank or comment lines are harmless. Start lines must be exact (decorator/annotation rules).

| language | expected parity | why |
|---|---|---|
| Go | **exact** (valid code) | `go/ast` is the reference grammar; shims in §4.3 |
| Clojure | **exact** | upstream is already a hand scanner; straight port (Unicode-ws/floor-div caveats) |
| Java | **near-exact** | decisions are keywords. Only `?` (wildcard) and `default` (modifier/annotation) need context |
| Python | **near-exact** | keyword-countable. Needs a real tokenizer (indent, f-strings) and soft-keyword `match`/`case` |
| Rust | **near-exact** | keyword-countable + arm counting. Risks: let-chain grammar drift, `?`/`&&`/`||` prefix vs infix |
| TS/TSX/JS | **near** on mainstream code, **approximate** on edge typing/JSX | `?` ternary vs type-level `?`, JSX vs generics vs comparison, regex vs division, ASI for end lines |

### 7.1 Python — must handle
- Tokenizer: physical/logical lines; implicit joining inside `()[]{}`; `\` continuation; INDENT/DEDENT
  (tabs: Python rounds to multiples of 8; consistency within a file is all that matters here);
  comment-only/blank lines don't affect indentation; CRLF already normalized.
- Strings: prefixes `r b u f rb br fr rf t tr…` in any case; `'`, `"`, `'''`, `"""`; escapes (skip in non-raw).
  **f-strings**: recurse into `{…}` interpolations (PEP 701: nested same quotes, nested f-strings, `{{`/`}}`
  escapes, `!r`, `=`, and format specs that themselves contain `{…}`). Decisions inside count. Plain/byte
  strings are opaque.
- Statement starts: `def`, `async def`, `class`, decorators `@…` (start_line stays on the `def`/`async`
  token), `if/elif/while/for/try/except/with/match/case`.
- Counting: `if`, `elif`, `while`, `except` (and `except*`), `and`, `or` → 1 each, anywhere in code.
  `for` → only at statement start (incl. after `async`). `for` inside brackets = comprehension = 0.
  `case` → only as a clause head inside a `match` block (+1). `if` inside a case pattern line is the guard → +1.
- Soft keywords: `match` is a statement iff it sits at statement start, is not followed by `=`, `.`, `(…)` +
  newline (a call), `[`… used as an expression statement, and the logical line ends with `:` followed by an
  indented block whose statements begin with `case`. Simplest robust test: at statement start, `match`
  followed by tokens up to a top-level `:` at end of line, and the next logical line is indented and starts
  with `case`. `match = 1`, `match(x)`, `match.x` → identifier.
- Spans: entry = `def` token (or `async`) → last token of its indented block (or of the same line for a
  one-liner `def f(): return 1`). Nested-class exclusion span = the `class` statement through the end of
  its block (class decorators are outside → counted in the parent function).
- Risky: Python 2 sources (print statements, backticks); `match`/`case` used as names inside match blocks;
  form feeds in indentation; files tree-sitter only partially parses (ERROR recovery differs).

### 7.2 TypeScript / TSX / JavaScript — must handle
- Lexer: `//`, `/* */`, hashbang `#!` on line 1; strings `'…'`/`"…"` with escapes and line continuations;
  **template literals** with nested `${ … }`, which can contain braces, strings, regexes, and nested templates.
  Keep a mode stack. Numeric literals incl. `1_000`, `0x`, `1n`, `.5`.
- **Regex vs division**: `/` starts a regex when the previous significant token is not an expression end
  (identifier/keyword-that-ends-expr such as `this`/`super`, literal, `)`, `]`, `}`-of-expression, `++`/`--`
  postfix). Keywords like `return typeof case do else in of new delete void throw yield await instanceof`
  are followed by a regex. Hard spots: `)` after `if (…)`/`while (…)`/`for (…)` → regex. `}` ending a
  block → regex; `}` ending an object literal → division. Track paren/brace provenance on a stack.
  Regex bodies: classes `[…]` may contain `/`. Flags follow.
- **`?` disambiguation** (TS): ternary iff in expression context. Not counted in type contexts: after a
  `:` type annotation (params, variables, return types, properties), `type X = …`, inside `<…>` type
  args/params, after `as`/`satisfies`, in `implements`/`extends` heritage, `interface` bodies (skipped
  anyway), optional members `x?:`, `m?(`, `[k]?:`, mapped types, function types `(a?: T) => U`. Type
  contexts end at `=` (initializer), `,`/`)` at the opening depth, `{` of a function body, `;`/ASI.
  `?.` followed by a decimal digit = `?` + number.
- **`<` disambiguation**: comparison vs type args (`f<T>(x)`, `new Map<K,V>()`) vs `.ts` type assertion
  `<T>expr` vs JSX (`.tsx`/`.js`/`.jsx`) vs generic arrow `<T,>(x) =>` (TSX). The counting impact is
  indirect: misreading `<` corrupts the regex/JSX/brace state. TS rule: after an identifier, try to parse
  a balanced type-argument list followed by `(`, `` ` ``, or a non-expression-start token. Else comparison.
- **JSX**: element/fragment tags, attributes (strings opaque; `{…}` expression containers are code), JSX text
  (opaque; `&&` in text is not counted), self-closing tags, nested elements inside expressions, entities.
- Keywords as property names: after `.`/`?.`, as object keys `if:`, as method names `if() {}` → not
  decisions (and the name is valid for entries).
- Decision tokens: `if`, `for`, `while` (excluding the trailing `while` of `do…while`), `do`, `catch`
  (statement keyword, not `.catch`), `?` ternary, `case`/`default` heads inside `switch` bodies, `&&`, `||`,
  `??`, `?.`. Not `&&=`/`||=`/`??=`, and not `default` in `export default` or as an object key.
- Structure: function declarations vs expressions (statement position ⇒ declaration; `export`/`export
  default` + `function name` ⇒ declaration; anonymous default ⇒ skip); `async`; generator `*` (exclude, and
  note that generators don't block nesting, §4.2); class declarations (statement position, with name; incl.
  `export`, `export default class Name`, `abstract class`, decorators) vs class expressions; class bodies:
  methods with modifiers/accessors/`static`/`async`/`*`/`override`/`readonly`/`declare`/`public`…; fields
  (skip); index signatures; overload signatures (no body); private names; computed keys.
- Arrow detection: `ident =>`, `async ident =>`, `(…) =>`, `async (…) =>`, `<T,…>(…) =>`, with an optional
  return type `): T =>`. The return type may contain `=>` (function types): parse types structurally
  (balanced `()[]{}<>`), not by scanning to the first `=>`. Arrow body = block, or an AssignmentExpression
  extending to the next `,`/`)`/`]`/`}`/`;` at depth 0, or an ASI break. Arrow parameters can contain
  default values with decisions (`(a = b ? 1 : 2) =>`), which count.
- Top-level `const/let` declarators: split at depth-0 commas. Value must *be* the function (no trailing
  call/member/operator/`as`). End line = end of the value: ASI approximation (a newline ends the statement
  unless the next token is `(`, `[`, `` ` ``, `.`, `?.`, a binary/ternary operator, or `,`).
- Route calls: member call with property ∈ set (case-insensitive), argument splitting at depth-0 commas,
  paren unwrapping, string-literal decoding (fragments only, escapes dropped), templates raw, `.route()` chain
  walk, `#n` claiming, byte-ordered output, and span exclusion from every enclosing entry.
- Risky: TS type-level `?` inside function bodies (local `type`s, generic constraints, `satisfies`); JSX vs
  `<T>` in `.js` files that use Flow; regex after `)`/`}`; decorators with arguments; `enum` initializers;
  `declare module`/`namespace` blocks; ASI-dependent end lines; files tree-sitter recovers from with ERROR
  nodes.

### 7.3 Go — use go/parser
Exact for valid code. Shims: the missing package clause (§4.3), `SkipObjectResolution`, CRLF normalization
first. Divergent only on syntax errors (accepted).

### 7.4 Java — must handle
- Lexer: `//`, `/* */`, strings, **text blocks** `"""…"""`, char literals `'\''`, `'\u0041'` (no unicode-escape
  preprocessing, same as tree-sitter), numeric literals with `_`, `0x`, `L`, `f`, `d`, `1e3`.
- Decisions: keywords `if`, `for`, `while` (minus the do-while tail), `do`, `catch`, `case` (each label head),
  `default` iff followed by `:` or `->`, `?` iff the previous significant token is not `<`, `,`, or an
  annotation (wildcards `<?>`, `<? extends T>`, `<? super T>`), `&&`, `||`. Generic bounds `A & B` use a single
  `&`, so they're safe.
- Structure: type declarations (`class|interface|enum|record|@interface` + identifier), bodies, members:
  method = `[modifiers/annotations] [<T>] Type name(…) [throws …] {` where a return type is present
  (constructor = `Name(` with no type, Name == class), `;` or `default …;` (annotation element) = no body.
  Anonymous class = `new T(…) {`. Local classes inside method bodies. Lambdas `->` (to know "inside
  lambda" in field initializers). Enum constants with bodies. Records with compact constructors `Name {`.
  Initializer blocks `{`/`static {` at member level. Annotations with args `@A(x = "y")` in modifiers
  (start_line = first annotation/modifier token).
- Risky: generic method calls `this.<T>m()`; annotations on types inside expressions; `non-sealed`, `sealed`,
  `permits`; `yield` as an identifier; unicode escapes forming keywords (ignored by both sides).

### 7.5 Rust — must handle
- Lexer: line/doc comments, **nested** block comments; strings with escapes; **raw strings**
  `r"…"`, `r#"…"#` (N hashes), `br…`, `cr…`, byte strings `b"…"`, C strings `c"…"`; **char vs lifetime**:
  `'` + (escape | one char) + `'` = char (`'a'`, `'\''`, `'\u{1F600}'`, `'{'`), else lifetime/label (`'a`,
  `'static`, `'outer:`). Byte chars `b'x'`. Numbers: `1..2` is a range (not the float `1.`), `x.0.1` tuple
  index, suffixes `1u8`, `1e3`, `0x_FF`.
- Macro invocations: path `!` followed by a delimited token tree → opaque region (no decisions). Includes
  `macro_rules! name { … }`. Attributes `#[…]`/`#![…]` → opaque.
- Decisions: keywords `if` (not a match-arm guard, i.e. not between an arm pattern and `=>`), `for` (not
  after `impl…`/in `for<'a>`; a `for` expression is `for PAT in EXPR {`), `while`, `loop`; match arms = count
  of `=>` at the match block's own brace depth (exact); `?` postfix iff the previous token ends an
  expression (excludes `?Sized` after `:`/`+`/`(`/`<`/`,`); `&&`/`||` iff infix (previous token ends an
  expression; a prefix `||` is a closure, a prefix `&&` is a double reference; `&&` in types/patterns follows
  `:`/`(`/`,`/`|`/`&`/`=`). Let chains: in an `if`/`while` condition with a depth-0 `let`, depth-0 `&&` → 0.
- Structure: `fn` items with qualifiers; bodies vs `;` (trait/extern signatures); fn-in-block exclusion;
  `impl` headers (type extraction per §4.5 needs a mini type parser: generic args `<…>` with nested `<>`,
  `::` paths, `&`/`dyn`/`[`/`(` forms → none); `trait`; inline `mod name {`; `mod tests`.
- Risky: let-chain shape depends on the grammar version (pin to **[obs]**); `if` inside closures in match
  guards; `>>` closing nested generics; `|` closure params vs or-patterns; `?` after a block-like expression
  `match {…}?`; async closures.

### 7.6 Clojure
Port §4.6 line by line. Caveats: Python `isspace`/`\s` Unicode sets; floor `//` and `%` for negative
counts; regex alternation order (Go RE2 is leftmost-first like Python for alternations, so it's
equivalent); `findall` non-overlap semantics. Expected: exact.

---

## 8. Test cases (→ Go unit tests + harness)
U = upstream test, O = observed on upstream **[obs]**. Format: `path` (root `/proj` unless noted) →
`[(namespace, name, cc)]` in output order.

### 8.1 Python
- U `src/demo/box.py`:
```python
def choose(x, ready):
    if x > 0 and x < 10:
        return 1
    elif x == 0 or ready:
        return 0
    for i in xs:
        if i == 2:
            return i
    while ready:
        break
    try:
        return x
    except ValueError:
        return 0
    except KeyError:
        return -1
    match x:
        case 1:
            return 1
        case 2 | 3:
            return 2
        case _:
            return 0
    return x if x > 0 else 0

class Box:
    def open(self, flag=False):
        return 1 if flag else 0

    @staticmethod
    def shut(self):
        xs = [n for n in range(3) if n]
        return xs

def outer():
    def inner(n):
        if n:
            return 1
        return 0
    return inner(1)

async def load():
    if True:
        return 1
```
→ `(demo.box,choose,14) (demo.box.Box,open,2) (demo.box.Box,shut,2) (demo.box,outer,2) (demo.box,load,2)`
- U `src/demo/sample.py`: `class Outer:\n    class Inner:\n        def tick(self, ready, ok):\n            if ready and ok:\n                return 1\n            return 0\n\ndef factory():\n    class Hidden:\n        def secret(self):\n            if True:\n                return 1\n            return 0\n    return Hidden()\n`
  → `(demo.sample.Outer.Inner,tick,3) (demo.sample,factory,1) (demo.sample.Hidden,secret,2)`
- U namespaces, `def f():\n    return 1\n`: `/proj/src/demo/app.py` (root `/proj`)→`demo.app`; `./src/demo/app.py` (root `.`)→`demo.app`;
  `pkg/src/demo/__init__.py` (root nil)→`demo`; `__init__.py` (nil)→`__init__`.
- U `src/demo/__init__.py`: `def literal():\n    """if and or for while except"""\n    text = "if x and y"\n    # if x: return 1\n    return text\n` → `(demo,literal,1)`
- U `huge.py`: `def huge():\n    return 1 + 1 + … (1200 terms)\n` → one entry `huge` (no stack overflow).
- U `src/app.py`: `def choose(x):\n    return x\n` → names `[choose]`. `src/app.py` `def run():\n    return 1\n` → `run`, language `python`.
- O (`a.py` → namespace `a`):
  - `def f(a,b):\n    return f"{a if b else 0}"\n` → 2
  - `def f(x):\n    match x:\n        case 1 if x > 0:\n            pass\n        case _:\n            pass\n` → 4
  - `def f():\n    try:\n        pass\n    except* ValueError:\n        pass\n` → 2
  - `async def f(xs):\n    async for x in xs:\n        pass\n    async with a:\n        pass\n    return [y async for y in xs if y]\n` → 3
  - `g = lambda x: 1 if x else 0\ndef f(x=1 if a else 2):\n    h = lambda y: y and 1\n    return h\n` → only `f` = 3, lines 2–4
  - `class A:\n    @property\n    @other\n    def m(self):\n        return 1\n` → `(a.A,m,1)` lines **4–5**
  - `def f(a,b,c):\n    return a and b or c and not a\n` → 4
  - `def f(xs):\n    return [x for ys in xs if ys for x in ys if x if x > 1]\n` → 4
  - `def f(x):\n    while x:\n        pass\n    else:\n        pass\n    for i in x:\n        pass\n    else:\n        pass\n    assert x and x\n    with open(x) as fh: pass\n    y = (z := 3)\n` → 4
  - `match = 1\ndef f(match):\n    match(match)\n    return match\n` → 1
  - `def f():\n    def g():\n        class C:\n            def m(self):\n                if a: pass\n        return C\n    return g\n` → `(a,f,1) (a.C,m,2)`
  - `class A:\n    def m(self):\n        class B:\n            def n(self):\n                if a: pass\n        return B\n` → `(a.A,m,1) (a.A.B,n,2)`
  - `def f(x):\n    return {k: v for k, v in x if k} or {a for a in x if a} or (g for g in x if g)\n` → 6
  - `def f(x):\n    if x:\n        pass\n    elif x:\n        pass\n    else:\n        pass\n` → 3
  - `def f(x):\n    print(f"{x!r:>{10 if x else 5}}")\n` → 2 · `def f(x):\n    return f"{'a' if x else 'b'}" + rf"{x and y}"\n` → 3
  - `def f(x):\n    try:\n        pass\n    except (A, B):\n        pass\n    except:\n        pass\n    finally:\n        pass\n` → 3
  - `def f(x):\n    match x:\n        case [1, 2] | {"a": 1}:\n            pass\n        case Point(x=0) if x and y:\n            pass\n` → 5
  - `class A:\n    x = 1 if a else 2\n    def m(self): pass\n` → `(a.A,m,1)`
  - `def f():\n    return 1\n    # trailing comment\n\n# outer\ndef g():\n    pass\n` → f lines 1–3, g lines 6–7

### 8.2 TypeScript / JavaScript
- U `src/demo/box.ts`:
```ts
export function choose(x: number): number {
  if (x > 0 && x < 10) return 1;
  for (let i = 0; i < x; i++) {
    if (i === 2) return i;
  }
  try {
    return x;
  } catch (e) {
    return 0;
  }
  return x > 0 ? 1 : 0;
}

export class Box {
  open(flag?: boolean): number {
    return flag ? 1 : 0;
  }
}

const arrow = (n: number) => (n > 0 ? n : 0);

function outer() {
  function inner(n: number) {
    if (n) return 1;
    return 0;
  }
  return inner(1);
}
```
→ `(demo.box,choose,7) (demo.box.Box,open,2) (demo.box,arrow,2) (demo.box,outer,2)`
- U `src/ui/view.tsx`: `export function View(ok: boolean, ready: boolean) {\n  return ok && ready ? 1 : 0;\n}\n` → `(ui.view,View,3)`
- U `src/demo/box.ts`: `export function choose(a, b, c) {\n  return a ?? b?.c ?? c?.() ?? c?.[0];\n}\nexport function text() {\n  return "a ?? b?.c";\n}\n` → `(choose,7) (text,1)`
- U `src/demo/routes.ts` (all namespace `demo.routes`):
```ts
export function mount(app) {
  const extra = [1].map((n) => (n ? 1 : 0));
  app.get("/users", (req, res) => {
    if (req.query.q) return 1;
    return 0;
  });
  app.post("/users", function (req, res) {
    return req.body ?? {};
  });
  app.use((req, res, next) => next());
  app.route("/items").get((req, res) => res.send(1)).post((req, res) => res.send(2));
  return extra;
}
app.get("/health", async (req, res) => res.send(req.query.ok ?? "no"));
app.get("/users", (req, res, next) => next(), (req, res) => res.send(req.id ?? 0));
app.delete(`/users/${id}`, ((req, res) => res.send(1)));
```
→ `(mount,2) (GET /users,2) (POST /users,2) (USE,1) (GET /items,1) (POST /items,1) (GET /health,2) (GET /users#2,1) (GET /users#3,2) (DELETE /users/${id},1)`
- U `src/demo/box.ts`: `function outer() {\n  const values = [1].map((n) => (n ? 1 : 0));\n  return values;\n}\n` → `(outer,2)`
- U `src/demo/app.mjs` and `src/demo/app.js`: `export function choose(a, b, c) {\n  return a ?? b?.c ?? c?.() ?? c?.[0];\n}\napp.get("/users", (req, res) => req.id ?? 0);\n` → `(demo.app,choose,7) (demo.app,GET /users,2)`
- U namespaces for `export function choose(): number { return 1 }\n`: `/app/src/demo/box.ts` (root `/app`)→`demo.box`; `./demo/box.ts` (nil)→`demo.box`; `/app/pkg/src/demo/box.mts` (nil)→`demo.box`.
- U `src/demo/box.ts`: `\n    function choose(): void;\n    class Box { open(): void; }\n    const plain = 1;\n    const arrow = (n: number) => n;\n    ` → `[(demo.box,arrow)]`; `const obj = { open() { return 1 } }\n` → `[]`; `class Box { [name]() { return 1 } }\n` → `[]`.
- U analyze: `src/low.ts`/`src/high.ts` = `export function {name}(x: number) {\n  if (x) return 1;\n  return 0;\n}\n`, bundle lcov `{"src/low.ts": {1:(1,1),2:(1,1),3:(1,1)}}` → `[(high,0.0,6.0),(low,100.0,2.0)]`; nil bundle → coverage/crap nil.
- O (`a.ts` unless noted, namespace `a`):
  - `export default function () { if (a) {} }\nexport default function named() { if (a) {} }\n` → `(named,2)` (ts & js)
  - `function* gen() { if (a) {} }\nasync function af() { if (a) {} }\nvar v = () => { if (a) {} };\nlet l = function () { if (a) {} }, m = async (x) => x ? 1 : 2;\n` → `(af,2) (l,2) (m,2)`
  - `const K = class { m() { if (a) {} } };\nclass A { m() { return { n() { if (a) {} } }; } }\n` → `(a.A,m,2) (a.A,n,2)`
  - `function f(a) { return a?.b.c?.d; }\nfunction g(a) { a ??= 1; a ||= 2; a &&= 3; return a; }\n` → `(f,3) (g,1)`
  - `class S { async run() { router.get("/x", (req) => { if (a) {} }); } }\n` → `(a.S,run,1) (a,GET /x,2)`
  - `app.get("/a\\nb", (r) => 1);` (JS source text `"/a\nb"`) → `GET /ab`; `app.get('/q', …)` → `GET /q`; `app.get(path, handler)` → none; `app.get(handler, "/late", function () {})` → `GET /late`; `app.GET("/up", () => 1)` → `GET /up`; `cache.get("k", () => 1)` → `GET k`
  - `function f(xs) { for (const x of xs) {} for (const k in xs) {} for await (const y of xs) {} label: while (1) { break label; } do {} while (0); switch (a) { case 1: case 2: break; default: } }` → 9
  - `class A { get x() { return a ? 1 : 2; } set x(v) { if (v) {} } static s() {} #p() { if (a) {} } "q"() {} 1() {} }` → `(a.A,x,2) (a.A,x,2) (a.A,s,1)`
  - `const {a, b} = obj;\nconst f = (() => 1);\nconst g = memo(() => a ? 1 : 2);\nif (x) { const h = () => a ? 1 : 2; }\n` → `(h,2)` line 4
  - `function o() { function i() { if (a) {} } const k = () => b ? 1 : 2; return class { m() { if (c) {} } }; }` → `(o,4)`
  - `function f() { return a\n  ? b\n  : c; }\n` → `(f,2)` lines 1–3
  - `namespace N { export function f() { if (a) {} } }\nabstract class B { abstract m(): void; n(): void { if (a) {} } }\ninterface I { m(): void }\nenum E { A }\ntype T<X> = X extends string ? 1 : 2;\nfunction g<T>(x: T): x is string { return (x as any)!.y ?? 1; }\nclass P { handler = () => { if (a) {} }; constructor(private x: number) { if (a) {} } }\ndeclare function d(): void;\nfunction over(a: string): void;\nfunction over(a: any) { if (a) {} }\n` → `(a,f,2) (a.B,n,2) (a,g,2) (a.P,constructor,2) (a,over,2)`
  - `a.tsx`: `export function V({ok}) { return <div>{ok && <span>a && b || c</span>}{ok ? <A/> : null}</div>; }\nconst W = <T,>(x: T) => x ? 1 : 0;\n` → `(V,3) (W,2)`; `a.jsx`: `export function V({ok}) { return <div>{ok && <span>a && b</span>}</div>; }` → `(V,2)`
  - `const a = 1 / 2 / 3;\nfunction r(s) { return s.replace(/if|&&|\?\./g, "x") / 2 > 1 ? \`t ${a ? \`n${b ?? c}\` : ""}\` : 0; }` → `(r,4)`
  - `export const x = { a: () => { if (q) {} } };\nexport function f() { const o = { m() { if (q) {} } }; return o; }` → `(f,2)`
  - `app.route("/items").get((req, res) => 1).post((req, res) => 2);\napp.route(\`/t\`).all(() => 1);\napp.use("/static", express.static("p"), (req,res) => 1);\napp.get("", () => 1);` → `GET /items, POST /items, ALL /t, USE /static, GET`
  - `app.get("/x", async function named(req) { if (a) { app.post("/y", () => 1); } });` → `(GET /x,2) (POST /y,1)`
  - `export default class { m() { if (a) {} } }\nexport default class Named { m() { if (a) {} } }\n@dec class D { m() { if (a) {} } }` → `(a.Named,m,2) (a.D,m,2)`
  - `function f(x?: number, y: string = a ? "1" : "2"): (a?: number) => void { type C<T> = T extends string ? 1 : 2; let q: C<number> | undefined; const o: { a?: number } = {}; return (a?: number) => {}; }` → 2
  - `function f() { return a ?.5 : 1; }` → 2 · `function g() { return <any>x && y; }` → 2 · `function h() { return f<string>(x) || g<T, U>(y); }` → 2 · `function k() { return a < b && c > d; }` → 2
  - `const f = (): ((a: number) => void) => (a) => { if (a) {} };\nconst g = async <T,>(x: T): Promise<T> => x ?? x;\nconst h = function named() { if (a) {} };\nconst i = async function* () { if (a) {} };` → `(f,2) (g,2) (h,2)`
  - `const f = (x) => x\nconst g = (y) => y\n  ? 1\n  : 2\nfunction z() {}\n` → `(f,1) L1–1, (g,2) L2–4, (z,1) L5`
  - `function* gen() { function inner() { if (a) {} } const ar = () => b ? 1 : 2; }\nclass S { static { const sb = () => c ? 1 : 2; } }` → `(inner,2) (ar,2) (sb,2)`
  - `a.js`: `function f() { return x / 2 / y ? /re/.test(s) : 1; }\nfunction g() { let a = b\n/c/i.exec(d) ? 1 : 0; }` → `(f,2) (g,2)` g lines 2–3
  - `a.js`: `export const f = () => { if (a) {} }\nmodule.exports = function () { if (a) {} };\nexports.h = async (e) => { if (a) {} };` → `(f,2)` only
  - `function f(o) { switch (o) { case 1: { if (a) {} } default: return o ? 1 : 2; } }\nconst obj = { if: 1, catch: 2, default: 3 };\nfunction g() { p.catch(() => 1).finally(); return obj.if || obj.default; }` → `(f,5) (g,2)`
  - `class A { if() { return 1; } catch() { if (a) {} } }\nfunction t() { try { } catch { } finally { } }` → `(a.A,if,1) (a.A,catch,2) (a,t,2)`

### 8.3 Go
- U `sample.go` (no go.mod reachable): `package sample\n\nfunc Branchy(x int, ch chan int) int {\n\tif x > 0 && x < 10 {\n\t\treturn 1\n\t}\n\tswitch x {\n\tcase 1, 2:\n\t\treturn 2\n\tdefault:\n\t\treturn 3\n\t}\n}\n` → `(sample,Branchy,5)`
- U `widget.go`: `package sample\n\ntype Widget struct{}\n\nfunc (w *Widget) Run() {\n\tfor i := 0; i < 10; i++ {\n\t}\n\tfor _, item := range items {\n\t}\n}\n\nfunc Simple() int { return 1 }\n` → `(sample.Widget,Run,3) (sample,Simple,1)`
- U go.mod `module github.com/acme/demo\n` at the root; file `internal/game/board.go`: `package game\n\ntype Board struct{}\n\nfunc (b *Board) Place() {\n\tif ready && ok {\n\t\treturn\n\t}\n}\n` → `(github.com/acme/demo/internal/game.Board,Place,3)`
- U `func Orphan() int { return 1 }\n` → `("",Orphan)`.
  `package p\nfunc Declared(x int)\nfunc Real() int { return 1 }\nfunc () Run() {}\nfunc (w *Widget) () {}\n` → `[(p,Real)]`.
  go.mod `module example.com/demo` + `board.go` at the module root → namespace `example.com/demo`.
- U `board.go` outside root (`analyze_files([tmp/board.go], tmp/other)`) → entry `Place`, path = the absolute path. `notes.md` → `[]`.

### 8.4 Java
- U SCORE (`class Sample { int score(boolean a, boolean b, int[] values) { for(;;){} for(int v:values){} while(a){a=false;} do{b=false;}while(b); if (a && b || values.length > 0) {} try { return a ? 1 : 0; } catch (RuntimeException ex) { return 2; } } }`) → `(score,10)`
- U NESTED (§ test_java.py, verbatim) → `(nested,13) (switched,4)`
- U ANONYMOUS: `outer()` containing `new Runnable() { @Override public void run() { if (true) {} } }` → `(outer,1)`
- U `package demo.pkg; public class Board { Board() { if (true) {} } public void place(int x) { if (x > 0 && ready) return; } class Inner { void tick() { return; } } }` →
  `(demo.pkg.Board,place,3) (demo.pkg.Board.Inner,tick,1)`; jacoco `demo.pkg.Board`, `demo.pkg.Board$Inner`
- U keywords in a string, `//`, and `/* */` → `(stable,1)`
- U `class Sample { Runnable r = () -> { class Local { int score() { if (true) return 1; return 0; } } return null; }; int outer() { return 1; } }` → `[outer]`
- U a 1200-term expression in `huge()` → `[huge]`
- O `package a.b;` S with: an anonymous Runnable field → `(a.b.S,run,2)` L3; enum E constant-body `f` → `(a.b.S.E,f,2)`, `g` → `(a.b.S.E,g,1)`; interface default `d` → `(a.b.S.I,d,2)`; record `y` (ternary) → `(a.b.S.R,y,2)`, compact ctor ignored; `@Override\n@Deprecated\npublic int h(int v) { return switch (v) { case 1, 2 -> 1; case 3 -> { yield 2; } default -> 3; }; }` → `(a.b.S,h,4)` **L7–11** (the annotation line); `l()` with a lambda `if` + local class → `(l,2)`; `p(Object o)` with `instanceof … &&` and a pattern switch with a `when` guard + default → `(p,5)`
- O `/* package wrong.pkg; */\npackage right.pkg;\nclass A { void m() {} }` → `(right.pkg.A,m,1)`
- O `class A { static { if (a) {} } void m() { new Thread() { public void run() {} }; try (var x = r()) {} catch (A | B e) {} finally {} } }\n@interface Ann { String v() default ""; }` → `(A,m,2)` only
- O `class G<T extends Comparable<T>> { <U> U m(java.util.List<? extends U> l) { return l.isEmpty() ? null : l.get(0); } }` → `(G,m,2)`

### 8.5 Rust
- U Cargo `[package]\nname = "demo-game"`, `src/lib.rs` = SOURCE (test_rust.py) → `(demo_game,choose,6) (demo_game::Sample,run,2) (demo_game::Sample,draw,1) (demo_game,outer,2)` (`mod tests` hidden)
- U `impl Trait for Vec<crate::Sample> {\n    fn draw(&self) {}\n}\n` → `draw`; namespace ends with `Vec` (observed: `demo::Vec`)
- U `src/bin/tool.rs` → namespace `tool`; `/tmp/loose.rs` (outside the crate) → name `loose` (namespace = `crate::loose`, since there's no Cargo.toml above /tmp); `notes.rs` with root nil → `crate::notes`
- U `_file_modules("/tmp/loose.rs", "demo", "/projects/demo")` → `["demo"]`
- U `src/game/board.rs`: `pub fn place() -> i32 { if ready { 1 } else { 0 } }` → `(demo::game::board,place,2)`
- U `trait Foo {\n    fn draw(&self) {}\n}\n` in `src/lib.rs` → `(demo,draw)`
- O (`x.rs`, no Cargo → `crate::x`):
  - `fn f(a: bool) { println!("{}", if a { 1 } else { 2 }); assert!(a && b); vec![x?]; }` → 1
  - `fn f() { if let Some(x) = y && x > 0 && let Ok(z) = w { } while let Some(v) = it.next() {} }` → 3
  - `fn f() { let c = |x| if x { 1 } else { 2 }; let d = async { a? }; match x { 1 | 2 if g => 1, _ => 2 } }` → 5
  - `fn f() { struct S; impl S { fn m(&self) { if a {} } } }` → `(crate::x,f,2) (crate::x::S,m,2)`
  - `mod a { mod tests { fn t() {} } mod b { fn g() { if a {} else if b {} else {} } } }\n#[cfg(test)]\nmod test { fn h() {} }` → `(crate::x::a::b,g,3) (crate::x::test,h,1)`
  - impls: `impl<T: Clone> fmt::Display for Wrapper<T> where T: X { fn fmt… }` → `crate::x::Wrapper`; `impl Foo for &Bar` → `crate::x`; `impl dyn Tr` → `crate::x`; `impl<T> Foo<T>` → `crate::x::Foo`; `impl crate::a::B` → `crate::x::B`
  - `fn f<'a>(x: &'a str) -> char { let c = 'a'; let d = '\''; let s = r#"if a && b { "#; let b = b'x'; /* if /* nested */ && */ if c == '{' {} c }` → 2
  - `macro_rules! m { ($x:expr) => { if $x {} }; }\nfn g() { m!(a); }\n#[test]\nfn attr() {}\npub(crate) async unsafe fn q() {}\nextern "C" { fn ext(); }\ntrait T { fn s(); fn d(&self) { if a {} } }` → `(g,1) (attr,1) L4 (q,1) (d,2)`
  - `fn f() -> Result<(), E> { let x = a()?.b()?; loop { break; } for i in 0..3 {} Ok(()) }` → 5
  - `fn f() { if let Some(x) = a && b {} }`→2 · `fn g() { if a && let Some(x) = b {} }`→2 · `fn h() { if (a && b) && let Some(x) = c {} }`→3 · `fn i() { while let Some(x) = a && b || c {} }`→3 · `fn j() { let ok = a && b; if let Some(x) = (a && b) {} }`→4
  - `fn f<T: ?Sized>(x: &&T) -> bool where T: Clone + ?Sized { let c = || a && b; let d = move || x?; let e = &&x; for<'a> |y: &'a u8| 1; 'outer: loop { break 'outer; } true }` → 4
  - `fn f() { match x { A => match y { B => 1, C => 2 }, D | E if z => { 3 } F => 4, } }` → 6

### 8.6 Clojure (`cyclomatic_complexity(text)`)
`(defn foo [])`→1 · `(defn foo [x]\n  (+ x 1))`→1 · `(if x 1 0)`→2 · `if-not`→2 · `if-let`→2 · `when`→2 ·
`when-first`→2 · `and`→2 · `or`→2 · `(loop [i 0] (recur (inc i)))`→2 ·
`(cond\n    (= x 1) :one\n    :else :other)`→3 · `(condp = x\n    1 :one\n    2 :two)`→3 ·
`(case x\n    1 :one\n    :other)`→3 · `(cond-> x\n    (pos? x) inc\n    (even? x) (* 2))`→3 ·
`(some-> x inc dec)`→3 · `(some->> x (map inc) (filter pos?))`→3 ·
strings/comments: `"if when cond and or"`→1, `"(if true 1 0)"`→1, `;; if when cond`→1, `(if x :yes :no) ; and or when cond)`→2 ·
the map-literal cond (test_clojure.py) → 6 · `(defn semi [] \; (if true 1 2))`→2 · `(defn quoted [] \" (if x 1 0))`→2.
Extraction: `(ns demo.core)\n\n(defn choose [x]\n  (if x 1 0))\n\n(defn- hidden [y]\n  y)\n` @ `src/demo/core.clj` →
`(choose,2,L3) (hidden,1,L6)` ns `demo.core` · `(defn alpha…)(def rules [{:pred (fn [v] (if v (when …)))}])(defn omega…)` →
`{alpha:2, omega:1}` · `namespace_from_path("src/demo/core_extra.clj")`→`demo.core-extra`, `("/proj/src/demo/core.cljc","/proj")`→`demo.core`,
`("pkg/src/demo/core.bb")`→`demo.core` · strings/char/comment inside one form → `[foo]` · the discard/metadata suite → `[kept, also, third, spaced]`;
`(defn ^{:private kept [] 1)`→`[]` · `(def x #{\"})\n(defn foo [] 1)`→`[foo]` · the reader suite → `(kept,1) (also,1) (quoted,2) (semi,2)` ns `demo.core` ·
`#_(defn dropped [] (if x 1 0))\n(defn kept [] 1)\n` → `[{kept, L2, L2, 1}]` · EOF safety: `(defn foo [] \`, `(defn foo [] \abc`,
`(defn foo [] ;; comment`, `(defn foo [] "hello` → `[]`; `(defn foo []\n  ;; comment\n  "hello")` → foo, text ends with `)`.
Cross-language smoke: `(defn choose [x] x)` @ `src/demo/core.clj` → name `choose`, language `clojure`.

### 8.7 Coverage
- `parse_form_coverage('<span class="pc" title="1 out of 2 forms covered">   4&nbsp;&nbsp;(if x 1)</span>')` → `{4:(1,2)}`; `0 out of 0` → `{}`.
- Forms beat LCOV: lcov `DA:3,1 DA:4,0 DA:9,3` + forms `{4:(1,2)}` for `src/demo/core.clj`, fn L3–5 → **50.0**.
- LCOV `DA:3,1 DA:4,0`, fn L3–4 → 50.0. Missing file → nil.
- BRDA wins: `DA:2,1 DA:3,1 BRDA:3,0,0,1 BRDA:3,0,1,-` → lines[2]=(1,1), branches[3]=(1,2), fn L2–3 → 50.0.
- Branchless span keeps lines: `DA:2,1 DA:3,0 BRDA:8,0,0,1 BRDA:8,0,1,0`, fn L2–3 → 50.0.
- LCOV without end_of_record: `SF:a.clj\nDA:1,1\nSF:b.clj\nDA:2,0\n` → a[1]=(1,1), b[2]=(0,1).
- Go: `github.com/acme/demo/board.go:4.1,6.2 2 1` / `…:8.1,9.2 1 0` → board.go 4–6 = 100.0, 8–9 = 0.0, other.go → nil. No overlap 10–12 → 0.0. `not a segment` → error. nil profile → nil.
- Key choice: `other.go` listed first doesn't steal `board.go`. Specific prefix: keys `other/proj/src/a.go`, `proj/src/a.go`, src `src/a.go` → 100.0. Tie `proj/a.go` vs `other/a.go` for `a.go` → nil. src `/` → nil.
  Exact wins: keys `a/b/main.go`(1), `b/main.go`(0) → `b/main.go` 0.0, `a/b/main.go` 100.0.
  Bound sources `[b/main.go, a/b/main.go]`, key `a/b/main.go` → short nil, long 100.0.
  Key `board.go`, bound `[cmd/board.go]` → 100.0; bound `[cmd/board.go, other/board.go]` → both nil.
  LCOV key `proj/src/demo/core.clj` for src `src/demo/core.clj` → 50.0.
- JaCoCo: `demo/pkg/Board#place` line 4 (missed 1, covered 3) → 75.0; `demo/pkg/Board$Inner#tick` → 100.0. A method with only a LINE counter is skipped; `line="nope"` → 0; a class-level counter is ignored ⇒ keys `[demo.pkg.Board#bad]`. Missing `missed` → 0 ⇒ 100.0. Unknown method → nil. Empty bundle → nil. Nearest line: lines 4 (100%) and 20 (0%), start 18 → 0.0.
- `normalize_path("file:./src//demo/core.clj")`→`src/demo/core.clj`; `"%2E%2Fsrc%2Fcore.clj"`→`src/core.clj`.
- `suffix_match("src/demo/core.clj","demo/core.clj")`→T, `("core.clj","src/demo/core.clj")`→F, `(…,"")`→F.
- `coverage_percent(0,0)`=0.0; `percent_for_range({},1,3)`=0.0; `({1:(1,1)},4,5)`=0.0.
- `load_bundle` over `target/coverage/lcov.info` (DA 3:1, 4:0) + `go/coverage.out` + `site/jacoco/jacoco.xml` + `target/coverage/src/demo/core.clj.html` (line 4 = 1 of 2 forms) + a form-less `index.html` → the clj fn at L3–4 = **50.0**, taken from the **form** map (key `src/demo/core.clj`, only line 4 is in range → 1/2). `go_profile` and `jacoco` are non-nil. On an empty tree: go/jacoco nil, lcov `{}`.
- analyze: `b/main.go` + `a/b/main.go`, profile key `a/b/main.go` → `[(Place,b/main.go,0.0),(Place,a/b/main.go,100.0)]`.

### 8.8 CLI / CRAP / EDN
- `crap(5,100)=5.0`, `crap(5,0)=30.0`, `crap(1,100)=1.0`, `crap(8,45)≈18.648`, `crap(3,nil)=nil`.
- Sort: worst(30) > mid(5) > missing(nil); plain(crap 1) before large(nil, cc8); nil rows: cc 8 before cc 1.
- EDN: `{:entries [` … `:name "layout"`, `:namespace "uml-viewer.layout"`, `:complexity 8`, `:coverage 82.0`, `:crap 3.4` … ends `}\n`; empty → `{:entries []}\n`; nil → `:coverage nil`, `:crap nil`.
- Multi-language project, `--no-coverage` (clj/java/go/ts/rust + `_test` files) → entries for `demo.core choose 2 nil`, `demo.Board place 3 nil`, `demo Place`, `ui.view view`, `demo open`; no `should-skip`/`TestPlace`. Golden stdout (upstream run):
```
CRAP Report
===========
Languages: clojure, go, java, rust, typescript

Function                       Namespace                             CC    Cov%     CRAP
----------------------------------------------------------------------------------------
place                          demo.Board                             3   N/A       N/A
Place                          demo                                   2   N/A       N/A
open                           demo                                   2   N/A       N/A
choose                         demo.core                              2   N/A       N/A
view                           ui.view                                2   N/A       N/A
```
- `--use-existing-coverage src/demo/core.clj` with lcov DA 3,4 hit → `:coverage 100.0`, `:crap 2.0`. All misses + `--threshold 1` → exit 2. crap == threshold (1.0 vs 1) → 0. `--no-coverage --threshold 1` → 0.
- `--help` → 0, stdout contains `uml-viewer`, `.clj-kondo`, `.test.cts`. `--not-an-option`/`--root`/`--threshold nope`/`--no-coverage --coverage-command true` → 1.
- Selection: `--source-root src`, a `src` dir positional, `src/demo/core.clj core` (path + filter), `notes.md` (→ "No source files to analyze.", exit 0).
- `--changed` with fake status ` M src/demo/core.clj\0` → includes choose; git failure 128 + `fatal: not a git repository` → exit 128, stderr contains it, nothing on stdout. Status `?? src/kept.clj\0` → only kept.
  `_changed_files`: ` M x\0?? sources/keep.py\0` → `[x, sources/keep.py]` (non-source files are kept at this stage). Error code 1 + `git failed` → GitError(1, "git failed"). 128 + empty stderr → "git status failed".
  Real git: a deleted `keep.go`, an untracked `café.go`, `fresh/new.go` → Cafe + New. `--root pkg` limits to `pkg/inside.go`.
- `--coverage-command "exit 3"` (status 3) with a stale 100% lcov → `:coverage 0.0`, `:crap 6.0`, exit 0. Status 0 → `:coverage 100.0`.
- `_take`: `["ok","--opt",""]` and `["--opt","-1"]` → "requires a value".
- Discovery: `language_of` table (§2); `types.d.ts`→nil; `app.test.cts` → language typescript, is_test true; `conftest.py`, `src/app.test.js`, `src/app.spec.mjs`, `tests/test_board.py` → test.
  Tree with `src/app.py`, `tests/hidden.py`, `target/gen.py` → `[src/app.py]`; missing root → `[]`; file root → `[file]`.

### 8.9 Golden fixture for the report (`--use-existing-coverage`, root = fixture dir)
`src/demo/box.py`:
```python
def choose(x, ready):
    if x > 0 and x < 10:
        return 1
    elif ready:
        return 0
    return x


class Box:
    def open(self, flag=False):
        return 1 if flag else 0
```
`src/demo/app.ts`:
```ts
export function view(ok: boolean): number {
  if (ok) {
    return 1;
  }
  return 0;
}
app.get("/users", (req, res) => req.id ?? 0);
```
`target/coverage/lcov.info`: `SF:src/demo/box.py` DA 1:1 2:1 3:0 4:1 5:0 6:1 9:1 10:1 11:0, end; `SF:src/demo/app.ts` DA 1:1 2:1 3:1 5:0 7:1, BRDA `2,0,0,1` `2,0,1,0` `7,1,0,-` `7,1,1,-`, end.
→ stdout as in §6.5. EDN:
```
{:entries [
  {:name "GET /users", :namespace "demo.app", :complexity 2, :coverage 0.0, :crap 6.0}
  {:name "choose", :namespace "demo.box", :complexity 4, :coverage 66.6667, :crap 4.5926}
  {:name "view", :namespace "demo.app", :complexity 2, :coverage 50.0, :crap 2.5}
  {:name "open", :namespace "demo.box.Box", :complexity 2, :coverage 50.0, :crap 2.5}
]}
```

### 8.10 Runners (argv-level; mock `run_shell`)
- vitest (`"test":"vitest run"` + vitest.config.ts) → no `c8`; contains `vitest run --coverage`, `--coverage.reporter=lcov`, `--coverage.reportsDirectory=<dir>`, `--coverage.include=src/book.ts`, and no test file.
- `coverage` script → `["npm","run","coverage"]`. `"test":"node --test"` → the exact c8 argv.
- Nearest package.json/Cargo.toml (`web/`, `src-tauri/`). The report slug `src-tauri` → `target/coverage/rust/src-tauri/lcov.info`.
- Manifest restore (package.json + lock rewritten, a new shrinkwrap deleted).
- Python: pyproject with pytest → roots `[root]`, `uses_pytest`, sources `src`, the exact run/lcov argv. setup.py → unittest `… -m unittest discover -s .`. Sources for `carddav.py` → `carddav`. A file outside the package is ignored.
- Rust llvm-cov argv; tarpaulin kind/argv; auto-install flow (which/rustup/cargo install).
- Go argv keeps `-coverprofile=<path with space>` as one element (len 4).
- argv vs shell: a list is never shell-interpreted (`a;touch pwned` creates nothing).
- `run_shell("exit 3")` → 3; OSError → 127 + `failed to start`.
- Clojure clean keeps `python/` reports, removes `target/coverage/lcov.info` and stale cloverage html/dirs, keeps `notes.txt`/`notes/keep.txt`.
- Warnings: `skipping Clojure coverage`, `No package.json test script`, `coverage provider is missing`, `coverage is missing`, invalid package.json (shows `package.json`, not "No package.json test script"), `Python coverage exited 1`, `coverage lcov exited 1`, `Coverage was still recorded`, custom command `exited 9`.

---

## 9. Upstream quirks to preserve (parity), each flagged
1. Test-dir detection uses absolute path components (§2): projects under `…/tests/…` find nothing.
2. Naming a test **dir** explicitly yields nothing. Naming a test file works.
3. `-s` and `--changed` silently ignore existing positional paths.
4. TS route detection has no receiver check (`cache.get("k", cb)` → `GET k`). Escapes are dropped from route strings.
5. TS object/class-expression methods inside named-class methods are double-counted and namespaced to the outer class.
6. Functions/const-arrows inside top-level generators are entries. Class-field arrows and `var`/`module.exports` functions are not, and their decisions vanish.
7. Java anonymous-class methods in field initializers are entries (README says otherwise).
8. Rust `mod tests` only (not `mod test`, not `#[cfg(test)]`). Fn directly in a block is not an entry, but fn in an impl in a block is.
9. Python case guards count twice.
10. The `_candidates` absolute form resolves against the process CWD.
11. go.mod `module` value is kept verbatim (quotes/comments). The first go.mod found wins even without a module line.
12. Coverage loader only looks 0–2 levels deep for Go/JaCoCo, and only at `root/coverage` / `root/target/coverage` for LCOV.
13. A malformed Go profile/JaCoCo XML crashes the run (exit 1).
14. Clojure `(some->)` etc. can make complexity < 1 (floor division).
15. Report rows are 1 char narrower than the header (the Cov% field is 6 wide under a 7-wide header).
16. Java package regex can match inside a multi-line block comment.
17. Python namespace for an absolute out-of-root path without `/src/` has a leading `.`.
