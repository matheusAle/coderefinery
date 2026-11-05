import { esc, html } from "../dom";

export function statusbar(left: string, exit: 0 | 2 | 3, opts: { dot?: boolean; links?: boolean } = {}): string {
  const tone = exit === 0 ? "ingot" : "slag";
  return html`<div class="statusbar">
    <span class="${opts.dot ? "dot" : ""}">${opts.links ? left : esc(left)}</span>
    <span class="exit ${tone}">exit ${exit}</span>
  </div>`;
}

export function head(kicker: string, title: string, id?: string): string {
  return html`<p class="kicker">${esc(kicker)}</p>
    <h2 class="h2"${id ? ` id="${id}"` : ""}>${title}</h2>`;
}

/**
 * The two "what the agent learns" transcript panes. Shared by P3 on the home
 * page and A1 on /agents (same copy on both). Tier 2, except the molten CRAP
 * line and the slag `exit 2` line, which land 200 ms after their pane.
 */
export function loopPanes(): string {
  return html`<div class="a-two-panes">
    <figure class="pane box a-pane" data-slot="card" aria-label="with an LLM reviewer">
      <div class="pane-bar"><span>with an LLM reviewer</span><span class="ash">what the agent learns</span></div>
      <div class="a-lines">
        <div class="a-line agent"><span class="agent-k mono ash">agent</span><span>Refactored for clarity and added comprehensive tests. Ready for review.</span></div>
        <div class="a-line mono ash">reviewer: Looks good overall. Consider simplifying <code class="ic">price()</code>. LGTM.</div>
        <div class="a-line agent"><span class="agent-k mono ash">agent</span><span>Thanks! Merging.</span></div>
        <div class="a-line mono slag">lesson: a confident summary is as good as a fix.</div>
      </div>
    </figure>
    <figure class="pane box a-pane" data-slot="card" aria-label="with the gate">
      <div class="pane-bar"><span>with the gate</span><span class="ash">what the agent learns</span></div>
      <div class="a-lines">
        <div class="a-line mono"><span class="prompt">$</span> coderefinery crap --changed --threshold 9.9999</div>
        <div class="a-line mono molten land">billing/pricing.py:price&nbsp;&nbsp;&nbsp;CRAP 50.5&nbsp;&nbsp;&nbsp;cov 20.0&nbsp;&nbsp;&nbsp;cc 9</div>
        <div class="a-line mono slag land">CRAP threshold exceeded: 50.5 &gt; 9.9999 · exit 2</div>
        <div class="a-line agent"><span class="agent-k mono ash">agent</span><span>Nine decisions, a fifth covered. Splitting <code class="ic">price()</code> into three functions and testing each.</span></div>
        <div class="a-line mono ingot">lesson: the number moves only when the code does.</div>
      </div>
    </figure>
  </div>`;
}

/** The four-formula strip: the M0 hero and the home-page teaser share it. Formula boxes get a molten seam on hover (§16). */
export function formulaStrip(): string {
  const rows: [string, string, string][] = [
    ["CRAP = CC² × (1 − cov)³ + CC", "complexity your tests never walk", "≥ 10 fails · exit 2"],
    ["score = |A ∩ B| / |A ∪ B|", "structural similarity, names erased", "≥ 0.8 fails · in the report"],
    ["killed / (killed + survived)", "mutants your tests noticed", "1 survivor fails · exit 3"],
    ["unique kills per test", "what each test alone catches", "zero kills or shadowed fails · exit 3"],
  ];
  return html`<div class="fstrip box" data-slot="visual" role="list">
    ${rows.map(
      ([f, s, g]) => html`<div class="fcell" role="listitem">
        <span class="fcell-f display">${esc(f)}</span>
        <span class="fcell-s mono ash">${esc(s)}</span>
        <span class="fcell-g mono molten">${esc(g)}</span>
      </div>`,
    )}
  </div>`;
}
