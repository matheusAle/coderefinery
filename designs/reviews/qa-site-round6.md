# QA · site · round 6 (live page, http://localhost:5173/)

Method: agent-browser CLI, private session `qa6`, live DOM only (no site code read or edited, nothing committed). Viewports 1440x900 and 1280x720 (plus reduced motion). Positions use the task formula, `scrollY = T - vh + m*1.5*vh`, with T taken from a fresh load at scrollY 0 (the deck makes later rects drift while a card is stuck). The page has `scroll-behavior: smooth`, so a programmatic jump takes 0.6-1.3 s to arrive; every measure waited for arrival (a first pass that did not wait produced two false alarms, both discarded). Helper scripts and raw data: `designs/reviews/qa-shots/r11-qa-*.js|sh|py|json`. Screenshots: `designs/reviews/qa-shots/r11-qa-*.png`.

Runway sweeps: every 0.01 of m from 0.30 to 1.00, fresh load per card, ascending, 380 ms settle, effective opacity (own x ancestors x `filter: opacity()`), clip-path and surface alpha (background, border, `--bx` seams) per element. Block-order sweeps: 0.01 of m (0.005 of e for the pinned sections) from 0.45 to 0.95.

## Pass / fail

| # | Check | Result | Evidence |
|---|---|---|---|
| 1a | Runway: slide only to 0.40 (nothing visible) | PASS | P1, P3, P4, §06, setup, /agents A0 + A2, both sizes: 0 text, 0 surfaces, kicker 0, title 0 chars at every m <= 0.395 (`r11-qa-raw-*-1440.json`, `*-1280.json`) |
| 1b | Kicker 0.40-0.43, headline types 0.40-0.56 | PASS | all six cards: kicker 0 at 0.40, 1.0 at 0.43; headline first char 0.41, complete 0.56 (P1 48 chars, P3 66, P4 47, §06 52, setup 60, A0 59, A2 47) |
| 1c | Blocks spread 0.52-0.80, last support at 0.88, captions 0.84-0.88, status 0.88-0.90 | PASS except setup (D1) | P1 columns 0.53 / 0.67 / 0.81, supports 0.58 / 0.72 / 0.86; P3 table 0.62, panes 0.74-0.81, last text 0.88; P4 tiles 0.53, quote 0.60, patterns 0.72, measure 0.73, CTA 0.81-0.86; §06 tiles 0.58, facts 0.62 / 0.71, last label 0.88; status 0.89 -> 1.00 at 0.90 on every card |
| 1d | Nothing changes 0.90-1.00 | PASS except P4 (D8) | P1, P3, §06, setup, A0, A2: zero changes after 0.90. P4: changes every 0.01 from 0.91 to 1.00 (the K1 bridge command typing, a documented round-10 choice) |
| 1e | No dead zone > 0.10 between 0.40 and 0.88 | FAIL on setup (D1) | largest no-change gap per card, 1440 / 1280: P1 0.01 / 0.01, P3 0.02 / 0.02, P4 0.01 / 0.01, §06 0.02 / 0.02, A0 0.07 / 0.06, A2 0.07 / 0.06, setup 0.21 / 0.21 (m 0.60 -> 0.81) |
| 2 | Text before box, then support (9 blocks) | FAIL (D3) | see table below; P1 columns, P3 table, P4 tiles, setup hook pane, /agents verdict table, dive session/outputs/before, §06 tiles pass. P4 right card and the /math card show support text before the box |
| 2b | Kicker fades, does not type | PASS | kicker opacity 0 -> 1 over 0.40-0.43 (0.83 at e 0.62 on /math), `.ch` children = 0 on `.kicker` |
| 3 | Spacing floors (about 100 surfaces measured) | FAIL (D4, D5, D6) | see tables below |
| 4a | Every unpinned card fits, both sizes | PASS | 7 home + 4 /agents cards: content bottom <= status top, `scrollHeight - clientHeight` 0, no horizontal overflow; also with `flex-shrink: 0 !important` on every descendant (1280x720: setup content bottom 662 vs status 677, P3 652 vs 677, P4 642 vs 677) |
| 4b | Dive states and math states fit | PASS | dive K1-K4 x 3 phases x both sizes: panes end <= 649 (720) / 781 (900), status at 676 / 850, session text <= 628 / 753. Math 11 states x both sizes: content <= 539 / 624, no overflow |
| 4c | P4 three mutant rows, setup fits at 1280x720 | PASS | `.m2t-row` x 3 at both sizes; setup fits (gap to status bar 15 px, tight). `r11-qa-setup-1280-m0.95.png`, `r11-qa-tests-1280-m0.95.png` |
| 5 | Page bottom, 3 routes x 2 sizes, 0..300 px in 50 px steps | PASS (note D9) | `elementFromPoint` on a 30x18 grid at 7 positions per route: only the last card (and the footer at max scroll) is ever hit; last card docked (section top 0 at +100 px), footer directly under it (section bottom = footer top). Dive covered state: 0 visible descendants of `.dive` and `data-hidden=1` from y = 10800 (card top -24) |
| 6a | Dive first state assembled at the dock | PASS | at e = 1.00: title, `$ coderefinery crap --changed ...`, two gate output lines, `exit 2` underlined, agent reading, outputs `before` table and `exit 2`, status line (`r11-qa-dive-entrance-e1.00.png`). Frames after words: session surface 0.785, outputs 0.82, before block 0.825 vs words 0.73-0.74, supports 0.855 (e) |
| 6b | K1-K4 scrub, hint boxes in the settle phase | PASS | hint hidden at t = 0.15 and 0.50 of every state, visible at 0.85 (`r11-qa-dive-K1..K4-settle-1440.png`) |
| 6c | Math: nothing before 40 %, first state by the dock, 100 vh per state | PASS | crap card e 0.30-0.58: no text, no border (only the `deck-dim` overlay); kicker 0.62; title 0.605-0.74; docked e = 1.00: formula, card, code, status bar all complete. Explainer heights 3600 / 2700 px at 900 = (states + 1) x vh; state k switches at k x vh |
| 7a | dry 3 formula lands `DUPLICATE` molten | PASS | `DUPLICATE` rgb(255,106,26), opacity 0.67 at t 0.4, 1.0 at 0.6; `0.80` and the rest chalk |
| 7b | Bridge typing keeps `$ ` | PASS | text `$ ` at m 0.86-0.90, `$ coder` ... `$ coderefinery crap --changed` to 1.00; 7 px between prompt and command (`r11-qa-bridge-m0.96.png`) |
| 7c | `/#hook` from /agents lands finished | PASS | clicked "see the hook ->": scrollY 13500, card top 0, title 60/60, status bar opacity 1, 8/8 transcript lines (but see D2) (`r11-qa-hook-landing.png`) |
| 7d | docs link != github link | PASS | `docs` -> `/docs` (HTTP 200 from the dev server), `github` -> `https://github.com/coderefinery` |
| 7e | Inline code on values and commands | PARTIAL (D10) | 114 `.ic` in the sampled containers (home 20, /math 80, /agents 14); 35 plain-text values or commands left, listed in D10 |
| 8a | Console | PASS | only `[vite] connecting/connected`; `errors` empty |
| 8b | Frame times while scrubbing | PASS | rAF scrub 40 px per frame over the whole page: `/` 351 frames, `/agents` 137, `/math` 373: worst 22 ms, p95 17 ms, 0 frames over 50 ms, 0 longtasks |
| 8c | Reduced motion | PASS | `prefers-reduced-motion: reduce`: at m = 0.55 every home card has kicker 1, title complete, status bar 1, blocks open |
| 8d | Hero stream | PASS | hero text length changes between samples (12409 -> 12404), `__streamCost` n 278, avg 0.13 ms, max 1.1 ms |
| 8e | Deck stacking | PASS | P1 covered by P2: scale 1.000 -> 0.988 (e 0.3) -> 0.980 -> 0.968 -> 0.960 at e = 1.0, translateY 0 -> -7 -> -12 -> -19 -> -23.97 px, `.deck-dim` 0 -> 0.30 (70 % brightness), `visibility:hidden` + `data-hidden` at e = 1.0 |

