# coderefinery site · QA round 1

Judged live in headless Chromium (agent-browser) against `designs/exports/*.png`, `designs/motion-spec.md` (§0b, 1, 4, 5a, 12, 13, 14, 15; §16 only for the home teaser) and `designs/tokens.json`. Dev server `http://localhost:5173/`. Viewports 1440×900 (primary), 900×900 (narrow), plus `prefers-reduced-motion: reduce` emulated at 1440×900. Screenshots live in `qa-shots/` next to this file; paths below are relative to `designs/reviews/`.

Scope notes from the coordinator, applied:
- The home page's full math section was replaced by a teaser; the teaser is checked against `M-the-math-teaser.png`. The old full section is not reported as missing.
- `/math` is being rebuilt against a new design and is **deferred to round 2**. It is not judged here (a few exploratory shots `qa-shots/m0-*`, `m1-*` … `m5-*` exist from before the deferral; ignore them).

Geometry measured on the page (used throughout):
- Sections (1440×900): P0 `#top` 0–900 · P1 `#why` 900 · P2 `#gates` 1683 · P3 2547 · P4 3509 · P5 `#hook` 4514 · dive `#inside-the-gate` 5434 (3600 px tall) · S9 `#numbers` 9034 · M teaser `#the-math` 9796 · footer 10383.
- Pin: sticky engages at scrollY 5459 and releases at 8159 → **2700 px = 300 vh of scrub** for four states (675 px each). The section itself is 400 vh tall (the sticky viewport eats the last 100 vh). State windows: K1 5459–6134, K2 6134–6809, K3 6809–7484, K4 7484–8159.

## 1. Pass / fail table

