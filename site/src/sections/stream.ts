/**
 * The hero's event stream (motion-spec §12 P0): an infinite stream of CI
 * events rising from the bottom edge. Pills (glyph · message · PR number) in
 * five tones spawn every 0.35–0.7 s (1 s under 1024 px) at a random x, spread
 * so no two live events that share a row come within 24 px of each other
 * horizontally, and rise at 42 px/s; running
 * events tick their bar and elapsed each second. A fade band keeps full
 * stream opacity (50 %) below the foot line and fades to 0 at the top of the
 * mechanism sub-heading; nothing is drawn above it. Failures (≈ 1 in 8) blink
 * twice on spawn with a soft slag halo for 2 s. Molten sparks (2–3 px) spawn
 * with each event, rise at 1.5× and drift ±6 px, fading over 1.6 s; a failure
 * bursts six. Budget: ≤ 30 events, ≤ 40 sparks, transforms only. The frame
 * cost is accumulated on `window.__streamCost` ({ avg, max } in ms).
 *
 * Placement: events spawn anywhere across the hero's full width and rise
 * through its full height; only the fade band limits them. The band is
 * measured from the live rects (ResizeObserver on the card, lede and foot line,
 * window resize, font load): full
 * opacity from one pill-height below the foot line, 0 once a pill's top edge
 * reaches the bottom of the mechanism sub-heading (so nothing is drawn over
 * it), and a pill is also at 0 while it overlaps the status bar, ramping to
 * full over one pill-height above it. So no pill crosses the foot line or the
 * status bar at full opacity, at any viewport size.
 *
 * Hover (pointer only, touch ignored): the event under the pointer pauses,
 * lifts to full opacity with a molten seam and shows a one-line detail
 * beneath it; its sparks pause with it; a failure stops blinking and shows
 * its reason. On leave it resumes and settles back to band opacity over
 * 300 ms. The install box and the numbers block keep pointer priority. The
 * hit test is arithmetic over ≤ 30 pills on the shared frame, no DOM queries.
 *
 * Parallax: every event gets a depth layer at spawn, near 30 % (scale 1,
 * speed 1×, band opacity 1×), mid 45 % (0.85, 0.7×, 0.7×) or far 25 % (0.7,
 * 0.45×, 0.45×); sparks follow their event's layer; near renders above far.
 * Scroll: as the hero card is covered (or scrolls out, without the deck), near
 * events shift up by 1.0× the distance, mid 0.6×, far 0.3×, so the stream
 * pulls apart. Pointer: the layers shift horizontally by ∓12 / ∓6 / ∓2 px
 * from the viewport's left edge to its right, eased over ≈ 400 ms. Hover
 * applies to near and mid only.
 *
 * Reduced motion: a static arrangement of 20 events, no sparks, no blinks, no
 * parallax shift (layers keep their scale and opacity; hover still shows the
 * detail, without the settle).
 */
import { onFrame } from "../gl/shader";
import { onScroll } from "../motion/scroll";

type Tone = "ingot" | "ember" | "slag" | "seam" | "ash";
interface Vocab {
  glyph: string;
  tone: Tone;
  msg: string;
  /** running events: elapsed seconds (ticks); bar: show ▂▃▅▆ */
  elapsed?: number;
  bar?: boolean;
  agent?: boolean;
  /** failures: the reason shown in the hover detail */
  reason?: string;
}

/**
 * The "blocked" glyph. ⛔ has no text presentation on macOS (Apple Color Emoji ignores U+FE0E), so it rendered
 * as a colour emoji; ⊘ (U+2298) is a plain text glyph that takes the slag tone like ✗.
 */
const STOP = "\u2298";