### Text-before-box measurements (first m / e at which each layer is visible)

| block | words | surface (tint, seams) | support | verdict |
|---|---|---|---|---|
| P1 column 1 / 2 / 3 | 0.53 / 0.67 / 0.81 | 0.56 / 0.70 / 0.84 (+0.03) | 0.58 / 0.72 / 0.86 (+0.02) | PASS |
| P3 table | 0.62 | 0.65 (+0.03) | 0.67 (+0.02, first-column labels) | PASS |
| P4 stat strip (3 tiles) | 0.53 | 0.56 (+0.03); tile 1 / 2 own seams 0.57 / 0.58 | 0.58 (+0.02 on the strip, +0.01 / 0.00 on the tile seams) | PASS (tile seams and sources land together) |
| P4 right card `.measure` | 0.73 (title, prose, `$ coderefinery tests` output, `preview` line) | 0.77-0.78 (+0.04) | 0.80 (note, bar label, prose) | FAIL: `.measure-prev` shows with the words, before the box |
| §06 numbers strip | figures 0.56 (placeholder zeros) | 0.56-0.57 | labels 0.58 | PASS |
| setup hook pane | 0.53 | 0.56 | 0.58 | PASS |
| /agents verdict table (A1) | 0.53 | 0.56 | no muted cells (all chalk) | PASS |
| /math crap card 1 | title 0.605 e, prose + rule text + caption 0.73 e | 0.78-0.80 e (rule box, visual slab, card rule) | `rule` label and `CC = 1 + 8 = 9` caption already visible at 0.73 e | FAIL: support before box |
| dive outputs / before block | 0.74 e | 0.82 / 0.825 e | 0.855 e | PASS |
| dive session pane | 0.73 e | 0.785 e | 0.855 e | PASS |

