# design-react: React metrics for function components and hooks

Status: v1 built (`internal/react`, `coderefinery react`). An **addition**: crapper, dryer and mutator
have no React mode, so nothing here touches their flags, `--help`, stdout, exit codes or `.metrics`
bytes (harness 36/36). The command is stdlib-only Go and reads the same tree the crap TypeScript reader
reads (`internal/lang/typescript`, tree-sitter-shaped TSX/JSX). Every v1 metric is **static**; only
CRAP's coverage needs a test run (the existing TypeScript coverage runner and LCOV loader).

## 1. Why: what crap sees today (calibration, excalidraw `dd2f8fc`)

`crap --no-coverage packages/excalidraw/components` scores a component as one function: every hook
callback, handler and nested render helper is charged to it. `LayerUI` is one row at CC 95;
`mutate --scan LayerUI.tsx` tags all 127 sites `[defn/LayerUI]`. Only Express route callbacks
(`GET /users#2`) are split out today (`internal/crap/lang_typescript.go`). `dry` works on TSX as on
any TS, with no notion of JSX. A CC-95 component says "big" without saying whether the render tree,
an effect or a handler is the problem.

## 2. Units: what counts as a component or a hook

Searched at module level (not inside a function or class):

| unit | rule |
|---|---|
| component | `function Name()` or `const Name = <arrow \| function expression>` whose name is PascalCase, or a `memo(...)` / `forwardRef(...)` / `React.memo` / `React.forwardRef` wrapper (any nesting), **and** the function returns JSX: its expression body, or one of its own `return` statements (not a nested function's), holds a JSX element |
| custom hook | the same declaration forms named `use[A-Z0-9]...`, any body |
| nested component | a component definition inside a unit: its own unit, named `Outer.Inner`, and counted in the outer unit's `nested-components` |

Not units: class components (out of scope by decision; crap already scores their methods), `export default function () {}`
(no name), components built by factories, object-literal members, `memo(Foo)` of an existing `Foo`
(`Foo` is the unit).

## 3. Entries: the CRAP rows

Each unit is split the way crap splits Express routes: the extracted function's decisions are **not**
also charged to the unit.

| entry | kind | name | span |
|---|---|---|---|
| unit body | `component` / `hook` | `Cart` | the declaration (declarator for `const`) |
| hook callback: an arrow/function argument of any `use*` call (`useEffect`, `useMemo`, `useCallback`, `useLayoutEffect`, `useState(() => init)`, custom hooks) | `callback` | `Cart#useEffect#1`, numbered per hook name in source order | the function |
| local handler: `const f = () => {}`, `function f() {}` directly in the unit body | `handler` | `Cart.onSubmit` | the declarator / declaration |
| inline JSX handler: an attribute whose value is a function literal, anywhere in the unit's JSX | `handler` | `Cart.onClick`; repeats `Cart.onClick#2` | the function |

A name already taken in the unit gets `#2`, `#3`... (crap's `_claim`). Other nested functions
(`.map`, `.filter`, `setTimeout` callbacks) stay charged to their enclosing entry, as in crap.

**Complexity** is crap's TypeScript decision set (`crap.TypeScriptDecision`: if, loops, catch, case,
ternary, `&&`, `||`, `??`, `?.`) counted over the entry, so a split row and a crap row agree on
whatever they both contain. **CRAP** = CC² × (1 − cov)³ + CC (`crap.Score`, rounded like crap), with
coverage from `coverage.Bundle.PercentFor` over the entry's line span (LCOV branch percent when the
range has `BRDA`, else lines). A unit body's span includes its extracted entries' lines (v1).

### Render vs logic split (units)

- **Render** = 1 + decisions inside what the component renders: the expression of its own `return`
  statements and of an expression body, any JSX subtree, the condition of an early return
  (`if (...) return ...` whose consequence is or holds a `return`), plus one per `.map(...)` call
  inside rendered JSX (a list render; not a crap decision, so not in CC).
- **Logic** = 1 + the unit's other decisions (body code that is neither rendered nor extracted).
- CC of the unit entry = 1 + render decisions without `.map` + logic decisions.

For hooks the `return` rule does not apply: Render counts JSX only (usually 1).

## 4. Unit metrics

