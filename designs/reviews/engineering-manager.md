# Review: engineering manager, three teams, owns the Actions bill

## First 5 seconds

Cover: "Ore in. Ingots out. One binary runs Uncle Bob's three quality gates on any repo, 3–120× faster than the originals." I got: a CLI, Uncle Bob, speed. Not what it does to my Tuesday. "Faster than the originals" assumes I know the originals; none of my 25 engineers have run crapper or dryer, and the report confirms that is normal ("the gate tools have roughly 35x fewer stars than the skill").

S0 is a Python pricing function. The agent's side message, "I run three gates: complexity without coverage, duplication, and tests that would miss a bug," is the best sentence on the page, and it sits in a grey side panel. The headline, "A real function. Eighteen lines, three ifs, one test," is a developer's hobby. My problem is 180 PRs a week, not one function.

## The core question

A line-by-line code story reaches my senior ICs. It does not reach me, and I will not forward it, because the thing I would forward it to say never appears. The report: review time "+91% in 2025, median time-in-review +441% in the 2026 dataset," PRs "154% bigger," "31% more PRs merge with no review at all." That is my quarter in numbers. The page says none of it.

The report also warns this audience is "convinced by running the tool, not by the page," and that abstract heroes get "what does it actually do?" So I am not asking for pain marketing. I am asking for one screen before S0: "80.2% of agent-written test patches carry weak or no explicit oracle signal (arXiv 2606.18168). Three deterministic gates on changed code, run before every commit. Here is one." Then S0. The walkthrough becomes the proof, and my leads can read the pitch in a Slack preview. S0 cannot be previewed; it is a code block and a scroll prompt.

DORA gives me the framing for the VP meeting: "The time saved in creation is frequently re-allocated to auditing and verification." If the page said "this is the auditing, done by the agent before you see the PR," I would not have to write that sentence myself.

## The business case

To approve a pilot:

- Wall-clock of the mutation run on a PR-sized change, on a named machine. S6 says "takes seconds." "4.2 s, 12 mutants, M2, Django app" is a number.
- Tests deleted per 1,000 on a real suite, and CI minutes recovered. S8 deletes 2 of 5. That is a toy. The report notes "no source quantifies near-duplicate AI tests" and Blacksmith's CI figures are "almost entirely vendor-sourced," so the first honest number owns this argument.
- Survivors caught per PR on a real repo, and how many were real bugs. Bitcoin Core's "under 25% to over 80% on a single PR" is the shape I want.
- Time from `curl | sh` to first exit 1 on our repo.

Of the current numbers: 36/36 byte-identical means "will not drift from upstream," a procurement sentence, not a headline. 3.8× and 120× are vanity because the baseline is tools we do not run; faster than nothing is not a speedup. 7 languages matters only as a checklist (we are TypeScript and Go). The S9 footer, "measured 2026-11-03, 3 runs, median, same machine as upstream," is the most credible line on the page and exactly what the report praises ("small, bounded, reproducible"). Promote it, shrink the digits.

## Rollout fears

What gets it rejected in week one:

1. Blocking gates on legacy code. CRAP < 10 on a 2019 billing module means nobody commits for a month. The report: "nobody has published whether CRAP < 10 is achievable on real codebases." `--changed` appears once in a transcript; the page never says "old code is exempt." That sentence is adoption versus revolt.
2. Mutation runtime. Trail of Bits: "five minutes and 1,000 mutants, that's 83 hours." The math page's "re-runs only touch survivors and functions whose text changed" is the right answer, buried in column three.
3. DRY false positives on a repo full of CRUD handlers. Google needed "hundreds of arid node suppression rules." No suppression story here.
4. "Another linter." The report's cure is "a small set of high-signal tools that block CI, with the rest advisory." The page never says which gates block, or whether `--no-verify` is stopped; slopmop lists it as the first thing agents reach for.

The page addresses none of these in prose. The math section addresses two in fine print.

## Tests section (S7, S8)

This is what I would pay for, and it is the most under-sold part. "A test that cannot fail is not a test. Throw it out" is the right headline. "Mutants it alone would kill" is the right artifact. "A test with zero kills costs CI time and gives false confidence" is the only time the page talks to my bill.

Not convincing yet. The motion spec admits "the per-test kill table is a proposed feature. The mutator today reports kills per function, not per test." I am being shown a screenshot of a thing that does not exist, on a page whose credibility is "identical to the byte." Label it or cut it. And 2 of 5 is a demo; I need a real suite, N tests, M deleted, minutes before and after, and what happens when a deleted test would have caught a later bug. Run the "1,800 tests" experiment and print the table.

Also: S8 shows the agent deleting tests with `sed`. The report has Kent Beck "having trouble stopping AI agents from deleting tests." The same frame must say why this deletion is different (justified by a kill count, not a red build), or my most paranoid lead screenshots S8 and the pilot is dead.

## What you'd mock

- 900 vh of pinned scroll for four commands. The report's only recorded reaction to the format: "fading in elements. WHY??"
- "Three numbers. Three rules. No opinions," followed by a page of opinions about thresholds.
- "Ore in. Ingots out." Fine for developers. I will not say it to a VP.
- An agent pane labelled "live" on a scripted storyboard.

## Visual variant

Terminal. Press reads as a joke in a budget deck. Brutalist is a poster; the orange slab and all-caps pitch will get "is this a hackathon project?" Terminal looks like the tool it describes, screenshots cleanly into Confluence, and the `exit 0` status bar survives a Slack paste. The PlanetScale caveat applies: keep body text legible, because the person I forward this to reads it on a phone.

## Verdict

Yes, one team, the one with the worst review backlog: `--changed` only, advisory for two weeks, then blocking on mutate alone. The one thing that makes it a yes without a meeting: a page above the scroll story that runs the gates on a named open-source repo and prints tests deleted, CI minutes saved, survivors found, and wall-clock on stated hardware. The report says that artifact "would be more convincing to this audience than any page." It is right; I am the audience with the budget.
