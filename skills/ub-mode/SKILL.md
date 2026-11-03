---
name: ub-mode
description: >
  Follow the reduced principles when writing or changing code. Every change
  follows First Make it Work, then Then Make it Right, then Follow Practice.
  Use when the user runs /ub-mode or asks for the reduced principles. Ships a
  launcher for ubgate, the CRAP, duplication and mutation quality gates.
license: MIT. Adapted from ub-mode by Robert C. Martin; see LICENSE.
metadata:
  upstream: https://github.com/unclebob/ub-mode
---

# ub-mode

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

* Run the gates with the bundled launcher `scripts/ubgate`. In Claude Code that is `"${CLAUDE_SKILL_DIR}/scripts/ubgate"`; in other agents, `scripts/ubgate` inside this skill's directory. Below, `ubgate` means that launcher.
* The launcher uses `$UBGATE_BIN`, else `ubgate` on `PATH`, else a cached download. The first run may fetch the pinned release (SHA256-checked) or fall back to `go install`. Exit 127 means no binary could be found or installed; its message says what to do.
* Run from the project root, in this order: crap, then dry, then mutate. Re-run a gate after every fix until it is clean.
* `--changed` limits a gate to added and modified files from `git status`. It needs a git repository; when git fails, the gate exits with git's code (128 outside a repository). With nothing changed it prints `No source files to analyze.` (crap, dry) or `No source files to mutate.` and exits 0.
* Each gate writes `.metrics/` at the root (`--root <path>` to change it). The files are EDN for uml-viewer.

## Crapper

* Run `ubgate crap --changed --threshold 9.9999`. It runs the project's tests with coverage, then scores every function: CRAP = CC² × (1 − coverage)³ + CC.
* Reduce each function under CRAP 10 by splitting it, by testing it, or both. The threshold trips only above its value and scores have 4 decimals, so 9.9999 fails exactly the functions at 10 or more.
* Consider whether case statements are worth reducing.
* Output: the report on stdout, worst first (Function, Namespace, CC, Cov%, CRAP), after the test runner's output. `.metrics/crap.edn` holds `{:entries [{:name :namespace :complexity :coverage :crap} ...]}`.
* Exit 0: every function is under 10. Exit 2: stderr ends with `CRAP threshold exceeded: <worst> > 10.0`; fix the rows at the top of the report and re-run. Exit 1: usage or internal error; read stderr.
* `N/A` coverage means the coverage report has no data for that file. A stderr warning such as `Python tests exited 1 ...` or `Go coverage exited 1 ...` means the tests or the coverage run failed. Fix either before trusting the gate. `--coverage-command <cmd>` runs your own coverage command, `--use-existing-coverage` reads reports already on disk, and `--no-coverage` scores complexity only (CRAP is N/A, so the threshold cannot trip).

## Dryer

* Run `ubgate dry --threshold 0.8`. The default threshold is 0.82, so always pass 0.8.
* Eliminate every duplication that ranks 0.8 or above.
* Output: one block per pair, `DUPLICATE score=0.91` followed by the two spans `path:start-end`. `.metrics/dry.edn` holds `{:candidates [{:score :language :left {:file :start-line :end-line} :right {...} ...}]}`; `No duplicate candidates found.` and an empty `:candidates` vector mean no duplication.
* Exit 0 whenever the report was written, duplicates or not, so read the report. Exit 1 is a usage error.
* `--changed` compares changed files only with each other, so it misses a new copy of unchanged code. Prefer a full run, or name the directories to compare. Candidates are functions and methods with at least 4 lines and 20 normalized nodes (`--min-lines`, `--min-nodes`). Plain JavaScript (`.js`, `.mjs`, `.cjs`, `.jsx`) is not compared; TypeScript is.

## Mutator

* Run `ubgate mutate --changed --use-existing-coverage` after the crap gate, which already left coverage reports on disk. Without `--use-existing-coverage` it runs the coverage tools again. `--scan` lists the mutation sites without running anything.
* Cover the uncovered: each `UNCOVERED path:line <mutation>` line is code no test executes. Write a test for it.
* Kill the survivors: each `SURVIVED path:line <mutation>` line is a change the tests did not notice. Add or sharpen the assertion that would fail.
* Delete dead code: a survivor that no honest test can kill marks code that does nothing. Remove it.
* Output: the site lines, then a table per function (Killed, Survived, Uncovered, Sites, Score). `.metrics/mutate/<namespace>.edn` keeps each outcome, so the next run re-runs only survivors and functions whose text changed (`--mutate-all` re-runs everything, `--lines <n,...>` targets lines).
* Exit 0: every executed mutant was killed, or there was nothing to run. Exit 3: at least one mutant survived. Exit 2: the baseline tests failed, so fix the tests first. Exit 1: usage or internal error.
* Go: if every site is UNCOVERED with `No coverage data for <file>`, the module path does not mirror the directories. Add `--fix-go-coverage`.

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

Adapted from [ub-mode](https://github.com/unclebob/ub-mode) by Robert C. Martin (MIT, see `LICENSE`). The principles are his text, unchanged. The Crapper, Dryer and Mutator sections now run `ubgate`, a Go port of his [crapper](https://github.com/unclebob/crapper), [dryer](https://github.com/unclebob/dryer) and [mutator](https://github.com/unclebob/mutator).
