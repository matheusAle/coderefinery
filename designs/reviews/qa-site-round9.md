# QA · site · round 9 (live page, http://localhost:5173/, round 15 mobile fixes re-check)

Method: agent-browser CLI, private session `qa9`, live DOM only (no site code edited, nothing committed, dev server untouched). Touch emulation with `site/.tools/touch.mjs` (CDP; `pointer: coarse` and `hover: none` confirmed, `data-mode` = `phone` at 390 / 360, `tablet` at 768 / 844x390), real touch taps through CDP `Input.dispatchTouchEvent` (`qa-shots/r15-qa-tap.mjs`, same logic as `site/.tools/tap.mjs`). Desktop 1440x900 and 1280x720 on a fresh browser with no emulation (`data-mode` = `desktop`, coarse false). Entrance `e` sampled with `scrollY = cardTop - vh*(1 - 0.6 e)`, 900 ms settle, ascending e on one load (mounted stays mounted, so ascending equals a fresh load). All scripts, raw output and screenshots: `designs/reviews/qa-shots/r15-qa-*`.

## Pass / fail (round 8 defects)

| # | Check | 390x844 | 360x800 | 768x1024 | 844x390 | Evidence |
|---|---|---|---|---|---|---|
| D1 | Menu sheet above the hero on `/`, real taps navigate | FIXED | FIXED | n/a | n/a | `elementFromPoint` hits the link on all 5 links on 3 routes; real CDP taps: 15/15 (5 links x `/`, `/agents`, `/math`) land on the right URL (`#why`, `/agents`, `/math`, `/docs`, github) at both sizes (`r15-qa-taps-390.txt`, `r15-qa-taps-360.txt`); sheet covers the hero (`r15-qa-menu-390.png`, `r15-qa-menu-360.png`) |
| D2 | /math states 1-11, rows span the full scroll width, numbers pinned | FIXED | FIXED | n/a | n/a | scrolled to the end: 0 short rows, 0 line numbers outside the pane on all 11 states; scrollWidth 490-851 vs pane 348 / 318; tint right edge overshoot 0; first number at x 41 -> 21 (= pane left + 1) (`r15-qa-rows.js`, `r15-qa-math-s9-scrolled-390.png`) |
| D3 | Edge fade on the code panes | FIXED | FIXED | n/a | n/a | all `.cp-lines`: `mask-image: linear-gradient(to right, #000 calc(100% - 20px), transparent)` (`r15-qa-fade.js`); see cosmetic C1 |
| D4 | Mono < 12.5 px / body < 15 px (stream exempt) | PARTLY | PARTLY | n/a | n/a | body < 15: 0 on all 3 routes at both sizes; mono < 12.5: `/` 6 nodes, `/agents` 0, `/math` 0 (was 96 / 30 / 58). Remaining 6: see D-A. No overflow, no overlap, no box wider than the viewport after the raise (below) |
| D5 | Entrance order (dive K1, /math states 1 and 5) | FIXED | not sampled | n/a | n/a | below |
| D7 | Padding >= 20 (16 narrow tiles, pane bars >= 14 vertical) | MOSTLY | MOSTLY | n/a | n/a | below; one miss, see D-B |
| D8 | Tap targets >= 44 | FIXED | FIXED | n/a | n/a | hit-area probe (`elementFromPoint` 30 px each way): 0 failures on `/`, `/agents`, `/math` at 390 and 360, including source links, status-bar links and cites; footer sources wrap on 4 lines, readable (`r15-qa-sheet-home-07.png`) |
| D9 | No unguarded `:hover`; A3 tap does not pair | FIXED | n/a | n/a | n/a | stylesheet walk: 4 `:hover` rules, 0 unguarded (was 5) on all 3 routes; real tap on an A3 `.mrow` -> 0 `.lit` rows (`r15-qa-a3-tap-390.png`) |
| D10 | `window.__flashLog` fed on phone | FIXED | n/a | n/a | n/a | 4 entries after K1's entrance (`phone:s0:0` slag, `phone:o0:0` slag, `phone:s0:1` ingot, `phone:o0:1` ingot, shader `css-fallback`); class mutations: `data-played` once, `flash-slag` at +10 ms, `flash-ingot` at +890 ms, no repeat after scrolling away and back |
| D11a | Session pane ends at its padding | FIXED | n/a | n/a | n/a | 4 dive session panes (721 / 684 / 748 / 902 px): 1 px blank under the last line (was ~200 px) |
| D11b | Odd last tile full width | FIXED | n/a | n/a | n/a | P4 `tstats` 174, 174, 348; /math `pnums` 174 x4 + 348 (`r15-qa-d11.js`, `r15-qa-sheet-math-05.png`) |
| D11c | Document height stable while scrolling | FIXED | n/a | n/a | n/a | `/` 18947-18948 (1 px), `/agents` 7305, `/math` 13281 (constant), docX 0 |

### D5 detail (390, `r15-qa-ent-dive-K1.txt`, `r15-qa-ent-math.txt`)

