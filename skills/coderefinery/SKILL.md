---
name: coderefinery
description: >
  Follow the reduced principles when writing or changing code. Every change
  follows First Make it Work, then Then Make it Right, then Follow Practice.
  Use when the user runs /coderefinery or asks for the reduced principles. Ships a
  launcher for coderefinery, the CRAP, duplication and mutation quality gates.
  It also runs the tests, reports which tests kill mutants and which add nothing,
  and scores React components and hooks.
license: MIT. Adapted from ub-mode by Robert C. Martin; see LICENSE.
metadata:
  upstream: https://github.com/unclebob/ub-mode
---

# coderefinery

Every time you are asked to make a change follow:
* First Make it Work.
* Then Make it Right.
* Follow Practice

# First Make it Work

## Laziness Protocol

* One decision lives in one place.
* Ship the smallest edit that solves the problem.

## Foundational Thinking

* Get the data shape right before you write the logic.
* Do first whatever helps every later phase.
* If a concurrent writer would matter, do not share state.

## TDD

* Write the failing test before you change the code.
* See it fail for the reason you expect.
* Make the smallest change that makes it pass.
* If an honest test is impractical, do not write a bad one.

## Redesign From First Principles

* Do not bolt a new requirement onto the old design.
* Ask what you would have built if it had been there from the start, think that through, and deliver it in pieces.

## Attack the Premise

* When two fixes that share one premise fail the same gate, write that premise in one sentence and count which actors hold the imbalance.
* Remove the assignment.
* Do not start another fix until you have done that.

## Subtract Before You Add

* Take complexity out before you add anything.

## Outcome-Oriented Execution

* Aim at the end state you can check.
* A break in the middle is allowed when you planned it and can undo it.

## Build the Lever

* If the work is not obvious at a glance, build the tool that does it.
* Do one unit by hand, then make the tool match that unit.

## Architecture

### Model the Domain

* Put the domain in a structure that makes the illegal states impossible.
* Maintain a glossary of domain terms.
* If the code is already clear and local, leave it.

### Boundary Discipline

* Check the data where it enters.
* Inside, trust it.
* Keep the business logic in a pure function.

### Type System Discipline

* Make the illegal value impossible to write.
* Parse outside data once, where it comes in.
* Do not silence the compiler.

### Make Operations Idempotent

* Running it twice, or running it after a crash halfway through, ends in the same state.

### Separate Before Serializing Shared State

* If they do not need the same mutable thing, give each its own.
* If they do, allow one writer, and enforce that in the structure.

# Then Make it Right

## Fix Root Causes

* Reproduce it.
* Fix the cause.
* A guard that only hides the crash is not a fix.

## Sequence work into verifiable units

* Finish one small unit and check it.
* Then start the next.

## Test Behavior, Not Implementation

* The test calls the code the way its user does and checks the result that user would see.
* If the test would still pass when every function returned nothing, throw it out.

## Quality Gates

* Run the gates with the bundled launcher `scripts/coderefinery`. In Claude Code that is `"${CLAUDE_SKILL_DIR}/scripts/coderefinery"`; in other agents, `scripts/coderefinery` inside this skill's directory. Below, `coderefinery` means that launcher.
* The launcher uses `$CODEREFINERY_BIN`, else `coderefinery` on `PATH`, else a cached download. The first run may fetch the pinned release (SHA256-checked) or fall back to `go install`. Exit 127 means no binary could be found or installed; its message says what to do, and `references/install.md` explains each message.
* Run from the project root, in this order: crap, then dry, then mutate. Re-run a gate after every fix until it is clean.
* In a project with React components (`.tsx`, `.jsx`), run react after dry.
* To judge the tests too, extend that order: crap (optionally `--branches`), then dry (then `dry --tests`), then react for TSX/JSX projects, then `coderefinery test`, then `mutate --attribute`, then `tests`. In Claude Code the first step is `"${CLAUDE_SKILL_DIR}/scripts/coderefinery" crap --changed --threshold 9.9999 --branches`.
* `--attribute`, `--fix-go-coverage`, `--fix-overlay`, `--min-timeout` and `--react` (mutate), `--branches` (crap, mutate), `--tests` and `--jsx` with `--jsx-threshold`, `--jsx-min-elements`, `--jsx-min-nodes` (dry) are coderefinery additions. They work, but they are not in the upstream `--help` text, which stays unchanged. `test`, `tests` and `react` are listed in `coderefinery --help`, and their flags in their own `--help`. Without these flags and commands, crap, dry and mutate behave exactly as upstream.
* `--changed` limits a gate to added and modified files from `git status`. It needs a git repository; when git fails, the gate exits with git's code (128 outside a repository). With nothing changed it prints `No source files to analyze.` (crap, dry) or `No source files to mutate.` and exits 0.
* Each gate writes `.metrics/` at the root (`--root <path>` to change it). The files are EDN for uml-viewer.

