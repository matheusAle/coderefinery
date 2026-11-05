/**
 * Section rhythm (motion-spec §0c). Every unpinned section is a 100 vh card,
 * sticky inside a 150 vh wrapper (the scroll runway): the card docks after
 * 100 vh of scroll and holds 50 vh before the next card appears. Mount
 * progress m ∈ [0, 1] runs over the whole runway (card top at the viewport
 * bottom → the next card about to arrive); m = 0.40 is the card top at 40 %
 * of the viewport, m = 0.667 the dock. Slots, scrubbed by scroll:
 *   0–0.40 slide only (deck.ts) · kicker fades 0.40–0.43 · headline types
 *   0.40–0.56 · blocks mount in reading order, starts spread evenly over
 *   0.52–0.75 so the last block's support text lands at 0.88 (each block
 *   takes 0.13, overlapping) · captions 0.84–0.88 · status bar 0.88–0.90.
 * Inside a block, text comes before its box: (1) its heading types and its
 * main text reveals (top-down clip, 8 px rise); (2) 0.03 later the container
 * arrives — surface tint and borders fade in, `.box` seams draw, table rules
 * cell by cell, nested boxes one step after their parent (all in by +0.08);
 * (3) at +0.10 the support text (muted labels, meta, sources, footers, exit
 * lines) fades in.
 * Numbers roll after step 3 (`slot(el)`). Scrolling back before m = 0.90
 * reverses; once complete the section stays mounted; outgoing sections never
 * animate. Reduced motion: complete on entering view. Under 1024 px the deck
 * gives the wrapper a 50svh runway under the natural-height card (§17.4), and
 * phones and tablets mount on the §17.2 entrance.
 *
 * Elements declare their slot with `data-slot`:
 *   kicker | title | visual | card | caption | status
 * (visual and card are both blocks; they mount in DOM order). Fallbacks:
 * `.kicker` → kicker, `.h2` → title, `.t2` / `.box` / `.lede` not inside
 * another slot → card, `.statusbar` → status. Inside a block,
 * `data-support` marks support text explicitly; muted (ash) text is support
 * by default.
 *
 * Sections that roll numbers register `onMount(section, (q, slot) => …)`;
 * `slot(el)` is the progress of the element's own number slot.
 */
import { typedTitle, type TypedTitle } from "./typed-title";
import { layoutMode } from "./mode";
import { naturalTop } from "./deck";
import { easeOutCubic, invalidate, onScroll, range, reducedMotion } from "./scroll";

type Kind = "kicker" | "title" | "visual" | "card" | "caption" | "status";
const ORDER: Kind[] = ["kicker", "title", "visual", "card", "caption", "status"];
/** A pinned section's first-state clip (in its entrance e). */
export const CLIP = 0.05;
/** Block timing inside the runway (m). */
const BLOCKS: [number, number] = [0.52, 0.88];
const TEXT = 0.05; // step 1: the clip opens
const BOX_AT = 0.03; // step 2 starts this long after the block
const BOX = 0.04;
const SUPPORT_AT = 0.1; // step 3: ≥ 0.02 after the last surface of the container step is in (QA r6 D3)
const SUPPORT = 0.03;
const ROLL_AT = 0.13; // numbers roll after step 3
const BLOCK_LEN = SUPPORT_AT + SUPPORT;

/** The §0c slots in m. The pinned sections use the same table over their entrance. */
export const SLOTS: Record<"kicker" | "title" | "visual" | "card" | "caption" | "status", [number, number]> = {
  kicker: [0.4, 0.43],
  title: [0.4, 0.56],
  visual: [0.52, 0.64],
  card: [0.56, 0.88],
  caption: [0.84, 0.88],
  status: [0.88, 0.9],
};

/**
 * A pinned section (dive, math explainers) has no runway: its first state assembles over its entrance e. The
 * slide ends at the same physical point (top at 40 % of the viewport, e = 0.60 ↔ m = 0.40) and the content is in
 * by the dock (e = 1 ↔ m = 0.90), so the same SLOTS table applies, compressed.
 */
