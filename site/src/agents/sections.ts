/**
 * The "for agents" page (A0–A4, motion-spec §15). A text page: almost
 * everything is tier 2 (appears at once); the structure does the moving —
 * seams draw, loop arrows draw, cells open. Copy is verbatim from
 * designs/reference/agents-reference.html.
 */
import { esc, fromHTML, html, pre, q, qa, prose } from "../dom";
import { mountSlagField } from "../gl/slag";
import { head, loopPanes, statusbar } from "../sections/common";
import { topbar } from "../sections/topbar";

/* ---------- A0 · hero ---------- */
const PROPS: [string, string][] = [
  ["repeatable", "same tree in, same number out. byte-identical to the reference tools on 36/36 cases."],
  ["local and fast", "seconds on `--changed`. the agent runs it after every edit, not once before the commit."],
  ["precise", "file:line, the metric, the threshold it crossed. nothing to interpret."],
  ["un-gameable", "the only way to change the number is to change the code or the tests."],
  ["machine-readable", "exit codes for the hook, `.metrics/*.edn` for tooling, stdout for the transcript."],
];

/** The A0 slag field at 50 % intensity (§15); hosted on the full-width wrapper. */
export function attachAgentsField(wrapper: HTMLElement): void {
  const host = document.createElement("div");
  host.className = "gl-host";
  host.setAttribute("aria-hidden", "true");
  wrapper.insertBefore(host, wrapper.firstChild);
  mountSlagField(wrapper, host, 0.5);
}

export function mountAgentsHero(): HTMLElement {
  const el = fromHTML(html`
    <header class="section a-hero" id="top">
      ${topbar("agents")}
      <div class="a-hero-head">
        ${head("§ for agents · why determinism", "The agent gets the same answer every time. So it learns the rule, not the reviewer.", "a-hero-h")}
      </div>
      <div class="a-two">
        <p class="a-lede" data-slot="card">An agent optimises for the signal it gets. A reviewer it can persuade teaches it to persuade. A formula it cannot move teaches it to move the code. coderefinery gives every edit a number, a threshold, a line, and an exit code. Same input, same verdict, on your laptop and in CI.</p>
        <div class="a-props box" data-slot="visual" role="list">
          ${PROPS.map(([k, v]) => html`<div class="a-prop" role="listitem"><span class="mono molten">${esc(k)}</span><span class="mono">${prose(v)}</span></div>`)}
        </div>
      </div>
      ${statusbar("no model in the loop · the gate does not read the PR description", 0, { dot: true })}
    </header>
  `);
  return el;
}

/* ---------- A1 · the loop ---------- */
const STEPS: [string, string, string][] = [
  ["1 · edit", "the agent changes code or tests. nothing else is in scope.", "ash"],
  ["2 · gate", "crap · dry · mutate · tests, on `--changed` only. seconds.", "molten"],
  ["3 · verdict", "`exit 2` · report · `exit 3` · `exit 0`, with file:line and the number.", "ember"],
  ["4 · fix the code", "split, merge, add the assertion, delete the dead test. never edit the gate.", "ingot"],
  ["5 · commit", "the hook re-runs the gate. `exit 0` is the only way through.", "ingot"],
];

export function mountLoop(): HTMLElement {
  const el = fromHTML(html`
    <section class="section a-loop" aria-labelledby="a-loop-h">
      ${head("§01 · the loop the agent lives in", "Edit. Gate. Read the exit code. Fix the code, not the gate.", "a-loop-h")}
      <ol class="loop" aria-label="the loop">
        ${STEPS.map(
          ([k, v, tone], i) => html`${i > 0 ? html`<li class="loop-arrow" aria-hidden="true" style="--i:${i}"><svg viewBox="0 0 24 12" width="24" height="12"><path d="M1 6h20M16 1l5 5-5 5" pathLength="1" fill="none" stroke="currentColor" stroke-width="1.2"/></svg></li>` : ""}
            <li class="loop-step box tone-${tone}" data-slot="card" style="--i:${i}">
              <span class="loop-k mono ${tone}"><strong>${esc(k)}</strong></span>
              <span class="loop-v mono">${prose(v)}</span>
            </li>`,
        )}
      </ol>
      ${loopPanes()}
      ${statusbar("a loop that closes in seconds runs hundreds of times a day. that is where behaviour comes from.", 0, { dot: true })}
    </section>
  `);
  // the arrows draw as strokes after the box to their left: one observer on the list
  const loop = q(el, ".loop");
  const io = new IntersectionObserver(
    (entries) => {
      if (!entries.some((e) => e.isIntersecting)) return;
      loop.classList.add("go");
      io.disconnect();
    },
    { threshold: 0.3 },
  );
  io.observe(loop);
  return el;
}

