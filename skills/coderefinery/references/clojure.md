# Clojure

Files: `.clj .cljc .cljs .bb` (dry also `.cljd`). Skipped: `*_test.clj`,
`*_test.cljc`, `*_test.cljs`, `*_test.bb`, `test/`, `spec/` and the other test
directories. Namespace: the `ns`; the function name is the `defn` name. dry
compares every top-level form except `ns`.

## crap and mutate: coverage

- Root only. Needs `deps.edn` or `bb.edn` at the root, else
  `No deps.edn or bb.edn; skipping Clojure coverage.`
- `clj -M:cov --lcov` (retried without `--lcov` on failure). The project must
  define a `:cov` alias running Cloverage.
- Coverage per function comes from Cloverage's HTML form counts
  (`C out of T forms covered`), else from LCOV.
- `Clojure coverage exited <n>. Clojure coverage will score 0%.` means a failed
  run.
- mutate test command, from the nearest `deps.edn` (else `bb.edn`):
  `clj -M:spec --tag ~no-mutate` (speclj), `clj -M:spec`, `clj -M:test`,
  `bb spec --tag ~no-mutate` or `bb test`. The speclj and `bb spec` forms skip
  tests tagged `no-mutate`.
- Mutations swap head symbols (`+`/`-`, `<`/`<=`, `if`/`if-not`, `when`/`when-not`,
  `and`/`or`, `first`/`second`, `filter`/`remove`, ...) and flip `true`/`false`
  and `0`/`1` literals.

## coderefinery test

- Kaocha through the `:test` alias:
  `clj -M:test --plugin kaocha.plugin/junit-xml --junit-xml-file <results> --plugin cloverage --cov-output <dir> --lcov`.
- Needs kaocha-junit-xml and kaocha-cloverage in that alias:
  `add lambdaisland/kaocha-junit-xml (and lambdaisland/kaocha-cloverage) to the :test alias`;
  and `clj is not installed: install the Clojure CLI or pass --test-command`.
- speclj, bb and `clj -M:spec` are not supported: pass `--test-command`. lein
  test2junit output is parsed, but lein is not run.
- Test ids: the namespace resolved to its file under the test paths, then the
  var name.
- Status: sample-only.

## Branches

- Granularity `form`, from Cloverage: `2 of 5 forms not covered`. It names code
  inside an executed form that never ran, not an arm.

## Approximate or different

- `(some-> ...)` and similar threading forms can make complexity drop below 1
  (upstream floor division).
- `#(+ % 1)`: `+` is not a mutation site (anonymous-function literal), the `1` is.
  `#_(...)` discarded forms are mutated.
- The reader does not reproduce tree-sitter's error recovery: malformed files
  contribute nothing.