export function pinnedMount(e: number): number {
  return e <= 0.6 ? (e / 0.6) * 0.4 : 0.4 + ((Math.min(1, e) - 0.6) / 0.4) * 0.5;
}

/**
 * Phone entrance (§17.2): e = 0 with the card top at the viewport bottom, 1 with it at 40 % of the viewport; the
 * §17.2 slots (kicker 0.30–0.33, headline 0.30–0.48, blocks 0.45–0.92, status 0.92–1.00) are mapped onto the m table
 * so the same block engine runs.
 */
const PHONE_MAP: [number, number][] = [
  [0, 0],
  [0.3, 0.4],
  [0.45, 0.52],
  [0.48, 0.56],
  [0.92, 0.88],
  [1, 0.9],
];
export function phoneMount(e: number): number {
  for (let i = 1; i < PHONE_MAP.length; i++) {
    const [e1, m1] = PHONE_MAP[i];
    const [e0, m0] = PHONE_MAP[i - 1];
    if (e <= e1) return m0 + ((e - e0) / (e1 - e0)) * (m1 - m0);
  }
  return 0.9;
}

/** Mount progress m of a runway wrapper: 0 with its top at the viewport bottom, 1 after 150 vh of scroll. */
export function runwayProgress(wrapper: Element): number {
  const vh = window.innerHeight;
  const top = wrapper.getBoundingClientRect().top;
  return Math.max(0, Math.min(1, (vh - top) / (vh * 1.5)));
}

interface Surface {
  el: HTMLElement;
  /** recorded stylesheet colours, restored when the container is fully in */
  bg: string | null;
  borders: [string, string][];
  box: boolean;
  /** 0–1 offset inside the block's container step (cell by cell, nested after outer) */
  lag: number;
  last: number;
}
interface Block {
  surfaces: Surface[];
  support: HTMLElement[];
  headings: TypedTitle[];
  measured: boolean;
}

interface Slot {
  el: HTMLElement;
  kind: Kind;
  start: number;
  end: number;
  typed?: TypedTitle;
  block?: Block;
}

/** `slot(el)`: the element's number slot (after its block's support text); `text(el)`: its block's text step */
type Hook = (q: number, slot: (el: Element) => number, text: (el: Element) => number) => void;
const hooks = new WeakMap<HTMLElement, Hook[]>();

export function onMount(section: HTMLElement, hook: Hook): void {
  const list = hooks.get(section) ?? [];
  list.push(hook);
  hooks.set(section, list);
}

/** Entrance progress of a block: 0 with its top at the viewport bottom, 1 with its top at the viewport top. */
export function entrance(el: Element): number {
  const vh = window.innerHeight;
  const top = el.getBoundingClientRect().top;
  return Math.max(0, Math.min(1, (vh - top) / vh));
}


/**
 * The viewport scale `--u` drives type and spacing: a clamp of the viewport against 1440 × 920 (the reference
 * height carries 20 px of headroom since titles moved to line-height 1.25, §0b, so every card fits 100 vh at
 * 1440 × 900 and 1280 × 720).
 */
export function setViewportScale(): void {
  // 1 at the 1440 × 900 reference (the floors are stated there); the 920 px height headroom only applies below it
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const u = vw >= 1440 && vh >= 900 ? 1 : Math.min(1, Math.max(0.72, Math.min(vh / 920, vw / 1440)));
  document.documentElement.style.setProperty("--u", u.toFixed(3));
  document.documentElement.style.setProperty("--ut", Math.max(0.85, u).toFixed(3));
  // the page-level scale, so a section that needs one type step down (§0c "breathing room") can derive from it
  document.documentElement.style.setProperty("--u0", u.toFixed(3));
  document.documentElement.style.setProperty("--ut0", Math.max(0.85, u).toFixed(3));
}

