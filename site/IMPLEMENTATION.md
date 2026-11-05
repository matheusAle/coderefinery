# coderefinery.dev · site implementation notes

Vite + TypeScript, no UI framework, no animation library, no analytics. The
only external request is Google Fonts (Azeret Mono 500/700, Geist 400/500,
Geist Mono 400/500/700).

## Run

```sh
cd site
npm install
npm run dev -- --port 5173 --strictPort   # http://localhost:5173/
npm run build                              # tsc --noEmit && vite build → dist/
```

`.dev.pid` holds the PID of the detached dev server started by the build agent
(`kill $(cat .dev.pid)` stops it). `.shots/` holds QA screenshots; both are
git-ignored.

## Structure

```
index.html                 fonts, <noscript> stacking rule, #app
data/intro.json            every number that rolls on the page (§11)
src/main.ts                mounts the sections in page order, wires reveals + scroll observer
src/dom.ts                 esc / html / fromHTML helpers
src/styles/tokens.css      designs/tokens.json as CSS custom properties, same names
src/styles/base.css        shell, status bars, tier-2 fade, seam draw, odometer, cursor
src/styles/sections.css    P0–P5, S9, M, footer, the single <1024 breakpoint
src/styles/dive.css        the pinned walkthrough
src/motion/scroll.ts       one rAF scroll-progress observer; viewProgress / pinProgress helpers
src/motion/odometer.ts     digit-column odometer (per-digit roll, right-most settles last)
src/motion/typer.ts        typed commands: chars = round(len × progress), clock-blinking cursor
src/motion/reveal.ts       tier-2 blocks (.t2) and "seams draw first" (.box) via IntersectionObserver
src/motion/mount.ts        §0c mount on entrance, the frozen centring of the fixed-row card layout, --u / --ut
src/motion/deck.ts         §0c / §17.4 deck stacking (desktop and compact geometry): sticky cards, the covered card's scale / rise / dim, the arrival, field hand-off, naturalTop for anchors
src/gl/shader.ts           raw WebGL 1 single-pass helper + 8 ms first-frame guard, shared rAF loop
src/gl/shaders.ts          slag field (hero), furnace (pin), verdict flash (terminals)
src/sections/*.ts          P0 hero, P1 pains, P2 helps, P3 gates, P4 tests (+ the K1 bridge), S9 numbers, M math, §08 setup, footer; common.ts has statusbar/head/loopPanes
src/code/python.ts         python tokeniser (the six syntax roles) → syntax-coloured HTML (the /math code panes)
src/dive/states.ts         K1–K4 content: session items, the three output blocks, title, hint, status bar
src/dive/pane.ts           DivePane: session pane + outputs pane, mounted once; apply(p) maps pin progress to properties
src/dive/dive.ts           400 vh pin container, sticky stage; reduced-motion and <1024 variants
```

Page order (§12, round 8): P0 hero → P1 pains → P2 how it helps → P3 gates beat reasoning → P4 the tests (its CTA types the bridge into K1) → K1…K4 (pinned, 400 vh) → S9 (§06) → M teaser (§07: headline, four-formula strip, `read the math →` to /math) → recommended setup (§08, `id="hook"`) → footer. The setup card used to be P5 between P4 and the dive.

### Third route: `/math` (§16) — four pinned explainers

```
math.html                  third Vite entry
src/math.ts                mounts M0 → crap (3 states) → dry (3) → mutate (3) → tests (2) → M5 → footer
src/math/content.ts        the four gates: formula per state, code, per-state annotations, card copy (verbatim, generated once by .tools/mathdata.py)
src/math/explainer.ts      the pinned explainer: formula · code pane · card · rail · status bar, one persistent layout
src/math/sections.ts       M0 hero, the explainers (from content.ts), M5 parity
src/styles/math.css        layout for the hero, the explainer stage (code pane, card visuals), M5
src/sections/common.ts     formulaStrip() shared by the M0 hero and the home teaser
```

Each explainer is a `states × 100 vh` container with a 100 vh sticky stage
(round 9 rewrite, §16). Persistent layout: kicker + state rail, the formula
across the top, the code pane on the left (the function the gate evaluates,
syntax-coloured, line numbers, per-line tint and a note at the right edge),
the card on the right (title, prose with inline code, the rule, a small visual
on the crap states), the status bar. The formula computes as you scroll: one
row per state from the reference (`CRAP = 9² × (1 − cov)³ + 9` →
`CRAP = 9² × (1 − 0.20)³ + 9` → `CRAP = 81 × 0.512 + 9 = 50.5`; dry, mutate,
tests likewise), cross-faded in place over the state's enter (crap starts from
the written formula `CRAP = CC² × (1 − cov)³ + CC`); the molten term is an
odometer that rolls over t 0.15–0.85 (a leading `(1 − ` stays put); earlier
substitutions are chalk in the next row. Rows wider than the stage scale the
formula down (`--fit`). Code annotations (§16.2, from the reference) tint and
pop in the order the card describes over t 0.15–0.85; the dry pairs land in
step; crap 3 and dry 3 tint the whole function(s) molten (the spec; the
reference only tints the first line). The previous state's annotations cool
over the next enter. Card visuals: nine squares light in step, the coverage
bar fills to 20 %, `50.5` (slag, rolling) beside `9` (ingot). Everything is a
function of pin progress; reduced motion: static states cross-faded; under
1024 px stacked.

M0: static headline, the shared formula strip (seams draw, hover molten
seam). M5 (lighter): five numbers roll from 0 when the strip enters view, the
molten `36/36` last; the four determinism items and the CTA appear at once;
`read the specs →` links to the specs on GitHub (placeholder org).

### Second route: `/agents` (§15)

```
agents.html                second Vite entry (rollupOptions.input in vite.config.ts)
src/agents.ts              mounts A0 → A1 → A2 → A3 → A4 → footer
src/agents/sections.ts     the five screens, copy verbatim from designs/reference/agents-reference.html
src/styles/agents.css      layout for the five screens
src/sections/topbar.ts     wordmark + nav shared by both routes ("for agents" item, active state)
src/gl/slag.ts             slag-field shader mount shared by both heroes (peak 1 on /, 0.5 on /agents)
```

Vite's dev server maps `/agents` to `agents.html`; `npm run build` emits
`dist/agents.html` (serve it at `/agents` with clean URLs on). Motion per §15,
strictly tier 2 except where noted: A0 table draws its seams and rows appear at
once; A1 step boxes draw their seams 80 ms apart and each arrow draws as an SVG
stroke after the box to its left (one IntersectionObserver on the list), the
two panes appear at once and only the molten CRAP line and `exit 2` line land
200 ms later; A2 outer seam then rows 40 ms apart, nothing rolls; A3 both
tables as A2, hover/focus on a mutant row lights a molten seam on the rule row
it pairs with and vice versa; A4 three cells open along their left seam (clip,
60 ms apart), the CTA draws its molten stroke and `see the hook →` links to
`/#hook` (the §08 setup card on the home page, now the last card). Rows are focusable so the A3 pairing works
from the keyboard. The home page's topbar gained the "for agents" item.

### The dive's two panes (§13.3, round 9)

