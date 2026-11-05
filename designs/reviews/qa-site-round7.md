# QA · site · round 7 (live page, http://localhost:5173/, re-check of the round 12 fixes)

Method: agent-browser CLI, private session `qa7`, live DOM only (no site code edited, nothing committed). Viewports 1440x900 and 1280x720. Positions use `scrollY = wrapperTop - vh + m*1.5*vh` (entrance `e` for pinned sections), every sample waited 380 ms (sweeps) to 1.5 s (single measures) after the jump. Helper scripts, raw JSON and screenshots: `designs/reviews/qa-shots/r12-qa-*`.

## Pass / fail

| # | Check | Result | Evidence |
|---|---|---|---|
| D1 | Setup runway: hook 0.52, transcript 0.635, steps 0.75, no dead zone > 0.10, status 0.88-0.90, nothing after 0.90 | FIXED | First text of each block (0.01 sampling) at 0.53 / 0.64 / 0.75, surfaces 0.56 / 0.67-0.69 / 0.79-0.81; largest no-change gap inside 0.40-0.88: 0.03 (1440 and 1280); status 0.89 -> 1.00 at 0.90; zero changes 0.90-1.00 (`r12-qa-raw-setup-1440.json`, `-1280.json`) |
| D2 | Setup tool bodies visible at m=1, page bottom, `/#hook` landing | FIXED | `.setup .tool-b` opacity 1/1 at m=1, at max scroll, and after clicking "see the hook ->" from /agents (settled: scrollY 13500, card top 0, title 60/60, status 1, 11/11 lines opacity 1). Two bodies now (the passing run is a one-line, as the implementer said), heights 51 / 34 at 1440. `r12-qa-bottom-home-1440-max.png`, `r12-qa-hook-landing-settled.png` |
| D3a | P4 right card: support >= 0.02 after box | FIXED | measure: words 0.70, box 0.73, `.measure-prev` + note + bar label 0.80 (+0.07) (`r12-qa-trk-1440.out`) |
| D3b | Dive K1 (session / outputs / before) | FIXED | words 0.73-0.74, box 0.79 / 0.82 / 0.83, support 0.87 (+0.04 to +0.08) |
| D3c | /math first card: rule label, captions, code bar, line numbers | PARTLY FIXED (see D11 below) | code box 0.78-0.84, code bar + line numbers + visual caption 0.85-0.88 (OK, >= +0.02); the `rule` label is still visible at 0.73-0.74 while the rule box draws 0.77-0.83 |
| D4 | Main -> support gaps >= 16 / 12.8 | FIXED | 1440: P1 quote->source 16.5 x3, P4 quote->source 16.5, P4 tile label->source 16.5 x3, setup agent label->message 16.5, setup tool header->body 16.5 x2; 1280: 13.19 for all of them (floor 12.8). A3 rule rows are side by side (same row, column gap 16.5 at 1440, rows 65 px). Dive hint kicker->text 13 / 10 (floor 12 / 9.6) |
| D5 | Dive hint padding, K3 table gutter | FIXED | hint padding 24/24/24/24 at 1440 (19.2 at 1280 = 0.8 x 24) for K1-K4; K3 table column gap 18 px (Survived 1118-1171, Uncovered 1189-1248), `r12-qa-dive-K3-settle-1440.png` |
| D6 | `--u` = 1 at 1440x900 | FIXED | `--u` = 1.000 (0.783 at 1280x720, the 0.8 clamp is applied through the floors) |
| D7 | Pane bars 14 px, setup hook bar one line | FIXED | all six visible pane bars 14/14 at 1440 (11.2/11.2 at 1280); hook bar height 44 / 35 = one line: `.claude/settings.json · Cursor: .cursor/hooks.json` |
| D9 | Last card = 100vh - footer, kicker on screen, footer directly under | FIXED | max scroll, 3 routes x 2 sizes: card top 0, bottom 847 (1440) / 660 (1280), footer 847-900 / 660-720; hits in the viewport grid: only the last card and the footer; 50-300 px above max only the last card (`r12-qa-bottom-*.json`, `r12-qa-bottom-home-1440-max.png` shows the kicker at the top) |
| D10 | Prose values / commands wrapped | FIXED except one | 75 inline code spans in prose (home 16, /agents 8, /math 51), 9 + 1 + 6 candidates left; of those only `returns 0.0` (home P4 pattern list) is a real leftover; the rest are survey percentages in P1 sources (citations), `$ gh pr checks --watch` (hero stream label), `CC` in the hint kicker and the math formulas (display type), word phrases `coderefinery gives`, `(two minutes)`, `(preview)` |
| R1 | Every card, dive state, math state fits, both sizes | PASS | 6 home + 4 /agents cards at m 0.45 / 0.7 / 0.95: scrollOver 0, docX 0, status bar 20 / 16 px above card bottom; tightest: setup 1280x720 content->status 15 px, P4 17 (1440). Dive K1-K4 x 0.5 / 0.85: panes end <= 806 (900) / 658 (720), status 849 / 676; math 11 states: content <= 635 / 539 |
| R2 | Page bottoms clean, 3 routes x 2 sizes | PASS | see D9 |
| R3 | Console | PASS | only `[vite] connecting/connected`, no errors |
| R4 | Frame times scrubbing whole pages at 40 px / frame | PASS | `/` 351 frames, worst 24 ms, p95 22; `/agents` 137, worst 23; `/math` 373, worst 22; 0 frames over 50 ms, 0 longtasks |
| R5 | Deck stacking | PASS | P1 under P2: scale 1 -> 0.988 -> 0.976 -> 0.96, translateY 0 -> -7.2 -> -14.4 -> -23.97 px, `.deck-dim` 0 -> 0.09 -> 0.18 -> 0.30 (70 %), hidden at e = 1 |
| R6 | Hero stream | PASS | text 797 -> 953 chars in 2.5 s, `__streamCost` n 941, avg 0.18 ms, max 1.4 ms |
| R7 | Reduced motion | PASS | at m 0.55 kicker 1, title complete, status 1, blocks present on P1, P3, P4, setup; tool bodies opacity 1 |
| R8 | Runway regression, other cards | PASS | all 10 cards, both sizes: nothing at m <= 0.395, kicker 0.41-0.43, status 0.89-0.90, no gap over 0.06 (see D12 for /agents A1, A4); P4 still changes 0.91-1.00 (bridge typing, old D8, unchanged by the round) |

