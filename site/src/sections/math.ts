/**
 * M · the math (teaser → /math). Headline, the four-formula strip, a
 * "read the math →" button to the /math route, and the footer line. Static
 * apart from the tier-2 block fades and the seam draws (§14.5, §16).
 */
import { fromHTML, html } from "../dom";
import { formulaStrip, head } from "./common";

export function mountMath(): HTMLElement {
  return fromHTML(html`
    <section class="section math" id="the-math" aria-labelledby="math-h">
      <div class="math-head">
        ${head("§07 · the math", "Three numbers. Three rules. No opinions.", "math-h")}
        <p class="lede">Every verdict is a formula you can check by hand. What each gate counts, where its threshold comes from, and what the number implies.</p>
      </div>
      ${formulaStrip()}
      <div class="cta box molten-seam math-cta" data-slot="card">
        <p class="cta-h display">The full math: decisions per language, the curve, every exit code.</p>
        <a class="cta-btn mono" href="/math">read the math →</a>
      </div>
      <div class="statusbar">
        <span>formulas and thresholds as implemented · byte-identical to crapper, dryer and mutator</span>
        <span class="exit ingot">exit 0</span>
      </div>
    </section>
  `);
}