| group | metric | definition |
|---|---|---|
| props | count | first parameter: destructured names (`{a, b = 1, c: d, "e": f}`), plus the members of the props type: a type annotation on the parameter, `React.FC<P>` / `FC<P>` on the declarator, or the last type argument of `memo<P>` / `forwardRef<R, P>`; `P` is resolved across modules and type operators (§10); an unresolvable part makes the props **open** (counts are lower bounds, `9+`, `:open true`). A plain `props` parameter adds `props.x` reads and `const {x} = props` |
| | boolean | type `boolean` / `true \| false` (optionally `\| undefined \| null`), or a `true`/`false` default |
| | optional | `?:` in the type, or a default value |
| | callback (`On`) | name `on[A-Z]...`, a function type, or a method signature |
| | rest | `...rest` in the pattern, or `{...props}` spread of a plain `props` parameter |
| | pass-through | props whose every read is the whole value of an attribute of a child **component** (`<Child x={x} />`, `<Child x={props.x} />`; PascalCase or dotted element). A read anywhere else (logic, hooks, a DOM element, shorthand `{x}`) is a use. Shadowing is not tracked. The prop-drilling signal |
| hooks | state | `useState` / `React.useState` calls |
| | effects | `useEffect`, `useLayoutEffect`, `useInsertionEffect` calls |
| | deps | the largest dependency-array literal among `useEffect`-family, `useMemo`, `useCallback`, `useImperativeHandle` |
| | setter-effects | effects whose callback only calls this unit's `useState` setters: an expression body that is a setter call, or a block whose statements are all setter calls or `if`/`else` of those (no cleanup, no other statement). The derived-state smell ("you might not need an effect") |
| render cost | jsx-depth | deepest JSX element nesting in the unit (fragments do not count; JSX inside `.map` counts) |
| | elements | JSX elements (fragments excluded) |
| | inline-functions | JSX attributes whose value is a function literal (each is also a handler entry) |
| | inline-objects | JSX attributes whose value is an object or array literal (`style={{...}}`) |
| | nested-components | component definitions inside the unit (re-created, so remounted, on every render) |

## 5. Thresholds

An entry offends at **CRAP ≥ 10** (`--crap`, the skill's "CRAP < 10"). With `--no-coverage` CRAP is
N/A and the entry offends at **CC ≥ 10**: CC is CRAP's floor (100% coverage), so CC ≥ 10 can never
pass the CRAP gate. A unit offends when a metric is **above** its limit (`--max-<flag> <n|off>`):

| flag | default | flag | default |
|---|---|---|---|
| `--max-props` | 7 | `--max-deps` | 6 |
| `--max-bool-props` | 2 | `--max-setter-effects` | 0 |
| `--max-pass-through` | 3 | `--max-jsx-depth` | 6 |
| `--max-state` | 5 | `--max-nested` | 0 |
| `--max-effects` | 3 | `--max-elements`, `--max-inline-functions`, `--max-inline-objects` | off |

Calibration on excalidraw (whole repo, `--no-coverage`): 360 units (314 components, 46 hooks), 977
entries, 145 violations: complexity 62, props 35, pass-through 16, boolean-props 13, setter-effects 6,
deps 5, effects 4, state 2, jsx-depth 1, nested-components 1. Changes made after that run:
pass-through went from > 2 to > 3 (26 → 16 violations; the dropped ones forward exactly 3 props,
e.g. the `Stats/*` editors passing `appState`, `scene`, `property` down); element count and inline function/object props stay reported but off by
default: inline handlers are idiomatic React and only matter for memoised children, which a static
pass cannot see. Every setter-effects and nested-components hit was a true positive on inspection
(`ColorInput` copies a prop into state in an effect; `FontPickerList` defines `FontPickerListItem`
in its body).

## 6. Command, output, exit codes

```
coderefinery react [--root <path>] [--changed] [--no-coverage | --use-existing-coverage |
                   --coverage-command <cmd>] [--crap <n>] [--max-<flag> <n|off>]... [path-or-filter ...]
```

Files: crap's selection (`crap.SelectFiles`: paths, filters, `--changed`, skipped directories, test
files excluded), kept to `.ts .tsx .mts .cts .js .jsx .mjs .cjs`; a file the parser rejects
contributes nothing (DEVIATIONS A2). Coverage: unless `--no-coverage`, the TypeScript coverage runner
(`npm run coverage`, Vitest LCOV, or c8) runs first, as in crap; `--use-existing-coverage` reads
the LCOV already under `coverage/` and `target/coverage/`.