### Reference files

Paths are relative to this skill's directory (`${CLAUDE_SKILL_DIR}` in Claude Code). Open a file when its trigger applies; do not load them all.

* `references/python.md`, `references/typescript-javascript.md`, `references/go.md`, `references/java.md`, `references/rust.md`, `references/clojure.md`: the first time you run the gates in a project, for each language it contains; and whenever coverage is `N/A` or 0%, a coverage or test tool is missing, or a result looks wrong. Each covers the commands coderefinery runs, coverage tools and install hints, branch granularity, flags such as `--fix-go-coverage`, and what is approximate.
* `references/testing.md`: before running `test`, `mutate --attribute`, `tests` or `dry --tests`, before deleting or merging a test, to read `--branches` lines, and when mutants time out or are killed `· by <build>` for no visible reason.
* `references/react.md`: before acting on `coderefinery react` output (`--profile` included), `dry --jsx` pairs or `mutate --react` lines.
* `references/install.md`: the launcher exits 127, the version is not the expected one, or a gate finds no files.

## Crapper

* Run `coderefinery crap --changed --threshold 9.9999`. It runs the project's tests with coverage, then scores every function: CRAP = CC² × (1 − coverage)³ + CC.
* Reduce each function under CRAP 10 by splitting it, by testing it, or both. The threshold trips only above its value and scores have 4 decimals, so 9.9999 fails exactly the functions at 10 or more.
* Consider whether case statements are worth reducing.
* Read: the report on stdout, worst first (Function, Namespace, CC, Cov%, CRAP), after the test runner's output. `.metrics/crap.edn` holds `{:entries [{:name :namespace :complexity :coverage :crap} ...]}`. CC 10 or more can never pass by testing alone (at full coverage CRAP equals CC): split the function. Below that, testing it may be enough.
* Exit 0: every function is under 10. Exit 2: stderr ends with `CRAP threshold exceeded: <worst> > 10.0`; fix the rows at the top of the report and re-run. Exit 1: usage or internal error; read stderr.
* `N/A` coverage means the coverage report has no data for that file. A stderr warning such as `Python tests exited 1 ...` or `Go coverage exited 1 ...` means the tests or the coverage run failed. Fix either before trusting the gate; the language reference says how. `--coverage-command <cmd>` runs your own coverage command, `--use-existing-coverage` reads reports already on disk, and `--no-coverage` scores complexity only (CRAP is N/A, so the threshold cannot trip).
* `coderefinery crap --changed --threshold 9.9999 --branches` adds an `Untested branches (<language> <granularity>, ...)` section after the report and writes `.metrics/branches.edn`. The report, `crap.edn` and the exit codes do not change. Each line is `path:line <decision> · <what was missed>`, only for decisions that ran.
* For each line, write the test that takes that arm. If no honest input can reach the arm, delete the dead arm. How precise a line is depends on its granularity (`branch`, `branch-count`, `block`, `region`, `form`); see `references/testing.md`. `No branch data: <language> (run coderefinery test)` means run `coderefinery test` first; Python always needs it. `--no-coverage --branches` is a usage error (exit 1).

## Dryer

* Run `coderefinery dry --threshold 0.8`. The default threshold is 0.82, so always pass 0.8.
* Eliminate every duplication that ranks 0.8 or above.
* Read: one block per pair, `DUPLICATE score=0.91` followed by the two spans `path:start-end`. `.metrics/dry.edn` holds `{:candidates [{:score :language :left {:file :start-line :end-line} :right {...} ...}]}`; `No duplicate candidates found.` and an empty `:candidates` vector mean no duplication.
* Exit 0 whenever the report was written, duplicates or not, so read the report. Exit 1 is a usage error.
* `--changed` compares changed files only with each other, so it misses a new copy of unchanged code. Prefer a full run, or name the directories to compare. Candidates are functions and methods with at least 4 lines and 20 normalized nodes (`--min-lines`, `--min-nodes`). Plain JavaScript (`.js`, `.mjs`, `.cjs`, `.jsx`) is not compared; TypeScript is.
* In a project with React components (`.tsx`, `.jsx`), also run `coderefinery dry --jsx --threshold 0.8`. It adds a `Duplicate JSX` section after the unchanged function report: pairs of repeated JSX element subtrees (tags and attribute names; text and attribute values ignored), written to `.metrics/dry-jsx.edn`. Extract each pair into a component. `--jsx-threshold` (default 0.82), `--jsx-min-elements` (4) and `--jsx-min-nodes` (40) tune it; each implies `--jsx`. Details: `references/react.md`.
* Then run `coderefinery dry --tests --threshold 0.8`. It compares test files too; the output, `.metrics/dry.edn` and the exit codes are the same as plain dry. A pair inside the tests is a copied test. Parametrize it (one test over a table of cases), or extract the shared setup into a fixture or helper.