| # | Check | Result | Evidence |
|---|---|---|---|
| 1.1 | Order of screens P0→P1→P2→P3→P4→P5→K1–K4→S9→teaser | PASS | section map above; `qa-shots/p1-900.png` … `qa-shots/s9-9034.png` |
| 1.2 | Copy verbatim vs PNGs (headlines, numbers, sources, commands, exit codes) | PASS with minor drift | P4 headline "Nobody verifies whether the new tests can fail." ✓ `qa-shots/p4-3509.png`; S9 kicker "§06 · after the intro" ✓ `qa-shots/s9-9034.png`. Drift: ASCII `->` where PNGs use `→` (P5 hook lines, K3 SURVIVED lines, A2 mutate rows); `<=`/`!=` where PNGs show `≤`/`≠` (P4 mutant table, A3 table, K1 line 14). See defect D8 |
| 1.3 | Layout fidelity P0 | PASS | `qa-shots/p0-hero-settled.png` vs `P0-hero-pain.png` |
| 1.4 | Layout fidelity P1 | PASS | `qa-shots/p1-900.png` (sources are links, `exit 3` slag) |
| 1.5 | Layout fidelity P2 | PASS | `qa-shots/p2-1683.png` |
| 1.6 | Layout fidelity P3 | PASS | `qa-shots/p3-2547.png` |
| 1.7 | Layout fidelity P4 | PASS | `qa-shots/p4-3509.png`, `qa-shots/p4b-3614.png` (CTA slab + status bar) |
| 1.8 | Layout fidelity P5 | PASS | `qa-shots/p5-4800.png` (transcript complete, commit hash ingot, "zero deleted" status bar) |
| 1.9 | Layout fidelity K1 | PASS with colour drift | `qa-shots/k1-bridge-6110.png` vs `K1-crap-dive.png`. Verdict lines chalk instead of slag/ingot (D2); hint prose ash not chalk (D6) |
| 1.10 | Layout fidelity K2 | PASS with colour drift | `qa-shots/k2-bridge-6785.png` vs `K2-dry-dive.png` (D2) |
| 1.11 | Layout fidelity K3 | PASS with colour drift | `qa-shots/k3-bridge-7460.png` vs `K3-mutate-dive.png` (D2, D8) |
| 1.12 | Layout fidelity K4 | PASS with colour drift | `qa-shots/k4-end-8150.png` vs `K4-tests-dive.png` (D2) |
| 1.13 | Layout fidelity S9 | PASS | `qa-shots/s9-9034.png` vs `S9-the-numbers.png` (kicker §06 as briefed) |
| 1.14 | Layout fidelity M teaser (headline, four-formula strip, "read the math →" → `/math`, footer line) | PASS | `qa-shots/m-teaser.png` vs `M-the-math-teaser.png`; button href `/math`, click lands on `/math`; footer "formulas and thresholds as implemented · byte-identical to crapper, dryer and mutator · exit 0" ✓ |
| 1.15 | Tokens: sampled computed colours | PASS | body bg `rgb(11,12,14)` = coke; slab `rgb(19,20,23)`; chalk h1 `rgb(236,237,239)`; ash `rgb(139,143,153)`; molten kicker/Prove it `rgb(255,106,26)`; ingot `rgb(61,220,132)`; slag `rgb(255,77,77)`; dive-ash `rgb(169,173,182)`; dive seam `rgba(255,106,26,0.2)` = heat-seam; code pane bg `rgb(25,18,16)` = furnace-slab; fonts Azeret Mono / Geist / Geist Mono |
| 1.16 | Furnace ground inside the dive | PASS | screenshot ground sample `(17.5,11,9)` ≈ `#110B09` furnace inside the pin vs `(11,12,14)` coke on P1; cross-fades over the first ~15 % (`qa-shots/k1-enter-5500.png` ground `(13,12,12)`) |
| 1.17 | Syntax colours in code (kw blue, str green, num amber, molten never syntax) | PASS | computed: `def` `rgb(124,184,255)` syn-kw; `'digital'` `rgb(143,217,168)` syn-str; `0.95` `rgb(255,201,92)` syn-num; only molten leaves in the code pane are note tags (`branch`), never code tokens |
| 1.18 | Seams draw (§14.1) | PASS | SVG seams with `stroke-dashoffset 0.22s` on every box (home, /agents); nested boxes draw after outer |
| 1.19 | Text tiers §0b on P1 | PASS | 100-px steps 500→1000: zero partial-opacity nodes, zero hidden leaf text, zero word-span structures at every step (`qa-shots/tier-p1-700.png`) |
| 1.20 | Text tiers §0b on P3 | PASS | 2000→2700 same probe, all zero (`qa-shots/tier-p3-2400.png`) |
| 1.21 | Text tiers §0b on A2 | PASS | rows appear whole (`qa-shots/a2-1729.png`); no partial nodes after reveal |
| 1.22 | Tier-1 items scrub/type (stats roll, verdicts, typed commands) | PASS | hero stats are odometers; CRAP/0.91/kill counts roll (`qa-shots/k1-settle-5965.png` shows mid-roll); bridge commands type char-by-char |
| 1.23 | Pin location (CTA slab reaching the top) | PASS with note | Page pins when the dive's top reaches the viewport top (scrollY 5459). P5 sits between the P4 CTA (top 4316) and the pin, per §12 order; the CTA's "no click needed · the page pins" copy therefore applies one screen later. CTA href `#inside-the-gate` |
| 1.24 | Pin length ≈ 400 vh | FAIL (minor) | 300 vh of scrub (2700 px); the 400 vh section minus the 100 vh sticky viewport. See D5 |
| 1.25 | K1 state: tints + CRAP roll | PASS | `qa-shots/k1-settle-5965.png`: eight decision lines tinted with `branch` tags, CRAP 50.5, after-terminal rolling; `qa-shots/k1-bridge-6110.png`: after settles to 3/100.0/3, `exit 0` ingot with molten underline |
| 1.26 | K2 state: duplicate pair tints + 0.91 | PASS | `qa-shots/k2-settle-6640.png`, `qa-shots/k2-bridge-6785.png` (quote() inserted at line 8, pairs tinted ember, `0.91 similar to price` tag) |
| 1.27 | K3 state: survivors | PASS | `qa-shots/k3-bridge-7460.png` (lines 6 and 9 keep mutant text, SURVIVED tags ember; killed tags ingot; two tests typed in patch pane) |
| 1.28 | K4 state: test file, struck tests, `tests` table | PASS | `qa-shots/k4-end-8150.png` (file bar `tests/test_pricing.py`, lines 1–2 and 4–6 struck, 0.93 pair tinted, table with 4/2, 1/1, 0/0, 0/0; parametrize block typed) |
| 1.29 | Code pane / terminals persist, content changes | PASS | same `.code`, `.term-before`, `.term-after` elements across all four probes; line count 16→13→14→13 |
| 1.30 | Depth gauge fills, marks pop | PASS | `.gauge-fill` 4→49→114→179→244→259 px across the pin; marks gain `passed`/`active` |
| 1.31 | Progress bar grows 25 % per state | PASS | `.dive-progress-done` 1.5 % → 18.7 % → 43.7 % → 68.7 % → 93.7 % → 99.7 % |
| 1.32 | Kicker number rolls 01→04 | PASS | `01/04 · crap` … `04/04 · tests` visible in each state shot |
| 1.33 | Bridge types the next command near the end of each state | PASS (K1→K2, K2→K3, K3→K4); FAIL for P5→K1 | at t≈0.96: `$ coderefinery dry --changed --thr` (`qa-shots/k1-bridge-6110.png`), `$ coderefinery mutate --changed --use-exis` (`qa-shots/k2-bridge-6785.png`), `$ coderefinery tests --` (`qa-shots/k3-bridge-7460.png`); "↓ types as you scroll" label on the first two only ✓. K1's own command arrives pre-typed; nothing types under the P5 transcript (D7) |
| 1.34 | Scrolling backwards reverses | PASS | 8150→6640 reproduces the K2 probe exactly (`qa-shots/k2-reverse-6640.png`); 6110→6050 deletes bridge chars back to `$ coderef` |
| 1.35 | Hero: "Prove it." lands after the first lines | PASS | `.hero-prove` animation `land 0.24s` with 200 ms delay (matches §12: 200 ms) |
| 1.36 | Hero: stats roll on load, not on scroll | PASS | odometer transforms identical before/after scrolling to 3000 and back; no re-roll (`qa-shots/p0-hero-return-150ms.png`) |
| 1.37 | Hero: session replays once on load, not on scroll | PASS | on return from 3000 the full 12-line transcript incl. `✗ commit refused` is visible within 150 ms; no replay |
| 1.38 | Hero: slag-field canvas present, behind text, reacts to pointer | PASS with risk | WebGL canvas in `.gl-host` (absolute, z 0, pointer-events none, precedes content; `elementFromPoint` on the h1 returns text). Pointer at (300,250) vs (700,650) moves the warm region: top-left band 20.5 vs 11.8 lum, bottom-left 11.9 vs 21.0 (`qa-shots/p0-pointer-A.png`, `qa-shots/p0-pointer-B.png`). Console: `shader disabled: first frame 9.7 ms > 8 ms` on load — one shader self-disabled by the §14.4 guard (D9) |
| 1.39 | Furnace shader behind the dive, stronger on K4 than K1, not over text | PASS | top-band luminance 27.6 (K1) → 32.5 → 36.9 → 42.1 (K4) → 43.4; `elementFromPoint` on code lines and terminals returns text spans; canvases z 0 / pointer-events none |
| 1.40 | Verdict flash when an exit code lands | NOT OBSERVED | `.flash-host` canvases exist on both terminals, but five captures across the K1 `exit 2` landing (`qa-shots/flash-5900…5990.png`) show no luminance change in the terminal (25,18,16 throughout). Either disabled by the 8 ms guard (see console) or too brief to capture; treat as unverified (D9) |
| 1.41 | Shaders absent under reduced motion | PASS | `document.querySelectorAll('canvas').length === 0`; all `.gl-host/.furnace-host/.flash-host` empty |
| 1.42 | Reduced motion: no typing, no odometers | FAIL | hero stats and session static ✓ (`qa-shots/rm-hero.png`); but inside the dive the patch pane still types (`qa-shots/rm-k4-7800.png`: `+ def test_tier_rate(tier, expected): assert price(order(100), user(tier)▮` mid-typed with cursor) and the bridge prompt + "↓ types as you scroll" label remain (D3) |
| 1.43 | Reduced motion: dive is four cross-faded states, gauge still fills | PARTIAL | states cross-fade (`dive-stage snapshot` at opacity 0.66 at the K1/K2 boundary, `qa-shots/rm-k1k2-boundary-6134.png`); gauge fill only 55 px at scrollY 8100 vs 259 px without RM (D3) |
| 1.44 | Narrow 900 px: hero stacks | PASS | h1, session pane and stats all at x 24, w 852 (`qa-shots/narrow-hero.png`) |
| 1.45 | Narrow 900 px: dive unpins and stacks | PASS | `.dive.stacked`, no `.dive-sticky`, four stages stacked, 4920 px tall, hint docked under the title, furnace ground + gauge kept (`qa-shots/narrow-dive-900.png`) |
| 1.46 | Narrow 900 px: nothing overflows horizontally | FAIL | `scrollWidth` 924 from scrollY ≈ 8700 to the end of the page: `figure.setup-session` is `translateX(48px)` after the P5→K1 hand-off (D1; `qa-shots/narrow-p5-to-dive.png`) |
| 1.47 | /agents order A0–A4 and copy | PASS | `qa-shots/a0-hero.png`, `a1-956.png`, `a2-1729.png`, `a3-2599.png`, `a4b-3425.png` vs A0–A4 PNGs; copy verbatim except `->`/`<=`/`!=` glyph drift (D8) |
| 1.48 | /agents seams draw, loop arrows draw | PASS | 19 SVG seams with `stroke-dashoffset 0.22s`; four `li.loop-arrow` SVG strokes between the five step boxes |
| 1.49 | /agents A3 hover pairing | PASS | hovering `0 → 1` lights it and `if it would pass with every function returning nothing…` (`.lit`, molten seam) (`qa-shots/a3-hover-mutant.png`); hovering the rule row lights the mutant row back (`qa-shots/a3-hover-rule.png`) |
| 1.50 | /agents CTA → home hook section | PASS | `see the hook →` href `/#hook` (= P5 `#hook`) |
| 1.51 | "for agents" nav item on all pages, active on /agents; "the math" present on all pages | PASS | home nav: why · for agents · the math (→ `/math`) · docs · github; `/agents`: `for agents` has `aria-current="page"`, `.active`, molten; `/math`: `the math` active (route itself deferred) |
| 1.52 | Performance smell while scrolling the dive | PASS | 240-frame rAF scrub 5459→8159: avg 16.7 ms, worst 17 ms, zero frames > 34 ms; no layout jumps seen between state shots (panes keep position) |
| 1.53 | Console errors | PASS | only vite debug lines and the one `[info] shader disabled…` line; no errors or warnings on `/`, `/agents` |

