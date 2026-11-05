# coderefinery.dev · scroll-driven intro · motion spec

Companion to `coderefinery-identity.pen`, column "S0…S9" (the storyboard keyframes).
This document describes what moves between those keyframes and how it is driven.

## 0. Principles

1. **Scroll is the timeline.** Nothing in the intro plays on a timer. Every value
   (opacity, position, counter, line state) is a function of scroll progress, so the
   user can scrub backwards and everything reverses. The one exception is the typing
   cursor blink, which runs on a clock.
2. **The agent does the work.** Fixes are never "just replaced". The agent pane
   emits a tool call (`sed`, `apply_patch`, `write`), the file bar shows the file as
   modified, and the code pane changes line by line as the command would change it.
   The viewer watches a file being edited, not a slide being swapped.
3. **Three regions, one clock.** Code pane, agent pane, hint box share the same
   progress value per stop. Within a stop they are staggered, never independent.
4. **Numbers never cut.** A number on screen either counts (scrubbed by scroll) or
   rolls (odometer). CRAP 50.5 → 3 is the most important moment in the sequence and is
   scrubbed over the longest range.
5. **Molten moves last.** Chalk text and seams settle first; the molten element in a
   stop (the tinted branch, the survived tag, the counter) lands at the end of the
   stop's range so the eye arrives on it.

## 0b. Text motion: typed titles, revealed text, faded labels

This is a text-heavy page. Each kind of text has exactly one way of arriving,
and the reader never waits for a sentence to finish.

| kind | what | how it arrives |
|---|---|---|
| **title** | section kickers (`§01 · crap`) and headlines | **typed**, like a command at a prompt: characters appear left to right, scrubbed by scroll over the title's slot, with a block cursor at the end that blinks on a clock and disappears when the title is complete. Kicker first, then headline. Word wrapping is pre-computed so lines never reflow while typing |
| **inner text** | body copy, cards, agent messages, hint prose, table rows, list items, the patch hunks | **revealed**: the block expands from zero height to its natural height with a top-down clip, 8 px rise, over its slot. The text inside is already complete; the clip opens in one motion of at most 12 % of the section's mount range, so a block becomes readable in one step. Never per word, never typed |
| **other text** | labels, captions, sources, status bars, file bars, nav, legends | **faded**: opacity 0 → 1 over its slot, no movement |
| **numbers and verdicts** | stats, scores, counters, exit codes, SURVIVED/killed lines | keep their own behaviour (roll, land) inside their slot, after the block that holds them has revealed |
| **commands** | `$ …` lines and tool-call headers in terminals | typed, as in § 5a |

Rules that follow:

- **Title line height is 1.25** for headlines (display sizes 44 px and up)
  and for card titles (20–34 px); kickers stay single-line. Was 0.98–1.05;
  the tighter setting crowded two-line headlines.
- **Variables and values in prose are code.** Any formula variable or computed
  value mentioned in running text (`CC`, `cov`, `0.512`, `exit 2`, `max(total,
  0)`) is set in the mono font with the inline-code style: chalk on a slab
  tint, 0.92 em. Never italic, never the body face.
- A title types, inner text expands, everything else fades. No fourth way.
- Nothing a reader needs to finish a sentence is behind scroll: a revealed
  block is complete the instant its clip is open.
- At most three motions run at the same time; slots are sequenced, not stacked.
- A number rolls only when the change is the point (`0 → 50.5`, `survived
  2 → 0`); a static figure in a tile arrives with its block.
- Earlier sections that say a block "appears at once" or "tier 2" now mean:
  revealed by expanding, per this table, inside the section's mount stagger
  (§ 0c). "Tier 1" means typed or rolled as before.
- Reduced motion: titles and commands appear complete, blocks are open, nothing
  fades; sections do not hold (§ 0c).

## 0c. Section rhythm: fit the viewport, mount on scroll, then release

- **Every unpinned section is exactly 100 vh tall** and its content fits inside
  it at 1440 × 900 and at 1280 × 720. Type and spacing scale with `clamp()`; if
  a section's content cannot fit, it is split into two sections, never
  overflowed. Each section clips its own overflow. No element crosses a
  section boundary; the only object that travels between sections is the
  session pane in the P5 → K1 hand-off (§ 14.2), and it does so inside the pin.
- **Mount on entrance, no hold.** A section mounts *while it scrolls into
  view*, not after it docks. Define entrance progress `e` for a section as the
  position of its top edge: `e = 0` when the top is at the bottom of the
  viewport, `e = 1` when it reaches the viewport top. The mount range is
  **`e ∈ [0.30, 0.90]`**: the sequence starts when the previous section has
  been scrolled 30 % out (the incoming top sits at 70 vh) and is complete when
  the incoming section is 90 % in (its top at 10 vh). The last 10 % of the
  entrance is quiet, so the section docks fully assembled and reading starts
  at once. No sticky hold; every unpinned section is a plain 100 vh block.
  **The slide itself is the first animation.** From `e = 0` to `e = 0.60`
  (card top from the viewport bottom up to 40 % of the viewport height) the
  incoming card shows no content at all, only its own arrival: it scales from
  0.97 to 1.00 about its top-centre, its brightness rises from 0.55 to 1.0,
  and a 1 px seam along its top edge glows molten at 60 % and cools to the
  normal seam colour by `e = 0.60`. Its slag field is visible from the start,
  so the card reads as a lit surface sliding in, not a blank. The covered card
  below does its deck motion at the same time (scale, shift, dim).
  **Content mounts only after the top reaches 40 % of the viewport, and it
  takes its time.** The entrance alone (60 vh of travel from the 40 % mark to
  the dock) is too short for a text-heavy card to assemble legibly, so every
  unpinned section gets a **scroll runway**: its wrapper is 150 vh tall and the
  100 vh card is sticky inside it. The card docks after 100 vh of scroll and
  then holds for 50 vh before the next card's top appears at the viewport
  bottom. Define mount progress `m ∈ [0, 1]` over the whole 150 vh (card top at
  the viewport bottom → next card about to arrive); `m = 0.40` is the 40 %
  mark, `m = 0.667` is the dock. Slots, all scrubbed by scroll:
  slide only 0–0.40 · kicker fades 0.40–0.43 · headline types 0.40–0.56 ·
  then the **blocks** (each card, column, table, pane, list, figure) mount in
  reading order, left to right, top to bottom, one starting every 0.04 and
  overlapping, from 0.52 to 0.88 · captions and sources fade 0.84–0.88 ·
  status bar fades 0.88–0.90. Everything is complete at `m = 0.90`; the
  last 10 % of the runway shows the finished card. The hero keeps its timed
  mount; the pinned sections (dive, math explainers) keep their own pins and
  their first state assembles over the entrance as before, using the same
  text-first order inside each block.
  **Inside a block, text comes before its box.** A block mounts in three
  steps, each its own scroll slot, so the layout forms around the words
  rather than the words filling a waiting frame: (1) the block's own title and
  its main text reveal (title typed if it is a heading, main text revealed as
  in § 0b); (2) **a scroll delay later (≥ 0.02 of `m`) the container arrives**:
  the surface tint fades in and the seams draw around the text (§ 14.1), table
  row rules and column seams included, cell by cell for tables; (3) another
  0.02 later the support text fades: labels, meta lines, footers, sources,
  legends, muted captions, badge chips, the exit-code line of a pane. Numbers
  and verdicts roll after step 3. A table therefore shows its header words and
  first-column words, then its grid, then the muted cells; a stat tile shows
  its figure's label and figure, then its tile, then its unit line. Nested
  blocks follow the same three steps inside the parent's step 2.
  reverses the mount; once a section has completed its mount it stays mounted
  (the reader is re-reading, not re-watching). Because mounting now overlaps
  the previous section's exit, the outgoing section must not animate on exit:
  it simply scrolls away.
