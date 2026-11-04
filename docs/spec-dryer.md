# spec-dryer — structural duplicate finder (Go re-implementation spec)

Reference: `upstream/dryer` @ `66ff6d2` (Python 3.11+, `tree-sitter>=0.24`, `tree-sitter-language-pack>=0.7`).
Verified against: Python 3.12.3, `tree-sitter 0.26.0`, `tree-sitter-language-pack 1.23.1` (all 80 upstream tests pass). Every number in this doc was produced by running upstream on that stack.

Target: one Go binary, stdlib only. Go sources may use `go/parser`/`go/ast`; every other language needs a hand-written lexer and parser.

---

## 0. Pipeline at a glance

```
argv -> parse_args -> select_files (sorted, deduped, absolute)
     -> for each file: read (utf-8, errors=replace, universal newlines)
          -> language_of -> candidates (per language) -> normalize -> Entry{lang,file,start,end,nodes,fingerprints}
          -> keep if (end-start+1) >= min_lines && nodes >= min_nodes
     -> find_duplicates(threshold): same language, i<j, skip same span, jaccard >= threshold
     -> sort -> write .metrics/dry.edn (always EDN) -> print stdout (text|edn) -> stderr "Wrote <abs path>"
```

---

## 1. CLI

### 1.1 Usage

`dryer [options] [path-or-filter ...]`

| Flag | Value | Default | Parse |
|---|---|---|---|
| `-h`, `--help` | — | — | If present **anywhere** in argv, print HELP to stdout, exit 0. Checked before any other parsing, so `--help --nope` exits 0. |
| `--root <path>` | path | `.` | Project root. Resolved against cwd (`Path.resolve()`, non-strict, follows symlinks: on macOS `/tmp` → `/private/tmp`). Last occurrence wins. |
| `-s`, `--source-root <path>` | path | none | Repeatable (appends). Each one is `(root / path).resolve()`. An absolute path replaces root. |
| `--changed` | — | false | Use added and modified files from `git status` (§2.3). |
| `--threshold N` | float | `0.82` | Python `float()`. Accepts `1e-3`, `inf`, `nan`, `" 0.5 "`, `1_0`. **No range check.** `nan` reports every pair, because `score < nan` is always false. |
| `--min-lines N` | int | `4` | Python `int()`: accepts `+5`, `" 5 "`, `1_0`. Negative → error. `0` is valid. |
| `--min-nodes N` | int | `20` | Same as `--min-lines`. |
| `--format F` | str | `text` | Any string is stored. It is checked after parsing (§1.3). |
| `--edn` | — | — | Same as `--format edn`. |
| `--text` | — | — | Same as `--format text`. The last format flag wins. |

Value taking (`_take`): an option fails with `"<opt> requires a value"` when there is no next arg, the next arg is the empty string, or the next arg starts with `-`. So `--threshold -0.5` and `--min-lines --min-nodes 1` are usage errors.

Conversion errors:
- float: `"<opt> requires a number"`
- int, not parseable: `"<opt> requires an integer"`
- int < 0: `"<opt> requires a non-negative integer"`

Any other arg that starts with `-` gives `"Unknown option: <arg>"`. Everything else is a positional. There is no `--` terminator.

Go port of number parsing: `strings.TrimSpace`, remove `_` only between digits (Python rule), then `strconv.ParseFloat` / `strconv.Atoi`. Python accepts `inf`/`infinity`/`nan` case-insensitively, and so does Go's ParseFloat.

### 1.2 HELP text

Literal text (the f-string is already expanded). `_skipped_directory_text()` = sorted(SKIP_DIRS ∪ TEST_DIRS), joined with `", "`, with `", and "` before the last item.

```
Usage: dryer [options] [path-or-filter ...]

Find candidate duplicate code in Clojure, Java, Go, TypeScript, Rust, and
Python. Each form is normalized — call and method names, operators, and
tree shape stay; local names, field names, and literals do not — and compared
with the other forms of the same language. The score is Jaccard similarity of
those structural fingerprints, the same score as dry4clj.

Prints a report and writes .metrics/dry.edn.

Languages: Clojure (.clj .cljc .cljs .cljd .bb), Java (.java), Go (.go),
TypeScript (.ts .tsx .mts .cts), Rust (.rs), Python (.py).

Options:
  -h, --help                    Print this help and exit.
  --root <path>                 Project root. Metrics are written here.
                                Default: the current directory.
  -s, --source-root <path>      Walk this tree instead of the project root.
                                May be repeated.
  --changed                     Compare added and modified source files from
                                git status.
  --threshold N                 Minimum similarity score, default 0.82.
  --min-lines N                 Minimum source lines in a candidate form,
                                default 4.
  --min-nodes N                 Minimum normalized syntax nodes, default 20.
  --format F                    text or edn, default text.
  --edn                         Same as --format edn.
  --text                        Same as --format text.

Arguments:
  path              File or directory to compare. Test paths are included when
                    you name them explicitly.
  filter            When the argument is not a path, only source files whose
                    path contains this text are compared.

With no paths, source files under the project root are compared. Directories
named .clj-kondo, .git, .hg, .idea, .metrics, .svn, .venv, __pycache__, __tests__, build, coverage, dist, node_modules, out, spec, specs, target, test, testdata, tests, vendor, and venv are skipped, as are *_test.go, *.test.cts,
*.spec.cts, *.spec.ts, test_*.py, and *_test.py files.

Clojure compares every top-level form except ns. The other languages compare
functions and methods. A form is only compared with forms in the same language.

Exit codes:
  0  the report was written
  1  usage error
  2  unknown output format
```

The directory sentence is **one long line**, because the f-string does not wrap it. Upstream HELP output is 2349 bytes, md5 `740bd09b788b09a72ecfc61a09992bb7`; golden-test it byte for byte.
HELP ends with exactly one `\n`.

### 1.3 Exit codes and streams

| Situation | stdout | stderr | exit |
|---|---|---|---|
| `-h`/`--help` | HELP | — | 0 |
| usage error | — | `"<msg>\n\n" + HELP` | 1 |
| format not in {text, edn}. Checked only after a clean parse, so a usage error takes precedence. | — | `"Unknown format: <F>\n"` | 2 |
| `--changed` and git fails | — | `<git stderr, stripped>` or `"git status failed"` when that is empty, then `\n` | **git's own exit code** (128 when not a repo) |
| no files selected | `"No source files to analyze.\n"` | — | 0. **No metrics file is written.** |
| normal run | report (§6) | Clojure warnings, one line each: `"dryer: <file>:<line>: <msg>"`, then `"Wrote <abs root>/.metrics/dry.edn"` | 0 |

Upstream crashes with a traceback (exit 1) when git is not installed, when a file cannot be read, or in the comment-callee bug (§4.6). Go: give the first two a clean stderr message and exit 1. For the third, see §4.6.

Write order: stdout report, then the stderr `Wrote` line. Clojure warnings are printed during the scan, before the report. Tests should capture the two streams separately.

---

## 2. File selection

### 2.1 Language and test classification (`discover.py`)

Extensions (exact, case-sensitive suffix):
`.clj .cljc .cljs .cljd .bb`→clojure, `.java`→java, `.go`→go, `.ts .tsx .mts .cts`→typescript, `.rs`→rust, `.py`→python.
- A name ending in `.d.ts` → none. `.d.mts` and `.d.cts` are **not** excluded.
- **JavaScript (`.js .jsx .mjs .cjs`) is not supported upstream.** The repo's `fixtures/javascript` tree gives "No source files to analyze."
- `.tsx` paths use the `tsx` grammar. `.ts .mts .cts` use `typescript`.

`is_test_file(path)` is true when any of these holds:
- name ends with `_test.go`, `_test.clj`, `_test.cljc`, `_test.cljs`, `_test.cljd`, or `_test.bb`
- name ends with `.test.ts .spec.ts .test.tsx .spec.tsx .test.mts .spec.mts .test.cts .spec.cts`
- name == `conftest.py`
- name ends with `_test.py`, or starts with `test_` and ends with `.py`
- **any component of the path as given** is in TEST_DIRS = `{test, tests, spec, specs, __tests__}`. Walks pass absolute paths, so an ancestor of the project root counts. A project under `/x/specs/proj` analyzes nothing. This is verified behavior and should be replicated.

SKIP_DIRS = `{.git .hg .svn .idea .venv venv node_modules vendor target dist build out coverage .metrics testdata __pycache__ .clj-kondo}`.

### 2.2 Walk (`iter_source_files(roots)`)

For each root:
- If it does not exist → nothing.
- If it is a file → include it as is, with no language or test check. In practice this path is only reached for source roots.
- Otherwise `os.walk` top-down, `followlinks=False`. Prune child dirs whose name is in SKIP_DIRS ∪ TEST_DIRS. The root's own name is not checked. Keep a file when `language_of` is not none and it is not a test file. Symlinked files are kept. Symlinked dirs are not descended.

Result: set of `resolve()`d paths, sorted by POSIX string. Python compares code points; Go byte order on UTF-8 gives the same order.

### 2.3 `select_files`

