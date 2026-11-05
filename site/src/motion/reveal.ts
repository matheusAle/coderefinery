/**
 * Tier-2 reveal (motion-spec §0b) and "seams draw first" (§14.1).
 *
 * - `.t2` blocks get `.in` when their container enters view: one 120 ms fade,
 *   8 px rise, no stagger. The block is readable in full the moment it is on
 *   screen; tier-1 elements inside it animate after.
 * - `.box` blocks get an SVG seam injected; the stroke draws top-left
 *   clockwise over 220 ms, then the content inside appears. Nested boxes draw
 *   80 ms after the outer one.
 */

let io: IntersectionObserver | null = null;

function observer(): IntersectionObserver {
  if (io) return io;
  io = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (!e.isIntersecting) continue;
        const el = e.target as HTMLElement;
        revealNow(el);
        io!.unobserve(el);
      }
    },
    { rootMargin: "0px 0px -8% 0px", threshold: 0.05 },
  );
  return io;
}

export function revealNow(el: HTMLElement): void {
  if (el.classList.contains("box")) el.classList.add("drawn");
  if (el.classList.contains("t2")) el.classList.add("in");
  // children that were waiting on this container
  el.querySelectorAll<HTMLElement>(".t2:not(.in)").forEach((c) => c.classList.add("in"));
  el.querySelectorAll<HTMLElement>(".box:not(.drawn)").forEach((c) => c.classList.add("drawn"));
}

function injectSeam(box: HTMLElement): void {
  if (box.querySelector(":scope > svg.seam")) return;
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "seam");
  svg.setAttribute("aria-hidden", "true");
  const rect = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  rect.setAttribute("x", "0.5");
  rect.setAttribute("y", "0.5");
  rect.setAttribute("width", "100%");
  rect.setAttribute("height", "100%");
  rect.setAttribute("pathLength", "1");
  svg.appendChild(rect);
  box.insertBefore(svg, box.firstChild);
  if (box.parentElement?.closest(".box")) box.classList.add("nested");
}

/** Wire every `.t2` and `.box` under `root`. Safe to call more than once. */
export function wireReveals(root: ParentNode): void {
  root.querySelectorAll<HTMLElement>(".box").forEach(injectSeam);
  const obs = observer();
  root.querySelectorAll<HTMLElement>(".t2, .box").forEach((el) => {
    // only observe top-level reveal containers; nested ones follow their parent
    if (el.parentElement?.closest(".t2, .box")) return;
    obs.observe(el);
  });
}

/** Reveal everything at once (no-JS parity, reduced motion, print). */
export function revealAll(root: ParentNode): void {
  root.querySelectorAll<HTMLElement>(".t2, .box").forEach(revealNow);
}
