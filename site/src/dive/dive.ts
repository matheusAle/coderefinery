/**
 * The pinned walkthrough (§13.2): a 400 vh container with a 100 vh sticky
 * stage; the page pins when the P4 CTA slab reaches the top of the viewport.
 *
 * - desktop: one DivePane, driven by pin progress
 * - reduced motion: four settled snapshots cross-faded by scroll; the gauge still fills
 * - under 1024 px: the four snapshots stacked, unpinned (§13.4), each a card of the compact deck (§17.4)
 */
import { el } from "../dom";
import { invalidate, onScroll, pinProgress, range, reducedMotion } from "../motion/scroll";
import { entrance, phoneMount, pinnedMount } from "../motion/mount";
import { DivePane } from "./pane";
import { STATES } from "./states";

export interface DiveHandle {
  el: HTMLElement;
}

const NARROW = "(max-width: 1023px)";

export function mountDive(): DiveHandle {
  const section = el("section", "dive");
  section.id = "inside-the-gate";
  section.setAttribute("aria-label", "inside the gate: the four gates on one function");
  const heading = el("h2", "sr-only", "Inside the gate");
  section.appendChild(heading);

  function buildPinned(): void {
    section.classList.remove("stacked");
    const sticky = el("div", "dive-sticky nojs-stack");
    section.appendChild(sticky);
    if (reducedMotion()) {
      const panes = STATES.map((_, i) => new DivePane({ snapshot: i }));
      panes.forEach((p) => sticky.appendChild(p.el));
      onScroll(() => {
        const p = pinProgress(section);
        const idx = Math.min(STATES.length - 1, Math.max(0, Math.floor(p * STATES.length)));
        const t = p * STATES.length - idx;
        panes.forEach((pane, i) => {
          const o = i === idx ? 1 : i === idx + 1 ? range(t, 0.85, 1) : 0;
          pane.el.style.opacity = o.toFixed(3);
          pane.el.style.visibility = o > 0 ? "visible" : "hidden";
          pane.el.style.setProperty("--ground", "1");
          pane.setDepth(Math.max(0, Math.min(1, p)));
        });
      });
      return;
    }
    const pane = new DivePane({ shaders: true });
    sticky.appendChild(pane.el);
    onScroll(() => {
      const p = pinProgress(section);
      const r = section.getBoundingClientRect();
      pane.setVisible(r.bottom > 0 && r.top < window.innerHeight);
      // the first state's title mounts during the entrance range so the pin begins assembled (§0c)
      pane.setEntrance(pinnedMount(entrance(section)));
      pane.apply(p);
      // K4 → numbers: the glow drains and the gauge fill drains upward as the panes unpin
      const leave = p > 1 ? range(r.bottom, window.innerHeight * 0.4, window.innerHeight) : 1;
      pane.el.style.setProperty("--leave", leave.toFixed(3));
    });
  }

  function buildStacked(): void {
    section.classList.add("stacked");
    section.querySelector(".dive-depth")?.remove();
    // each state is a deck card (§17.4): sticky inside its own hold, which carries the runway
    const panes = STATES.map((_, i) => {
      const pane = new DivePane({ snapshot: i });
      const hold = el("div", "deck-hold");
      hold.appendChild(pane.el);
      section.appendChild(hold);
      return pane;
    });
    if (reducedMotion()) return;
    // phone and tablet (§17.2): each state card mounts on its own entrance in the §17.2 order and stays mounted
    const best = panes.map(() => 0);
    panes.forEach((p) => p.setPhoneEntrance(0));
    onScroll(() => {
      const vh = window.innerHeight;
      panes.forEach((p, i) => {
        if (best[i] >= 0.9) return;
        const e = Math.max(0, Math.min(1, (vh - p.el.getBoundingClientRect().top) / (0.6 * vh)));
        const m = Math.max(best[i], phoneMount(e));
        if (m === best[i] && m > 0) return;
        best[i] = m;
        p.setPhoneEntrance(m);
      });
    });
    // the depth gauge becomes a thin horizontal bar under the first kicker, filling as the four states scroll past
    const bar = el("div", "dive-depth");
    bar.setAttribute("aria-hidden", "true");
    bar.appendChild(el("span", ""));
    // sticky across the whole dive (a child of the section, before K1), so it keeps filling while K2–K4 scroll by
    section.insertBefore(bar, panes[0].el.parentElement);
    const fill = bar.firstElementChild as HTMLElement;
    onScroll(() => {
      const r = section.getBoundingClientRect();
      const vh = window.innerHeight;
      fill.style.setProperty("--p", range(vh * 0.4 - r.top, 0, Math.max(1, r.height - vh * 0.6)).toFixed(3));
    });
  }

  function build(): void {
    section.querySelectorAll(".dive-sticky, .deck-hold, .dive-stage").forEach((n) => n.remove());
    if (window.matchMedia(NARROW).matches) buildStacked();
    else buildPinned();
    invalidate();
  }
  build();
  window.matchMedia(NARROW).addEventListener("change", build);
  window.matchMedia("(max-width: 767px)").addEventListener("change", build);

  return { el: section };
}