Stdout, three tables (no violations table when clean):

1. `React Report`: Entry, Kind, Namespace (crap's TS module namespace), Line, CC, Cov%, CRAP; sorted
   like crap (scored by −CRAP, unscored by −CC, then namespace, name, path).
2. `Components and hooks`: Render, Logic, Props, Bool, Opt, On, Rest, Pass, State, Eff, Deps, SetFx,
   Depth, Elems, InFn, InObj, Nest (props columns are `-` for hooks); worst first (most violations,
   then Render + Logic, then namespace, name).
3. `Threshold violations`: name, namespace, `props 9 > 7` / `crap 27.1 >= 10.0` /
   `complexity 10 >= 10.0 (no coverage)`; entry rows first, then units in table-2 order.

Stderr: `Wrote <root>/.metrics/react.edn`, then `React thresholds exceeded: N violation(s)`.

| exit | meaning |
|---|---|
| 0 | no violations (also: no files, no units) |
| 1 | usage error, or an internal error (`react: <message>`); a git failure under `--changed` exits with git's code |
| 2 | the coverage run failed (a per-language TypeScript coverage command or `--coverage-command` exited non-zero: failing tests). Nothing is written. Fix the tests or use `--no-coverage` |
| 3 | at least one violation |

### `.metrics/react.edn`

```clojure
{:entries [
  {:name "Cart#useEffect#1", :namespace "Cart", :kind :callback, :unit "Cart", :path "src/Cart.tsx",
   :line 22, :complexity 10, :coverage 44.4444, :crap 27.1468}   ; one line per entry
]
 :components [
  {:name "Cart", :namespace "Cart", :kind :component, :path "src/Cart.tsx", :line 16, :render 5, :logic 1,
   :props {:count 4, :boolean 0, :optional 1, :callback 1, :rest false, :pass-through [], :open false},
   :hooks {:state 4, :effects 2, :max-deps 2, :setter-effects 1},
   :render-cost {:jsx-depth 3, :elements 9, :inline-functions 0, :inline-objects 0, :nested-components 0}}
]
 :violations [
  {:name "Cart", :namespace "Cart", :path "src/Cart.tsx", :line 16, :metric :setter-effects, :value 1.0, :limit 0.0}
]}
```

Entries and units in report order; `:props nil` for hooks; `:coverage`/`:crap` `nil` without
coverage; numbers use crap's EDN number format. `:metric` is `:crap`, `:complexity` or a table-1
metric name (`:props :boolean-props :pass-through :state :effects :deps :setter-effects :jsx-depth
:elements :inline-functions :inline-objects :nested-components`).

## 7. Shared code touched (behaviour-preserving)

- `internal/crap/lang_typescript.go`: `TypeScriptDecision`, an exported alias of the reader's
  predicate (no logic change).
- `internal/coverage/runners.go`: `Runner.Failed` counts TypeScript coverage runs that exited
  non-zero; crap ignores it, so its output and exit codes are unchanged.
- Reused as is: `crap.SelectFiles`, `crap.TypeScriptModuleNamespace`, `crap.Complexity`,
  `crap.Score`, `crap.RoundMetric`, `coverage.Runner`, `coverage.LoadBundle`, `lang.ParseText`.

## 8. Fixture and tests

`fixtures/react` (React 19, Vitest 3.2.7 + v8 coverage, jsdom, @testing-library/react; installed by
`fixtures/setup-tests.sh`): a clean `Badge`; `SettingsPanel` (9 props, 5 boolean); `Cart` (a CC-10
effect, a setter-only effect, handlers, an early return); `Dashboard` → `Layout` → `Sidebar` (prop
drilling, `props.x` reads); `TodoList` (inline handlers and objects, a nested `TodoRow`); `IconButton`
(`memo(forwardRef<R, P>)`, untested); `useCounter` (a custom hook); `Toolbar` (props
`Omit<ButtonBase, "variant"> & {...}` with `ButtonBase` from `src/types/` through an index re-export: 8
props) and `ToolbarGroup` (`& HTMLAttributes<HTMLDivElement>`: open, `2+`). `cmd/coderefinery/react_e2e_test.go`
checks the exact `--no-coverage` report and exit 3 (8 violations), the coverage run (exact Cov%/CRAP
rows, 9 violations; skipped without `node_modules` or under `-short`) and exit 2 on a failing
coverage command. `internal/react/react_test.go` covers each metric with exact values.

## 9. Follow-ups (v2, out of scope here)

- **React mutation operators** in `mutate` (opt-in, hidden flag, like `--attribute`): remove a
  conditional render (`cond && <X/>` → `false`), swap ternary branches in JSX, drop a prop from a
  child element, turn a handler into a no-op, remove one dependency from a dependency array.
- **dry over JSX subtrees**: duplicate element trees modulo literals and attribute values.
- **Runtime render cost** via `<Profiler>` (render counts and durations per component during the
  test run) to confirm what inline-functions / inline-objects only suggest statically.
- A unit body's coverage without its extracted entries' lines, shadowing in pass-through.
  (Props from imported types and `Omit`/`Pick`: done, §10.)

## 10. Props type resolution (v2)

`internal/react/types_modules.go` (modules) and `types.go` (type evaluation). Stdlib only, no
TypeScript checker: a syntactic evaluator that knows what it does not know.

**Modules.** Each module is indexed once per run (one `resolver` per `react` run caches every parsed
file, the analysed files included): top-level `interface` / `type` declarations (merged interfaces
kept together), imports (`import type { A as B }`, `{ type A }`, default, `* as NS`), exports
(`export interface`, `export default interface X` / `export default X`, `export { X as Y }`,
`export { X as Y } from`, `export { default as Y } from`, `export *`, `export * as NS`).
Specifiers resolve inside the project root only (never `node_modules`, never a package): relative
paths with TypeScript's order (`.tsx .ts .d.ts .jsx .js .mts .cts`, then `<dir>/index.*`; an ESM
`./x.js` also tries `./x.ts`), then the nearest `tsconfig.json` / `jsconfig.json` between the file
and the root: `compilerOptions.paths` (exact keys first, then the longest wildcard prefix; targets
relative to `baseUrl` when set, else to the file declaring `paths`) and `baseUrl`, following a
relative `extends` chain (a package `extends` is not followed). tsconfig is read as JSON with
comments and trailing commas.

