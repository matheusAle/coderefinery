# QA · site · round 8 (live page, http://localhost:5173/, the mobile build, §17)

Method: agent-browser CLI, private session `qa8`, live DOM only (no site code edited, nothing committed). Touch emulation with `site/.tools/touch.mjs` (CDP, `pointer: coarse` and `hover: none` both true, confirmed with matchMedia and `data-mode`), real touch taps with `qa-shots/r14-qa-tap.mjs` (CDP `Input.dispatchTouchEvent`). Viewports 390x844, 360x800 (phone, mode `phone`), 768x1024 and 844x390 (touch, mode `tablet`), then 1440x900 and 1280x720 without touch (mode `desktop`). Entrance `e` positions: `scrollY = cardTop - vh*(1 - 0.6 e)`, 450-500 ms settle, each card sampled on a fresh load (once mounted a card stays mounted). Scripts, raw output and screenshots: `designs/reviews/qa-shots/r14-qa-*`.

## Pass / fail

| # | Check | 390x844 | 360x800 | 768x1024 | 844x390 | Evidence |
|---|---|---|---|---|---|---|
| 1 | No horizontal document scroll, 12 positions, 3 routes | PASS | PASS | PASS | PASS | scrollWidth == innerWidth at all 12 on `/`, `/agents`, `/math`; 0 elements outside the viewport that are not in a scroller (`r14-qa-overflow.js`) |
| 1b | Fade on every clipped edge | FAIL | FAIL | n/a | n/a | scrollers with a mask: transcripts, `a-lines`, `measure-out`, `term-stack`, `hook-json`; the 11 `/math` code panes (`.cp-lines`) have none (D3) |
| 2a | Cards natural height >= 100svh, relative, no transform, 1 px seam | PASS | PASS | n/a | n/a | home cards 1026-1914 px (390), 1027-2180 (360), dive states 1958; all `position: relative`, none transformed, 1 px between wrappers |
| 2b | Kicker first, status bar last, dot with text | PASS | not sampled | n/a | n/a | child order checked on all cards on the 3 routes; 10 status bars wrap to 2-3 lines, the dot stays on the first line |
| 2c | One column, tiles two per row, P3 stacked pairs | PASS | not sampled | n/a | n/a | hero 2x2, S9 2x2, P4 2+1 (lone third tile, D11), P3 cells 348 px wide stacked |
| 2d | Hero order, stream only in the band, foot line inside the hero | PASS | PASS | FAIL | FAIL | 390: h1, lede, install, band, numbers 2x2, foot, status; hero 1082 px; 0 events overlap text; 360: hero 1168, band 543-683, 0 overlaps. 768 / 844: see D6 |
| 2e | Topbar: wordmark + menu, sheet, 48 px links | PARTLY | n/a | n/a | n/a | sheet 390 wide, links 48, button 57x44, closes on Escape and outside tap (real touch). On `/` the sheet is under the hero (D1) |
| 2f | `/#hook` from `/agents` | PASS | n/a | n/a | n/a | landed scrollY 17498, card top 0, title 60/60, kicker/status/blocks 1.0, every card above mounted |
| 3 | Floors at 80 % | FAIL | n/a | n/a | n/a | headlines 24/24 ok (28.0+ px, lh 1.25), formulas 27.3 px; mono floor, body floor, padding floor and tap floor have misses (D4, D7, D8) |
| 4 | Motion §17.2: P1 | PASS | n/a | n/a | n/a | nothing at e 0.20; kicker 0.30-0.34 (0.01 at 0.30, 1.0 at 0.34); title 0 at 0.30, 10/48 at 0.34, 36/48 at 0.45, 48/48 at 0.48; status 0.005 at 0.92, 0.993 at 1.00 |
| 4 | Motion: P4 | PASS | n/a | n/a | n/a | kicker 0 at 0.30, 1 at 0.34; title 10/47 at 0.34, 23 at 0.40, 35 at 0.45, 47 at 0.48; status 0 at 0.92, 1 at 1.00 |
| 4 | Motion: A1 | PASS | n/a | n/a | n/a | kicker 0.063 at 0.30; title 12/59 at 0.34, 45/59 at 0.45, 59/59 at 0.60; status 0.031 at 0.92, 1.0 at 1.00 |
| 4 | Motion: tall cards mount blocks on their own entrance | PASS | n/a | n/a | n/a | P1 / P4 / A1 blocks: text step 1.0 at block top 95 % of vh, support 1.0 by 82-88 %; stays mounted when scrolled back |
| 4 | Motion: math state card, dive K1 | FAIL | n/a | n/a | n/a | everything visible at e 0.20 (D5) |
| 5 | Dive on phone | PASS (3 notes) | n/a | n/a | n/a | 4 stacked states 1958 px, session pane above outputs, hint directly under the title, each state's final content, flash plays once (classes), depth bar 3 px sticky at top filling 0.005 -> 1.000, furnace ground is a CSS tint only (D10, D11) |
| 6 | Math on phone | FAIL | PASS | n/a | n/a | 11 stacked cards, formula 27.3 px wraps, active term molten, rule + prose + code pane, tags after the text (0 overlaps on 11 states) but line numbers lost on states 8-11 (D2) and no edge fade (D3) |
| 7 | Stream | PASS | PASS | FAIL | FAIL | phone: max 6 events / 9 sparks (limits 12 / 16); tablet 17-18 events (D6); tap shows the detail line for ~2.0 s (synthetic `pointerup`, see note) |
| 7b | No hover on touch | PARTLY | n/a | n/a | n/a | A3 `pointerenter` ignores touch; 5 unguarded `:hover` rules remain (D9) |
| 8 | Console | PASS | PASS | PASS | PASS | no errors or warnings, only `[vite]` debug lines |
| 8b | Frame times scrolling with the slag field on | PASS | n/a | n/a | n/a | 390: `/` 396 frames median 16.7 p95 16.8 worst 16.8; `/agents` 207, worst 16.8; `/math` 395, worst 16.9; 0 frames over 18 ms; `__shaderLog`: slag fields `running`, no css-fallback; the 18 ms guard did not trigger in headless; `__streamCost` avg 0.10 ms, max 0.9 ms (reported, not judged) |
| T | Tablet 768x1024 | PARTLY | n/a | no overflow, dive and math readable | no overflow, hero not clipped | stacked layout kept (declared); hero is not as on the phone (D6); math cards good (`r14-qa-math-768.png`), dive readable (`r14-qa-dive-K1-768.png`) |
| L | Landscape 844x390 | PASS | n/a | n/a | PASS | hero 1087 px tall, scrolls, nothing clipped, no overflow on 3 routes (`r14-qa-hero-844x390.png`) |
| R1 | Desktop: every card fits | PASS | n/a | n/a | n/a | 1440x900 and 1280x720, 3 routes, 11 runway cards at m 0.7 / 0.95: scrollOver 0, docX 0, content bottom 880 / 704 (section 900 / 720); math pinned cards sweep: no settled overflow (two transient `ex-row` bottoms 935 / 910 at 1440 are the incoming card still sliding in) |
| R2 | Desktop: page bottoms | PASS | n/a | n/a | n/a | 3 routes x 2 sizes: last card bottom 847 / 660, footer 847-900 / 660-720, docX 0 |
| R3 | Desktop: runway timing, 2 cards (P1, A1) | PASS | n/a | n/a | n/a | both sizes: kicker first 0.42, full 0.44; title 0.42 -> 0.56; status 0.90 (0.02 sampling); identical to round 7 |
| R4 | Desktop: deck stacking | PASS | n/a | n/a | n/a | P1 under P2: `.deck-dim` 0 -> 0.09 -> 0.18 -> 0.27 at e 0 / 0.3 / 0.6 / 0.9; the section transform reads `none` on this sample (scale not re-measured) |
| R5 | Desktop: console | PASS | n/a | n/a | n/a | clean |