- **Deck stacking.** Sections do not scroll past each other; they stack.
  Every unpinned section is `position: sticky; top: 0; height: 100 vh`, so it
  stays put while the next section slides up over it. The covered section
  behaves like a card being covered in a deck: over the incoming section's
  slide (m 0 → 0.667 of the incoming runway, § 0c) it scales from 1.00 to 0.96 about its top-centre,
  translates up 24 px, and dims to 70 % brightness with its slag field
  paused; by the dock it is fully hidden under the new card and is set
  `visibility: hidden` until the user scrolls back. At most one covered card
  is ever partly visible, so there is no deep stack to render. The incoming
  card carries a 1 px seam along its top edge; no shadow, no rounding, per the
  identity. Mount-on-entrance (above) runs on the incoming card while it
  slides, so content assembles as the card arrives. The pinned sections (the
  dive, the math explainers) are cards too: they slide over the previous card
  the same way, then run their own pin, and when the next card covers them they scale,
  shift and dim exactly like an unpinned card. The hero is the first card. Reduced
  motion: cards still stack (sticky), without the scale, shift or dim.
- **Card layout, fixed rows.** Every card is a three-row grid: the kicker
  (`§XX · …`) in the top row, always at the top of the card under the topbar
  where one exists; the content in the middle row, vertically centred within
  the space it has; the status bar (divider plus its line) in the bottom row,
  always at the card's bottom edge. The headline is the first element of the
  middle row. Rows never move between cards, so the eye finds the kicker,
  the title and the status line at the same place on every card. This includes
  the dive states and the math explainer states: each carries a status bar in
  the bottom row (dive: the gate's one-line verdict and its exit code; math
  explainers: `● <gate> · <state title in lowercase>` left and the gate's exit
  code right; M5: as designed). The design PNGs for the explainer states predate
  this rule and omit the bar; the spec wins.
- **Breathing room on surfaces.** Cards, tiles, table cells, panes and hint
  boxes are never visually compacted. Floors at 1440 × 900, scaled with
  `clamp()` down to 1280 × 720 but never below 80 % of these values:
  card and pane padding 28 px (24 px on narrow tiles); gap between a block's
  title and its main text 12 px; between paragraphs or list rows 10 px;
  between a block's main text and its support text 16 px; table cell padding
  14 px vertical, 18 px horizontal; gap between sibling cards or columns
  24 px; stat tile figure to label 8 px; status bar 20 px above the card's
  bottom edge. When a card would not fit the viewport with these floors,
  reduce the type scale one step first, then the number of items shown, and
  split the section last; never shrink the padding. The design PNGs predate
  this rule where they are tighter; the spec wins.
- **Reading time is the reader's.** After release the section scrolls at
  normal speed; nothing in it moves any more except hover states.
- **Exceptions.** The hero (P0) mounts on load with its timed sequence, not on
  scroll. The pinned walkthrough (K1–K4) and the pinned math explainers pin when
  their top docks, as before (§ 13, § 16), but their first state's title types
  and its cards reveal during the entrance range above, so the pin begins
  assembled. Reduced motion: sections are complete when they enter view.
- **Discreet field.** The hero's slag-field shader (§ 14.4 item 1) also runs
  behind every unpinned section on every route, at low intensity: dots at 3 %
  instead of 6 %, heat peak 2 %, radius 60 % of the hero's, and the melt-on-exit
  removed. It never runs inside the K dive, which has the furnace instead.
  Same fallbacks and budget as the hero's.

## 1. Scroll geometry

- The intro is one pinned section. Pin height = **9 stops × 100 vh = 900 vh** of
  scroll for a 100 vh sticky viewport. S9 (numbers) unpins and scrolls normally.
- Progress `p ∈ [0, 1]` across the pin. Stop `i` owns `p ∈ [i/9, (i+1)/9]`.
  Within a stop, local progress `t ∈ [0, 1]`.
- Each stop splits into three phases:

  | phase | t range | what happens |
  |---|---|---|
  | **enter** | 0.00–0.10 | the typed command "executes": heading swaps, previous hint box leaves, output begins |
  | **work** | 0.10–0.70 | the stop's main motion: annotation, edit, counters |
  | **settle** | 0.70–0.85 | verdict line + exit code land, hint box arrives, rail dot grows |
  | **bridge** | 0.85–1.00 | the *next* stop's first machine line types, character by character, under the cursor (§ 5a) |

- Easing: scrubbed values use `linear` over scroll, with an `easeOutCubic` applied
  to the *mapped* range so motion decelerates as a stop completes. No spring, no
  bounce, no overshoot anywhere.
- Scroll snapping: **none**. Snap fights scrubbing. The rail dots are the affordance.

## 2. The scroll rail (left edge)

- 10 dots. The active dot is a 6×28 molten bar; others are 4×4 seam squares.
- Between stops the active bar **slides** to the next dot and shrinks while the next
  grows: both are scrubbed over the last 10 % of the outgoing stop and the first
  10 % of the incoming one, so there is never a moment without an active bar.
- Hover a dot: show the stop title in mono, 11 px, ash. Click: smooth-scroll to that
  stop's start (the only timer-driven scroll on the page).

## 3. Heading (code pane, top)

- Two lines: kicker (`§01 · crap`, molten mono) and title (Azeret Mono 34).
- **Kicker**: the `§0n` number rolls like an odometer, digit sliding up 1 em while the
  new digit slides in from below. The word after the dot cross-fades with a 4 px
  horizontal slide.
- **Title**: per-word stagger. Old words fall 8 px and fade over 60 ms of equivalent
  scroll; new words rise 8 px into place, 30 ms stagger per word, left to right.
  Words are never scrambled or "decoded"; this is a terminal, not a hacker film.

## 4. Code pane · line model

Treat the file as an ordered list of line records `{id, text, state, note}`.
Lines have stable ids across stops so the same line can move, re-tint, or die
without re-rendering the block. Line height 22 px (13 px × 1.7).

Allowed line transitions, all scrubbed:

| transition | motion |
|---|---|
| **tint on** (`""` → `hot/dup`) | background sweeps in from the left edge (clip-path 0 → 100 %), 80 ms equiv. Note tag pops in at the right 40 ms later: scale 0.9 → 1, opacity 0 → 1. |
| **tint off** | reverse of the above. |
| **retint** (`hot` → `new`) | background cross-fades; note tag text is swapped with a 1-line vertical roll. |
| **insert** | the lines below slide down by 22 px × n; new lines fade in from 0 → 1 while rising 6 px. Gutter numbers roll (odometer) as they renumber. |
| **delete** | line gets `del` tint and a strikethrough that draws from left to right (text-decoration via a clip on a pseudo element); then it collapses to 0 height and the lines below close the gap. |
| **replace text** | this is the `sed` moment, see § 6. Characters are not scrambled; the old text is struck and the new text types in beneath on the same line, then the struck part collapses. |