```
root = resolve(--root)
for each positional: p = abs ? arg : root/arg      # relative to ROOT, not cwd
    exists(p) ? existing += resolve(p) : filters += arg
files =
  --changed        -> changed_source(root)          # existing positionals are IGNORED
  elif --source-root -> walk(each (root/sr).resolve())  # existing positionals ignored
  elif existing    -> for each: dir ? walk(dir) : (language_of(f) ? [f] : [])
                      # explicit files skip the test/skip checks; explicit dirs are walked
                      # (so `dryer tests` finds nothing, because "tests" is in every path)
  else             -> walk(root)
if filters: keep f where any(filter is a substring of f.as_posix())   # ABSOLUTE resolved path
return sorted(unique(resolve(f)), key=posix)
```

`--changed`:
1. `git -C <root> rev-parse --show-toplevel` → toplevel (stdout stripped).
2. `git -C <root> status --porcelain -z --no-renames --untracked-files=all -- .`
3. Split on NUL. Skip entries with `len <= 3`. Take `path = resolve(toplevel / entry[3:])`. Keep it when it is a regular file and is under `resolve(root)`. Deletions are gone. A rename shows up as delete + add, so only the new path stays.
4. `_tracked_source(path, root)`: the components of the path relative to root must not meet SKIP_DIRS, `language_of` must be known, and the file must not be a test file (test check on the absolute path).
A non-zero git exit gives `GitStatusError(code, stderr.strip() or "git status failed")`. Its handling is in §1.3.

### 2.4 Reading

`Path.read_text(encoding="utf-8", errors="replace")`. Go equivalent:
- Invalid UTF-8 → U+FFFD. Python replaces each invalid byte, not each run as `strings.ToValidUTF8` does. This only matters if invalid bytes sit inside a kept callee name.
- **Universal newlines**: `\r\n` → `\n`, then a lone `\r` → `\n`. This happens before parsing and line counting.
- A BOM is kept for tree-sitter languages. The Clojure reader strips it.

Display path: `resolved.relative_to(resolve(root)).as_posix()`, or the absolute POSIX path when the file is outside root.

---

## 3. Candidate extraction (`extract.py`)

Line numbers (tree-sitter languages):
- `start_line = start_point.row + 1`
- `end_line = (end_point.col == 0) ? max(start_line, end_point.row) : end_point.row + 1` (rows are 0-based; `end_point` is exclusive)

A candidate's span is that of the **candidate node** below. Comments that sit inside the node count. Candidates come out in pre-order (document order) per file.

### 3.1 Python (`python` grammar)
- Every `function_definition` (sync or async) whose nearest `class_definition`-or-`function_definition` ancestor is not a `function_definition`.
  - So methods are kept, top-level defs under `if`/`try` are kept, and defs nested in a def are dropped.
  - **A method of a class defined inside a function IS kept.** It overlaps its enclosing function and both are compared.
- Decorators are excluded. The node is the `function_definition`, a child of `decorated_definition`.
- Lambdas are not candidates.
- **End line quirk:** tree-sitter-python keeps trailing comments *inside* the innermost open block when the comment's column ≥ that block's indentation, even after blank lines. Verified:
  - `def a(x):\n    return x\n    # trailing\n` → span 1-3.
  - A comment at col 6, where body indent is 8 and class body indent is 4, belongs to the class block, not the method.
  - A dedented comment is outside.

### 3.2 TypeScript / TSX
- `function_declaration` and `method_definition` that have a **direct** `statement_block` child and no ancestor in `{function_declaration, method_definition, arrow_function, function_expression}`.
- `arrow_function` and `function_expression` whose **parent** is one of `{variable_declarator, public_field_definition, property_definition}`, where that parent has no ancestor in the function set above. **The candidate node is the parent** (declarator or field), and that node is also what gets normalized.
  - The body may be an expression. `const f = (x) => x` at top level is a candidate.
  - Span: `alpha = (...) => {...}`. `const`/`export` are excluded, and so is the `;`.
- Not candidates:
  - `generator_function_declaration` (`function*`)
  - `function_signature` (overloads), `abstract_method_signature`
  - arrows in object `pair`s, call arguments, or `export default` expressions
  - anything nested inside a function, including methods of a class declared inside a function
- Object-literal methods (`method_definition` in an `object`) outside functions are kept.

### 3.3 Go
- `function_declaration` and `method_declaration` with a direct `block` child (`func f()` with no body is dropped).
- `func_literal` is never a candidate; it stays inside its function.

### 3.4 Java
- Explicit stack, pre-order. On reaching a `method_declaration`, **do not descend into it**. Keep it if it has a direct `block` child and no ancestor in `{method_declaration, constructor_declaration, lambda_expression}`.
- The walk does descend into constructors, initializer blocks, field initializers, and enum bodies.
  - Methods of anonymous or local classes inside **constructors** or lambdas are dropped.
  - **Methods of anonymous classes in field initializers, static/instance initializer blocks, and enum constant bodies are KEPT.** This contradicts the README and is verified: `Runnable r = new Runnable(){ public void run(){…} };` yields `run`.
- Abstract, interface, and native methods without bodies are dropped. Interface `default`/`static` methods with bodies are kept. Constructors are never candidates.

### 3.5 Rust
- `function_item` with a direct `block` child, whose **parent is not `block`**, and with no ancestor `mod_item` whose first `identifier` child's text == `"tests"` (exact name; `#[cfg(test)]` is not considered).
- Kept: functions in `impl`/`trait` blocks (`declaration_list`), including `impl` blocks nested inside a fn body. That gives overlapping candidates.
- Dropped: `function_signature_item`, and fns declared directly in a block. Closures are never candidates.

### 3.6 Clojure
Every top-level form that is a non-empty list whose head is not the symbol `ns`. See §7.

### 3.7 Filter
`lines = end - start + 1`. Keep when `lines >= min_lines && nodes >= min_nodes`. This runs per file, before pairing.

---

## 4. Normalization (`astnorm.py`)

The normalized value is a tree over three atom kinds:
- `K(name)`: a keyword, printed `:name`
- `str`: printed `"…"`, escaping only `\` → `\\` and `"` → `\"`
- `list`: printed `[a b c]`

The sentinel `NONE` means "drop this child".

### 4.1 Sets (exact)

```
SKIP        = comment line_comment block_comment documentation_comment doc_comment
IDENTIFIERS = identifier type_identifier field_identifier property_identifier
              shorthand_property_identifier shorthand_property_identifier_pattern package_identifier
LITERALS    = string string_literal interpreted_string_literal raw_string_literal char_literal
              rune_literal character_literal number integer float decimal_integer_literal
              hex_integer_literal octal_integer_literal binary_integer_literal
              decimal_floating_point_literal hex_floating_point_literal int_literal float_literal
              imaginary_literal integer_literal true false null nil none undefined null_literal
              boolean_literal template_string regex regex_literal
OPERATORS   = + - * / % ** == != < > <= >= === !== && || & | ^ << >> >>> += -= *= /= %= &= |= ^=
              <<= >>= >>>= ! ~ ++ -- and or not ?? ?. = := => ?
CALLS       = call call_expression method_invocation object_creation_expression new_expression
ATTRIBUTES  = attribute member_expression selector_expression field_access field_expression
SCOPED      = scoped_identifier scoped_type_identifier        (+ "generic_type" wherever SCOPED is tested)
ARG_LISTS   = argument_list arguments
TYPE_ARGS   = type_arguments type_parameters
```

Not in OPERATORS, so dropped: `//`, `@`, `->`, `<-`, `::`, `..`, `...`, `in`, `is`, `not in`, `is not`, `**=`, `//=`, `@=`, and all keywords except `and or not`.

`is_literal(n)`: `n.type ∈ LITERALS`, except a `string` that has a child of type `interpolation`. That exception covers Python f-strings. TS `template_string` with `${}` is still a literal.

### 4.2 Algorithm (equivalent recursive form)

Upstream uses an explicit post-order stack keyed by `(id(node), mode, head)` to avoid RecursionError on 1,200-deep trees. It is semantically identical to the recursion below. py-tree-sitter caches `children`, and `named_children` returns the same objects, so the ids are stable. **The Go port must also be non-recursive, or recurse on a goroutine with a large stack.** There is a test with 1,200 nested `+`.

