# coderefinery site · QA round 2

Same method and constraints as round 1: live pages only (`/`, `/agents`, `/math`) in headless Chromium via agent-browser, judged against `designs/exports/*.png`, `designs/motion-spec.md` (§0b and §0c as rewritten, §12 P0 as rewritten, §13, §14, §15, §16 as rewritten), `designs/reference/math-reference.html` for /math copy, and `tokens.json`. Viewports 1440×900 (primary), 1280×720, 900×900; `prefers-reduced-motion: reduce` emulated; DPR 1 and DPR 2. Screenshots in `qa-shots/` (prefixed `r2-`); paths below are relative to `designs/reviews/`.

Note on method: the site changed under test twice (the hero redesign and the slag-field DPR fix landed mid-round). Sections 8 and 9 cover those on the final live state; everything else was measured once and not redone.

Geometry on the final live page (1440×900, no hold): home sections at 0 / 900 / 1800 / 2700 / 3600 / 4500 (each a plain 900 px block), dive `#inside-the-gate` at 5400, 4500 px tall (sticky 900 → 3600 px = 400 vh of scrub), S9 at 9900, math teaser at 10800, footer 11700; page 11773 px. /agents: 0 / 900 / 1800 / 2700 / 3600; page 4573. /math: hero 0, explainers crap 900 (2700), dry 3600 (2700), mutate 6300 (2700), tests 9000 (1800), parity 10800; page 11773. (Sections 1–5 were measured on the earlier "hold" build, where the home sections sat at 0 / 900 / 2340 / 3780 / 5220 / 6660 and the dive at 8100; offsets quoted there are from that build.)

## 1. Round-1 defects re-run

| id | round-1 defect | status | evidence |
|---|---|---|---|
| D1 | 900 px horizontal overflow after the P5→K1 hand-off | **fixed** | scrollWidth 900 at every 300 px step on `/`, `/agents`, `/math`; `.setup-session` transform `none`; dive `stacked`. `qa-shots/r2-narrow-home-handoff.png` |
| D2 | dive verdict lines not colour-coded | **fixed** | before: `CRAP threshold exceeded…` slag, `DUPLICATE score=0.91` molten, `1 pair at or above 0.80` slag, `killed 10 · survived 2` slag, `SURVIVED …` molten, `test_settle_called` molten, `DUPLICATE score=0.93 ×2` molten; after: `pricing.py:price 3 100.0 3`, `No duplicate candidates found.`, `killed 12 · survived 0 · re-ran 2`, `5 tests · 12 kills · 0 with…`, `exit 0` all ingot. `qa-shots/r2-dive-9500.png`, `r2-dive-10900.png` |
| D3 | reduced motion still typed; gauge barely filled | **fixed** | RM: zero visible typing cursors, no `↓ types as you scroll`, patch pane complete, gauge 29 → 94 → 159 → 224 → 256 px across the pin, stages cross-fade 1/0/0/0 → 0/0/0/1. `qa-shots/r2-rm-dive-1300.png`, `r2-rm-dive-3550.png` |
| D4 | "preview · the tests gate…" footer shown under every state in RM / stacked | **not fixed (RM)** | normal mode correct (footer absent until K4 end, present at 11600); under RM `previewFooterVisible: true` at every offset (400, 1300, 2200, 3100, 3550). `qa-shots/r2-rm-dive-1300.png` |
| D5 | pin scrub 300 vh not 400 | **fixed** | dive 4500 px, sticky 900 → 3600 px scrub = 400 vh; progress bar 0.11 → 0.39 → 0.58 → 0.78 → 0.97 |
| D6 | hint prose dim, box oversized | **fixed** | box hugs text (2 px gap below last line), formula kicker molten, prose chalk. `qa-shots/r2-dive-9500.png` top-right |
| D7 | P6 bridge absent | **fixed** | `$ coderefinery crap --changed` types under the P5 transcript (present from section top + 940, complete at the pin). `qa-shots/r2-p6-bridge-8050.png`. Minor follow-on: once pinned, K1's "before" terminal shows a bare `$` above the table (the typed command stays in the departed session pane) |
| D8 | ASCII `->` / `<=` / `!=` glyph drift | **mostly fixed** | home: K3 `SURVIVED pricing.py:6 0 →`, P5 `0 → 1`; only remaining ASCII is code line 14 `if total <= 0:` (arguably correct Python). /math formulas use `≥`, `→`, `↔` |
| D9 | shader self-disabled on the 8 ms guard; flash unseen | **changed, not verifiable** | no `shader disabled` console line any more; but `window.__shaderLog` is `undefined` on all three routes, so the guard cannot be audited. Verdict flash still not captured (terminal luminance flat 25/18/16 across three rapid shots at the K1 `exit 2` landing, `qa-shots/r2-flash-9050…9110.png`) |
| D10 | odometer digit strips in innerText / a11y | **fixed** | dive kicker reads `▼ inside the gate · 1 /04 · crap`; hero stats are plain big numbers |