The file bar (`billing/pricing.py · python · 24 lines`) shows the line count as a
rolling number and gets a small molten `●` (modified indicator) for the duration of
any edit, which turns ingot on settle.

## 5. Agent pane · transcript model

The transcript is append-only within a stop and **scrolls itself to the bottom**
as lines arrive. On scrubbing backwards, lines are removed from the bottom.

### 5a. The bridge: the next command types as you scroll

The command for stop `i + 1` is **not** typed inside stop `i + 1`. It types during
the last 15 % of stop `i` (the bridge phase), so the act of scrolling *is* the act
of typing, and the stop boundary is the Enter key.

- At `t = 0.85` of stop `i` a fresh prompt line appears at the bottom of the
  transcript: molten `$` (for a gate command) or molten `▸` (for an agent tool
  call), then the block cursor. A faint ash label at the right edge of that row
  reads `↓ types as you scroll` on the first two stops only, then is dropped.
- Characters map linearly onto scroll: `chars_shown = round(len(cmd) × (t − 0.85)
  / 0.15)`. Scrolling back deletes them one by one. The cursor blinks on a clock
  the whole time and never moves ahead of the last typed character.
- At `t = 1.00` the line is complete. The first 2 % of stop `i + 1` is the Enter:
  the cursor drops one line, the typed line loses its cursor and becomes a normal
  `cmd` line, and the output / tool body starts. Only then do the heading and the
  code pane react. Cause (typing) always precedes effect (the pane changing).
- The storyboard shows each stop with its bridge already half-typed at the bottom
  of the transcript, which is how a frozen mid-scroll frame would look.
- What types per bridge (the exact first machine line of the next stop):

  | leaving | types |
  |---|---|
  | S0 | `$ coderefinery crap --changed` |
  | S1 | `▸ Edit billing/pricing.py · apply_patch <<'EOF'` (the first tool call of the fix) |
  | S2 | `$ coderefinery dry --threshold 0.8` |
  | S3 | `▸ Edit billing/quotes.py · apply_patch <<'EOF' · -def quote …` |
  | S4 | `$ coderefinery mutate billing/` |
  | S5 | `▸ Write tests/test_pricing.py · +def test_coupon_never_goes_negative …` |
  | S6 | `$ coderefinery dry tests/test_pricing.py` (typed slowest: the explicit path is the lesson) |
  | S7 | `$ coderefinery mutate billing/ --use-existing-coverage` |
  | S8 | `$ ` and nothing more; the empty prompt is held while S9 unpins and scrolls in |

- Typing speed is therefore a function of scroll distance, not time: a longer
  command uses the same 15 % of the stop, so it types faster per pixel. If that
  reads as rushed for the two tool calls, extend their bridge to 20 % by shrinking
  the work phase of S1 and S3 (both have slack after their tints land).
- Re-runs inside a fix stop (`$ coderefinery crap --changed` again in S2, S4, S6)
  are typed inside the stop, in the work phase, as before: they are not the next
  stop's line.

Line kinds and their motion:

- **command** (`$ coderefinery crap --changed`): arrives already typed, from the
  bridge of the previous stop (§ 5a). The block cursor blinks on a 1 Hz clock.
  Re-run commands inside fix stops type in place, scrubbed, with the same cursor.
- **output table**: rows reveal top to bottom, 1 row per 3 % of `t`. Numeric cells
  are odometers: on first reveal they roll from `0` to the value.
- **verdict** (`1 function above the gate`, slag) and **exit code**: appear together
  in the settle phase. A non-zero exit (`exit 2` for crap, `exit 3` for mutate) is slag; `exit 0` is ingot. dry always exits 0 and its verdict line is the pair count, so its slag line is the count, not the exit. The exit line gets a
  one-time 2 px molten underline that draws left to right on landing.
- **agent message** (boxed): the box draws its seam first (stroke dash-offset,
  top-left clockwise), then the text appears at once with its box (tier 2). Nothing types here;
  prose is not a terminal.
- **tool call** (new, used in fix stops): rendered like Claude Code's tool blocks:
  a collapsed header `▸ Edit billing/pricing.py` in chalk, with the command body
  beneath in ash mono. Header slides in; body types; a status chip at the right goes
  `running` (ember, pulsing dot on a clock) → `done` (ingot) when the matching code
  pane edit completes.

## 6. The fix beats (S2, S4, S6) · "sed, like an agent would"

This is the signature motion. Each fix stop runs the same choreography:

1. **t 0.00–0.10 · the first tool call executes.** Its header was typed in the
   previous bridge; now the body finishes and the chip goes `running`. An agent
   message with one sentence of intent fades in beside it.
2. **t 0.10–0.60 · tool calls.** The remaining tool blocks type in sequence.
   Each tool block's completion is *bound* to the code pane: as the command finishes
   typing, the corresponding lines change, with the same scroll range. The viewer
   sees cause on the right and effect on the left in lockstep.
3. **t 0.60–0.80 · re-run.** The gate command is typed again. Output rolls.
4. **t 0.80–1.00 · settle.** Exit code flips to `exit 0` ingot, hint arrives.

Tool calls per stop (the text shown; keep it real so engineers trust it):

**S2 · crap fixed**

```
▸ Edit billing/pricing.py                                  running
  apply_patch <<'EOF'
  @@ def price(order, user, coupon=None):
  -    total = 0
  -    for item in order.items:
  …
  +    total = sum(_item_cost(i) for i in order.items)
  +    total = _apply_tier(total, user.tier)
  EOF
▸ Write billing/pricing.py  (+_item_cost, +_apply_tier, +TIER_RATE)
▸ Write tests/test_pricing.py  (+4 cases)
$ coderefinery crap --changed
```

Code pane binding: the first hunk collapses lines 2–9 into one `new` line (delete
+ insert, as in § 4). The second collapses 10–13. The `Write` for helpers inserts
the two functions below, sliding the test comment down. The test-file write only
changes the note tag on the last line: `cov 20 % → 100 %` with the two numbers
rolling.

**S4 · dry fixed**

```
▸ Edit billing/pricing.py
  apply_patch <<'EOF'
  -def quote(cart, account, promo=None):
  -    amount = sum(_item_cost(i) for i in cart.items)
  …
  EOF
▸ Edit billing/pricing.py
  apply_patch <<'EOF'
  @@ def price(order, user, coupon=None):
  -    total = sum(_item_cost(i) for i in order.items)
  -    total = _apply_tier(total, user.tier)
  -    if coupon and coupon.valid():
  -        total -= coupon.amount
  -    return round(max(total, 0), 2)
  +    return _settle(order.items, user.tier, coupon)
  EOF
$ coderefinery dry --threshold 0.8
```

Binding: the first hunk strikes `quote`'s body lines (strike draws, then collapse)
while the ember tint on them fades. The patch replaces `price`'s body with one
`new` line and inserts `_settle` below. The `0.91` note on the old pair tag rolls
down to `—` and the tag fades.

**S6 · mutate fixed**

