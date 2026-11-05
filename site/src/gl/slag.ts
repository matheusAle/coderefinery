/**
 * Slag field (§14.4.1, §0c "discreet field"): the dot grid with a
 * pointer-following heat that lags ≈300 ms. Two mounts:
 * - `mountSlagField(card, host, peak)` — the hero's, clipped to the hero card;
 *   it pauses (keeps its last frame) while the card is being covered by the
 *   next one (§0c "Deck stacking"; the cover replaces the old melt-on-exit).
 * - `mountDiscreetField()` — one canvas at low intensity (dots 3 %,
 *   heat peak 2 %, radius 60 %, no melt). It lives inside the face of the
 *   deck's active card (`attach(face)`, at every width since §17.4), so it is
 *   clipped to that card; when it moves on, the card it leaves keeps a frozen
 *   copy of its last frame (the field "paused" under the dimmed, covered card).
 *   Where the active card takes no field (the K dive) it stops drawing.
 */
import type { FieldHost } from "../motion/deck";
import { createPass, onFrame } from "./shader";
import { SLAG_FRAG } from "./shaders";

const dpr = () => Math.min(window.devicePixelRatio || 1, 2);
/** dot pitch: 6 px, √2 × on phones so the field draws half the dots (§17.2) */
const pitch = () => (window.matchMedia("(max-width: 767px)").matches ? 6 * Math.SQRT2 : 6);

export function mountSlagField(section: HTMLElement, host: HTMLElement, peak = 1): void {
  // `section` is the hero's card (the full-width wrapper); `data-covered` is set by the deck
  const pass = createPass(host, SLAG_FRAG, "slag field (hero)");
  if (!pass) return;
  let px = -9999;
  let py = -9999;
  let tx = px;
  let ty = py;
  // pointer → canvas space: offset from the canvas rect, in device pixels, y flipped (GL origin is bottom-left)
  window.addEventListener("pointermove", (e) => {
    const r = pass.canvas.getBoundingClientRect();
    const d = dpr();
    tx = (e.clientX - r.left) * d;
    ty = (r.bottom - e.clientY) * d;
  });
  section.addEventListener("pointerleave", () => {
    tx = -9999;
    ty = -9999;
  });
  onFrame((t) => {
    // paused while the card is covered: the canvas keeps its last frame under the dim
    if (!pass.alive || section.dataset.covered) return;
    px += (tx - px) * 0.12;
    py += (ty - py) * 0.12;
    pass.uniform("u_pointer", px, py);
    pass.uniform("u_intensity", peak);
    pass.uniform("u_dot", 0.055);
    pass.uniform("u_heat", 0.06);
    pass.uniform("u_radius", 280);
    pass.uniform("u_dpr", dpr());
    pass.uniform("u_pitch", pitch());
    pass.draw(t);
  });
}

export function mountDiscreetField(): FieldHost | null {
  const host = document.createElement("div");
  host.className = "gl-field";
  host.setAttribute("aria-hidden", "true");
  document.body.prepend(host);
  const pass = createPass(host, SLAG_FRAG, "slag field (discreet)");
  if (!pass) {
    host.remove();
    return null;
  }
  // where the field lives: the face of the active card (null until the deck attaches it, or on the dive)
  let current: HTMLElement | null = null;
  let wanted: HTMLElement | null = null;
  const freeze = (face: HTMLElement): void => {
    let c = face.querySelector<HTMLCanvasElement>(":scope > canvas.gl-freeze");
    if (!c) {
      c = document.createElement("canvas");
      c.className = "gl-freeze";
      c.setAttribute("aria-hidden", "true");
      face.insertBefore(c, face.firstChild);
    }
    c.width = pass.canvas.width;
    c.height = pass.canvas.height;
    // the WebGL back buffer is still valid right after draw() in the same frame
    c.getContext("2d")?.drawImage(pass.canvas, 0, 0);
  };
  /** Move the canvas to the face it is wanted on; the face it leaves keeps a frozen copy of the frame just drawn. */
  const move = (face: HTMLElement): void => {
    if (current) freeze(current);
    current = face;
    face.querySelector(":scope > canvas.gl-freeze")?.remove();
    host.classList.add("in-card");
    face.insertBefore(host, face.firstChild);
  };
  let px = -9999;
  let py = -9999;
  let tx = px;
  let ty = py;
  window.addEventListener("pointermove", (e) => {
    const r = pass.canvas.getBoundingClientRect();
    const d = dpr();
    tx = (e.clientX - r.left) * d;
    ty = (r.bottom - e.clientY) * d;
  });
  onFrame((t) => {
    if (!pass.alive) return;
    // the active card takes no field (the dive): pause where it is; its card keeps the last frame
    if (!wanted) return;
    px += (tx - px) * 0.12;
    py += (ty - py) * 0.12;
    pass.uniform("u_pointer", px, py);
    pass.uniform("u_intensity", 1);
    pass.uniform("u_dot", 0.03);
    pass.uniform("u_heat", 0.02);
    pass.uniform("u_radius", 168);
    pass.uniform("u_dpr", dpr());
    pass.uniform("u_pitch", pitch());
    pass.draw(t);
    if (wanted !== current) move(wanted);
  });
  return {
    attach(face) {
      wanted = face;
    },
  };
}
