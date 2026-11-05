# coderefinery site · QA round 3 (screenshots prefixed `r4-qa-`)

Live pages only (`/`, `/agents`, `/math`) in headless Chromium via agent-browser; no site source read. Viewports 1440×900 (primary), 1280×720, 1920×1080, 900×900, plus `prefers-reduced-motion: reduce` emulated. Evidence: DOM reads, computed styles, scroll positions, rAF samplers, screenshots in `qa-shots/` (paths relative to `designs/reviews/`). Probe helpers kept in `qa-shots/r4-qa-*.js`. Smooth scroll is on in CSS, so every scrollY below was set with `behavior: auto`.

Geometry (1440×900): home cards at 0/900/1800/2700/3600/4500, dive 5400 (4500 tall, pin 400 vh), numbers 9900, math teaser 10800, page 11771 (constant at every scroll step). /agents 0/900/1800/2700/3600, page 4571. /math hero 0, crap 900 (2700), dry 3600 (2700), mutate 6300 (2700), tests 9000 (1800), parity 10800, page 11771.

## Pass/fail

| # | check | result | evidence |
|---|---|---|---|
| A-D16 | fade band from live rects, 1280 and 1440 | PASS | 22 s overlap samples. 1440: max event opacity over `.hero-lede` 0, over status bar 0, over install 0.18, over foot line 0.416 (full near-layer opacity is 0.50, so no full-opacity pill). 1280: lede 0, status 0, install 0.23, foot 0.41, numbers 0.01. `qa-shots/r4-qa-hero-band-1440.png`, `r4-qa-hero-band-1280.png` |
| A-D4 | RM: preview footer only under K4 | PASS | RM dive at 5400/5800/6300/6700/7200/7600: the 4 `.term-preview` nodes for K1-K3 are opacity 0; K4's is 1.00 / visible from 8100 to 9300. `r4-qa-rm-dive-6300.png`, `r4-qa-rm-dive-8500.png` |
| A-D12 | card clip ≤ 12 % of mount range; number rolls after its card is open | PASS (note) | Fresh-load sweeps at 0.02 e steps. 10 %→90 % of the clip: P1 col0 e 0.48→0.52, col1 0.58→0.60, P2 col 0.56→0.60, P3 table 0.46→0.50, i.e. 0.04 e = 6.7 % of the 0.60 range. Note: col0 has a slow tail 344 → 361 px up to e 0.66. Rolls: every `[data-odo]` first changes 0.02-0.03 e after its card reaches `--r ≥ 0.95` (P1 0.52 → 0.55, 0.61 → 0.63, 0.69 → 0.72; P4 0.55 → 0.58; numbers 0.50 → 0.53) |
| A-D13 | M5 numbers roll, 36/36 last | PASS | /math card 5: open 0.55; first roll 99.3 % at 0.58, 100 % at 0.59, 99.5 % / 100 % at 0.60, 36/36 at 0.62 (last). Digits end `3/6/3/6`, `9/9/3`, etc. |
| A-D14 | `__shaderLog` and `__flashLog` on all routes; flash on each exit landing in the dive | PASS | both are objects on `/`, `/agents`, `/math`. Scrubbing the dive forward logged 8 flashes: b0 slag at y 6075 (exit 2), a0 ingot 6175, b1 slag 6975, a1 ingot 7075, b2 slag 7875 (exit 3), a2 ingot 8000, b3 slag 8750, a3 ingot 8925; each at the scrollY where the exit line reached full opacity. Note: `firstFrameMs` is `null` for 4 of 5 shaders (only the hero slag field reports 0.2 ms), so the 8 ms guard is still not auditable |
| B stream | spawn 0.35-0.7 s | PASS | 1440: 28 spawns in 14 s, max gap 700 ms; 1920: 25 spawns, gaps 340-701 ms, avg 566 ms (none below 340) |
| B | rise ≈ 42 px/s | PASS | near layer 42.0-42.1 px/s (mid 29.4 = 0.7x, far 18.9 = 0.45x) |
| B | full hero width at 1920, not a column | PASS | spawn x 35 → 1728, 8-bin histogram `4,4,2,4,1,5,4,1`; pills on both sides of the 1312 px content column. `r4-qa-hero-stream-1920.png` |
| B | ≤ 30 events, ≤ 40 sparks | PASS | max 21-23 events, 13-30 sparks |
| B | failures blink with halo; sparks fade ≈ 1.6 s | PASS | `.fail` animations `ev-blink, ev-halo`; sparks live up to 1.57 s |
| B | hover: pause, full opacity, molten seam, detail, reason, settle | PASS | hovered near event: y frozen at 881.8 for 0.9 s, opacity 1.00, border `rgb(255,106,26)`, detail `ev-d` "PR #491 · opened 7 min ago · 4 checks · 1,204 tests" under the pill; on leave opacity 1 → 0.50 in ~300 ms and rising resumes. Failure: animation `none`, `ev-d` "PR #512 · exit 3 · survived 2 · pricing.py:6, :9". `r4-qa-stream-hover.png`, `r4-qa-stream-hover-fail.png` |
| B | sparks freeze with the hovered event | NOT VERIFIED | sparks are not linked to their event in the DOM; other events' sparks keep moving so a global compare is inconclusive |
| B | no hover on touch; numbers block keeps priority | NOT VERIFIED | no touch pointer in this setup; no event ever overlapped the numbers box in 22 s (it ends at y 606, events there are at ≈0 opacity). Install box verified: 126 overlap samples at (465,726), 0 `hov` |
| B | parallax: 3 layers, scale/opacity | PASS | scales 1.0 / 0.85 / 0.7; max opacity 0.50 / 0.35 / ≈0.22 |
| B | scroll shift 1.0 / 0.6 / 0.3 | PASS | scrolled 300 px, rise subtracted: k = 1.00 / 0.60 / 0.30 on every sampled event |
| B | pointer shift ±12 / 6 / 2 | PASS | pointer left edge → right edge: 24 / 12 / 4 px total |
| B | `__streamCost` | PASS | avg 0.09-0.10 ms, max 1.2 ms (n ≈ 1000-9400 frames) |
| B | RM static, ~20 events | PASS | 20 events (6/9/5 per layer, scales kept), 0 sparks, 0 animations, identical transforms 2 s apart. `r4-qa-stream-rm-1440.png` |
| C | sticky top 0, next slides over | PASS | `/`, `/agents`, `/math`: every unpinned card `position: sticky; top: 0`; the next card covers it |
| C | covered card ≈ 0.96, up 24 px, 70 %, hidden once covered | PASS | e = 0.25 / 0.5 / 0.75 / 0.95 → scale 0.990 / 0.980 / 0.970 / 0.962, ty −6 / −12 / −18 / −22.8, dim overlay 0.07 / 0.15 / 0.23 / 0.28; `visibility: hidden` at e = 1. Identical on all three routes. `r4-qa-home-deck-450.png`, `r4-qa-agents-deck-450.png` |
| C | 1 px top seam on incoming card | PASS | card `::before` 1 px `#26282E`; visible in `r4-qa-handoff-5150.png` |
| C | mount over e ∈ [0.30, 0.90] on the incoming card | PASS | home P1 (kicker 0 at 0.3, 11/18 at 0.4, h2 done 0.6, cards 0.5-0.7, status 0.42 at 0.8, 1.00 at 0.9), P3, /agents A2 (kicker 20/49 at 0.4, status 1.00 at 0.9), /math M5 (cards 0.6-0.8). Nothing before 0.30; identical at 0.9 and 1.0 |
| C | scrolling back works | PASS | 3000 → 1000 → 450: hero visible again at 0.98 / −12; back at 0 it is 1.0 / 0; 9000 → 4900 → 4500 restores P5 |
| C | P5 → K1 hand-off | PASS (note) | the dive card slides over P5; code pane arrives from x 48 → 88 with opacity 0.5 → 1.0 and the title types ("Too complex for it" mid-entrance). The session pane no longer slides out to the right: it is covered (x 605 → 608). `r4-qa-handoff-5150.png`, `-5400.png` |
| C | dive and explainers slide over, then pin | PASS | crap card pins at top −1800 while dry slides over it (`r4-qa-math-deck-3150.png`); dive over P5 as above |
| C | RM stacks without scale/shift/dim | PASS | all three routes: sc 1, ty 0, dim 0.00 at every e; still sticky and hidden at e = 1 |
| C | document height unchanged | PASS | 11771 / 4571 / 11771 at every step in all modes (was growing 540 px in round 2) |
| D | `§XX` kicker top row, status bar bottom edge, content centred | PASS (minor deviations below) | 1440: kicker 38..55 on all 5 non-hero home cards, 5 /agents cards, M5; status 832..862 (gap to bottom 38) on 7 of 8 cards; content above/below gaps equal on P1-P4 (125/125, 86/86, 48/48, 25/25) and A1-A4. 1280: kicker 25..42, status 668..695 everywhere except P5. Heroes carry the topbar (kicker at 92 on /agents, 90 on /math). Dive: kicker top row with title beside it (documented deviation). Math explainers: kicker 36..53 on all states. `r4-qa-home-dock-9900.png`, `-10800.png` |
| E | fit 100 vh, nothing outside card, no overflow at 900 | PASS | 1440 and 1280: every unpinned card scrollHeight == clientHeight, 0 elements outside the card on all routes; pinned dive and explainers 0 elements outside the viewport (only the decorative `.heat-glow`, 260 px above). 900: `scrollWidth` 900 at every 300 px step on all three routes |
| F | dive full bleed at 1280 / 1440 / 1920 | PASS | `.furnace-host`, glow and `canvas.gl` rects exactly 0,0 → vw,vh (1280×720, 1440×900, 1920×1080); no coke margins in `r4-qa-dive-bleed-1920.png`, `-1440.png`, `-1280.png` |
| F | stacked states at 900 | PASS (minor) | each stage 0..900 full width, ground is furnace through all four states (`r4-qa-dive-narrow-900-8200.png`); see D19 |
| G | headline and card-title line height 1.25; kickers single line | FAIL (1 element) | computed 1.250 on hero h1, every `h2.slot-title`, `.dive-title`, `.ex-title`, `.gate-q`, `.measure-q`, `.cta-h`; **exception: the `§06` headline "Same output as the originals. A fraction of the time." is 1.000** (D17). Kickers: one line at 1440 and 900 on all routes |
| H | order and copy | PASS | /math: 281 of 289 reference lines present verbatim (the 8 others are the `2/3`, `3/3` counters, which roll, and the `●` glyph rendered by CSS); `/`, `/agents` differ only by typed dive strings split across spans, the hero stream's own events, and the `->` to `→` arrow swap accepted earlier |
| H | nav active | PASS | `aria-current="page"` and molten on "for agents" (/agents) and "the math" (/math); none on `/` |
| H | console | PASS | no errors or warnings on any route (info lines about shader guards only) |
| H | long frames while scrubbing | PASS | dive 5000 → 10000 in 6 s: 359 frames, avg 16.7 ms, p99 20 ms, max 23.5, 0 > 34 ms. Math 800 → 10800 in 8 s: 478 frames, avg 16.7, max 19, 0 > 34 ms |