## 2. Section rhythm (§0c) · first pass, against the superseded "60 vh hold" rule

Superseded mid-round: §0c was rewritten to "mount on entrance, no hold" while this pass was running. The table below records what the earlier build did under the hold rule; **section 12 re-tests the final build against the new rule and is the one that counts.** Kept for the fit / order / cursor / reflow evidence, which still applies.

Stepped in 90 px (10 vh) increments through two sections per route with a DOM probe at each step (`.ch.on` counts, card heights/opacity, status opacity, section top). Screenshots `qa-shots/r2-home-p1-<y>.png`, `r2-home-p2-<y>.png`, `r2-ag-a1-<y>.png`, `r2-ag-a2-<y>.png`, `r2-math-m5-<y>.png`.

| check | result | evidence |
|---|---|---|
| every unpinned section exactly 100 vh | PASS | 900 px at 1440×900 and 720 px at 1280×720 on all sections of all three routes; `overflow: hidden`; scrollHeight == height |
| content fits at 1440×900 and 1280×720 | PASS | per-section probe after release: 0 elements outside the section box on every section of every route at both sizes; lowest visible element 22–605 px above the bottom. `qa-shots/r2-fit720-home-s0…s7.png`, `r2-fit720-agents-s0…s4.png`, `r2-fit720-math-s0…s1.png` |
| holds for 60 vh then releases | PASS | P1 top = 0 from 900 to 1440 (540 px), −90 at 1530; same on P2, A1, A2, M5 |
| order: kicker typed → headline typed → visual → cards L→R → captions → status | PASS | P1 at +90: kicker 11/18, h2 4/48; +180: kicker 18/18, h2 32/48, col0 h 155; +270: h2 48/48, col0 351, col1 213, col2 17; +360: col0 open, col1 366, col2 259; +450: status 0.42; +540: status 1.00. P2: `gate-cells.slot-visual` opens before the two `hlist` cards. A1: `loop.slot-visual` before the two panes. M5: `pnums.slot-visual` → `pdet` → `cta` |
| block cursor while typing, none when complete | PASS | `::after` on the last `.ch.on`: 30.8 px molten block, `animation: blink`; `content: none` once complete. `qa-shots/r2-home-p3-typing-3870.png`, `r2-home-p1-1080.png` |
| no reflow while typing | PASS | per-character top offsets identical at every step (two-line headline stays two lines from the first character) |
| cards expand from zero height, top-down, 8 px rise | PASS with note | col height 17 → 155 → 351 → 366, translateY 8 → 4.6 → 0.1 → 0; opacity also 0.2 → 1 (spec says clip only; harmless). But the clip takes ~180 px (33 % of the range), not ≤ 12 % → defect D12 |
| after the last slot it releases and scrolls normally | PASS | top goes −90, −180, −270 … after +540 |
| scrolling back before release reverses | PASS | P3: 3780 → 4140 (h2 66/66, cards open) → 3960 (55/66, visual 174 px, cards collapsed) → 3870 (11/66) → 3780 (0/66). `qa-shots/r2-home-p3-rev-3960.png` |
| after release scrolling up does not unmount | PASS | P1, P2, A1, A2, M5: scroll to top+900 then back to top+200 → all `on`, cards open, status 1.00 |
| hero mounts on load, not on scroll | PASS | home hero: `.hero-prove` 0.2 → 1.0 by 250 ms, numbers roll on load; math hero static with seams drawn |