### Spacing, 1440x900 (`--u` = 0.978) and 1280x720 (`--u` = 0.783, clamped to 0.8)

| surface | measured 1440 / 1280 | floor | result |
|---|---|---|---|
| section padding, cards (kicker top, left/right) | 31-38 top / 64 sides; 20-25 / 64 | 28 / 22.4 | PASS |
| P2 gate cells, P2 teaser formula cells, §06 tiles, P4 stat tiles, A1 loop steps, A4 cells (padding) | 23 / 19 | 24 / 19.2 | 23 < 24 at 1440 (D6), 19 ok |
| CTA slabs (P4, teaser, /agents, M5) | 27 / 22 | 28 / 22.4 | 27 < 28 at 1440 (D6) |
| table cells: `.vs3-row`, `.hlist-row`, `.m2t-row`, `.mrow`, `.rrow`, A2 `.vrow` | 14 v / 18 h (13.7 / 17.6); 11 / 14 | 14 / 18; 11.2 / 14.4 | PASS (within rounding) |
| P3 / A0 / setup / measure panes (body) | inset 27-31; 21-23 | 28 / 22.4 | PASS (27 at 1440, D6) |
| pane bars (home, agents, dive) | 10 v / 27 h; 8 / 21 | 28 / 22.4 | vertical 10: strip, not a body; noted in D7 |
| P1 columns | open columns, no padding, 31 / 25 px gap | gap 24 / 19.2 | PASS |
| sibling gaps: P3 panes 28.2, setup panes 28.2, P4 evidence / measure 35.2, /agents tables 28.2, cells 23.5, P1 31.3 | 1280: 19-24 | 24 / 19.2 | PASS |
| title -> main: pain-k -> stat 11.7, gate cell rows 11.7, fact-h -> rule 17.8, measure-q -> p 11.7, ex-card (11.7) | 1280: 9.6 | 12 / 9.6 | PASS (0.978 x floor at 1440) |
| list rows 9.8 / 8.0 | | 10 / 8 | PASS (0.978 x floor) |
| stat figure -> label: `.tstat`, `.bignum` 7.8 / 6.4; `.pain-stat` -> sub 11.7 | | 8 / 6.4 | PASS |
| status bar above card bottom | 20 / 16 | 20 / 16 | PASS on every card, dive, explainer |
| main -> support: P1 quote -> source 5.9 / 4.7; P4 quote -> source 5.3 / 4.2; P4 tile label -> source 7.8 / 6.4; A3 rule row title -> sub 1.8; setup agent label -> message 3.5 / 2.8; setup tool header -> body 7.5 / 6.8; dive hint kicker -> text 10 / 8 | | 16 / 12.8 (12 / 9.6 for title -> text) | FAIL (D4) |
| dive hint box padding | 13.7 v / 23.5 h; 11.2 / 19.2 | 28 / 24; 22.4 / 19.2 | FAIL vertical (D5) |

