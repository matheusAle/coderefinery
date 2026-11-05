# QA · site · round 10 (live page, http://localhost:5173/, round 17: the deck on phones and tablets, §17.4)

Method: agent-browser CLI, private session `qa10`, live DOM only (no site code edited, nothing committed, dev server untouched). Touch emulation with `site/.tools/touch.mjs` (CDP, `pointer: coarse`, `data-mode` = `phone` at 390 / 360, `tablet` at 768 / 844x390), real touch taps and swipes through `tap.mjs` / `swipe.mjs`, 4x CPU throttle with `throttle.mjs` (checked: a 3e7-iteration loop took 79 ms throttled vs 21 ms not). Desktop run on a fresh browser with touch off (`data-mode` = `desktop`, coarse false). Geometry and motion probes reuse the implementer's `deck17.js` (copied to `r17-qa-deck.js`), but every number below was also read from computed style or `elementFromPoint`, and the visual read is my own. Entrance `e` sampled with `scrollY = cardNaturalTop - vh*(1 - 0.6 e)`, ascending on one load. All scripts, raw output and screenshots: `designs/reviews/qa-shots/r17-qa-*`.

## Pass / fail

| # | Check | 390x844 | 360x800 | 768x1024 | 844x390 | Evidence |
|---|---|---|---|---|---|---|
| 1a | Every card sticky; <= viewport pins at 0, taller pins by its bottom | PASS | PASS | PASS | PASS | computed `position: sticky`, `top` = `svh - cardH` on all cards but the last (hero -246, P2 -1122, K1 -898 at 390); `r17-qa-geo.js`, `r17-qa-run-*.txt` |
| 1b | Every pixel of every tall card reachable | PASS | PASS | PASS | PASS | per card: face top 0 at its natural top, face bottom exactly = viewport bottom, still there at the end of the hold. `/` 12 of 12 tall at 390 / 360 / 844x390, 10 at 768; `/agents` 5 / 5 / 3 / 5; `/math` 13 / 13 / 1 / 13. One probe flag: the last card of `/` at 390 / 360 (growth, see D-1), not a reach failure |
| 1c | ~50 svh hold before the next card's top appears | PASS | PASS | PASS | PASS | hold = 422 / 422 (390), 512 / 512 (768) on all cards that have a successor (one 425 / 515: P4 grew 3 px); `r17-qa-holds-390.txt`, `-768.txt`. Exception: the hero, see D-3 |
| 1d | Hero first, never arrives; last card not sticky, footer under it | PASS | PASS | PASS | PASS | hero = unit 0, `hold.no-hold`; last card `position: relative`; bottom probe at max scroll and 50 / 100 / 200 / 300 px above: foreign hits 0, earlier cards visible 0 (390, 360, 844x390 all routes); 768 `/agents` 300 px above the end: A3 still partly visible under arriving A4 (mid-slide, by design); clean from 200 px |
| 1e | Sticky continuity (no jumps) | PASS | n/m | PASS | n/m | `/` cards 1-10, 60 samples each from 25 % before arrival to the end of the hold: face top equals the expected `-(y - T)` then `vh - fh`, max error 0.0 px (`r17-qa-cont.txt`) |
| 2a | Covered card: scale 1 -> 0.96, up 16 px, dim 70 % | PASS | PASS | PASS | PASS | at incoming-top travel f: 0.09 s 0.9964 / -1.44 px / dim 0.027; 0.5 s 0.980 / -8.0 / 0.150; 0.95 s 0.962 / -15.2 / 0.285; 1.0 hidden. Identical on tall card (P2, 1966 px), dive K1 / K2, math 1/3 -> 2/3, `/agents` A1 -> A2, 768 and 844x390 (`r17-qa-motion-*.txt`) |
| 2b | Covered card hidden and inert, real tap lands on the incoming card | PASS | n/m | PASS | PASS | after dock: `visibility: hidden`, `inert`, `data-hidden`; 12 real CDP taps on a 3x4 grid per case, 60 of 60 clicks land in the incoming card (P3 over P2, K2 over K1, math 2/3 over 1/3 at 390; P3 over P2 at 768; math dry 1/3 over crap 3/3 at 844x390; `r17-qa-covtap.txt`). `elementFromPoint` 5x9 grid at 61 positions ascending and 4x8 at 61 descending: hits on hidden cards 0, hidden-not-inert 0 |
| 2c | At most one covered card partly visible | PASS | PASS | PASS | PASS | max 1 at every position, ascending and descending (`r17-qa-rev.txt`) |
| 2d | Incoming: scale 0.97 -> 1, brightness 0.55 -> 1, molten seam cooling | PASS | PASS | PASS | PASS | scale 0.970 -> 0.985 -> 1.000 and overlay 0.45 -> 0.225 -> 0 over f 0..0.18 (= e 0..0.30); `--seam-glow` 1 -> 0.75 -> 0.50 -> 0.25 -> cleared; seam colour at 1 px height goes `srgb(.63,.30,.14)` -> `srgb(.149,.157,.180)` = `--seam` (`r17-qa-seam.js`) |
| 2e | §17.2 slots: slide, kicker, typed headline, blocks text -> box -> support, status last | PASS (tall: see D-2) | n/m | n/m | n/m | P1 (390): e 0.30 nothing, kicker 0.34, title 10/48 -> 48/48 by 0.5, blocks 0.4-0.6, status 0.95-1.0. Dive K2 and `/math` 2/3: kicker 0.34, title types 0.34-0.5, `ent-r` 0.5, box 0.6, sup 0.8, status 1.0; identical on both, as round 9 (`r17-qa-ent-390-*.txt`) |
| 2f | Tall cards mount below-fold blocks on their own entrance | PASS | n/m | n/m | n/m | P2 (1966 px) at 390: block 1 mounts with the card, block 2 at card top -270 (e 2.2), block 3 later; status already 1.0 at e 1.6 (D-2) |
| 3a | Menu sheet above everything, closes when its card starts being covered | PASS | PASS | PASS | PASS | open sheet: all 5 links hit-test to themselves while on screen at 390 / 360 / 768 on 3 routes; open at scroll 0, scroll across the cover start: closes at the first covered scroll step (390 `/`: open at 246, closed at 260; 768: closed at scrollY 1; 844x390: open until the hero's bottom pin ends, closed by 1100) (`r17-qa-menu*.txt`, `r17-qa-menu-390.png`) |
| 3b | `/#hook` from `/agents`, hashchange `#why` land exactly with everything above mounted | PASS | n/m | PASS | n/m | click on the `/#hook` link at `/agents`: `scrollY` 21104 = natural top (390), 18571 (768), target top 0, 12 of 12 cards above fully mounted (every `--f/--r/--sup/--ent-*` >= 0.99); `location.hash = '#why'`: 1090 (390) / 1024 (768) = natural top of P1, cards 0-1 mounted; docX 0 (`r17-qa-hash.txt`) |
| 3c | Real swipe scrolls a code pane inside a covered, transformed card, page does not move | PASS | PASS | n/a | n/a | `/math` crap 2/3 pane under crap 3/3 at 390 (face scale 0.994): scrollLeft 0 -> 221 (= max), `scrollY` 3210 unchanged; 360: 246 of 251, `scrollY` unchanged. 768 / 844x390: pane does not overflow, nothing to swipe |
| 3d | Stream tap still shows the detail line | PASS | n/m | PASS | n/m | tap on a near / mid event: `.ev.hov` with `PR #470 . opened 31 min ago . 2 checks . 1,204 tests` (390), failure detail `exit 1 . 3 failed . test_pricing.py:41, :57, :88` (768); gone after 2.4 s (`r17-qa-stream.txt`). Far-layer events are not hoverable by design |
| 4a | `scrollWidth === innerWidth` at 12 positions, 3 routes | PASS | PASS | PASS | PASS | 12 of 12 zero on all 12 combinations (`r17-qa-run-*.txt`); 61 positions in the probe also zero |
| 4b | Page bottom clean (last card + footer) | PASS | PASS | PASS | PASS | see 1d; last card bottom then footer, `last@max` -1074 / footer 580 at 390 |
| 5a | Worst frame scrolling each route (30 px per frame, headless 60 Hz) | PASS | PASS | PASS | PASS | worst 16.8 ms on all 12 combinations (16.9 on `/math` 768), p99 16.8, over-17.5 count 0 |
| 5b | 4x CPU throttle: frames over 33 ms and where | PASS (note) | PASS (note) | PASS | PASS | 390 `/`: 1 frame at 33.4 (30 px/frame), 3 frames at 33 (12 px/frame, scrollY 2268 = P2 entrance, 17508 and 18948 = §06 and §07 mounts); 360 `/`: 2 at 33 (18510, 19950 = §06, §07 mounts); `/agents`, `/math`, 768, 844x390: 0. None above 33.4 ms, none during a slide-only range; the headless frame is 60 Hz, not a phone (`r17-qa-throttle-390.txt`) |
| 5c | Shader fallback guard | PASS | PASS | PASS | PASS | every field `running` (hero and discreet) before and after, also under throttle; guard never triggered |
| 6 | Sweeps vs round 9 baseline | PASS | PASS | n/a | n/a | below |
| 7 | Reduced motion | PASS | n/m | PASS | n/m | below |
| 8 | Visual read | PASS | n/m | PASS | n/m | below |
| R | Desktop regression | PASS | PASS | | | below |

### 6 · Sweeps (390 and 360; run with the deck flattened as the implementer did, because hidden cards fail hit-tests by design)
`r17-qa-floors-flat.js` = round 9's `r15-qa-floors.js` preceded by `main.deck-measure` and a rule that un-hides covered cards; baselines `r15-qa-floors-*.json`.

| route | headline < 28 | body < 15 | mono < 12.5 | tap < 44 | pad bad / total | cell bad / total |
|---|---|---|---|---|---|---|
| `/` 390 / 360 | 0 (r15 0) | 0 (0) | **0 (r15 6)** | 0 of 28 (0) | 42 / 64 (42 / 64) | 6 / 24 (6 / 24) |
| `/agents` | 0 (0) | 0 (0) | 0 (0) | 0 of 12 (0) | 12 / 21 (12 / 21) | 30 / 57 (30 / 57) |
| `/math` | 0 (0) | 0 (0) | 0 (0) | 0 of 12 (0) | 37 / 54 (37 / 54) | 0 / 0 |

Change from round 9: the 6 typed nodes under the mono floor on `/` (round 9's D-A: P2 gate commands at 11.05 px, P5 diff lines at 9.6 px) are gone, 0 now. Same bad-padding counts; the one "new" `div.statusbar.*` / `div.mtable.box` row per route is a 0.2-1 px measurement wobble on the same element. Headline count on `/` is 12 against 24 in round 9 (0 bad both times), the stacked dive no longer exposes its non-current titles. Without the flattening the sweep reports 8-12 false tap failures per phone (links in covered or hidden cards), which is the deck working.
Tablet (not floors by spec, §17: desktop type; listed for completeness): at 768 and 844x390 mono < 12.5 is 371 / 109 / 586 nodes and body < 15 is 18 / 22 / 11 on `/`, `/agents`, `/math`; tap failures 2 per route, both in the topbar (D-4).

### 7 · Reduced motion (`set media light reduced-motion`, `matchMedia` true)
At f 0 / 0.09 / 0.5 / 0.95 on P3 over P2 and K2 over K1 (390) and math 2/3 over 1/3 (768): covered scale 1, shift 0, dim 0, incoming scale 1, dim 0, no seam glow, `will-change` never set; hidden at f 1 and `reach` all true, probes 0 hits on hidden cards, max 1 covered visible, on 3 routes (`r17-qa-reduced.txt`). Cards stack (all `position: sticky`). Content complete on entry (P1 at e 0.05: title 48/48, blocks 1, status 1).

### 8 · Visual read
Shots: `r17-qa-shot-{390,768}-{home,agents,math}-NN-{slide,top,bot}.png` (every card mid-slide at card top 55 % of the viewport, docked, and pinned by its bottom), contact sheets `r17-qa-sheet-*.png`, enlargements `r17-qa-view-home-390-{a,b}.png`, `r17-qa-view-s08-768.png`. Read all 12 / 5 / 13 cards at 390 and 768. Nothing broken: no sticky jump (see 1e), no overlap between a card and the one covering it, no clipped text except the intended code-pane scrollers with their fade, kicker first and status last on every card, the covered card dims and sinks under the incoming card, the dive's thin depth bar stays at the top through the four states, math states read as formula / prose / pane / status stacks. §08 at 768 (was 4,510 px, now 1,146 px confirmed): one column, settings file above the terminal, full width, readable, status line at the bottom (`r17-qa-view-s08-768.png`). §08 at 390: 1587 -> 1654 px (D-1). In the 768 sheets some "bot" cells are empty only because those cards are exactly one viewport tall (nothing to pin), not a rendering failure.

### R · Desktop regression (touch off, `data-mode` desktop)
| Check | 1440x900 | 1280x720 | Evidence |
|---|---|---|---|
| Every card, dive state, math state fits | PASS | PASS | home 7 holds, `/agents` 4, `/math` 1: all ok (sections 873 / 822 and 698 / 640); pinned sweep only the known transient `ex-row` bottoms (935 / 910 and 779 / 738 / 749 / 745), same numbers as round 9 (`r17-qa-desktop-1440.txt`, `-1280.txt`) |
| Page bottoms clean | PASS | PASS | last card bottom 847 / 660, footer 847-900 / 660-720, docX 0, 3 routes |
| Runway timing P1, A1, P4 | PASS | PASS | kicker 0.42 -> 0.44, title 0.42 -> 0.56, status 0.90 on P1, A1 and the §06 numbers card at both sizes; identical to round 9 (`r17-qa-runway.txt`) |
| Deck values unchanged | PASS | n/m | covered 0.994 / -3.6 px at f 0.15 .. 0.968 / -19.2 at 0.8 (-24 px and 0.96 at 1.0), dim 0.30 at 1.0, incoming slide over e 0..0.60 (glow 1 -> 0 by f 0.6), hidden at f 1; 9 desktop cards |
| Console | PASS | PASS | only `[vite]` debug lines on desktop and on the touch run, no errors or warnings |

## Ranked remaining defects

### D-1 · Low · §08 card grows 67 px on phones when its typed lines type; document height not stable
- Spec: §17.4 (heights settle, deck tolerates one-off svh jumps), round 9 D11c (document height stable while scrolling). Declared by the implementer.
- Observed: at 390 and 360 the last card goes 1587 -> 1654 px (1672 -> 1739 at 360) when the P5-style typed tool lines wrap to three lines; `scrollHeight` 22955 -> 23022 at 390 and 23881 -> 23947 at 360, changing at `scrollY` about 20760 / 21600. 768 and 844x390: 0 growth. Harmless to the deck (last card is not sticky and the 1d / 4b probes are clean), but it moves the footer 67 px while someone is reading near the bottom, and it is the only height change left after round 9's D11c fix. Also what trips the probe's `reach` flag on the last card (`bottomAt` 911 / 867).
- Screenshot / data: `r17-qa-grow.txt`, `r17-qa-run-390.txt` (bad row on §08), `r17-qa-shot-390-home-11-top.png`.
- Repro: 390x844 touch, load `/`, run `r17-qa-grow.js`.

### D-2 · Low · Status bar mounts before the below-fold blocks of a tall card
- Spec: §17.2 / §17.4 (status last; tall cards mount below-fold blocks on their own entrance).
- Observed: the status bar still mounts on the card's own e 0.92-1.00. On P1 (1484 px) at 390 `status --f` is 0.99 at e 1.0 while columns 2 and 3 are at `--r` 0 (they mount at e about 1.5 and later); on P2 (1966 px) status 1.0 at e 1.6, block 2 mounts at about e 2.2, block 3 after. The bar sits at the card's bottom, offscreen at that time, so nobody sees it animate, but the order "status last" is not met and its fade is spent unseen.
- Data: `r17-qa-ent-390-P1.txt`, `r17-qa-ent-390-more2.txt` (card 2 rows).
- Repro: 390x844, `r17-qa-ent2.sh 390 844 "" 2 0.5 1 1.6 2.2 2.6`.

### D-3 · Low · The hero has no hold
- Spec: §17.4 ("every card is sticky inside a wrapper ... holds for 50 svh after it pins"; only the last card's wrapper has no runway).
- Observed: every card except the last holds 422 / 512 px (= 50 svh), the hero holds 0: P1 starts sliding over it the moment its bottom pins (at 768, hero = 1024 = viewport, P1's top moves at `scrollY` 1). Implementation notes say "the hero has no runway (unchanged)" but it is not in the declared deviations. Desktop hero behaves the same way.
- Data: `r17-qa-holds-390.txt`, `-768.txt` (rows `0:`).

### D-4 · Low · Tablet topbar links `why` and `docs` under 44 px wide
- Spec: §17.1 tap targets >= 44, "touch applies the touch rules at any width".
- Observed at 768x1024 and 844x390 with coarse pointer, all 3 routes: `a "why"` 23 x 44 (hit area 24 x 44) and `a "docs"` 31 x 44 (32 x 44); other nav links are wide enough. Phones are fine (menu sheet). Round 9 did not measure tablet taps, so not shown to be a regression.
- Data: `r17-qa-floors-tablet.txt`.

### Notes, not defects
- Declared deviations checked and acceptable: brightness is a black overlay on a child of the card (computed opacity 0.45 -> 0 incoming, 0 -> 0.30 covered, seam stays full colour above it; no visible banding in any shot); scale origin at the visible part's top-centre (`50% cardH - svh`, e.g. 195px 246px for the hero, 1122px for P2): no sideways or vertical shift of the visible part in any shot or in the continuity probe; frame budget is headless only.
- Round 9 cosmetics C1-C4 were not re-examined.
- The address-bar (svh) one-off jump cannot be exercised in headless Chrome.
- `/agents` at 768 bottom probe, 300 px above the end: A3 partly visible under arriving A4 is the slide in progress, not a leak.

## Verdict

**Ship.** The phone / tablet deck works on the live page at all four viewports on all three routes: every card is sticky with the right offset, every pixel of every tall card is reachable, each holds 50 svh before the next arrives, covered and incoming values match §17.4 exactly (and the desktop values are untouched), covered cards are hidden and inert (60 of 60 real taps land on the incoming card, 0 hits on hidden cards ascending or descending), anchors and the menu behave, a real swipe scrolls a code pane in a covered transformed card without moving the page, no horizontal overflow, bottoms are clean, frames are 16.8 ms headless and at worst a handful of 33 ms frames at content mounts under 4x throttle, reduced motion degrades correctly, and the type floors improved on round 9 (mono 6 -> 0 on `/`). Four low items remain (D-1 to D-4); none blocks. Fix D-1 and D-3 if a stable page height and a uniform hold matter, D-2 and D-4 are polish.
