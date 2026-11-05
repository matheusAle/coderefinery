# QA, coderefinery site, round 4 (verification pass of D17-D23)

Live pages only (`http://localhost:5173/`, `/agents`, `/math`), driven with agent-browser at 1440x900, 1280x720 and 900 px wide, plus `prefers-reduced-motion: reduce`. Screenshots are in `designs/reviews/qa-shots/`, prefix `r5-qa-`.

## Pass/fail

| # | Check | Result | Evidence |
|---|---|---|---|
| 1 | D17 line-height >= 1.25 on headings and titles >= 20 px | PASS | `/`: 12 headings/titles (hero 74.3 px, five 54.8 px slot titles, the S9 headline 54.8 px, four 28 px dive titles, one 43 px) all exactly 1.250. `/agents`: 5 headings (54.8 px) 1.250. `/math`: 13 (62.6, 28 x11, 50.9) 1.250. S9 headline at scrollY 9900 is 1.250 and the two lines no longer touch: `r5-qa-home-s9-9900.png`. Non-heading display text under 1.25 is by design (numerals 1.0, formula terms 1.1, the `.wm-*` wordmark 1.2, `.sr-only` h2 1.2), see R1 |
| 2 | D18 status-bar divider y identical on every card | PASS | 1440x900: y = 831 on all 19 samples on `/` (hero, P1-P5, all four dive states, S9, math teaser), 9 on `/agents`, 17 on `/math` (all 11 explainer states). 1280x720: y = 667.1 on every sample. The only other values (813.8, 651.9) are the last math state scrolling away after its pin, not a docked card. `r5-qa-1280-p5-4500.png`, `r5-qa-1280-hero-0.png`, `r5-qa-1280-dive-k2-7000.png`, `r5-qa-1280-math-mutate-7245.png`, `r5-qa-1280-agents-a1-900.png` |
| 3 | D19 narrow dive full-bleed, gauge hidden | PASS | 900x900: all four `.dive-stage` blocks are 1303 px tall, 6529..11742, contiguous, each with its own `.furnace-host` 900 wide, background the furnace colour; `.gauge` is `display:none`; scrollWidth 900. `r5-qa-narrow-dive-6800.png`, `-8200.png`, `-9500.png`, `-11000.png` |
| 4 | D20 math rule callout and M5 strip show 1 px outline | PASS | Both now draw a 1 px svg `.seam` rect (stroke `rgb(38,40,46)`, 1 px, dashoffset 0) once mounted. `r5-qa-math-rule-1350.png`, `r5-qa-math-m5-10800.png` |
| 5 | D21 status bar on K1-K4 and every math explainer state, M5 without | PASS | K1 `crap · CRAP threshold exceeded: 50.5 > 9.9999 / exit 2`; K2 `dry · DUPLICATE score=0.91 · 1 pair at or above 0.80 / exit 0 · verdict is in the report`; K3 `mutate · killed 10 · survived 2 / exit 3`; K4 `tests · 2 tests with 0 kills · DUPLICATE score=0.93 x2 / exit 3`. Math: 11 states, each `<gate> · <lowercase state title>` plus exit code (e.g. `crap · count the decisions`, `tests · which test is the only one?`). M5 (`.m-parity`) has no status bar. |
| 6 | D22 covered pinned card scales, shifts and dims | PASS | Measured the sticky stage element, section untouched. Math crap to dry: stage `matrix(0.9956 .. -2.67)` at 2800, 0.9778/-13.3 at 3200, 0.9622/-22.7 at 3550; `.deck-dim` overlay 0.2 at 3300; section goes `visibility:hidden` at 3600. Dive to S9: stage scale 1.0 at 9000, 0.9911 at 9200, 0.9733 at 9600, 0.9604/-23.7 px at 9890; dim 0 to 0.297 (about 70 % brightness); hidden at 9900. Dim is an overlay (stage `filter` stays none). Reduced motion: transform none, dim 0. `r5-qa-math-deck-3300.png`, `r5-qa-dive-deck-9500.png` |
| 7 | D23 `__shaderLog` numeric firstFrameMs everywhere | PASS (see R2) | `/`: furnace, verdict flash before/after, slag hero (0.1-0.2), slag discreet; `/agents`: hero, discreet; `/math`: discreet. All numeric, none null, all `status: running`. |
| 8 | Failure glyph is plain, slag | PASS | Failure pills render `✗` (U+2717) and `⊘` in `Geist Mono`, colour `rgb(255,77,77)` = `--slag` (#ff4d4d). No U+26D4 anywhere in the DOM or CSS on any route after a full scroll. `r5-qa-home-pills-hero.png` |
| 9 | Mount slots (step e 0.05) | PASS with one minor, see R3 | Table below. Nothing visible at e = 0.30 on P1, A1 or the first dive state (kicker 0, title 0 chars, cards 0, status 0, no outline or padding paint): `r5-qa-mount-home-p1-e030.png`, `r5-qa-mount-agents-a1-e030.png`, `r5-qa-mount-dive-k1-e030.png`. |
| 10 | Regression | PASS | Copy: `/`, `/agents`, `/math` match the references; differences are only the known ones (hero stream events, counters that roll, CSS `●` and ASCII rules, dive strings split across spans), plus `<=`/`!=` rendered as `≤`/`≠` in the agents mutate table (R4). Nav links `why`, `for agents`, `the math`, `docs`, `github` present. Console: only Vite debug lines on all routes, no errors or warnings. `__streamCost`: avg 0.122 ms, max 0.7 ms, n 368. No horizontal overflow at 900 (scrollWidth 900 on all routes; one clipped `span.dot` right edge at 1090 on `/math`, harmless, inside a clipped stack). Reduced motion: every section is complete on entry (kicker 1, full title, cards 1, status 1), no transform or dim on covered cards, hero stream is a static 20 events on `/`, page height unchanged. |

### Mount slots measured (value of the scrub variable at each e)

| e | P1 home: kicker / title chars / cards | A1 agents: kicker / title / visual / cards | K1 dive: kicker / title / rows / cards / status |
|---|---|---|---|
| 0.25, 0.30 | 0 / 0 of 48 / 0,0,0 | 0 / 0 of 59 / 0 / 0,0 | 0 / 0 of 26 / 0 / 0 / 0 |
| 0.35 | 1 / 10 / 0,0,0 | 1 / 12 / 0 / 0,0 | 1 / 5 / 0 / 0 / 0 |
| 0.45 | 1 / 29 / 0 | 1 / 35 / 0 | 1 / 16 / 0 |
| 0.50 | 1 / 38 / 0 | 1 / 47 / 0.97 | 1 / 21 / 0.97 |
| 0.55 | 1 / 48 (done) / 0,0,0 | 1 / 59 (done) / 1 / 0,0 | 1 / 26 (done) / 1 / 0 / 0 |
| 0.60 | cards 0.97,0,0 | cards 0.97,0 | cards 0 |
| 0.70 | cards 1,0.88,0 | cards 1,0 | cards 0 |
| 0.80 | cards 1,1,0.67 | cards 1,0.67 | cards 0 |
| 0.85 | cards 1,1,1; status 0 | cards 1,1; status 0 | cards 0.63; status 0 |
| 0.88 | status 0.6 | status 0.6 | cards 1; status 0.6 |
| 0.90 | status 1 | status 1 | status 1 |

Kicker is fully in by 0.35 (fades inside 0.30-0.33), headline complete exactly at 0.55, visual done by 0.55 (slot 0.45-0.70), P1 and A1 cards each finish within their 0.55-0.85 slot and take at most about 12 % each, status 0 at 0.85 and 1 at 0.90.

## Remaining defects, ranked

None blocking. All minor.

**R3 (minor) · K1 dive cards reveal late.** The K1 cards variable `--ent-c` is 0 at e = 0.80, 0.63 at 0.85 and 1 at 0.88, so the card reveal runs about 0.80-0.88 instead of inside 0.55-0.85 (the code pane `--ent-r` does finish by 0.55). Cards are done before the status bar (0.85-0.90) starts to matter, and complete by 0.90, so nothing is visible-wrong to a reader. Repro: `/`, 1440x900, scrollY = 5400 - 900 x (1 - e), read `.dive-stage` custom property `--ent-c` for e = 0.80, 0.85, 0.88.

**R5 (minor) · Narrow dive status bar exit code wraps and floats.** At 900 px the dive status line wraps: `exit 3` sits on its own second line, at x about 235 in K4 and at x = 24 in K2, instead of right-aligned in the bottom row. Screenshots `r5-qa-narrow-dive-8200.png`, `r5-qa-narrow-dive-11000.png`. Repro: `/` at 900x900, scrollY 11000. Fix idea: allow the message to ellipsize and keep `.dive-st-stack` exit `margin-left:auto` with `flex-wrap: nowrap`.

**R4 (cosmetic) · ASCII comparison copy typographically changed.** The agents mutate table shows `< → ≤` and `== → !=` rendered as `≠`-style glyphs where the reference text says `<=`. If "verbatim" is meant literally these should be `<=`. Repro: `/agents`, read the "the test it demands" rows.

**R1 (observation) · Non-heading big text is below 1.25.** Numerals (`odo`, 1.0), formula terms (`.ex-term`, 1.1) and the `.wm-code/.wm-refinery` wordmark (1.2) are under 1.25. Single-line display items, no collision observed. Left as is unless the rule is meant to cover them.

**R2 (observation) · `firstFrameMs` is 0 for most shaders.** Only the hero slag field reports non-zero (0.1-0.2 ms). The others read exactly 0, which is likely a coarse-timer clamp rather than a real measurement. It is numeric, so D23 as written passes, but the 8 ms rule cannot be audited from a log that never goes above 0.2.

## Verdict

**Ship.** All of D17-D23 from round 3 are closed with measured evidence: every heading is at 1.25, the status-bar divider sits at one y on every card at both desktop sizes (831 at 900 px high, 667.1 at 720), the narrow dive is full-bleed furnace with the gauge hidden, the rule callout and M5 strip have their outline, the dive and math explainer states carry the expected status text, covered pinned cards scale to about 0.96, shift 23 px and dim to about 70 %, the shader log is numeric everywhere, the failure glyph is a plain slag `✗`, and the mount schedule matches the spec slots (nothing visible at 0.30, headline done at 0.55, status 0 at 0.85 and 1 at 0.90). Console is clean, the stream costs about 0.12 ms per frame and nothing overflows at 900. What remains is a late-by-0.03 card reveal on the first dive state, a wrapped exit code in the narrow dive status bar, and two copy and measurement nits; none needs to block release.