## Ranked defects

### D1 · High · §08 setup card: right transcript pane arrives after a 0.21 dead zone, card is half empty at the dock
- Spec: §0c blocks "one starting every 0.04 ... from 0.52 to 0.88"; "no scroll dead zone where nothing happens for more than 0.10 of m between 0.40 and 0.88"; the card should dock assembled (m 0.667).
- Observed: the left `.hook` pane mounts 0.53-0.60 (10 text elements, 5 surfaces) and then nothing changes until m 0.82. The right pane (`.setup-session`, the transcript) has opacity 0 until m ≈ 0.82; its surface tints at 0.84-0.86, and the transcript lines (8 lines, prompts, hook boxes, verdicts, agent text) plus the status bar all land between 0.88 and 0.90 (text count 12 at 0.88, 15 at 0.89, 23 at 0.90). At the dock (m 0.667) and for the next 0.15 of m, the right half of the card is empty.
- Measured: no-change gap 0.21 (0.60 -> 0.81) at 1440x900 and 1280x720 (`r11-qa-raw-setup-1440.json`, `...-1280.json`). Every other card has a largest gap of 0.01-0.07 (the 0.1 slide gap before the kicker aside).
- Screenshot: `designs/reviews/qa-shots/r11-qa-setup-m0.80.png` (title + hook pane, right half blank), `r11-qa-setup-m0.88.png`, `r11-qa-setup-m1.00.png`.
- Repro: fresh load `/`, T = `.setup`'s `.hold` top (13500 at 900), scrollY = T - 900 + m*1350 for m 0.60, 0.70, 0.80, 0.84, 0.90; watch the right pane.

### D2 · Medium-High · §08 transcript: tool-call bodies never become visible
- Spec: §0b/§12 P5 (a scrubbed transcript replay); §0c "status bar last"; the content is the story (the surviving mutants, the new tests, `killed 12 survived 0 exit 0`).
- Observed: the three `.tool-b` bodies (`SURVIVED billing/pricing.py:6 0 -> 1 ...`, `+def test_coupon_never_goes_negative(): ...`, `killed 12 survived 0 exit 0`) stay at opacity 0 at every m from 0.5 to 1.0, at the bottom of the page and after the `/#hook` landing. The tool boxes appear at m 0.90 (opacity 1) as 99 px / 82 px / 82 px boxes holding one header line and a blank 33-50 px interior; on screen they read as empty or unfinished boxes. The dive renders the same kind of box with its body visible.
- Measured: `.setup .tool-b` opacity 0/0/0 at m 0.5, 0.7, 0.84, 0.88, 0.9, 0.95, 1.0 and at max scroll; heights 50 / 33 / 33.
- Screenshots: `r11-qa-hook-landing.png`, `r11-qa-setup-1280-m0.95.png`, `r11-qa-bottom-home-1440-max.png`.
- Repro: `/`, scroll to the bottom, evaluate `[...document.querySelectorAll('.setup .tool-b')].map(b => getComputedStyle(b).opacity)`.

