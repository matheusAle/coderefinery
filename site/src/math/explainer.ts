/**
 * A pinned explainer (motion-spec §16): `states × 100 vh` with one persistent
 * layout — the formula across the top, the code the gate evaluates on the left
 * (syntax-coloured, with annotation tints and notes), the card on the right
 * (title, prose with inline code, the rule, a small visual where the design
 * has one), the state rail top-right and the status bar in the bottom row.
 *
 * The formula computes as you scroll (§16.1): each state shows the value it
 * just computed in molten; numbers roll (odometer), the rest of the row is
 * chalk, so earlier substitutions stay chalk and only the newest is molten.
 * Per state t ∈ [0, 1]: 0.00–0.15 the previous row and its annotations cool
 * and fade, the new row cross-fades in, the card swaps (title types, body and
 * rule reveal); 0.15–0.85 the code annotations tint and pop in the order the
 * card describes (§16.2) while the value rolls to its figure; 0.85–1.00 hold.
 * Everything is a function of pin progress, so scrolling back reverses it.
 * Reduced motion: static states, cross-faded. Under 1024 px: stacked.
 */
import { el, esc, fromHTML, html, prose, q, qa } from "../dom";
import { clamp01, easeOutCubic, invalidate, onScroll, pinProgress, range, reducedMotion } from "../motion/scroll";
import { CLIP, SLOTS, entrance, phoneMount, pinnedMount } from "../motion/mount";
import { odometer, type Odometer } from "../motion/odometer";
import { typedTitle, type TypedTitle } from "../motion/typed-title";
import { renderPython } from "../code/python";
import type { Annotation, Gate, MathState, Term } from "./content";

const NARROW = "(max-width: 1023px)";
/** annotations land over the work phase of the state */
const WORK: [number, number] = [0.15, 0.85];