```
norm(n, head=false):
  if n.type ∈ SKIP:            return NONE          # any comment, named or not
  if n.type ∈ OPERATORS:       return [K"symbol", n.type]   # checked BEFORE is_named
  if !n.is_named:              return NONE          # punctuation, keywords
  if n.type ∈ IDENTIFIERS:     return head ? [K"symbol", text(n)] : K"symbol"
  if is_literal(n):            return K"literal"
  if n.type == "parenthesized_expression" && len(named_children(n)) == 1:
                               return norm(named_children(n)[0], head)   # head passes through
  if n.type ∈ CALLS:           return call(n)        # head ignored
  if n.type ∈ ATTRIBUTES:      return attr(n, head)
  return [K n.type] ++ [v for c in n.children (ALL, incl. anonymous) if (v = norm(c)) != NONE]

call(n):
  named = named_children(n)
  i = index of first c in named with c.type ∈ ARG_LISTS (or none)
  incoming = named[:i]  (all of named if none);  args = named[i] or nil
  # named children AFTER the arg list are DROPPED (e.g. Java anonymous class_body)
  rest  = [c ∈ incoming | c.type ∉ TYPE_ARGS];  targs = [c ∈ incoming | c.type ∈ TYPE_ARGS]
  out = [K n.type]
  if rest nonempty:
      for r in rest[:-1]: append norm(r) unless NONE        # receivers
      append callee(rest[-1])                               # NOT None-checked (§4.6)
  for t in targs: append norm(t) unless NONE                # type args come AFTER callee
  if args: append norm(args) unless NONE
  return out

callee(c):
  if c.type ∈ IDENTIFIERS:            return [K"symbol", text(c)]
  if c.type ∈ ATTRIBUTES:             return attr(c, head=true)
  if c.type ∈ SCOPED ∪ {generic_type}: return path(c)
  return norm(c, head=true)

attr(n, head):
  named = named_children(n)                  # anonymous children ignored entirely
  if empty: return [K n.type]
  objects = named[:-1]; name = named[-1]
  out = [K n.type] ++ [v for o in objects if (v = norm(o)) != NONE]
  if name.type ∈ IDENTIFIERS: append (head ? [K"symbol", text(name)] : K"symbol")
  else: v = head ? callee(name) : norm(name); append v unless NONE
  return out

path(n):                                     # keeps every name in a callee path
  out = [K n.type]
  for c in named_children(n):
      if c.type ∈ IDENTIFIERS:              append [K"symbol", text(c)]
      elif c.type ∈ SCOPED ∪ {generic_type}: append path(c)
      else: append norm(c) unless NONE        # e.g. type_arguments: "<" ">" kept, type names erased
  return out
```

The root call is `norm(candidate_node, head=false)`. It always returns a list.

Consequences, all verified:
- Callee names are kept: `filter`, `xs.filter` → `filter`, `fmt.Println` → `Println` (`fmt` erased), `Vec::new` → `Vec`+`new`, `new ArrayList<String>()` → `ArrayList` (type args erased), `(len)(x)` == `len(x)`.
- Member names not in call position → `:symbol`. A TS `#private` name → `[:private_property_identifier]`, with no text.
- Rust `xs.collect::<Vec<_>>()`: the callee is `generic_function`, which goes through the default branch with head=false inside, so `collect` is **erased**.
- Go `f[int](x)`, `m[k](x)`, `[]byte(s)`, `map[K]V(m)` → `type_conversion_expression` (not a call), so the callee name is erased. `(*T)(x)` → `call_expression` with callee `[:unary_expression [:symbol "*"] :symbol]`.
- Rust macros: `println!(…)` → `[:macro_invocation :symbol [:symbol "!"] [:token_tree …]]`. The name is erased. Punctuation in a token_tree that is in OPERATORS is kept.
- TS optional call `a?.()`: the callee becomes `[:optional_chain [:symbol "?."]]`. Tagged template `` tag`x` ``: the callee is the template literal → `:literal`, and the tag → `:symbol`.
- Python `f(x for x in y)`: `generator_expression` is not in ARG_LISTS, so `f` → `:symbol` as a receiver and the generator becomes the "callee", normalized with head=true.
- Java `new Foo(){…}`: the anonymous class body is dropped from the tree.
- Rust `attribute` (`#[…]`) is in ATTRIBUTES by name collision, so it goes through `attr()`.
- Operators inside types are kept: `<` `>` in type_arguments (TS, Java, Rust, JSX tags), `*` in Go pointer types, `|` in TS unions and Rust closures, `&` in Rust refs, `?` in TS optional params, Java wildcards, and Rust try, `!` in TS non-null and Rust macros, `=>` in TS arrows, `=` in declarations, `:=` in Go.

### 4.3 Node count (`--min-nodes`, `:left-nodes`, `:right-nodes`)

`node_count(v)` = 1 for an atom. For a list it is 1 + the sum over its elements. Every list, every K (including the type tag), and every string counts.
- `[:symbol "len"]` = 3. `[:list :symbol :literal]` = 4.
- Python `def alpha(xs):\n    return filter(xs, odd)` = 19.

### 4.4 Fingerprints

`fingerprints(v)` = set of `pr(s)` for every list subtree s (root included), ∪ `pr(a)` for every atom a.
- Type tags show up as `":call"`, and names as `"\"filter\""`.
- Example, `def beta(xs): return select(xs, odd)` (15 fingerprints):
  ```
  "select"  :argument_list  :block  :call  :function_definition  :parameters  :return_statement  :symbol
  [:argument_list :symbol :symbol]   [:parameters :symbol]   [:symbol "select"]
  [:call [:symbol "select"] [:argument_list :symbol :symbol]]
  [:return_statement …]   [:block …]   [:function_definition …]
  ```
- Go: build the printed string bottom-up, caching each child's string. Use a `map[string]struct{}` for the set. Hashing to 64- or 128-bit is safe if collisions are acceptable, but exact strings are simplest and needed for golden debugging.

### 4.5 Score

`jaccard(A,B) = |A∩B| / |A∪B|` as float64 (`0.0` if the union is empty). Python int/int division and Go `float64(a)/float64(b)` are both IEEE correctly rounded, so the results are identical.

### 4.6 Upstream bug: comment as the last named child before the args

`foo /* c */ (x)` is legal in Go, Java, TS, and Rust. `callee(comment)` → NONE gets appended, and `pr` raises `TypeError: cannot print None`. The whole run crashes with a traceback (verified with Go). Recommendation: treat it as absent (skip the append) and record it as a deliberate deviation. No test depends on the crash.

---

## 5. Pairing and ordering (`scan.py`)

```
by_language = ordered map (first-seen order) lang -> [entries in file order, then document order]
for each group: for i < j:
    if same file && same start && same end: skip      # also skips two different one-liners on the same line
    s = jaccard(e_i.fp, e_j.fp)
    if s < threshold: skip                             # equal to threshold => reported
    emit Duplicate(s, lang, left=e_i span, right=e_j span, left_nodes, right_nodes)
stable sort by (-score, language, left.file, left.start, right.file, right.start)
```

- Same-file pairs and overlapping pairs (a nested candidate against its container) **are** compared.
- No cross-language pairs. No dedup beyond the same-span rule. N copies give N·(N-1)/2 pairs.
- Left/right come from enumeration order: files are sorted by absolute POSIX path. Remaining ties keep enumeration order, so use `sort.SliceStable`.

---

## 6. Output (`report.py`)

### 6.1 Text (stdout, `--format text`)
- Empty: `"No duplicate candidates found.\n"`.
- Otherwise, each candidate is a block:
  ```
  DUPLICATE score=%.2f
    <left.file>:<start>-<end>
    <right.file>:<start>-<end>
  ```
  Blocks are joined with `"\n\n"` and the output ends with `"\n"`.
- `%.2f`: Python and Go both round the exact binary value, half-to-even on exact ties. 0.875 → `0.88`, 0.125 → `0.12`, 0.995 → `0.99`, 0.8214… → `0.82`. Verified identical in Go 1.26.

### 6.2 EDN (stdout with `--edn`, and **always** the content of `<root>/.metrics/dry.edn`)
- Empty: `"{:candidates []}\n"`.
- Otherwise: `"{:candidates [\n" + join("\n", rows) + "\n]}\n"`, where each row is:
  ```
   {:score <F>
    :language "<lang>"
    :left {:file "<f>", :start-line <n>, :end-line <m>}
    :right {:file "<f>", :start-line <n>, :end-line <m>}
    :left-nodes <n>
    :right-nodes <m>}
  ```
  (one leading space on the first line, two on the others, no trailing comma).
- `<F>` = `strconv.FormatFloat(s,'f',12,64)`, strip trailing `0`s, then a trailing `.`, then append `.0` if no `.` remains. Same as Python `f"{v:.12f}".rstrip("0").rstrip(".")`. Examples: `1.0`, `0.875`, `0.890909090909`, `0.821428571429`, `0.428571428571`, `0.525`.
- Strings escape only `\` and `"`.
- Language values: `clojure java go typescript rust python`.
- The file is written with `mkdir -p .metrics` and replaced on every analysis, even with no candidates. It is not written on "No source files" or on errors. Write it before printing stdout.

---

## 7. Clojure (`clojure_read.py`) — hand reader, no tree-sitter (exact port is easy)

### 7.1 Reader
- Strip a leading U+FEFF. If the text starts with `#!`, skip to the first `\n`.
- Whitespace = `space \t \n \r ,`. `;` comments run to `\n`. Lines are counted on `\n`.
- Terminators (end a token) = `( ) [ ] { } " ; ' @ ^ ` ~ \ , # :` plus whitespace.
- Collections: `(`→list, `[`→vector, `{`→map, `#{`→set, `#(`→list with `Sym("fn*")` prepended. Closing a map with an odd number of items → error `map literal has an odd number of forms` (line of the opener). Pairs keep source order.
- Wrappers: `'`→`(quote x)`, `@`→`(deref x)`, `` ` ``→`(syntax-quote x)` (not expanded), `~`→`unquote`, `~@`→`unquote-splicing`. When the wrapped form is NONE or a splice, you get a 1-element list `(quote)`.
- `^meta form` → the meta form is read and discarded.
- `#_form` → NONE. `#"…"` → Lit. `#tag form` (dispatch with any other char) → `read_token` (the tag), then the next form is read and replaced by `Lit` (line of `#`).
- `#?(…)` / `#?@(…)`: the body must be a list, otherwise error `reader conditional body must be a list` (line = current reader line). Scan pairs (feature, expr) at index 0, 2, …; the first feature that is a keyword named `clj` or `default` wins. A trailing odd feature is ignored. No match → NONE.
  - Splicing: the chosen value must be a list or vector, otherwise error `splicing reader conditional needs a collection`. The items splice into the enclosing collection. At top level the splice is dropped.