### D3 · Medium · Support text shows before its box (two blocks)
- Spec: §0c text-before-box, steps (1) words, (2) container >= 0.02 later, (3) support text 0.02 after that; §14.1.
- Observed, P4 right card (`.measure`): the title, prose, the `$ coderefinery tests --changed` output and the muted ember `preview · ships with the tests gate` line are all visible at m 0.74 while the pane background and seam are at 0 until 0.77-0.78 (0.833 at 0.78) and the bar label / note / prose support only at 0.80. `.measure-prev` is support text (muted, a footer-style line) and arrives with the words, before the box.
- Observed, /math crap card 1 (`#crap .ex-card`): at e 0.74 (m-equivalent 0.49) the title, prose, rule text, the `rule` label and the `CC = 1 + 8 = 9` caption are all visible, with no rule box, no card top rule, no code-pane frame and no square slab; those arrive at e 0.78-0.80. Support (`rule`, caption) is not 0.02 after the box; it is 0.05 before it. Same order on dry, mutate, tests cards (same component).
- Measured: `r11-qa-trk-p4-1440.json` (measure: main 0.73, surf 0.77, sup 0.73); `r11-qa-trk-mcrapf-1440.json` (card1: main 0.73, surf 0.73 for the first seam sample, sup 0.73); visual `r11-qa-math-crap-e0.74.png` vs `r11-qa-math-crap-e0.80.png`.
- Repro: fresh load `/`, scrollY for `.tests` m 0.72, 0.74, 0.76, 0.78, 0.80 and read `getComputedStyle(document.querySelector('.tests .measure-prev')).opacity`, `.measure` `--bx`; /math: scrollTo(0, 0.74 x 900), then 0.80 x 900.

### D4 · Medium · Main -> support gaps well below the 16 px floor, and a few title -> text gaps below 12
- Spec: §0c Breathing room (main text to support text 16 px; title to main text 12 px).
- Observed (1440x900 / 1280x720): P1 quote -> source line 5.9 / 4.7; P4 quote -> source 5.3 / 4.2; P4 stat tile label -> source link 7.8 / 6.4; A3 rule-table row, rule name -> muted line 1.8 / ~1.5; §08 transcript `agent` label -> message 3.5 / 2.8, tool header -> body 7.5 / 6.8; dive hint kicker -> hint text 10 / 8 (floor 11.7 / 9.6).
- Screenshots: `r11-qa-p4-m0.88.png` (quote and tile source lines), `r11-qa-setup-1280-m0.95.png`.
- Repro: `getBoundingClientRect` bottom of `.pain-col .quote p` vs top of `.quote cite`; `.tstat-l` vs `.tstat-s`; `.rrow span` vs `.rrow .ash`; `.setup .agent .agent-k` vs `p`; `.hint-k` vs `.hint-p`.

### D5 · Medium · Dive hint boxes and the dive outputs table are compacted
- Spec: §0c "Cards, tiles, table cells, panes and hint boxes are never visually compacted", padding 28 px (24 on narrow tiles), >= 80 % at 1280.
- Observed: `.hint` padding 13.7 / 23.5 px (vertical / horizontal) at 1440x900 and 11.2 / 19.2 at 1280x720, i.e. vertical padding at 49-50 % of the 28 / 22.4 floor. In the K3 outputs `before` table the header words `Survived` and `Uncovered` touch (`SurvivedUncovered`), a column gutter of about 0 (`r11-qa-dive-K3-settle-1440.png`); the cell padding of the other tables is 14 / 18.
- Repro: scroll to dive K1-K4 t = 0.85 (6300 + (k + 0.85) x 900 at 900 px) and read `.hint.on` computed padding.