**Types.** Evaluated to a member list (name, optional, boolean, callback, rules of §4):

| form | members |
|---|---|
| interface (with `extends A, B<T>`, merged declarations), type alias, object type | its signatures plus the bases' |
| `A & B` | both; a prop is optional only when optional on both sides |
| `A \| B` | every branch's members; a member missing from a branch is optional; `undefined` / `null` branches skipped |
| `Omit<T, K>` / `Pick<T, K>` | `K` is a string literal, a union of them, `keyof X` of a closed `X`, or an alias of those; a picked key the (open) base does not list still counts, type unknown |
| `Partial` / `Required` / `Readonly` | all optional / none optional / unchanged (also `React.`-qualified) |
| `PropsWithChildren<P>` | `P` plus `children?` (React's definition) |
| `Box<X>` / `Box` of `type Box<T = D> = ...` | type parameters bound to the arguments, else to their defaults; an unbound parameter is open |
| `NS.T` with `import * as NS from "./x"` | `T` exported by `./x` |

**Open.** Anything else marks the props open and contributes only what is known: names imported from
a package (`react`, `ComponentProps<'button'>`, `HTMLAttributes<...>`, `React.*`), a module that is
missing or does not parse, `any` / `unknown` / `object`, an index signature or mapped type,
conditional types, `typeof`, indexed access, function types, a cycle (an alias or interface reached
again while being expanded) and depth > 32 or > 16 import hops. Destructured names still count. An
untyped parameter (JavaScript) is not open: there is no type to resolve; `...rest` stays the
separate `Rest` column. Report: `Props` shows `9+`; EDN `:props {... :open true}` (always present).

**Violations with open props.** Open never suppresses or invents a violation: the known counts are
lower bounds (more members can only add props, booleans, optionals, callbacks), so a known count
above a limit is a real violation, and an open unit under the limit is reported as is (`2+`) without
offending. Thresholds are unchanged.

**Side fix.** Quoted member names (`"data-testid": string`) are now unquoted like destructured keys,
so a quoted prop that is both typed and destructured is counted once (it was counted twice).

### Calibration (excalidraw `dd2f8fc`, whole repo, `--no-coverage`)

314 components. Before: 25 had a props type the in-file resolver could not fully read (imports,
`Omit`/`Pick`, library types), silently. After: 4 of them resolve completely (e.g. `ExcalidrawBase`,
whose `ExcalidrawProps` is imported: 52 → 53 props, optional 6 → 53, booleans 4 → 11), 21 stay open,
all on React's attribute types, directly or through an imported props type
(`& React.HTMLAttributes<HTMLDivElement>`, `Omit<React.ButtonHTMLAttributes<...>, ...>`,
`React.RefAttributes`, `DetailedHTMLProps`); none newly open. 15 units' props changed.
Violations 145 → 147: new `props` on `ConfirmDialog` (10, `extends Omit<DialogProps, ...>` from
`./Dialog`), `DropdownMenuItemCheckbox` (8+), `IconButton` (21, a three-branch union over a shared
base) and `IconButton` boolean-props (6); gone `props` on `Island` and `ToolPopover` (8 → 6 and 8 → 7:
the quoted-name double count). Runtime unchanged within noise (~0.5 s user before and after; every
imported module is parsed once).

## 11. Runtime renders (`--profile`, v2 item 3)

Status: built (`internal/react/profile*.go`). Opt-in because it runs the test suite once more
(after the coverage run, if any, so coverage instrumentation does not inflate the timings) and its
durations change from run to run: without `--profile` the report and `react.edn` are byte-identical
to v1.

### Mechanism: the React DevTools global hook

`profile_hook.js` (embedded in the binary as text) defines `globalThis.__REACT_DEVTOOLS_GLOBAL_HOOK__`
before react-dom loads. react-dom (16.5+ dev or profiling build) calls `hook.inject(renderer)` once and
`hook.onCommitFiberRoot(id, root)` after every commit. The hook walks the committed tree the way the
React DevTools Profiler does:

- a root whose previous `current` had no element is a mount: every fiber below is new;
- otherwise from the root down: a fiber whose child list is the previous one (`next.child ===
  prev.child`) did not re-render anything below it and is skipped; a child with an `alternate` is an
  update, one without is a mount;
- a fiber **rendered** in the commit when it is a function, class, `forwardRef` or simple-`memo`
  fiber (tags 0, 1, 11, 15; a `memo` wrapper fiber, 14, is skipped: its inner fiber is counted) and
  carries `PerformedWork` (`flags & 1`; `effectTag` before React 17);
- per component name (`displayName`, else the function name; `forwardRef`'s render function) it
  counts renders and mounts and sums React's `actualDuration` (time spent rendering the fiber and the
  descendants that rendered with it in that commit), keeping the max.

**Why not `<Profiler>`.** It is the public API, but it measures a subtree, not a component: one
number per component needs a `<Profiler>` around every element (rewriting the JSX runtime via module
aliases, which changes element types that user code and tests compare), and its `onRender` also fires
for every ancestor of a component that re-rendered by itself, so a parent would be charged renders
it never did (exactly the case `Toggle` in the fixture checks). The hook relies on fiber fields that
are not a public API (`tag`, `flags`, `alternate`, `child`, `actualDuration`), but they are the ones
React DevTools, react-scan and similar tools read, unchanged since React 17; it changes nothing in
the rendered tree and works with any JSX transform, class components and both runners. React 19's
Performance Tracks (`console.timeStamp`) were rejected too: 19.2+ only, and a logging format, not an
API.

### Running the tests without editing the project

Packages are the `package.json` dirs above the analyzed files (as for coverage). Per package, Vitest
first (the `test` script mentions `vitest`, or a `vitest.config.*` exists), then Jest (the script,
a `jest.config.*`, or a `jest` key). Generated files go to `.metrics/react-profile[/<pkg slug>]/`:

| runner | generated | command |
|---|---|---|
| Vitest | `setup.vitest.mjs` (hook; `afterAll` flush), `vitest.config.mjs`: imports the user's config (`--config` in the test script, else `vitest.config.*`, else `vite.config.*`; object, promise or function) and returns it with the setup file **first** in `test.setupFiles`, so the hook exists before a user setup file imports react-dom | `vitest run --config <generated> --root <pkg> --coverage.enabled=false` |
| Jest | `setup.jest.cjs` (hook), `setup-after-env.jest.cjs` (`afterAll` flush). Jest CLI lists replace the config's, so `jest --showConfig` supplies the user's resolved `setupFiles` / `setupFilesAfterEnv`, passed back with ours first / last | `jest --ci --coverage=false --setupFiles <ours> <theirs>... --setupFilesAfterEnv <theirs>... <ours>` |

Each test file's flush writes `profile-<pid>-<time>-<random>.json` (old ones are deleted first); Go
merges them (sums, max of maxima). The test output goes to stderr (stdout is the report).

One-line failures (exit 1, `react: --profile: ...`, nothing written): no Vitest or Jest in a package;
Jest with several projects; React never connected to the hook (no react-dom render in the tests, or
react-dom loaded before the setup file); a production build (no `actualDuration`: run with
`NODE_ENV=test`); the hook threw. A failing profiled run exits 2 (`react: the profiled test run
failed; fix the failing tests or drop --profile`), like the coverage run. Not supported: Vitest
`test.projects` / workspaces (root `setupFiles` do not reach projects), mocha, Jest projects.

### Join, flag, output

React reports names, not files: a runtime name joins the component unit whose last name segment
matches (`TodoList.TodoRow` renders as `TodoRow`). A name no unit has falls back to the name without
trailing digits, because esbuild renames `const Label = memo(function Label() {})`'s inner function
to `Label2`. A name shared by several units gets one unjoined row (`(N components)`); names of no
unit (library components, Testing Library's `TestComponent`) are dropped; anonymous components
(`memo(() => ...)`, `forwardRef((p, r) => ...)` without `displayName`) cannot be joined. Hooks have
no runtime rows.

A row is **flagged** (`re-renders with inline props`) when updates >= 3 x mounts (updates when
nothing mounted) and the unit passes inline function or object/array props (`InFn + InObj > 0`).
Updates per mount, not raw updates, so a component mounted in many tests or list rows is not called
hot for being rendered often. Flags are not threshold violations: exit codes stay those of the static
checks.

Report: a `Runtime renders (<runners> test run; ...)` table after the violations: Component,
Namespace, Renders, Mounts, Updates, Upd/Mount, TotalMs, MaxMs, Inline, Flag, sorted by updates,
renders, name; then `Not rendered by the tests: ...` (component units with no runtime row). EDN: a
last key `:renders {:runners [...], :not-rendered [...], :components [{:name :namespace :path
:renders :mounts :updates :total-ms :max-ms :inline-props :ambiguous :flagged}]}` (`:namespace` /
`:path` nil when ambiguous).

### What the numbers mean

They count what the **tests** made React do in a **development** build: each `render()` mounts, each
event or state change the test fires commits. More tests mean more renders, so compare updates per
mount and the flags, not raw counts across projects. Counts are exact and repeatable (React's commit
behaviour is deterministic; `StrictMode` double rendering is one commit, counted once). Durations are
wall-clock: the first mount in a test file includes module and JIT warm-up, a loaded machine stretches
everything, dev builds are several times slower than production, and `actualDuration` includes the
descendants that rendered with the component (a parent's total covers its children's). Use them to
rank within one run, never as production timings.

### Fixture and tests

`fixtures/react` gains `Clicker` (state; an inline handler and an inline style object passed to
memo `Label`) and memo `Toggle` (own state, no props), tested by clicking add three times and the
toggle once: Clicker 4 renders / 1 mount / 3 updates (flagged), Label 4/1/3 (memo defeated by the
inline props, not flagged: none of its own), Toggle 2/1/1 (Clicker's renders are not charged to
it). `TestReactProfileOnTheFixture` asserts every runtime row's counts exactly (durations only as
numbers), the `:renders` EDN, the generated config, and exit 2 on a failing test;
`internal/react/profile_test.go` covers merging, joining, the flag threshold, the generated files,
the Jest arguments and the table/EDN text. The Jest path was checked by hand on a scratch Jest 30 +
jsdom project with the same components (same counts).

## 12. Duplicate JSX (`dry --jsx`)

Status: built (`internal/dry/jsx.go`, React v2 item 2). dryer pairs whole functions, so two
components that repeat a card, a form row or a list item inside different bodies go unseen. A
repeated element tree is the "extract a component" signal.

### Surface: a hidden `dry --jsx` flag, not a `react` section

- Duplication, the threshold and the Jaccard score are dry's concepts. The engine (`Fingerprinter`,
  `groupHits`: interned ids, size pruning, parallel rows) is package-internal to `internal/dry`, and
  `--jsx` reuses it as is. A `react` section would have to export or fork it.