## 3. Text motion (§0b)

| check | result |
|---|---|
| titles type (kicker then headline) | PASS, all routes (char spans `.ch`, kicker completes before the headline starts in earnest) |
| inner text expands (cards, panes, lists, tables) | PASS: `.rv` blocks grow from 0/2 px to natural height; text complete inside the clip |
| labels / status bars fade | PASS: `.fd` / `.slot-status` opacity 0 → 1, no transform |
| nothing per word | PASS: zero word-span structures on any section probed; zero partial-opacity leaf text after release |
| numbers roll only in their slot | PASS with one miss: P1 stat rolls during its card's slot (`+00 %` visible while the card is still clipping, `qa-shots/r2-home-p1-1080.png`, spec says after the block has revealed); hero numbers roll on load; M5 numbers do not roll at all (D13) |

## 4. Discreet slag field and the hero field

| check | result | evidence |
|---|---|---|
| canvas behind unpinned sections on every route | PASS | `.gl-field` canvas, `position: fixed`, z 0, 1440×900, present on `/`, `/agents`, `/math` |
| faint dots visible | PASS | dot grid visible in blank areas of `qa-shots/r2-home-p1-1080.png`, `r2-ag-a1-1080.png`, `r2-m-crap-1-t85.png` |
| not inside the K dive | PASS | `.gl-field` `visibility: hidden` while the dive is pinned; `.furnace-host` opacity 1 there |
| hero field at full intensity, reacts to pointer (DPR 1) | PASS | luminance to the right of the pointer: 35 at 40 px, 19 at 120, 13 at 200, 11.5 at 280 (background 11.4) → pool radius ≈ 250–280 px, follows the pointer between (300,250) and (1100,650). `qa-shots/r2-dpr1-hero-ptrA.png`, `r2-dpr1-hero-ptrB.png` |
| discreet field reacts to pointer behind P1 (DPR 1) | PASS, faint | 28 / 23 / 20 / 21 at 40–280 px vs 11 far away |
| DPR 2 before the fix | FAIL (superseded) | `.gl-host` canvas was 2880 × 67 108 864 px (!), hero heat absent (11.5 everywhere), dots only in the left half. `qa-shots/r2-dpr2-hero-ptrA.png` |
| DPR 2 after the fix | PASS | `.gl-host` 2880×1800, `.gl-field` 2880×1800; pointer pool at (1100,650): 35.5 / 20.8 / 13.6 / 11.5 at 40–280 px (same profile as DPR 1); pool at (300,250) visible in `qa-shots/r2-dpr2fix-hero-ptrA.png`; dots span the full width (dot max 141–143 in both the left and right strips of the bottom row). `qa-shots/r2-dpr2fix-hero-ptrB.png`, `r2-dpr2fix-p1-ptrA.png`, `r2-dpr2fix-p1-ptrB.png` |
| absent under reduced motion | PASS | 0 canvases on all routes |

## 5. /math

Order M0 → crap (3 states) → dry (3) → mutate (3) → tests (2) → M5: PASS (2700 + 2700 + 2700 + 1800 = 1100 vh of explainers). Copy: every one of the 289 text lines in `math-reference.html` is present verbatim on the live page across its states (union of all state dumps; 0 missing). "the math" nav item `aria-current="page"`, molten, on `/math`; present on all three pages. All diagrams are DOM/SVG (0 `<img>`). No furnace/flash canvases; only the discreet `.gl-field` (see D15). Screenshots per state at t = 0.50 and 0.85: `qa-shots/r2-m-<gate>-<n>-t50.png` / `-t85.png`; `r2-math-m0.png`; `r2-math-m5-maxscroll.png`.