```
▸ Write tests/test_pricing.py
  +def test_coupon_never_goes_negative():
  +    assert price(order(5), gold, coupon(50)) == 0.0
  +def test_unknown_tier_pays_full_price():
  +    assert price(order(100), bronze) == 100.0
$ coderefinery mutate billing/
```

Binding: two test functions insert at the bottom (`new` tint). As each lands, the
matching `SURVIVED` note tag on line 6 / line 9 retints ember → ingot and its text
rolls `SURVIVED` → `killed`. In the agent pane the `survived 2` counter rolls to
`0` and `killed 10` to `12` in the same range.

Every edit is shown as an `apply_patch` hunk, never a `sed` one-liner: `sed -i`
syntax differs between GNU and BSD, and the point is that the commands shown
would actually produce the file shown on any machine.

## 7. Find beats (S1, S3, S5)

- **S1 crap.** The command arrived typed; the table reveals. As the CRAP cell rolls `0 → 50.5`
  (scrubbed over t 0.25–0.55), the eight decision lines tint molten one at a time, in
  source order, one per 4 % of t, so the number and the tints finish together.
  Line 16's note (`branch · untested`) is the last tag to pop, at t 0.60. The agent
  message writes the formula; the three terms of the formula `cc²`, `(1 − cov)³`,
  `cc` each briefly flash molten when the corresponding code tint completes.
- **S3 dry.** The second function `quote()` does not exist at t 0. It **inserts**
  below `price` (lines slide, gutter rolls) over t 0.15–0.40, in plain chalk. Then
  the command types, and as the score rolls `0.00 → 0.91` the two bodies tint ember
  *in pairs*: line 2 of `price` and line 9 of `quote` together, then the next pair,
  so the structural match is visible as a rhythm. The pair tag `0.91 similar to
  price` pops last.
- **S5 mutate.** Command types; `12 sites · 12 mutants · running tests` appears
  with a progress bar made of `█▒░` that fills over t 0.2–0.6 (scrubbed). Behind it
  the code pane cycles: each mutation site briefly shows its mutant text (`and` →
  `or`, 180 ms equiv.) and reverts, with the note tag reading `killed` in ingot,
  except the two survivors, whose mutant text **stays** and whose tag lands as
  `SURVIVED` in ember. `killed 10` / `survived 2` roll in the settle phase.

## 8. Hint box

- Appears only in the settle phase of each stop (t 0.80–1.00): rises 12 px while
  fading in; the molten stroke draws clockwise from top-left over the same range.
- Leaves in the enter phase of the next stop (t 0.00–0.15): drops 12 px and fades.
- **Position is bound to a target line.** Each hint declares an anchor
  (`S1 → L16`, `S3 → L8`, `S5 → L6`, fixes → the verdict line in the agent pane). On
  desktop the box sits 24 px below the anchor's row, left-aligned to the code text,
  clamped to stay inside the viewport. A 1 px molten leader line connects box to
  anchor and draws with the stroke. On narrow screens (< 1024 px) it docks to the
  bottom edge and the leader is dropped.
- Formula line (`CRAP = cc² × (1 − cov)³ + cc`): each term fades in 40 ms apart.

## 8b. Test beats (S7, S8) · the tests are code too

The three gates are run on the *tests* now. The file bar switches to
`tests/test_pricing.py · python · test file`; the switch itself is a motion: the old
file name slides up and out, the new one slides in from below, and the modified
dot turns ash because this file has not been touched by the gates yet.

**S7 · tests · dedup.** The skill's rule is that every duplication at 0.8 or above
goes, and the gates skip test directories unless you name the file. So the command
is the long form, `coderefinery dry tests/test_pricing.py`, and the typing of the
explicit path is deliberately the slowest typed command in the intro (t 0.00–0.25),
so the viewer notices it; it types in the bridge out of S6.

1. **t 0.25–0.50 · the pairs.** Score rolls `0.00 → 0.93` twice, once per pair. As
   each pair's score lands, the two tests tint ember *in lockstep line by line*
   (same rhythm as S3): `make_order(...)` in gold with `make_order(...)` in silver,
   then the two `assert` lines. The second pair repeats it against bronze.
2. **t 0.50–0.80 · the patch.** One tool call, `apply_patch`, typed. Bound to the
   code pane: the three test functions strike through top to bottom, collapse, and
   the parametrized block inserts where the first one stood. The `@pytest.mark.
   parametrize` line is the first `new` line to land; the three tuples
   `('gold', 90.0) ('silver', 95.0) ('bronze', 100.0)` then type in one at a time,
   and as each tuple finishes, the matching collapsed test's ghost (a 1-line ash
   label at the right margin, `← test_silver_discount`) fades out. Three functions
   become three rows of one table, visibly.
3. **t 0.80–1.00 · settle.** Line count in the file bar rolls `14 → 9`. The chip on
   the tool call goes `running → done`. Hint arrives.

**S8 · tests · relevance.** This is the only stop whose number does not move much,
and that is the point: `killed 12 / survived 0` lands early (t 0.15) in ingot and
stays. The motion is in a second table.

1. **t 0.15–0.45 · per-test kills.** A table `per test: mutants it alone would
   kill` reveals one row per 5 % of `t`. Each count rolls from `0`. The rows that
   land on a non-zero value stay ingot; the two that land on `0` flip to molten
   on landing, and at the same moment their test function in the code pane tints
   `del` with the strikethrough drawing left to right.
2. **t 0.45–0.65 · the note tags.** The tags explain the zero: `passes if price()
   returns 0` and `tests the implementation`. They pop one after the other, and
   on the spy test the tag text is replaced in place by a second tag,
   `survives max→min, 1.0→0.0`, with a 1-line roll, so the viewer reads cause then
   consequence.
3. **t 0.65–0.85 · the delete.** One tool call, an `apply_patch` removing the two
   functions, typed. Bound: both struck blocks collapse, the lines below slide up, gutter
   rolls. The surviving `test_coupon_never_goes_negative` gets a quiet `kills
   max→min` tag in chalk so the contrast is explicit: this test is kept because it
   kills something.
4. **t 0.85–1.00 · settle.** `exit 0` lands in ingot. Hint arrives, anchored to
   the deleted spy test's former position (the anchor is the line id, which now
   points at the gap, so the leader line points at where the test *was*).

Note for the build: the per-test kill table is a **proposed** feature. The
mutator today reports kills per function, not per test. Until it exists, the
honest version of this stop runs `mutate` twice, once with the suspect tests
deselected (`-k "not returns_a_number and not settle_called"`), and shows that
the kill count does not change. The motion is the same; only the transcript text
differs. Decide before implementing.

## 9. S9 · the numbers

Unpinned, normal scrolling. Triggered when the section is 30 % in view, scrubbed
over the next 40 % of viewport travel:

- Each big number rolls from `0` to its value, odometer per digit, right-most digit
  settling last. The `×` and `/` glyphs are static. `120×` (molten) starts 10 % later
  than the others so it lands last.
- The fact columns reveal top to bottom per line, ash → chalk on landing.
- The status bar's `exit 0` draws its molten underline when the section reaches
  60 % in view. It is the last thing to move on the intro.

## 10. Reduced motion and fallbacks

- `prefers-reduced-motion: reduce`: no typing, no odometers, no line slides. Each
  stop becomes a static keyframe (exactly the storyboard frames) cross-faded on
  scroll. The hint box and verdict lines still appear and disappear, by opacity only.
