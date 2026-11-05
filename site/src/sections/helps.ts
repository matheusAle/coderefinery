/**
 * P2 · how it helps. The four gate cells open along their left seam (§14.3),
 * 60 ms apart; each cell's command types in, scrubbed, as the cell opens. The
 * two lists beneath draw their seams and appear at once (tier 2).
 */
import { esc, prose, fromHTML, html, qa } from "../dom";
import { typer } from "../motion/typer";
import { onMount } from "../motion/mount";
import { range, reducedMotion } from "../motion/scroll";
import { head, statusbar } from "./common";

const GATES: [string, string, string, string][] = [
  ["coderefinery crap --changed", "Is it too complex for its tests?", "counts every decision, reads the coverage report, scores `CRAP` per function. `≥ 10 → exit 2`.", "after Alberto Savoia's CRAP (2007) and Uncle Bob's crapper (ub-mode, 2025)"],
  ["coderefinery dry --changed", "Is it a copy with new names?", "normalises each function's syntax tree, erases names and literals, compares every pair. `≥ 0.8` → in the report.", "after dry4clj and Uncle Bob's dryer"],
  ["coderefinery mutate --changed", "Would the tests notice a bug?", "flips operators and constants one at a time, runs the suite per mutant, re-runs only survivors and changed functions. survivor → `exit 3`.", "after Uncle Bob's mutator; the idea is from 1978"],
  ["coderefinery tests", "Does this test catch anything?", "attributes every kill to the tests that caused it. a test with zero kills, or shadowed by other tests, is listed for deletion. → `exit 3`.", "reads JUnit XML from any runner."],
];
const DOES: [string, string][] = [
  ["gates the diff", "`--changed` on every command. a ten-year codebase never passes a repo-wide mandate; a PR can."],
  ["refuses, then explains", "exit code first, line numbers second. the agent reads both and fixes the code, not the gate."],
  ["runs where the agent runs", "a hook in Claude Code or Cursor, the same three commands in CI. one binary, no runtime."],
  ["stays honest", "byte-identical to the reference tools on 36/36 cases. 3–120× faster, measured on 12 repos."],
];
const REFUSES: [string, string][] = [
  ["a coverage %", "agents write `assert True` to hit `90 %`. the gate asks whether the test can fail."],
  ["another AI reviewer", "no model in the loop. same input, same verdict."],
  ["a repo-wide audit", "it does not grade your legacy. it grades the change."],
  ["a warning", "a warning is a suggestion. an exit code is a decision."],
];

function list(title: string, tone: "ingot" | "slag", rows: [string, string][]): string {
  return html`<div class="hlist box">
    <div class="hlist-head mono ${tone}"><strong>${esc(title)}</strong></div>
    ${rows.map(
      ([k, v]) => html`<div class="hlist-row">
        <span class="hlist-k mono ${tone}">${esc(k)}</span><span class="hlist-v mono">${prose(v)}</span>
      </div>`,
    )}
  </div>`;
}

export function mountHelps(): HTMLElement {
  const el = fromHTML(html`
    <section class="section helps" id="gates" aria-labelledby="helps-h">
      ${head("§02 · coderefinery", "Four gates. One binary. Exit codes.", "helps-h")}
      <div class="gate-cells box" data-slot="visual" role="list">
        ${GATES.map(
          ([cmd, q, body, after], i) => html`<div class="gate-cell" role="listitem" style="--i:${i}">
            <div class="gate-cmd mono molten"><span class="prompt">$</span> <span data-typer="${esc(cmd)}"></span></div>
            <h3 class="gate-q display">${esc(q)}</h3>
            <p class="gate-body mono">${prose(body)}</p>
            <p class="gate-after mono ash">${esc(after)}</p>
          </div>`,
        )}
      </div>
      <div class="hlists">
        ${list("what it does", "ingot", DOES)}
        ${list("what it refuses to be", "slag", REFUSES)}
      </div>
      ${statusbar("crap · dry · mutate · tests · next: why a formula changes what the agent does", 0, { dot: true })}
    </section>
  `);

  const cells = qa(el, ".gate-cell");
  const typers = qa(el, "[data-typer]").map((h) => {
    const t = typer(h.dataset.typer!);
    h.appendChild(t.el);
    return t;
  });
  const cellsBox = el.querySelector<HTMLElement>(".gate-cells")!;
  const rm = reducedMotion();
  // the cells open along their left seam as the slot opens; each command types as its cell opens (§12 P2)
  onMount(el, (_q, slot) => {
    const sp = rm ? 1 : slot(cellsBox);
    cells.forEach((c, i) => c.classList.toggle("open", rm || sp > 0.15 + i * 0.08));
    typers.forEach((t, i) => {
      const p = rm ? 1 : range(sp, 0.2 + i * 0.08, 0.55 + i * 0.08);
      t.set(p);
      t.cursor(p > 0 && p < 1);
    });
  });
  return el;
}