## 2. Defects, ranked

No blockers. Nothing stops a visitor from reading the page or breaks navigation.

### Major

**D1 · Horizontal overflow at 900 px after the P5→K1 hand-off.**
- Spec: §13.4 / brief: "nothing overflows horizontally" at narrow widths.
- Page: the setup-session pane (`figure.setup-session`) is translated right by 48 px as part of the P5→K1 slide (§14.2) and stays there. At 900 px the pane is already full-width, so the slide pushes it to right = 924 and the document gets a 24 px horizontal scroll range for the rest of the page (dive, numbers, teaser, footer).
- Screenshot: `qa-shots/narrow-p5-to-dive.png`.
- Repro: `/` at 900×900, scroll to the `#hook` section's bottom (scrollY ≈ 8700 and anything below); `document.documentElement.scrollWidth` → 924. At 1440 the same transform is harmless (scrollWidth 1440).

**D2 · Dive terminals: verdict lines are not colour-coded.**
- Design: K1–K4 PNGs render failing verdicts in slag (`CRAP threshold exceeded: 50.5 > 9.9999`, `DUPLICATE score=0.91` + the two spans + `1 pair at or above 0.80`, `killed 10 · survived 2`, `DUPLICATE score=0.93 ×2`, the two `0 0 —` rows) and the "after" results in ingot (`3 100.0 3`, `No duplicate candidates found.`, `killed 12 · survived 0 · re-ran 2 mutants only`, `5 tests · 12 kills · 0 with zero unique kills`). §5 "verdict … slag; exit 0 ingot"; §13.1 "a molten line always means a verdict".
- Page: all of those lines are chalk `rgb(236,237,239)`; only the `exit N` lines are tinted. The colour that carries the meaning of each state is missing.
- Screenshots: `qa-shots/k1-bridge-6110.png`, `qa-shots/k2-bridge-6785.png`, `qa-shots/k3-bridge-7460.png`, `qa-shots/k4-end-8150.png` (compare with `exports/K1…K4`).
- Repro: `/` at 1440×900, scrollY 6110 (K1 settled); computed colour of the `CRAP threshold exceeded` span is `rgb(236,237,239)`.

