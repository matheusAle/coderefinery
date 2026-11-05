/**
 * Deck stacking (motion-spec §0c "Deck stacking", §17.4 "The deck on a phone"): sections stack like cards instead
 * of scrolling past each other. One module, two geometries:
 *
 * - desktop (≥ 1024 px): every top-level card of the route is `position: sticky` with
 *   `top = min(0, 100vh − its height)`, so a 100 vh card sticks as its top docks and a pinned card (the dive, an
 *   explainer) sticks once its own pin has run out; runway wrappers (150 vh) hold their 100 vh face sticky inside.
 * - compact (phones and tablets, < 1024 px): cards are natural height (at least 100svh). Every card is sticky in a
 *   wrapper whose height is the card's plus a 50svh runway (CSS padding); the card sticks at
 *   `top = min(0, 100svh − cardHeight)`, so a tall card is read through and pins by its bottom edge, and the wrapper
 *   sticks at `min(0, 100svh − wrapperHeight)` so the held card stays while the next one slides over. The dive and
 *   the math explainers are groups of stacked state cards; each state is a deck card, sticky inside its group, and
 *   the group itself sticks by its bottom edge so its last state can be covered like any card. The route's last
 *   card has no runway and is not sticky (the footer scrolls in under it).
 *
 * Over the incoming card's slide (its top from the viewport bottom to the top) the covered card's face scales
 * 1.00 → 0.96 about its top-centre (compact: the top of what is on screen, since a tall card is pinned by its
 * bottom), rises 24 px (compact 16 px) and dims to 70 % (a black overlay at up to 30 %); its fields pause
 * (`data-covered`). Fully covered, it is `visibility: hidden` and inert until the reader scrolls back. Only the
 * card directly under the incoming one is ever partly visible. While a card arrives it plays its own slide: scale
 * 0.97 → 1.00 about its top-centre, brightness 0.55 → 1.0 (the same overlay at 45 % → 0) and its top seam cooling
 * from molten at 60 % (`--seam-glow` 1 → 0), over its top travelling from the viewport bottom to the 40 % mark
 * (desktop, e 0–0.60) or over e 0–0.30 of the §17.2 entrance (compact: the top from the viewport bottom to 82 %).
 * Transforms go on the face only, with `will-change` only while covered or arriving. Reduced motion: sticky
 * stacking only, no scale, shift, dim or seam glow. The hero (card 0) never arrives.
 *
 * The discreet slag field is handed to the face of the active card (the incoming one while a card slides, else the
 * top one) so it is clipped to its card; in compact geometry it is one viewport tall and follows the visible part
 * of a tall card.
 */
import { layoutMode } from "./mode";
import { clamp01, invalidate, onScroll, reducedMotion } from "./scroll";

export interface FieldHost {
  /** `face`: where the field lives (null: nowhere visible, it pauses). */
  attach(face: HTMLElement | null): void;
}

type Geometry = "desktop" | "compact";

interface Unit {
  /** sticky in its parent (main, or a group): the runway wrapper, the hero, a pinned section, a state's hold */
  el: HTMLElement;
  /** what is on screen while the card is stuck: transformed, dimmed, holds the field */
  face: HTMLElement;
  dim: HTMLElement;
  /** may hold the discreet field (no own field, not the furnace dive) */
  field: boolean;
  last: boolean;
  /** compact: the face's offset of its visible top while pinned by its bottom (the covered transform's origin) */
  originY: number;
  state: string;
}

interface Geo {
  scale: number;
  rise: number;
  /** the arrival runs while the card's top travels this fraction of the viewport from its bottom edge */
  slide: number;
}
const GEO: Record<Geometry, Geo> = {
  // §0c: covered 1 → 0.96, up 24 px; the slide over e 0–0.60 of a 100 vh entrance
  desktop: { scale: 0.04, rise: 24, slide: 0.6 },
  // §17.4: covered 1 → 0.96, up 16 px; the slide over e 0–0.30 of the §17.2 entrance (0 → 1 over 60 % of the viewport)
  compact: { scale: 0.04, rise: 16, slide: 0.3 * 0.6 },
};
const DIM = 0.3;
const ARRIVE_SCALE = 0.03; // 0.97 → 1.00
const ARRIVE_DIM = 0.45; // brightness 0.55 → 1.0

/** Classes the deck puts on the page, removed when the geometry changes. */
const UNIT_CLASSES = ["card", "deck-card", "card-last", "deck-seam"];

/**
 * The document position of `el` with the deck's sticky offsets and transforms switched off: where an anchor should
 * land, whatever the cards above or around it are doing right now.
 */
export function naturalTop(el: Element): number {
  const main = el.closest("main");
  main?.classList.add("deck-measure");
  const y = el.getBoundingClientRect().top + window.scrollY;
  main?.classList.remove("deck-measure");
  return y;
}

