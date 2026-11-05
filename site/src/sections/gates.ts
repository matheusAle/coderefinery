/**
 * P3 · gates beat reasoning. The three-column table draws its outer seam;
 * rows appear top to bottom, 40 ms apart, each at once. Beneath it the two
 * transcript panes appear at once; the only tier-1 motion is the slag `exit 2`
 * line and the molten CRAP line in the deterministic pane, which land 200 ms
 * after their pane. Nothing types.
 */
import { esc, fromHTML, html, prose } from "../dom";
import { head, loopPanes, statusbar } from "./common";

const ROWS: [string, string, string][] = [
  ["what the agent is told", "“Looks good. Consider simplifying `price()`.”", "CRAP 50.5 > 9.9999 · pricing.py:1 · exit 2"],
  ["what it learns", "write a better summary", "split the function"],
  ["can it argue?", "yes, and it will. the reviewer reads the PR description it wrote.", "no. the gate reads the syntax tree and the coverage report."],
  ["same diff tomorrow", "a different verdict", "the same number, on the laptop and in CI"],
  ["what stops the commit", "nothing", "the exit code, through the hook"],
];

export function mountGates(): HTMLElement {
  return fromHTML(html`
    <section class="section gates type-step" aria-labelledby="gates-h">
      ${head("§03 · why a formula, for an agent", "A model can be talked into it.<br />A formula cannot. The agent notices.", "gates-h")}
      <p class="lede">An agent optimises for the signal it gets. Give it a reviewer it can persuade and it learns to persuade. Give it a number it cannot move and it learns to move the code.</p>
      <div class="vs3 box" data-slot="visual" role="table" aria-label="an LLM reviewing the diff versus a deterministic gate">
        <div class="vs3-row vs3-head" role="row">
          <span role="columnheader"></span>
          <span role="columnheader" class="mono slag"><strong>an LLM reviewing the diff</strong></span>
          <span role="columnheader" class="mono ingot"><strong>a deterministic gate</strong></span>
        </div>
        ${ROWS.map(
          ([k, a, b]) => html`<div class="vs3-row" role="row">
            <span role="rowheader" class="mono ash">${esc(k)}</span>
            <span role="cell" class="mono">${prose(a)}</span>
            <span role="cell" class="mono">${esc(b)}</span>
          </div>`,
        )}
      </div>
      ${loopPanes()}
      ${statusbar(html`the full argument, with every verdict and the move it names: <a href="/agents">/agents</a>`, 0, { dot: true, links: true })}
    </section>
  `);
}