No code pane in the dive any more (the code lives on /math). The left pane is
the agent's session: the first command (typed in the previous state's bridge;
K1's types over the pin's first 6 %, after P4's CTA typed it), the gate's
output landing line by line from t 0.12, the agent's reading (revealed at
0.34), the tool calls over 0.42–0.60 (header types, body lines reveal, `done`
lands), the re-run typing at 0.62 and its lines landing. The right pane holds
three blocks per state: `before` (the gate's table or pair, numbers rolling,
its exit landing with the session's), `what the agent changed` (diff summary
lines in −/+ tones and the `−12 +9 · 2 files` footer, revealed over the tool
calls), `after` (the re-run's rows and `exit 0`). All four states' content is
mounted once in a stack per pane and cross-faded on enter. Every exit line in
either pane fires the verdict flash once (`__flashLog` keys `s<state>:<n>` and
`o<state>:<n>`, panes `session` / `outputs`); shader names are
`verdict flash (session)` and `verdict flash (outputs)`. Hint, furnace, gauge,
progress bar, full bleed and the status bars are unchanged; the K4 preview
note is part of its status bar now.

### Scroll geometry (§1, §13.2)

The pin is a 400 vh container with a 100 vh sticky stage. `pinProgress` gives
`p`; state `s = floor(4p)`, local `t`. Phases per state: enter 0–0.12 (layout
lerp, heading/file-name roll), work 0.12–0.70, settle 0.70–0.85 (verdict +
exit, hint rises with its stroke drawing), bridge 0.85–1.00 (the next
state's command types in the "before" terminal, `↓ types as you scroll` label
on the first two stops only). Mapped ranges use `easeOutCubic`; no springs.
Every value is a pure function of scroll and reverses when scrubbing back,
except the one-shot verdict flash and the cursor blink (clock, per spec).

The bridge into K1 types in P4's CTA button over the last 10 % of P4's
entrance (`tests.ts`); the dive card then slides over P4 and pins. There is no
session-pane hand-off any more: the code pane and terminals arrive with the
dive card's own slide (the old `setup.onBridge → dive.setApproach` path and the
code pane's `--slide-in` are removed).

### Tiers (§0b)

- Tier 1 (scrubbed/typed): headlines in the dive, kicker number, every number
  that changes meaning (hero stats, P1/P3 stats, P3 kills, CRAP 0→50.5, 0.91,
  killed/survived, 0.93, S9 numbers), typed commands and tool calls, tinting /
  striking / inserting / moving lines, patch hunks, depth gauge, exit codes.
- Tier 2 (`.t2`): body copy, quotes, bullets, agent messages, hint prose,
  table rows without numbers — one 120 ms fade, 8 px rise, on entering view.
- Tier 3: nav, status bars, file bars, kickers — static.

### Shaders (§14.4)

Three fragment shaders, WebGL 1, one full-screen triangle each, no framebuffer,
all rendered behind content. All are skipped under `prefers-reduced-motion`,
`navigator.connection.saveData`, missing WebGL, and when the first frame takes
more than 8 ms (`gl.finish()` timed); the CSS gradient (`.heat-glow`) is the
fallback. The furnace intensity is `0.3 + 0.7 × depth`; peak ground luminance
is capped at 12 % in the shader; scanline 1 px every 3 px at 4 %. The verdict
flash runs once per exit landing (set of fired keys) and never replays.

### Responsive and reduced motion

- One breakpoint at `< 1024 px`: everything stacks; the walkthrough unpins and
  renders the four settled states as static snapshots (same `DivePane` class
  with `snapshot: n`), code pane above the terminals, hint docked under the
  title, furnace ground and depth gauge kept.
- `prefers-reduced-motion: reduce`: no odometers, typing, or line slides; the
  pin shows four settled snapshots cross-faded by scroll; tier-2 blocks appear
  without transition; shaders off.
- No JS: `<noscript>` rules un-stick the pin; the page is readable as text.

## Hero (§12 P0, redesigned) and the slag field at DPR 2

`src/sections/hero.ts` + `src/sections/stream.ts`: left column (headline,
"Prove it." at 200 ms, mechanism, install line, foot), right column the four
big numbers as a 2 × 2 block (rolling on load, molten last), and behind
everything, above the slag field, the event stream. The stream lives in the
hero's full-width card wrapper (not the centred column), so it spans the
viewport edge to edge at any width and is clipped to the hero card.

- Events: pills (glyph · message · PR number) from a 32-string vocabulary in
  the five tones; one spawns every 0.35–0.7 s (1 s under 1024 px) at a random x
  across the full width, at the bottom edge of the content (the status bar's
  top), rising at 42 px/s on the shared rAF clock. Spawn x keeps 24 px of
  horizontal clearance from every live event of the same depth layer that
  shares its row (a spawn that cannot fit waits a frame). Running events carry
  `▂▃▅▆` + elapsed from spawn and tick each second; PR numbers rise with a base
  that increments every five spawns. Budget ≤ 30 events / ≤ 40 sparks.
- Fade band (D16): derived from the live rects of the foot line, the lede and
  the status bar on every resize (ResizeObserver on the card, lede and foot) and on font load. Full stream
  opacity (50 %) only once a pill's top is one pill-height (26 px) below the
  foot line; 0 once its top reaches the lede's bottom, so nothing is drawn over
  the mechanism paragraph; 0 while it overlaps the status bar, ramping to full
  over one pill-height above it. Measured max opacity over the lede 0, over the
  status bar 0, over the foot line ≈ 0.41 (< 0.5 full) at 1280 × 720, 1440 × 900,
  1920 × 1080 and 900 px. The eight pipes obey the band.
- Failures (1 in 8; glyphs ✗ and ⊘) blink twice (120 ms) with a 10 px 27 % slag halo for 2 s,
  started when the pill rises into view; 1–3 molten 2–3 px sparks per event (6
  on a failure) spawn as it appears, rise at 1.5×, drift ±6 px, fade over 1.6 s.
- Parallax: depth layer per event at spawn, near 30 % (scale 1, speed 1×, band
  opacity 1×), mid 45 % (0.85, 0.7×, 0.7×), far 25 % (0.7, 0.45×, 0.45×);
  sparks follow their event; near above mid above far (z-index per layer).
  Scroll: as the hero card is covered (deck) or scrolls out (no deck), layers
  shift up by 1.0× / 0.6× / 0.3× the distance. Pointer: ∓12 / ∓6 / ∓2 px from
  the viewport's left to its right edge, eased with τ = 133 ms (≈ 400 ms).
- Hover (pointer only, `pointerType === "touch"` ignored): hit-tested in JS
  over the pills' parallaxed rects on the shared frame (no DOM queries), near
  and mid layers only, the install box (now opaque coke) and the numbers block
  excluded. The hovered pill pauses, goes to opacity 1 with a molten border and
  shows a one-line detail beneath it (`PR #466 · opened 23 min ago · 2 checks ·
  1,204 tests`; failures show their reason, e.g. `exit 3 · survived 2 ·
  pricing.py:6, :9`), its blink/halo animation is cancelled and its sparks
  freeze; on leave it resumes and settles to band opacity over 300 ms.
- Cost: `window.__streamCost` {sum, n, max, avg, recent} in ms of JS per
  frame (`recent` = the last 600 frames). Measured over ~14 s with all layers,
  hover and parallax: 1440 × 900 avg 0.16 ms, p50 0.1, p99 0.9, 1 frame of 600
  at 1.2 ms; 1280 × 720 avg 0.13 ms, p99 0.6, max 1.0 (timer resolution
  0.1 ms). The pre-seed spawns three per frame; the band re-measures on
  ResizeObserver / resize / font load, not on the clock. The stream stops drawing while its card is fully
  covered (visibility hidden) or off-screen.
- Reduced motion: a static arrangement of up to 20 events in rows across the
  band (rows never cross the foot line or the lede; 20 at 1440 × 900, 18 at
  1280 × 720 where the band is shorter), layers keep scale and opacity, no
  sparks, blinks or parallax shift; hover still shows the detail.

Status bar unchanged ("… rubber-stamped · scroll ↓"); the refused-commit
session lives on P5 only.

Slag field (user report on retina): the canvas host is now the full-width
`.hold` wrapper (not the centred column), the backing store is the host rect ×
`devicePixelRatio` (≤ 2) with `gl.viewport` reset on every change (a
`ResizeObserver` on the host), the pointer is converted from the canvas rect
into device pixels with y flipped, heat radius 280 CSS px (168 for the
discreet field), peaks 6 % / 2 %, lag ≈ 300 ms. Verified in a headed session at
DPR 2 (`.shots/dpr2-hero-a.png`, `dpr2-hero-b.png`).

## Section rhythm (§0b, §0c "mount on entrance, no hold") and QA round 1

`src/motion/mount.ts` wraps every unpinned section in a plain 100 vh block
(`.hold`, `overflow: hidden`). With entrance progress e = 0 when the section's
top is at the viewport bottom and e = 1 when it reaches the viewport top, the
mount stagger runs over e ∈ [0.30, 0.90]: kicker typed → headline typed
(`typed-title.ts`: one span per character so wrapping is pre-computed; the
cursor is a pseudo-element on the last typed character) → primary visual →
cards/columns in DOM order (`.rv`: `max-height` 0 → measured natural height
with an 8 px rise, a top-down clip) → captions (faded) → status bar (faded),
slots overlapping by half. It docks fully assembled and the last 10 % is quiet.
Scrolling back before e = 0.90 reverses; completion is latched, and outgoing
sections never animate. Numbers roll inside their block's slot via
`onMount(section, (q, slot) => …)`. The pinned sections (K dive, math
explainers) keep their pin geometry; their first state's title (and the
explainers' first card) mount over the same entrance range (`setEntrance` /
`stage.entrance`) so the pin begins assembled. Reduced motion: complete on
entering view. Under 1024 px sections flow naturally and are complete. Type
and spacing scale with `--u` (JS clamp of `min(100vh/900px, 100vw/1440px)`
into [0.72, 1]) and `--ut` (≥ 0.85 for body text).

### Page geometry (no hold; every unpinned section is 100 vh)

- 1440 × 900: `/` hero 0 · P1 900 · P2 1800 · P3 2700 · P4 3600 · P5 4500 · K dive 5400–9900 (500 vh) · S9 9900 · M teaser 10800 · footer 11700; `/agents` A0 0 · A1 900 · A2 1800 · A3 2700 · A4 3600 · footer 4500; `/math` M0 0 · crap 900 · dry 3600 · mutate 6300 · tests 9000 · M5 10800 · footer 11700. Totals are measured in the report.
- Every section's content fits its 100 vh at 1440 × 900 and 1280 × 720 (measured `section.scrollHeight ≤ innerHeight` with reveals open); nothing is cut and nothing had to be split.

QA round 1: D1 sections clip overflow (`.sec-sticky { overflow: hidden }`),
D2 `.tl.slag/.ingot/.molten/.ember` rules (the tones were in the data but the
`.tl` colour out-specified them), D3 snapshots complete every typer, hide the
bridge prompt, and the gauge is driven across the four static states
(`setDepth`), D4 the preview line keys off `data-last` only, D5 the dive is
500 vh (400 vh of scrub, one full viewport per state), D6 hint box
`align-self: start` and chalk prose, D7 the P6 bridge line types under the P5
transcript as the section leaves and K1's terminal starts empty, D8 `→ ≤ ≠`
in transcripts and tables while the code pane keeps `<=`/`!=`, D9 the shader
guard warms two frames, measures the third against 12 ms and logs the shader
it drops, D10 odometers keep the real number in a visually hidden span and
draw strips with pseudo-elements (innerText and the accessibility tree read
`441`).

### Page geometry (1440 × 900; every hold is 160 vh = 1440 px, the hold starts where the section's top reaches the viewport top and lasts 60 vh = 540 px)

- `/`: P0 hero 0 (mounts on load, 900) · P1 hold starts 900 · P2 2340 · P3 3780 · P4 5220 · P5 6660 · K dive 8100–12600 (500 vh; scrub 8100–11700, one viewport per state: K1 8100, K2 9000, K3 9900, K4 10800) · S9 12600 · M teaser 14040 · footer 15480 · total 15553 px.
- `/agents`: A0 0 · A1 900 · A2 2340 · A3 3780 · A4 5220 · footer 6660 · total 6733 px.
- `/math`: M0 0 · crap 900–3600 (3 states) · dry 3600–6300 · mutate 6300–9000 · tests 9000–10800 (2 states) · M5 10800 · footer 12240 · total 12313 px.
- At 1280 × 720 (`--u` = 0.8) the holds are 1152 px (mount range 432 px): `/` 12454 px, `/agents` 5398 px, `/math` 9862 px. Every unpinned section's content fits its 100 vh at both viewports (measured: `section.scrollHeight ≤ innerHeight` with reveals open); nothing had to be split.

## QA round 2 (round 3 fixes), verified in the browser

