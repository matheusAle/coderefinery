# QA · site · round 5 (live page, http://localhost:5173/)

Method: agent-browser CLI, dedicated session (the default session is shared with other QA agents and was contaminated once; everything below was re-measured in a private session). Viewports 1440x900, 1280x720, 900x800, plus `prefers-reduced-motion: reduce`. Nothing under site/ was read or edited. Screenshots: `designs/reviews/qa-shots/r9-qa-*.png`.

**Caveat, the build moved during the run.** The console shows HMR updates of `base.css` and `sections.css` mid-session. Before the change every home card sat in a 100 vh slot (tops 900, 1800, 2700 ...) and the entrance timeline met §0c. After it, each home card sits in a `.hold` wrapper of 1.5 x vh (1350 px at 900 px, 1080 px at 720 px). Entrance results are therefore reported for the latest build, with the earlier numbers noted. Dive, math, fit, perf and reduced-motion checks were re-run or re-confirmed on the latest build.

## Pass / fail

| # | Check | Result | Evidence |
|---|---|---|---|
| 1a | Home order P0 P1 P2 P3 P4, dive K1-K4, S9 §06, teaser §07, setup §08, footer | PASS | kickers in DOM order: hero, §01 what changed, §02 coderefinery, §03 why a formula, §04 the tests, dive, §06 after the intro, §07 the math, §08 recommended setup · two minutes, footer. `r9-qa-home-1440-*.png` |
| 1b | `$ coderefinery crap --changed` types inside P4's CTA in the last stretch before the dive slides over | PASS (latest build) | types between scrollY 5260 and 5400 (140 px), complete exactly when the dive top reaches 900. `r9-qa-bridge-typing-3560.png`, `r9-qa-bridge-now-5330.png` |
| 1c | `/#hook` from /agents lands on the setup card | PASS, with a note (D8) | scrollY 13050, `#hook` top 0, kicker `§08 · recommended setup · two minutes`. `r9-qa-hook-landing-now.png` |
| 2a | Dive: no code pane; session pane left; outputs pane right with three blocks and footers (`−12 +9 · 2 files`, `+2 tests · 1 file`, `−14 +5 · 1 file`) | PASS | `r9-qa-dive-1440-K1..K4-810.png` vs K1-K4 PNGs |
| 2b | Dive copy verbatim | PASS | 103 live strings checked against page-reference.html; only the UI labels `Inside the gate` and `↓ types as you scroll` are not in it |
| 2c | Verdict flash on exit lines | PASS | `window.__flashLog` has slag and ingot entries for all four states, session and outputs panes, shader `gl` |
| 2d | Furnace full bleed, status bar with verdict + exit code, titles clear of the hint box | PASS | furnace-host 0,0 1440x900; status bars e.g. `exit 2 → exit 0`; no title/hint overlap at 1440 or 1280 (K3 title wraps to two lines) |
| 2e | Hint box present throughout each state | FAIL (D4) | hint is `visibility:hidden` until roughly t = 0.7 of every state |
| 3a | Math formula per state substitutes the computed value (list below) | PASS with deviations (D5) | screenshots `r9-qa-math-1440-*`, `r9-qa-math-1280-*` |
| 3b | Code pane with syntax colour, annotations of §16.2 landing in order | PASS | notes land in order in all 11 states; `r9-qa-math-1440-crap1-85.png` etc. |
| 3c | Card title / prose / rule / visual verbatim | PASS | all 11 cards (title, body, rule, visual) match math-reference.html after stripping backticks and whitespace |
| 3d | Status bar, formula on one line, nothing overlaps | PASS | formula `scrollWidth == clientWidth`, height 47 px (1280) / 59 px (1440); no overlap in 44 state captures |
| 4 | Variables and values are inline code (mono, slab tint, 0.92 em) | FAIL (D3) | everything that IS wrapped passes (50 `.ic` sampled: Geist Mono, `rgb(31,32,35)` tint, 0.92, not italic); many values are not wrapped |
| 5a | Entrance: kicker 0.60-0.63, nothing before 0.60 | FAIL on math (D2); PASS on home and /agents | |
| 5b | Headline complete by 0.74, cards 0.72-0.86, status bar full at 0.90 | FAIL on latest build (D1); PASS on the earlier build | |
| 5c | Covered card scales / shifts / dims | PASS | `.h2` left 64 → 77 → 89 and width 1312 → 1262 (scale 0.96), card top −24 px, `visibility:hidden` at e = 1 |
| 5d | Title line-height 1.25 | PASS | all h1/h2/h3/`.ex-title` computed 1.25 on /, /agents, /math (only small mono labels are `normal`) |
| 6a | Every card, dive state, math state fits 100 vh at both sizes, reveals open | PASS | 1440x900 and 1280x720: 7 home cards, 4 /agents cards, 8 dive captures, 22 + 22 math captures; no overflow |
| 6b | No overflow at 900 px | PASS | scrollWidth 900 at every 400 px step on /, /agents, /math; dive stacks cleanly (`r9-qa-dive-900-1800.png`) |
| 6c | Reduced motion sane | PASS | titles complete, blocks open on every home card and /agents card; dive and math cross-fade states legibly (`r9-qa-rm-*.png`) |
| 7a | Console clean | PASS | only `[vite]` connect/HMR lines |
| 7b | `window.__streamCost` under 1 ms | PASS | avg 0.10 ms, max 1.0 ms over 141 samples |
| 7c | Nav active states | PASS with a note (D9) | /agents: `for agents` `aria-current=page`; /math: `the math`; home none |
| 7d | No long frames scrubbing dive and explainers | PASS | rAF scrub 30-150 px per frame over the dive, whole home, /math, /agents: 0 frames over 50 ms, 0 longtasks (worst single frame 44 ms on /agents) |