## Ranked defects

### D1 · High · Home: the open menu sheet sits under the hero, links 2-5 cannot be tapped
- Spec: §17.1 Topbar ("the nav opens as a full-width sheet under the topbar").
- Observed: on `/` at scrollY 0 the sheet (opaque `rgb(11,12,14)`, `z-index: auto`, topbar `z-index: 2`) is painted behind the hero content: the hero headline, "Prove it." and the lede draw over the link labels. `elementFromPoint` at the centre of each link: "why" ok, "for agents" hits `SPAN.hero-line`, "the math" hits `H1.hero-h1`, "docs" `SPAN.hero-prove`, "github" `P.hero-lede`. A real touch tap at "the math" (200, 213) did nothing and closed the sheet (URL stayed `/`). `/agents` and `/math` are fine (all 5 links hit). The sheet also starts 8 px under the topbar (84 vs 76).
- Screenshot: `designs/reviews/qa-shots/r14-qa-menu-390.png`. Script `r14-qa-menuhit.js`.
- Repro: 390x844 touch, open `/`, tap `menu`, tap any link below "why".

### D2 · Medium-high · /math (phone): line numbers vanish and row tints stop when the pane is scrolled right, states 8-11
- Spec: §17.2 ("line numbers stay fixed while scrolling the pane", brief check 6; round 14b claim "every tinted row at least the pane's width").
- Observed: rows are only as wide as the pane (min 348 px) while the widest row is 720 / 851 / 747 / 747 px on states 8-11 (mutate 2/3, 3/3, tests 1/2, 2/2). The sticky `.cp-n` is bounded by its own row, so at full scroll 6, 12, 5 and 5 line numbers have left the pane (state 9, `scrollLeft` 503: numbers 5, 7, 8, 10, 11 and 13 gone, first number at x -126). States 1-7 are fine (scroll <= 266 < 348). Tinted rows also end at 348 px, so the row tint stops mid-scroll.
- Screenshot: `r14-qa-math-s9-scrolled-390.png`. Script `r14-qa-math.js`.
- Repro: 390x844 `/math`, state `#crap`..`#tests` index 8, scroll `.cp-lines` to the right end.

