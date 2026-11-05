/**
 * One scroll-progress observer for the whole page (motion-spec §11).
 * Every subscriber is called once per animation frame in which the scroll
 * position or viewport size changed, with the metrics it asked for.
 * Values are plain functions of scroll, so scrubbing backwards reverses them.
 */

export const reducedMotion = (): boolean =>
  window.matchMedia("(prefers-reduced-motion: reduce)").matches;

export const clamp01 = (v: number): number => (v < 0 ? 0 : v > 1 ? 1 : v);

/** Maps t from [a, b] to [0, 1], clamped. */
export const range = (t: number, a: number, b: number): number =>
  clamp01((t - a) / (b - a));

export const easeOutCubic = (t: number): number => 1 - Math.pow(1 - clamp01(t), 3);

export const lerp = (a: number, b: number, t: number): number => a + (b - a) * t;

type Subscriber = () => void;

const subs = new Set<Subscriber>();
let scheduled = false;
let lastY = -1;
let lastH = -1;

function flush(): void {
  scheduled = false;
  lastY = window.scrollY;
  lastH = window.innerHeight;
  for (const s of subs) s();
}

function schedule(): void {
  if (scheduled) return;
  scheduled = true;
  requestAnimationFrame(flush);
}

export function onScroll(fn: Subscriber): () => void {
  subs.add(fn);
  schedule();
  return () => subs.delete(fn);
}

/** Force a re-run on the next frame (after layout changes). */
export function invalidate(): void {
  lastY = -1;
  schedule();
}

export function startScrollObserver(): void {
  window.addEventListener(
    "scroll",
    () => {
      if (window.scrollY !== lastY) schedule();
    },
    { passive: true },
  );
  window.addEventListener("resize", () => {
    if (window.innerHeight !== lastH) invalidate();
    else schedule();
  });
  schedule();
}

/** How far a section has entered: 0 when its top is at the viewport bottom, 1 when its top reaches the viewport top. */
export function enterProgress(el: Element): number {
  const r = el.getBoundingClientRect();
  const vh = window.innerHeight;
  return clamp01((vh - r.top) / vh);
}

/**
 * Pin progress of a tall container with a sticky 100 vh child:
 * 0 when the container's top hits the viewport top, 1 when its bottom
 * hits the viewport bottom. Values outside [0, 1] are returned unclamped so
 * callers can see the approach (negative) and the exit (> 1).
 */
export function pinProgress(container: Element): number {
  const r = container.getBoundingClientRect();
  const travel = r.height - window.innerHeight;
  if (travel <= 0) return 0;
  return -r.top / travel;
}