### Math formula text per state (§16.1 comparison)

| state | live formula (molten term in **bold**) | §16.1 |
|---|---|---|
| crap 1 | CRAP = **9²** × (1 − cov)³ + 9 | match, but the second `9` is chalk (spec: both roll in) |
| crap 2 | CRAP = 9² × **(1 − 0.20)³** + 9 | match |
| crap 3 | CRAP = 81 × 0.512 + 9 = **50.5** | match |
| dry 1 | A = fp(price) · **22**   B = fp(quote) · 22 | match, B's `22` is chalk |
| dry 2 | score = **20** / 22 | match |
| dry 3 | score = 0.91 ≥ **0.80** → DUPLICATE | text matches; molten term is `0.80` (matches M2.3 PNG), spec says the word lands |
| mutate 1 | sites = **12**   mutants = 12 | match |
| mutate 2 | score = **10** / (10 + 2) | match |
| mutate 3 | score = 10 / 12 = 0.83 · survived **2** → exit 3 | text matches; `0.83` and `exit 3` stay chalk |
| tests 1 | kills(t) = { m : t fails on m } → **4 · 1 · 5 · 0 · 0** | match |
| tests 2 | unique(t) = kills(t) − ⋃ kills(others) → **2 · 1 · 0 · 0 · 0** | match |

Earlier values stay chalk in every state. Annotations at t = 0.5 / 0.85 land in the order of §16.2 (e.g. crap 1: notes 1-4 at 0.5, all eight at 0.85; mutate 2: killed ingot, then two SURVIVED slag, then the `kills …` notes).

## Ranked defects

### D1 · High · Entrance timeline runs past the dock (regression within this run)
- Spec: §0c, headline types 0.60-0.74, cards 0.72-0.86, status bar 0.86-0.90, complete at e = 0.90; "No sticky hold".
- Observed (latest build): home `.helps`, 1280x720: kicker at e 0.65, headline 22/35 at 0.75, 29/35 at 0.80, complete at 0.85; first card opens at 0.90, second at ~1.1, third at ~1.3; status bar opacity 0 until ~1.4 (i.e. 0.4 vh after the card has docked). /agents `.a-reliable`: headline 47/47 only at e 0.9, status bar at ~1.4. P1 docked at 1280x720 shows the stat numbers still rolling and no status bar (`r9-qa-home-1280-pains.png`). Each home card now sits in a hold of 1.5 x vh, which is the "sticky hold" §0c removes. Earlier in the run (100 vh slots) the same measurement gave headline 35/35 at e 0.75 and status bar 1 at 0.90.
- Screenshots: `r9-qa-deck-now-1800.png`, `r9-qa-home-1280-pains.png`.
- Repro: open `/`, 1280x720, `scrollTo(0,0)`; hold top = `document.querySelector('.helps').parentElement.getBoundingClientRect().top` (1800); for e in 0.6..1.5 `scrollTo(0, 1800 − 720 + e·720)` and read the `.helps` kicker opacity, `.typed-title .ch.on` count, `.statusbar` opacity.

### D2 · High · Math explainers show content before e = 0.60
- Spec: §0c "nothing before e = 0.60" (the slide is the first animation).
- Observed: on all four explainers the incoming card shows its molten top rule, the bottom of the `rule` box and the visual strip (9 squares / bar) from e ≈ 0.35 while title, prose, formula and code are still hidden. Card style is `--o: 1; --rv: 0` already at e 0.35, so the clip does not hold the rule and visual back. Home and /agents cards are clean at the same e.
- Screenshots: `r9-qa-entrance-math-crap-e.5-now.png`, `r9-qa-entrance-math-tests-e.5.png`, `r9-qa-entrance-math-crap-e.5.png`.
- Repro: `/math`, 1440x900, fresh load, `scrollTo(0,450)` (crap card top at 450). Visible: orange rule at y ≈ 710 and `rule CC 9 means nine inputs…` clipped at y ≈ 810, squares at y ≈ 865. Same at `scrollTo(0, 3150 / 5850 / 8550)` for dry / mutate / tests.