**D3 · Reduced motion still types and the gauge does not fill.**
- Spec: §10/§13.4 "no typing, no odometers … four static states cross-faded by scroll, the depth gauge still fills"; §0b "typing is reserved for commands".
- Page: with `prefers-reduced-motion: reduce` the canvases are gone and the hero is static (good), and the states cross-fade, but (a) the patch pane still types its lines with a block cursor (K4 at scrollY 7800 shows a half-typed `+ def test_tier_rate(…)▮`), (b) the "before" terminal still shows the bridge prompt and the `↓ types as you scroll` label, (c) `.gauge-fill` reaches only 55 px at the end of the pin (259 px without RM).
- Screenshots: `qa-shots/rm-k4-7800.png`, `qa-shots/rm-k1k2-boundary-6134.png`.
- Repro: emulate reduced motion, reload `/` at 1440×900, scroll to 7800 and 8100.

### Minor

**D4 · The "preview · the tests gate ships next…" footer shows under every state in stacked/RM modes.**
- Design: that line belongs to K4 only (`K4-tests-dive.png`).
- Page: under reduced motion it sits under the K2 after-terminal (`qa-shots/rm-k1k2-boundary-6134.png`); at 900 px it sits under K1's after-terminal (`qa-shots/narrow-dive-900.png`). Normal 1440 mode is correct.
- Repro: RM at 1440×900 scrollY 6134; or 900×900 scrollY ≈ 9400.