## React

* In TSX/JSX projects run `coderefinery react --changed --use-existing-coverage` after crap (which left the coverage on disk). It scores function components and custom hooks; each hook callback (`Cart#useEffect#1`) and handler (`Cart.onSubmit`) is its own CRAP entry, not charged to the component.
* Read: three tables. `React Report` (CRAP per entry, worst first), `Components and hooks` (render vs logic complexity, props, hooks, render cost per unit), and `Threshold violations`, one line per violation such as `props 9 > 7` or `crap 27.1 >= 10.0`. `.metrics/react.edn` holds the same.
* Props shown as `2+` are open (`:open true` in `.metrics/react.edn`): part of the props type could not be read (React's DOM attributes, a library type, a mapped or conditional type), so the counts are lower bounds. A count above its limit is still a real violation; an open count under the limit does not offend.
* `--profile` also runs the Vitest or Jest tests with a render profiler and appends `Runtime renders` (renders, mounts, updates, React actualDuration per component) and `Not rendered by the tests: ...`. A component flagged `re-renders with inline props` re-renders often and passes inline function or object props: memoize those props or move the state down. Counts are exact; durations are noisy test-run numbers, never production timings. The project is not edited; exit 2 also means the profiled run failed.
* Fix every violation line. An entry at CRAP 10 or more: split it or test it, as in crap. A unit metric over its limit: the action per metric is in `references/react.md`. Limits are `--crap <n>` and `--max-<metric> <n|off>`; keep the defaults unless the project agreed on others.
* Exit 0: no violations. Exit 3: at least one violation. Exit 2: the coverage run failed (failing tests); nothing was written, so fix the tests or use `--no-coverage`. Exit 1: usage or internal error.

## Mutator

* Run `coderefinery mutate --changed --use-existing-coverage` after the crap gate, which already left coverage reports on disk. Without `--use-existing-coverage` it runs the coverage tools again. `--scan` lists the mutation sites without running anything.
* Cover the uncovered: each `UNCOVERED path:line <mutation>` line is code no test executes. Write a test for it.
* Kill the survivors: each `SURVIVED path:line <mutation>` line is a change the tests did not notice. Add or sharpen the assertion that would fail.
* Delete dead code: a survivor that no honest test can kill marks code that does nothing. Remove it.
* Read: the site lines, then a table per function (Killed, Survived, Uncovered, Sites, Score). `.metrics/mutate/<namespace>.edn` keeps each outcome, so the next run re-runs only survivors and functions whose text changed (`--mutate-all` re-runs everything, `--lines <n,...>` targets lines).
* Exit 0: every executed mutant was killed, or there was nothing to run. Exit 3: at least one mutant survived. Exit 2: the baseline tests failed, so fix the tests first. Exit 1: usage or internal error.
* Go: if every site is UNCOVERED with `No coverage data for <file>`, the module path does not mirror the directories. Add `--fix-go-coverage`.
* To credit kills to tests, run `coderefinery mutate --changed --use-existing-coverage --attribute` after `coderefinery test`. KILLED lines end with ` · by <test id>`, plus ` (+N)` when more tests killed it, and the ids go into `:killed-by` in `.metrics/mutate/<namespace>.edn`. A kill no test can be credited with counts as `<timeout>` or `<build>`. Exit codes are the same; on exit 2, stderr adds `failing tests: <ids>`. A missing reporter or plugin exits 1 with one line that names what to install. Details: `references/testing.md`.
* In a project with React components, add `--react` (`coderefinery mutate --react --fix-overlay --changed --use-existing-coverage`). It adds React operators to the usual sites: `drop render`, `always render`, `swap ternary`, `drop prop`, `noop onClick`, `drop dep`. Kill each survivor with a test that checks what the user sees. Keep `--fix-overlay` on a Vitest project with `environment: "jsdom"` (or a Jest project): without it every mutant is reported KILLED under jsdom, and Jest can report killable mutants SURVIVED. Details: `references/react.md`.
* If a survivor turns into a timeout kill (`· by <timeout>`) only on a loaded machine, add `--min-timeout <seconds>` (for example 300). It raises the 2 s floor of each mutant's timeout and changes nothing else.
* Add `--fix-overlay` when the upstream worker copy of the project gives false results: every mutant in a package KILLED (`· by <build>` with `--attribute`) because it imports a package directory named `coverage`, `build`, `out`, `dist` or `target`; or, in TypeScript/JavaScript, every mutant KILLED under Vitest + jsdom, or survivors that Jest never tested. `--attribute` turns it on by itself. Details: `references/testing.md`, `references/typescript-javascript.md`.
* `--branches` gives each UNCOVERED line a reason: ` · in the true arm of :16 (if total <= 0), never taken`, ` · line never executed`, or ` · no coverage data for file`.

## Test Runner

* Run `coderefinery test` before `mutate --attribute` and `tests`. It runs each project's tests once, with a JUnit reporter (`go test -json` for Go) and branch coverage, and writes `.metrics/tests/`.
* Read: stderr ends with one line per project, such as `python .: pytest 12 passed, 0 failed, 1 skipped (branch coverage, contexts)`, then `Wrote <path>/runs.edn`.
* Exit 0: every test passed or was skipped. Exit 2: a test failed or errored, the build or collection failed, or the runner wrote no results; fix the tests first. Exit 1: usage error, or a missing tool or plugin. The one line names what to install; coderefinery never installs it.
* `--repeat <n>` marks a test whose status changes as flaky. `--changed` runs only the projects of changed files, test files included. `--test-command <cmd>` runs your own runner, `--no-coverage` skips coverage. Contract and per-language runners: `references/testing.md` and the language reference.

## Test Relevance

* Run `coderefinery tests` after `mutate --attribute`. It prints one row per test case: Kills (the mutants the test fails on), Unique (kills no other test has), Fns, Level (`unit`, `integration`, `api`, `e2e`; `unit~` means guessed from fan-out) and Shadowed by. It writes `.metrics/tests.edn`.
* Kills 0, shown as `(no kills)`: the test notices no change to the code. Delete the test.
* Unique 0 with tests under Shadowed by: read those tests. If they are at the same or a lower level, delete this test. If this test guards a seam (an integration boundary the lower tests stub out), keep it and say why in the commit message.
* Unique 0 with `-` under Shadowed by: the covering set needs it. It is not an offender; keep it.
* `Same function, different levels`: keep the lower-level test. Keep the higher one only if it has unique kills elsewhere. `keep both; ... push a <lower> test down` means the higher test kills mutants the lower tests miss there: write that lower-level test, then apply this rule again.
* `Flaky` section (from `test --repeat`): those tests are left out of the math. Fix them; do not delete them.
* `Unattributed kills: N <timeout>, M <build>` counts kills credited to no test. They are not offenders.
* Deleting every shadowed test keeps every killed mutant killed. Prove it: after deleting, re-run `coderefinery test`, then `mutate --attribute --mutate-all` on the files those tests covered, then `tests`.
* `--group` gives one row per parametrized test instead of one per case. `--changed` limits the rows to tests in changed test files and tests that kill a mutant in a changed source file; Unique and the covering set still use every test.
* Exit 0: no offenders. Exit 3: a test kills nothing or is shadowed. Exit 2: the latest test results have failures (`fix the tests first`). Exit 1: usage error, or `no attribution data: run coderefinery mutate --attribute`.

## Refactoring

* Pin what the code does.
* Change the structure.
* Prove the behavior did not change.

# Practice

## Keep a Log

* Keep a log of every action you take and the reason you took it.
* Report that log upon completion.

## Guard the Context Window

* Keep the main thread small.
* Send the bulk elsewhere, and keep the summary.

## Worktree Cleanup

* Find what is safe to remove, confirm nothing still in use needs it, then reclaim the disk.
* Uncommitted work waits for a decision.

## Commit and Push

* Commit often, in small ordered commits.
* Each commit says why, what changed, and how you proved it.
* Push to the main line at the end.

# Credits

Adapted from [ub-mode](https://github.com/unclebob/ub-mode) by Robert C. Martin (MIT, see `LICENSE`). The principles are his text, unchanged. The Crapper, Dryer and Mutator sections now run `coderefinery`, a Go port of his [crapper](https://github.com/unclebob/crapper), [dryer](https://github.com/unclebob/dryer) and [mutator](https://github.com/unclebob/mutator).