### D3 · Medium · /math code panes have no fade on the clipped edge
- Spec: §17.1 (wide content "scrolls horizontally inside its card with a fade on the clipped edge").
- Observed: all 11 `.cp-lines` panes scroll sideways (scrollWidth 490-851 vs 348) with `mask-image: none` and no `::before` / `::after` gradient on the pane or the line container (`r14-qa-fade.js`); the screenshot shows the cut text with no fade. Transcripts, `a-lines`, `measure-out`, `term-stack` and `hook-json` do have a mask.
- Screenshot: `r14-qa-math-s2-e0.45.png` (annotation tags cut at the right edge), `r14-qa-math-s9-scrolled-390.png`.

### D4 · Medium · Mono and body type floors missed on phone
- Spec: §17.1 (mono >= 12.5 px, body >= 15 px; stream pills exempt).
- Observed at 390 after mounting (96 mono text nodes on `/`, 30 on `/agents`, 58 on `/math`), by size: 8.8 px (`agent` labels, "Kent Beck · June 2025" cite), 9.35 (pain cite links, 3), 9.6-9.57 (`code.ic` in P3, `sline` lines, diff lines), 10.2 (`fcell-g`, `pnum-l`), 10.4 (P3 table cells: LLM vs gate, 20+ nodes; `cta-s`), 11.05 (terminal command lines, `fcell-s`, `a-install-s`, `strong` step labels), 11.9 (kickers `§06`, `bignum-l`, `cta-btn`, `exit`), 11.96 (inline `code.ic`, 17 on `/`, 12 on `/math`; §0b gives 0.92 em of the body mono, so at the 13 px base it is under 12.5), 12.0 (pane bars, `● live`, `tool-h`). Body face below 15 px: 13 px transcript messages (`Refactored for clarity...`, `Thanks! Merging.`, 3 on `/`, 3 on `/agents`), 13 px `vdoes` (8 on `/agents`), 13.6 px Kent Beck quote, 14.45 px P3 lede (`p.lede`).
- Screenshot: `r14-qa-p4-tiles-390.png` (the 8.8 px cite and the 11-12 px pattern list). Script `r14-qa-floors.js` + `r14-qa-floors2.py`.

### D5 · Medium · Dive states and math state cards skip the §17.2 mount order
- Spec: §17.2 first bullet (slide only <= 0.30, kicker 0.30-0.33, headline 0.30-0.48, blocks 0.45-0.92, status 0.92-1.00), brief check 4.
- Observed: dive K1 (and K2-K4) and every `/math` state card show kicker, title, formula, panes, status and gauge at full opacity at e = 0.20 (all 8 samples identical, `title ch178/178`). P1, P4 and A1 follow the order exactly. The dive paragraph says "each state shows its final content", so the dive may be intended; the math paragraph only exempts the formula value, which "rolls on entrance". The cards also give no entrance sign at all (no kicker typing, no block reveals). Flag for a spec decision; if intended, §17.2 should say these two sections are exempt.
- Screenshots: `r14-qa-math-s2-e0.2.png` (state 2/3 fully drawn with its top 100 px on screen), `r14-qa-dive-K1-0-390.png`.
- Minor within the passing cards: P4's first block (`tstats`) reveals at e 0.40 while the headline is still typing (own-entrance rule for tall cards, as designed).

### D6 · Medium · Tablet and landscape hero is not "laid out as on a phone"
- Spec: §17 preamble and §17.1 Hero; brief "hero as on phone" at 768x1024; §17.2 (touch rules at any width: <= 12 events).
- Observed at 768x1024 and 844x390 (mode `tablet`, touch): the foot line sits directly under the install box and before the numbers; the four numbers are one column of four full-width rows, not 2x2; install box 315 px, not full width; the stream runs behind the text and the numbers (no band, `clip-path` empty): 18 / 17 events alive (limit 12 on touch), 3 / 1 events overlap text boxes, and a pill is visible across "3-120x"; hero 1087 px, status bar at the bottom. No overflow, not clipped (landscape scrolls).
- Screenshots: `r14-qa-hero-768.png`, `r14-qa-hero-844x390.png`.
- Also tablet: no depth bar and no gauge in the dive (`.dive-depth` removed, `.gauge` `display: none`, only the 4 px per-state progress bar). Not covered by the declared deviation (which only names the bar position on the phone).