| e | Dive K1 | /math state 1 (idx 0) and state 5 (idx 4), identical |
|---|---|---|
| 0.20 | nothing (kicker 0, title 0/51 chars, panes 0, status 0) | nothing (kicker 0, title 0/20, formula / code 0, status 0) |
| 0.30 | same, kicker still 0 | same |
| 0.34 | kicker 1.0, title 10/51 | kicker 1.0, title 4/20 |
| 0.45 | title 38/51, blocks still 0 | title 15/20, blocks 0 |
| 0.50 | title 51/51, hint + both panes text step in | title 20/20, prose + rule + formula + pane in |
| 0.60 | box 0.445, support 0 | box 0.446, support 0 |
| 0.80 | box 1, support 1, status 0 | box 1, support 1, status 0 |
| 0.92 | status 0 | status 0 |
| 1.00 | status 1.0 | status 1.0 |

Order holds on both: slide (card visible, nothing else) <= 0.30, kicker 0.30-0.34, headline types 0.34-0.48, blocks text -> box -> support from 0.45, status last. Dive: the session command / lines replay once on the block's entrance (lines 9 -> 4 -> 9 over ~1 s at e 0.5, no replay on scrolling to the top and back, 0.40 pre-state hidden by the pane's 0 opacity); `data-played` set once; flash once (D10). Mounted stays mounted: after scrolling back to e 0.20, kicker / title / panes / status all 1.00 on both the dive (K1 after e 1.0) and /math state 1.

### D7 detail (computed padding, 390; `r15-qa-pad*.js`)

At or above floor: `.install` 19 block (+1 px seam = 20) / 20 sides, `.gate-cell` 20, `.hint` 20, `.loop-step` 20, `.a-cell` 20, `.measure-body` 20, `.cta` 20, `.hook-json` 20, `.tstat` 16 sides / 19 top (narrow tile floor 16), `.pane-bar` 14 / 20 on home, `/math` (11 panes) and `/agents` (3 of 4), `.cp-lines` 14 vertical, table / list cells (`.hlist-row`, `.a-prop`, `.vtable` span, `.mrow`, `.rrow`) 12 x 14 as the spec.

### Type floors and layout after the raise

Sweep `r15-qa-floors.js` (json per route and size in `r15-qa-floors-*.json`): headlines 0 bad (>= 28 px, line-height 1.25), body < 15 px 0, no document overflow at 12 scroll positions on the 3 routes at 390, 360, 768 and 844x390 (`r15-qa-overflow.txt`: `nbad` 0 everywhere, every scroller has a fade mask), no element wider than the viewport. Visual read of all 51 viewports at 390 (`r15-qa-sheet-home-00..07.png`, `-agents-00..03`, `-math-00..05`): no overlap, no clipping other than the intended in-card scrollers, raised mono reads fine.

## Tablet and landscape (touch)

| Check | 768x1024 | 844x390 | Evidence |
|---|---|---|---|
| Hero as on phone: h1, lede, install, stream band, numbers 2x2, foot | PASS | PASS | 768: h1 120-268, lede 288-336, install 356-415 (full width 720), band clip 421-561, numbers 2x2 (567-817 at 844), foot, status; hero 1024 / 967 px (`r15-qa-hero-768x1024.png`, `r15-qa-hero-844x390.png`, `r15-qa-hero-844x390-band.png`) |
| Stream only inside the band | PASS | PASS | `clip-path: inset(421px 0 calc(100% - 561px))`; 60 samples x 200 ms: 0 events overlapping text |
| <= 12 events | PASS | PASS | max alive 6 / 8 (was 17-18), sparks 5 / 6 (limit 16) |
| Dive depth bar present and filling | PASS | PASS | `.dive-depth` sticky top 0, 3 px high, full width; fill 0 -> 0.092 -> 0.319 -> 0.546 -> 0.773 -> 1.000 at 768 (`--p`), 0 -> 1.000 at 844; also on 390 (0.051 -> 1.000) |
| No horizontal overflow, 3 routes | PASS | PASS | `r15-qa-overflow.txt` |

Phone hero at 390 / 360 (re-run): events max 5, 0 overlaps with text, band clip 465-605 / 552-692, hero 1090 / 1176 px (`r15-qa-hero-390x844.png`, `r15-qa-hero-360x800.png`).

## Desktop regression (no touch)