- `react` runs the coverage runner by default and exits 3 on a violation. Duplicate markup needs no
  test run and is not a gate, like dry, which always exits 0.
- Parity: the flags are hidden (dryer `--help` unchanged). The function report and `.metrics/dry.edn`
  are byte-identical with or without `--jsx`. The pairs get their own section and
  `.metrics/dry-jsx.edn`.

Flags (each one implies `--jsx`): `--jsx-threshold N` (0.82), `--jsx-min-elements N` (4),
`--jsx-min-nodes N` (40). dry's `--threshold`, `--min-lines` and `--min-nodes` still apply to
functions only. `--tests`, `--changed`, paths and filters work as in dry.

### Files and candidates

`--jsx` widens dry's selection walk to `.jsx` and `.js` (dry has no JavaScript), with crap's
test-file rule. `ScanFiles` skips the files dryer has no language for, so the function report does
not change. Every `jsx_element` / `jsx_self_closing_element` in a `.tsx`/`.jsx`/`.js` file is a
candidate, fragments included. That covers elements nested in other elements, in `{...}` children
(`.map`, `&&`, ternaries) and in attribute values. A file the parser rejects contributes nothing (A2).
All files form one pool, so a `.jsx` copy pairs with its `.tsx` original. The javascript grammar has
no `required_parameter`, so an arrow inside JSX normalizes one node differently in `.jsx` and `.tsx`.