function collect(section: HTMLElement): Slot[] {
  const explicit = Array.from(section.querySelectorAll<HTMLElement>("[data-slot]"));
  const byKind = new Map<Kind, HTMLElement[]>();
  for (const k of ORDER) byKind.set(k, []);
  const push = (k: Kind, el: HTMLElement) => byKind.get(k)!.push(el);
  for (const el of explicit) push(el.dataset.slot as Kind, el);
  if (byKind.get("kicker")!.length === 0) {
    const k = section.querySelector<HTMLElement>(".kicker");
    if (k && !k.closest("[data-slot]")) push("kicker", k);
  }
  if (byKind.get("title")!.length === 0) {
    const h = section.querySelector<HTMLElement>(".h2");
    if (h && !h.closest("[data-slot]")) push("title", h);
  }
  const explicitSet = new Set(explicit);
  const candidates = Array.from(section.querySelectorAll<HTMLElement>(".t2, .box, .lede"));
  for (const c of candidates) {
    if (explicitSet.has(c)) continue;
    let inside = false;
    let p = c.parentElement;
    while (p && p !== section) {
      if (explicitSet.has(p) || p.classList.contains("t2") || p.classList.contains("box")) {
        inside = true;
        break;
      }
      p = p.parentElement;
    }
    if (!inside) push("card", c);
  }
  if (byKind.get("status")!.length === 0) {
    const s = section.querySelector<HTMLElement>(".statusbar");
    if (s && !s.closest("[data-slot]")) push("status", s);
  }
  const ordered: { el: HTMLElement; kind: Kind }[] = [];
  for (const k of ORDER) {
    const els = byKind.get(k)!;
    els.sort((a, b) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1));
    for (const el of els) ordered.push({ el, kind: k });
  }
  // kicker, title, captions and status take their kind's slot; every block (visual or card) takes the next block
  // start in reading (DOM) order, one every 0.04, squeezed if there are many so the last is in by 0.88
  const blocks = ordered.filter((o) => o.kind === "visual" || o.kind === "card");
  blocks.sort((a, b) => (a.el.compareDocumentPosition(b.el) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1));
  const n = blocks.length;
  // spread evenly over the whole block range: the last block's support text lands at 0.88
  const step = n > 1 ? (BLOCKS[1] - BLOCK_LEN - BLOCKS[0]) / (n - 1) : 0;
  const starts = new Map(blocks.map((o, i) => [o.el, BLOCKS[0] + i * step]));
  return ordered.map((o) => {
    const start = starts.get(o.el);
    if (start !== undefined) return { el: o.el, kind: o.kind, start, end: Math.min(0.9, start + ROLL_AT + 0.1) };
    const [a, b] = SLOTS[o.kind];
    return { el: o.el, kind: o.kind, start: a, end: b };
  });
}

const TRANSPARENT = /^(transparent|rgba\([^)]*,\s*0\))$/;
const MUTED = new Set(["rgb(139, 143, 153)", "rgb(169, 173, 182)"]); // ash, dive-ash