- `#:ns{…}`, `#::{…}`, `#:{…}`: consume `:`, then an optional `:`, otherwise a qualifier token (skipped when the next char is `{` or whitespace), then read the map normally. **Quirk:** `#::foo{…}` reads `foo` as a separate symbol form.
- Strings `"…"` handle `\x` escapes → Lit (line of the opening quote). EOF gives `unterminated string`.
- Chars: `\` + one char consumed unconditionally, then non-terminators (`\newline`, `\(`) → Lit. EOF right after `\` gives `incomplete character`.
- Keywords: `:` then an optional second `:`. If the next char is EOF, whitespace, or a terminator → `Kw("")`, else `Kw(token)`.
- Token: maximal run of non-terminators, else error `expected a symbol`. Number when it matches
  `^[+-]?(?:\d+/\d+|\d+\.\d+(?:[eE][+-]?\d+)?|\.\d+(?:[eE][+-]?\d+)?|\d+(?:[eE][+-]?\d+)?|0[xX][0-9A-Fa-f]+)[MN]?$` → Lit, else Sym.
  - `\d` is Unicode Nd, so use `\p{Nd}` in Go.
  - `1.` is a Sym. `nil`, `true`, and `false` are **Syms** → `:symbol`.
- Other errors: `unterminated list|vector|map|set` (opener line; `#(` reports `list`), `unexpected end of file` (current line), `incomplete dispatch` (`#` at EOF).
- **Verified quirks to replicate.** Each yields `expected a symbol` and stops reading the file:
  - `#'var`, `##Inf`
  - auto-gensym `x#` followed by whitespace or a closer
  - `foo'` before a closer, e.g. `(a c')`
  - stray `)`
  `a:b` reads as `a` followed by `:b`.
- `read_source`: loop over top-level forms. On a ReadError, return the forms read so far plus the warning `"<line>: <msg>"`. The scan prints `dryer: <file>:<line>: <msg>`. Top-level NONE and splices are skipped.

### 7.2 Normalize (dry4clj `normalize-form`)
- Sym → head ? `[:symbol "name"]` : `:symbol`. Kw → `:keyword` (in the head too: `(:kind m)`). Lit → `:literal`.
- list → `[:list norm(head, head=true) norm(arg)...]`. Empty list → `[:list :literal]`.
- vector → `[:vector …]`, set → `[:set …]`, map → `[:map [k v] [k v] …]`. Each pair is an **untagged** 2-element list.
- Unknown collection kind → `[:literal]`.

### 7.3 Lines
- start = line of the opening `(`.
- end = **max `line` attribute over the form and all descendants**. Collections carry their opener line, and atoms their start line. **The line of the closing paren is ignored:** `(f\n  x\n)` → end 2. Multi-line strings count their start line only.

Verified: README invoice/receipt → `score 0.890909090909`, spans 3-13 / 3-14, nodes 158/166 (49 shared / 55 union).

---

## 8. Upstream quirks: decide replicate or fix

| # | Quirk | Recommendation |
|---|---|---|
| Q1 | Any ancestor dir named test/tests/spec/specs/__tests__ hides every file | replicate (parity) |
| Q2 | `--changed` ignores existing positionals. They are not even used as filters. | replicate |
| Q3 | Filters match the **absolute** path | replicate |
| Q4 | Java anonymous-class methods in field initializers and initializer blocks are candidates | replicate |
| Q5 | Comment as callee crashes (§4.6) | **fix**: skip NONE, document it |
| Q6 | Clojure `#'`, `##`, `x#` stop the reader | replicate (warnings match upstream) |
| Q7 | Two candidates on the same file and same line span are never paired | replicate |
| Q8 | Grammar version is unpinned upstream (`>=0.7`). Node shapes vary by pack build. | pin the parity target to **language-pack 1.23.1** |
| Q9 | Python trailing indented comments extend `end_line` | replicate (affects `--min-lines`) |
| Q10 | No JS support | parity: ignore `.js`. Adding JS later is a new feature with no oracle. |

---

## 9. Feasibility: matching tree-sitter shapes without tree-sitter

### 9.1 What actually has to match

The output is only spans, node counts, and Jaccard scores. Node-type **names never reach the output**. Requirements:
1. **Same candidate set and spans.** Needs the right function boundaries, Python trailing-comment attachment, and the nesting rules.
2. **Isomorphic normalized trees:**
   - same nesting, same named-node boundaries, and same "is this node in a special set" decisions
   - same kept operator tokens, with the same spelling per operator
   - same callee texts
   Jaccard is invariant under any **injective relabeling** of type names, provided labels never collide with `symbol`, `literal`, or `keyword`. So wrong names alone do not change scores. A missing or extra **wrapper node** does change them (extra subtrees and atoms), and so does merging two distinct types. Both also change `nodes`.
3. **Same node counts.** These show up in EDN and drive `--min-nodes`.

The large simplifications:
- All 7 IDENTIFIER kinds behave identically, so the parser can emit one ident kind and keep the text.
- All LITERAL kinds collapse to `:literal`. Literal internals are irrelevant, except Python f-strings.
- Anonymous tokens matter only if they are in OPERATORS. Keywords and punctuation can be omitted entirely.
- `parenthesized_expression` with one named child disappears (`if (x)` in Java/TS too).

What does **not** simplify: every named non-identifier, non-literal node kind inside a function, *including signatures and type annotations*, must appear exactly where tree-sitter puts it. Hidden rules (`_expression`), supertypes, and inlined rules must **not** appear. Aliases appear under their alias name. Leaf named nodes such as `this`, `self`, `super`, `crate`, `predefined_type`, `integral_type`, `void_type`, `primitive_type`, `mutable_specifier`, `visibility_modifier`, `label_name`, `iota`, `dot`, `blank_identifier`, `string_start`, `escape_sequence`, `keyword_separator`, `line_continuation`, and `jsx_text` must be emitted as `[:type]`.

**Strategy, applying to all languages:**
- Hand parsers emit tree-sitter's **exact** type names. They are free, and they let golden tests compare printed normalized trees **as strings** against an oracle.
- A shared CST node type: `{Type string; Named bool; Text string (identifiers only); Children []*Node; Start, End Point}`. Emit only named nodes, OPERATORS tokens, and comments (needed for §4.6 and Python spans).
- One normalizer over that CST, ported 1:1 from §4.2.
- **Differential oracle** (Appendix B): run upstream on a corpus, dump `{file, start, end, nodes, pr(normalized)}` per candidate, and require string equality in Go tests.
- Parse errors: tree-sitter error recovery (`ERROR`/`MISSING` nodes) cannot be reproduced. Go policy: if a function fails to parse, skip that candidate (or file) with a stderr note. That is a documented divergence that only affects invalid code.

### 9.2 Per language

Grammar inventory for pack 1.23.1 (visible named kinds): python 123, typescript 177, tsx 184, go 107, java 142, rust 163. Full lists are in Appendix A.

| Lang | Approach | What must be mirrored | Hard spots | Parity |
|---|---|---|---|---|
| **Clojure** | port the upstream reader verbatim | nothing from tree-sitter | none | **exact** |
| **Go** | `go/parser` + `go/ast` → tree-sitter-go CST translator (~100 kinds) | `statement_list` inside non-empty `block`; `expression_list` wrapping LHS/RHS of `:=`/`=`/`op=`, return values, range LHS, case values, var_spec values; receiver `parameter_list` *before* the name; a single unnamed result is a bare type, while multiple results are `parameter_list` of `parameter_declaration`; `parameter_declaration` holds several names; `variadic_parameter_declaration`; `literal_value`/`literal_element`/`keyed_element`; `qualified_type` vs `selector_expression` (types and composite-literal types vs expressions); `if_statement` alternative is a bare nested `if_statement` or `block` (no else node); `type_switch_statement` flattens the guard (`[:expression_list] ":=" :symbol [:type_case …]`); `var_spec_list` for grouped `var (…)`; `inc_statement`/`dec_statement`; `unary_expression` for `& * - ! ^ <-` (`<-` dropped); `parenthesized_type` not unwrapped; `variadic_argument`; `implicit_length_array_type`; `type_elem` in `type_arguments`; `label_name`, `iota`, `blank_identifier` | tree-sitter's syntactic choice for `X[...](args)` (X is ident or `a.b`, indices type-like, including `m[k](x)`) → `type_conversion_expression(generic_type(...))`, while `xs[0](y)` → `call_expression(index_expression)`; `T{}` vs block in `if` headers (go/parser rules are close); comment-attachment quirks need no handling since comments are dropped; ERROR trees | **exact** for valid code after golden-tuning the ambiguity rules |
| **Java** | hand lexer + recursive descent | `modifiers` (annotations: `marker_annotation`, `annotation` + `annotation_argument_list`), `integral_type`/`floating_point_type`/`boolean_type`/`void_type`, `array_type`+`dimensions`, `generic_type`, `type_arguments`, `wildcard`, `scoped_type_identifier`, `formal_parameters`/`formal_parameter`/`spread_parameter`, `local_variable_declaration`/`variable_declarator`, `method_invocation` (object, name, type_arguments, argument_list), `field_access`, `object_creation_expression`, `array_creation_expression`/`dimensions_expr`/`array_initializer`, `lambda_expression`/`inferred_parameters`, `method_reference`, `cast_expression`, `instanceof_expression`/patterns, switch (`switch_expression`, `switch_block`, `switch_rule`, `switch_label`, `switch_block_statement_group`), `enhanced_for_statement`, try (`try_with_resources_statement`, `resource_specification`, `catch_clause`/`catch_formal_parameter`/`catch_type`, `finally_clause`), `yield_statement`, `ternary_expression`, `update_expression`, local classes and records | generic-method vs relational `<` ambiguity (Java's grammar makes this tractable); text blocks are literals anyway | **exact/near** |
| **Python** | hand lexer (INDENT/DEDENT, implicit line joining, f-strings) + recursive descent | pack 1.23.1 has **no `expression_statement` wrapper** (call/assignment sit directly in `block`; this differs from public tree-sitter-python docs); `assignment`/`augmented_assignment`; `parameters` with `typed_parameter`/`default_parameter`/`typed_default_parameter`/`list_splat_pattern`/`dictionary_splat_pattern`/`keyword_separator`/`positional_separator`; `type` wrapper around every annotation; `generic_type`/`type_parameter`; comprehensions (`for_in_clause`, `if_clause`); `pattern_list`/`tuple_pattern`; `with_clause`/`with_item`/`as_pattern`/`as_pattern_target`; `except_clause`; `match_statement`/`case_clause`/patterns; `conditional_expression`, `boolean_operator`, `not_operator`, `comparison_operator`, `binary_operator`, `unary_operator`; `await`, `lambda`/`lambda_parameters`, `named_expression`, `slice`, `subscript`; **f-string interior**: `[:string [:string_start] [:string_content [:escape_sequence]…] [:interpolation expr [:type_conversion] [:format_specifier [:format_expression expr]]] [:string_end]]`; `concatenated_string` | trailing-comment block attachment (Q9; needed for spans); `line_continuation` is an *extra* named node kept in the tree wherever tree-sitter attaches it (approximate); `print`/`exec` statements (py2 forms in grammar) | **near** (exact on typical code) |
| **TypeScript/TSX** | hand lexer (regex vs divide, template strings, JSX text mode) + recursive descent; one parser, TSX flag | statements (`lexical_declaration`, `variable_declaration`, `expression_statement`, `statement_block`, `for_in_statement` also for `for…of`); `formal_parameters`/`required_parameter`/`optional_parameter` (with `?`)/`rest_pattern`/`assignment_pattern`; `type_annotation`; the whole type language (`predefined_type`, `array_type`, `generic_type`, `union_type`/`intersection_type` with `|`/`&` kept, `literal_type`, `object_type`/`property_signature`, `function_type`, `tuple_type`, `type_query`, `lookup_type`, `index_type_query`, `conditional_type`, `infer_type`, `nested_type_identifier`, `type_predicate`…); `member_expression`/`optional_chain`/`subscript_expression`; `call_expression`/`new_expression`/`type_arguments`; `arrow_function` (`=>` kept); `object`/`pair`/`shorthand_property_identifier`/`spread_element`/`computed_property_name`; `as_expression`, `satisfies_expression`, `non_null_expression`, `type_assertion` (.ts only); `ternary_expression` (`?` kept); `await_expression`, `yield_expression`; JSX (`jsx_element`, `jsx_opening_element` and `jsx_closing_element` keep `<` `>` operators, `jsx_self_closing_element`, `jsx_attribute`, `jsx_expression`, `jsx_text`); class members (`public_field_definition`, `accessibility_modifier`, `override_modifier`, `decorator`) | ASI; arrow-vs-parenthesized and generic-call-vs-comparison disambiguation (`a < b > (c)`); `<T>x` cast vs JSX; the grammar is the largest of the set | **near** for application code, **approximate** for type-heavy code |
| **Rust** | hand lexer (raw strings, lifetimes vs chars, nested block comments) + recursive descent | `function_item` (`visibility_modifier`, `function_modifiers`, `parameters`/`parameter`/`self_parameter`, `type_parameters`, `where_clause`); block tail expression is **not** wrapped, while statements are `expression_statement`/`let_declaration` (`mutable_specifier`, `=` kept); `call_expression`/`arguments`; `field_expression`; `scoped_identifier` (path); `generic_function`, `generic_type`, `generic_type_with_turbofish`; `macro_invocation` + `token_tree` (flat tokens: idents, literals, nested `token_tree`, OPERATOR punctuation kept, other punctuation dropped); `closure_expression`/`closure_parameters` (`|` kept); `reference_expression`/`reference_type` (`&`), `try_expression` (`?`), `unary_expression`; `if_expression`/`let_condition`/`let_chain`; `match_expression`/`match_block`/`match_arm`/`match_pattern`; patterns (`tuple_struct_pattern`, `struct_pattern`, `field_pattern`, `shorthand_field_identifier`, `ref_pattern`, `or_pattern`, `range_pattern`); `struct_expression`/`field_initializer_list`; `range_expression`; `type_cast_expression`; `attribute_item`/`attribute`; `lifetime`; `self`/`crate`/`super` leaves | turbofish forms; macro token trees must split tokens the way tree-sitter-rust does; `negative_literal` in patterns; ERROR recovery common in macro-heavy code | **near** typical, **approximate** macro-heavy |

### 9.3 Risk assessment
- **Biggest risk: borderline scores.** The repo's TS fixture `src/stats.ts:1-11 ~ 13-23` scores **0.821428571429** (23/28) against the 0.82 threshold. One stray or missing wrapper node flips it from reported to silent. So "identical candidate pairs at the default threshold" **requires exact tree isomorphism** for that code. Approximate scores are not enough at the margin.
- Wrapper-node errors propagate: one wrong node changes the printed string of every ancestor. A single local error in a function's body can cut its self-similarity sharply. Golden diffs pinpoint these immediately.
- The grammar version is the oracle. Freeze upstream at language-pack 1.23.1. Any pack bump can silently change shapes, as with Python's missing `expression_statement`.
- Recursion depth: hand parsers and the normalizer must survive 1,200-deep expressions.

### 9.4 Recommended plan
1. Shared core: CST type, normalizer, fingerprints, pairing, report, CLI. Clojure reader port gives exact parity from day one.
2. Build Appendix B's oracle and a corpus: upstream tests, the repo's `fixtures/`, and a per-language construct catalogue (one function per grammar construct).
3. Go: `go/ast` translator. Python: hand parser. Then TS/TSX, Java, Rust, ordered by this repo's needs.
4. Acceptance per language:
   - (a) every upstream test case reproduces score and verdict
   - (b) the corpus matches printed normalized trees 100% for "exact" languages and ≥ 98% of candidates for "near"
   - (c) the `.metrics/dry.edn` for each fixture tree is byte-identical
5. Track any construct the hand parser does not model as a known divergence in a table. Never fall back silently.

---

## 10. Test cases to port

`pairs()` means `scan(min_lines=1, min_nodes=1)` then `find_duplicates(threshold=0.99)`.

### 10.1 Scores (all verified on the reference stack)

| Case | Left | Right | Expected |
|---|---|---|---|
| renamed locals ×5 langs (`test_each_language_matches_renamed_locals`) | Java/Go/Py/TS/TSX/Rust `alpha(xs){ys = filter(xs, odd); return map(ys, inc)}` | same with `beta/items/kept/even/dec` | each **1.0**. Nodes: go 61, java 52, py 35 (method in class), rust 62, ts 50, tsx (arrow in `const`) 58. TS pairs are ordered `(left.ts,right.ts)`, then `(left.tsx,right.tsx)`. Spans: go 3-6, java 2-5, py 2-4, rust 1-4, ts/tsx 1-4 |
| different callee | `def alpha(xs):\n    return filter(xs, odd)\n` | `…select…` | **0.428571428571** (9/21), nodes 19/19 |
| extra statement (Java) | JAVA_LEFT | adds `int[] extra = filter(kept, even);` and returns `map(extra, dec)` | **0.882352941176** (15/17), nodes 52/75, spans 2-5 / 2-6 |
| different member callee (py) | `return xs.filter(odd)` | `return items.select(even)` | **0.416666666667**, nodes 21/21 |
| different path callee (rust) | `fn alpha() {\n    Vec::new()\n}\n` | `Vec::with_capacity(1)` | **0.4**, nodes 19/20 |
| interpolated string | `return "xs"` | `return f"{items}"` | **0.285714285714**, nodes 11/19 |
| parenthesized callee | `return (len)(xs)` | `return len(items)` | **1.0**, nodes 18/18. The fingerprints contain `"len"` |
| string vs number | `return "xs"` | `return 1` | **1.0**. `:literal` is in the fingerprints |
| callee kept / member erased (`test_callee_names_stay_and_member_names_do_not`) | py `xs.filter(odd)` + `(len)(xs.total)`; go `fmt.Println(xs)`; rust `Vec::new()`, `HashMap::<String,i32>::new()`, `xs.iter()`; java `new ArrayList<String>()`, `ys.add(xs)`, `xs.total`; ts `foo<string>(xs.bar)` | renamed receivers/members: `log.Println`, `<Integer>`, `<number>`, `items.count`, `items.baz` | each language exactly one pair, **1.0** |
| CLI project (py) | `src/left.py` 4-line filter/map/list | `src/right.py` | **1.0**, nodes 48/48, 1-4 / 1-4. `tests/test_left.py` excluded |
| Clojure README alpha/beta (`.cljc`, `ns` first) | `(defn alpha [xs] (let [ys (filter odd? xs)] (map inc ys)))` | beta | **1.0**, nodes 31/31, spans 3-5 |
| Clojure invoice/receipt | test_clojure `invoice.clj` | `receipt.clj` | **0.890909090909**, 158/166, 3-13 / 3-14, reported at defaults |
| Clojure 3 copies in one file | `a`, `b`, `c` same body | — | 3 pairs, all 1.0 |
| Clojure maps/sets/keyword calls | `(when (#{:a :b} (:kind m)) {:left (:a m) :right (:b m)})` | `:c :d`, `row` | 1.0 |
| Clojure metadata/comment/discard | `; keep`, `#_(println xs)` | `^String` | 1.0 |
| Clojure reader conditional | `#?(:clj (when (pos? x) (inc x)))` | y | 1.0 (min_lines 1) |
| Clojure 2-line forms | `(defn a [x] (+ x 1))` | b | **not reported** at min_lines 3 |

Golden normalized forms (string-compare in Go unit tests):

```
PY  def alpha(xs): return filter(xs, odd)
[:function_definition :symbol [:parameters :symbol] [:block [:return_statement [:call [:symbol "filter"] [:argument_list :symbol :symbol]]]]]

PY  method alpha (PY_LEFT)
[:function_definition :symbol [:parameters :symbol :symbol] [:block [:assignment :symbol [:symbol "="] [:call [:symbol "filter"] [:argument_list :symbol :symbol]]] [:return_statement [:call [:symbol "map"] [:argument_list :symbol :symbol]]]]]

PY  f"{items}"   -> [:string [:string_start] [:interpolation :symbol] [:string_end]]

GO  GO_LEFT
[:method_declaration [:parameter_list [:parameter_declaration :symbol [:pointer_type [:symbol "*"] :symbol]]] :symbol [:parameter_list [:parameter_declaration :symbol [:slice_type :symbol]]] [:slice_type :symbol] [:block [:statement_list [:short_var_declaration [:expression_list :symbol] [:symbol ":="] [:expression_list [:call_expression [:symbol "filter"] [:argument_list :symbol :symbol]]]] [:return_statement [:expression_list [:call_expression [:symbol "mapFn"] [:argument_list :symbol :symbol]]]]]]]

GO  func a() {}  -> [:function_declaration :symbol [:parameter_list] [:block]]    (7 nodes)

JAVA JAVA_LEFT
[:method_declaration [:integral_type] :symbol [:formal_parameters [:formal_parameter [:array_type [:integral_type] [:dimensions]] :symbol]] [:block [:local_variable_declaration [:array_type [:integral_type] [:dimensions]] [:variable_declarator :symbol [:symbol "="] [:method_invocation [:symbol "filter"] [:argument_list :symbol :symbol]]]] [:return_statement [:method_invocation [:symbol "map"] [:argument_list :symbol :symbol]]]]]

JAVA this.<T>foo(1)  -> [:method_invocation [:this] [:symbol "foo"] [:type_arguments [:symbol "<"] :symbol [:symbol ">"]] [:argument_list :literal]]

RUST RUST_LEFT
[:function_item [:visibility_modifier] :symbol [:parameters [:parameter :symbol [:generic_type :symbol [:type_arguments [:symbol "<"] [:primitive_type] [:symbol ">"]]]]] [:generic_type :symbol [:type_arguments [:symbol "<"] [:primitive_type] [:symbol ">"]]] [:block [:let_declaration :symbol [:symbol "="] [:call_expression [:symbol "filter"] [:arguments :symbol :symbol]]] [:call_expression [:symbol "map"] [:arguments :symbol :symbol]]]]

RUST HashMap::<String, i32>::new()
[:call_expression [:scoped_identifier [:generic_type [:symbol "HashMap"] [:type_arguments [:symbol "<"] :symbol [:primitive_type] [:symbol ">"]]] [:symbol "new"]] [:arguments]]

TS  TS_LEFT
[:function_declaration :symbol [:formal_parameters [:required_parameter :symbol [:type_annotation [:array_type [:predefined_type]]]]] [:type_annotation [:array_type [:predefined_type]]] [:statement_block [:lexical_declaration [:variable_declarator :symbol [:symbol "="] [:call_expression [:symbol "filter"] [:arguments :symbol :symbol]]]] [:return_statement [:call_expression [:symbol "map"] [:arguments :symbol :symbol]]]]]

TSX TSX_LEFT (candidate = variable_declarator)
[:variable_declarator :symbol [:symbol "="] [:arrow_function [:formal_parameters [:required_parameter :symbol [:type_annotation [:array_type [:predefined_type]]]]] [:type_annotation [:array_type [:predefined_type]]] [:symbol "=>"] [:statement_block …same as TS_LEFT body…]]]

TS  return this.#total -> [:return_statement [:member_expression [:this] [:private_property_identifier]]]

CLJ (defn a [x] ()) contains "[:list :literal]";  {foo 1} prints == {bar 2};  #{foo bar} starts "[:set "
```

### 10.2 Candidate spans (`spans(lang, src)`)

| Source | Expected |
|---|---|
| `class Board:\n    def alpha(self, xs):\n        return xs\n` (py) | `[(2,3)]` |
| Python nested def (`def alpha` containing `def inner`) vs the same in beta | one pair, 1.0, left.start 1 (inner is not a candidate) |
| Java abstract class: `abstract int declared(...)`, then `alpha` with local class `Inner.run` (lines 3-10) | `[(3,10)]` |
| Go `func declared(xs []int)` (no body), then `alpha` lines 5-7 | `[(5,7)]` |
| Rust `fn alpha() {\n    let f = || { let x = 1; };\n}\n` | `[(1,3)]` |
| TS `outer` containing `class Hidden { run() {...} }` (1-8) | `[(1,8)]` |
| Rust `fn declared();`, `alpha` 3-5, `mod helpers { fn beta 8-10 }`, `mod tests { fn gamma }` | `[(3,5),(8,10)]` |
| Rust `mod tests { helper }` + top-level alpha vs RUST_RIGHT | one pair, left `a.rs` start 1 |
| TS `class Box{run 2-4}`, `outer 6-12` with nested `inner` and `const cb = (zs)=>zs`, top-level `xs.map((item)=>item+1)` | `[(2,4),(6,12)]` |
| TS `this.#total` / `this.#hidden(xs)` methods | `[(2,4),(5,7)]`. No fingerprint contains `#total` or `#hidden` |
| `def alpha(xs):\n    return xs\n` | `[(1,2)]`. Module root end_line == 2 (col-0 rule) |
| Python trailing comment `def a(x):\n    # lead\n    return x\n    # trailing comment\n` | `(1,4)` (new, from upstream behavior) |
| Java field initializer `Runnable r = new Runnable(){ public void run(){ go(); } };` (line 11) | candidate `(11,11)`. Same inside a constructor → none |

### 10.3 Pairing and filter unit tests (synthetic entries)
- Same file, same 1-4 span, same fps → `[]` at threshold 0. `a.py` 1-4 vs `b.py` 1-4 → 1 pair. `a.py` 1-5 vs 3-5 → 1. `a.py` 1-4 vs 1-8 → 1.
- `{x,y}` vs `{x,z}` (1/3) → kept at threshold exactly `1/3`, dropped at `1/3+0.01`.
- Ordering: rust `{x,y}~{x,z}` plus python `{only}~{only}` at 0.3 → scores `[1.0, 1/3]`.
- `_keep`: (1-4, 20 nodes, min 4/20) true; (1-3) false; (19 nodes) false.
- Deep input: Python `return 1 + 1 + …` (1,200 terms) and Clojure `(inc (inc … x))` (1,200 deep) must not crash and must produce entries.
- `node_count(K)` = 1. `node_count([:list :symbol :literal])` = 4. `pr(K("symbol"))` = `:symbol`.

### 10.4 Output and CLI
- `render_edn([Duplicate(0.875,"clojure",a.clj 10-14,b.clj 20-24,30,31)])` ==
  `"{:candidates [\n {:score 0.875\n  :language \"clojure\"\n  :left {:file \"a.clj\", :start-line 10, :end-line 14}\n  :right {:file \"b.clj\", :start-line 20, :end-line 24}\n  :left-nodes 30\n  :right-nodes 31}\n]}\n"`
- `format_text` of the same → `"DUPLICATE score=0.88\n  a.clj:10-14\n  b.clj:20-24\n"`. Empty → `"No duplicate candidates found.\n"`.
- `--help` → 0. stdout contains `Usage: dryer`, `__pycache__`, `.test.cts`, `--threshold`.
- `--format csv` → 2, stderr exactly `"Unknown format: csv\n"`. `--nope` → 1, `Unknown option: --nope`.
- `--threshold` (no value) → 1, `--threshold requires a value`. `file.py --min-lines ""` → 1, `--min-lines requires a value`. `--min-lines --min-nodes 1` → 1, `--min-lines requires a value`. `--min-lines nope` → `requires an integer`. `--threshold nope` → `requires a number`. `-1` → `non-negative`. `0` is ok.
- Parse `--threshold 0.9 --min-lines 5 --min-nodes 30 --edn spec` → 0.9/5/30/edn/positionals `[spec]`.
- `--edn` on a project with one 2-line def → stdout `"{:candidates []}\n"`. `--edn --text` → `"No duplicate candidates found.\n"`.
- Empty root → stdout `"No source files to analyze.\n"`, exit 0.
- Path filter `board` keeps `src/board/*`, drops `src/other/*`. Explicit `src/a.py tests/test_a.py` includes both.
- Language detection: `core.clj` and `core.cljd` → clojure, `ui/view.tsx` → typescript, `types.d.ts` and `notes.md` → none.
- Test names (all true): `pkg/foo_test.go`, `src/app_test.{clj,cljc,cljs,cljd,bb}`, `ui/a.{test,spec}.{ts,tsx,mts,cts}`, `tests/conftest.py`, `pkg/foo_test.py`, `pkg/test_foo.py`, `tests/app.py`. `src/app.py` is false.
- Walk: `{src/app.py, src/foo_test.py, target/app.py, tests/app.py, src/app.test.cts, src/app.spec.cts, src/widget.cts}` → `[src/app.py, src/widget.cts]`. `--source-root src` → only `src/a.py`.
- git, with a real repo:
  - `_changed_files` includes the renamed `src/b.py`, `src/c d.py`, `src/café.py`, and untracked `fresh/nested/new.py`. It excludes deleted `gone.py` and the rename source `src/old.py`.
  - In a non-repo, exit **128** with stderr containing `not a git repository`, and stdout without "No source files".
  - Mocked git exit 128 with blank stderr → message `git status failed`.
  - `--changed` from subdir `repo/sub` → only `sub/src/below.py`.
  - A single untracked `a.py` is found (short-name entry).
  - `--changed` report: new `src/a.py` + `src/b.py` reported, committed `src/c.py` absent, `tests/test_a.py` absent.
- The second run replaces the snapshot: `.metrics/dry.edn` == `"{:candidates []}\n"`.
- Clojure reader cases: copy `tests/test_clojure.py` verbatim. Warning strings: `"left.clj:6: unterminated list"`, `"2: unterminated string"`, `"1: unterminated string"`, `"1: incomplete character"`, `"1: reader conditional body must be a list"`, `"3: splicing reader conditional needs a collection"`. `"#?(:cljs (inc x) :default (dec x))"` → head `dec`. `"#?((inc x) (dec x))"` → no forms. Namespaced maps `#:foo{:a 1}`, `#::{:a 1}`, `#:{:a 1}` → map with key `a`. Reader macros `'x @x \`x ~x ~@x` → `quote deref syntax-quote unquote unquote-splicing`. `'#_x` → `(quote)`. `:` → `Kw("")`. `#!only` → no forms.

### 10.5 Harness goldens: the repo's `fixtures/` (upstream on copies, reference stack)
| `--root` | args | expected `.metrics/dry.edn` candidates |
|---|---|---|
| fixtures/go | defaults | 1: go `calc.go` 52-62 ~ 65-75, **1.0**, nodes 97/97 |
| fixtures/go | `--min-lines 1 --min-nodes 1 --threshold 0.5` | + `calc.go` 10-25 ~ 52-62 **0.525** (153/97), + 10-25 ~ 65-75 **0.525** |
| fixtures/python | defaults | 1: `src/shop/reports.py` 4-12 ~ 15-23, **1.0**, 100/100 |
| fixtures/python | loose | + `src/shop/pricing.py` 8-10 ~ `src/shop/reports.py` 43-44 **0.631578947368** (30/19) |
| fixtures/typescript | defaults | 1: `src/stats.ts` 1-11 ~ 13-23 **0.821428571429** (93/99). **Borderline canary.** |
| fixtures/typescript | loose | `src/router.ts` 19-21 ~ 23-25, 19-21 ~ 27-29, 23-25 ~ 27-29 (all 1.0, 48/48), then the stats pair |
| fixtures/javascript | any | `No source files to analyze.` (exit 0, no metrics file) |

Regenerate these with Appendix B whenever fixtures change. Do not hand-edit them.

---

## Appendix A — pack 1.23.1 grammar inventory

Operator tokens that can occur per grammar (anonymous ∩ OPERATORS):
- **python**: `!= % %= & &= * ** *= + += - -= / /= := < << <<= <= = == > >= >> >>= ^ ^= and not or | |= ~`
- **typescript/tsx**: `! != !== % %= & && &= * ** *= + ++ += - -- -= / /= < << <<= <= = == === => > >= >> >>= >>> >>>= ? ?. ?? ^ ^= | |= || ~`
- **go**: `! != % %= & && &= * *= + ++ += - -- -= / /= := < << <<= <= = == > >= >> >>= ^ ^= | |= || ~`
- **java**: `! != % %= & && &= * *= + ++ += - -- -= / /= < << <<= <= = == > >= >> >>= >>> >>>= ? ^ ^= | |= || ~`
- **rust**: `! != % %= & && &= * *= + += - -= / /= < << <<= <= = == => > >= >> >>= ? ^ ^= | |= ||`

Special-set members present per grammar:
- python: calls `call`, attrs `attribute`, args `argument_list`, literals `false float integer none string true`
- ts/tsx: calls `call_expression new_expression`, attrs `member_expression`, args `arguments`, targs `type_arguments type_parameters`, idents `identifier property_identifier shorthand_property_identifier shorthand_property_identifier_pattern type_identifier`, literals `false null number regex string template_string true undefined`
- go: calls `call_expression`, attrs `selector_expression`, args `argument_list`, targs `type_arguments`, idents `field_identifier identifier package_identifier type_identifier`, literals `false float_literal imaginary_literal int_literal interpreted_string_literal nil raw_string_literal rune_literal true`
- java: calls `method_invocation object_creation_expression`, attrs `field_access`, scoped `scoped_identifier scoped_type_identifier generic_type`, skip `block_comment line_comment`, literals `binary_integer_literal character_literal decimal_floating_point_literal decimal_integer_literal false hex_floating_point_literal hex_integer_literal null_literal octal_integer_literal string_literal true`
- rust: calls `call_expression`, attrs `attribute field_expression`, scoped `scoped_identifier scoped_type_identifier generic_type`, skip `block_comment doc_comment line_comment`, literals `boolean_literal char_literal float_literal integer_literal raw_string_literal string_literal`

Visible named kinds (the checklist a hand parser must cover; anything absent here can never be emitted):

**python (123)**: aliased_import argument_list as_pattern as_pattern_target assert_statement assignment attribute augmented_assignment await binary_operator block boolean_operator break_statement call case_clause case_pattern chevron class_definition class_pattern comment comparison_operator complex_pattern concatenated_string conditional_expression constrained_type continue_statement decorated_definition decorator default_parameter delete_statement dict_pattern dictionary dictionary_comprehension dictionary_splat dictionary_splat_pattern dotted_name elif_clause ellipsis else_clause escape_interpolation escape_sequence except_clause exec_statement expression_list false finally_clause float for_in_clause for_statement format_expression format_specifier function_definition future_import_statement generator_expression generic_type global_statement identifier if_clause if_statement import_from_statement import_prefix import_statement integer interpolation keyword_argument keyword_pattern keyword_separator lambda lambda_parameters line_continuation list list_comprehension list_pattern list_splat list_splat_pattern match_statement member_type module named_expression none nonlocal_statement not_operator pair parameters parenthesized_expression parenthesized_list_splat pass_statement pattern_list positional_separator print_statement raise_statement relative_import return_statement set set_comprehension slice splat_pattern splat_type string string_content string_end string_start subscript true try_statement tuple tuple_expression tuple_pattern type type_alias_statement type_conversion type_parameter typed_default_parameter typed_parameter unary_operator union_pattern union_type while_statement wildcard_import with_clause with_item with_statement yield

**typescript (177; tsx adds jsx_attribute jsx_closing_element jsx_element jsx_expression jsx_namespace_name jsx_opening_element jsx_self_closing_element html_character_reference and drops type_assertion)**: abstract_class_declaration abstract_method_signature accessibility_modifier adding_type_annotation ambient_declaration arguments array array_pattern array_type arrow_function as_expression asserts asserts_annotation assignment_expression assignment_pattern augmented_assignment_expression await_expression binary_expression break_statement call_expression call_signature catch_clause class class_body class_declaration class_heritage class_static_block comment computed_property_name conditional_type constraint construct_signature constructor_type continue_statement debugger_statement decorator default_type do_statement else_clause empty_statement enum_assignment enum_body enum_declaration escape_sequence existential_type export_clause export_specifier export_statement expression_statement extends_clause extends_type_clause false finally_clause flow_maybe_type for_in_statement for_statement formal_parameters function_declaration function_expression function_signature function_type generator_function generator_function_declaration generic_type hash_bang_line html_comment identifier if_statement implements_clause import import_alias import_attribute import_clause import_require_clause import_specifier import_statement index_signature index_type_query infer_type instantiation_expression interface_body interface_declaration internal_module intersection_type jsx_text labeled_statement lexical_declaration literal_type lookup_type mapped_type_clause member_expression meta_property method_definition method_signature module named_imports namespace_export namespace_import nested_identifier nested_type_identifier new_expression non_null_expression null number object object_assignment_pattern object_pattern object_type omitting_type_annotation opting_type_annotation optional_chain optional_parameter optional_type override_modifier pair pair_pattern parenthesized_expression parenthesized_type predefined_type private_property_identifier program property_identifier property_signature public_field_definition readonly_type regex regex_flags regex_pattern required_parameter rest_pattern rest_type return_statement satisfies_expression sequence_expression shorthand_property_identifier shorthand_property_identifier_pattern spread_element statement_block statement_identifier string string_fragment subscript_expression super switch_body switch_case switch_default switch_statement template_literal_type template_string template_substitution template_type ternary_expression this this_type throw_statement true try_statement tuple_type type_alias_declaration type_annotation type_arguments type_assertion type_identifier type_parameter type_parameters type_predicate type_predicate_annotation type_query unary_expression undefined union_type update_expression variable_declaration variable_declarator while_statement with_statement yield_expression

**go (107)**: argument_list array_type assignment_statement binary_expression blank_identifier block break_statement call_expression channel_type comment communication_case composite_literal const_declaration const_spec continue_statement dec_statement default_case defer_statement dot empty_statement escape_sequence expression_case expression_list expression_statement expression_switch_statement fallthrough_statement false field_declaration field_declaration_list field_identifier float_literal for_clause for_statement func_literal function_declaration function_type generic_type go_statement goto_statement identifier if_statement imaginary_literal implicit_length_array_type import_declaration import_spec import_spec_list inc_statement index_expression int_literal interface_type interpreted_string_literal interpreted_string_literal_content iota keyed_element label_name labeled_statement literal_element literal_value map_type method_declaration method_elem negated_type nil package_clause package_identifier parameter_declaration parameter_list parenthesized_expression parenthesized_type pointer_type qualified_type range_clause raw_string_literal raw_string_literal_content receive_statement return_statement rune_literal select_statement selector_expression send_statement short_var_declaration slice_expression slice_type source_file statement_list struct_type true type_alias type_arguments type_assertion_expression type_case type_constraint type_conversion_expression type_declaration type_elem type_identifier type_instantiation_expression type_parameter_declaration type_parameter_list type_spec type_switch_statement unary_expression var_declaration var_spec var_spec_list variadic_argument variadic_parameter_declaration

**java (142)**: annotated_type annotation annotation_argument_list annotation_type_body annotation_type_declaration annotation_type_element_declaration argument_list array_access array_creation_expression array_initializer array_type assert_statement assignment_expression asterisk binary_expression binary_integer_literal block block_comment boolean_type break_statement cast_expression catch_clause catch_formal_parameter catch_type character_literal class_body class_declaration class_literal compact_constructor_declaration constant_declaration constructor_body constructor_declaration continue_statement decimal_floating_point_literal decimal_integer_literal dimensions dimensions_expr do_statement element_value_array_initializer element_value_pair enhanced_for_statement enum_body enum_body_declarations enum_constant enum_declaration escape_sequence explicit_constructor_invocation exports_module_directive expression_statement extends_interfaces false field_access field_declaration finally_clause floating_point_type for_statement formal_parameter formal_parameters generic_type guard hex_floating_point_literal hex_integer_literal identifier if_statement import_declaration inferred_parameters instanceof_expression integral_type interface_body interface_declaration labeled_statement lambda_expression line_comment local_variable_declaration marker_annotation method_declaration method_invocation method_reference modifiers module_body module_declaration multiline_string_fragment null_literal object_creation_expression octal_integer_literal opens_module_directive package_declaration parenthesized_expression pattern permits program provides_module_directive receiver_parameter record_declaration record_pattern record_pattern_body record_pattern_component requires_modifier requires_module_directive resource resource_specification return_statement scoped_identifier scoped_type_identifier spread_parameter static_initializer string_fragment string_interpolation string_literal super super_interfaces superclass switch_block switch_block_statement_group switch_expression switch_label switch_rule synchronized_statement template_expression ternary_expression this throw_statement throws true try_statement try_with_resources_statement type_arguments type_bound type_identifier type_list type_parameter type_parameters type_pattern unary_expression underscore_pattern update_expression uses_module_directive variable_declarator void_type while_statement wildcard yield_statement

**rust (163)**: abstract_type arguments array_expression array_type assignment_expression associated_type async_block attribute attribute_item await_expression base_field_initializer binary_expression block block_comment boolean_literal bounded_type bracketed_type break_expression call_expression captured_pattern char_literal closure_expression closure_parameters compound_assignment_expr const_block const_item const_parameter continue_expression crate declaration_list doc_comment dynamic_type else_clause empty_statement enum_item enum_variant enum_variant_list escape_sequence expression_statement extern_crate_declaration extern_modifier field_declaration field_declaration_list field_expression field_identifier field_initializer field_initializer_list field_pattern float_literal for_expression for_lifetimes foreign_mod_item fragment_specifier function_item function_modifiers function_signature_item function_type gen_block generic_function generic_pattern generic_type generic_type_with_turbofish higher_ranked_trait_bound identifier if_expression impl_item index_expression inner_attribute_item inner_doc_comment_marker integer_literal label let_chain let_condition let_declaration lifetime lifetime_parameter line_comment loop_expression macro_definition macro_invocation macro_rule match_arm match_block match_expression match_pattern metavariable mod_item mut_pattern mutable_specifier negative_literal never_type or_pattern ordered_field_declaration_list outer_doc_comment_marker parameter parameters parenthesized_expression pointer_type primitive_type qualified_type range_expression range_pattern raw_string_literal ref_pattern reference_expression reference_pattern reference_type remaining_field_pattern removed_trait_bound return_expression scoped_identifier scoped_type_identifier scoped_use_list self self_parameter shebang shorthand_field_identifier shorthand_field_initializer slice_pattern source_file static_item string_content string_literal struct_expression struct_item struct_pattern super token_binding_pattern token_repetition token_repetition_pattern token_tree token_tree_pattern trait_bounds trait_item try_block try_expression tuple_expression tuple_pattern tuple_struct_pattern tuple_type type_arguments type_binding type_cast_expression type_identifier type_item type_parameter type_parameters unary_expression union_item unit_expression unit_type unsafe_block use_as_clause use_bounds use_declaration use_list use_wildcard variadic_parameter visibility_modifier where_clause where_predicate while_expression yield_expression

---

## Appendix B — differential oracle (run outside `upstream/`)

Setup: `python3 -m venv /tmp/dryer-env/venv && /tmp/dryer-env/venv/bin/pip install tree-sitter==0.26.0 tree-sitter-language-pack==1.23.1`. Copy `upstream/dryer/src` next to it; never run inside `upstream/`, because pytest and the tool write caches and `.metrics`.

```python
# dryer_oracle.py <lang> <file> [--tree]   ->  one JSON line per candidate
import sys, json; sys.path.insert(0, "src")
from dryer.extract import _PICKERS, _grammar
from dryer.treesitter import parse, start_line, end_line
from dryer.astnorm import normalize
from dryer.shape import pr, node_count, fingerprints
lang, path = sys.argv[1], sys.argv[2]
src = open(path, encoding="utf-8", errors="replace").read()
g, pick = _PICKERS[lang]; g = g(path) if callable(g) else g
data, tree = parse(src, g)
for n in pick(data, tree.root_node):
    v = normalize(n, data)
    print(json.dumps({"start": start_line(n), "end": end_line(n), "nodes": node_count(v),
                      "fps": len(fingerprints(v)), "norm": pr(v)}))
```

Go tests: for each corpus file, run the Go extractor and require equal `start`, `end`, `nodes`, and `norm` per candidate, in order. For Clojure use `dryer.clojure_read.read_source` + `is_candidate_form` + `normalize` + `max_line`. For CLI goldens, run `python -m dryer --root <copy>` and compare stdout, stderr, and `.metrics/dry.edn` byte for byte. Replace the absolute root in `Wrote …`.