### D3 · Medium · Variables and values in prose are not all inline code
- Spec: §0b "Variables and values in prose are code" (`CC`, `cov`, `0.512`, `exit 2`, `max(total, 0)`).
- Observed, set in the body face with no tint: dive hint box K1 `81 × 0.512 + 9 = 50.5`; dive agent reading K3 `max(total, 0)`; math card prose `exit 3` (mutate 3 "Any survivor is exit 3"), `kills 5` (tests 2), `0.8` and `0.82` (dry 3), `20 of 22`, `3 of 15 lines`, `83 %`; /agents `zero kills`; home §08 setup list: `npx skills add coderefinery` and `exit ≠ 0` are plain text separated by `&nbsp;&nbsp;&nbsp;` (visible gap in `r9-qa-hook-landing-now.png`).
- Repro: `/` scroll to the dive K3 and inspect `.agent-p`; `/math` inspect the `.ex-body` of mutate 3; `/` jump to `#hook` and inspect the first list item. Wrapped ones are correct: 6 on home (P2), 30 on /math, 14 on /agents.

### D4 · Medium · Hint box arrives late in every dive state
- Spec: §13.1 "one per state; the heuristic, in one sentence, with the formula as its kicker"; §0c first state is assembled when the pin begins.
- Observed: K1 hint is hidden until scroll ≈ 4800-5200 of the 4500-5400 K1 range (earlier build) and at 1280x720 it is hidden at t = 0.5 in all four states, visible at 0.85. At the pin start the K1 panes are also empty (command not yet typed) so the card is not "assembled".
- Screenshot: `r9-qa-dive-entrance-4500.png`, `r9-qa-dive-1280-K3-85.png` (visible) vs a t = 0.5 capture.
- Repro: `.dive .hint` style is `--o: 0; visibility:hidden` until about t = 0.7.

### D5 · Low · Formula molten terms differ from §16.1 in a few states
See the table: crap 1 second `9`, dry 1 second `22`, mutate 2 `(10 + 2)`, mutate 3 `0.83` and `exit 3`, dry 3 `DUPLICATE`. The dry 3 case follows the M2.3 PNG (molten `0.80`), so spec and PNG disagree; decide which wins.

### D6 · Low · Explainer pin length is shorter than states x 100 vh
- Spec: §16 "states x 100 vh".
- Observed: each explainer state gets 2/3 vh of scroll (crap 600 px at 900 vh, 480 at 720) and tests gets 0.5 vh per state (450 px); the section element is states x vh tall but the pin is that minus one viewport. The dive uses a full vh per state.
- Repro: `/math` crap `offsetHeight` 2700; the formula row switches at scrollY 1500 and 2100.

### D7 · Low · Prompt loses its space while the next command types
The bottom line of the session pane shows `$coderefinery d` (and a bare `$` at the end of K3) vs `$ coderefinery …` everywhere else. `r9-qa-dive-1440-K1-810.png`, `r9-qa-dive-1280-K3-85.png`.

### D8 · Low · Deep link to `#hook` lands on a card whose transcript and status bar are not complete
At scrollY 13050 the transcript has shown only the refused commit, and no status bar ("zero deleted" fades in later). Expected under a scrubbed design, but a reader arriving from /agents sees an unfinished card until they scroll. `r9-qa-hook-landing-now.png`.

### D9 · Low · Nav
`docs` and `github` both point at `https://github.com/coderefinery`; home has no active item. Nav exists only inside the hero, so it is absent for the rest of the home page.

## Verdict
The round-5 content is right: the home order and the P4 → dive bridge work, the dive reads as a CLI-and-agent session with verbatim copy, correct verdict flashes, full-bleed furnace and clean titles, and all eleven math states carry the correct formula, annotations, card copy and a fitting layout at both sizes, at 900 px and under reduced motion, with a clean console, `__streamCost` at 0.1 ms and no long frames. It is not ready to sign off. Two items block it: the entrance timeline on home and /agents now finishes after the card has docked (headline, cards and status bar are 0.1-0.5 vh late, with a 0.5 vh hold per card that §0c forbids; it met spec earlier in this very run, so it looks like a regression from the latest CSS change), and the math explainers show their rule box and visual before e = 0.60. Fix those, then wrap the remaining variables and values in inline code (D3), and let the hint box arrive with its state (D4); the rest is polish.