/* ---------- A2 · what the agent reads ---------- */
interface VerdictRow {
  gate: string;
  tone: "slag" | "ember" | "ingot";
  sees: string[];
  does: string;
}
const VERDICTS: VerdictRow[] = [
  { gate: "crap · exit 2", tone: "slag", sees: ["Function  Namespace  CC  Cov%  CRAP", "price     billing     9  20.0  50.5", "CRAP threshold exceeded: 50.5 > 9.9999"], does: "reduce under `10` by splitting, by testing, or both. split first when `CC ≥ 10`: no coverage can save it." },
  { gate: "crap · N/A", tone: "ember", sees: ["Cov% N/A · stderr: Python tests exited 1"], does: "the tests or the coverage run failed. fix that before trusting the gate." },
  { gate: "dry · report", tone: "ember", sees: ["DUPLICATE score=0.91", "  billing/pricing.py:1-6", "  billing/quotes.py:8-13"], does: "eliminate every pair at `0.8` or above: one body, two names. exit is `0`; the skill reads the report." },
  { gate: "mutate · exit 3", tone: "slag", sees: ["SURVIVED  pricing.py:6  0 → 1", "SURVIVED  pricing.py:9  1 → 0"], does: "add or sharpen the assertion that would fail. if no honest test can kill it, the code does nothing: delete it." },
  { gate: "mutate · UNCOVERED", tone: "ember", sees: ["UNCOVERED pricing.py:14  and → or"], does: "no test executes that line. write the test first; the mutant runs next time." },
  { gate: "mutate · exit 2", tone: "slag", sees: ["Baseline failed: 1 test failed before any mutation"], does: "the suite is red on its own. fix the tests, then run the gate." },
  { gate: "tests · exit 3", tone: "slag", sees: ["test_settle_called   kills 0   unique 0", "test_quote_gold      kills 5   unique 0   shadowed by test_tier_rate[gold]"], does: "zero kills: delete. zero unique kills: delete if the shadow is at the same or a lower level; keep if it guards a seam." },
  { gate: "all · exit 0", tone: "ingot", sees: ["0 functions above the gate · No duplicate candidates found. · killed 12 survived 0"], does: "commit. the hook agrees." },
];

export function mountVerdicts(): HTMLElement {
  return fromHTML(html`
    <section class="section a-verdicts" aria-labelledby="a-verdicts-h">
      ${head("§02 · what the agent reads, and what it does next", "Every verdict names the next move.", "a-verdicts-h")}
      <div class="vtable box" data-slot="visual" role="table" aria-label="verdicts and the next move">
        <div class="vrow vhead" role="row">
          <span role="columnheader" class="mono"><strong>gate · exit</strong></span>
          <span role="columnheader" class="mono"><strong>what the agent sees</strong></span>
          <span role="columnheader" class="mono"><strong>what the skill tells it to do</strong></span>
        </div>
        ${VERDICTS.map(
          (r) => html`<div class="vrow" role="row">
            <span role="cell" class="mono ${r.tone}"><strong>${esc(r.gate)}</strong></span>
            <span role="cell" class="mono vsees">${r.sees.map((s) => html`<span>${pre(s)}</span>`)}</span>
            <span role="cell" class="vdoes">${prose(r.does)}</span>
          </div>`,
        )}
      </div>
      ${statusbar("output is sorted worst first and never depends on time, locale or machine. the transcript is the same in CI.", 0, { dot: true })}
    </section>
  `);
}

/* ---------- A3 · reliable tests ---------- */
/** [mutant, test it demands, index of the rule row it pairs with on hover] */
const MUTANTS: [string, string, number][] = [
  ["< → ≤", "a case exactly on the boundary. the equality is the bug.", 0],
  ["0 → 1", "a case where the result must be zero, not merely small.", 2],
  ["and → or", "each operand alone: one true, one false, assert the branch is not taken.", 3],
  ["− → +", "an expected value that changes sign or direction, not one that happens to match.", 0],
  ["true → false", "both branches observed through the result, not through a spy.", 1],
  ["delete not", "the negated input, asserted on the outcome.", 5],
  ["== → ≠", "a case that is equal and a case that is not, with different expected results.", 0],
  ["delete −x", "a negative input whose sign reaches the output.", 5],
];
const RULES: [string, string][] = [
  ["write the failing test before the code", "mutate: a test written after the fact that cannot fail has zero kills"],
  ["test behaviour, not implementation", "tests: a spy on a private call kills nothing; it is listed for deletion"],
  ["if it would pass with every function returning nothing, throw it out", "tests: zero kills, `exit 3`"],
  ["split before you test", "crap: `CC ≥ 10` cannot pass at any coverage"],
  ["one body, two names is one body", "dry: `0.8` on the normalised tree, names erased"],
  ["delete what no honest test can kill", "mutate: a survivor nobody can kill marks dead code"],
  ["test the diff, not the repo", "`--changed` on every gate; the hook runs in seconds"],
];

