/**
 * S9 · the numbers. Unpinned. Triggered when the section is 30 % in view and
 * scrubbed over the next 40 % of viewport travel: each big number rolls from 0,
 * right-most digit settling last; `120×` starts 10 % later so it lands last.
 * The fact columns reveal top to bottom per line, ash → chalk on landing. The
 * status bar's `exit 0` draws its molten underline at 60 % in view.
 */
import intro from "../../data/intro.json";
import { esc, fromHTML, html, prose, q, qa } from "../dom";
import { odometer } from "../motion/odometer";
import { onMount } from "../motion/mount";
import { range, reducedMotion } from "../motion/scroll";
import { head, statusbar } from "./common";

const FACTS: [string, string[]][] = [
  ["languages · 7", ["python · typescript · tsx · javascript", "go · java · rust · clojure", "same tree shape as tree-sitter, hand-written in Go"]],
  ["one binary", ["`7.2 MB` · stdlib only · no cgo", "macOS + Linux · amd64 + arm64", "no venv, no node, no jvm to run it"]],
  ["install", ["`$ curl -fsSL https://get.coderefinery.dev | sh`", "`$ npx skills add coderefinery`", "`$ go install github.com/.../coderefinery@latest`"]],
];

export function mountNumbers(): HTMLElement {
  const el = fromHTML(html`
    <section class="section numbers" id="numbers" aria-labelledby="numbers-h">
      <div class="numbers-head">${head("§06 · after the intro", "Same output as the originals.<br />A fraction of the time.", "numbers-h")}</div>
      <div class="bignums box" data-slot="visual" role="list">
        ${intro.numbers.map(
          (n) => html`<div class="bignum" role="listitem">
            <span class="bignum-v display ${n.molten ? "molten" : ""}" data-odo="${esc(n.value)}"></span>
            <span class="bignum-l mono ash">${esc(n.label)}</span>
          </div>`,
        )}
      </div>
      <div class="facts">
        ${FACTS.map(
          ([t, lines]) => html`<div class="fact" data-slot="card">
            <h3 class="fact-h mono">${esc(t)}</h3>
            <div class="fact-rule" aria-hidden="true"></div>
            ${lines.map((l) => html`<p class="fact-l mono">${prose(l)}</p>`)}
          </div>`,
        )}
      </div>
      ${statusbar("measured 2026-11-03 · 3 runs · median · same machine as upstream", 0, { dot: true })}
    </section>
  `);

  const odos = qa(el, "[data-odo]").map((h) => {
    const o = odometer(h.dataset.odo!);
    h.appendChild(o.el);
    return o;
  });
  const factLines = qa(el, ".fact-l");
  const exit = q(el, ".exit");
  const strip = q(el, ".bignums");
  const rm = reducedMotion();
  // the numbers roll inside the strip's slot, 120× starts 10 % later so it lands last; the exit underline draws with the status bar
  onMount(el, (q, slot) => {
    const roll = rm ? 1 : range(slot(strip), 0, 0.9);
    odos.forEach((o, i) => o.set(i === 1 ? range(roll, 0.1, 1) : roll));
    factLines.forEach((l, i) => l.classList.toggle("in", rm || range(slot(l), 0.3, 1) >= (i % 3) * 0.25 + 0.2));
    const sb = rm ? 1 : slot(exit);
    exit.style.setProperty("--underline", sb >= 0.9 ? "1" : "0");
    exit.classList.toggle("drawn", sb >= 0.9);
    void q;
  });
  return el;
}