- No JS: the pinned section is replaced by the eight keyframes stacked vertically.
- Touch / narrow: pin height drops to 7 × 100 vh by merging each fix beat into its
  find beat and S7 into S8 (tool calls still shown, code changes without per-line choreography).

## 11. Implementation notes

- Prefer CSS scroll-driven animations (`animation-timeline: view()` /
  `scroll()`) for everything that is a pure property tween: tints, slides, opacity,
  stroke draw. Use a small JS scroll-progress observer only for the things CSS
  cannot express: odometer digit mapping, typed-character count, line insert /
  delete reflow, and transcript auto-scroll. GSAP ScrollTrigger with `scrub: true`
  is the fallback where native timelines are unsupported.
- Render the code pane from the line-record model, not from a string, so ids are
  stable and reflow is animatable. Gutter numbers are their own odometers.
- All numbers that roll come from one data file (`site/data/intro.json`) that also
  feeds the storyboard, so the design, the copy and the animation never disagree.
- Budget: the pinned section must stay under 16 ms per frame on a 2020 laptop with
  all 8 stops' DOM mounted. Mount every stop's lines once; animate state, never
  re-create nodes during scroll.

## 12. Pain-first opening (P0–P6) · replaces S0

Home page order: hero, the pains, what coderefinery is, why a formula
changes what the agent does, the tests the agent writes, then the pinned
walkthrough (K1–K4), the numbers (S9), the math teaser, and **the recommended
setup as the last card** before the footer. P5 therefore no longer precedes
the dive; P4's CTA slab leads straight into it. They scroll
normally; the only scroll-driven motion is number rolls, row reveals and the
transcript replays. Tier rules (§0b) apply. The walkthrough then pins and starts
at K1. S0 is retired.

- **P0 · hero.** Two columns over an event stream. Left: headline, one line
  of mechanism, install line, the "one binary …" foot line. Right: the four big
  numbers as a 2 × 2 block. Behind everything, above the slag field: an
  **infinite stream of CI events rising from the bottom edge**. This is the
  hero's identity; the load sequence stays on a clock ("Prove it." at 200 ms,
  numbers roll molten last), and the stream starts with the page and never
  stops.

  The stream:
  - **Events** are small pills: a status glyph, a message, a PR number.
    Glyphs and tones: `✓` ingot (lint ok, tests passed, gate killed 12, merged),
    `▸` ember (build running 0:22, tests running `▂▃▅▆ 3:12`, mutate running),
    `✗` / `⛔` slag (build failed, 3 tests failed, `survived 2 · exit 3`,
    `CRAP threshold exceeded`, `DUPLICATE score=0.91`), `○` seam (queued,
    waiting for checks, review requested), `↑` ash (agent pushed commits). The
    design PNG shows the vocabulary; the live site draws from a list of ~30
    event strings with PR numbers that increment over time.
  - **Spawn** one event every 0.35–0.7 s at a random x across the width, at the
    bottom edge, rising at 42 px/s (constant, no easing). Spawn x is spread so
    no two live events overlap horizontally within 24 px of each other. Running events tick
    their bar and elapsed time each second while visible.
  - **Fade band.** Events are at full stream opacity (50 %) below the foot line
    (the "one binary …" text, design y = 594). From the foot line upward they
    fade linearly and reach 0 at the top of the mechanism sub-heading (design
    y = 330). Above that nothing is drawn, so the headline area is clean. The
    thin horizontal pipes behind the events obey the same band.
  - **Blinks.** A failing event (slag) blinks twice on spawn, 120 ms each, and
    carries a soft slag halo (10 px, 27 %) for its first 2 s. One in roughly
    eight events is a failure.
  - **Particles.** 2–3 px molten sparks spawn with each event (1–3 of them),
    rise at 1.5× the stream speed, drift ±6 px sideways, and fade over 1.6 s. A
    failure spawns a small burst of 6. Sparks obey the fade band.
  - **Attention budget.** At most 30 events and 40 sparks on screen; older
    ones are removed as they leave the band. Total motion stays under 1 ms per
    frame of JS; events are transformed, never re-laid-out.
  - **Parallax.** The stream has three depth layers, assigned at spawn:
    near (30 % of events, scale 1.0, speed 1.0×, band opacity 1.0×), mid
    (45 %, scale 0.85, speed 0.7×, opacity 0.7×), far (25 %, scale 0.7, speed
    0.45×, opacity 0.45×). Sparks belong to their event's layer. Two inputs
    move the layers against each other: **scroll**, where the hero's own
    scroll-out shifts near events up by 1.0× the scrolled distance, mid by
    0.6× and far by 0.3×, so the stream pulls apart as the hero leaves; and
    **pointer**, where the layers shift horizontally by −12 / −6 / −2 px at
    the viewport's left edge to +12 / +6 / +2 px at the right, eased over
    400 ms. Near events render above far ones. Hover applies to near and mid
    only; far events are not interactive. Reduced motion: the layers keep
    their scale and opacity but no parallax shift.
  - **Placement.** Events spawn anywhere across the hero's full width and
    rise through its full height, at every viewport size; they are not
    confined to any column or box. The fade band is the only thing that limits
    them, and it is measured from the live positions of the foot line and the
    sub-heading, so it tracks every layout.
  - **Hover.** An event under the pointer pauses, lifts to full opacity with a
    molten seam, and shows a one-line detail beneath it (`PR #482 · opened
    14 min ago · 3 checks · 1,204 tests`). Its sparks pause with it. On leave it
    resumes at the stream speed and settles back to band opacity over 300 ms.
    A hovered failure stops blinking and shows its reason in the detail line
    (`exit 3 · survived 2 · pricing.py:6, :9`). Hover is pointer-only; touch
    ignores it. The install box and the numbers block keep pointer priority,
    so an event under them is not hoverable.
  - The pointer heat of the slag field still shows through the stream.
  - **Reduced motion**: a static arrangement of ~20 events in the band, no
    sparks, no blinks. Narrow screens: the stream keeps running but spawns
    every 1 s.
- **P1 · the pains.** Three columns (review, trust, ci): the stat rolls (tier 1);
  the quote and bullets appear at once with the column (tier 2). Sources in the
  status bar are links. Every number must trace to
  `designs/reports/Developer pains with AI code quality.md`; a number that cannot
  be cited is removed, not softened.
- **P2 · how it helps.** The four gate cells open along their left seam
  (§14.3), 60 ms apart; each cell's command types in, scrubbed, as the cell
  opens. The two lists beneath draw their seams and appear at once.
- **P3 · gates beat reasoning.** The three-column table draws its outer seam;
  rows appear top to bottom, 40 ms apart, each at once. Beneath it the two
  transcript panes appear at once; the only tier-1 motion is the slag `exit 2`
  line and the molten CRAP line in the deterministic pane, which land 200 ms
  after their pane. Nothing types.
- **P4 · the tests.** Stats roll (tier 1); the quote and the pattern list
  appear at once (tier 2). The slab on the right appears at once; its
  mutant-to-test rows are tier 2. The CTA slab at the bottom draws its molten
  stroke and the button's arrow nudges down once on a clock. No click is needed:
  the pin starts when the slab reaches the top of the viewport.