export function mountReliable(): HTMLElement {
  const el = fromHTML(html`
    <section class="section a-reliable type-step" aria-labelledby="a-reliable-h">
      ${head("§03 · reliable tests, mechanically", "The mutant tells the agent which test to write.", "a-reliable-h")}
      <p class="lede a-lede-wide" data-slot="card">A surviving mutant is a precise specification of a missing assertion. The agent does not have to guess what a good test is; it reads the edit that went unnoticed and writes the case that would notice it.</p>
      <div class="a-two-tables">
        <div class="mtable box" data-slot="card" role="table" aria-label="surviving mutant and the test it demands">
          <div class="pane-bar"><span>surviving mutant</span><span class="ash">the test it demands</span></div>
          ${MUTANTS.map(
            ([m, t, pair]) => html`<div class="mrow" role="row" data-pair="${pair}" tabindex="0">
              <span role="cell" class="mono molten">${esc(m)}</span><span role="cell" class="mono">${esc(t)}</span>
            </div>`,
          )}
        </div>
        <div class="rtable box" data-slot="card" role="table" aria-label="what the skill says and what enforces it">
          <div class="pane-bar"><span>what the skill says</span><span class="ash">what enforces it</span></div>
          ${RULES.map(
            ([r, e], i) => html`<div class="rrow" role="row" data-rule="${i}" tabindex="0">
              <span role="cell">${esc(r)}</span><span role="cell" class="mono ash">${prose(e)}</span>
            </div>`,
          )}
        </div>
      </div>
      ${statusbar("the skill is advice. the exit code is the enforcement. the research found no other way that survives contact with an agent.", 0, { dot: true })}
    </section>
  `);
  // hover / focus a mutant row: its rule row lights a molten seam, and vice versa
  const mrows = qa(el, ".mrow");
  const rrows = qa(el, ".rrow");
  const clear = () => {
    mrows.forEach((r) => r.classList.remove("lit"));
    rrows.forEach((r) => r.classList.remove("lit"));
  };
  const light = (rule: number, from?: HTMLElement) => {
    clear();
    rrows[rule]?.classList.add("lit");
    if (from) from.classList.add("lit");
    else mrows.filter((m) => Number(m.dataset.pair) === rule).forEach((m) => m.classList.add("lit"));
  };
  let touched = false;
  document.addEventListener("keydown", () => (touched = false));
  for (const m of mrows) {
    const on = () => light(Number(m.dataset.pair), m);
    // no hover on touch (§17.2); keyboard focus still pairs the rows
    m.addEventListener("pointerenter", (e) => e.pointerType !== "touch" && on());
    m.addEventListener("pointerdown", (e) => (touched = e.pointerType === "touch"));
    // a tap focuses the row; on touch that must not pair (§17.2 no hover), keyboard focus still does
    m.addEventListener("focus", () => !touched && on());
    m.addEventListener("pointerleave", clear);
    m.addEventListener("blur", clear);
  }
  for (const r of rrows) {
    const on = () => light(Number(r.dataset.rule));
    // no hover on touch (§17.2); keyboard focus still pairs the rows
    r.addEventListener("pointerenter", (e) => e.pointerType !== "touch" && on());
    r.addEventListener("pointerdown", (e) => (touched = e.pointerType === "touch"));
    r.addEventListener("focus", () => !touched && on());
    r.addEventListener("pointerleave", clear);
    r.addEventListener("blur", clear);
  }
  return el;
}

/* ---------- A4 · same everywhere + CTA ---------- */
const CELLS: [string, string][] = [
  ["no runtime", "one Go binary, stdlib only, `7.2 MB`. nothing to install in the agent's sandbox, nothing to version-skew in CI."],
  ["no clock, no dice", "output never depends on time, locale, machine or ordering. two runs diff clean. the agent can diff them too."],
  ["no second config", "the hook, the CI job and the agent's own run use the same three commands and read the same `.metrics/`."],
];

export function mountEverywhere(): HTMLElement {
  const el = fromHTML(html`
    <section class="section a-everywhere" aria-labelledby="a-every-h">
      ${head("§04 · same everywhere", "Your laptop, the hook, CI: one binary, one answer.", "a-every-h")}
      <div class="a-cells" role="list">
        ${CELLS.map(
          ([k, v], i) => html`<div class="a-cell box" data-slot="card" role="listitem" style="--i:${i}">
            <span class="mono molten"><strong>${esc(k)}</strong></span>
            <p class="a-cell-p">${prose(v)}</p>
          </div>`,
        )}
      </div>
      <div class="cta box molten-seam" data-slot="card">
        <div class="cta-left">
          <p class="cta-h display">Give the agent a number it cannot argue with.</p>
          <p class="a-install mono"><span class="prompt">$</span> <code class="ic">npx skills add coderefinery</code> <span class="ash a-install-s">· then add the hook (two minutes)</span></p>
        </div>
        <a class="cta-btn mono" href="/#hook">see the hook →</a>
      </div>
      ${statusbar("byte-identical to crapper, dryer and mutator · 36/36 · measured on 12 repos", 0, { dot: true })}
    </section>
  `);
  const cells = qa(el, ".a-cell");
  const host = q(el, ".a-cells");
  const io = new IntersectionObserver(
    (entries) => {
      if (!entries.some((e) => e.isIntersecting)) return;
      cells.forEach((c) => c.classList.add("open"));
      io.disconnect();
    },
    { threshold: 0.3 },
  );
  io.observe(host);
  return el;
}