/** Step 2 and 3 targets of a block, read from the stylesheet once (before any inline colour is written). */
function scanBlock(root: HTMLElement): Block {
  const surfaces: Surface[] = [];
  const support: HTMLElement[] = [];
  const all = [root, ...Array.from(root.querySelectorAll<HTMLElement>("*"))].filter((e) => !(e instanceof SVGElement) && !e.closest("svg"));
  for (const el of all) {
    const cs = getComputedStyle(el);
    const bg = TRANSPARENT.test(cs.backgroundColor) ? null : cs.backgroundColor;
    const borders: [string, string][] = [];
    for (const side of ["Top", "Right", "Bottom", "Left"]) {
      const w = parseFloat(cs.getPropertyValue(`border-${side.toLowerCase()}-width`));
      const c = cs.getPropertyValue(`border-${side.toLowerCase()}-color`);
      if (w > 0 && !TRANSPARENT.test(c)) borders.push([`border${side}Color`, c]);
    }
    const box = el.classList.contains("box") && !!el.querySelector(":scope > svg.seam");
    if (bg || borders.length || box) surfaces.push({ el, bg, borders, box, lag: 0, last: -1 });
    // support text: explicitly marked, or muted text with its own words, outside typed titles and live transcripts
    if (el === root || el.closest(".typed-title, .transcript, [data-typer], .odo")) continue;
    const own = Array.from(el.childNodes).some((n) => n.nodeType === 3 && n.textContent!.trim());
    if (el.hasAttribute("data-support") || (own && MUTED.has(cs.color) && !el.parentElement?.closest("[data-support-on]"))) {
      el.setAttribute("data-support-on", "");
      support.push(el);
    }
  }
  // cell by cell: surfaces spread over the container step in DOM order; a nested box one step after its parent
  surfaces.forEach((sf, i) => {
    const depth = sf.box ? Math.max(0, countBoxAncestors(sf.el, root)) : 0;
    sf.lag = Math.min(0.6, (surfaces.length > 1 ? (i / (surfaces.length - 1)) * 0.5 : 0) + depth * 0.25);
  });
  const headings = Array.from(root.querySelectorAll<HTMLElement>("h3, .cta-h")).filter((h) => !h.closest(".typed-title")).map((h) => typedTitle(h));
  return { surfaces, support, headings, measured: true };
}

function countBoxAncestors(el: HTMLElement, root: HTMLElement): number {
  let n = 0;
  let p = el.parentElement;
  while (p && p !== root.parentElement) {
    if (p.classList.contains("box")) n++;
    if (p === root) break;
    p = p.parentElement;
  }
  return n;
}

function paintSurface(sf: Surface, p: number): void {
  if (Math.abs(p - sf.last) < 0.002) return;
  sf.last = p;
  const done = p >= 1;
  const mix = (c: string) => `color-mix(in srgb, ${c} ${(p * 100).toFixed(1)}%, transparent)`;
  if (sf.bg) sf.el.style.backgroundColor = done ? "" : mix(sf.bg);
  for (const [prop, c] of sf.borders) (sf.el.style as unknown as Record<string, string>)[prop] = done ? "" : mix(c);
  if (sf.box) sf.el.style.setProperty("--bx", p.toFixed(3));
}

/** Every runway section, in page order, with a way to complete its mount at once (anchor landings). */
const sections: { wrapper: HTMLElement; complete: () => void }[] = [];

/**
 * A page loaded with a hash, or a jump to an anchor (§0c): the target card and every card above it count as fully
 * mounted (m = 1) at once, so the reader never lands on an unfinished card. Then the target is scrolled into place.
 */
export function mountThrough(target: Element | null, scroll = false): void {
  if (!target) return;
  const main = target.closest("main");
  if (!main) return;
  const cards = Array.from(main.children);
  const card = cards.find((c) => c.contains(target));
  if (!card) return;
  const at = cards.indexOf(card);
  for (const s of sections) if (cards.indexOf(s.wrapper) <= at) s.complete();
  invalidate();
  // the natural position (deck offsets and transforms off): a card above the viewport is stuck or hidden right now
  if (scroll) requestAnimationFrame(() => window.scrollTo({ top: naturalTop(target), behavior: "instant" }));
}

/** Wire anchor landings for a route: the initial hash, hash changes and in-page anchor clicks. */
export function wireAnchors(): void {
  const byHash = (h: string): Element | null => (h.length > 1 ? document.getElementById(decodeURIComponent(h.slice(1))) : null);
  if (location.hash) {
    const go = () => mountThrough(byHash(location.hash), true);
    requestAnimationFrame(go);
    document.fonts?.ready.then(go);
  }
  window.addEventListener("hashchange", () => mountThrough(byHash(location.hash), true));
  document.addEventListener("click", (e) => {
    const a = (e.target as Element | null)?.closest?.('a[href^="#"]') as HTMLAnchorElement | null;
    if (a) mountThrough(byHash(a.getAttribute("href")!));
  });
}

