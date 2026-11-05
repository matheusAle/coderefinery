/**
 * P4 · the tests. Stats roll (tier 1); the quote and the pattern list appear
 * at once (tier 2). The slab on the right appears at once; its mutant-to-test
 * rows are tier 2. The CTA slab at the bottom draws its molten stroke and the
 * button's arrow nudges down once on a clock. No click is needed. The bridge
 * into K1 (§12): over the last 10 % of the card's runway (its hold), before the dive
 * card arrives, `$ coderefinery crap --changed` types inside the button,
 * replacing its "keep scrolling ↓ · the gate opens" label (scrubbed, so it
 * reverses); then the dive card slides over P4 and pins.
 */
import intro from "../../data/intro.json";
import { esc, fromHTML, html, prose, qa } from "../dom";
import { odometer } from "../motion/odometer";
import { onMount, runwayProgress } from "../motion/mount";
import { onScroll, range, reducedMotion } from "../motion/scroll";
import { layoutMode } from "../motion/mode";
import { typer } from "../motion/typer";
import { head, statusbar } from "./common";

const STATS: [string, string, string, string][] = [
  [intro.tests.stats[0], "of 86,156 agent-written test patches have weak or no assertion", "arXiv 2606.18168 · 2026", "https://arxiv.org/html/2606.18168v1"],
  [intro.tests.stats[1], "of agent test runs piped through tail or head, hiding the exit code", "claude-code #104504", "https://github.com/anthropics/claude-code/issues/104504"],
  [intro.tests.stats[2], "“all tests pass” claims in 150 sessions; 80 had no run behind them", "claude-code #104504", "https://github.com/anthropics/claude-code/issues/104504"],
];
const PATTERNS: [string, string][] = [
  ["assert isinstance(result, float)", "passes if the function returns `0.0`"],
  ["mock every dependency, assert the mock was called", "tests the implementation"],
  ["reimplement the function inside the test", "mirrors the bug perfectly"],
  ["the same test three times with new names", "drifts, never fails"],
  ["skipped: flaky in CI", "the agent's favourite fix"],
];
const MUTANTS: [string, string][] = [
  ["0 → 1", "a case where the result must be exactly zero"],
  ["< → ≤", "a case on the boundary"],
  ["and → or", "each operand alone"],
  // ["1 → 0", "the default path, observed through the result"] — dropped so the card fits with the §0c spacing
  // floors (after one type step: "reduce the type scale one step first, then the number of items shown")
];

export function mountTests(): HTMLElement {
  const el = fromHTML(html`
    <section class="section tests type-step" aria-labelledby="tests-h">
      ${head("§04 · the tests", "Nobody verifies whether the new tests can fail.", "tests-h")}
      <div class="tests-row">
        <div class="evidence">
          <div class="tstats box" data-slot="card" role="list">
            ${STATS.map(
              ([v, l, s, href], i) => html`<div class="tstat" role="listitem">
                <span class="tstat-v display ${i === 0 ? "molten" : ""}" data-odo="${esc(v)}"></span>
                <span class="tstat-l mono">${esc(l)}</span>
                <a class="tstat-s mono ash" href="${href}" rel="noopener">${esc(s)}</a>
              </div>`,
            )}
          </div>
          <blockquote class="quote big" data-slot="card">
            <p>“The genie was cheating, for example by disabling or deleting tests.”</p>
            <cite class="mono ash"><a href="https://newsletter.kentbeck.com/p/augmented-coding-beyond-the-vibes" rel="noopener">Kent Beck · June 2025</a></cite>
          </blockquote>
          <div class="patterns" data-slot="card">
            <h3 class="mono patterns-h">what the suite fills up with</h3>
            <ul class="mono ash">
              ${PATTERNS.map(([a, b]) => html`<li><span>${esc(a)}</span><span>${prose(b)}</span></li>`)}
            </ul>
          </div>
        </div>
        <figure class="measure pane box" data-slot="visual" aria-label="what the agent does with a survivor">
          <div class="pane-bar"><span>what the agent does with a survivor</span><span class="ash">mutate · tests</span></div>
          <div class="measure-body">
            <p class="measure-q display">The mutant tells the agent which test to write.</p>
            <p class="measure-p">A surviving mutant is a precise spec of a missing assertion. The agent reads the edit nobody noticed and writes the case that would notice it.</p>
            <div class="m2t box" role="table" aria-label="mutant to test">
              ${MUTANTS.map(([m, t]) => html`<div class="m2t-row" role="row"><span role="cell" class="mono molten">${esc(m)}</span><span role="cell" class="mono">${esc(t)}</span></div>`)}
            </div>
            <p class="measure-note mono ash">and a test that kills nothing is listed for deletion:</p>
            <pre class="measure-out mono">$ coderefinery tests --changed
test_settle_called         kills 0   unique 0   ← delete
test_price_returns_number  kills 0   unique 0   ← delete
exit 3</pre>
            <p class="measure-prev mono ember" data-support>preview · ships with the tests gate</p>
          </div>
        </figure>
      </div>
      <div class="cta box molten-seam" id="cta" data-slot="card">
        <div class="cta-left">
          <p class="cta-h display">Next: inside the gates. Watch the loop run on one function.</p>
          <p class="cta-s mono ash">no click needed · the page pins and the code changes as you scroll · four stops</p>
        </div>
        <a class="cta-btn mono" href="#inside-the-gate" aria-label="keep scrolling: the gate opens">
          <span class="cta-label" aria-hidden="true">keep scrolling <span class="arrow">↓</span> · the gate opens</span>
          <span class="cta-bridge" aria-hidden="true"><span class="prompt">$</span> <span data-bridge="coderefinery crap --changed"></span></span>
        </a>
      </div>
      ${statusbar("green is not a verdict. a suite that cannot fail cannot be trusted.", 3, { dot: true })}
    </section>
  `);

  const statOdos = qa(el, ".tstat-v[data-odo]").map((h) => {
    const o = odometer(h.dataset.odo!);
    h.appendChild(o.el);
    return o;
  });
  const rm = reducedMotion();
  const stats = el.querySelector<HTMLElement>(".tstats")!;
  onMount(el, (_q, slot) => {
    const sp = rm ? 1 : slot(stats);
    statOdos.forEach((o, i) => o.set(rm ? 1 : range(sp, i * 0.1, 0.6 + i * 0.1)));
  });

  // the bridge into K1: the last 10 % of the card's entrance (e 0.90 → 1.00), scrubbed
  const btn = el.querySelector<HTMLElement>(".cta-btn")!;
  const bridgeHost = el.querySelector<HTMLElement>("[data-bridge]")!;
  const bridge = typer(bridgeHost.dataset.bridge!, { cursor: true });
  bridgeHost.appendChild(bridge.el);
  onScroll(() => {
    const card = el.closest<HTMLElement>(".hold") ?? el;
    // the last 10 % of P4's runway (its hold), just before the dive card's top appears
    const b = rm || layoutMode() !== "desktop" ? 0 : range(runwayProgress(card), 0.9, 1);
    btn.classList.toggle("bridging", b > 0);
    bridge.set(b);
    bridge.cursor(b > 0 && b < 1);
  });
  return el;
}
