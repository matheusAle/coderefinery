/**
 * P1 · the pains. Three columns reveal left to right as the section enters
 * view; the stat rolls (tier 1); quote and bullets appear at once (tier 2).
 * Sources in the status bar are links to the report's citations.
 */
import intro from "../../data/intro.json";
import { esc, fromHTML, html, qa } from "../dom";
import { odometer } from "../motion/odometer";
import { onMount } from "../motion/mount";
import { range, reducedMotion } from "../motion/scroll";
import { head, statusbar } from "./common";

const SRC = {
  faros25: "https://www.faros.ai/blog/ai-productivity-paradox-report-2025",
  faros26: "https://faros.ai/blog/ai-acceleration-whiplash-takeaways",
  dora: "https://dora.dev/research/2025/dora-report/",
  sonar: "https://www.sonarsource.com/company/press-releases/sonar-data-reveals-critical-verification-gap-in-ai-coding/",
  so: "https://survey.stackoverflow.co/2025/ai",
  blacksmith: "https://www.techtimes.com/articles/329367/20261022/sequoia-backed-blacksmith-raises-66m-1b-fix-ci-broken-ai-coding-agents.htm",
  hn: "https://news.ycombinator.com/item?id=45278819",
  coderabbit: "https://www.coderabbit.ai/blog/state-of-ai-vs-human-code-generation-report",
};

interface Column {
  key: string;
  stat: string;
  sub: string;
  quote: string;
  cite: string;
  citeHref: string;
  bullets: string[];
}

const COLS: Column[] = [
  {
    key: "review",
    stat: intro.pains.review,
    sub: "merged PRs per developer, 2025 · review time +91 %",
    quote: "“Believable crap. It looks right, it reads right, and it is wrong in a way that takes twenty minutes to find.”",
    cite: "staff engineer · Hacker News · 2026",
    citeHref: SRC.hn,
    bullets: [
      "· DORA 2025: time saved writing is re-spent auditing and verifying",
      "· Sonar 2026: 96 % do not fully trust AI-written code; 48 % always check",
      "· CodeRabbit: AI PRs carry 1.7× the issues, logic errors +75 %",
    ],
  },
  {
    key: "trust",
    stat: intro.pains.trust,
    sub: "name “almost right” as their top frustration with AI code",
    quote: "“Time saved in creation is re-allocated to auditing and verification.”",
    cite: "DORA · State of AI-assisted development · 2025",
    citeHref: SRC.dora,
    bullets: [
      "· Sonar 2026: 96 % do not fully trust AI-written code; only 48 % always check it",
      "· Stack Overflow 2025: 46 % distrust the output, 33 % trust it",
      "· DORA 2025: 30 % report little or no trust, while throughput and instability both rise",
    ],
  },
  {
    key: "ci",
    stat: intro.pains.ci,
    sub: "CI jobs per week on one runner fleet, 5M → 90M in 2026",
    quote: "“Ten to twenty CI runs per task. The agent loops until green, and every loop is a bill.”",
    cite: "Blacksmith · 2026",
    citeHref: SRC.blacksmith,
    bullets: [
      "· GitHub Actions: 500M → 2.1B minutes a week",
      "· agent PRs 4M → 17M in six months",
      "· most of those minutes run tests that would pass with every function returning nothing",
    ],
  },
];

export function mountPains(): HTMLElement {
  const sources = html`sources:
    <a href="${SRC.faros25}" rel="noopener">Faros 2025</a>/<a href="${SRC.faros26}" rel="noopener">2026</a> ·
    <a href="${SRC.dora}" rel="noopener">DORA 2025</a> ·
    <a href="${SRC.sonar}" rel="noopener">Sonar 2026</a> ·
    <a href="${SRC.so}" rel="noopener">Stack Overflow 2025</a> ·
    <a href="${SRC.blacksmith}" rel="noopener">Blacksmith 2026</a> ·
    full list in <a href="#why">/why</a>`;

  const el = fromHTML(html`
    <section class="section pains" id="why" aria-labelledby="pains-h">
      ${head("§01 · what changed", "The cost moved from writing code to checking it.", "pains-h")}
      <div class="pain-cols">
        ${COLS.map(
          (c, i) => html`<div class="pain-col" data-slot="card" data-col="${i}">
            <h3 class="pain-k mono">${esc(c.key)}</h3>
            <p class="pain-stat display molten" data-odo="${esc(c.stat)}"></p>
            <p class="pain-sub mono ash">${esc(c.sub)}</p>
            <blockquote class="quote">
              <p>${esc(c.quote)}</p>
              <cite class="mono ash"><a href="${c.citeHref}" rel="noopener">${esc(c.cite)}</a></cite>
            </blockquote>
            <ul class="pain-bullets mono">
              ${c.bullets.map((b) => html`<li>${esc(b)}</li>`)}
            </ul>
          </div>`,
        )}
      </div>
      ${statusbar(sources, 3, { links: true })}
    </section>
  `);

  const odos = qa(el, "[data-odo]").map((h) => {
    const o = odometer(h.dataset.odo!);
    h.appendChild(o.el);
    return o;
  });
  const cols = qa(el, ".pain-col");
  const rm = reducedMotion();
  // the stat rolls after its column has revealed (§0b: numbers inside their slot)
  onMount(el, (_q, slot) => {
    odos.forEach((o, i) => o.set(rm ? 1 : range(slot(cols[i]), 0, 0.8)));
  });
  return el;
}