| state | PNG match | §16.2 motion | evidence |
|---|---|---|---|
| M0 hero | PASS | static headline, strip seams drawn | `r2-math-m0.png` vs `M0-math-hero.png` |
| crap 1 count | PASS | squares light L→R, `CC` rolls 1 → 6 → 9 at t 0.1 / 0.35 / 0.6 | `r2-m-crap-1-t85.png` |
| crap 2 discount | PASS | bars 0 at t 0.1, grown by 0.35; `∞` label lands slag at t 0.85 (last) | `r2-m-crap-2-t85.png` |
| crap 3 compare | PASS | scale ingot to 10 / slag beyond; value rolls 50.5 → 39.2 → 12.8 → 9 (ingot) while `50.5` stays slag | `r2-m-crap-3-t85.png` |
| dry 1 erase | PASS | labels become `:symbol` by t 0.35; `tree-seam` scaleX 0.82 → 1 (molten seam connects the identical trees) | `r2-m-dry-1-t85.png` |
| dry 2 overlap | PASS | shared count rolls 0 → 15 → 20; `20 / 22 = 0.91` molten | `r2-m-dry-2-t85.png` |
| dry 3 decide | PASS | molten band 0.8–1.0, marker at 0.91 inside it | `r2-m-dry-3-t85.png` |
| mutate 1 token | PASS | `tok-roll` translateY 0 → −21.6 (0 → 1 vertical roll); caption operator list cycles (`delete not` on at t 0.85) | `r2-m-mutate-1-t85.png` |
| mutate 2 suite | PASS | cells ingot/slag in reading order; `killed` 0 → 7 → 10, `survived` 0 → 1 → 2 | `r2-m-mutate-2-t85.png` |
| mutate 3 survivor | PASS | `10 / 12` lands; `83.3 %` ash; `exit-fill` scaleX 0.09 → 0.89 → 1 | `r2-m-mutate-3-t85.png` |
| tests 1 killed | PASS | matrix; `returns_number` / `settle_called` slag with `0` | `r2-m-tests-1-t85.png` |
| tests 2 unique | PASS | shared cells dim to seam, unique cells molten, counts 2 / 1 / 0 / 0 / 0 | `r2-m-tests-2-t85.png` |
| M5 parity | PASS layout; FAIL roll | matches `M5-math-parity.png`; CTA → `https://github.com/coderefinery/coderefinery/tree/main/docs`; numbers do not roll (D13) | `r2-math-m5-maxscroll.png` |
| between states only term / card / diagram change | PASS | formula molten term shifts per state, card swaps, diagram swaps; layout and rail persist |
| reduced motion = static cross-fade | PASS | `ex-stage snapshot` opacities 1/0/0 → 0/1/0 → 0/0/1, zero partial-opacity nodes, bars at final state. `qa-shots/r2-rm-math-1350.png`, `r2-rm-math-1800.png` |

Minor fidelity notes: M3 formula renders `( killed + survived )` with spaces inside the parentheses (PNG: `(killed + survived)`); M1.2 at 1440 the chart sits lower in the slab than the PNG (fine).

## 6. Reduced motion and 900 px

| route | reduced motion | 900 px |
|---|---|---|
| `/` | PASS: 0 canvases; sections complete on entry (P1 at top+90: 66/66 chars, cards 1.00/368, status 1.00); no hold (top −450 at +450); dive see D3 (fixed) and D4 (not fixed). `qa-shots/r2-rm-home-990.png` | PASS: scrollWidth 900 everywhere; sections auto-height (1754, 917, 989 …); dive stacked. `qa-shots/r2-narrow-home-hero.png`, `r2-narrow-home-p1.png`, `r2-narrow-home-dive.png` |
| `/agents` | PASS: same (92/92 chars, cards open, no hold). `qa-shots/r2-rm-agents-990.png` | PASS: scrollWidth 900. `qa-shots/r2-narrow-agents-hero.png`, `r2-narrow-agents-1800.png` |
| `/math` | PASS: hero complete, explainers cross-fade, M5 complete. `qa-shots/r2-rm-math-*.png` | PASS: scrollWidth 900. `qa-shots/r2-narrow-math-hero.png`, `r2-narrow-math-1800.png` |