### Normalization

dryer's normalizer (§4 of spec-dryer) with one hook: the JSX state takes over element nodes at any
depth, and the rest of the expression follows dryer's rules.

| construct | normalized |
|---|---|
| element `<Tag ...>children</Tag>`, `<Tag ... />` | `[:element [:tag "Tag"] attr... child...]`. Self-closing and empty paired tags are equal. Member/namespace tags keep their text (`Sidebar.Tab`, `svg:rect`) |
| fragment `<>...</>` | `[:fragment child...]` (not counted as an element) |
| attribute | `[:attr "name"]`. The value is dropped (string, `className`, `{expression}`): values become props when the markup is extracted. Exception: a value holding JSX (`icon={<Icon/>}`, render props) is normalized like a child |
| `key` | dropped: reconciliation, not markup. A list item inside `.map` has it, the same markup outside does not |
| `{...props}` | dryer's form of the expression (`[:jsx_expression [:spread_element :symbol]]`) |
| child `{expression}` | dryer's form: locals become `:symbol`, literals `:literal`, callees keep their names (`map`), JSX inside is normalized as above. A `{}` holding only comments is dropped |
| text, `&amp;` | dropped |

Size: **elements** counts the subtree's elements, itself included. **Nodes** is dryer's
`node_count` of the normalized tree. A candidate needs elements >= `--jsx-min-elements`, nodes >=
`--jsx-min-nodes`, and at least one element that is not an SVG drawing element (`svg g path circle
rect ... fe*`). An icon's content is its path data, an attribute value that is dropped, so any two
icons with as many paths look identical.