- **P6 · hand-off (now from P4).** The last 10 % of P4's entrance-complete
  scroll is the bridge into K1: `$ coderefinery crap --changed` types inside
  the CTA slab (replacing the "keep scrolling" label), and on completion the
  dive card slides over P4 and K1's enter phase runs (§ 5a). There is no
  session pane to hand over any more: the code pane and the terminals arrive
  with the dive card's own slide.
- **P5 · recommended setup, last card.** Kicker `§08 · recommended setup ·
  two minutes`. The settings file is static. The transcript plays the refused
  commit and continues it: refusal, the agent writing two tests (tool call
  types, scrubbed in its slot), second commit, hook passes, `exit 0`, the commit
  hash lands in ingot. The status bar's "zero deleted" fades in last. Linked
  from the agents page CTA as `/#hook`. The footer follows.

## 13. The walkthrough as shipped (K1–K4) · the dive

K1–K4 are the pinned walkthrough on the page. S1–S9 stay in the file as the
long-form reference for the same transitions; the site ships K. The CTA on P4
needs no click: the page pins when the slab reaches the top of the viewport.

### 13.1 Look: you are inside the machine

Everything above the pin is coke black. The pin is a different place:

- **Full bleed.** While the dive is pinned, its ground, heat glow, scanlines
  and furnace canvas cover the entire viewport edge to edge, not just the
  section's content box: no coke margins at the sides, top or bottom, and the
  topbar area is furnace too. The furnace layers are fixed to the viewport
  (not to the content column) for the whole pin, and the card's top seam is
  the only line between the previous card and the furnace. The same applies
  under 1024 px when the dive stacks: each stacked state is full-bleed.