| # | Check | 1440x900 | 1280x720 | Evidence |
|---|---|---|---|---|
| R1 | Every card, dive state and math state fits | PASS | PASS | home 7 holds, `/agents` 4, `/math` 1: scrollOver 0, docX 0, content bottom <= section bottom at m 0.45 / 0.7 / 0.95; sections 873 / 822 (1440), 698 / 640 (1280). Pinned sweep: only transient `ex-row` bottoms from the incoming math section sliding in (935 / 910 at 1440; 779 / 738 / 749 / 745 at 1280); re-sampled at 1.4 s settles, every settled row ends at 501 or above (`r15-qa-mrow.js`). Same behaviour as round 8, not a defect |
| R2 | Page bottoms | PASS | PASS | 3 routes: last card bottom 847 / 660, footer 847-900 / 660-720, docX 0 (`r15-qa-desktop-1440.txt`, `-1280.txt`) |
| R3 | Hero numbers and P4 / M tile dividers | PASS | n/a | hero 2x2: one vertical and one horizontal inner divider, outer box closed (`r15-qa-desk-hero-1440.png`); P4 3 tiles with 2 inner dividers; /math 5 tiles with 4 inner dividers and a closed outer box (`r15-qa-desk-p4-1440.png`, `r15-qa-desk-mnums-1440.png`, odometers mid-roll in the shots) |
| R4 | Runway timing P1 (and A1) | PASS | n/a | P1: kicker first 0.42, full 0.44; title 0.42 -> 0.56; status 0.90. A1 identical. Identical to round 8 (`r15-qa-runway.txt`) |
| R5 | Console | PASS | PASS | only `[vite]` debug lines, no errors or warnings |

## Ranked remaining defects

### D-A · Low-medium · Typed command and diff lines stay under the mono floor on `/`
- Spec: §17.1 (mono >= 12.5 px).
- Observed at 390 and 360 after the page has mounted: 6 mono text nodes under 12.5 px on `/`. 4 are the P2 gate commands (`coderefinery crap|dry|mutate --changed`, `coderefinery tests (preview)`) at **11.05 px**, inside `SPAN.typed` in `.gate-cmd.mono.molten` in `.gate-cell.open`; 2 are the P5 tool-call diff lines (`+def test_coupon_never_goes_ne…`, `+def test_unknown_tier_pays_fu…`) at **9.6 px**, `SPAN.typed` in `SPAN.in` in `.tool-b` in `.tool.box.sline`. The inline floor is applied to the original nodes; the typer's rebuilt `.typed` spans keep their own size. `/agents` and `/math`: 0. The 9.6 px lines are visibly tiny next to the 12.5 px lines around them.
- Screenshot: `r15-qa-sheet-home-01.png` (gate commands, left) and `r15-qa-sheet-home-07.png` (middle column, P5 tool box with the 9.6 px diff lines); data `r15-qa-floors-390-home.json`, `r15-qa-floors-360-home.json`.
- Repro: 390x844 touch, load `/`, scroll to the bottom so P2 and P5 mount, run `qa-shots/r15-qa-floors.js` (mono list).

### D-B · Low · `/agents` pane bar under the 14 px vertical floor
- Spec: §17.1 / brief D7 (pane bars >= 14 px vertical).
- Observed at 390: the pane bar of `.mtable.box` ("what the skill says / what enforces it" table, `/agents`) has padding 11.2 / 20 / 11.2 / 20. The other pane bars on the 3 routes are 14 / 20. 
- Script `r15-qa-pad4.js`; screenshot `r15-qa-sheet-agents-02.png` (left column, bottom: the bar is the slimmer one).

### Cosmetic (not blocking)
- C1: the code pane's right fade (20 px mask) stays on when the pane is scrolled to its end, so the last 20 px of the longest rows are always faded (`r15-qa-math-s9-scrolled-390.png`, tint fades at the right edge). Readable; a mask that drops at scroll end would be cleaner.
- C2: dive state titles reserve the tallest title's height; K2 and K4 (one- and two-line titles) leave about 50-60 px blank under the headline (`r15-qa-sheet-home-04.png`, `-05.png`).
- C3: preformatted output in the `/agents` verdict table wraps mid-column (`test_settle_called kills 0 unique / 0 shadowed by …`, `r15-qa-sheet-agents-01.png`).
- C4: stream pills can overlap each other inside the band (not text); inherent to the stream, 0 overlaps with text.

## Notes on method limits
- Stream tap-to-detail was not re-measured (round 8 note stands); A3 tap checked with a real CDP tap: 0 `.lit` rows.
- D5 sampled at 390 only (K1; /math states 1 and 5); the card entrance code is shared, so other states were not each sampled.
- Frame times were not re-run (not in this round's list); the headless 60 Hz vsync caveat of round 8 stands.
- Two long agent-browser calls hung mid-run (a background screenshot loop); the session was closed and relaunched, no page state was lost that matters (every check was re-run on a fresh load). Desktop checks run on a fresh browser because touch emulation survives a viewport change.

## Verdict

**Ship, with two small follow-ups.** All of D1, D2, D3, D5, D8, D9, D10 and D11 are fixed on the live page with measurements (menu taps 30/30 across two phone sizes, full-width rows with pinned numbers and fade on all 11 states, entrance order at the required `e` values, 0 tap and hover failures), tablet and landscape now follow the phone hero with <= 8 events inside the band and a filling depth bar, and desktop is unchanged (fits, bottoms, tile dividers, runway timing, console). Open: D-A (6 typed nodes on `/` under the mono floor, 11.05 and 9.6 px) and D-B (one pane bar at 11.2 px); neither breaks layout. Fix D-A before calling D4 closed.
