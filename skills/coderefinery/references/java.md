# Java

Files: `.java`. Skipped: test directories (`test/`, `target/`, `build/`, ...).
Namespace: `package.Class`, the function name is the method name.

## crap and mutate: coverage (Maven only)

- Per nearest `pom.xml`:
  `mvn -q org.jacoco:jacoco-maven-plugin:0.8.12:prepare-agent test org.jacoco:jacoco-maven-plugin:0.8.12:report`.
  Maven resolves the plugin; nothing to install besides Maven.
- Coverage per method is JaCoCo **instruction** coverage, matched by class and
  method name, then by the nearest line (JaCoCo's line is the first body line).
- `target/site/jacoco/jacoco.xml` is read only at the root and one or two
  directories below. Deeper modules are measured but not read.
- A malformed JaCoCo XML stops the run (exit 1).
- mutate runs `mvn -q test -DexcludeTags=no-mutate`: tests tagged `no-mutate`
  never run against mutants.
- Gradle: crap and mutate have no Gradle plan (upstream). Pass
  `--coverage-command` (it must leave `target/site/jacoco/jacoco.xml`) and, for
  mutate, `--test-command`.
- A package directory named `build`, `coverage`, `out`, `dist` or `target`
  (`com.acme.build`) is left out of the worker copy (upstream), so mutants that
  need it are false KILLED (`· by <build>` with `--attribute`). Add
  `--fix-overlay` (on with `--attribute`): it links such a directory when it
  holds `.java` files. `target/` at the project root is never linked.
- stderr `Java coverage exited <n> in <module>. Java coverage will score 0%.`
  means a failed build or tests.

## coderefinery test

- Maven: the same JaCoCo goals plus `-Dmaven.test.failure.ignore=true
  -Dsurefire.skipAfterFailureCount=0`; results from `target/surefire-reports/TEST-*.xml`.
- Gradle: `./gradlew test jacocoTestReport --continue`, results from
  `build/test-results/test/`. Without the jacoco plugin:
  `apply the jacoco plugin to get coverage, or pass --no-coverage`.
- Missing tools: `mvn is not installed: install Maven or pass --test-command`,
  `gradle is not installed: install Gradle or pass --test-command`.
- Test ids: the test file path when it resolves (`src/test/java/<pkg>/<Outer>.java`),
  then `Outer::Inner::method`.
- `mutate --attribute` runs the full module suite. Gradle attribution needs
  `--test-command`.
- Status: sample-only (parsers tested on recorded reports; the runner is not
  run in CI).

## Branches

- Granularity `branch-count`, from JaCoCo `mb`/`cb` per line:
  `1 of 4 branches missed`. It gives the line and the count, not the arm: read
  the line and find the arm yourself.

## Approximate or different

- Methods of anonymous classes in field initializers are entries.
- Constructors have no mutation sites (no owner in crap's function list).
- `(Foo) -y` with a reference type is mutated as a binary `-`; tree-sitter
  cannot tell it from a cast.
- Files the parser rejects contribute nothing: invalid code, `0_7`/`0x_1`
  literals, annotations before `...`.