- D16 — see the stream's fade band above. `.shots/r4-d16-1280x720.png`, `r4-d16-1440x900.png`.
- D4 — inactive reduced-motion snapshots are `visibility: hidden`; the K4-only preview line is visible at dive +3100 / +3550 only (0 at +400 / +1300 / +2200). `.shots/r4-d4-rm-1300.png`, `r4-d4-rm-3550.png`.
- D12 — a revealed block's clip opens over `CLIP = 0.12` of the mount range from its slot start; measured 60 px of the 540 px range for P1's second column, its stat starts rolling 15 px after the clip is open; slots keep their stagger. `.shots/r4-d12-p1-e52.png`.
- D13 — M5's five numbers roll from 0 inside the strip's post-clip slot (e 0.57 → 0.72), the molten `36/36` settling last.
- D14 — `window.__shaderLog` (array of `{ shader, status, firstFrameMs, reason }`) and `window.__flashLog` exist on `/`, `/agents`, `/math`; scrubbing the dive logs all eight exit landings (`b0…a3`, slag / ingot, `gl`), and the flash band is visible in `.shots/r4-d14-flash-b0.png`.
- D8 remainder — `if total <= 0:` stays real code in the pane (decision). D15 — the discreet field runs on /math; no furnace or verdict shaders there.

## Deck stacking and the fixed-row card layout (§0c, round 4)

`src/motion/deck.ts`. Every top-level child of `main` except the footer is a
card: `position: sticky; top: min(0, 100vh − height)` (so 100 vh cards stick as
they dock and the dive / explainers stick once their own pin has run out),
`z-index: 0; isolation: isolate; overflow: clip` and an opaque coke ground.
The last card before the footer stays `position: relative`. Each incoming card
draws a 1 px seam on its top edge (`::before`), the footer too. Over the
incoming card's entrance e (its top from the viewport bottom to the top) the
covered card's face (the card, or the dive's / explainer's sticky stage)
scales 1 → 0.96 about its top-centre, rises 24 px and dims to 70 % (a black
`.deck-dim` overlay at up to 30 %); at e = 1 it is `visibility: hidden` and
drops its frozen field frame. Page heights are unchanged by the stacking
(home 11 771 px, /agents 4571, /math 11 771 at 1440 × 900; the 2 px against
round 3 come from the `--u` change below). Reduced motion: sticky stacking, no
scale / rise / dim. Under 1024 px the deck is off and the cards flow.

Fields in the deck: the hero's (and /agents' A0) field pauses (keeps its last
frame) while its card is covered. The discreet field's canvas is handed to the
face of the active card (the incoming one while a card slides), so it is
clipped to that card; the card it leaves gets a 2D copy of the frame just drawn
(`canvas.gl-freeze`, the "paused" field under the dim). When the active card
takes no field (the dive) the canvas stays where it is and stops drawing. The
K1 bridge types in P4's CTA (round 8; it used to come from P5).

Card layout, fixed rows (≥ 1024 px): sections are flex columns; the kicker is
the first row (under the topbar on the heroes), the status bar is pushed to the
bottom edge (`margin-top: auto`), and the headline opens the middle row with a
top margin that centres the content in the remaining space. That margin is
measured with every reveal open and frozen as `--mid` (mount.ts), so the
headline never moves while blocks expand. Head wrappers (`.numbers-head`,
`.math-head`, `.a-hero-head`, `.m-hero-head`) are `display: contents`.
P5's typed tool-call lines reserve their line before typing. Pinned cards: the
dive keeps its head (kicker + title + hint) as the top row and centres the
panes below it; the explainers keep kicker + rail on top and centre the formula
and the card/diagram row; neither has a status bar (the dive's 4 px progress
bar sits on its bottom edge). Deviation: the dive title stays in the head row
rather than opening the middle row, because the hint box shares that row.

Typography (§0b): every title — `.h2`, `.hero-h1`, the hero / parity / teaser
h2s, `.dive-title`, `.ex-title`, `.measure-q`, `.gate-q`, `.cta-h` — is at
line-height 1.25. To keep every card inside 100 vh, `--u` now clamps the
viewport against 1440 × 920 (u = 0.978 at 1440 × 900, 0.783 at 1280 × 720), the
P4 slab width follows `--ut` (its text does) and P4's row gap is 26 instead of
30 × u². Fit, measured with reveals open: every unpinned card on all three
routes has scrollHeight = 100 vh at 1440 × 900 and 1280 × 720 (P5's status line
sits 8 px lower than the others at both sizes, still inside the card); the
dive's panes end at 703 px of 720 at 1280 × 720 (stage padding overflows, no
content clipped), the explainer stages fit.

Dive full bleed (§13.1): the stage is full width with the content column kept
by a side margin (`--side`), so the furnace ground, the heat glow, the
scanlines and the furnace canvas cover the viewport edge to edge at 1280,
1440 and 1920 px; the depth gauge moves with the column; stacked states under
1024 px are full width too. `.shots/r4-dive-K2-1920x1080.png`.

## QA round 3 (D17–D23) and the explicit mount slots (round 5)

- Mount slots (§0c, explicit): `SLOTS` in `mount.ts`, in e — kicker fades 0.30–0.33 (no longer typed), headline types 0.30–0.55, primary visual 0.45–0.70, cards/columns 0.55–0.85 staggered in DOM order with each clip `CLIP` = 12 % of the range, captions 0.80–0.88, status bar 0.85–0.90; complete at e = 0.90. A collapsed block is at opacity 0 (its padding and seam no longer peek out before its slot). The dive's and each explainer's first state use the same table (`--ent-k / --ent-r / --ent-c / --ent-s` on the stage: kicker, panes or formula + diagram slab with a top-down clip, gauge, status bar). Stepped at e += 0.05 on P1 and A1: nothing visible before 0.30, kicker 1 at 0.35, headline complete at 0.55, cards open in turn 0.55–0.85, status bar 0 at 0.85 and 1 at 0.90. `.shots/r5-stagger-strip-p1.png`, `r5-stagger-strip-a1.png`.
- D17 — `.numbers .h2` was a fixed 56 px line; now 1.25. A sweep of every h1–h4 / title ≥ 20 px on all three routes at 1440 and 900 px finds none below 1.25. `.shots/r5-d17-s9-9900.png`.
- D18 — the bottom row is identical on every card: hero, dive stage and explainer stages end with the sections' `40px·u²` padding, the status bar is one fixed box (`12px·u + 19px`, one 18 px line, items centred) at ≥ 1024 px, P5's status bar lost a stale 8 px `translateY` from the transcript-line rule, and the M0 sources line stays on one line (−0.03 em). Measured divider at 831 px (1440 × 900) and 667 px (1280 × 720) on all 19 cards that have a bar. `.shots/r5-d18-p5-4500.png`.
- D19 — under 1024 px the depth gauge is hidden and the stage padding follows; every stacked state carries its own full-width furnace ground and CSS heat glow (no WebGL per stacked state; the furnace canvas is a pinned-mode layer). `.shots/r5-d19-narrow-900-k1k2.png`, `r5-d19-narrow-900-k4.png`.
- D20 — boxes that clip their own overflow (every revealed `.rv.box`, the explainer `rule` callout) clipped their outer seam; they now draw it inside their border box. Restores the rule outline and M5's strip outline. `.shots/r5-d20-d21-math-crap-1.png`, `r5-d20-math-m5.png`.
- D21 — dive states carry a status bar (`● <gate> · <verdict>` / exit code, per state in `STATES[].status`, cross-faded with the state, exit codes right-aligned); explainer states carry `● <gate> · <state title, lowercase>` and the gate's exit code (crap 2, dry 0, mutate 3, tests 3). M5 as designed (no bar). The dive's panes zoom with `min(1, u / 0.88)` so head + panes + bar fit 720 px. `.shots/r5-d21-dive-k1.png`, `r5-d21-dive-k2.png`.
- D22 — already in place since round 4 (the deck transforms the pinned card's sticky face): crap → dry at 3150 and dive → S9 at 9450 measure `scale(0.98) translateY(−12px)` and dim 0.15 at mid-cover. `.shots/r5-d22-math-crap-dry-3150.png`, `r5-d22-dive-s9-9450.png`.
- D23 — `createPass` completes the §14.4 guard two frames after mount for every pass that has not reached its measured frame (warm two, time the third, clear if nothing real was drawn), so `firstFrameMs` is a number for the furnace, both flashes and the discreet fields on every route.
- Stream glyph: ⛔ has no text presentation on macOS (U+FE0E is ignored by Apple Color Emoji), so the blocked glyph is ⊘ (U+2298), monochrome in the slag tone. `.shots/r5-stop-glyph.png`.

## QA round 4 leftovers and final state

- R3 — the K1 entrance's card variable (`--ent-c`, the gauge layer) now runs over the cards' slot, e 0.55–0.85 (measured 0 at 0.55, 0.5 at 0.70, 1 at 0.85), like every other card.
- R5 — under 1024 px the dive and explainer status bars stay one row: the verdict ellipsizes, the exit code keeps its place at the right edge (900 px: K2 `exit 0 · verdict is in the report` ends at 876 = bar right, K4 `exit 3` likewise, same line; 600 px: verdict truncated, code on the line). `.shots/r6-r5-narrow-dive-k2.png`, `r6-r5-narrow-dive-k4.png`, `r6-r5-narrow-dive-600.png`.
- Left as is by decision: R1 (single-line display numerals, formula terms and the wordmark below 1.25), R4 (`≤` / `≠` in the agents mutate table, per the glyph decision). R2 noted: `firstFrameMs` reads 0 for most passes because the browser clamps `performance.now()` to 0.1 ms; the value is numeric and the 12 ms guard still applies.

Final state (QA round 4: ship). Three routes (`/`, `/agents`, `/math`), Vite + TypeScript, no runtime dependencies. Every section is a 100 vh card in a sticky deck (covered cards scale, rise, dim and hide; pinned cards included), laid out in three fixed rows (kicker, centred content, status bar at one shared height) that fit 1440 × 900 and 1280 × 720. Cards slide in first (scale, brightness, molten seam over e 0–0.60), then mount in the §0c slots (nothing before e = 0.60, complete at 0.90). Titles are at line height 1.25. The hero's event stream is full width, density 0.35–0.7 s at 42 px/s, three parallax layers, a live-measured fade band, pointer hover with detail lines; about 0.12–0.16 ms of JS per frame. The dive is full bleed with per-state status bars; the slag, furnace and verdict-flash shaders report to `window.__shaderLog`, landings to `window.__flashLog`. Reduced motion and < 1024 px variants hold on every route.

## Entrance: the slide first, content from e = 0.60 (round 7, supersedes the round-5 slot numbers)

- The slide (deck.ts, `arrive`): while an incoming card's own e runs 0 → 0.60 it shows no content, only its arrival: its face scales 0.97 → 1.00 about the top-centre, brightness 0.55 → 1.0 (the deck's black overlay at 45 % → 0), and its top seam goes from molten at 60 % to the seam colour (`--seam-glow` 1 → 0, a `color-mix` on the `::before` seam). The discreet slag field is already in the card from the first pixel (the deck hands it to the arriving card); the dive arrives on its furnace ground. The covered card does its deck motion at the same time. Transforms, overlay opacity and one custom property only, no layout. The hero (card 0, mounts on load) never arrives and is untouched. Reduced motion: no slide effect, content complete on entry.
- Slots (`SLOTS` in mount.ts, in e): kicker fades 0.60–0.63, headline types 0.60–0.74, primary visual 0.68–0.80, cards 0.72–0.86 staggered with each clip 7.2 % of the entrance (`CLIP` 0.12 of the 0.60–0.90 range), captions 0.84–0.88, status bar 0.86–0.90; complete at 0.90. Same table for the dive's and each explainer's first state (kicker, title, panes / formula + slab, gauge, status bar).
- Verified stepping e += 0.05 (1440 × 900): P1, A1 and K1 show scale 0.970 → 0.9975, dim 0.45 → 0.04, glow 1 → 0.08 over 0–0.55 and nothing else; at 0.60 the slide is done and content is still 0; kicker 1 at 0.65; title complete by 0.75; blocks open 0.75–0.85; status bar 0 at 0.85, 1 at 0.90. Scrolling back from e = 0.50 to 0.25 and to the top restores scale 0.9825 / dim 0.262 / glow 0.583 and then the waiting state, with the hero's transform back to none. Strips: `.shots/r7-entrance-strip-p1.png`, `r7-entrance-strip-a1.png`, `r7-entrance-strip-k1.png`.