const VOCAB: Vocab[] = [
  { glyph: "✓", tone: "ingot", msg: "lint · ok 2 s" },
  { glyph: "✓", tone: "ingot", msg: "lint · ok 3 s" },
  { glyph: "✓", tone: "ingot", msg: "build · ok 41 s" },
  { glyph: "✓", tone: "ingot", msg: "build · ok 39 s" },
  { glyph: "✓", tone: "ingot", msg: "tests · 1,204 passed · 4:50" },
  { glyph: "✓", tone: "ingot", msg: "tests · 1,198 passed · 4:46" },
  { glyph: "✓", tone: "ingot", msg: "coderefinery gate · killed 12 · exit 0" },
  { glyph: "✓", tone: "ingot", msg: "coderefinery crap --changed · 0 above 10 · exit 0" },
  { glyph: "✓", tone: "ingot", msg: "coderefinery dry --changed · no duplicate candidates" },
  { glyph: "✓", tone: "ingot", msg: "merged · 4f2c1d9" },
  { glyph: "✓", tone: "ingot", msg: "merged · 9b31e07" },
  { glyph: "▸", tone: "ember", msg: "build · running", elapsed: 22 },
  { glyph: "▸", tone: "ember", msg: "tests · 1,204 · running", elapsed: 192, bar: true },
  { glyph: "▸", tone: "ember", msg: "coderefinery mutate --changed · 12 mutants" },
  { glyph: "▸", tone: "ember", msg: "coderefinery mutate --changed · running", elapsed: 4, bar: true },
  { glyph: "▸", tone: "ember", msg: "coverage · running", elapsed: 11 },
  { glyph: "✗", tone: "slag", msg: "build failed · 12 s", agent: true, reason: "exit 1 · SyntaxError · quotes.py:22" },
  { glyph: "✗", tone: "slag", msg: "tests · 3 failed · 1,201 passed", reason: "exit 1 · 3 failed · test_pricing.py:41, :57, :88" },
  { glyph: "✗", tone: "slag", msg: "DUPLICATE score=0.91 · pricing.py ↔ quotes.py", reason: "exit 2 · score 0.91 · pricing.py:1-6 ↔ quotes.py:8-13" },
  { glyph: STOP, tone: "slag", msg: "coderefinery gate · survived 2 · exit 3", reason: "exit 3 · survived 2 · pricing.py:6, :9" },
  { glyph: STOP, tone: "slag", msg: "CRAP threshold exceeded: 50.5 > 9.9999 · exit 2", reason: "exit 2 · CRAP 50.5 · cc 9 · cov 20 % · pricing.py:1" },
  { glyph: STOP, tone: "slag", msg: "commit blocked by hook · exit 3", reason: "exit 3 · survived 2 · pricing.py:6, :9" },
  { glyph: "○", tone: "seam", msg: "waiting for checks · 14 min" },
  { glyph: "○", tone: "seam", msg: "waiting for checks · 2 h" },
  { glyph: "○", tone: "seam", msg: "queued · 6 min" },
  { glyph: "○", tone: "seam", msg: "queued · 38 min" },
  { glyph: "○", tone: "seam", msg: "review requested · 2 pending" },
  { glyph: "○", tone: "seam", msg: "review requested · 1 pending" },
  { glyph: "↑", tone: "ash", msg: "agent pushed 3 commits" },
  { glyph: "↑", tone: "ash", msg: "agent pushed 1 commit" },
  { glyph: "↑", tone: "ash", msg: "agent opened PR · add quote endpoint" },
  { glyph: "↑", tone: "ash", msg: "agent pushed 2 commits · fix tier rounding" },
];
const FAILS = VOCAB.map((v, i) => (v.tone === "slag" ? i : -1)).filter((i) => i >= 0);
const OTHERS = VOCAB.map((v, i) => (v.tone !== "slag" ? i : -1)).filter((i) => i >= 0);
const BARS = ["▂▃▅▆", "▃▅▆▂", "▅▆▂▃", "▆▂▃▅"];
const mmss = (s: number): string => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