/** A molten term rolls its numbers; a leading "(1 − " stays put so only the substituted value moves. */
function termHTML(t: Term): string {
  if (!t.molten) return `<span class="ex-term">${esc(t.text)}</span>`;
  // a word (DUPLICATE) lands instead of rolling (§16.1)
  if (!/\d/.test(t.text)) return `<span class="ex-term molten ex-land">${esc(t.text)}</span>`;
  const m = t.text.match(/^(\(1 − )(.*)$/);
  const lead = m ? m[1] : "";
  const roll = m ? m[2] : t.text;
  return `<span class="ex-term molten">${lead ? esc(lead) : ""}<span data-roll="${esc(roll)}"></span></span>`;
}

function rowHTML(terms: Term[], label: string): string {
  return `<span class="ex-frow" aria-label="${esc(label)}">${terms.map(termHTML).join(" ")}</span>`;
}

function visualHTML(s: MathState): string {
  switch (s.visual) {
    case "squares":
      return html`<div class="ex-vis vis-squares"><div class="sq-row">${Array.from({ length: 9 }, () => `<span class="sq"></span>`)}</div><p class="mono" data-support><code class="ic">CC = 1 + 8 = 9</code></p></div>`;
    case "coverage":
      return html`<div class="ex-vis vis-coverage"><div class="cov-bar"><span class="cov-fill"></span></div><p class="mono" data-support><code class="ic">cov = 0.20 · (1 − 0.20)³ = 0.512</code></p></div>`;
    case "markers":
      return html`<div class="ex-vis vis-markers">
        <div class="mk mk-before"><span class="mk-v display slag" data-roll="50.5"></span><span class="mk-l mono">before · exit 2</span></div>
        <div class="mk mk-after"><span class="mk-v display ingot">9</span><span class="mk-l mono">after the split</span></div>
      </div>`;
    default:
      return "";
  }
}

interface LineNode {
  el: HTMLElement;
  note: HTMLElement;
  key: string;
}

class Stage {
  readonly el: HTMLElement;
  private kickerN: HTMLElement;
  private dots: HTMLElement[];
  private rows: HTMLElement[];
  private rowRolls: Odometer[][];
  private lines: LineNode[];
  private cards: HTMLElement[];
  private cardTitles: TypedTitle[];
  private statusLines: HTMLElement[];
  private squares: HTMLElement[][];
  private covFill: (HTMLElement | null)[];
  private markers: ({ before: HTMLElement; after: HTMLElement; roll: Odometer } | null)[];
  private rm: boolean;
  /** the state's annotations as steps: [annotation, step, steps] */
  private plan: { a: Annotation; at: number }[][];
  entrance = 0;

  constructor(
    private gate: Gate,
    private snapshot?: number,
  ) {
    this.rm = reducedMotion();
    const st = gate.states;
    const n = st.length;
    this.el = fromHTML(html`<div class="ex-stage">
      <div class="ex-head">
        <p class="kicker">${esc(gate.kicker)} · <span class="ex-n"></span>/${n}</p>
        <div class="ex-rail" aria-hidden="true">${st.map(() => html`<span class="ex-dot"></span>`)}</div>
      </div>
      <p class="ex-formula display">
        ${gate.pre ? rowHTML(gate.pre, gate.pre.map((t) => t.text).join(" ")) : ""}
        ${st.map((s) => rowHTML(s.formula, s.formula.map((t) => t.text).join(" ")))}
      </p>
      <div class="ex-row">
        <figure class="ex-code pane" aria-label="${esc(gate.file)}">
          <div class="pane-bar mono"><span>${esc(gate.file)}</span><span class="ash">python</span></div>
          <div class="cp-lines">
            ${gate.code.map(
              (c, i) => html`<div class="cp-line"><span class="cp-n mono">${i + 1}</span><code class="cp-code mono">${renderPython(c) || " "}</code><span class="cp-note mono"></span></div>`,
            )}
          </div>
        </figure>
        <div class="ex-cards">
          ${st.map(
            (s, i) => html`<article class="ex-card" aria-labelledby="${gate.id}-${i}-h">
              <h3 class="ex-title display" id="${gate.id}-${i}-h">${esc(s.title)}</h3>
              <p class="ex-body">${prose(s.body)}</p>
              <div class="ex-rule box"><span class="mono molten">rule</span><p class="mono">${prose(s.rule)}</p></div>
              ${visualHTML(s)}
            </article>`,
          )}
        </div>
      </div>
      <div class="statusbar ex-status">
        <span class="dot"><span class="ex-st-stack">${st.map((s) => html`<span class="ex-st">${esc(s.status)}</span>`)}</span></span>
        <span class="exit ${gate.exit.tone}">${esc(gate.exit.code)}</span>
      </div>
    </div>`);

    this.kickerN = q(this.el, ".ex-n");
    this.dots = qa(this.el, ".ex-dot");
    this.rows = qa(this.el, ".ex-frow");
    this.rowRolls = this.rows.map((r) =>
      qa(r, "[data-roll]").map((h) => {
        const o = odometer(h.dataset.roll!);
        h.appendChild(o.el);
        return o;
      }),
    );
    this.lines = qa(this.el, ".cp-line").map((l) => ({ el: l, note: q(l, ".cp-note"), key: "" }));
    this.cards = qa(this.el, ".ex-card");
    this.cardTitles = this.cards.map((c) => typedTitle(q(c, ".ex-title")));
    this.statusLines = qa(this.el, ".ex-st");
    this.squares = this.cards.map((c) => qa(c, ".sq"));
    this.covFill = this.cards.map((c) => c.querySelector<HTMLElement>(".cov-fill"));
    this.markers = this.cards.map((c) => {
      const m = c.querySelector<HTMLElement>(".vis-markers");
      if (!m) return null;
      const h = q(m, "[data-roll]");
      const roll = odometer(h.dataset.roll!);
      h.appendChild(roll.el);
      return { before: q(m, ".mk-before"), after: q(m, ".mk-after"), roll };
    });
    // annotation timing: in the order the card describes; equal `step`s land together
    this.plan = st.map((s) => {
      const steps = s.annotations.map((a, i) => a.step ?? i);
      const count = Math.max(...steps, 0) + 1;
      return s.annotations.map((a, i) => ({ a, at: WORK[0] + ((WORK[1] - WORK[0] - 0.08) * steps[i]) / Math.max(1, count - 1 || 1) }));
    });

    if (snapshot !== undefined) {
      this.el.classList.add("snapshot");
      this.apply((snapshot + 0.9) / n);
    }
    // the formula stays on one line: if the longest row is wider than the stage, the formula's size scales down
    const formula = q(this.el, ".ex-formula");
    const fit = (): void => {
      formula.style.removeProperty("--fit");
      const avail = formula.clientWidth;
      const widest = Math.max(...this.rows.map((r) => r.scrollWidth));
      if (avail > 0 && widest > avail) formula.style.setProperty("--fit", (avail / widest).toFixed(3));
    };
    requestAnimationFrame(fit);
    document.fonts?.ready.then(fit);
    window.addEventListener("resize", fit);
  }

  /**
   * Phone (§17.2): a stacked state card mounts in the entrance order — kicker, the card title typing, the formula
   * and code text (the value rolls once, `play`), the card's words, then the boxes and the support text, the
   * status bar last. `m` is the card's mount progress (mount.ts `phoneMount`).
   */
  setPhoneEntrance(m: number): void {
    const i = this.snapshot;
    if (i === undefined) return;
    this.el.style.setProperty("--ent-k", range(m, 0.4, 0.43).toFixed(3));
    this.cardTitles[i].set(range(m, 0.4, 0.56));
    const r = easeOutCubic(range(m, 0.52, 0.57));
    this.el.style.setProperty("--ent-r", r.toFixed(3));
    this.el.classList.toggle("ent-open", r >= 1);
    this.cards[i].style.setProperty("--rv", range(m, 0.56, 0.61).toFixed(3));
    this.el.style.setProperty("--ent-box", range(m, 0.62, 0.68).toFixed(3));
    this.el.style.setProperty("--ent-sup", range(m, 0.72, 0.76).toFixed(3));
    this.el.style.setProperty("--ent-s", range(m, 0.88, 0.9).toFixed(3));
    if (m >= 0.52) this.play();
  }

  /** Stacked state on a phone (§17.2): its formula value rolls once, inside the formula's block step. */
  play(): void {
    if (this.snapshot === undefined || this.el.dataset.played) return;
    this.el.dataset.played = "1";
    const pre = this.gate.pre ? 1 : 0;
    const rolls = this.rowRolls[this.snapshot + pre] ?? [];
    const lands = Array.from(this.rows[this.snapshot + pre]?.querySelectorAll<HTMLElement>(".ex-land") ?? []);
    const t0 = performance.now();
    const tick = (now: number): void => {
      const t = Math.min(1, (now - t0) / 900);
      const e = 1 - Math.pow(1 - t, 3);
      for (const r of rolls) r.set(e);
      for (const w of lands) w.style.setProperty("--lp", range(t, 0.6, 0.9).toFixed(3));
      if (t < 1) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }

  /** pin progress p ∈ [0, 1] */
  apply(p: number): void {
    const st = this.gate.states;
    const n = st.length;
    const pc = clamp01(p);
    const s = Math.min(n - 1, Math.floor(pc * n));
    const t = this.snapshot !== undefined ? 0.9 : clamp01(pc * n - s);
    const enter = s === 0 ? 1 : easeOutCubic(range(t, 0, 0.15));
    const work = this.rm || this.snapshot !== undefined ? 1 : easeOutCubic(range(t, ...WORK));

    this.kickerN.textContent = String(s + 1);
    this.dots.forEach((d, i) => d.style.setProperty("--w", (i === s ? enter : s > 0 && i === s - 1 ? 1 - enter : 0).toFixed(3)));
    this.statusLines.forEach((l, i) => {
      l.style.setProperty("--o", (i === s ? enter : s > 0 && i === s - 1 ? 1 - enter : 0).toFixed(3));
      l.classList.toggle("cur", i === s);
    });

    // ---- formula: the row of the active state (and the written-out formula before the first substitution) ----
    const pre = this.gate.pre ? 1 : 0;
    const swapIn = s === 0 ? (pre ? (this.rm || this.snapshot !== undefined ? 1 : range(t, 0.08, 0.16)) : 1) : enter;
    this.rows.forEach((r, ri) => {
      const k = ri - pre; // state index of the row (-1 = the written formula)
      let o = 0;
      if (k === s) o = swapIn;
      else if (k === s - 1) o = 1 - swapIn;
      r.style.setProperty("--o", o.toFixed(3));
      r.classList.toggle("on", o > 0.01);
      if (k >= 0) {
        for (const roll of this.rowRolls[ri]) roll.set(k < s ? 1 : k === s ? work : 0);
        r.querySelectorAll<HTMLElement>(".ex-land").forEach((w) => w.style.setProperty("--lp", (k < s ? 1 : k === s ? range(work, 0.6, 0.8) : 0).toFixed(3)));
      }
    });

    // ---- code annotations (§16.2): the active state's, in order; the previous state's cool during the enter ----
    const target = new Map<number, { a: Annotation; p: number }>();
    if (s > 0 && enter < 1) for (const { a } of this.plan[s - 1]) target.set(a.line, { a, p: 1 - enter });
    for (const { a, at } of this.plan[s]) {
      const pa = this.rm || this.snapshot !== undefined ? 1 : easeOutCubic(range(t, at, at + 0.08));
      if (pa > 0 || !target.has(a.line)) target.set(a.line, { a, p: pa });
    }
    this.lines.forEach((ln, i) => {
      const hit = target.get(i + 1);
      const p = hit ? hit.p : 0;
      const key = hit && p > 0 ? `${hit.a.tint ?? ""}|${hit.a.note ?? ""}|${hit.a.tone ?? ""}|${p.toFixed(3)}` : "";
      if (key === ln.key) return;
      ln.key = key;
      ln.el.dataset.tint = hit && p > 0 ? hit.a.tint ?? "" : "";
      ln.el.style.setProperty("--tp", p.toFixed(3));
      const note = hit && p > 0 ? hit.a.note ?? "" : "";
      if (ln.note.textContent !== note) ln.note.textContent = note;
      ln.note.dataset.tone = hit?.a.tone ?? "";
      ln.note.style.setProperty("--np", note ? p.toFixed(3) : "0");
    });

    // ---- cards: old content drops 8 px and fades; the new one appears at t = 0.10, title types, body and rule reveal ----
    this.cards.forEach((c, i) => {
      if (i === s) {
        // the first card exists only from the title's slot on (nothing but the slide before m = 0.40, §0c)
        const on = this.rm || (s === 0 && (this.entrance >= SLOTS.title[0] || this.snapshot !== undefined || pc > 0)) || t >= 0.1;
        c.classList.toggle("in", on);
        c.style.setProperty("--o", "1");
        c.style.setProperty("--y", "0");
        const tp = this.rm || this.snapshot !== undefined ? 1 : s === 0 ? Math.max(range(this.entrance, ...SLOTS.title), range(pc * n, 0, 0.1)) : range(t, 0.1, 0.24);
        this.cardTitles[i].set(tp);
        c.style.setProperty(
          "--rv",
          (this.rm || this.snapshot !== undefined ? 1 : s === 0 ? Math.max(range(this.entrance, SLOTS.card[0], SLOTS.card[0] + CLIP), range(pc * n, 0.05, 0.18)) : range(t, 0.16, 0.3)).toFixed(3),
        );
      } else if (i === s - 1) {
        const leave = this.rm ? 1 : range(t, 0, 0.1);
        c.classList.toggle("in", leave < 1);
        c.style.setProperty("--o", (1 - leave).toFixed(3));
        c.style.setProperty("--y", (-8 * leave).toFixed(2));
      } else c.classList.remove("in");
      // the card's visual moves in step with the code annotations
      const w = i < s ? 1 : i === s ? work : 0;
      this.squares[i].forEach((sq, k) => sq.classList.toggle("lit", w >= (k + 0.5) / 9));
      this.covFill[i]?.style.setProperty("--f", (0.2 * w).toFixed(4));
      const mk = this.markers[i];
      if (mk) {
        mk.roll.set(range(w, 0, 0.5));
        mk.before.style.setProperty("--o", range(w, 0, 0.15).toFixed(3));
        mk.after.style.setProperty("--o", range(w, 0.6, 0.8).toFixed(3));
      }
    });

    // first-state mount during the entrance (§0c slots): kicker + rail fade; formula and code pane reveal as the
    // primary visual; the status bar fades last; complete once the pin has started
    const ent = this.rm || this.snapshot !== undefined || pc > 0 ? 1 : this.entrance;
    this.el.style.setProperty("--ent-k", range(ent, ...SLOTS.kicker).toFixed(3));
    const entR = easeOutCubic(range(ent, SLOTS.visual[0], SLOTS.visual[0] + CLIP));
    this.el.style.setProperty("--ent-r", entR.toFixed(3));
    this.el.classList.toggle("ent-open", entR >= 1);
    // text before its box (§0c): the panes' surfaces and seams arrive after their words, the support text after that
    // the boxes (code pane surface, the card's rule, the rule box seam, the visual's slab) after the card's words
    this.el.style.setProperty("--ent-box", range(ent, 0.62, 0.68).toFixed(3));
    // support ≥ 0.02 of e after the boxes are drawn (box ends at m 0.68 ↔ e 0.824; support from m 0.72 ↔ e 0.856)
    this.el.style.setProperty("--ent-sup", range(ent, 0.72, 0.76).toFixed(3));
    this.el.style.setProperty("--ent-s", range(ent, ...SLOTS.status).toFixed(3));
  }
}

export function mountExplainer(gate: Gate): HTMLElement {
  const section = el("section", "explainer");
  section.id = gate.id;
  section.setAttribute("aria-label", gate.kicker);
  const n = gate.states.length;
  section.style.setProperty("--states", String(n));

  function build(): void {
    section.querySelectorAll(".ex-sticky, .deck-hold, .ex-stage").forEach((x) => x.remove());
    if (window.matchMedia(NARROW).matches) {
      section.classList.add("stacked");
      const stages = gate.states.map((_, i) => new Stage(gate, i));
      // each state is a deck card (§17.4): sticky inside its own hold, which carries the runway
      stages.forEach((s) => {
        const hold = el("div", "deck-hold");
        hold.appendChild(s.el);
        section.appendChild(hold);
      });
      if (!reducedMotion()) {
        // each state card mounts on its own entrance (§17.2) and stays mounted
        const best = stages.map(() => 0);
        stages.forEach((s) => s.setPhoneEntrance(0));
        onScroll(() => {
          const vh = window.innerHeight;
          stages.forEach((s, i) => {
            if (best[i] >= 0.9) return;
            const e = Math.max(0, Math.min(1, (vh - s.el.getBoundingClientRect().top) / (0.6 * vh)));
            const m = Math.max(best[i], phoneMount(e));
            if (m === best[i] && m > 0) return;
            best[i] = m;
            s.setPhoneEntrance(m);
          });
        });
      }
      return;
    }
    section.classList.remove("stacked");
    const sticky = el("div", "ex-sticky");
    section.appendChild(sticky);
    if (reducedMotion()) {
      // static frames, cross-faded by scroll
      const stages = gate.states.map((_, i) => new Stage(gate, i));
      stages.forEach((s) => sticky.appendChild(s.el));
      onScroll(() => {
        const p = clamp01(pinProgress(section));
        const idx = Math.min(n - 1, Math.floor(p * n));
        const t = p * n - idx;
        stages.forEach((s, i) => {
          s.el.style.opacity = (i === idx ? 1 : i === idx + 1 ? range(t, 0.85, 1) : 0).toFixed(3);
        });
      });
      return;
    }
    const stage = new Stage(gate);
    sticky.appendChild(stage.el);
    onScroll(() => {
      stage.entrance = pinnedMount(entrance(section));
      stage.apply(pinProgress(section));
    });
  }
  build();
  window.matchMedia(NARROW).addEventListener("change", () => {
    build();
    invalidate();
  });
  return section;
}
