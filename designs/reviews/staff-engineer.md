# Review: staff engineer, 15 yrs, Go/TS, tired of agent PRs

## First 5 seconds

Cover: "One binary runs Uncle Bob's three quality gates on any repo, 3–120× faster than the originals." I got: three linter-ish things, Go, fast. I did not get what a gate rejects, and "Uncle Bob" is a coin flip for my crowd (the report notes HN's "long-standing scepticism of Uncle Bob"). "Ore in. Ingots out." is a tagline, so I skipped it. The `curl | sh` box kept me on the page: a binary, not a waitlist.

S0 earned the scroll: a real function with line numbers and "It has one test, which checks the happy path." That is pain #1 in the report ("handles 95% of cases perfectly but silently drops the other 5%"). Words that did it: `billing/pricing.py`, `exit`, `--changed`. Words that nearly lost me: "Paste a function, or point me at a repo" in a bubble labelled `agent`. That is a chatbot, and I came here to be rid of those.

Verdict: scroll, barely.

## The core question

Keep the code open. Do not open with "review load is up 91%". The report says the top dismissal on pages that lead with framing is literally "what does it actually do?" and that the job is to "show a real terminal or code diff above the fold." Every AI-review vendor opens with my pain; the report calls their numbers "directionally useful, though... the vendor sells review tooling." Naming my pain back at me is marketing. Showing me a function I would have rubber-stamped is what a colleague does.

But the first screen is the wrong frame. S0 is setup; the payoff is S1 (`40.8  20.0  8`, `exit 1`). The report: "shows the rejection in a real terminal above the fold." So screen one should be S1's state on S0's code: the function left, the failing table and `exit 1` right, heading "This passed review. CRAP 40.8. Line 16 has never run under a test." Then the scrub can start. Nobody should scroll a viewport to see the tool say no.

## What you'd mock

- S0: "Eighteen lines, three ifs" beside a file bar saying "24 lines" and code with four `if`s and two `elif`s. A page about counting decisions miscounts its own ifs.
- S1: "Eight branches, one of five covered" next to `cov% 20.0`. Five what?
- S5 and M: `max(total, 0) → min(total, 0)` is SURVIVED. With the tests on the same screen, `test_coupon` expects 80.0 and `min(80, 0)` is 0. That mutant is killed. The flagship mutation example is wrong, and the math page repeats it as the worked example. This is the screenshot.
- M vs S3: the math page scores `price()`/`quote()` at 11/15 = 0.73, pass; the storyboard says 0.91, fail. M says one survivor is `exit 3`; S5 shows `exit 1`.
- `120×` in orange. No baseline named, no hardware ("same machine as upstream" is not a machine), no list for "12 repos." The report's Turbopack lesson: "the methodology correction was all anyone remembered." My guess: that is JVM startup vs a Go binary, a fact about the originals, not you.
- The agent fixing code with `sed -i '2,9c\...'`. GNU-only syntax, breaks on the macOS laptop reading the page. The motion spec says "keep it real so engineers trust it."
- S8 has the agent `sed ... d`-ing tests out. Pain #4 in the report is agents deleting tests to go green (Kent Beck, Vincent's `rm -rf **/*test*`). That clip travels without context.
- "Three numbers. Three rules. No opinions." followed by 10, 0.8, 20 nodes, 9.9999. Those are opinions. Good ones; say so.

## What you'd trust

- `exit 1` with file:line in a pipeable table. The report's non-theatre gate is "actionable (line numbers, not 'coverage too low')." This is that.
- `--changed`. The whole practitioner consensus is "scoped to new or changed code." It appears once, in S1, and not in the hero. Promote it.
- "36/36 byte-identical", "7.2 MB, stdlib only, no cgo", "3 runs, median." Small, bounded, checkable.
- The `--no-coverage ... (that is slower. you were warned.)` error. Honest tooling voice.
- The spec admitting the per-test kill table "is a proposed feature." Good to write down; bad that the storyboard shows it as `live`.

Before installing: a public repo with the 12-repo benchmark script and the 36 fixture pairs; one pair with the original's output beside yours; a one-liner that finishes under 30 s on my repo and prints three verdicts; and a first-person write-up with survivor counts and a "when not to use this" (the report says none exists, "both a gap and an opportunity").

## The math section

Mostly credible. The formula, the exact decision tokens, and the normalisation keep/drop lists are the first time the page spoke to me as an engineer. The "cov needed to pass, by CC" table is the best thing on the site: CC ≥ 10 is a hard fail regardless, so the gate is really "split anything with ten decisions," a rule I can argue with.

Numerology creeps in with `--threshold 9.9999` "because scores carry four decimals" (say `>= 10`), the 0.82 default lowered to 0.8 for no stated reason, and "fails to compile counts as killed" (Google called those bad mutants and suppressed them; counting them inflates the score). Missing entirely: achievability. The report: "nobody has published whether CRAP < 10 is achievable on real codebases." You ran 12 repos. What fraction of functions fail on django, on your own binary? Without that, `0 above 10` in the status bar is a demo of a demo.

## Tests section (S7, S8)

"A test that cannot fail is not a test" lands. It is the report's falsifiability rule ("a good test must not provide its own success conditions"), and `isinstance(..., float)` is exactly the 80.2%-weak-oracle shape I delete weekly. The spy test ("passes when the code is wrong and fails when the code is refactored") is the sharpest line on the page.

The per-test kill table is not believable: it does not exist yet, and 4+1+1+0+0 cannot union to 12 kills, so rows are missing. Ship the spec's honest version (`-k "not ..."` rerun, same kill count) until it is real. S7 is minor, but its `apply_patch` is the one edit that looks like a real agent did it.

## Visual variant

Terminal. The storyboard already is one, the three-verdict status bar with `exit 0` is the identity, and the report's praised pages are "terse and readable, not ugly on purpose." Press is the one I would admire in a screenshot and never associate with a CLI, and "PRICE: ONE EXIT CODE" breaks the voice board's own "never cute." Brutalist all-caps paragraphs are unreadable at length; the PlanetScale backlash was exactly that. Caveat: mind the ash-on-black contrast; "I struggle reading white text on black backgrounds" is in the report.

## Verdict

Not today. I would run `crap --changed` on one repo once the benchmark repo and fixture pairs are public; what would change my mind is one honest post from someone who ran all three gates on a real 50k-line codebase, with timings, survivor counts, and where they turned it off.