## Defects, ranked

No blockers.

### Major

**D17 · `§06` headline line height is 1.0, not 1.25.** Spec: §0b "Title line height is 1.25 for headlines (display sizes 44 px and up)". Observed: `h2.h2.slot-title.typed-title` "Same output as the originals. A fraction of the time." (54.8 px) computes `line-height: 54.8px` (1.000); every other headline is 1.25. The two lines visibly touch compared with the others. Screenshot `qa-shots/r4-qa-home-dock-9900.png`. Repro: `/`, 1440×900, scrollY 9900 (and the same at 1280×720, scrollY 7920); read `getComputedStyle(document.querySelector('#numbers h2')).lineHeight`. Note fixing it adds about 27 px of height to a card with 299 px of free space above the content, so fit is not at risk.

### Minor

**D18 · P5 (`§05`) and the hero status bars sit lower than every other card's.** Spec §0c "Card layout, fixed rows … rows never move between cards". Observed at 1440×900: status bar 832..862 (38 px above the bottom) on P1-P4, A1-A4, numbers, math teaser, but 840..870 (30 px) on P5 and 845..873 (27 px) on the hero; at 1280×720 25 px vs 17 px (P5) vs 22 px (hero). P5's content is also off-centre by 8 px (35 above, 43 below). `qa-shots/r4-qa-home-dock-4500.png`. Repro: `/`, scrollY 4500 (1440×900), compare `.slot-status` top with scrollY 1800.