## Home order change (round 8): setup last, the bridge from P4

- The recommended-setup card moved from between P4 and the dive to the end: P0 … P4 → K1–K4 → S9 (§06) → teaser (§07) → setup (§08 · recommended setup · two minutes) → footer. Its content and the refused-commit transcript are unchanged; it is the deck's last card (not sticky, the footer scrolls in under it), and `/#hook` from /agents lands on it (scrollY 10 800, card top 0 at 1440 × 900). `.shots/r8-agents-hook-link.png`, `r8-teaser-setup-footer-1440x900-0.5.png`, `-1.0.png`, `r8-teaser-setup-footer-1280x720-0.5.png`, `-1.0.png`.
- The bridge: over P4's entrance e 0.90 → 1.00 (the quiet last 10 %, right before the dive card arrives) `$ coderefinery crap --changed` types inside the CTA button, replacing its "keep scrolling ↓ · the gate opens" label in the same grid cell (button width unchanged: 315 px at 1440, 280 px at 1280); scrubbed, so it reverses. Then the dive card slides over P4 (P4 covered, scaling and dimming) and pins. `.shots/r8-p4-bridge-1440x900-e1.00-crop.png`, `r8-p4-bridge-1280x720-e0.95-crop.png`, `r8-p4-dive-1440x900-e0.3.png`, `r8-p4-dive-1280x720-e0.6.png`.
- Removed with the hand-off (dead code): `SetupHandle.onBridge`, the `.p6-bridge` line, the session pane's `--slide`, `DiveHandle.setApproach`, `DivePane.setApproach` / `codePane` and the code pane's `--slide-in`.
- Fit, re-measured with flex shrinking disabled (the earlier probe could be fooled by flex items shrinking): P4 was 5 px (1440 × 900) and 32 px (1280 × 720) over its natural height; its section gap is now 20 × u², the slab body gap 8 × u and the CTA's vertical padding 12 × u, and every unpinned card on all three routes now fits at both sizes. Pinned stages: content ends above the status bar (dive 649 / 667 px, explainers 644 / 667 px at 1280 × 720).

## Round 9: /math shows the code, the dive shows the agent; inline code

- /math (§16, §16.1, §16.2) and the dive (§13.3) rewritten as described above; copy verbatim from the regenerated `math-reference.html` and `page-reference.html` (the code listings were checked token by token against the reference, whose export drops whitespace). Removed with the old layouts: `src/math/diagrams.ts` (the eleven diagrams), `src/dive/model.ts` (the line registry; the tokeniser moved to `src/code/python.ts`), the dive's code pane, patch pane and before/after terminals and their CSS, the `.dg*` diagram styles.
- Inline code (§0b): `prose()` in `dom.ts` turns `backticked` spans into `<code class="ic">` (mono, chalk on a slab tint, 0.92 em). Used by every math card body and rule, and on the home cards and /agents where a variable or computed value sits in prose: the P2 gate cells and lists (`CRAP`, `≥ 10 → exit 2`, `≥ 0.8`, `exit 3`, `--changed`), the A1 loop steps, the A2 "what it does" column, the A3 rule evidence, the A0 properties. Text is unchanged; only the marking is added.
- Fit at 1440 × 900 and 1280 × 720: every dive state's content ends above its status bar (708 / 831 px and 616 / 667 px), every math state likewise (602 / 831, 514 / 667 at most).
- Screenshots at 1440 × 900, t ≈ 0.85: `.shots/r9-math-crap-1.png` … `r9-math-crap-3.png`, `r9-math-dry-1…3.png`, `r9-math-mutate-1…3.png`, `r9-math-tests-1.png`, `r9-math-tests-2.png`, `r9-dive-k1.png` … `r9-dive-k4.png`; narrow and reduced motion: `r9-narrow-dive-k3.png`, `r9-narrow-math.png`, `r9-rm-math-dry2.png`, `r9-rm-dive-k3.png`.

## Round 10: scroll runway, text before its box, breathing room (§0c, §14.1)

Runway (mount.ts, deck.ts, base.css). Every unpinned section is a 150 vh `.hold` wrapper with the 100 vh card
(`.hold-sticky`) sticky inside; the card docks after 100 vh and holds 50 vh. In the deck the wrapper itself sticks at
`top = 100vh − 150vh`, so the held card stays while the next one slides over; the deck's face is the inner sticky card.
Mount progress `m = (vh − wrapperTop) / 1.5 vh`. The hero keeps its timed mount (no runway). Page heights at
1440 × 900: `/` 14 902 px, `/agents` 6352, `/math` 12 202 (each unpinned section +450 px).

| m | what |
|---|---|
| 0–0.40 | slide only (deck: scale 0.97 → 1, brightness 0.55 → 1, molten seam cooling; covered card scales/rises/dims over 0–0.667) |
| 0.40–0.43 | kicker fades |
| 0.40–0.56 | headline types |
| 0.52 + i·0.28/(n−1) | block i starts (reading/DOM order), spread evenly so the last block's support text lands at 0.88 (round 10b) |
| block + 0–0.05 | (1) its headings type, its main text reveals (top-down clip, 8 px rise) |
| block + 0.03–0.07 | (2) the container: surface tint, borders and `.box` seams, cell by cell in DOM order, nested boxes after their parent |
| block + 0.05–0.08 | (3) support text fades (muted/ash text, `data-support`) |
| block + 0.08 → | numbers roll (`slot(el)`) |
| 0.84–0.88 | captions fade |
| 0.88–0.90 | status bar fades; complete at 0.90, the last 10 % is quiet |

Step 2 is generic: on first measure each block records the stylesheet background and border colours of itself and its
descendants and the step scrubs them in with `color-mix(… p %, transparent)` inline (removed when complete); `.box`
seams follow `--bx` (scrubbed, no clock). Step 3 uses `filter: opacity(var(--sup))`, independent of the elements' own
opacity choreography. The pinned sections map their entrance onto the same table (`pinnedMount(e)`: slide to
e 0.60, complete at the dock) and add `--ent-box` / `--ent-sup` so their panes' surfaces, seams and support text arrive
after the words. The hero's stats strip and install box run the same order on the load clock (surface and seams at
200 ms, sources at 440 ms, figures roll from 500 ms). P4's K1 bridge types over m 0.90–1.00 (its hold).

Breathing room: floors as CSS variables (`--pad-card` 28, `--pad-tile` 24, `--gap-title` 12, `--gap-row` 10,
`--gap-support` 16, `--cell-v` 14 / `--cell-h` 18, `--gap-sib` 24, `--gap-fig` 8, all × `--uf = max(--u, 0.8)`), status
bar 20 px × `--uf` above every card's bottom edge. Widened: pane bars (home, agents, math, dive), the P3/A1 transcript
panes (`.a-lines`, `.a-line`) and the P5/home transcript, transcript agent/tool boxes, hero install box and stat tiles
(`.bn`), P1 columns (title→text, list rows, column gap), P3 table cells (`.vs3-row`), P4 stat tiles (`.tstat`), pattern
list, measure pane body and its table (`.m2t-row`), CTA slabs (home, teaser, /agents, M5), P2 gate cells and both
lists (`.hlist-head/-row`), P5 hook file and notes, S9 big-number tiles and facts, teaser formula cells (`.fcell`),
A0 property rows, A1 loop steps, A2 verdict table cells, A3 rule table bar and rows, A4 cells, /math code pane (bar,
lines), rule callouts, card visuals, M5 tiles and items, dive hint boxes, session pane, agent/tool boxes, outputs
blocks. Fit after the pass (natural height, flex shrinking disabled): P3, P4, P5 and A3 no longer fit, so they take one
type step down (`.type-step`: `--u` and `--ut` × 0.9, paddings keep their floors); P4 then still did not fit and shows
one fewer mutant row in its slab (`1 → 0`, the fourth); P5 tightens its section rhythm (gaps between kicker, headline,
panes and status) at 720 px. Every unpinned card on all three routes fits 1440 × 900 and 1280 × 720; every dive and
explainer state ends above its status bar (dive 779 / 850 and 647 / 676; math ≤ 624 / 850 and ≤ 539 / 676).

