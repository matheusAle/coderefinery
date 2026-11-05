# Review: indie founder, ships with Claude Code all day

Solo dev, Python + TS, three products in prod, starred ub-mode the day it went viral. Burned twice by the agent editing tests to go green. Installs via `curl | sh` without reading.

## First 5 seconds

Cover: good. Orange on black, mono, `curl` right there, status bar ends in `exit 0`. "Ore in. Ingots out." made me read the next sentence; "One binary runs Uncle Bob's three quality gates" did the actual work. I'd copy the curl before scrolling.

S0: the pricing function is fine, but the agent box top-right ("Paste a function, or point me at a repo...") is what hooked me. The right pane looks like my terminal. That's why I kept scrolling.

## The core question

The walkthrough is well-built but answers a question I'm not asking. CRAP 40.8 -> 3 is a refactoring lesson. My fear is the one the report nails: "73% of test runs were piped through `tail` or `head` so a failing run exited 0" (issue #104504), and Kent Beck "having trouble stopping AI agents from deleting tests in order to make them 'pass!'". I don't lose sleep over cyclomatic complexity. I lose sleep over "4/4 green" after the agent swapped the test.

S5 and S8 hit that fear. S1-S4 are a long way of saying "your agent writes slop and this catches it", and the report's own evidence says don't: "Don't gate 'what it does' behind a long scroll-driven animation"; the only reaction found was "fading in elements. WHY??".

First screen should be S5-shaped: a test file the agent wrote, `mutate` run, `survived 2`, `exit 1`, and the agent's next tool call blocked. Then "scroll for the other two gates." Lead with the gate that catches cheating; that's the pain with the viral stories.

## The agent angle

Half there. The agent pane does the edits, which is right ("The agent does the work" in the motion spec is the correct principle). But I never see what I'm paying for: the gate stopping the agent. S6's hint says "the skill makes an agent do it before every commit", which is prose, and the report is blunt that prose is the part that doesn't work: "a skill file only asks the agent to behave well, while the enforceable gates are what actually stop bad output."

Show me:
- `git commit` attempted, hook fires, `exit 1`, commit refused. One frame.
- The agent reading the exit code and going back to work, not around it.
- `--no-verify` blocked; slopmop lists it as the first thing agents reach for.
- The hook line in `.claude/settings.json`, so I know it's a hook and not a suggestion.

Exit codes are on screen but nothing consumes them. Show the consumer.

## Install and first run

I type `curl -fsSL https://get.coderefinery.dev | sh`. `npx skills add coderefinery` is what I'd type second and what I actually want, because it's the only one that implies the agent gets wired up. If `npx skills add` also installs the binary, say so in one line; if not, that's two steps I'll get wrong.

First 30 seconds: `cd` into my repo, one command, a table with file:line, an exit code. S1's output is exactly that; reuse it. The voice board's error ("no coverage report found. run the tests first, or pass --no-coverage... that is slower. you were warned.") is great. Put it on the page so I know what the first failure looks like.

## The math section

I'd skip it, then come back when the gate fails on something I think is fine. "cov needed to pass, by CC" is the one table I'd read; "CC 10+ impossible: CC alone is >= 10. split first" is the kind of blunt I trust. It builds trust by existing; the report says numbers are believed when "small, bounded, and reproducible", and the "byte-identical to crapper, dryer and mutator" footer does that.

## Tests section (S7, S8)

S8 is the best screen on the page. "A test that cannot fail is not a test. Throw it out" is my rule now. `test_settle_called(mocker)` with `assert spy.called` is the test Claude writes me daily; the report has wrs saying the same ("Claude is very good at writing that utterly useless kind of unit test"). Per-test kills landing on 0 and flipping molten is the screenshot moment.

Missing: it only covers the agent writing weak tests, not deleting or narrowing them, which is what burned me. Jesse Vincent's rule, "The only thing worse than a failing test is a reduction in test coverage", is the gate I want: killed count dropped since last run -> exit 1. Also the motion spec admits per-test kills is "a proposed feature" the mutator doesn't do yet. Ship the honest two-run version or label it "in development". If I install and that table doesn't exist, I tweet that instead.

## What I'd mock or screenshot

Mock: "Ore in. Ingots out." in the press variant's serif italic, that's a wine label. ingot/slag/ash/molten as verdict words; nobody says that at a terminal. 900vh of pinned scroll on a phone.

"Finally": the S8 table with two zeros. "that is slower. you were warned." The `sed -i` tool calls, because they look like my real session.

## Visual variant

Terminal. Press is beautiful and I'd never trust it in my commit path; it reads like a magazine about tools. Brutalist is a poster, fun for one tweet, exhausting for a table. Terminal is the only one where the page looks like what my shell shows ten seconds later, and the report says that's the whole loop: "this audience trusts the run, not the page."

## Verdict

Install, yes, from the cover alone.

What makes me tweet: a 10-second clip of Claude Code trying to commit, the hook returning `exit 1` with `survived 2`, and the agent writing the missing test instead of deleting the failing one. That's the product in one screen and it isn't on the page yet.