- **Ground** turns `furnace` (#150904), a warm near-black, over the first 15 % of
  the pin (cross-fade of the body background, scrubbed). Seams turn from grey to
  `heat-seam` (molten at 35 %) in the same range. Nothing else on the page uses
  this ground, so the colour alone says "we went down".
- **Heat glow**: a radial molten glow bleeds in from the top edge. Its opacity is
  tied to depth: faint on K1, strongest on K4. On exit it fades over the hand-off
  back to the numbers section.
- **Hint box** is filled molten with coke text, the inverse of every other box on
  the page. One per state; it is the heuristic, in one sentence, with the formula
  as its kicker.
- **Depth gauge** replaces the scroll rail at the left edge: `surface` at the
  top, `core` at the bottom, four marks. The molten fill descends as the state
  advances, and the active mark is larger. A 4 px molten progress bar along the
  bottom edge fills 25 % per state.
- **Kicker** reads `▼ inside the gate · 0n/04 · <gate>` and the `0n` rolls.
- **Restraint**: the ground is only two steps warmer than coke (#110B09), the
  glow sits at 10 % and the seams at 20 %. Orange is a temperature cue, not a
  surface. Secondary text inside the dive uses `dive-ash` (#A9ADB6), one step
  lighter than `ash`, to keep contrast on the warm ground.
- **Syntax colour** in every code pane, the same six roles everywhere on the
  site: keywords `syn-kw` (#7CB8FF), strings `syn-str` (#8FD9A8), numbers
  `syn-num` (#FFC95C), function names after `def` in chalk, comments in ash,
  decorators `syn-dec` (#C9A0FF). Tints and strikethroughs sit under the
  colour; a struck line goes fully ash. Molten is never a syntax colour, so a
  molten line always means a verdict.

### 13.2 Scroll geometry

Pin height = 4 states × 100 vh = 400 vh. Each state owns a quarter. Phases per
state as in § 1 (enter, work, settle, bridge), with the bridge typing the next
state's command in the "before" terminal (§ 5a).

### 13.3 What changes between states · CLI and agent action

The dive no longer shows a full code pane. Each state is the agent's session
on the left (commands, the gate's output, the agent reading it and acting
through tool calls, the re-run) and an **outputs** pane on the right with three
blocks: `before` (the gate's table and exit code), `what the agent changed`
(a diff summary: `−12 +9 · 2 files`, the hunks as one-line summaries), and
`after` (the re-run's table and `exit 0`). The hint box at the top right holds
the formula or heuristic; the full math is on `/math`, which keeps the code.

Persistent objects: the session pane and the outputs pane. Between states
their content changes; the layout never does.

| state | session pane (scrubbed in order) | outputs pane |
|---|---|---|
| K1 crap | `crap --changed` types (bridge) → the CRAP line lands molten, `exit 2` slag → the agent's reading appears (revealed) → two tool calls type their headers, bodies reveal → re-run types → the three `exit 0` lines land ingot | `before` reveals with the 50.5 rolling in; `what the agent changed` reveals hunk by hunk as the tool calls complete; `after` reveals on the re-run, rows landing ingot |
| K2 dry | `dry` types → pair block lands molten → reading → one tool call → re-run → "No duplicate candidates found." | same three blocks; `before` is the pair, `after` is the clean line |
| K3 mutate | `mutate` types → progress line → two SURVIVED lines land molten, `exit 3` slag → reading → one `Write` tool call with the two tests → re-run → `killed 12 · survived 0 · re-ran 2` | `before` table with survivors in molten; changed: two tests, "no source line changed"; `after` all 100 % |
| K4 tests | `tests` types → table with two zero rows molten → `dry tests/…` types → the 0.93 pair → reading → one tool call → re-run clean | `before`: the two zero-kill rows and the copies; changed: three deletions in slag, one parametrize in ingot; `after` clean |
| K4 → numbers | the glow fades, the ground returns to coke, the gauge drains, the panes unpin and scroll away as one block | |

The verdict flash (§ 14.4) fires on every exit line in both panes.

### 13.4 Reduced motion and mobile

Reduced motion: four static states cross-faded by scroll, the depth gauge still
fills. Mobile: the same four states stacked, unpinned, code pane above the
terminals, hint box docked under the title; the furnace ground and the depth
gauge remain so the section still reads as a different place.


## 14. Structure motion and shaders: where the fun lives

Text appears; structure moves. The "fun" of the page is in how the layout
itself behaves, never in the prose. Four devices, each with one job.

### 14.1 Seams draw around the words

Every bordered block on the page (stat strips, comparison tables, gate cells,
terminal panes, cards) draws its seams *after* its title and main text are in
place and *before* its support text (§ 0c, text-before-box): a 1 px stroke
animates along its path, top-left clockwise, over its scroll slot (220 ms on a
clock when the block mounts on load, as in the hero), while the surface tint
fades in underneath. Nested blocks draw inner seams one slot step after the
outer. This is the one structural reveal that applies everywhere, so the page
has a signature without animating text: the words land, the frame forms
around them, the small print follows.

### 14.2 Panes are objects, not screens

Across the whole page there are three persistent panes: the session pane
(P0 → P5), the code pane and the terminals (K1 → K4). They are never re-mounted.
They slide, resize and swap content. The hand-offs are the set pieces:

- **P5 → K1**: the session pane slides right and out while the code pane slides
  in from the left; both share the same vertical centre so the eye follows one
  object. The ground cross-fades to furnace underneath during the slide.
- **K3 → K4**: the source file leaves upward and the test file arrives from
  below inside the same pane; the file bar rolls its name like an odometer.
- **K4 → numbers**: the panes unpin and scroll away as one block; the glow drains.

### 14.3 Layout moves in opposition

- **P2 comparison**: left-column rows slide in from the left, right-column rows
  from the right, in alternation, so each pair meets in the middle seam.
- **P4 gate cells**: each cell opens along its left seam: the seam splits, the
  cell's content is revealed by a clip that widens from the seam outward, 180 ms,
  cells 60 ms apart. Gates open.
- **Hero stats strip**: the strip's top seam draws across the full width, then
  the four tiles appear at once; only the figures roll.
- **Depth gauge**: the molten fill descends in a continuous scrub across K1–K4;
  the marks pop when passed. It is the only element that moves across all four
  states without a reset.

### 14.4 Shaders: temperature, not decoration

Three fragment shaders, all WebGL 1, all full-screen quads behind content,
never over text. Each has an `intensity` uniform driven by scroll and a `time`
uniform for drift. All are disabled under `prefers-reduced-motion`, on
`navigator.connection.saveData`, and when the first frame renders slower than
8 ms; the CSS fallback is the static gradient already in the design.

1. **Slag field (hero, P0 only)**: a fine dot grid (6 px pitch) on coke. A
   radial heat (molten at 6 % peak) follows the pointer with 300 ms lag. When
   the hero scrolls out, the dots brighten briefly and sink along y, as if
   melting, over the last 20 % of the hero's travel. Intensity 0 → 1 → 0.
2. **Furnace (the pin, K1–K4)**: value-noise ember glow bleeding from the top
   edge, drifting at 0.02 Hz, amplitude tied to depth: 0.3 on K1 to 1.0 on K4.
   Hue is fixed molten; only luminance moves. Peak luminance on the ground never
   exceeds 12 %, so the code panes keep contrast. The scanline is a 1 px
   horizontal darkening every 3 px at 4 % opacity, static, present only inside
   the pin. It is the "inside the machine" texture.
3. **Verdict flash (terminals)**: when an exit code lands, the terminal pane
   runs a 240 ms ripple: a horizontal band of +3 % luminance sweeps top to
   bottom once, like a phosphor refresh. Ingot-tinted for `exit 0`, slag-tinted
   otherwise. It runs on a clock at the moment the scrubbed verdict reaches
   full opacity; scrolling back does not replay it.

Budget: all three shaders together stay under 2 ms per frame on a 2020 laptop
at 1440 px wide. Each is a single pass with no framebuffer. If the implementer
cannot hit that, drop 3, then 1, never 2.

### 14.5 What never moves

Nav, footers, status bars, file bars, the math section's formulas, the wordmark
(except its cursor blink), and any prose. Hover states are colour only: molten
text on links, a 1 px molten seam on cards. No scale, no shadow, no lift.

## 15. The "for agents" page (A0–A4) · route /agents

A second route, linked from the nav as "for agents". It argues one thing: a
deterministic verdict changes what the agent does, because the agent optimises
for the signal it gets. Text-heavy by design; tier rules (§0b) apply strictly.

- **A0 · hero.** Headline static. The five-row property table draws its seams
  (§14.1) and its rows appear at once. No shader on this page except the slag
  field at 50 % intensity in the hero.
- **A1 · the loop.** The five step boxes draw their seams left to right, 80 ms
  apart, and the arrows between them draw as strokes after the box to their
  left. The two transcript panes beneath appear at once (tier 2); the only tier-1
  motion is the `exit 2` line and the molten CRAP line in the deterministic
  pane, which land 200 ms after their pane appears.
- **A2 · what the agent reads.** The table draws its outer seam, then rows
  appear top to bottom 40 ms apart, each at once. The gate cell's coloured label
  is tier 3. Nothing rolls on this screen: every number here is an example, not
  a change.
- **A3 · reliable tests.** Both tables as A2. On hover, a mutant row highlights
  its matching rule row on the right with a molten seam, and vice versa, so the
  two tables read as one argument.
- **A4 · same everywhere.** Three cells open along their left seam (§14.3);
  the CTA slab draws its molten stroke. The button links to the hook section on
  the home page (P5).

## 16. The "math" page · route /math · code plus formula

A third route, linked from the nav as "the math". The home page keeps only a
teaser in M's place (headline, the four-formula strip, "read the math →").

The page is M0 (hero), then four pinned explainers, one per gate, then M5
(parity). Each explainer is one pinned section of `states × 100 vh`
(crap 3, dry 3, mutate 3, tests 2) with the same persistent layout: the
**formula across the top**, the **code pane on the left** (the function the
gate is evaluating, with annotations that show where each value comes from),
the **card on the right** (title, prose with variables set as code, the rule,
and a small visual where one helps), a state rail top-right, a status bar in
the bottom row. This is the one place on the site where code is shown in
full; the home dive shows the CLI and the agent instead.

### 16.1 The formula computes as you scroll

The formula title is not static text: each state **substitutes the value it
just computed** for the variable, in molten, and the substitution is scrubbed.
crap: `CRAP = CC² × (1 − cov)³ + CC` → state 1 `9²` and `9` roll in as the
nine decision lines tint in the code → state 2 `(1 − 0.20)³` as the covered
lines turn ingot and the rest ember → state 3 `81 × 0.512 + 9 = 50.5` with
the whole function tinted and the `exit 2` note landing. dry: `A = fp(price) ·
22  B = fp(quote) · 22` as the names fade to `:symbol` → `score = 20 / 22` as
the matching line pairs tint in step → `score = 0.91 ≥ 0.80 → DUPLICATE`.
mutate: `sites = 12  mutants = 12` as the site notes pop → `score = 10 / (10
+ 2)` as the killed lines turn ingot and the two survivors slag → `score =
10 / 12 = 0.83 · survived 2 → exit 3`. tests: `kills(t) = … → 4 · 1 · 5 · 0 ·
0` with the per-test notes landing → `unique(t) = … → 2 · 1 · 0 · 0 · 0`.
A value rolls (odometer) when it is a number; a word (`DUPLICATE`, `exit 3`)
lands. Previously substituted values stay chalk; only the newest is molten.

Shared choreography per state (t ∈ [0, 1]): 0.00–0.15 the previous molten
term cools, the new substitution begins, the card content reveals; 0.15–0.85
the code annotations tint/pop in the order the card describes while the value
rolls to its final figure; 0.85–1.00 hold. Scrolling back reverses. Reduced
motion: static states, cross-faded.

### 16.2 Code pane annotations per state

| state | code annotations (scrubbed) |
|---|---|
| crap 1 | the eight decision lines tint molten one by one with notes `1 · for` … `8 · if`; the card's nine squares light in step |
| crap 2 | covered lines turn ingot (`covered`), the rest ember (`never run`); the card's bar fills to 20 % |
| crap 3 | the whole function tints molten; note `CRAP 50.5 · exit 2`; the card shows 50.5 (slag) beside 9 (ingot) |
| dry 1 | the renamed arguments get `order → :symbol` notes, ember |
| dry 2 | the five matching line pairs tint ingot in step, notes `= line 9` … |
| dry 3 | both bodies tint molten; `DUPLICATE score=0.91` and `↔ pricing.py:1-6` |
| mutate 1 | the twelve site notes pop on their lines (`and → or`, `− → +`, `0 → 1`, …) |
| mutate 2 | killed sites turn ingot, the two survivors slag with `SURVIVED`; the test lines get `kills …` notes |
| mutate 3 | survivors keep slag with the test each one needs (`needs: a case where the result is exactly 0`) |
| tests 1 | each test gets its `kills n` note; the two zero-kill tests tint slag |
| tests 2 | `unique n` notes; the shadowed test turns ember with `shadowed by lines 1, 4` |

### 16.3 M0 and M5

- **M0 · hero.** Static headline; the four-formula strip draws its seams then
  appears at once. Hover: molten seam on a formula box.
- **M5 · parity.** The five numbers roll from 0 when the strip enters view,
  the molten `36/36` last. The four determinism items and the CTA appear at
  once; the CTA draws its molten stroke. The button links to the specs on
  GitHub.

No furnace or verdict shaders on this page; the discreet slag field (§ 0c) runs
behind every section here as on every route.

## 17. Mobile and touch · the page on a phone

Breakpoints by viewport width: **desktop ≥ 1024 px** is everything above;
**tablet 768–1023 px** keeps the stacked tablet layout the site already has
(desktop type and motion, columns stacked where the § 0c floors would break,
no 100 vh fit requirement, the hero laid out as on a phone so landscape
phones are not clipped);
**phone ≤ 767 px** follows this section. Touch (`pointer: coarse`) applies the
touch rules at any width. Test at 390 × 844, 360 × 800, 768 × 1024 and in
landscape 844 × 390 for sanity.

### 17.1 Layout: stack, no deck, natural height

- Cards are **natural height, minimum `100svh`**, and they **stack as a deck
  exactly like desktop** (§ 17.4): sticky cards, the incoming card sliding
  over the covered one with the same scale, brightness and molten-seam
  arrival. The kicker still sits first in the card and the status bar last;
  the status bar may wrap to two lines.
- One column everywhere: columns, side-by-side panes and tile grids stack in
  reading order. Stat tiles go two per row. Two-column comparison tables (P3)
  become stacked pairs: left cell as a label row, right cell under it. Wider
  tables scroll horizontally inside their card with a fade on the clipped edge;
  the document itself never scrolls horizontally (document width equals the
  viewport width at every scroll position).
- Spacing floors (§ 0c) run at 80 %: card and pane padding 20 px (16 px on
  narrow tiles), gap between stacked cards' inner blocks 16 px, table cells
  12 × 14 px. Type: headlines `clamp()` down to no smaller than 28 px, still
  line-height 1.25; body no smaller than 15 px; mono no smaller than 12.5 px;
  inline code as § 0b. Tap targets at least 44 × 44 px.
- Topbar: the wordmark and a menu button; the nav opens as a full-width sheet
  under the topbar, closes on choose. Anchor landings (`/#hook`) behave as on
  desktop: the target and everything above it mounted at once.
- Hero: headline, sub-heading, the install box full width, the four numbers as
  a 2 × 2 grid under it, then the foot line. Height `100svh` minimum, never a
  fixed `100vh`.

### 17.2 Motion: same order, shorter runway

- Mount on entrance still applies, with entrance `e` for the card itself
  (0 = card top at the viewport bottom, 1 = card top at 40 % of the viewport).
  Slide 0–0.30 (scale 0.97 → 1.00 about the top-centre, brightness 0.55 → 1,
  molten top seam cooling, as on desktop), kicker 0.30–0.33,
  headline types 0.30–0.48, blocks in reading order from 0.45 to 0.92 with the
  text → box → support steps of § 0c, status bar 0.92–1.00. Blocks start every
  0.06; a card with many blocks compresses the step so the last ends at 0.92.
  Once a card has mounted it stays mounted. Cards taller than the viewport
  mount their blocks on each block's own entrance (block top reaching 85 % of
  the viewport) rather than the card's, so nothing below the fold waits.
- The dive (§ 13.4): four states stacked and unpinned, each a card with
  furnace ground, the session pane above the outputs pane, the hint box
  directly under the title. Each state shows its final content; the typed
  command, output lines and verdict flashes play on the block's entrance, once.
  The depth gauge becomes a thin horizontal bar pinned at the top of the dive
  that fills as the four states scroll past.
- The math explainers: each state is its own stacked card: formula (allowed
  to wrap, no smaller than 24 px, the active term molten and its value rolled
  on entrance), the card prose and rule, then the code pane with that state's
  annotations already applied. No pin, no state rail; the kicker carries
  `§01 · crap · 1/3`.
  On a phone the code pane's annotation tags never overlay the code: the pane
  scrolls horizontally and each tag is appended after its line's text, on the
  same row, so the tag sits past the code and is reached by the scroll; the
  rail dots and row tints stay in place. The hero's event stream runs only in
  the band between the install box and the foot line, never behind text.
- The hero stream runs with at most 12 events and 16 sparks, no parallax from
  the pointer, scroll shift only; a tap on an event shows its detail line for
  two seconds. No hover anywhere on touch. The slag field runs at half dot
  density on phones and keeps its fallbacks; the furnace stays.
- Odometers, typed titles and reveals keep their durations; reduced motion as
  § 10.

### 17.3 Budget

Scrubbing a phone page must stay under 16 ms per frame on a 2022 mid-range
device; if a shader cannot keep it, it drops to its static fallback for that
session. Images and fonts the same as desktop; no extra assets.

### 17.4 The deck on a phone

The desktop section effects (§ 0c deck stacking and slide) run on phones and
tablets too; only the geometry changes because cards may be taller than the
viewport.

- **Sticky by the edge that fits.** Every card is `position: sticky` inside a
  wrapper. A card no taller than the viewport sticks at `top: 0` as on desktop.
  A card taller than the viewport sticks at `top: calc(100svh − cardHeight)`,
  a negative offset, so the reader scrolls through it normally and it pins by
  its bottom edge once its bottom reaches the viewport bottom; the next card
  then slides over it. The wrapper height is `cardHeight + 50svh`: the card
  holds for 50 svh after it pins before the next card's top appears, the same
  runway idea as desktop, shortened to a phone's scroll. The last card's
  wrapper has no runway and ends with the footer under it.
- **Covered card.** During the incoming card's slide (its top from the
  viewport bottom to the dock) the covered card scales 1.00 → 0.96 about its
  top-centre, moves up 16 px, dims to 70 % and pauses its field; once fully
  covered it is `visibility: hidden` and everything inside it is inert, as on
  desktop. At most one covered card is partly visible.
- **Incoming card.** Scale 0.97 → 1.00, brightness 0.55 → 1.0, and the 1 px
  molten top seam cooling to the seam colour, over the slide (§ 17.2, e 0–0.30
  of its entrance, where the slide range is the card's travel from the viewport
  bottom to the 40 % mark). Content mounts after that in the § 17.2 slots;
  blocks below the fold of a tall card mount on their own entrance.
- **Pinned desktop sections** (dive, math) stay stacked state cards on phones
  (§ 17.2); each state card is a deck card like any other.
- **Viewport units.** All heights use `svh` so the address bar collapsing does
  not resize cards mid-scroll; the deck tolerates the resulting one-off jump
  rather than animating it. `transform` and `filter` only on the card element,
  `will-change` set while a card is covered or arriving, cleared otherwise;
  the frame budget of § 17.3 holds. Reduced motion: cards still stack
  (sticky), without the scale, shift, dim or seam glow.