**D19 · Narrow (900 px) dive: the depth gauge overlaps the panes and the furnace canvas covers only the first stacked state.** Spec §13.1 "under 1024 px … each stacked state is full-bleed". Observed: ground colour is furnace everywhere, but `.furnace-host` and its canvas are 0..900 × 6545..7779 only (the first stage of 4936 px), and the gauge label "surface" is clipped by the pane's left edge at x ≈ 56. `qa-shots/r4-qa-dive-narrow-900-8200.png`, `-6700.png`, `-10000.png`. Repro: `/` at 900×900, scrollY 8200.

**D20 · Math `rule` callout lost its outline.** Design `exports/M1.1-crap-count.png` draws the "rule …" line inside a 1 px seam box; live shows bare text (`r4-qa-math-state-1350.png`). Repro: `/math`, 1440×900, scrollY 1350. Also the §05 strip lacks the outer seam box that `M5-math-parity.png` has (`r4-qa-math-m5-10800.png`).

**D21 · Math explainers and M5 have no status bar.** Spec §0c puts a status bar in the bottom row of every card; the PNGs M1.1 and M5 do not draw one either, so this is a spec/design mismatch, not a page bug. Decide and align the spec. Repro: `/math` at scrollY 10800.

**D22 · Pinned cards are not dimmed or scaled when the next card slides over them** (dive → numbers, crap → dry etc.): scale 1.0, no dim; the pinned card is simply held while the next covers it (explainers) or scrolls away with it (dive, spec'd in §13.3). Spec §0c says the pinned sections are cards "the same way"; it only defines the covered treatment for unpinned cards. Probably intended, say so in the spec. Repro: `/math`, scrollY 3150 (`r4-qa-math-deck-3150.png`).

**D23 · Shader guard is still not auditable.** `window.__shaderLog` exists but only the hero slag field reports `firstFrameMs` (0.2); furnace, both verdict flashes and the discreet field report `null`, so the §14.4 8 ms rule cannot be checked. Repro: any route, `window.__shaderLog`.

**Unverified, not defects:** sparks freezing with a hovered event; touch ignoring hover; the numbers block's pointer priority (no event got behind it during 22 s).

## Verdict

**Fix-then-ship.** Everything the round-2 list left open is closed: the fade band tracks the live foot, lede and status rects at 1280 and 1440, reduced motion shows the preview footer under K4 only, the clip opens in about 7 % of the mount range and numbers roll after their card, M5 rolls with 36/36 last, both shader logs exist on all routes and the verdict flash fired on all 8 exit landings. The hero stream matches the spec on cadence (0.35-0.7 s), speed (42 / 29.4 / 18.9 px/s), full-width placement at 1920, caps, hover, parallax (1.0/0.6/0.3 scroll, ±12/6/2 pointer) and cost (about 0.1 ms), and is static with 20 events under reduced motion. Deck stacking, fixed card rows, fit, full-bleed dive, constant document height, console and frame times all pass on every route. What is left is one explicit spec miss (the `§06` headline at line height 1.0, a one-line CSS fix) and a handful of small items (P5 and hero status rows 8-11 px lower, narrow dive gauge and furnace coverage, math rule outline, shader log gaps). None blocks a visitor.