const MAX_EVENTS = 30;
const MAX_SPARKS = 40;
/** phones (§17.2): at most 12 events and 16 sparks */
const PHONE_EVENTS = 12;
const PHONE_SPARKS = 16;
/** below desktop the hero is the phone layout (§17.1): the stream runs in its band */
const phone = (): boolean => window.matchMedia("(max-width: 1023px)").matches;
/** phones, tablets and any touch device (§17.2): at most 12 events and 16 sparks */
const small = (): boolean => phone() || window.matchMedia("(pointer: coarse)").matches;
const maxEvents = (): number => (small() ? PHONE_EVENTS : MAX_EVENTS);
const maxSparks = (): number => (small() ? PHONE_SPARKS : MAX_SPARKS);
const SPEED = 42; // px/s
const SPARK_LIFE = 1.6; // s
/** Horizontal clearance between live events that share vertical space. */
const GAP = 24;
/** Depth layers (§12 P0 "Parallax"): 0 near, 1 mid, 2 far. */
const LAYERS = [
  { share: 0.3, scale: 1, speed: 1, opacity: 1, scroll: 1, shift: 12 },
  { share: 0.45, scale: 0.85, speed: 0.7, opacity: 0.7, scroll: 0.6, shift: 6 },
  { share: 0.25, scale: 0.7, speed: 0.45, opacity: 0.45, scroll: 0.3, shift: 2 },
];
const pickLayer = (): number => {
  const r = Math.random();
  return r < LAYERS[0].share ? 0 : r < LAYERS[0].share + LAYERS[1].share ? 1 : 2;
};
/** Time constant of the pointer parallax: ≈ 95 % of the shift within 400 ms. */
const POINTER_TAU = 0.4 / 3;
const PILL_H = 26;
/** Full stream opacity (§12 P0: 50 %); a hovered event lifts to 1. */
const STREAM_O = 0.5;
/** On leave a hovered event settles back to band opacity over this many seconds. */
const SETTLE = 0.3;

interface Ev {
  el: HTMLElement;
  msg: HTMLElement;
  detail: HTMLElement | null;
  pr: string;
  opened: number;
  checks: number;
  /** logical position (before parallax) and the scaled width */
  x: number;
  y: number;
  w: number;
  layer: number;
  /** detail line size (scaled), read once when first shown */
  dw: number;
  dh: number;
  v: Vocab;
  elapsed: number;
  sparked: boolean;
  fail: boolean;
  hover: boolean;
  /** 1 while hovered, decays to 0 over SETTLE after leave */
  lift: number;
}
interface Spark {
  el: HTMLElement;
  ev: Ev | null;
  layer: number;
  x: number;
  y: number;
  dx: number;
  age: number;
}

/** The band and the hit-test exclusions, all in scope coordinates, re-measured from the live layout. */
interface Band {
  full: number; // pill top at or below this: full stream opacity (one pill-height below the foot line)
  zero: number; // pill top at or above this: nothing drawn (bottom of the mechanism sub-heading)
  status: number; // top of the status bar: a pill overlapping it is at 0, full one pill-height above
  footT: number; // the foot line's rect (the reduced-motion arrangement keeps its rows off it)
  footB: number;
}
interface Rect {
  l: number;
  t: number;
  r: number;
  b: number;
}

const rnd = (a: number, b: number): number => a + Math.random() * (b - a);
const inRect = (x: number, y: number, r: Rect): boolean => x >= r.l && x <= r.r && y >= r.t && y <= r.b;

/**
 * `scope` is the hero's full-width wrapper (not the centred column): the stream
 * covers it edge to edge, measures in its coordinates and takes pointer events
 * from it. The caller inserts the returned element behind the hero.
 */
