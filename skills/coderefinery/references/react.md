# coderefinery react

Open this before acting on `coderefinery react` output, `dry --jsx` pairs or
`mutate --react` lines. All three are coderefinery additions with no upstream
counterpart; without them crap, dry and mutate behave exactly as upstream.

## Contents

- Command and flags
- Units and entries
- Reading the three tables
- Thresholds and per-violation actions
- Exit codes and `.metrics/react.edn`
- Runtime renders (`react --profile`)
- Duplicate JSX (`dry --jsx`)
- React mutation operators (`mutate --react`)

## Command and flags

```
coderefinery react [--root <path>] [--changed] [--no-coverage | --use-existing-coverage |
                   --coverage-command <cmd>] [--crap <n>] [--profile]
                   [--max-<metric> <n|off>]... [path-or-filter ...]
```

- Files: crap's selection (paths, filters, `--changed`, test files and skipped
  directories excluded), kept to `.ts .tsx .mts .cts .js .jsx .mjs .cjs`.
- Coverage: by default the TypeScript coverage runner runs first, as in crap
  (`npm run coverage`, Vitest LCOV or c8; see `typescript-javascript.md`).
  After crap, pass `--use-existing-coverage` to read the LCOV it left. With
  `--no-coverage`, CRAP is N/A and an entry offends at CC ≥ the CRAP limit (CC
  is CRAP's floor, so it could never pass anyway).
- Everything else is static, from the syntax tree, except `--profile` (below).

## Units and entries

- **Component**: a module-level `function Name()` or `const Name = () => ...`
  with a PascalCase name (or wrapped in `memo`/`forwardRef`, any nesting) whose
  own `return` or expression body holds JSX. **Hook**: the same forms named
  `use[A-Z0-9]...`. A component defined inside another is its own unit,
  `Outer.Inner`.
- Not units: class components (crap scores their methods), anonymous default
  exports, factory-built components, object-literal members.
- **Entries** (CRAP rows): the unit body (`component`/`hook`); each function
  passed to a `use*` call (`callback`, `Cart#useEffect#1`, numbered per hook
  name); each local handler `const f = () => {}` / `function f() {}` in the unit
  body and each inline JSX attribute function (`handler`, `Cart.onSubmit`,
  repeats `#2`). Their decisions are not charged to the unit. `.map`, `.filter`
  and `setTimeout` callbacks stay in their enclosing entry.
- Complexity uses crap's TypeScript decisions (if, loops, catch, case, ternary,
  `&&`, `||`, `??`, `?.`). A unit body's coverage span still includes its
  extracted entries' lines.

## Reading the three tables

1. `React Report`: Entry, Kind, Namespace, Line, CC, Cov%, CRAP; worst first.
2. `Components and hooks`: Render (1 + decisions in what it renders: returned
   JSX, early-return conditions, one per `.map` inside JSX), Logic (1 + the
   other body decisions), Props, Bool, Opt, On (callback props), Rest, Pass
   (pass-through), State, Eff, Deps (largest dependency array), SetFx
   (setter-only effects), Depth (JSX nesting), Elems, InFn, InObj (inline
   function / object props), Nest (nested components). Props columns are `-`
   for hooks. Worst first. `9+` in Props means the props are **open**: part
   of the props type could not be read, so every props count of that unit is
   a lower bound (see below).
3. `Threshold violations` (absent when clean): name, namespace and
   `props 9 > 7`, `crap 27.1 >= 10.0`, `complexity 10 >= 10.0 (no coverage)`.

stderr: `Wrote <root>/.metrics/react.edn`, then
`React thresholds exceeded: N violation(s)`.

## Thresholds and per-violation actions

An entry offends at CRAP ≥ 10 (`--crap <n>`). A unit offends when a metric is
above its limit; every limit is `--max-<metric> <n|off>`. Keep the defaults
unless the project agreed on others.

| violation (flag) | default | do |
|---|---|---|
| `crap` / `complexity` on an entry (`--crap`) | ≥ 10 | Same as crap: split the callback or handler, or test it. Move effect logic into a pure function and test it directly. |
| `props` (`--max-props`) | > 7 | Split the component, or group related props into one object, or pass children instead of data. |
| `boolean-props` (`--max-bool-props`) | > 2 | Replace flag combinations with one variant prop, or separate components. |
| `pass-through` (`--max-pass-through`) | > 3 | Prop drilling: pass the child element (composition), move the state down to its user, or use context. |
| `state` (`--max-state`) | > 5 | Derive values instead of storing them; group state that changes together (`useReducer`). |
| `effects` (`--max-effects`) | > 3 | Move event-driven work into handlers; extract a custom hook. |
| `deps` (`--max-deps`) | > 6 | The effect or memo does too much; split it. |
| `setter-effects` (`--max-setter-effects`) | > 0 | Derived state: compute the value during render (or `useMemo`) instead of copying it into state in an effect; reset with a `key`. |
| `jsx-depth` (`--max-jsx-depth`) | > 6 | Extract subtrees into components. |
| `nested-components` (`--max-nested`) | > 0 | Move the inner component to module level; it is re-created, so remounted, on every render. |
| `elements`, `inline-functions`, `inline-objects` (`--max-elements`, `--max-inline-functions`, `--max-inline-objects`) | off | Reported only. Inline handlers and objects matter only for memoised children, which a static pass cannot see. |

