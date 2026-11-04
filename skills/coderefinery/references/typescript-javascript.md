# TypeScript and JavaScript

Files: `.ts .tsx .mts .cts .js .jsx .mjs .cjs`. Skipped: `*.test.*`, `*.spec.*`,
`__tests__/`, `node_modules/`, `dist/`, `build/`, `coverage/` and the other test
directories. Namespace: the dotted module path; methods are `module.Class`.
`dry` compares TypeScript and TSX only; plain JavaScript (`.js .mjs .cjs .jsx`)
is never compared, except JSX subtrees under `dry --jsx` (`.tsx .jsx .js`; see
`react.md`). For React components and hooks, also run `coderefinery react`
(see `react.md`): crap charges every hook callback and handler to its component.

## crap and mutate: coverage

Per nearest `package.json`:

1. `scripts.coverage` exists: `npm run coverage`. In a sub-package that writes
   `<pkg>/coverage/lcov.info`, which is not read (only `<root>/coverage/**` and
   `<root>/target/coverage/**` are). Use `--coverage-command` there.
2. Vitest (`vitest` in `scripts.test`, or a `vitest.config.*`): `vitest run
   --coverage` with LCOV into `target/coverage/typescript[/<pkg>]/`. If
   `@vitest/coverage-v8` is missing, crap installs it with `npm install --no-save`
   and restores `package.json` and the lockfile (upstream behaviour).
3. Otherwise `npx --yes c8 --reporter=lcov ... npm test`.
4. No `test` script: skipped (`No package.json test script in <pkg>`).

Coverage per function is the LCOV branch percent when the range has `BRDA`
records, else line percent. mutate runs `npm test` in the nearest
`package.json` directory. For React projects add `mutate --react` (see
`react.md`).

## --fix-overlay (mutate)

mutate runs each mutant in a worker that links the project's files (upstream).
Test runners see through the links:

- Vitest with `environment: "jsdom"`: Vite resolves a linked test or setup file
  to its real path, outside the worker, and refuses to load it
  (`Failed to load url ... Does the file exist?`). Every run fails, so **every
  mutant is reported KILLED**.
- Jest skips symlinked files: linked test files do not run, so a test that
  reaches the mutated file through a non-relative import (an alias,
  `moduleDirectories`) never kills it, and the mutant is reported SURVIVED.

`--fix-overlay` makes the worker hold real directories and copies of the
project's `.ts .tsx .mts .cts .js .jsx .mjs .cjs` files (other files,
`node_modules` and dot-directories stay links). The project and its config are
not touched; no `server.fs` setting is needed. `--attribute` turns it on.
Cost: one copy of the JS/TS sources per worker. The skip list stays upstream's
here: `dist/`, `build/`, `coverage/`, `out/` and `target/` hold build output and
reports in these projects. A coderefinery addition, hidden from `mutate --help`.

## coderefinery test

- Only when the first token of `scripts.test` is `vitest`, `jest` or `mocha`.
  Anything else: `cannot add a JUnit reporter to npm test ("<script>"): pass --test-command`.
  node:test and other runners: `--test-command` only.
- Vitest: `npm test -- --reporter=junit ... --coverage` (env `CI=true`). Needs
  `@vitest/coverage-v8` or `-istanbul`: `install the Vitest coverage provider: npm i -D @vitest/coverage-v8@<vitest version>`.
- Jest: needs jest-junit (`install jest-junit: npm i -D jest-junit`).
- Mocha: needs mocha-junit-reporter (`install mocha-junit-reporter: npm i -D mocha-junit-reporter`)
  and c8 (`install c8: npm i -D c8`).
- coderefinery never installs these; the one line says what to install (exit 1).
- `mutate --attribute` runs the full suite for every mutant (no narrowing).
- Status: Vitest and Jest live-tested; Mocha parser-only.

## Branches

- Granularity `branch` from Istanbul JSON (`coverage-final.json`, written by
  `coderefinery test`): arm-precise for `if`, `?:`, `&&`/`||`/`??` operands,
  `switch` cases and default arguments.
- From crap's LCOV alone (`BRDA`): `branch-count`, a count per line; read the
  line to find the arm.

## Approximate or different

- Express-style route callbacks are their own entries (`GET /users#2`). The
  detection has no receiver check, so `cache.get("k", cb)` becomes `GET k`.
- Class-field arrows and `var`/`module.exports` functions are not entries, and
  their decisions are counted nowhere.
- Object or class-expression methods inside a named class's method are counted
  twice and namespaced to the outer class.
- Files the parser rejects contribute nothing: `export type * from "m"`, a bare
  `&` in JSX text, `a ?.5 : 1`, `useState<import("m").T[]>([])`.