### D7 · Low-medium · Padding floors (20 px, 16 px narrow tiles) missed on several surfaces
- Spec: §17.1 (card and pane padding 20, narrow tiles 16, cells 12 x 14).
- Observed (computed padding, 390): `.install` 12 / 14 / 12 / 14; `.gate-cell` 16 (348 px wide, not a narrow tile); `.hint` 16 (dive); `.loop-step` 16 (A1); `.pane-bar` 11.2 top and bottom; `.cp-lines` 11.2 top and bottom; `.pain-col` top 11.5. At 20: `.cta`, `.hook-json`, `.measure-body`, `.a-cell`, `.pane-bar` sides, sections (20 side, 24-32 top). Tiles 16 (`.tstat`) are within the narrow-tile floor. P3 and table cells: the cell check found no `td`/`th` (cells are `span`s with 0 padding inside the stacked pair); gap between pair rows 16.
- Script `r14-qa-pad.js`.

### D8 · Low · Tap targets under 44 px
- Spec: §17.1 (>= 44 x 44; the round 14 note claims a 44 px hit area on status-bar and source links).
- Observed (bounding box): P1/P4 source links 196x36, 258x36, 95x36 (`staff engineer`, `DORA`, `Blacksmith` cites, 36 px high); status-bar source links 78/70/148/117/125 x 41-42 and `2026` 31x42, `/agents` 55x42, `/why` 31x42; `Kent Beck · June 2025` 111x12. Menu button 57x44, nav links 48, install 48, cta 48 are fine.

### D9 · Low · Hover still styled on touch
- Spec: §17.2 ("No hover anywhere on touch").
- Observed: 5 `:hover` rules outside any `@media (hover: hover)` guard on all three routes: `a:hover` colour molten, `.wordmark:hover`, `.cta-btn:hover`, `.fcell:hover::after`, `.fcell:hover` border. A tap leaves the sticky hover colour. Note (not a defect): on A3 a tap on a mutant row pairs it with its rule through `focus` (2 `.lit` rows after a real tap) although `pointerenter` from touch is ignored.

### D10 · Low · Phone dive has no furnace shader; `__flashLog` is not fed on the phone path
- Observed: `.furnace-host` has no canvas on phone, `__shaderLog` lists only the two slag fields, so the "furnace ground" is the CSS tint + `.heat-glow` (visible warm ground in `r14-qa-dive-K1-0-390.png`); §17.2 says "the furnace stays". `DivePane.play()` flashes through CSS classes only, so `window.__flashLog` stays empty (length 0 after the whole page). Verified through class mutations instead: on K1's entrance (`data-played` set once) both panes get `flash-slag` at +16 ms (exit 2) and `flash-ingot` at +900 ms (exit 0), nothing on scrolling away and back (`r14-qa-flash.js`).

### D11 · Low / cosmetic
- Dive session pane is 894 px tall with about 200 px blank under the last line (`r14-qa-dive-K1-700-390.png`).
- P4 stat tiles: 3 tiles in a 2-column grid leave an empty half cell (`r14-qa-p4-tiles-390.png`).
- Math state cards: stacked prose cards reserve the tallest height (376 px for 299-358 px content) and the status bar is 80-105 px tall, so the card feels airy.
- Document height moves by 19 px while scrolling `/` at 390 (19126 -> 19145), i.e. a small layout shift as reserved reveals settle.
- The open sheet starts 8 px below the topbar (see D1).

### Notes on method limits
- Stream tap: the event band moves faster than a CLI round trip (0.5-1 s), so real CDP taps always landed on empty space after the first 3 lucky ones; with a synthetic `pointerup` (pointerType touch) fired in the same tick on an event, the detail line showed and hid at about 1.75-2.0 s. The real pointer events do reach the page (`pointerdown`, `pointerup`, `click`, pointerType `touch`).
- The 18 ms frame guard did not trigger in headless; the median frame is the 60 Hz vsync (16.7 ms), so this measures nothing about a real mid-range phone.
- 360x800 was run for overflow, hero, card heights and routes; the floor and motion sweeps were done at 390.

## Verdict

**Fix first.** The phone layout is solid (no horizontal scroll anywhere, cards natural height with seams, hero order and band, the dive and the math stacks, performance, console), but D1 makes the menu unusable on the home page, D2 and D3 break two explicit /math code-pane requirements, D4 is a broad type-floor miss (96 mono nodes under 12.5 px on `/`), and D6 leaves tablet / landscape with the old hero and an over-limit stream. D5 needs a spec ruling for the dive and math cards. D7-D11 can follow.