## Exit codes and `.metrics/react.edn`

| exit | meaning |
|---|---|
| 0 | no violations (also no files or no units) |
| 1 | usage or internal error (`react: <message>`); a git failure under `--changed` exits with git's code |
| 2 | the coverage run, or the `--profile` test run, failed (failing tests). Nothing is written. Fix the tests, or use `--no-coverage` / drop `--profile` |
| 3 | at least one violation |

`.metrics/react.edn` holds `:entries` (`:name :namespace :kind :unit :path :line
:complexity :coverage :crap`), `:components` (`:render :logic`, `:props {:count
:boolean :optional :callback :rest :pass-through :open}`, `:hooks {:state :effects
:max-deps :setter-effects}`, `:render-cost {:jsx-depth :elements
:inline-functions :inline-objects :nested-components}`; `:props nil` for hooks)
and `:violations` (`:name :namespace :path :line :metric :value :limit`).
With `--profile`, `:renders` is added (`:runners`, `:not-rendered`,
`:components` with `:renders :mounts :updates :total-ms :max-ms :inline-props
:flagged`).

Props types are followed across the project's modules (imports, re-exports,
index files, tsconfig `paths` and `baseUrl`; never `node_modules`) and through
`extends`, `&`, `|`, `Omit`, `Pick`, `Partial`, `Required`, `Readonly`,
`PropsWithChildren` and generic defaults. Anything else makes the props open
(`9+`, `:open true`): React's DOM attribute types and other library types,
`any`/`unknown`, index signatures, mapped and conditional types, `typeof`,
indexed access, cycles. Open never hides or invents a violation: a known count
above its limit is real, and an open unit under the limit does not offend.
Known limits: a unit body's coverage still includes its extracted entries'
lines; pass-through does not track shadowing.

## Runtime renders (`react --profile`)

`--profile` runs the Vitest (else Jest) tests once more with a render profiler
(the React DevTools hook, installed by a generated setup file under
`.metrics/react-profile/`; the project is not edited) and appends (columns
shortened):

```
Runtime renders (vitest test run; durations are React actualDuration in ms, noisy)

Component   Namespace   Renders Mounts Updates Upd/Mount TotalMs MaxMs Inline Flag
Clicker     Clicker           4      1       3       3.0    12.3  10.2      3 re-renders with inline props

Not rendered by the tests: IconButton
```

- Counts are what the tests made React do in a development build: exact and
  repeatable. Compare updates per mount, not raw counts. Durations include the
  descendants rendered with the component and vary with load: rank within one
  run only, never read them as production timings.
- `re-renders with inline props` (updates >= 3x mounts and inline function or
  object props): memoize those props (`useCallback`, `useMemo`, a module
  constant) or move the state that changes down to the component that uses it.
- `Not rendered by the tests`: components no test renders. Write a test that
  renders them before trusting their numbers.
- Needs Vitest or Jest; otherwise exit 1 (`react: --profile: no Vitest or Jest
  test setup in ...`). A failing test exits 2.

## Duplicate JSX (`dry --jsx`)

`coderefinery dry --jsx --threshold 0.8` also pairs JSX element subtrees in
`.tsx`, `.jsx` and `.js` files, inside and across components. Tags and
attribute names count; text, attribute values, `key` and SVG-only subtrees
(icons) do not. The largest enclosing pair is reported. After the unchanged
function report:

```
Duplicate JSX
=============

DUPLICATE score=1.00
  src/ProfileCard.tsx:6-17 <section> 7 elements
  src/TeamCard.tsx:5-16 <section> 7 elements
```

or `No duplicate JSX candidates found.` The pairs go to `.metrics/dry-jsx.edn`
(`{:jsx-candidates [{:score :left {:file :start-line :end-line :tag :elements
:nodes} :right {...}}]}`); `dry.edn` does not change. Extract each pair into one
component whose props carry what differs. Tuning: `--jsx-threshold` (default
0.82), `--jsx-min-elements` (4), `--jsx-min-nodes` (40); each implies `--jsx`.

## React mutation operators (`mutate --react`)

`coderefinery mutate --react --fix-overlay --changed --use-existing-coverage` adds React sites
to the usual ones (TS/TSX/JS files; the dependency operator also in `.ts` hooks):

| operator (site label) | mutation | a survivor means |
|---|---|---|
| `drop render <X/>` / `always render <X/>` | `cond && <X/>` → `false` / `<X/>` | no test checks when `X` shows and when it does not |
| `swap ternary <A/> : <B/>` | `c ? A : B` → `c ? B : A` | no test tells the two branches apart |
| `drop prop <name> from <X/>` | one JSX attribute deleted (not `key`, `className`, `style`, handlers, spreads) | no test sees that prop's effect |
| `noop onClick` | `onX={f}` → `onX={() => {}}` | no test fires the event and checks the result |
| `drop dep <dep> from useEffect` | one entry of a React hook dependency array deleted | no test changes that dependency and checks the effect re-ran |

Kill survivors with tests that render the component and check what the user
sees (Testing Library). Sites keep crapper's form ids, so snapshots,
differential reruns, `--attribute` and `tests` work as usual. On a Vitest
project with `environment: "jsdom"` add `--fix-overlay` (`--attribute` turns it
on), or every mutant is reported KILLED (see `typescript-javascript.md`).