### D6 · Low · Floors are 2.2 % under at the 1440x900 reference size
- Spec: floors are stated at 1440 x 900.
- Observed: `--u` is 0.978 at 1440x900, so `--pad-card` 27.4 (28), `--pad-tile` 23.5 (24), title gap 11.7 (12), row 9.8 (10), support 15.6 (16), cell 13.7 / 17.6 (14 / 18), sibling 23.5 (24), fig -> label 7.8 (8). At 1280x720 the floor is exactly the 0.8 clamp. Probably the usable width (scrollbar or the clamp's base). Visually immaterial; recorded for the numbers.

### D7 · Low · Pane bars carry 10 px vertical padding, `.hook` pane bar wraps
- Pane bars (home P3 / P4 / setup, agents, dive) are 10 v / 27 h. As strips they read fine, but the home setup `.hook` bar wraps `.claude/settings.json (Cursor: .cursor/hooks.json)` and `step 2 · add to your project` to two lines at both sizes (`r11-qa-hook-landing.png`).

### D8 · Low · P4 changes after m 0.90 (bridge typing)
- Spec: §0c "nothing changes 0.90-1.00".
- Observed: `$ coderefinery crap --changed` types at m 0.91-1.00 in P4's CTA (documented in IMPLEMENTATION.md round 10, §5a). Prompt and command keep the space (7 px). Kept as a spec conflict to resolve; behaviour is as the implementer describes.

### D9 · Low · The last card's kicker row is clipped at the bottom of the page
- At max scroll the last card is docked with the footer directly under it, but the card sits 52 px (1440) / 60 px (1280) above the viewport top, so the `§08` / A4 / M5 kicker (row 1 of the card) is off-screen at the very bottom on all three routes (`r11-qa-bottom-home-1440-max.png`). 100 px above max the card is exactly docked with its kicker.

### D10 · Low · Plain-text values and commands still left in running text
Sampled 114 wrapped `.ic` and 35 candidates left plain (not counting survey percentages in P1 source bullets such as `96 %`, `48 %`, which are citations, not values from the formulas):
- /math rule callouts: `exit 2` (crap 3), `0.8` (dry 3), `exit 3` (mutate 3), `kills 0` (tests 1), `50 %` (crap 2); the visual caption `cov = 0.20 · (1 − 0.20)³ = 0.512` (crap 2); `$ npx skills add coderefinery` in M5.
- /agents: `price()` in the reviewer quote and in the agent line (`Splitting price() into three functions`), `7.2 MB`, `kills 0` / `kills 5` in the verdict table cells, `npx skills add coderefinery` in the A4 install line.
- home: `7.2 MB`, the three `$ ...` install commands in the §06 facts, `(preview)` in the K4 hint.
The dive hints and agent readings, the math card prose, the setup list, P2 and /agents A0 are fully wrapped.

### Notes, not defects
- P4's headline wraps with a one-word second line (`fail.`) at 1440 (`r11-qa-p4-m0.88.png`); §06's two-line headline is balanced.
- Setup at 1280x720 is the tightest card: 15 px between panes and the status bar, 18 px between the headline and the panes.
- Section-level gaps (kicker -> headline up to 134 px on P1 / 180 px on A4) are by design (middle row centred).

## Verdict

Fix first. Round 10/11 work is solid: the runway timing is exact on five of six cards at both sizes (kicker 0.40-0.43, headline 0.40-0.56, blocks spread, status 0.88-0.90, nothing before 0.40), the page bottom leak is gone on all three routes at both sizes (only the last card and the footer are ever hit, the dive's covered state renders nothing), the dive and math entrances assemble before the dock, dry 3 lands `DUPLICATE` molten, the bridge keeps its `$ `, `/#hook` lands finished, docs and github differ, the deck stacks at 0.96 / -24 px / 70 %, every card, dive state and math state fits at both sizes, the console is clean and no frame exceeded 22 ms. It is not shippable yet: D1 (setup card half empty with a 0.21 dead zone, transcript crammed into the last 0.02) and D2 (tool-call bodies never visible, leaving blank boxes in the finished setup card) are visible to every reader of the last card; D3 (support before box on P4's right card and the /math cards), D4 (main -> support gaps of 2-8 px against a 16 px floor) and D5 (hint boxes and a dive table header compacted) break rules the round was meant to enforce. D6-D10 are polish.
