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
| props | count | first parameter: destructured names (`{a, b = 1, c: d, "e": f}`), plus the members of the props type when it is resolvable in the file: a type annotation on the parameter, `React.FC<P>` / `FC<P>` on the declarator, or the last type argument of `memo<P>` / `forwardRef<R, P>`; `P` is an interface (with its `extends`), a type alias, an inline object type, an intersection, or `PropsWithChildren<>` / `Readonly<>` of those. `Omit`, `Pick` and imported types are not resolved. A plain `props` parameter adds `props.x` reads and `const {x} = props` |
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
   :props {:count 4, :boolean 0, :optional 1, :callback 1, :rest false, :pass-through []},
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
(`memo(forwardRef<R, P>)`, untested); `useCounter` (a custom hook). `cmd/coderefinery/react_e2e_test.go`
checks the exact `--no-coverage` report and exit 3 (7 violations), the coverage run (exact Cov%/CRAP
rows, 8 violations; skipped without `node_modules` or under `-short`) and exit 2 on a failing
coverage command. `internal/react/react_test.go` covers each metric with exact values.

## 9. Follow-ups (v2, out of scope here)

- **React mutation operators** in `mutate` (opt-in, hidden flag, like `--attribute`): remove a
  conditional render (`cond && <X/>` → `false`), swap ternary branches in JSX, drop a prop from a
  child element, turn a handler into a no-op, remove one dependency from a dependency array.
- **dry over JSX subtrees**: duplicate element trees modulo literals and attribute values.
- **Runtime render cost** via `<Profiler>` (render counts and durations per component during the
  test run) to confirm what inline-functions / inline-objects only suggest statically.
- Props from imported types (cross-file resolution), `Omit`/`Pick`, a unit
  body's coverage without its extracted entries' lines, shadowing in pass-through.
