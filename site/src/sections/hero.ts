/**
 * P0 · hero (motion-spec §12 P0). Two columns over an event stream: headline,
 * mechanism, install line and foot on the left; the four big numbers as a
 * 2 × 2 block on the right; behind everything, above the slag field, the
 * infinite stream of CI events rising from the bottom edge (`stream.ts`). On
 * load, on a clock: "Prove it." lands at 200 ms; the numbers roll from 0 left
 * to right, top to bottom, the molten one last; the stream starts with the
 * page and never stops.
 */
import intro from "../../data/intro.json";
import { esc, fromHTML, html, q, qa } from "../dom";
import { odometer } from "../motion/odometer";
import { reducedMotion } from "../motion/scroll";
import { mountSlagField } from "../gl/slag";
import { mountStream } from "./stream";
import { topbar } from "./topbar";

export interface HeroHandle {
  el: HTMLElement;
  /** The slag field and the event stream cover the whole hero at any width: both live in the full-width wrapper, behind the centred column. */
  attachField(wrapper: HTMLElement): void;
}

export function mountHero(): HeroHandle {
  const stats = intro.hero.stats;
  const el = fromHTML(html`
    <header class="hero section" id="top" data-plain>
      ${topbar()}
      <div class="hero-cols">
        <div class="pitch">
          <h1 class="hero-h1">
            <span class="hero-line">Your agent says<br />all tests pass.</span>
            <span class="hero-prove">Prove it.</span>
          </h1>
          <p class="hero-lede">Three gates on the diff, before the commit. Complexity without tests. Copies under new names. Tests that cannot fail. If one fails, the commit fails.</p>
          <div class="install box" role="group" aria-label="install command">
            <span class="prompt molten">$</span>
            <code class="install-cmd">npx skills add matheusAle/coderefinery</code>
            <span class="wm-cursor" aria-hidden="true"></span>
          </div>
          <p class="hero-foot mono ash">one binary · no runtime · Claude Code, Cursor, CI · byte-identical to Uncle Bob's crapper, dryer, mutator</p>
        </div>
        <div class="bignums-2 box" role="list">
          ${stats.map(
            (s) => html`<div class="bn" role="listitem">
              <span class="bn-v display ${s.molten ? "molten" : ""}" data-odo="${esc(s.value)}"></span>
              <span class="bn-l mono">${esc(s.label)}</span>
              <a class="bn-s mono ash" href="${esc(s.href)}" rel="noopener">${esc(s.source)}</a>
            </div>`,
          )}
        </div>
      </div>
      <div class="statusbar hero-status">
        <span class="dot">the exit code is the only review that cannot be rubber-stamped</span>
        <span class="molten">scroll ↓</span>
      </div>
    </header>
  `);

  const rm = reducedMotion();
  const odos = qa(el, "[data-odo]").map((h) => {
    const o = odometer(h.dataset.odo!);
    h.appendChild(o.el);
    return o;
  });
  // numbers roll left to right, top to bottom, the molten one (index 2) last
  const order = [0, 1, 3, 2];
  if (rm) {
    odos.forEach((o) => o.set(1));
  } else {
    order.forEach((idx, i) => {
      const o = odos[idx];
      const start = performance.now() + 500 + i * 220;
      const tick = (now: number) => {
        const p = (now - start) / 1100;
        if (p < 0) return requestAnimationFrame(tick);
        o.set(1 - Math.pow(1 - Math.min(1, p), 3));
        if (p < 1) requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
  }

  return {
    el,
    attachField(wrapper) {
      const host = document.createElement("div");
      host.className = "gl-host";
      host.setAttribute("aria-hidden", "true");
      wrapper.insertBefore(host, wrapper.firstChild);
      mountSlagField(wrapper, host);
      // the event stream starts with the page and never stops (§12 P0). It spans the full-width wrapper, above the
      // slag field and behind the hero (a z-1 sibling before it); the install box and the numbers block keep pointer priority.
      const stream = mountStream(
        wrapper,
        { foot: q(el, ".hero-foot"), lede: q(el, ".hero-lede"), status: q(el, ".hero-status"), priority: [q(el, ".install"), q(el, ".bignums-2")] },
        rm,
      );
      wrapper.insertBefore(stream, el);
    },
  };
}