export function mountStream(
  scope: HTMLElement,
  refs: { foot: HTMLElement; lede: HTMLElement; status: HTMLElement; priority: HTMLElement[] },
  reduced: boolean,
): HTMLElement {
  const host = document.createElement("div");
  host.className = "stream";
  host.setAttribute("aria-hidden", "true");
  const pipes = document.createElement("div");
  pipes.className = "stream-pipes";
  host.appendChild(pipes);
  const cmd = document.createElement("p");
  cmd.className = "stream-cmd mono";
  cmd.textContent = "$ gh pr checks --watch · events rise, fade past the foot line, vanish at the sub-heading · ci minutes this week: 2.1B · agent PRs: 17M";
  host.appendChild(cmd);

  /** no stream at all (a phone whose band is under 120 px) */
  let off = false;
  const band: Band = { full: 650, zero: 485, status: 840, footT: 588, footB: 622 };
  let W = 1440;
  let blocked: Rect[] = [];
  const pipeEls: HTMLElement[] = [];
  for (let i = 0; i < 8; i++) {
    const p = document.createElement("span");
    p.className = "stream-pipe";
    pipes.appendChild(p);
    pipeEls.push(p);
  }
  /** Band opacity (0–1) for a pill whose top edge is at y. */
  const bandAt = (y: number): number => {
    const up = y >= band.full ? 1 : y <= band.zero ? 0 : (y - band.zero) / (band.full - band.zero);
    const down = Math.max(0, Math.min(1, (band.status - (y + PILL_H)) / PILL_H));
    return up * down;
  };
  /** Band opacity for a point (a spark or a 1 px pipe). */
  const bandPoint = (y: number): number => {
    const up = y >= band.full ? 1 : y <= band.zero ? 0 : (y - band.zero) / (band.full - band.zero);
    const down = Math.max(0, Math.min(1, (band.status - y) / PILL_H));
    return up * down;
  };

  function measure(): void {
    const hr = scope.getBoundingClientRect();
    W = hr.width;
    const rel = (el: HTMLElement): Rect => {
      const r = el.getBoundingClientRect();
      return { l: r.left - hr.left, t: r.top - hr.top, r: r.right - hr.left, b: r.bottom - hr.top };
    };
    band.zero = rel(refs.lede).b;
    const foot = rel(refs.foot);
    band.footT = foot.t;
    band.footB = foot.b;
    band.full = Math.max(band.zero + 1, foot.b + PILL_H);
    band.status = rel(refs.status).t;
    blocked = refs.priority.map(rel);
    if (phone()) {
      // phones (§17.2): the stream runs only in the empty band between the install box and the numbers, never
      // behind text: fade in over one pill under the install box, gone at the numbers' top; no band, no stream
      const install = blocked[0];
      const numbers = blocked[1];
      band.zero = install.b + 6;
      band.full = band.zero + PILL_H;
      band.status = numbers.t - 6;
      band.footT = band.footB = -1;
      off = band.status - band.zero < 120;
      host.style.display = off ? "none" : "";
      host.style.clipPath = `inset(${band.zero.toFixed(0)}px 0 calc(100% - ${band.status.toFixed(0)}px) 0)`;
    } else {
      off = false;
      host.style.display = "";
      host.style.clipPath = "";
    }
    pipeEls.forEach((p, i) => {
      const y = band.zero + ((i + 0.5) / pipeEls.length) * (band.status - band.zero);
      p.style.transform = `translate3d(0, ${y.toFixed(1)}px, 0)`;
      p.style.opacity = (STREAM_O * bandPoint(y)).toFixed(3);
    });
  }
  requestAnimationFrame(measure);
  const ro = new ResizeObserver(() => measure());
  ro.observe(scope);
  ro.observe(refs.lede);
  ro.observe(refs.foot);
  window.addEventListener("resize", measure);
  document.fonts?.ready.then(measure);

  // ---- pools ----
  const events: Ev[] = [];
  const sparks: Spark[] = [];
  let spawnCount = 0;
  let prBase = 468;
  const nextPr = (): string => {
    // PR numbers increment over time: a window of four around a slowly rising base
    prBase = 468 + Math.floor(spawnCount / 5);
    return `PR #${prBase - Math.floor(Math.random() * 4)}`;
  };

  function makePill(v: Vocab, pr: string, layer: number): { el: HTMLElement; msg: HTMLElement } {
    const el = document.createElement("div");
    el.className = `ev ev-${v.tone} ev-l${layer}`;
    const g = document.createElement("span");
    g.className = "ev-g";
    g.textContent = v.glyph;
    const m = document.createElement("span");
    m.className = "ev-m";
    m.textContent = v.msg;
    const p = document.createElement("span");
    p.className = "ev-pr";
    p.textContent = v.agent ? `${pr} · agent` : pr;
    el.append(g, m, p);
    return { el, msg: m };
  }

  function newEv(v: Vocab, pr: string, x: number, y: number, el: HTMLElement, msg: HTMLElement, w: number, layer: number): Ev {
    return {
      el,
      msg,
      detail: null,
      pr,
      opened: 2 + Math.floor(Math.random() * 40),
      checks: 2 + Math.floor(Math.random() * 4),
      x,
      y,
      w,
      layer,
      dw: 0,
      dh: 0,
      v,
      elapsed: v.elapsed ?? 0,
      sparked: false,
      fail: v.tone === "slag",
      hover: false,
      lift: 0,
    };
  }

  /**
   * A spawn x anywhere across the full width that keeps GAP px of horizontal
   * clearance from every live event of its layer sharing vertical space with it;
   * null when no candidate fits (the spawn waits a frame for the row to clear).
   */
  function pickX(w: number, y: number, layer: number): number | null {
    const span = Math.max(0, W - w);
    for (let k = 0; k < 12; k++) {
      const x = rnd(0, span);
      let ok = true;
      for (const ev of events) {
        // the rule binds within a depth layer: one layer moves as a sheet, so spacing at spawn holds for life;
        // across layers, overlap is the depth (near passes over far)
        if (ev.layer !== layer || Math.abs(ev.y - y) >= PILL_H + GAP) continue;
        if (x < ev.x + ev.w + GAP && x + w + GAP > ev.x) {
          ok = false;
          break;
        }
      }
      if (ok) return x;
    }
    return null;
  }

  function spawnEvent(y: number): Ev | null {
    if (events.length >= maxEvents()) return null;
    const fail = Math.random() < 1 / 8;
    const v = VOCAB[fail ? FAILS[Math.floor(Math.random() * FAILS.length)] : OTHERS[Math.floor(Math.random() * OTHERS.length)]];
    const pr = nextPr();
    const layer = pickLayer();
    const pill = makePill(v, pr, layer);
    // running events carry their bar and elapsed from the start, so the measured width is the width they keep
    if (v.elapsed !== undefined) pill.msg.textContent = `${v.msg} ${v.bar ? BARS[0] + " " : ""}${mmss(v.elapsed)}`;
    pill.el.style.opacity = "0";
    host.appendChild(pill.el);
    const w = (pill.el.offsetWidth || 260) * LAYERS[layer].scale;
    // anywhere across the hero's full width (§12 P0 "Placement"), clear of its neighbours
    const x = pickX(w, y, layer);
    if (x === null) {
      pill.el.remove();
      return null;
    }
    spawnCount++;
    const ev = newEv(v, pr, x, y, pill.el, pill.msg, w, layer);
    pill.el.style.transform = `translate3d(${x.toFixed(1)}px, ${y.toFixed(1)}px, 0) scale(${LAYERS[layer].scale})`;
    events.push(ev);
    return ev;
  }

  function spawnSpark(ev: Ev | null, x: number, y: number): void {
    if (sparks.length >= maxSparks()) return;
    const layer = ev?.layer ?? 0;
    const el = document.createElement("span");
    el.className = `spark ev-l${layer}`;
    const size = Math.random() < 0.4 ? 3 : 2;
    el.style.width = `${size}px`;
    el.style.height = `${size}px`;
    el.style.opacity = "0";
    host.appendChild(el);
    sparks.push({ el, ev, layer, x, y, dx: rnd(-6, 6), age: 0 });
  }

  function tickText(ev: Ev, tick: number): void {
    if (ev.v.elapsed === undefined) return;
    ev.msg.textContent = `${ev.v.msg} ${ev.v.bar ? BARS[tick % 4] + " " : ""}${mmss(ev.elapsed)}`;
  }

  // ---- parallax (§12 P0): scroll distance as the hero card is covered, eased pointer x ----
  let scrollD = 0;
  let ptrTarget = 0; // −1 at the viewport's left edge, +1 at its right
  let ptrNow = 0;
  const shiftX = [0, 0, 0];
  const vx = (ev: { x: number; layer: number }): number => ev.x + shiftX[ev.layer];
  const vy = (ev: { y: number; layer: number }): number => ev.y - scrollD * LAYERS[ev.layer].scroll;
  if (!reduced) {
    onScroll(() => {
      // without the deck the hero scrolls out (its top goes negative); in the deck it stays and the next card covers it
      const vh = window.innerHeight;
      const top = scope.getBoundingClientRect().top;
      const next = scope.nextElementSibling;
      const nextIn = next ? Math.max(0, Math.min(vh, vh - next.getBoundingClientRect().top)) : 0;
      scrollD = Math.max(0, -top, nextIn);
    });
    window.addEventListener(
      "pointermove",
      (e) => {
        if (e.pointerType === "touch") return;
        ptrTarget = (e.clientX / Math.max(1, window.innerWidth)) * 2 - 1;
      },
      { passive: true },
    );
  }

  // ---- hover (pointer only; touch ignores it) ----
  let px = 0;
  let py = 0;
  let pointer = false;
  let hovered: Ev | null = null;

  function detailText(ev: Ev): string {
    if (ev.fail && ev.v.reason) return `${ev.pr} · ${ev.v.reason}`;
    return `${ev.pr} · opened ${ev.opened} min ago · ${ev.checks} checks · 1,204 tests`;
  }

  function setHover(ev: Ev | null): void {
    if (ev === hovered) return;
    if (hovered) {
      hovered.hover = false;
      hovered.el.classList.remove("hov");
    }
    hovered = ev;
    if (!ev) return;
    ev.hover = true;
    ev.lift = 1;
    if (!ev.detail) {
      const d = document.createElement("span");
      d.className = "ev-d";
      d.textContent = detailText(ev);
      ev.el.appendChild(d);
      ev.detail = d;
    }
    ev.el.classList.add("hov");
    ev.el.style.opacity = "1";
    if (!ev.dw) {
      const sc = LAYERS[ev.layer].scale;
      ev.dw = ev.detail.offsetWidth * sc;
      ev.dh = (ev.detail.offsetHeight + 4) * sc;
    }
  }

  /** Topmost visible event under the pointer (scope coordinates), unless the install box or the numbers block holds it. */
  function hitTest(): Ev | null {
    if (!pointer) return null;
    for (const r of blocked) if (inRect(px, py, r)) return null;
    // near renders above mid above far: test near first, then mid; far is never hoverable
    for (let layer = 0; layer < 2; layer++) {
      for (let i = events.length - 1; i >= 0; i--) {
        const ev = events[i];
        if (ev.layer !== layer) continue;
        const x = vx(ev);
        const y = vy(ev);
        if (bandAt(y) < 0.05 && !ev.hover) continue;
        const sc = LAYERS[ev.layer].scale;
        const bottom = y + PILL_H * sc + (ev.hover ? ev.dh : 0);
        const right = x + Math.max(ev.w, ev.hover ? ev.dw : 0);
        if (px >= x && px <= right && py >= y && py <= bottom) return ev;
      }
    }
    return null;
  }

  let clientX = 0;
  let clientY = 0;
  const onPointer = (e: PointerEvent): void => {
    if (e.pointerType === "touch") return;
    pointer = true;
    clientX = e.clientX;
    clientY = e.clientY;
    if (reduced) hoverStatic();
  };
  // touch (§17.2): no hover; a tap on an event shows its detail line for two seconds
  let tapTimer = 0;
  scope.addEventListener(
    "pointerup",
    (e) => {
      if (e.pointerType !== "touch") return;
      clientX = e.clientX;
      clientY = e.clientY;
      pointer = true;
      localPointer();
      const hit = hitTest();
      pointer = false;
      window.clearTimeout(tapTimer);
      tapTimer = 0;
      setHover(hit);
      if (reduced && hit) hit.el.style.opacity = "1";
      if (hit) tapTimer = window.setTimeout(() => {
        tapTimer = 0;
        setHover(null);
        if (reduced) hit.el.style.opacity = (STREAM_O * LAYERS[hit.layer].opacity * bandAt(hit.y)).toFixed(3);
      }, 2000);
    },
    { passive: true },
  );
  scope.addEventListener("pointermove", onPointer, { passive: true });
  scope.addEventListener("pointerdown", onPointer, { passive: true });
  scope.addEventListener("pointerleave", () => {
    pointer = false;
    if (reduced) hoverStatic();
  });
  /** Pointer in scope coordinates; one rect read, taken before any style write of the frame. */
  const localPointer = (): void => {
    const hr = scope.getBoundingClientRect();
    px = clientX - hr.left;
    py = clientY - hr.top;
  };

  function hoverStatic(): void {
    if (pointer) localPointer();
    const prev = hovered;
    setHover(hitTest());
    if (prev && prev !== hovered) prev.el.style.opacity = (STREAM_O * LAYERS[prev.layer].opacity * bandAt(prev.y)).toFixed(3);
  }

  if (reduced) {
    // static arrangement: 20 events across the band in rows, clear of each other, no sparks, no blinks
    requestAnimationFrame(() => {
      measure();
      // rows one pill + GAP apart (so the clearance rule only binds within a row), from just under the lede down to
      // the status bar, skipping any row that would cross the foot line; the lower rows fill first
      const pitch = PILL_H + GAP;
      const rowYs: number[] = [];
      for (let y = band.status - PILL_H * 1.5; y > band.zero + 4; y -= pitch) {
        if (y < band.footB + 2 && y + PILL_H > band.footT - 2) y = band.footT - 2 - PILL_H;
        if (y > band.zero + 4) rowYs.push(y);
      }
      const rows = Math.max(1, rowYs.length);
      let placed = 0;
      for (let i = 0; placed < 20 && i < 160; i++) {
        const v = VOCAB[(i * 7) % VOCAB.length];
        const pr = `PR #${468 + (i % 5) * 4}`;
        const layer = i % 20 < 6 ? 0 : i % 20 < 15 ? 1 : 2;
        const L = LAYERS[layer];
        const pill = makePill(v, pr, layer);
        if (v.elapsed !== undefined) pill.msg.textContent = `${v.msg} ${v.bar ? "▂▃▅▆ " : ""}${mmss(v.elapsed)}`;
        host.appendChild(pill.el);
        const w = (pill.el.offsetWidth || 260) * L.scale;
        const y = rowYs[i % rows] ?? band.full;
        const x = pickX(w, y, layer);
        if (x === null) {
          pill.el.remove();
          continue;
        }
        pill.el.style.transform = `translate3d(${x.toFixed(1)}px, ${y.toFixed(1)}px, 0) scale(${L.scale})`;
        pill.el.style.opacity = (STREAM_O * L.opacity * bandAt(y)).toFixed(3);
        events.push(newEv(v, pr, x, y, pill.el, pill.msg, w, layer));
        placed++;
      }
    });
    return host;
  }

  // ---- the clock ----
  const narrow = () => window.matchMedia("(max-width: 1023px)").matches;
  let last = -1;
  let nextSpawn = 0.4;
  let lastTick = 0;
  let tick = 0;
  /** per-frame JS cost in ms: running sum / count / max / avg, plus the last 600 frames for percentiles */
  const cost = { sum: 0, n: 0, max: 0, avg: 0, recent: new Float32Array(600) };
  (window as unknown as { __streamCost: typeof cost }).__streamCost = cost;
  let visible = true;
  new IntersectionObserver((e) => (visible = e[0].isIntersecting), { threshold: 0 }).observe(scope);

  let seeded = 0;
  onFrame((t) => {
    if (last < 0) last = t;
    const dt = Math.min(0.1, t - last);
    last = t;
    // off-screen, or the hero card fully covered by the next card (deck): nothing to draw
    if (!visible || scope.style.visibility === "hidden") return;
    const t0 = performance.now();
    ptrNow += (ptrTarget - ptrNow) * (1 - Math.exp(-dt / POINTER_TAU));
    for (let l = 0; l < 3; l++) shiftX[l] = ptrNow * LAYERS[l].shift;
    if (seeded < 16) {
      // pre-seed: sixteen events already in flight, spread over the band, three per frame so no frame pays for
      // more than three pill measurements
      if (seeded === 0) measure();
      for (let k = 0; k < 3 && seeded < 16; k++, seeded++) spawnEvent(band.zero + ((seeded + 0.5) / 16) * (band.status - band.zero));
    }

    if (off) return;
    if (t >= nextSpawn && events.length < maxEvents()) {
      // spawn at the bottom edge of the content (the status bar's top), where the pill is still at 0 and rises into
      // view; if no x keeps the 24 px clearance this frame, try again next frame
      if (spawnEvent(band.status)) nextSpawn = t + (narrow() ? 1 : rnd(0.35, 0.7));
    }
    if (t - lastTick >= 1) {
      lastTick = t;
      tick++;
      for (const ev of events) {
        if (ev.v.elapsed === undefined) continue;
        ev.elapsed++;
        tickText(ev, tick);
      }
    }
    if (pointer) localPointer();
    if (!tapTimer || pointer) setHover(hitTest());

    // events rise at a constant speed unless hovered; opacity follows the band; gone above it
    for (let i = events.length - 1; i >= 0; i--) {
      const ev = events[i];
      const L = LAYERS[ev.layer];
      if (!ev.hover) {
        ev.y -= SPEED * L.speed * dt;
        if (ev.lift > 0) ev.lift = Math.max(0, ev.lift - dt / SETTLE);
      }
      const x = vx(ev);
      const y = vy(ev);
      const b = bandAt(y);
      if (ev.y <= band.zero && !ev.hover && ev.lift <= 0) {
        ev.el.remove();
        events.splice(i, 1);
        continue;
      }
      if (!ev.sparked && b > 0) {
        // the event "spawns" visibly as it rises into view: sparks (1–3, a burst of 6 on a failure)
        ev.sparked = true;
        // a failure blinks twice as it appears and carries a soft slag halo for 2 s (CSS); hover cancels both
        if (ev.fail) ev.el.classList.add("fail");
        const n = ev.fail ? 6 : 1 + Math.floor(Math.random() * 3);
        for (let k = 0; k < n; k++) spawnSpark(ev, ev.x + rnd(0, ev.w), ev.y + PILL_H * L.scale - rnd(0, 10));
      }
      // a hovered event is paused in y; the parallax still carries it (it is part of its layer)
      ev.el.style.transform = `translate3d(${x.toFixed(1)}px, ${y.toFixed(1)}px, 0) scale(${L.scale})`;
      if (ev.hover) continue; // at full opacity
      const base = STREAM_O * L.opacity * b;
      ev.el.style.opacity = (base + (1 - base) * ev.lift).toFixed(3);
    }
    for (let i = sparks.length - 1; i >= 0; i--) {
      const s = sparks[i];
      if (s.ev?.hover) continue; // a hovered event's sparks pause with it
      const L = LAYERS[s.layer];
      s.age += dt;
      s.y -= SPEED * 1.5 * L.speed * dt;
      const sy = vy(s);
      const o = s.age > SPARK_LIFE ? 0 : (1 - s.age / SPARK_LIFE) * L.opacity * bandPoint(sy);
      if (o <= 0 && s.age > 0.2) {
        s.el.remove();
        sparks.splice(i, 1);
        continue;
      }
      const x = vx(s) + s.dx * (s.age / SPARK_LIFE);
      s.el.style.transform = `translate3d(${x.toFixed(1)}px, ${sy.toFixed(1)}px, 0) scale(${L.scale})`;
      s.el.style.opacity = (STREAM_O * o).toFixed(3);
    }

    const c = performance.now() - t0;
    cost.sum += c;
    cost.n++;
    cost.max = Math.max(cost.max, c);
    cost.avg = cost.sum / cost.n;
    cost.recent[cost.n % 600] = c;
  });

  return host;
}
