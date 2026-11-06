/**
 * The "math" page (motion-spec §16): M0 hero, four pinned explainers (one
 * per gate), M5 parity. Copy is verbatim from
 * designs/reference/math-reference.html. No shaders.
 */
import { esc, fromHTML, html, qa } from "../dom";
import { odometer } from "../motion/odometer";
import { onMount } from "../motion/mount";
import { range, reducedMotion } from "../motion/scroll";
import { formulaStrip, head, statusbar } from "../sections/common";
import { topbar } from "../sections/topbar";
import { mountExplainer } from "./explainer";
import { GATES } from "./content";

/* ---------- M0 · hero ---------- */
export function mountMathHero(): HTMLElement {
  return fromHTML(html`
    <header class="section m-hero" id="top">
      ${topbar("math")}
      <div class="m-hero-head">
        ${head("§ the math · every verdict, by hand", "Three numbers. Three rules.<br />No opinions.", "m-hero-h")}
        <p class="lede">Every verdict coderefinery gives is a formula you can check with a pencil. This page shows what each gate counts, how it scores, where its threshold comes from, and what the number quietly implies. All of it as implemented, byte-identical to crapper, dryer and mutator.</p>
      </div>
      ${formulaStrip()}
      ${statusbar("sources: docs/spec-crapper.md · spec-dryer.md · spec-mutator.md · the skill · every number below is reproducible with the fixtures in the repo", 0, { dot: true })}
    </header>
  `);
}

/* ---------- the four explainers: content in ./content.ts (verbatim from the reference) ---------- */
export function mountExplainers(): HTMLElement[] {
  return GATES.map(mountExplainer);
}

/* ---------- M5 · parity ---------- */
const NUMBERS: [string, string, boolean][] = [
  ["36/36", "harness cases byte-identical to the originals", true],
  ["99.3 %", "of the Go stdlib parses to the same tree", false],
  ["100 %", "of the JDK sources, 15,759 files", false],
  ["99.5 %", "of ~/.cargo, 10,216 crates", false],
  ["100 %", "of a production TS monorepo, 12,167 files", false],
];
const DETERMINISM: [string, string][] = [
  ["no clock", "timestamps only in :tested-at, never in a score or an order"],
  ["no locale", "sorting by code point, not collation"],
  ["no randomness", "no sampling of pairs, mutants or files"],
  ["no model", "nothing reads the PR description, the commit message or the transcript"],
];

export function mountParity(): HTMLElement {
  const el = fromHTML(html`
    <section class="section m-parity" aria-labelledby="m5-h">
      ${head("§05 · why you can check any of this by hand", "Same tree in, same number out.", "m5-h")}
      <div class="pnums box" data-slot="visual" role="list">
        ${NUMBERS.map(
          ([v, l, molten]) => html`<div class="pnum" role="listitem">
            <span class="pnum-v display ${molten ? "molten" : ""}" data-odo="${esc(v)}"></span>
            <span class="pnum-l mono ash">${esc(l)}</span>
          </div>`,
        )}
      </div>
      <div class="pdet" data-slot="card">
        ${DETERMINISM.map(([k, v]) => html`<div class="pdet-i"><span class="mono"><strong>${esc(k)}</strong></span><span class="mono ash">${esc(v)}</span></div>`)}
      </div>
      <div class="cta box molten-seam" data-slot="card">
        <div class="cta-left">
          <p class="cta-h display">Run it on your repo and check a number by hand.</p>
          <p class="cta-s mono ash"><code class="ic">$ npx skills add matheusAle/coderefinery</code> · fixtures, harness and benchmarks are in the repo</p>
        </div>
        <a class="cta-btn mono" href="https://github.com/matheusAle/coderefinery/tree/main/docs" rel="noopener">read the specs →</a>
      </div>
    </section>
  `);
  // the five numbers roll from 0 when the strip enters view, the molten 36/36 last
  const odos = qa(el, "[data-odo]").map((h) => {
    const o = odometer(h.dataset.odo!);
    h.appendChild(o.el);
    return o;
  });
  const strip = el.querySelector<HTMLElement>(".pnums")!;
  const rm = reducedMotion();
  onMount(el, (_q, slot) => {
    const sp = rm ? 1 : slot(strip);
    odos.forEach((o, i) => o.set(rm ? 1 : i === 0 ? range(sp, 0.3, 1) : range(sp, 0.05 * i, 0.6 + 0.05 * i)));
  });
  return el;
}