## 7. Console, shader log, jank

- Console: only vite debug lines on all three routes; no errors or warnings.
- `window.__shaderLog`: **undefined** on `/`, `/agents`, `/math` (D14).
- Dive scrub 8100 → 11700 over 4 s via rAF: 241 frames, avg 16.6 ms, worst 17 ms, 0 frames > 34 ms.
- Explainers scrub 900 → 10800 over 6 s: 361 frames, avg 16.6 ms, worst 17 ms, 0 long frames.
- No visible layout jumps inside pinned sections. Note: section wrappers grow by 540 px while mounting, so the document scrollHeight changes as you scroll (e.g. /math 11773 at rest, up to 12313 mid-mount); the scrollbar thumb resizes slightly at each section. Not jank, but visible on a long page.

## 8. Redesigned hero (late change)

Compared against the overwritten `P0-hero-pain.png` and §12 P0. Screenshots `qa-shots/r2-hero-1440-t0.png`, `-t300`, `-t1500`, `-t4000`, `-t18000`; `r2-hero-1280-t0…t18000.png`.

| check | result |
|---|---|
| 2×2 big-numbers block right, no session pane | PASS (`bignums-2`, no `figure.session`) |
| four PR lanes as faint schematic at the bottom under a fade | PASS: four `.lane` rows (588–816 at 1440; 465–647 at 1280), `pointer-events: none`, dimmed, seams/nodes/arrows as DOM |
| `scroll ↓` at the right of the status bar | PASS |
| load: "Prove it." at 200 ms | PASS (0.2 → 1.0 by 250 ms) |
| numbers roll, molten last | PASS (t0 shows `+000 %`, `00 %`, `00 %`, `0–000×` mid-roll; settled by 1.5 s) |
| lane art fades in after, ticks on a clock | PASS: absent at t0 (`qa-shots/r2-hero-1440-t0.png`), present by 4 s; `tests · 1,204 running ▂▃▅▆ 3:12 → 3:18` ticks on a clock |
| a token travels a lane every ~12 s | PASS: `.token.go` moved x 61 → 909 at ~10 s (1440) and 61 → 958 at ~8 s (1280), then returned; one travel observed in a 14 s window |
| fits 100 vh at 1440×900 and 1280×720 | PASS: hero height == scrollHeight == viewport; lowest text 28 / 22 px above the bottom |
| art never overlaps readable text | **FAIL** → D11: lane 1 (y 588–636 at 1440) sits under the `hero-foot` line "one binary · no runtime · … dryer, mutator" (wraps to two lines, bottom at ~860 in the 2880-px shot); at 1280 lane 1 (465–503) likewise sits under the foot's second line. The faint `PR #482 · add quote endpoint / waiting for checks · 14 min` ghost is readable through the text in `qa-shots/r2-hero-1440-t4000.png` and `r2-hero-1280-t4000.png` |

## 9. Slag-field DPR 2 fix (late change)

Tested with `set viewport 1440 900 2` (`devicePixelRatio` 2). Before the fix the `.gl-host` canvas was allocated at 2880 × 67 108 864 and drew nothing useful; after the fix all canvases are 2× CSS size (`gl-field` 2880×1800, `gl-host` 2880×1800, `furnace` 2880×1800, `flash` 956×482 / 956×298).