**D5 · Pin scrub is 300 vh, not ≈ 400 vh.**
- Spec: §13.2 "Pin height = 4 states × 100 vh = 400 vh. Each state owns a quarter."; brief: "measure pin length ≈ 400 vh".
- Page: section is 400 vh tall, but the sticky viewport consumes 100 vh of it, so each state gets 75 vh (675 px) of scroll. The tool-call bridges feel quick: the `dry` command is only ~75 % typed with 24 px of state left (`qa-shots/k1-bridge-6110.png`). If the intent is 100 vh per state the section needs to be 500 vh.
- Repro: `/` at 1440×900; `.dive-sticky` top hits 0 at scrollY 5459 and `.dive-progress-done` reaches 100 % at 8159.

**D6 · Hint box prose is dimmer than the design; box taller than its content.**
- Design: K PNGs show the hint sentence in chalk inside a molten-stroked box that hugs the text. (§13.1 even asks for a molten-filled box with coke text; the PNGs are the outlined version, which the page follows.)
- Page: prose renders ash-grey, and the box has ~50 px of empty space below the text in every state.
- Screenshot: `qa-shots/k1-settle-5965.png` (top-right).
- Repro: `/` at 1440×900, scrollY 5965.

**D7 · P6 bridge absent: K1's command arrives pre-typed.**
- Spec: §12 P6 "The last 10 % of P5's scroll range is the bridge into K1: `$ coderefinery crap --changed` types under the transcript"; §5a "cause (typing) always precedes effect".
- Page: nothing types under the P5 transcript (it ends at `✓ [main 4f2c1d9] …`), and the dive's "before" terminal already contains `$ coderefinery crap --changed` plus its table at the first pinned frame. The slide (session pane right, code pane in from the left) does run.
- Screenshots: `qa-shots/p6-handoff-5420.png`, `qa-shots/k1-enter-5500.png`.
- Repro: `/` at 1440×900, scroll 5250→5470.

