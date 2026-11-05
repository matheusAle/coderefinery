# Review: platform / developer-experience engineer

Context: 1,000 engineers, one monorepo plus satellites. I own CI, the SonarQube and Codecov gates, pre-commit, and the AI coding guidelines. I ran PIT and Stryker in production; both are off today (runtime, equivalent mutants, noise). I still have a crap4j report in a drawer.

## First 5 seconds

The cover says `curl | sh` and the status bar says `exit 0`: a CLI with exit codes. It does not say where it runs. S0's agent pane with a green `live` dot says "an agent runs it in a chat", the least interesting target for me. Nothing on the cover or S0 says hook, CI, `--base`, or JSON out. The cover passes the report's "what does it actually do?" test and fails my follow-up: where, and who can turn it off.

## The core question

The walkthrough is good, and the report backs putting it above the fold: "show the rejection in a real terminal above the fold"; before/after code "is the most compact way to make a claim falsifiable." Keep it. But it is the wrong *first* screen for anyone wiring tools in. The report's summary of what works is an integration diagram: "sub-second local hooks → commit-time hard gate with bypass blocked → CI gates on new or changed code only → AI review as advisory," with four properties: "scoped to new or changed code, actionable, un-bypassable by the agent (`--no-verify` blocked, independent CI rerun), and few." Thoughtworks' Adopt call carries the same condition: "automated mutation runs are time-consuming," so run "incremental checks on recently changed code."

The storyboard shows the first half (actionable, changed-code) and is silent on the second (un-bypassable, independent rerun). The first screen should be one frame, three columns: the hook (`PreToolUse` on `git commit`, exit 2 blocks the agent), the CI job (`coderefinery check --base origin/main --format json`, N seconds on a named repo), and the exit-code table. Then S0 onward as the "what the gates find" chapter.

## Technical credibility

The math page is the best page in the deck. It still has holes.

- **CRAP < 10 is a complexity cap of 9.** The table admits it ("CC 10+ impossible"). Savoia's 30 let a CC 5 function pass untested; yours needs 42%. Say "CC ≤ 9 plus a coverage curve" out loud, and say what happens to the thousands of existing functions above it. `--changed` is the answer, but changed against which base is never stated.
- **I count CC 9 for `price()`, not 8.** Your own rule counts `for`, five `if`/`elif`, and the `and`: 1 + 8 = 9. The heading says "three ifs"; there are five. It is the first number on the page.
- **Branch vs line.** S1 says "eight branches, one of five covered"; the formula says "cov is the share of its lines." For Go, `go tool cover` gives statement blocks, neither lines nor branches. Byte-identical to crapper does not tell me which coverage format is consumed per language.
- **The dry example contradicts S3.** Math: price/quote share 11/15 = 0.73 → pass. S3: same pair 0.91 → fail. And "same bodies, renamed args = 1.00" describes exactly S3's pair. Pick one.
- **`--min-nodes 20` vs S7.** The duplicated tests are two statements each. Do they clear 20 normalised nodes?
- **Equivalent mutants are never mentioned.** "One survivor fails the gate: exit 3" is the sentence that gets this tool turned off. Google: "the vast majority of mutants were simply bad," hundreds of arid-node rules. Bitcoin Core caps at 7 survivors. I need an `equivalent` annotation and a cap, or every refactor of a log line goes red. The report: "Multiple sources explicitly warn against a repo-wide mutation score as a merge gate."
- **"The test suite runs once per mutant."** A 4-minute suite and 300 mutants is 20 hours. Trail of Bits: "1,000 mutants, that's 83 hours." You have coverage; use it to select the tests that execute the site, and say so. "Re-runs only touch survivors" helps the second run, not the first.
- **Timeout = max(2 s, baseline × 10)** is nondeterministic on shared runners: a mutant "killed" by timeout on a slow box survives on a fast one. Report timeouts as a third state. Same for "fails to compile counts as killed"; PIT and Stryker exclude those from the denominator, which is why their scores are comparable.
- **Exit codes.** `1` for crap/dry, `2` for missing coverage, `3` for a survivor. Possibly fine, never listed together, no `--format json` or SARIF anywhere.

## What would make it un-bypassable

Nothing on the page. S6 reads "the skill makes an agent do it before every commit." The report's ub-mode section is about why that fails: "a skill file only asks the agent to behave well, while the enforceable gates are what actually stop bad output," and the gate tools have "35x fewer stars than the skill." Issue #104504: "73% of test runs were piped through `tail` or `head`." Show a `PreToolUse` hook returning exit 2, a `core.hooksPath` the agent cannot edit, the CI job rerunning the same commands so `--no-verify` buys nothing, and a "coverage must not shrink" rule. Worse: S8 shows the agent running `sed -i '.../d'` to delete two tests with no human in the loop. That is the Jesse Vincent incident as a feature demo.

## Tests section (S7, S8)

The per-test kill table is the feature I would pay for, and the motion spec is honest that "the mutator today reports kills per function, not per test." Cost: attribution needs every mutant run without fail-fast with JUnit parsed per run, so mutants × full suite with `-x` off. On our monorepo that is a nightly, not a hook; say so. "Mutants it alone would kill" is a unique-kill count; zero means redundant, not useless (redundant tests localise failures). "Shadowed by" from a greedy set cover is order-dependent; a hint, never a delete rule. S8's deletions are justified by the tags ("passes if price() returns 0", "tests the implementation"), not by the zero. Lead with the tag.

## What I'd mock

"3–120×" with no absolute times, hardware, or repo link; "3 runs · median · same machine as upstream" is a method without a result. The report's Vercel lesson: "cherry-picked, non-peer-reviewed, borderline-misleading numbers." "Under a second on the Django codebase" with no machine. "No opinions" above a page of thresholds chosen by the skill (9.9999, 0.8 vs the tool's 0.82, min-nodes 20). The agent deleting tests. 0.73 vs 0.91.

## What I'd trust

"36/36 harness cases identical" and "byte-identical to crapper, dryer and mutator" land; link the harness. "7.2 MB · stdlib only · no cgo" is exactly the sentence a platform team wants and it is buried on S9. "Re-runs only touch survivors and functions whose text changed" is the most important operational claim and it is the last line of the math page. Move all three up.

## Visual variant

Terminal. It matches the output I will read, and the status-bar `exit 0` is the right punchline. Press is clever for one glance and then fights the transcript; brutalist shouts. Mind the report's PlanetScale note on "white text on black backgrounds": keep body copy at chalk, not ash.

## Verdict

I would pilot `crap --changed` and `dry` in CI as advisory on one repo this quarter: deterministic, fast, changed-code scoped. I would not enable `mutate` as a gate. The one blocker: mutate runs the full suite per mutant with no coverage-based test selection and no equivalent-mutant suppression, and "one survivor fails" on top is the exact configuration that got PIT turned off here. Show test selection, a survivor cap, an equivalence annotation, and a hook plus CI rerun the agent cannot edit, and I would wire it in.