## Ranked remaining defects

### D11 · Medium · /math first card: the `rule` label (support) shows before its box
- Spec: §0c text-before-box, step 3 (support text >= 0.02 after the container); §14.1.
- Observed: on `#crap .ex-card:nth-of-type(1)` the `rule` label and the rule text fade in together at e 0.73-0.74 (label opacity 0.25 at 0.73, 1.0 at 0.74); the rule box's seam only draws 0.77 (dashoffset 1 px) to 0.83 (0%). The label shows 0.03-0.09 before its box. Same component on the dry, mutate and tests cards. The visual caption (`CC = 1 + 8 = 9`), code bar and line numbers are correctly after their boxes (0.85-0.88 after code box 0.78-0.84).
- Measured: `r12-qa-mathfine-1440.json`, `r12-qa-mathfine2-1440.json`, seam sample in the log above; `r12-qa-math-crap-e0.74.png` (label, no box) vs `r12-qa-math-crap-e0.82.png` (box drawn).
- Repro: /math at 1440x900, `T = #crap top` (900), scrollY = T - 900 + e*900 for e 0.72 / 0.74 / 0.78 / 0.83; read effective opacity of `#crap .ex-card .ex-rule > span` and the `.ex-rule rect` `stroke-dashoffset`.
- Note: the implementer's "box 0.62-0.68, support 0.70-0.74" does not match the live page on this e basis (round 6 measured the same 0.73 / 0.78-0.80); the card's words and support are unchanged from round 6, only the caption, code bar and line numbers are now right.

### D12 · Low · Setup card at 1280x720 is legible but small
- Spec: §0c "never visually compacted"; type scales with clamp.
- Observed at m = 1: body mono 8.64 px (hook JSON, transcript lines, steps), labels 9.36 px, smallest 7.9 px (the `agent` label and tool-call secondary), headings of the tool boxes 13 px. It reads as a clean, uncrowded card (panes padded, 15 px to the status bar, no overlap, nothing clipped), but the hierarchy is steep: 13 px tool headers over 8.6 px bodies, and sub-9 px mono is hard to read on a real 1280x720 screen. Not compacted by padding, compacted by type size. At 1440x900 it is comfortable.
- Screenshot: `designs/reviews/qa-shots/r12-qa-setup-1280-m1.png`; 1440 `r12-qa-bottom-home-1440-max.png`.
- Repro: 1280x720, /, scrollY = `.setup` hold top - 720 + 1.5 x 720, read font sizes of `.setup` leaf text.

### D13 · Low · A1 verdict table card has a dead zone 0.65 -> 0.89 (pre-existing)
- Spec: §0c "no scroll dead zone > 0.10 between 0.40 and 0.88".
- Observed: `/agents` `.a-verdicts` (one block: the table) is complete at 0.65, then nothing changes until the status bar at 0.89 (gap 0.24 at both sizes). `.a-everywhere` has a 0.10 gap (0.65 -> 0.75 at 1440, 0.65 -> 0.76 at 1280) and `.a-reliable` 0.06. Round 6 did not sweep A1; the layout is single-block, so a later start (blocks spread to 0.75) would fix it.
- Repro: `r12-qa-runcard.sh http://localhost:5173/agents .a-verdicts a-verdicts-1440`, `r12-qa-raw-a-verdicts-1440.json`.

### D14 · Low · One plain value left in prose
- Home P4 pattern list: "passes if the function returns 0.0" (`returns 0.0` not wrapped in inline code), the only real leftover from the D10 sample.

### D8 (carried) · Low · P4 changes 0.91-1.00
- The bridge command typing in P4's CTA, a documented implementer choice; unchanged.

### Cosmetic
- Setup hook JSON: the wrapped `"command"` string continues at column 0 (`9.9999 && coderefinery dry ...`) instead of the indent, at both sizes (`r12-qa-setup-1280-m1.png`).
- /agents A1 text: first dive sample at t = 0.15 of K1 was taken while the smooth scroll was still travelling (status bar at 1591); ignored, K1 measured at 0.5 and 0.85.

## Verdict

Ship after one small fix. Round 12 fixed D1, D2, D4, D5, D6, D7, D9 and D10 as specified (measured on the live page at both sizes), the P4 and dive halves of D3, and it caused no regression: every card, dive and math state fits, page bottoms are clean on three routes, console clean, worst frame 24 ms, deck 0.96 / -24 px / 70 %, hero stream and reduced motion fine. The one real miss is D11 (the `rule` label on the /math cards appears before its rule box); D12-D14 are polish and can follow. The setup card at 1280x720 reads clean and uncramped, but its type is 8-9 px (D12): legible, not comfortable.