### Pairing and nested pairs

Fingerprints and scores are dryer's (the set of printed subtrees, Jaccard). One `Fingerprinter`
serves the pool, and `groupHits` scores the pairs. Then:

1. A pair where one side holds the other (same file, byte range inside) is dropped: an element and
   its own child are not duplicates.
2. A pair is dropped when another remaining pair dominates it. Each side of the other pair must be
   the same candidate as, an ancestor of, or a descendant of this pair's side, and either (a) it
   strictly encloses this pair on both sides (the largest enclosing duplicate wins whatever the
   scores), or (b) it has a higher score, or the same score and more nodes. Rule (b) covers a shared
   side, or one side larger and one smaller: `(li, li')` at 1.0 beats `(li, <aside>{x && li'}</aside>)`.
   Two sibling rows inside one parent pair with each other, not with the parent.

Order: score descending, then left file/line, right file/line, then document order. Left is the
earlier candidate (file order, then start offset).

Rule (a) holds at any threshold. At a low `--jsx-threshold`, a weak outer pair can hide a perfect
inner pair. Rows whose spans have the same file and lines on both sides are never paired:
`groupHits` skips equal spans, as dryer does. For JSX that means two equal elements on the same
line(s).

### Output

```
Duplicate JSX
=============

DUPLICATE score=1.00
  src/ProfileCard.tsx:6-17 <section> 7 elements
  src/TeamCard.tsx:5-16 <section> 7 elements
```