**D8 · Glyph/copy drift: ASCII operators where the PNGs use typographic ones.**
- Design: `0 → 1`, `1 → 0`, `and → or` (P5 hook block, K3 SURVIVED lines, A2 mutate rows, K3 patch `kills 0 → 1`); `< → ≤` (P4 and A3 mutant tables); `== → ≠` (A3); `if total ≤ 0:` (K1 line 14).
- Page: `0 -> 1`, `1 -> 0`, `< → <=`, `== → !=`, `<= 0`. (The patch-pane `kills 0 → 1` does use the arrow.) The PNGs' `≤`/`≠`/`→` in code may be font ligatures in the design tool; decide one way and make the transcript and the code pane agree.
- Screenshots: `qa-shots/p5-4800.png`, `qa-shots/k3-bridge-7460.png`, `qa-shots/p4-3509.png`, `qa-shots/a2-1729.png`, `qa-shots/a3-2599.png`.

**D9 · A shader self-disables on the 8 ms first-frame guard; verdict flash never observed.**
- Spec: §14.4 shaders disabled "when the first frame renders slower than 8 ms" with the CSS gradient as fallback; flash = 240 ms band when an exit code lands.
- Page: console logs `shader disabled: first frame 9.7 ms > 8 ms` on every load in this (headless, software-GL) browser. The hero slag field still reacts to the pointer and the furnace glow still grows K1→K4 (CSS `.heat-glow` fallback or GL, indistinguishable from the outside), but the verdict flash was never visible in five captures around the K1 `exit 2` landing. Worth checking on real hardware which of the three is being dropped and whether 8 ms is too tight for a cold first frame (the guard fires before the shader has warmed).
- Screenshots: `qa-shots/flash-5960.png`, `qa-shots/flash-try-5940.png`.
- Repro: load `/`, read the console.

**D10 · Odometer digit strips leak into text content.**
- Page: every rolling number is built as a strip of `0 1 2 3 4 5 6 7 8 9 0`, so `innerText`/accessibility tree of the hero stats, the dive kicker (`▼ inside the gate · 0 1 2 3 … /04 · crap`), the CRAP cells and kill counts read as digit runs. Screen readers and copy-paste get `0 1 2 3 4 5 6 7 8 9 0` instead of `441`.
- Repro: `/` → `document.querySelector('.hero').innerText`.

**Observations, not defects**
- P1 columns enter one after another (the ci column is still absent while the trust column is mid-fade, `qa-shots/p0-hero-return-150ms.png`). Each column is one tier-2 block, so this is allowed by §0b, but it is the one place a reader may notice staggering.
- P4's CTA copy promises "the page pins … four stops" yet P5 (the setup screen) comes first, per the §12 order. Expected, but the sentence reads one screen early.
- K3 "after" terminal and the second patched test only land in the last ~20 % of the state (empty at t = 0.75, full at t = 0.96). Within §1's settle/bridge windows, just late.

## 3. Verdict

**Fix-then-ship.** The page is structurally faithful to the exports: every screen is present in order, copy matches apart from arrow/operator glyphs, tier-2 text never waits for scroll, the pin scrubs forwards and backwards through all four states with persistent panes, the furnace ground, gauge, progress bar and bridges behave, `/agents` matches A0–A4 including the hover pairing, and scroll performance is clean. What stands between it and shipping is small and contained: the 24 px horizontal overflow at narrow widths from the hand-off slide (D1), the missing slag/ingot colour on the dive's verdict lines, which is the one semantic cue the design leans on (D2), and the reduced-motion path that still types and barely fills the gauge (D3). Those three plus the minor glyph drift are an afternoon's work; `/math` is deferred to round 2.
