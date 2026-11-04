# coderefinery react

Open this before acting on `coderefinery react` output. It is a coderefinery
addition with no upstream counterpart; it changes nothing in crap, dry or mutate.

## Contents

- Command and flags
- Units and entries
- Reading the three tables
- Thresholds and per-violation actions
- Exit codes and `.metrics/react.edn`

## Command and flags

```
coderefinery react [--root <path>] [--changed] [--no-coverage | --use-existing-coverage |
                   --coverage-command <cmd>] [--crap <n>] [--max-<metric> <n|off>]... [path-or-filter ...]
```

- Files: crap's selection (paths, filters, `--changed`, test files and skipped
  directories excluded), kept to `.ts .tsx .mts .cts .js .jsx .mjs .cjs`.
- Coverage: by default the TypeScript coverage runner runs first, as in crap
  (`npm run coverage`, Vitest LCOV or c8; see `typescript-javascript.md`).
  After crap, pass `--use-existing-coverage` to read the LCOV it left. With
  `--no-coverage`, CRAP is N/A and an entry offends at CC ≥ the CRAP limit (CC
  is CRAP's floor, so it could never pass anyway).
- Everything else is static, from the syntax tree.

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
   for hooks. Worst first.
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
| 2 | the coverage run failed (failing tests). Nothing is written. Fix the tests, or use `--no-coverage` |
| 3 | at least one violation |

`.metrics/react.edn` holds `:entries` (`:name :namespace :kind :unit :path :line
:complexity :coverage :crap`), `:components` (`:render :logic`, `:props {:count
:boolean :optional :callback :rest :pass-through}`, `:hooks {:state :effects
:max-deps :setter-effects}`, `:render-cost {:jsx-depth :elements
:inline-functions :inline-objects :nested-components}`; `:props nil` for hooks)
and `:violations` (`:name :namespace :path :line :metric :value :limit`).

Known limits: props from imported types, `Omit` and `Pick` are not resolved;
pass-through does not track shadowing.
