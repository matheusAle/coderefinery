/**
 * Layout modes (motion-spec §17): desktop ≥ 1024 px (the deck, runways, pins), tablet 768–1023 px (the stacked,
 * natural-height layout with desktop type), phone ≤ 767 px (§17.1–17.2). Phones and tablets share the deck's
 * compact geometry (§17.4) and the §17.2 entrance.
 * `pointer: coarse` turns on the touch rules at any width. The current mode is mirrored on <html data-mode>.
 */
export type Mode = "desktop" | "tablet" | "phone";

export const PHONE = "(max-width: 767px)";
export const NARROW = "(max-width: 1023px)";
export const COARSE = "(pointer: coarse)";

export function layoutMode(): Mode {
  if (window.matchMedia(PHONE).matches) return "phone";
  if (window.matchMedia(NARROW).matches) return "tablet";
  return "desktop";
}

export const isTouch = (): boolean => window.matchMedia(COARSE).matches;

/** Keep <html data-mode="…" data-touch> in sync (CSS uses media queries; scripts read this). */
export function wireMode(): void {
  const set = (): void => {
    document.documentElement.dataset.mode = layoutMode();
    document.documentElement.toggleAttribute("data-touch", isTouch());
  };
  set();
  window.matchMedia(PHONE).addEventListener("change", set);
  window.matchMedia(NARROW).addEventListener("change", set);
  window.matchMedia(COARSE).addEventListener("change", set);
}

/**
 * Type floors on phones (§17.1): no mono text under 12.5 px and no body text under 15 px. The stylesheet sizes
 * most text with the viewport scale, which on a phone lands a few px under; this clamps every text-bearing element
 * that is still under its floor (inline, reversible on leaving phone mode). The hero stream is exempt (decorative).
 */
const FLOORED = "data-type-floor";
export function applyTypeFloors(root: ParentNode = document): void {
  const phone = layoutMode() === "phone";
  root.querySelectorAll<HTMLElement>(`[${FLOORED}]`).forEach((e) => {
    e.style.removeProperty("font-size");
    e.removeAttribute(FLOORED);
  });
  if (!phone) return;
  const all = root.querySelectorAll<HTMLElement>("main *, .topbar *");
  const fix: [HTMLElement, number][] = [];
  for (const e of all) {
    if (e.closest(".stream, svg, .sr-only, canvas, .odo-sr")) continue;
    // typed lines are empty until they play, so they count as text whatever they hold right now
    const own =
      e.classList.contains("typed") ||
      Array.from(e.childNodes).some((n) => n.nodeType === 3 && (n.textContent ?? "").trim().length > 0);
    if (!own) continue;
    const cs = getComputedStyle(e);
    const fs = parseFloat(cs.fontSize);
    if (/mono/i.test(cs.fontFamily)) {
      if (fs < 12.5) fix.push([e, 12.5]);
    } else if (fs < 15 && !e.closest(".kicker, .statusbar, .pane-bar")) fix.push([e, 15]);
  }
  // writes after reads
  for (const [e, px] of fix) {
    e.style.fontSize = `${px}px`;
    e.setAttribute(FLOORED, "");
  }
}

/** Re-apply the floors when the layout mode or the viewport changes, and once the fonts are in. */
export function wireTypeFloors(): void {
  let t = 0;
  const run = (): void => {
    window.clearTimeout(t);
    t = window.setTimeout(() => applyTypeFloors(), 120);
  };
  run();
  document.fonts?.ready.then(run);
  window.addEventListener("resize", run);
  window.matchMedia(PHONE).addEventListener("change", run);
  // content mounted later (typed lines are built on their block's entrance; the dive's and the explainers' stacked
  // states are rebuilt on a mode change): re-check when elements are added, not on text or attribute changes
  window.setTimeout(run, 1500);
  const main = document.querySelector("main");
  if (main)
    new MutationObserver((records) => {
      const added = (r: MutationRecord): boolean =>
        !(r.target as Element).closest?.(".stream") && Array.from(r.addedNodes).some((n) => n.nodeType === 1);
      if (layoutMode() === "phone" && records.some(added)) run();
    }).observe(main, { childList: true, subtree: true });
}

/**
 * Sideways scrollers on phones fade their clipped right edge (phone.css); the fade drops once the scroller is at its
 * end (or has nothing to scroll), marked `data-scroll-end`.
 */
const FADED = ".measure-out, .cp-lines, .vtable, .mtable, .rtable, .term-outputs .term-stack, .a-lines, .transcript, .vsees";
function markEnd(e: Element): void {
  const end = e.scrollLeft + e.clientWidth >= e.scrollWidth - 1;
  e.toggleAttribute("data-scroll-end", end);
}
export function wireScrollFades(): void {
  document.addEventListener(
    "scroll",
    (ev) => {
      const t = ev.target;
      if (t instanceof Element && t.matches(FADED)) markEnd(t);
    },
    { capture: true, passive: true },
  );
  let t = 0;
  const all = (): void => {
    window.clearTimeout(t);
    t = window.setTimeout(() => document.querySelectorAll(FADED).forEach(markEnd), 150);
  };
  all();
  window.addEventListener("resize", all);
  document.fonts?.ready.then(all);
  window.setTimeout(all, 1500);
}