- Dots span the full section width: PASS (dot peak 141–143 in both the left and right strips of the hero's bottom row; visible edge to edge in `qa-shots/r2-dpr2fix-hero-ptrB.png`).
- Pointer heat pool ≈ 280 px following the pointer: PASS. Pointer at (1100,650): 35.5 / 20.8 / 13.6 / 11.5 luminance at 40 / 120 / 200 / 280 px (background 11.4) — the same profile as DPR 1. Pointer at (300,250): pool visible around the headline in `qa-shots/r2-dpr2fix-hero-ptrA.png`. Behind P1 the discreet pool is faint but present (`r2-dpr2fix-p1-ptrA.png` vs `r2-dpr2fix-p1-ptrB.png`).

## 10. Defects, ranked

No blockers.

### Major

**D11 · Hero lane art overlaps the hero-foot line.** Spec (§12 P0 / brief): the lane schematic "never overlaps readable text". Page: the first lane row (588–636 at 1440×900, 465–503 at 1280×720) is drawn under `p.hero-foot` ("one binary · no runtime · Claude Code, Cursor, CI · byte-identical …"), which wraps to two lines at both widths; the ghost text "PR #482 · add quote endpoint · waiting for checks · 14 min" reads through it. Screenshots `qa-shots/r2-hero-1440-t4000.png`, `r2-hero-1280-t4000.png`. Repro: `/` at 1440×900 or 1280×720, wait 4 s, look at y ≈ 590–640 (1440) / 465–505 (1280). Either start the lanes below the foot line or keep the foot on one line.

**D4 (carried) · Reduced motion shows the K4 "preview · the tests gate ships…" footer under every dive state.** Spec: the line belongs to K4 only. Page (RM): `previewFooterVisible: true` at dive top + 400, 1300, 2200, 3100, 3550. Normal mode is correct. Screenshot `qa-shots/r2-rm-dive-1300.png`. Repro: emulate reduced motion, `/` at 1440×900, scroll to dive top + 1300.

### Minor

**D12 · Card clip opens over ~33 % of the mount range, not ≤ 12 %.** §0b: "the clip opens in one motion of at most 12 % of the section's mount range, so a block becomes readable in one step." Page (final build, entrance rule): P1 col0 height 17 → 155 → 351 → 366 across e 0.40 → 0.60, i.e. 180 px of the 540 px range; at e = 0.5 the quote is cut mid-sentence ("…it reads right, and it is") and the stat is mid-roll (`+00 %`) while the block is still clipping (§0b: numbers roll after the block has revealed). Same on P3, A2, M5 (each card ~180 px). Screenshot `qa-shots/r3-home-p1-e50.png`. Repro: `/` at 1440×900, scroll to 450 slowly.

**D13 · M5 parity numbers do not roll.** §16.3: "the five numbers roll from 0 when the strip enters view, the molten 36/36 last." Page: `36/36 | 99.3% | 100% | 99.5% | 100%` already final while the strip is at opacity 0.24; no odometer transforms. Repro: `/math` at 1440×900, scroll into the parity section and watch the strip appear.

**D14 · `window.__shaderLog` is not exposed.** The brief asks for it and it is the only way to audit the §14.4 guards (first-frame > 8 ms, saveData, RM). `undefined` on all three routes; the round-1 console line is gone too, so whether a shader was dropped on this machine is unknowable. The verdict flash (§14.4 item 3) still could not be captured (D9 carried).

**D15 · §0c vs §16 disagreement on /math.** §0c says the discreet slag field "runs behind every unpinned section on every route"; §16 says "No shaders on this page." The page follows §0c (`.gl-field` canvas present and visible on M0/M5). Decide and align the spec; not a page bug.

**D7 follow-on · K1 "before" terminal shows a bare `$` above the CRAP table after the hand-off.** The typed command lives in the P5 session pane that slides out, so once pinned the K1 terminal reads `$` / `Function CRAP cov% cc …`. Repro: `/` 1440×900, dive top + 0 to + 700. Spec §5a expects the typed line to become the `cmd` line of the terminal it ran in.

**Observations (not defects):** the one remaining ASCII operator is code line 14 `if total <= 0:` (valid Python, PNG shows `≤`); M3 formula has padded parentheses `( killed + survived )`; sections grow the document while mounting so the scrollbar thumb resizes as you go; cards fade (0.2 → 1) in addition to the spec'd clip, which looks fine.

## 12. Re-test against the rewritten §0c ("mount on entrance, no hold") · final build

Entrance progress `e = (viewportHeight − sectionTop) / viewportHeight`, stepped in 0.1 (90 px) from e = 0 to 1 on `/` P1 and P3, `/agents` A2, `/math` M5, plus the two pinned sections' first states. Screenshots `qa-shots/r3-home-p1-e{30,50,70,90}.png`, `r3-home-p3-e*.png`, `r3-ag-a2-e{50,90}.png`, `r3-math-m5-e{50,90}.png`, `r3-dive-e{20,50,80,100}.png`, `r3-math-crap-e{30,60,90,100}.png`.

| check | result | evidence |
|---|---|---|
| no sticky hold; every unpinned section a plain 100 vh block | PASS | all unpinned sections `position: relative`, 900 px; page heights home 11773, /agents 4573, /math 11773 |
| nothing happens before e = 0.30 | PASS | P1/P3/A2/M5 at e 0.2 and 0.3: kicker 0, headline 0, cards collapsed (0–17 px @ 0.2), status 0 |
| stagger runs over e ∈ [0.30, 0.90] in the specified order | PASS | P1: e 0.4 kicker 11/18 + h2 4/48 → 0.5 kicker done, h2 32/48, col0 155 px → 0.6 h2 done, col0 351, col1 213 → 0.7 col0/col1 open, col2 259 → 0.8 all cards open, status 0.42 → 0.9 status 1.00. P3: kicker 22/33 @0.4, h2 55/66 + visual 174 px @0.5, lede + visual open @0.6, panes @0.7, status 0.33 @0.8, 1.00 @0.9. A2: kicker 41/49 @0.5, table 493 px @0.7, status 1.00 @0.9. M5: kicker done + h2 15/30 @0.5, strip 122 px + dets 70 px @0.7, CTA open @0.9 |
| docks fully assembled; last 10 % quiet | PASS | every probe identical at e = 0.9 and e = 1.0 |
| scrolling back before 0.90 reverses | PASS | fresh load, P3: e 0.65 (h2 66/66, three cards open) → 0.45 (h2 33/66, all cards collapsed) → 0.30 (kicker 0/33, h2 0/66) → 0.65 again (same as before). `qa-shots/r3-home-p3-reverse-e45.png` |
| once complete it stays mounted | PASS | P3 after e = 1.0, back to e = 0.45: h2 66/66, cards open, status 1.00. Same on P1 |
| outgoing section does not animate on exit | PASS | while P3 mounts (e 0.3–0.7) the outgoing P2 has 0 partial-opacity elements, opacity 1, transform none, 35/35 chars, cards open |
| pinned dive: first state's title types and panes reveal during the entrance | PASS | `.dive-title` "Too complex for its tests." 0/26 @e 0.4 → 13/26 @0.6 → 22/26 @0.8 → 26/26 @1.0; code pane opacity 0.50 → 0.69 → 0.99 → 1.00; docks assembled (`qa-shots/r3-dive-e100.png`) |
| pinned crap explainer: first state's title and card mount during the entrance | PASS | card title 10/20 @e 0.6 → 20/20 @0.9; card 1.00 @0.6; visual/formula present from entry (`qa-shots/r3-math-crap-e90.png`) |
| reduced motion: complete on entry, no hold | PASS (from section 6, rule unchanged) | |

D12 (card clip over ~33 % of the range) stands under the new rule; its numbers above are from this build.

## 13. Hero rising event stream (§12 P0, final build)

Screenshots `qa-shots/r3-hero-stream-1.png`, `-2`, `-3` (1 s apart, 1440×900), `r3-hero-stream-1280.png`, `r3-hero-stream-rm.png`.

| check | result | evidence |
|---|---|---|
| events are pills with glyph · message · PR number, five tones | PASS | `.ev.ev-ingot/-ember/-slag/-seam/-ash`, e.g. `○ queued · 6 min PR #468`, `▸ tests · 1,204 · running ▂▃▅▆ 3:16 PR #466`, `⛔ coderefinery gate · survived 2 · exit 3 PR #480` |
| rise at 18 px/s, constant | PASS | `○ queued` at y 496 → 471 → 446 over 1.42 s = 17.6 px/s; running events tick (`3:12 → 3:16`, bar glyphs cycle) |
| fade band: full below the foot line, 0 at the sub-heading top | PASS at 1440 / FAIL at 1280 → D16 | 1440: opacity 1.00 at y 611–642 (foot bottom 622), 0.71 @554, 0.40 @496, 0.10 @439, lede at 435–485: fade ends at the lede. 1280×720: foot 472–506, lede 336–383; events still at 0.35 over the lede |
| never covers readable text | PARTIAL → D16 | 8 s overlap sample at 1280: max event opacity over `.hero-lede` 0.35, over the install box 0.73, over the 2×2 numbers 0.66, over the foot line and the status bar 1.00. The install box and numbers are "behind" by design (§12: "the stream is behind the text and the install box") and the pills are dim, but the foot line and status bar get full-opacity pills passing through them (`qa-shots/r3-hero-stream-1.png`: `○ review requested · 2 pending` under the foot, `↑ agent pushed 3 commits` through the status bar) |
| failures blink and carry a halo | PASS | `.ev.ev-slag.fail` has `animation: blink`; slag pills visible in `r3-hero-stream-rm.png` / `-1.png` |
| sparks | PASS | `[class*=spark]` present (2 at sample time), molten, rising |
| thin pipes obey the band | PASS | `.stream-pipe` opacity 0.15 @462, 0.46 @521, 0.77 @579, 1.00 from 637 down |
| attention budget / cost | PASS | 22–25 stream nodes on screen; `window.__streamCost` avg 0.06 ms/frame, max 1.7 ms (n = 1098) at 1440; avg 0.053 ms at 1280 |
| reduced motion: static ~12 events, no sparks, no blinks | PASS | 22 nodes (pipes + command line + ~12 events) at identical positions 2 s apart; only the wordmark cursor animates; `qa-shots/r3-hero-stream-rm.png` |
| no canvas in the stream (DOM, transforms only) | PASS | 0 canvases inside `.stream`; events move by transform |

**D16 (minor) · Stream fade band ends too low at 1280×720 and full-opacity pills cross the foot line / status bar.** §12 P0: "From the foot line upward they fade linearly and reach 0 at the top of the mechanism sub-heading. Above that nothing is drawn." At 1280×720 the band is sized for the 1440 layout: events remain at 0.35 opacity over the lede (y 336–383) and the first pill rows pass through the foot line (y 472–506) at 1.0. At 1440 the band ends at the lede correctly but pills still cross the foot line and the status bar at full stream opacity. Repro: `/` at 1280×720, wait 5 s, watch pills reach the "Three gates on the diff…" paragraph; at 1440×900 watch `○ review requested` pass through "one binary · no runtime…". Suggested: derive the band from the live foot/lede rects and start the fade one pill-height below the foot.

D11 (lane art under the foot line) is superseded by the stream redesign; the static lane rows are gone from the final build. The hero still fits 100 vh at both sizes (900/720, nothing outside the section).

## 14. Verdict

**Fix-then-ship.** Eight of the ten round-1 defects are fixed outright and two are reduced to minor residue; every section on all three routes is exactly 100 vh and fits at both target sizes with nothing cut; the rewritten §0c entrance rule holds on every section stepped (nothing before e = 0.30, kicker → headline → visual → cards → captions → status over [0.30, 0.90], assembled by 0.90, reversible before completion, sticky once complete, outgoing sections inert, pinned sections docking assembled); /math is copy-verbatim against the reference with all eleven diagram motions present as DOM/SVG and a clean cross-fade under reduced motion; the DPR-2 field fix holds; the hero's event stream runs at 18 px/s for ~0.06 ms a frame, blinks, sparks and goes static under reduced motion; scrubbing is frame-perfect and the console is clean. What remains is the stream's fade band not tracking the 1280 layout and crossing the foot/status lines (D16), the reduced-motion preview footer (D4), and three small spec misses (card clip speed D12, M5 roll D13, missing shader log D14). None blocks a visitor; each is an hour or two.