Screenshots: `.shots/r10-p1-m0.45.png`, `-m0.60`, `-m0.75`, `-m0.90`, `r10-p4-m0.45.png` … `-m0.90`,
`r10-math-m1-entrance.png`, `r10-math-m1-docked.png`, `r10-dive-k1-entrance.png`, `r10-dive-k1-docked.png`.

## Round 10b

- The dive begins assembled (§0c Exceptions): K1's opening content — its command, the gate's two output lines, the
  agent's reading in the session pane and the `before` block in the outputs pane — mounts with the entrance (pinned
  mount m: the title types 0.40–0.56, those words reveal 0.56–0.65 in order, the panes' surfaces and seams draw
  0.67–0.71, the pane bars and block labels fade 0.71–0.74, the status line 0.88–0.90). K1's pin then starts at the
  tool calls (`K1_SCHEDULE`: tools 0.08–0.36, re-run 0.44); K2–K4 keep their schedule. The `exit 2` flashes fire as
  they land in the entrance. At the dock: 4 session lines and the full `before` block. `.shots/r10b-dive-k1-docked.png`.
- Block starts spread evenly over the runway: `start_i = 0.52 + i × 0.28 / (n − 1)`, each block 0.08. P4 (5 blocks)
  measured: block 1 text at 0.56, block 2 at 0.62, 3 at 0.68, 4 at 0.74, 5 at 0.80, its support at 0.88.
  `.shots/r10b-p4-m0.80.png`.
- Recorded as content and spec cuts from round 10 (left as they are): P4's slab shows three mutant rows (the fourth,
  `1 → 0`, was dropped to fit with the spacing floors), and the §08 setup card tightens its section rhythm at 720 px.

## Round 11 (QA round 5 + the bottom-of-page leak)

- Math first state (QA D2): nothing but the slide before m 0.40 — the first card exists from the title's slot on, its
  clipped body / rule / visual are at opacity 0 while collapsed, and the card's molten rule, the rule box seam and the
  visual's slab arrive with the boxes (`--ent-box` 0.62–0.68) after the words (card words 0.56–0.61, formula and code
  0.52–0.57), support (code bar, line numbers, `rule` label) 0.68–0.72. Measured at e 0.35 and 0.50: no visible text,
  border or seam on the crap card. `.shots/r11-math-crap-e0.50.png`, `r11-math-crap-e0.70.png`.
- Inline code (QA D3), wrapped: dive hints K1 `81 × 0.512 + 9 = 50.5`, K2 `20 of 22`, `0.91`, `0.8`; dive agent
  readings K1 `price()`, `_item_cost`, `_apply_tier`, K2 `quote()`, `price()`, `_settle(items, tier, discount)`, K3
  `max(total, 0)`, `0`, K4 `isinstance()`, `_settle`; math cards crap 2 `3 of 15`, `80 %`, dry 2 `20 of 22`, dry 3
  `0.82`, `0.8`, `0`, mutate 3 `83 %`, `exit 3`, tests 1 `4, 1, 5, 0, 0`, tests 2 `kills 5`; §08 setup list
  `npx skills add coderefinery`, `exit ≠ 0`, `.metrics/` (the `&nbsp;` gaps replaced by a colon or plain space); P2
  "what it refuses" `assert True`, `90 %`; /agents A0 `.metrics/*.edn`, A4 `.metrics/`. Left as words: "zero kills"
  (a phrase, not a value).
- Math pin length (QA D6): `.explainer` is `(states + 1) × 100 vh`, so each state gets a full viewport of scrub like
  the dive (crap/dry/mutate 3600 px, tests 2700 px at 900 px; /math is 14 902 px).
- Dry 3 (QA D5): the verdict word `DUPLICATE` is the molten term and lands (drops in over the work phase);
  `0.80` is chalk. `.shots/r11-math-dry3.png`.
- Prompt (QA D7): the bridge line is a flex row where the space text node collapsed; the prompt keeps a 1 ch margin,
  so it reads `$ coderefinery …` while typing. `.shots/r11-dive-bridge.png`.
- Anchor landings (QA D8): `wireAnchors()` (mount.ts, all routes) — a hash on load, a hash change or an in-page
  anchor click marks the target card and every runway card above it as mounted (m = 1) and scrolls the target into
  place. /agents → `/#hook`: scrollY 13 500, card top 0, transcript 13/13 lines, status bar 1. `.shots/r11-hook-landing.png`.