export function mountDeck(main: HTMLElement, field: FieldHost | null = null): void {
  const rm = reducedMotion();
  // 100svh in px: compact heights are in svh so a collapsing address bar does not move the geometry
  const probe = document.createElement("div");
  probe.setAttribute("aria-hidden", "true");
  probe.style.cssText = "position:fixed;top:0;left:0;width:0;height:100svh;visibility:hidden;pointer-events:none";
  document.body.appendChild(probe);

  let geo: Geometry = "desktop";
  let units: Unit[] = [];
  let groups: HTMLElement[] = [];
  const ro = new ResizeObserver(() => layout());

  const dimOf = (face: HTMLElement): HTMLElement => {
    let dim = face.querySelector<HTMLElement>(":scope > .deck-dim");
    if (!dim) {
      dim = document.createElement("div");
      dim.className = "deck-dim";
      dim.setAttribute("aria-hidden", "true");
      face.appendChild(dim);
    }
    return dim;
  };
  const unit = (el: HTMLElement, face: HTMLElement, field: boolean): Unit => ({
    el,
    face,
    dim: dimOf(face),
    field,
    last: false,
    originY: 0,
    state: "",
  });

  /** The cards of the route in page order, for the geometry. */
  const collect = (g: Geometry): { units: Unit[]; groups: HTMLElement[] } => {
    const tops = Array.from(main.children).filter((c): c is HTMLElement => c instanceof HTMLElement && c.tagName !== "FOOTER");
    const list: Unit[] = [];
    const grp: HTMLElement[] = [];
    for (const el of tops) {
      const furnace = el.classList.contains("dive");
      const holds = g === "compact" ? Array.from(el.querySelectorAll<HTMLElement>(":scope > .deck-hold")) : [];
      if (holds.length) {
        // a stacked pinned section: each state is a card, the section only groups them
        grp.push(el);
        for (const h of holds) list.push(unit(h, (h.firstElementChild as HTMLElement) ?? h, !furnace));
        continue;
      }
      const face = el.querySelector<HTMLElement>(":scope > .dive-sticky, :scope > .ex-sticky, :scope > .hold-sticky") ?? el;
      list.push(unit(el, face, !el.querySelector(":scope > .gl-host") && !furnace));
    }
    if (list.length) list[list.length - 1].last = true;
    return { units: list, groups: grp };
  };

  const reset = (u: Unit): void => {
    if (u.state === "") return;
    u.state = "";
    u.face.style.transform = "";
    u.face.style.transformOrigin = "";
    u.face.style.willChange = "";
    u.dim.style.opacity = "";
    u.el.style.visibility = "";
    u.el.inert = false;
    u.el.style.removeProperty("--seam-glow");
    delete u.el.dataset.covered;
    delete u.el.dataset.hidden;
  };

  /** Strip everything the deck wrote (geometry change, or the page rebuilt a pinned section). */
  const teardown = (): void => {
    for (const u of units) {
      u.state = "x";
      reset(u);
      u.el.classList.remove(...UNIT_CLASSES);
      u.face.classList.remove("card-face");
      u.el.style.top = "";
      u.face.style.top = "";
      u.face.querySelector(":scope > canvas.gl-freeze")?.remove();
    }
    for (const g of groups) {
      g.classList.remove("deck-group");
      g.style.top = "";
      delete g.dataset.hidden;
    }
    ro.disconnect();
  };

  function layout(): void {
    const g: Geometry = layoutMode() === "desktop" ? "desktop" : "compact";
    const next = collect(g);
    const same = g === geo && units.length > 0 && next.units.length === units.length && next.units.every((u, i) => u.el === units[i].el && u.face === units[i].face);
    if (!same) {
      teardown();
      geo = g;
      units = next.units;
      groups = next.groups;
      main.classList.toggle("deck", g === "desktop");
      main.classList.toggle("deck-compact", g === "compact");
      units.forEach((u, i) => {
        u.el.classList.add(g === "desktop" ? "card" : "deck-card");
        u.face.classList.add("card-face");
        if (u.last) u.el.classList.add("card-last");
        if (i > 0) u.el.classList.add("deck-seam");
        ro.observe(u.el);
        if (u.face !== u.el) ro.observe(u.face);
      });
      for (const grp of groups) {
        grp.classList.add("deck-group");
        ro.observe(grp);
      }
    }
    // reads, then writes
    const vh = g === "compact" ? probe.offsetHeight || window.innerHeight : window.innerHeight;
    const hs = units.map((u) => [u.el.offsetHeight, u.face.offsetHeight]);
    const gh = groups.map((x) => x.offsetHeight);
    const footer = main.querySelector<HTMLElement>(":scope > footer");
    const fh = g === "desktop" && footer ? footer.offsetHeight : 0;
    units.forEach((u, i) => {
      const [h, fh2] = hs[i];
      // sticky cards stick at min(0, 100vh − height); the last card is position: relative, where a `top` would shift
      // it up and uncover the cards below (the bottom-of-page leak)
      u.el.style.top = u.last ? "" : `${Math.min(0, vh - h)}px`;
      // compact: the card inside its runway wrapper sticks by the edge that fits (§17.4)
      if (g === "compact" && u.face !== u.el) u.face.style.top = u.last ? "" : `${Math.min(0, vh - fh2)}px`;
      u.originY = g === "compact" ? Math.max(0, fh2 - vh) : 0;
    });
    groups.forEach((x, i) => (x.style.top = `${Math.min(0, vh - gh[i])}px`));
    // desktop: the last card is 100 vh minus the footer, so card and footer fill the viewport exactly at the bottom
    main.style.setProperty("--footer-h", `${fh}px`);
    invalidate();
  }
  layout();
  window.addEventListener("resize", layout);
  // card heights change once the fonts are in (and when a pinned section rebuilds: the ResizeObserver)
  document.fonts?.ready.then(layout);

  /** The incoming card's arrival at k ∈ [0, 1] (k = 0 is also the waiting state below the viewport). */
  const arrive = (u: Unit, k: number): void => {
    u.state = k <= 0 ? "waiting" : "arriving";
    u.el.style.visibility = "";
    u.el.inert = false;
    delete u.el.dataset.covered;
    delete u.el.dataset.hidden;
    u.face.style.transformOrigin = "";
    u.face.style.willChange = k > 0 ? "transform" : "";
    u.face.style.transform = `scale(${(1 - ARRIVE_SCALE * (1 - k)).toFixed(4)})`;
    u.dim.style.opacity = (ARRIVE_DIM * (1 - k)).toFixed(3);
    u.el.style.setProperty("--seam-glow", (1 - k).toFixed(3));
  };

  /** compact: the field is one viewport tall and follows the visible part of a tall card (desktop: no offset) */
  let fieldHost: HTMLElement | null = null;
  let fieldY = -1;
  const placeField = (u: Unit | undefined): void => {
    fieldHost ??= document.querySelector<HTMLElement>(".gl-field");
    if (!fieldHost) return;
    let y = 0;
    if (geo === "compact" && u && u.field) {
      const top = u.face.getBoundingClientRect().top;
      const vh = probe.offsetHeight || window.innerHeight;
      y = Math.round(Math.max(0, Math.min(-top, u.face.offsetHeight - vh)));
    }
    if (y === fieldY) return;
    fieldY = y;
    fieldHost.style.transform = y ? `translate3d(0, ${y}px, 0)` : "";
  };

  onScroll(() => {
    const G = GEO[geo];
    const vh = window.innerHeight;
    // reads first, then writes
    const tops = units.map((u) => u.el.getBoundingClientRect().top);
    let active = 0;
    for (let j = 0; j < units.length; j++) if (tops[j] < vh - 0.5) active = j;
    for (let i = 0; i < units.length; i++) {
      const u = units[i];
      // covered is a state of the scroll position: the furthest any later card has come in (it survives the end of
      // the document, where later cards are docked or already scrolled past)
      let next = 0;
      // (a later card whose top is within half a pixel of the viewport top covers it: sub-pixel tops in groups)
      for (let j = i + 1; j < units.length; j++) next = Math.max(next, tops[j] <= 0.5 ? 1 : clamp01((vh - tops[j]) / vh));
      if (next <= 0) {
        // not covered: arriving (its own slide), waiting below the viewport, or docked
        const k = clamp01((vh - tops[i]) / (vh * G.slide));
        if (i > 0 && !rm && k < 1) {
          if (k <= 0) {
            if (u.state !== "waiting") arrive(u, 0);
          } else arrive(u, k);
        } else reset(u);
        continue;
      }
      if (!u.el.dataset.covered) {
        u.el.dataset.covered = "1";
        // a sheet opened from this card (the phone menu) closes as the card goes under
        u.el.dispatchEvent(new CustomEvent("deck:cover", { bubbles: true }));
      }
      if (next >= 1) {
        if (u.state !== "hidden") {
          u.state = "hidden";
          u.el.style.visibility = "hidden";
          u.el.inert = true;
          u.face.style.willChange = "";
          // descendants with their own `visibility: visible` (active sections, hints) are forced hidden too (CSS)
          u.el.dataset.hidden = "1";
          // a fully covered card drops its frozen field frame (only the card under the incoming one keeps one)
          u.face.querySelector(":scope > canvas.gl-freeze")?.remove();
        }
        continue;
      }
      u.state = "covering";
      u.el.style.visibility = "";
      u.el.inert = false;
      delete u.el.dataset.hidden;
      if (!rm) {
        u.face.style.willChange = "transform";
        u.face.style.transformOrigin = u.originY ? `50% ${u.originY}px` : "";
        u.face.style.transform = `translate3d(0, ${(-G.rise * next).toFixed(2)}px, 0) scale(${(1 - G.scale * next).toFixed(4)})`;
        u.dim.style.opacity = (DIM * next).toFixed(3);
      }
    }
    // a group whose every state is fully covered renders nothing either (its sticky depth bar)
    for (const grp of groups) {
      const all = units.every((u) => !grp.contains(u.el) || u.state === "hidden");
      if (all) grp.dataset.hidden = "1";
      else delete grp.dataset.hidden;
    }
    const a = units[active];
    field?.attach(a && a.field ? a.face : null);
    placeField(a);
  });
}