The section follows the function report after a blank line. It reads `No duplicate JSX candidates
found.` when there are no pairs. `.metrics/dry-jsx.edn` (and, under `--edn`, a second map on stdout
after dry's):

```clojure
{:jsx-candidates [
 {:score 1.0
  :left {:file "src/ProfileCard.tsx", :start-line 6, :end-line 17, :tag "section", :elements 7, :nodes 68}
  :right {:file "src/TeamCard.tsx", :start-line 5, :end-line 16, :tag "section", :elements 7, :nodes 68}}
]}
```

`:tag` is `""` for a fragment (`<>` in the text report). stderr adds `Wrote <root>/.metrics/dry-jsx.edn`
after dry's line.

### Calibration (excalidraw `dd2f8fc`, whole repo)

259 JSX files hold 2,796 elements. 286 pass the defaults, 24 pairs are reported. The JSX pass
takes 1.1 s to parse and normalize and 7 ms to pair. The whole `dry --jsx` run took 2.8-6.5 s
against 1.1-3.4 s for plain `dry`, measured at load average ~120, so the absolute times are loose.
Changes made while calibrating:

- Attribute values were first kept as dryer expressions. The fixture's ProfileCard/TeamCard cards
  (same markup, different handler names) then scored 0.70: one differing callee changes every
  enclosing subtree's fingerprint. Dropping values (JSX-holding values excepted) gives 1.00.
- `key` kept a `.map` item and its copy outside a list under 0.82, so it is now dropped.
- SVG-only subtrees: 742 of 771 pairs were icons in `components/icons.tsx` (`<g>` / fragments of
  `<path>`s), so they are now excluded. 771 pairs became 24.
- Looser settings were tried and are not the default. `--jsx-min-elements 3 --jsx-min-nodes 30`
  gives 43 pairs (adds 3-element `StatsRow`s, `Trans` blocks, `PublishLibrary` fields: real but small).
  `--jsx-threshold 0.75` gives 33 (adds `SearchMenu` result rows at 0.80 and the export `Card`s of
  `ExportToExcalidrawPlus` / `JSONExportDialog` at 0.76).

Top pairs at the defaults:

| score | left | right |
|---|---|---|
| 1.00 | `dev-docs/src/components/Homepage/index.js:39-47` `<div>` 6 | `dev-docs/src/components/Homepage/index.tsx:47-55` `<div>` 6 |
| 1.00 | `packages/excalidraw/actions/actionProperties.tsx:734-770` `<fieldset>` 4 | `…:791-824` `<fieldset>` 4 |
| 1.00 | `…actionProperties.tsx:734-770` | `…:883-904` |
| 1.00 | `…actionProperties.tsx:734-770` | `…:927-960` |
| 1.00 | `…actionProperties.tsx:734-770` | `…:1028-1097` |

The rest: the same 7 fieldsets (stroke width, sloppiness, ...: `fieldset > legend + div.buttonList >
RadioSelection`, 21 pairs in all), two `PublishLibrary` labels (1.00) and two `AppSidebar`
`Sidebar.Tab`s (0.93). Every reported pair was a real repeated block on inspection.

### Tests

`internal/dry/jsx_test.go` pins the normalized forms and counts, the exact reported pairs (nested,
sibling, lopsided and below-threshold cases), a hand-computed 7/9 score, SVG exclusion, the hidden
flags and their usage errors, file selection, and the run output with an unchanged `dry.edn`.
`cmd/coderefinery/dry_jsx_e2e_test.go` runs `dry --jsx` on `fixtures/react-jsx` (a small fixture
with no dependencies).

### Follow-ups

- Cluster the pairs: 7 copies are now 21 pairs. A "same markup at N sites" group would read better.
- A `Duplicate JSX` count per component in `coderefinery react` (needs the engine exported).
- `.tsx` files are parsed twice (once for functions, once for JSX). Sharing the tree would save
  about the parse share of the 1.1 s above.