- Nav (QA D9): `docs` points at `/docs` — a placeholder route that does not exist yet; `github` keeps the GitHub org.
- Bottom-of-page leak (user report): the deck wrote `top: min(0, 100vh − height)` on every card, including the last
  one, which is `position: relative` — so the setup card was shifted up 450 px and the stuck dive showed through
  under its status bar. The last card has no `top` now. Also: covered is derived from the furthest any later card
  has come in (not only the next one), and a fully covered card carries `data-hidden`, which forces
  `visibility: hidden` and `pointer-events: none` on every descendant (active pane sections and hints set their own
  `visibility: visible`). The pinned dive keeps its deck geometry (it sticks once its pin has run out so the next
  card can cover it) but renders nothing once covered. Checked at 1440 × 900 and 1280 × 720 on `/`, `/agents`,
  `/math`, last 300 px of scroll in 50 px steps, `elementFromPoint` on a grid: no element of an earlier card
  anywhere; at the bottom the last card is docked and complete with the footer directly under it (card top −52 /
  −60 px, the footer's height). `.shots/r11-bottom-<route>-<size>-<6…0>.png`.

## Round 12 (QA round 6)

- D1 §08 setup: three blocks in reading order — the hook file (left top), the transcript (right, both rows), the
  steps list (left bottom, now its own box) — at m 0.52 / 0.635 / 0.75. The card is unpinned, so the transcript is
  the session block's text: its lines (and typed tool-call lines, complete) reveal with the block's top-down clip at
  its text step, not as a scrubbed replay. Measured: hook in at 0.60, transcript lines 11/11 at 0.70, steps at 0.80,
  status bar 0.88–0.90. `.shots/r12-setup-m0.70.png`.
- D2: `.setup .tool-b` was hidden by the dive's `.tool-b { opacity: 0 }` (global class); the dive rules are scoped to
  `.tl-tool .tool-b`. Tool bodies at opacity 1 at m 1, at the page bottom and after `/#hook`. `.shots/r12-setup-m1.png`,
  `r12-hook-landing.png`.
- D3: support text comes ≥ 0.02 after the container in every block: step 3 starts at block + 0.10 (the container's
  last surface is in by + 0.08; blocks now take 0.13 and start 0.52–0.75); P4's `preview · ships…` is marked
  `data-support` (it is ember, not ash); measured box in at 0.76, support from 0.79. /math first card: box
  0.62–0.68, support (`rule` label, visual caption, marker labels, code bar, line numbers) 0.70–0.74; dive K1 box
  0.67–0.71, support 0.73–0.76.
- D4: main → support ≥ 16 px: P1 / P4 quote → source, P4 tile label → source (`.tstat-s` margin), A3 rule name →
  evidence (now side by side, a 16 px column gap, so seven rows do not stack the floor into overflow), §08 agent label →
  message and tool header → body, dive hint kicker → text (title → text 12). The floor variables carry 0.5 px of
  headroom (`--gap-support` 16.5, `--gap-title` 12.5) so measured text-box gaps are at or above the floor
  (16.0–16.5 at 1440, ≥ 12.8 at 1280).
- D5: dive hint boxes have 24 px (`--pad-tile`) on every side; outputs tables get an 18 px column gap and 7 px of
  vertical cell padding per side (14 px per row); K3's header words are 18 px apart. `.shots/r12-dive-k3-hint.png`.
- D6: `--u` is 1 at ≥ 1440 × 900 (the 920 px headroom applies only below the reference), so the floors are exact there.
- D7: pane bars have 14 px vertical padding; the §08 hook bar reads `.claude/settings.json · Cursor: .cursor/hooks.json`
  on one line (the `step 2 · add to your project` label is dropped from the bar; the steps list says it).
- D9: the last card on every route is `100 vh − footer` tall (`--footer-h`, set by the deck), so at max scroll card
  and footer fill the viewport: card 0–847, kicker on screen, footer 847–900 on `/`, `/agents`, `/math`.
- D10, wrapped: /math rules `CC 9`, `50 %`, `CRAP ≥ 10 → exit 2`, `CC 10`, `score ≥ 0.8`, `survived > 0 → exit 3`,
  `kills 0`, `unique 0`; the crap visual captions `CC = 1 + 8 = 9`, `cov = 0.20 · (1 − 0.20)³ = 0.512`; M5
  `$ npx skills add coderefinery`; the P3 / A1 transcript lines and the P3 table quote `price()`; S9 facts `7.2 MB` and
  the three install commands; /agents A4 `7.2 MB` and the install line. Left: `kills 0` / `kills 5` inside the A2
  verdict table's terminal-output column (they are tool output, already mono, not prose) and `(preview)` in the K4 hint
  (a word).
- Fit after the round (natural height, no flex shrink): every card fits at 1440 × 900 and 1280 × 720 on all routes.
  §08 needed cuts in the spec order: the passing hook run in its transcript is one line instead of a box with two
  body lines (content kept, `killed 12 survived 0 · exit 0`), its section rhythm (gaps, top padding) tightened, and at
  ≤ 800 px height a second type step (× 0.82) — the steps list and the transcript are already the minimum story.
  P4's section gap is 18 × u² (was 20). All pinned states end above their status bar (dive 805 / 849, 658 / 676).

## Round 13 (QA round 7)

- D11 /math first-state support: the `rule` label was never reached by the support rule (`.ex-rule > span:first-child`
  missed it: the box's first child is its injected SVG seam), and the rule seam drew on the 220 ms clock instead of
  the scrub. Now the label is selected by class and fades with `--ent-sup` (opacity, m 0.72–0.76), the seam is
  scrubbed with `--ent-box`. Measured live on crap, dry, mutate and tests at 1440 × 900 and 1280 × 720: rule box fully
  drawn at e 0.83, label (and the crap visual caption) first visible at e 0.86. `.shots/r13-math-crap-e0.80.png`.
- D13 /agents: every A1 loop step and every A4 cell is its own block (spread over 0.52–0.75 with the panes / CTA);
  largest no-change gap A1 0.01, A4 0.04–0.05 of m. `.shots/r13-agents-a1-m0.75.png`.
- D14: P4's pattern list renders with `prose()`; `0.0` is inline code.
- §08 settings file: one block per line with a hanging indent, a wrapped line continues at its string's column
  (`.shots/r13-hook-json.png`); the file column follows the page scale (`500px × --u0`) rather than the card's type
  steps, so the file wraps less and the card still fits (838 / 847 at 1440, 652 / 660 at 1280).

## Round 14: phone and touch (§17)

Modes (`src/motion/mode.ts`, mirrored on `<html data-mode data-touch>`): **desktop ≥ 1024** (deck, runways, pins —
unchanged), **tablet 768–1023** (the stacked, natural-height layout this site already had below 1024, unchanged),
**phone ≤ 767** (new, §17.1–17.2). `pointer: coarse` turns on the touch rules at any width. CSS: `src/styles/phone.css`,
loaded last on every route; scripts branch on `layoutMode()`.

Layout (phone): cards natural height, `min-height: 100svh`, a 1 px seam between cards, no deck / runway / scale /
dim; the card's arrival is a brightness ramp and a molten top seam over e 0–0.30 (`data-arriving`, `--arrive`).
Revealed blocks reserve their height (a clip-path reveal instead of max-height) so a natural-height card never grows
under the reader. One column everywhere; stat tiles (hero, P4, S9, M5) two per row; P3's comparison table stacks
cell under cell; wide content scrolls inside its own box with a right-edge fade (settings file, slab output, code
panes, A2/A3 tables, outputs tables, transcripts); grid/flex children may shrink (`min-width: 0`) so nothing pushes
the document wider. Floors at 80 % (card/pane padding 20, tiles 16, cells 12 × 14, sibling gap 16); headlines
≥ 28 px at 1.25, hero 34 px, body 15 px, mono 12.5–13 px; tap targets ≥ 44 px (menu, nav sheet links 48, CTA
buttons 48, install box 48, status-bar/source links get a 44 px hit area). Topbar: wordmark + `menu` button; the nav
opens as a full-width sheet under the bar, closes on choose, Escape or a tap outside (`wireTopbar`). Hero: headline,
sub-heading, install box full width, the numbers 2 × 2, then the foot line; `min-height: 100svh` (also below 1024,
so a landscape phone at 844 × 390 is no longer clipped).

Motion (phone): `mountSection` runs `phone()` instead of the runway: e = 0 with the card top at the viewport
bottom, 1 at 40 %; §17.2's slots are mapped onto the existing m table (kicker 0.30–0.33, headline 0.30–0.48, blocks
0.45–0.92 text → box → support, status 0.92–1.00). Cards taller than the viewport mount each block on its own
entrance (block top at 85 %). Once mounted, a card stays mounted. Dive: four stacked state cards (session above
outputs, hint under the title), each with its final content; on its entrance (top at 85 %) the command types, the
lines land and the exit lines flash once (`DivePane.play()`, CSS flash band). The depth gauge is a 3 px bar at the top
of the dive, sticky across the four states and filling as they scroll by. Math: each state is its own card (formula
wraps, ≥ 24 px; the card prose and rule; then the code pane with that state's annotations), no rail; the molten value
rolls once on entrance (`Stage.play()`). Stream: ≤ 12 events / 16 sparks on phones; touch never hovers; a tap on an
event shows its detail line for 2 s. A3's hover pairing ignores touch. The slag field draws half its dots on phones
(pitch × √2, `u_pitch`). Budget (§17.3): below 1024, if the median frame over 90 scrolling frames exceeds 18 ms, every
shader drops to its CSS fallback for the session (logged in `__shaderLog`).

Verified in headless Chromium with touch emulation (CDP `Emulation.setTouchEmulationEnabled`, `.tools/touch.mjs`:
`pointer: coarse`, `hover: none`) at 390 × 844, 360 × 800, 768 × 1024 and 844 × 390: `scrollWidth == innerWidth` at
every 0.7 vh on `/`, `/agents`, `/math`; no element outside the viewport except inside a horizontal scroller; menu
sheet 390 px wide with 48 px links, closes on choose; `/agents` → `/#hook` lands with the card complete and every card
above mounted. Desktop re-verified: every card fits at 1440 × 900 and 1280 × 720 on all routes, no bottom leak.
Screenshots: `.shots/r14-<route>-<viewport>-<section>.png` (hero, p1, p3, p4, dive-k1, dive-k3, s06, setup, a1, a3,
m1, m3 at 390 × 844; home hero and dive at 768 × 1024; hero at 844 × 390; menu and hook landing at 390 × 844).

Not met / deviations: tablet (768–1023) keeps the existing stacked layout rather than the desktop deck (§17 says
desktop rules; the brief said keep tablet as it is, and the desktop cards do not fit 100 vh at 768 px wide); the depth
bar sits at the top of the dive section (sticky), not inside K1 under its kicker, so it can keep filling through K4;
the frame budget is enforced by the guard but not measured on a real mid-range phone (headless only); the hero stream
pills stay at 10–11 px (decorative, `aria-hidden`).

## Round 14b

- Phone hero stream: the stream runs only in an empty band between the install box and the numbers (132 px of
  space above the numbers on phones; the stream is clipped to the band, fades in over one pill under the install box
  and ends at the numbers' top; with a band under 120 px the stream is dropped). Measured at 390 × 844 and 360 × 800:
  band 152 px, 0 events outside it, the foot line fully inside the hero (the hero grows past 100svh).
  `.shots/r14b-home-390-hero.png`.
- Phone /math code panes: the pane scrolls sideways; rows are `max-content` wide (at least the pane), so each tag
  follows its line's text on the same row; the line numbers stay at the left edge while scrolling and keep their
  row's tint. Checked on all 11 states at 390 and 360: 0 tags over code, every tinted row at least the pane's width.
  The dive's K cards have no inline tags (their panes wrap or scroll). The pinned cards' status dots stay on the line
  they mark when the bar stacks. `.shots/r14b-math-390-m1.png`, `r14b-math-390-m1-scrolled.png`.

## Round 15 (QA round 8)

- D1 menu: on phones each card's top bar sits on its own layer above the card content (`z-index: 40`), so the open
  sheet covers the hero. Real CDP touch taps (touchStart/touchEnd, `.tools/tap.mjs`) open the menu and navigate on
  `/`, `/agents` and `/math`. `.shots/r15-menu-390.png`.
- D2/D3 /math code panes: the rows are one grid column `minmax(max-content, 1fr)`, so every row is as wide as the
  widest line (at least the pane) and its tint reaches the scroll end; line numbers are sticky. The clipped edge
  fades over 20 px. Checked on all 11 states at 390 and 360 (scrolled to the end: 0 short rows, number at the left
  edge). `.shots/r15-math-s9-scrolled-390.png`.
- D4 type floors: `applyTypeFloors` (`src/motion/mode.ts`) raises any phone text still under its floor inline (mono
  12.5 px, body 15 px; the stream, svg and screen-reader text are exempt), undone on leaving phone mode. QA's sweep
  (`designs/reviews/qa-shots/r14-qa-floors.js`) under floor, before → after at 390: `/` mono 96 → 0, body 5 → 0;
  `/agents` 30 → 0, 11 → 0; `/math` 58 → 0, 0 → 0. Same 0/0 at 360.
- D5 stacked entrances (dive K1–K4 and /math states on phones): slide ≤ 0.30, kicker, headline typing, card body,
  box, support, status last, on `phoneMount(e)`; the command/outputs (dive) and formula row (/math) play once when
  their block starts. `.shots/r15-dive-k1-390-e0.60.png` (headline done, box drawing, support not yet).
- D6 below desktop (768 × 1024, 844 × 390): the hero is laid out as on a phone (headline, sub, install, stream band,
  numbers 2 × 2, foot); the stream is clipped to its band with at most 12 events on touch (6–8 measured, 0 outside
  the band); the tablet dive has the sticky depth bar. `.shots/r15-home-768-hero.png`.
- D7 padding: install box 19 px block padding (text 20 from the edge), gate cells, hints, loop steps and A cells
  20, `ex-rule`/tool/agent boxes 20, pane bars and code lines 14 vertical; a full-row stat tile gets the 20 floor.
- D8 tap targets on touch: source links, status-bar links and cites are `inline-block` with a centred `::after` at
  least 44 × 44; the footer's wrapped source list gets 44 px lines on touch so neighbouring areas do not overlap.
  QA sweep tap failures 0 on all three routes (was 21 / 9 / 9).
- D9: every `:hover` rule is inside `@media (hover: hover)` (0 unguarded rules found by walking the stylesheets). A
  touch tap on an A3 row focuses it without pairing; keyboard focus still pairs.
- D10: the phone dive pushes each flash to `window.__flashLog` (`key: "phone:…"`, `shader: "css-fallback"`; 15
  entries over one pass at 390). On phones the furnace is a CSS tint, not the shader: this is the accepted budget
  fallback.
- D11: the dive session pane sizes to its content on phones; an odd last stat tile (P4's third, the /math numbers'
  fifth) takes the whole row. The 19 px height shift on `/` came from typed lines growing while the dive and setup
  states played; below desktop the typer lays out the untyped rest invisibly (`.typed-rest`), and the stacked /math
  cards reveal by opacity and lift without collapsing their height. Document height while scrolling at 390: `/`
  18985–18986 px, `/agents` 7305, `/math` 13302 (constant).
- Seams: the tile grids' `nth-child` rules count tiles only (`:nth-child(2n of .bn)` etc.), because each box's first
  child is its seam svg; this also fixes the desktop hero numbers' inner seams.
- Desktop regressions at 1440 × 900 and 1280 × 720: every card fits, runways 150 vh, footer bottom at the viewport
  bottom, no horizontal overflow, no type floors or typed-rest applied.

## Round 16 (QA round 9)

- Type floors on typed lines: `applyTypeFloors` treats every `.typed` span as text even while it is still empty, and
  re-runs (debounced) when elements are added under `main` (the stream excluded), so the P2 gate commands
  (11.05 px) and the P5 diff lines (9.6 px) are at 12.5 px. QA's sweep after scrolling each route through, at 390
  and 360: mono 0, body 0 on `/`, `/agents`, `/math`.
- `/agents` `.mtable` pane bar: 14 px vertical like the other pane bars.
- Phone state status bars (dive K cards, /math states): dot, verdict and exit code on one row; only the verdict
  wraps; only the card's own verdict is laid out (`.cur`, set where the status lines cross-fade), so no gap is
  reserved for a longer verdict. Bar height 28 px (one line) or 46 px (wrapped). `.shots/r16-math-s9-end-390.png`.
- Sideways scrollers on phones drop their right-edge fade at the scroll end or when nothing overflows
  (`wireScrollFades`, `data-scroll-end`).
- Dive state cards on phones lay out only their own title and drop the head's 96 px minimum, so K2 and K4 have no
  blank space under the headline (title → hint 24 px on all four). `.shots/r16-dive-k2-390.png`.
- `/agents` verdict table on phones: the gate output (`.vsees`) keeps its columns and scrolls sideways in its cell
  with the edge fade.
- Desktop 1440 × 900: every card fits, runways 150 vh, footer bottom at the viewport bottom, no floors applied.

## Round 17: the deck on phones and tablets (§17.4)

One deck module, two geometries (`src/motion/deck.ts`): **desktop** (≥ 1024, unchanged) and **compact** (< 1024,
phones and tablets). `layoutMode()` picks the geometry on every layout pass; switching re-collects the cards and
strips everything the other geometry wrote (classes, inline tops, transforms).

Geometry (compact). `svh` = a fixed probe of `height: 100svh` (so the address bar does not move the geometry).
- Card (the face: `.hold-sticky`, the hero itself, a dive / explainer state stage): `min-height: 100svh`,
  `position: sticky; top = min(0, svh − cardHeight)` — a card no taller than the viewport sticks at 0, a taller one
  is read through and pins by its bottom edge.
- Wrapper (`.hold`, or `.deck-hold` around each stacked state): height = cardHeight + 50svh. The runway is an
  in-flow `::after { height: 50svh }`, not padding: a sticky box can only travel inside its container's content
  box, so with padding the card never pinned (caught by the hold-end probe). The wrapper sticks at
  `top = min(0, svh − wrapperHeight)` so the held card stays put while the next one slides over.
- Groups: the stacked dive and each stacked explainer (`.deck-group`) stick at `min(0, svh − groupHeight)` in
  `main`, so their last state can be covered like any card; their states are deck cards inside them (the dive's
  sticky depth bar stays the group's first child, above the states).
- The hero (card 0) has no runway and never arrives (unchanged). The last card has no runway, is not sticky, and
  the footer scrolls in under it.
- Heights refresh on resize, on `document.fonts.ready` and through a ResizeObserver on every card, wrapper and group
  (card heights settle ≈ 1.5 s after load when the type floors run; a pinned section rebuilding also lands here).

Motion (same code path as desktop, per-geometry constants `GEO`):
- Covered card, over the incoming top's travel from the viewport bottom to the top (n = (vh − top) / vh): scale
  1 − 0.04 n, translateY −16 n px (desktop 24), dim overlay 0.30 n; origin `50% max(0, cardHeight − svh)px`, i.e. the
  top-centre of what is on screen (a tall covered card is pinned by its bottom; scaling about its offscreen top would
  shift the visible part by 4 % of the card's height). n = 1 (or the later top within 0.5 px of 0): `visibility:
  hidden`, `inert`, `data-hidden` (every descendant forced hidden, no pointer events). Hero field pauses on
  `data-covered`.
- Incoming card, k = (vh − top) / (0.18 vh) — e 0–0.30 of the §17.2 entrance (e = (vh − top) / 0.6 vh): scale
  0.97 → 1 about its top-centre, overlay 0.45 → 0 (brightness 0.55 → 1), `--seam-glow` 1 → 0 on its 1 px top seam.
- `will-change: transform` on the face only while covering or arriving; cleared when docked, waiting or hidden.
  Transforms only on the face; the dim is the desktop's black overlay, sized to the whole card in compact.
- Reduced motion: sticky stacking, hidden / inert when covered, no scale, shift, dim or glow.
- Content: phones and tablets both mount on the §17.2 entrance now (`mountSection` → `phone()`, the stacked dive and
  explainer states via `setPhoneEntrance`); the height-reserving clip-path reveal moved from ≤ 767 to ≤ 1023 so a
  tablet card never grows while pinned. Tall cards still mount each block on its own entrance.
- Discreet field: handed to the active card's face as on desktop; in compact it is one `100svh` tall and translated
  to the visible part of a tall card (`placeField`); a covered card's frozen frame sits at its bottom edge.
- Anchors: `mountThrough` scrolls to `naturalTop(target)` (deck.ts: measured with the deck's sticky offsets and
  transforms switched off, `main.deck-measure`), so `/#hook` and `#why` land right even when the target card is
  stuck or hidden; `hashchange` scrolls too. The phone menu sheet closes when its card starts being covered
  (`deck:cover` event, topbar.ts); open, all five links hit-test to themselves.

Removed (dead after the change): the phone `data-arriving` / `--arrive` brightness ramp (mount.ts, phone.css), the
phone border-top seams between cards (the deck's seams replace them), the slag field's no-deck path (fixed canvas,
`exclude` fade over the dive, `attach(…, inDeck)`), the tablet "complete, no entrance" branch.

Fixed on the way (pre-existing, tablets): below 1024 the §08 setup grid went to one column but its items kept their
two-column `grid-area`s, so the settings file sat in an implicit 47 px column and the card was 4 510 px tall at
768 × 1024; the areas are reset below 1024 (sections.css) and the phone-only `!important` copy is gone. Now 1 146 px.

Verified (headless Chrome, agent-browser session `impl17`, touch emulation `.tools/touch.mjs`; probes in
`.tools/deck17.js`, runner `.tools/deck17-run.sh`, swipe `.tools/swipe.mjs`, CPU throttle `.tools/throttle.mjs`):

| | 390 × 844 | 360 × 800 | 768 × 1024 | 844 × 390 |
|---|---|---|---|---|
| cards `/` · `/agents` · `/math` (taller than the viewport) | 12 · 5 · 13 (all) | 12 · 5 · 13 (all) | 12 (10) · 5 (3) · 13 (1) | 12 · 5 · 13 (all) |
| top at 0 and bottom at the viewport bottom through the whole 50svh hold, every card | yes | yes | yes | yes |
| elementFromPoint 5 × 9 grid at 61 positions: hits on hidden cards · max covered-visible · hidden not inert | 0 · 1 · 0 | 0 · 1 · 0 | 0 · 1 · 0 | 0 · 1 · 0 |
| page bottom (max scroll and 50–300 px above): earlier cards hit / visible | 0 / 0 | 0 / 0 | 0 / 0 ¹ | 0 / 0 |
| `scrollWidth − innerWidth` at 12 positions, 3 routes | 0 | 0 | 0 | 0 |
| worst frame scrolling 30 px/frame, `/` · `/agents` · `/math` | 16.8 · 16.8 · 16.8 ms | 16.8 · 16.8 · 16.8 | 16.8 · 16.8 · 16.9 | 16.8 · 16.8 · 16.8 |

¹ `/agents` at 768: 300 px above max scroll A3 is still partly visible under the arriving A4 (mid-slide, by design);
clean from 200 px up. At 4× CPU throttle (390, a rough mid-range stand-in) `/` drops 4–5 of 737 frames to 33 ms,
each at a card's content mount (P4, S9, teaser, setup at e ≈ 0.5, the type-floor re-scan on added typed lines), none
in a slide; `/agents` and `/math` none. The shader guard is unchanged (all fields `running`).

Deck motion at 390 × 844 (f = incoming top travel, 0 bottom → 1 top):

| f | P1 over hero (1090 px) | P3 over P2 (1966 px, tall) | K2 over K1 (dive state, 1742 px) | incoming |
|---|---|---|---|---|
| 0.09 | s 0.9964, −1.44 px, dim 0.027 | s 0.9964, −1.45, 0.027 | s 0.9964, −1.44, 0.027 | s 0.985, dim 0.225, glow 0.50 |
| 0.18 | 0.9928, −2.88, 0.054 | 0.9928, −2.89, 0.054 | 0.9928, −2.88, 0.054 | s 1.000, 0, 0 |
| 0.50 | 0.9800, −8.00, 0.150 | 0.9800, −8.01, 0.150 | 0.9800, −7.99, 0.150 | done |
| 0.95 | 0.9620, −15.21, 0.285 | same | 0.9620, −15.20, 0.285 | |
| 1.00 | hidden + inert | hidden + inert | hidden + inert | |

Origins: hero `50% 246px`, P2 `50% 1122px`, K1 `50% 898px` (cardHeight − svh). 768 × 1024, P3 over P2 (1312 px):
identical curve. Reduced motion: scale 1, shift 0, dim 0, no glow at every f; every card still reachable.
Sideways scroller in a covered card (/math crap 2 code pane while crap 3 slides over it, face at scale 0.988): a real
touch swipe scrolls it to its end (scrollLeft 142 = scrollWidth − clientWidth), page scroll unchanged, line numbers
stay pinned. `/agents` → tap `/#hook` (390): scrollY 21 104 = natural top, card top 0, blocks open, status 1, all 7
runway cards above mounted; `location.hash = '#why'` from there lands at 1 090 (P1's natural top). Type floors
(`r15-qa-floors.js` with the deck's offsets off): headlines, body, mono and taps 0 at 390 and 360 on all routes;
padding and cell lists identical to QA's round-15 baseline (`r15-qa-floors-390-*.json`, same elements, same counts).

Desktop regression (no touch): `r15-qa-desk.js` / `r15-qa-desk2.js` output identical to `r15-qa-desktop-1440.txt` /
`-1280.txt` (every card fits: sections 873 / 822 and 698 / 640, content bottom ≤ section bottom at m 0.45 / 0.7 / 0.95;
pinned sweep only the known transient `ex-row` bottoms; bottoms 847 / 900 and 660 / 720; docX 0). Runway timing P1
and A1: kicker 0.42 → 0.44, title 0.42 → 0.56, status 0.89 → 0.90 (as round 15). Deck values unchanged (covered
−24 px, incoming slide over e 0–0.60). Home 14 903 px at 1440 × 900. Console: only `[vite]` debug lines. Switching
1440 → 390 → 1440 re-collects cleanly (9 desktop cards ↔ 12 compact cards, 1 group, 4 holds).

Not met / deviations:
- Brightness is the desktop's black overlay (opacity on one child of the card), not a CSS `filter` on the card; the
  overlay keeps the 1 px seam above it at full colour and is what the desktop deck already does.
- The covered transform's origin is the top-centre of the visible part of a tall card (see above), not its
  offscreen top edge.
- The incoming slide is short by spec on phones (top from 100 % to 82 % of the viewport, 152 px at 844).
- The 16 ms budget is measured headless (60 Hz rAF, with and without 4× CPU throttle), not on a real 2022 phone.
- §08's typed tool lines still grow the last card by 67 px when they type at 390 (they reserve one line and wrap to
  three); harmless for the deck (the last card is not sticky), left as it was.

Screenshots: `.shots/r17-home-390-p1-slide.png` (P1 at f 0.10 over the hero), `r17-home-390-tall-pinned.png` (P2,
1966 px, pinned by its bottom with P3 arriving at f 0.12), `r17-home-390-dive-k2-slide.png` (K2 at f 0.10 over K1),
`r17-home-768-p3-slide.png` (P2 pinned, P3 at f 0.10, tablet); also `r17-hook-landing-390.png`, `r17-menu-390.png`,
`r17-math-390-covered-scroller.png`.

## Round 18 (QA round 10: D-1 to D-4)

- D-1 §08 grows on phones: the setup card opted out of the below-desktop typed-line reservation
  (`.setup .typed-rest { display: none }`, base.css); removed, so its typed tool lines lay out their untyped rest
  invisibly like every other typed line and hold their wrapped height before typing. 390: §08 1654 px from load to
  the end of a full scroll pass (was 1587 → 1654), document 23 444 px constant; 360: 1739 constant, document
  24 347 constant; both typed lines 49 px before and after typing.
- D-2 status last on tall cards (mount.ts `phone()`): when a card mounts its blocks on their own entrances, its
  status slot is driven by the last block, `min(card q, 0.88 + 0.02 × range(lastBlockQ, last.start + 0.13,
  last.end))`, so the bar fades only after the last block's support step. 390: P1 (1484 px) last block opens at
  e 1.94, its support is in at e 2.13, status 2.13 → 2.17 (was 1.0 at e 1.0); P2 (1966 px) last block support in at
  e 3.16, status 3.16 → 3.20 (was 1.0 at e 1.6). Cards that fit keep the §17.2 slot (0.92–1.00).
- D-3 hero hold (phones and tablets; desktop unchanged): the hero is its own deck face (its field and event stream
  live in its wrapper), so its 50svh hold is `margin-bottom: 50svh` on the hero card in the compact deck: it stays
  pinned by its bottom edge while the margin scrolls by below the viewport, then P1 arrives. It still never arrives
  itself. Hold 422 px at 390 × 844, 400 at 360 × 800, 512 at 768 × 1024; through the hold the foot line stays at
  688–739 / 644–695 / 855–889 px, hit-testable, hero not covered. `.shots/r18-home-390-hero-hold.png`.
- D-4 tablet nav taps (phone.css, `pointer: coarse`): each topbar link gets a centred `::after` of
  `max(100%, 44px)` × `max(100%, 44px)`; links already 44 px wide keep their own box, so no area reaches into a
  neighbour across the ~11.5 px gap (a `+ 8px` variant, as on the status links, lost `why` to `for agents` by paint
  order: 41 px). 768 × 1024 and 844 × 390, all routes: `why` 23 → hit 44 × 44, `docs` 31 → 44 × 44, `for agents`
  78, `the math` 64, `github` 48. Look unchanged (`.shots/r18-topbar-768.png`).
- Re-verified (touch at 390 × 844, 360 × 800, 768 × 1024, `.tools/deck17-run.sh`): every card reachable top to bottom
  with its hold on all routes (`reach` now true for §08 too), 0 hits on hidden cards, at most 1 covered card visible,
  bottoms clean, `scrollWidth − innerWidth` 0 at 12 positions, worst frame 16.8–16.9 ms. Type and tap sweep at 390:
  headlines, body, mono, taps 0 on all routes; padding / cell lists identical to the round-15 baseline. Desktop
  (no touch) 1440 × 900 and 1280 × 720: `r15-qa-desk.js` / `-desk2.js` identical to round 17 (every card fits; bottoms
  847 / 900 and 660 / 720; home 14 903 px at 1440). Types clean; build checked into a scratch outDir (`dist/` left as
  deployed).

## Spec coverage

Done:
- §0/§0b principles and tiers; §1/§13.2 geometry (400 vh pin, phases, bridge typing, Enter = first 2 %).
- §16.2 code annotations on /math (tint + note per line, in the card's order); the dive's line model is retired (round 9).
- §5/§5a transcripts: commands typed in the bridge, verdict + exit in settle, exit underline draw, K3 `█▒░` bar, K4's `dry tests/…` typed in place.
- §9 S9 numbers (30 %/40 % rule, `120×` starts 10 % later, fact lines ash → chalk, `exit 0` underline at 60 %).
- §12 P0–P5: hero timed entrance + on-load session + stats roll (molten last) + slag-field shader; P1 column reveal + stat rolls + source links; P2 gate cells opening along their left seam 60 ms apart with their commands typing as they open, lists draw seams and appear at once; P3 three-column table draws its outer seam with rows 40 ms apart, the two transcript panes appear at once and only the molten CRAP / slag `exit 2` lines land 200 ms later, nothing types; P4 stat rolls, quote + patterns at once, the mutant-to-test slab at once (rows tier 2), CTA stroke + one arrow nudge, pin starts as the slab reaches the top; P5 scrubbed transcript replay with typed tool call, ingot commit hash, status bar last.
- §13.1 look: furnace ground cross-fade over K1's first 15 %, heat seams, molten hint with the formula kicker, depth gauge (continuous fill, marks pop, active mark larger), 4 px progress bar, kicker odometer, syntax colours from `syn-*` tokens only.
- §13.3 (round 9): session pane + outputs pane per state, scrubbed in the order of the table; verdict flash on every exit line in both panes.
- §14.1 seams draw first on every `.box` (SVG rect, 220 ms, nested +80 ms); §14.4 three shaders with fallbacks; §14.5 nothing else moves.
- Accessibility: landmark elements, real headings (dive titles are h2s, snapshots too), `role="img"` + `aria-label` on odometers and typers, focus-visible styles, links underlined in status bars, reduced motion.

Approximated:
- Dive heading per-word stagger (§3): the title crosses as one line (8 px fall/rise) rather than per word.
- Hint box anchoring (§8): the K hint is fixed in the head (the K designs place it there); no leader line.
- K2 → K3 "`_settle` is the same line objects": `_settle` in K3 reuses fresh ids (the K2 pane never shows `_settle` as code lines, only in the patch pane), so it fades in rather than slides from the patch.
- "Before" terminal between states: the bridge line types at the bottom of the outgoing transcript; on Enter the outgoing block fades and the new state's command line appears already typed at the top (not literally the same node).
- Tool-call status chips (`running` → `done`) are not rendered; the K designs do not show them. The modified dot carries that state instead.
- Mobile "pin height 7 × 100 vh" (§10): under 1024 the walkthrough is fully unpinned per §13.4, which supersedes §10.
- Every tween is a CSS property driven by the one JS progress observer (all scrubbed, so still reversible); native `animation-timeline` is not needed after the P2/P3 rewrite removed the formula strip. GSAP was not needed.
- The `↓ types as you scroll` label and the `exit 0` underline run their one-time draw via a 240 ms CSS transition on landing (the landing itself is scrubbed).

- M3.1 caption (§16.2 "the caption cycles through the operator list"): the caption keeps the verbatim full list and the cycling is shown as emphasis (the current operator turns chalk), so the copy stays intact.
- M1.3 "the marker slides from the slag zone into the ingot zone": both design numbers are shown (`9` after, `50.5` before); the `after the split` figure is the one that rolls 50.5 → 9 and the marker on the bar slides with it.
- M4.2 "shared cells dim to seam": the design keeps shared kills ingot, so they dim by opacity rather than to the seam colour.
- A3 hover pairing (§15): the spec does not say which rule each mutant maps to; the mapping in `MUTANTS` (`src/agents/sections.ts`) is editorial (boundary/sign/equality mutants → "write the failing test first", `0 → 1` → "returning nothing", `and → or` → "split before you test", `true → false` → "behaviour, not implementation", deletions → "delete what no honest test can kill").

Copy decisions:
- P4 headline: the regenerated reference and PNG read "Nobody verifies whether the new tests can fail."; that is shipped (an earlier export had a typo).
- S9 kicker: `§06 · after the intro` (HTML export), consistent with P5 = §05 and M = §07; the PNG still shows §04.
- Sources behind P1/P3 numbers link to the citations in `designs/reports/Developer pains with AI code quality.md` (Faros 2025/2026, DORA 2025, Sonar, Stack Overflow 2025, Blacksmith via TechTimes, Hacker News thread, arXiv 2606.18168, claude-code #104504, Kent Beck).
- Nav: `docs` points at `/docs` (placeholder, no page yet); `github` at a placeholder GitHub org; `why` and `the math` are in-page anchors.

## Release · Cloudflare Pages (2026-11-05)

- Project `coderefinery` on Cloudflare Pages (classic Pages, created with `--force`
  because wrangler's Workers delegation requires Vite ≥ 6), production branch `main`,
  direct upload of `dist/`. Preview host: https://coderefinery-21a.pages.dev
- `npm run deploy` builds and uploads. Wrangler runs through the repo-local npm cache
  (`.local/npm-cache`), nothing installed outside the repo.
- `public/_headers`: immutable cache on `/assets/*`, nosniff, referrer policy.
  `public/_redirects`: `/docs → /` (302) until docs exist.
- Custom domains `coderefinery.dev` and `www.coderefinery.dev` are attached to the
  project and wait for their CNAME records (`→ coderefinery-21a.pages.dev`, proxied);
  the wrangler OAuth token has no DNS write scope, so those records are added in the
  Cloudflare dashboard (Pages → coderefinery → Custom domains → Activate).