/** Wrap `section` in its 150 vh runway (the 100 vh card sticky inside) and wire its mount. Returns the wrapper. */
export function mountSection(section: HTMLElement): HTMLElement {
  const wrapper = document.createElement("div");
  wrapper.className = "hold";
  const sticky = document.createElement("div");
  sticky.className = "hold-sticky";
  sticky.appendChild(section);
  wrapper.appendChild(sticky);
  section.classList.add("sec-sticky");
  const rm = reducedMotion();
  const narrow = () => window.matchMedia("(max-width: 1023px)").matches;

  const slots = collect(section);
  const index = new Map<Element, Slot>();
  for (const s of slots) {
    index.set(s.el, s);
    s.el.classList.add(`slot-${s.kind}`);
    if (s.kind === "title") s.typed = typedTitle(s.el);
    if (s.kind === "card" || s.kind === "visual") s.el.classList.add("rv");
    if (s.kind === "kicker" || s.kind === "caption" || s.kind === "status") s.el.classList.add("fd");
  }
  const blockSlots = slots.filter((s) => s.kind === "card" || s.kind === "visual");

  // card layout, fixed rows (§0c): the headline's top margin centres the content between the kicker and the status
  // bar; it is measured with every reveal open and frozen, so the headline never moves while blocks expand
  const headline = section.querySelector<HTMLElement>(".h2");
  const measure = (): void => {
    section.style.removeProperty("--mid");
    section.classList.add("measuring");
    for (const s of blockSlots) s.el.style.setProperty("--nh", `${s.el.offsetHeight}px`);
    // the first measure (stylesheet colours, nothing inline yet) records each block's container and support text
    for (const s of blockSlots) if (!s.block) s.block = scanBlock(s.el);
    const mid = headline && !narrow() ? parseFloat(getComputedStyle(headline).marginTop) : NaN;
    section.classList.remove("measuring");
    if (mid > 0) section.style.setProperty("--mid", `${mid.toFixed(1)}px`);
  };

  let released = false;
  let lastQ = -1;
  /** `blockQ`: per-block progress (phone, cards taller than the viewport: each block mounts on its own entrance) */
  const apply = (q: number, blockQ?: Map<Slot, number>): void => {
    if (q === lastQ && !blockQ) return;
    lastQ = q;
    const qOf = (s: Slot): number => blockQ?.get(s) ?? q;
    for (const s0 of slots) {
      const s = s0;
      const q0 = q;
      // eslint-style shadowing kept local: inside a block the block's own progress drives every step
      // (a tall phone card's status bar follows its last block, see phone())
      const qq = s.kind === "card" || s.kind === "visual" || s.kind === "status" ? qOf(s) : q0;
      const p = range(qq, s.start, s.end);
      if (s.typed) s.typed.set(p);
      else if (s.kind === "card" || s.kind === "visual") {
        // (1) headings type and the main text reveals
        const e = easeOutCubic(range(qq, s.start, s.start + TEXT));
        s.el.style.setProperty("--r", e.toFixed(3));
        s.el.classList.toggle("open", e >= 1);
        s.el.classList.toggle("drawn", p > 0);
        s.el.querySelectorAll<HTMLElement>(".t2").forEach((b) => b.classList.toggle("in", p > 0));
        const blk = s.block;
        if (!blk) continue;
        for (const h of blk.headings) h.set(range(qq, s.start, s.start + TEXT * 0.8));
        // (2) the container: surface tint, borders and seams, cell by cell, nested after outer
        const b0 = s.start + BOX_AT;
        for (const sf of blk.surfaces) {
          const a = b0 + sf.lag * BOX;
          paintSurface(sf, range(qq, a, a + BOX * 0.6));
        }
        // (3) the support text
        s.el.style.setProperty("--sup", range(qq, s.start + SUPPORT_AT, s.start + SUPPORT_AT + SUPPORT).toFixed(3));
      } else s.el.style.setProperty("--f", p.toFixed(3));
    }
    const list = hooks.get(section);
    if (list) {
      // a number inside a block rolls only after the block's support text is in
      const slotP = (el: Element): number => {
        const s = index.get(el) ?? index.get(el.closest(".slot-card, .slot-visual, .slot-caption, .slot-status") as Element);
        if (!s) return q;
        const from = s.kind === "card" || s.kind === "visual" ? s.start + ROLL_AT : s.start;
        return range(qOf(s), from, Math.max(s.end, from + 0.05));
      };
      const textP = (el: Element): number => {
        const s = index.get(el) ?? index.get(el.closest(".slot-card, .slot-visual") as Element);
        return s ? range(qOf(s), s.start, s.start + TEXT) : q;
      };
      for (const h of list) h(q, slotP, textP);
    }
  };

  const remeasure = () => {
    measure();
    invalidate();
  };
  document.fonts?.ready.then(remeasure);
  setTimeout(remeasure, 1500);
  requestAnimationFrame(() => {
    measure();
    section.classList.add("mounted");
  });
  window.addEventListener("resize", remeasure);

  sections.push({
    wrapper,
    complete: () => {
      released = true;
    },
  });
  let blockQ: Map<Slot, number> | undefined;
  onScroll(() => {
    // phones and tablets: the §17.2 entrance (the deck's compact geometry, §17.4)
    if (layoutMode() !== "desktop") {
      phone();
      return;
    }
    const m = runwayProgress(wrapper);
    let q = rm ? (m > 0 ? 1 : 0) : m;
    if (q >= 0.9) released = true;
    if (released) q = 1;
    apply(q);
  });

  /**
   * Phone and tablet (§17.2): the card's own entrance e drives the slots (the slide over e 0–0.30 is the deck's, §17.4), and a
   * card taller than the viewport mounts each block on the block's own entrance (its top reaching 85 % of the
   * viewport), and its status bar fades after the last block's support step.
   */
  function phone(): void {
    const vh = window.innerHeight;
    const top = wrapper.getBoundingClientRect().top;
    const e = Math.max(0, Math.min(1, (vh - top) / (0.6 * vh)));
    if (rm || released) {
      apply(1);
      return;
    }
    const q = phoneMount(e);
    const tall = section.offsetHeight > vh * 1.02;
    let done = e >= 1;
    if (tall) {
      blockQ = new Map();
      for (const s of blockSlots) {
        const bt = s.el.getBoundingClientRect().top;
        const eb = Math.max(0, Math.min(1, (vh - bt) / (0.15 * vh)));
        blockQ.set(s, s.start + eb * (s.end - s.start));
        if (eb < 1) done = false;
      }
      // status last (§17.2): the bar fades only after the last block's support step, on that block's own entrance
      const last = blockSlots[blockSlots.length - 1];
      if (last) {
        const lq = blockQ.get(last)!;
        const after = SLOTS.status[0] + (SLOTS.status[1] - SLOTS.status[0]) * range(lq, last.start + BLOCK_LEN, last.end);
        for (const st of slots) if (st.kind === "status") blockQ.set(st, Math.min(q, after));
      }
    } else blockQ = undefined;
    if (done) released = true;
    apply(q, blockQ);
  }
  return wrapper;
}

/** Hero: mounts on load; it still gets the geometry (100 vh, clipped). */
export function plainSection(section: HTMLElement): HTMLElement {
  const wrapper = document.createElement("div");
  wrapper.className = "hold no-hold";
  wrapper.appendChild(section);
  section.classList.add("sec-sticky", "mounted");
  return wrapper;
}
