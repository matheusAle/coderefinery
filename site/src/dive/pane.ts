/**
 * The walkthrough stage (K1–K4, motion-spec §13.3). Persistent objects: the
 * heading, the hint boxes, the agent's session pane (left), the outputs pane
 * (right: `before`, `what the agent changed`, `after`), the depth gauge, the
 * progress bar and the status bar. Between states their content swaps; the
 * layout never moves. `apply(p)` is the only way state gets in: every property
 * is a function of pin progress, so scrubbing backwards reverses everything
 * except the one-shot verdict flash. Nodes are mounted once.
 *
 * Per state t ∈ [0, 1]: 0–0.12 enter (sections cross-fade, the command that
 * typed in the bridge is already there); the gate's output lands line by line
 * and the `before` block reveals with it (numbers roll); the agent's reading
 * reveals; each tool call types its header, reveals its body and lands `done`
 * while `what the agent changed` reveals hunk by hunk; the re-run types and its
 * lines land, `after` reveals with them; the hint arrives in 0.72–0.85; the
 * bridge (0.85–1) types the next state's command at the bottom of the session.
 * Every exit line in either pane fires the verdict flash once (§14.4).
 */
import { esc, el, fromHTML, html, prose, q, qa } from "../dom";
import { odometer, type Odometer } from "../motion/odometer";
import { typer, type Typer } from "../motion/typer";
import { typedTitle, type TypedTitle } from "../motion/typed-title";
import { CLIP, SLOTS } from "../motion/mount";
import { clamp01, easeOutCubic, lerp, range } from "../motion/scroll";
import { createPass, onFrame, type ShaderPass } from "../gl/shader";
import { FLASH_FRAG, FURNACE_FRAG } from "../gl/shaders";
import { STATES, type OutBlock, type OutRow, type SessionItem } from "./states";

export interface DiveOptions {
  /** Static snapshot: render one settled state, no shaders, no scrubbing. */
  snapshot?: number;
  shaders?: boolean;
}

/** one timed node: shown from `at`; a typer types over [at, at + dur]; an odometer rolls over [at, at + dur] */
interface Beat {
  el: HTMLElement;
  at: number;
  dur: number;
  typer?: Typer;
  odo?: Odometer;
  exit?: { pane: 0 | 1; key: string; tint: number };
  done?: HTMLElement;
  /** K1's opening content mounts with the entrance (§0c Exceptions): the pinned-mount m at which it reveals */
  ent?: number;
}

interface Section {
  el: HTMLElement;
  beats: Beat[];
  bridge?: { el: HTMLElement; typer: Typer };
}

/* ---------- the per-state schedule (fractions of the state) ---------- */
const LINE_STEP = 0.035;
const BRIDGE: [number, number] = [0.85, 1];
interface Schedule {
  gateFrom: number; // the gate's output starts landing
  agentAt: number;
  toolsFrom: number;
  toolsTo: number;
  rerunAt: number;
}
const SCHEDULE: Schedule = { gateFrom: 0.12, agentAt: 0.34, toolsFrom: 0.42, toolsTo: 0.6, rerunAt: 0.62 };
/**
 * K1 begins assembled (§0c Exceptions): its command, the gate's output, the agent's reading and the `before` block
 * mount with the entrance, so its pin starts at the tool calls.
 */
const K1_SCHEDULE: Schedule = { gateFrom: 0, agentAt: 0, toolsFrom: 0.08, toolsTo: 0.36, rerunAt: 0.44 };
/** K1's entrance words, in pinned-mount m (after the title types, before the panes' surfaces at 0.67) */
const K1_WORDS_FROM = 0.56;
const K1_WORD_STEP = 0.022;
const K1_BOX: [number, number] = [0.67, 0.71];
const K1_SUPPORT: [number, number] = [0.73, 0.76];

export class DivePane {
  readonly el: HTMLElement;
  private kickerOdo: Odometer;
  private titles: HTMLElement[];
  private typedTitles: TypedTitle[];
  private hints: HTMLElement[];
  /** status-bar lines: the four verdicts, then the four exit codes (index % 4 = state) */
  private statusLines: HTMLElement[];
  private session: Section[] = [];
  private outputs: Section[] = [];
  private gaugeFill: HTMLElement;
  private gaugeMarks: HTMLElement[];
  private progress: HTMLElement;
  private furnace: ShaderPass | null = null;
  private flashes: { pass: ShaderPass | null; start: number; tint: number }[] = [];
  private fired = new Set<string>();
  private depth = 0;
  private entrance = 0;
  private visible = true;
  private snapshot: number | undefined;

  constructor(opts: DiveOptions = {}) {
    this.snapshot = opts.snapshot;
    this.el = fromHTML(html`
      <div class="dive-stage">
        <div class="furnace-host" aria-hidden="true"></div>
        <div class="heat-glow" aria-hidden="true"></div>
        <div class="gauge" aria-hidden="true">
          <span class="gauge-l">surface</span>
          <div class="gauge-track"><div class="gauge-fill"></div>
            ${[0, 1, 2, 3].map((i) => html`<span class="gauge-mark" style="--i:${i}"></span>`)}
          </div>
          <span class="gauge-l">core</span>
        </div>
        <div class="dive-head">
          <div class="dive-titles">
            <p class="dive-kicker mono molten">▼ inside the gate · <span class="k-n"></span>/04 · <span class="k-gates">${STATES.map((s) => html`<span class="k-gate">${s.key}</span>`)}</span></p>
            <div class="dive-title-stack">
              ${STATES.map((s) => html`<h2 class="dive-title display" id="k${s.n}-h">${esc(s.title)}</h2>`)}
            </div>
          </div>
          <div class="hint-stack">
            ${STATES.map(
              (s) => html`<aside class="hint" aria-labelledby="k${s.n}-h">
                <p class="hint-k mono">${esc(s.hintKicker)}</p>
                <p class="hint-p">${prose(s.hint)}</p>
              </aside>`,
            )}
          </div>
        </div>
        <div class="dive-row">
          <section class="term term-session" aria-label="the agent's session">
            <div class="flash-host" aria-hidden="true"></div>
            <div class="pane-bar mono"><span>claude code · ~/work/billing</span><span class="ingot">● live</span></div>
            <div class="term-stack">${STATES.map(() => html`<div class="term-sec"></div>`)}</div>
          </section>
          <section class="term term-outputs" aria-label="outputs: before, what the agent changed, after">
            <div class="flash-host" aria-hidden="true"></div>
            <div class="term-stack">${STATES.map(() => html`<div class="term-sec"></div>`)}</div>
          </section>
        </div>
        <div class="statusbar dive-status">
          <span class="dot"><span class="dive-st-stack">${STATES.map((s) => html`<span class="dive-st">${esc(s.status.verdict)}</span>`)}</span></span>
          <span class="dive-st-stack">${STATES.map((s) => html`<span class="dive-st exit ${s.status.tone}">${esc(s.status.exit)}</span>`)}</span>
        </div>
        <div class="dive-progress" aria-hidden="true"><div class="dive-progress-done"></div></div>
      </div>
    `);

    this.kickerOdo = odometer("01");
    q(this.el, ".k-n").appendChild(this.kickerOdo.el);
    this.titles = qa(this.el, ".dive-title");
    this.statusLines = qa(this.el, ".dive-st");
    this.typedTitles = this.titles.map((h) => typedTitle(h));
    this.hints = qa(this.el, ".hint");
    this.gaugeFill = q(this.el, ".gauge-fill");
    this.gaugeMarks = qa(this.el, ".gauge-mark");
    this.progress = q(this.el, ".dive-progress-done");

    const sessHosts = qa(this.el, ".term-session .term-sec");
    const outHosts = qa(this.el, ".term-outputs .term-sec");
    STATES.forEach((st, si) => {
      const sec = this.buildSession(sessHosts[si], st.session, si);
      this.session.push(sec);
      this.outputs.push(this.buildOutputs(outHosts[si], st.before, st.changed, st.after, sec.beats, si));
    });

    // ---- shaders ----
    if (opts.shaders && this.snapshot === undefined) {
      this.furnace = createPass(q(this.el, ".furnace-host"), FURNACE_FRAG, "furnace");
      this.flashes = qa(this.el, ".flash-host").map((h, i) => ({ pass: createPass(h, FLASH_FRAG, i === 0 ? "verdict flash (session)" : "verdict flash (outputs)"), start: -1, tint: 1 }));
      const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
      onFrame((t) => {
        if (!this.visible) return;
        if (this.furnace?.alive) {
          this.furnace.uniform("u_intensity", 0.3 + 0.7 * this.depth);
          this.furnace.uniform("u_dpr", dpr);
          this.furnace.draw(t);
        }
        for (const f of this.flashes) {
          if (!f.pass?.alive || f.start < 0) continue;
          const s = (performance.now() - f.start) / 240;
          if (s > 1) {
            f.start = -1;
            f.pass.uniform("u_intensity", 0);
            f.pass.draw(t);
            continue;
          }
          f.pass.uniform("u_sweep", s);
          f.pass.uniform("u_tint", f.tint);
          f.pass.uniform("u_intensity", 1);
          f.pass.draw(t);
        }
      });
    }

    if (this.snapshot !== undefined) {
      this.el.classList.add("snapshot");
      this.apply((this.snapshot + 0.84) / STATES.length);
      // reduced motion / stacked: nothing types, no bridge, titles complete
      for (const sec of [...this.session, ...this.outputs]) {
        for (const b of sec.beats) {
          b.typer?.set(1);
          b.typer?.cursor(false);
          b.odo?.set(1);
        }
        if (sec.bridge) sec.bridge.el.style.display = "none";
      }
      this.typedTitles.forEach((t) => t.set(1));
    }
  }

  /* ---------- building ---------- */

  private buildSession(host: HTMLElement, items: SessionItem[], si: number): Section {
    const beats: Beat[] = [];
    const sc = si === 0 ? K1_SCHEDULE : SCHEDULE;
    let cursor = sc.gateFrom;
    let phase: "gate" | "tools" | "rerun" = "gate";
    const tools = items.filter((i) => i.kind === "tool").length;
    const toolSpan = (sc.toolsTo - sc.toolsFrom) / Math.max(1, tools);
    // K1: everything before the first tool call is entrance content
    let entK = 0;
    const entrance = (b: Beat): Beat => {
      if (si === 0 && phase === "gate") {
        b.at = 0;
        b.ent = K1_WORDS_FROM + entK++ * K1_WORD_STEP;
      }
      return b;
    };
    let toolIndex = 0;
    let exits = 0;
    items.forEach((it, idx) => {
      if (it.kind === "cmd") {
        const row = el("div", "tl tl-cmd mono");
        row.appendChild(el("span", "prompt", "$"));
        row.appendChild(document.createTextNode(" "));
        const ty = typer(it.text, { cursor: true });
        row.appendChild(ty.el);
        host.appendChild(row);
        if (idx === 0) {
          // the first command typed in the bridge before this state (K1: in P4's CTA, then again in the entrance)
          beats.push(entrance({ el: row, at: 0, dur: 0, typer: ty }));
        } else if (phase === "gate") {
          beats.push(entrance({ el: row, at: cursor, dur: 0.06, typer: ty }));
          cursor += 0.08;
        } else {
          phase = "rerun";
          cursor = Math.max(cursor, sc.rerunAt);
          beats.push({ el: row, at: cursor, dur: 0.05, typer: ty });
          cursor += 0.08;
        }
      } else if (it.kind === "out") {
        const row = el("div", `tl tl-out mono ${it.tone}`, it.text);
        host.appendChild(row);
        const beat: Beat = { el: row, at: cursor, dur: 0 };
        if (it.exit) {
          row.classList.add("exit");
          beat.exit = { pane: 0, key: `s${si}:${exits++}`, tint: it.tone === "ingot" ? 1 : 0 };
        }
        beats.push(entrance(beat));
        cursor += LINE_STEP;
      } else if (it.kind === "agent") {
        const box = fromHTML(html`<div class="tl tl-agent"><p class="agent-k mono">agent · reads the output</p><p class="agent-p">${prose(it.text)}</p></div>`);
        host.appendChild(box);
        cursor = Math.max(cursor, sc.agentAt);
        beats.push(entrance({ el: box, at: cursor, dur: 0 }));
        cursor = sc.toolsFrom;
        phase = "tools";
      } else {
        const box = fromHTML(html`<div class="tl tl-tool">
          <div class="tool-h mono"><span class="tool-head"></span><span class="tool-done ingot">done</span></div>
          ${it.body.map((b) => html`<p class="tool-b mono">${esc(b)}</p>`)}
        </div>`);
        const ty = typer(it.head, { cursor: true });
        q(box, ".tool-head").appendChild(ty.el);
        host.appendChild(box);
        const at = sc.toolsFrom + toolIndex * toolSpan;
        toolIndex++;
        const done = q(box, ".tool-done");
        beats.push({ el: box, at, dur: toolSpan * 0.4, typer: ty, done });
        qa(box, ".tool-b").forEach((b, k) => beats.push({ el: b, at: at + toolSpan * (0.45 + k * 0.15), dur: 0 }));
        cursor = at + toolSpan;
      }
    });
    const sec: Section = { el: host, beats };
    if (si < STATES.length - 1) {
      // the bridge: the next state's command types at the bottom of the session
      const b = el("div", "tl tl-bridge mono");
      b.appendChild(el("span", "prompt", "$"));
      b.appendChild(document.createTextNode(" "));
      const next = STATES[si + 1].session[0];
      const ty = typer(next.kind === "cmd" ? next.text : "", { cursor: true });
      b.appendChild(ty.el);
      if (si < 2) b.appendChild(el("span", "bridge-label dive-ash", "↓ types as you scroll"));
      host.appendChild(b);
      sec.bridge = { el: b, typer: ty };
    }
    return sec;
  }

  /** The outputs pane follows the session: `before` with the gate's output, `changed` over the tool calls, `after` with the re-run. */
  private buildOutputs(host: HTMLElement, before: OutBlock, changed: OutBlock, after: OutBlock, session: Beat[], si: number): Section {
    const beats: Beat[] = [];
    const sc = si === 0 ? K1_SCHEDULE : SCHEDULE;
    const sessExits = session.filter((b) => b.exit);
    const gateExitAt = sessExits[0]?.at ?? 0.3;
    const lastExitAt = sessExits[sessExits.length - 1]?.at ?? 0.8;
    const rerunAt = session.filter((b) => b.typer && b.at >= sc.rerunAt)[0]?.at ?? sc.rerunAt;
    let exits = 0;
    let entK = 0;
    const block = (b: OutBlock, from: number, to: number): void => {
      const box = fromHTML(html`<div class="ob"><p class="ob-l mono dive-ash">${esc(b.label)}</p></div>`);
      host.appendChild(box);
      // K1's `before` block is entrance content, revealed with the session's first words
      const ent = (bt: Beat): Beat => {
        if (si === 0 && b === before) {
          bt.at = 0;
          bt.ent = K1_WORDS_FROM + 0.01 + entK++ * K1_WORD_STEP;
        }
        return bt;
      };
      beats.push(ent({ el: box, at: from, dur: 0 }));
      const cols = b.head?.length ?? 0;
      if (b.head) {
        const h = el("div", "ob-row ob-head mono");
        h.style.gridTemplateColumns = `1.6fr repeat(${cols - 1}, 1fr)`;
        b.head.forEach((c) => h.appendChild(el("span", "", c)));
        box.appendChild(h);
      }
      const n = b.rows.length;
      b.rows.forEach((r: OutRow, k) => {
        const at = r.exit ? (b === before ? gateExitAt : lastExitAt) : lerp(from, to, n > 1 ? k / (n - 1) : 0);
        const row = el("div", `ob-row mono ${r.tone}${r.bold ? " bold" : ""}`);
        const beat: Beat = { el: row, at, dur: 0 };
        if (r.cells) {
          row.style.gridTemplateColumns = `1.6fr repeat(${r.cells.length - 1}, 1fr)`;
          r.cells.forEach((c, ci) => {
            const cell = el("span", "");
            if (r.roll === ci) {
              const o = odometer(c);
              cell.appendChild(o.el);
              beat.odo = o;
              beat.dur = 0.18;
            } else cell.textContent = c;
            row.appendChild(cell);
          });
        } else {
          row.classList.add("ob-text");
          row.textContent = r.text ?? "";
        }
        if (r.exit) {
          row.classList.add("exit");
          beat.exit = { pane: 1, key: `o${si}:${exits++}`, tint: r.tone === "ingot" ? 1 : 0 };
        }
        box.appendChild(row);
        beats.push(ent(beat));
      });
    };
    block(before, sc.gateFrom, Math.max(sc.gateFrom, gateExitAt - LINE_STEP));
    block(changed, sc.toolsFrom + 0.04, sc.toolsTo + 0.02);
    block(after, rerunAt + 0.06, Math.max(rerunAt + 0.06, lastExitAt - LINE_STEP));
    return { el: host, beats };
  }

  /**
   * Stacked states on a phone (§17.2): the state shows its final content; on its entrance the typed command, the
   * output lines and the verdict flashes play once, on a clock.
   */
  /**
   * Phone (§17.2): the stacked state card mounts in the entrance order — kicker, the title typing, then the panes:
   * their words (the play below: command, output lines, flashes) → their surfaces → bars and labels — and the
   * status bar last. `m` is the card's mount progress (mount.ts `phoneMount`).
   */
  setPhoneEntrance(m: number): void {
    const s = this.snapshot;
    if (s === undefined) return;
    this.el.style.setProperty("--ent-k", range(m, 0.4, 0.43).toFixed(3));
    this.typedTitles[s].set(range(m, 0.4, 0.56));
    const r = easeOutCubic(range(m, 0.52, 0.57));
    this.el.style.setProperty("--ent-r", r.toFixed(3));
    this.el.classList.toggle("ent-open", r >= 1);
    this.el.style.setProperty("--ent-box", range(m, 0.62, 0.68).toFixed(3));
    this.el.style.setProperty("--ent-sup", range(m, 0.72, 0.76).toFixed(3));
    this.el.style.setProperty("--ent-s", range(m, 0.88, 0.9).toFixed(3));
    if (m >= 0.52) this.play();
  }

  play(): void {
    const s = this.snapshot;
    if (s === undefined || this.el.dataset.played) return;
    this.el.dataset.played = "1";
    const beats = [...this.session[s].beats, ...this.outputs[s].beats].filter((b) => b.el.isConnected);
    const order = [...beats].sort((a, b) => a.at - b.at);
    for (const b of order) b.el.classList.remove("on");
    const t0 = performance.now();
    const total = 1600;
    const tick = (now: number): void => {
      const t = Math.min(1, (now - t0) / total);
      for (const b of order) {
        const at = Math.min(0.92, b.at);
        const vis = t >= at;
        b.el.classList.toggle("on", vis);
        if (b.typer) b.typer.set(vis ? Math.min(1, (t - at) / 0.08) : 0);
        if (b.odo) b.odo.set(vis ? Math.min(1, (t - at) / 0.15) : 0);
        if (b.exit && vis && !b.el.dataset.flashed) {
          b.el.dataset.flashed = "1";
          // logged like the pinned flash; on phones the flash is the CSS band (the shader budget fallback)
          const w = window as unknown as { __flashLog?: { key: string; pane: string; tint: string; at: number; shader: string }[] };
          (w.__flashLog ??= []).push({ key: `phone:${b.exit.key}`, pane: b.exit.pane === 0 ? "session" : "outputs", tint: b.exit.tint ? "ingot" : "slag", at: Math.round(performance.now()), shader: "css-fallback" });
          const term = b.el.closest<HTMLElement>(".term");
          term?.classList.remove("flash-slag", "flash-ingot");
          void term?.offsetWidth;
          term?.classList.add(b.exit.tint ? "flash-ingot" : "flash-slag");
        }
      }
      if (t < 1) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }

  /* ---------- driving ---------- */

  /** Depth-gauge fill, marks and the bottom progress bar, driven across the whole pin (also for the reduced-motion snapshots). */
  setDepth(pc: number): void {
    const n = STATES.length;
    const s = Math.min(n - 1, Math.floor(pc * n));
    this.gaugeFill.style.setProperty("--p", pc.toFixed(4));
    this.gaugeMarks.forEach((m, i) => {
      m.classList.toggle("passed", pc >= i / n);
      m.classList.toggle("active", i === s);
    });
    this.progress.style.setProperty("--p", pc.toFixed(4));
  }

  setVisible(v: boolean): void {
    this.visible = v;
  }

  /**
   * Entrance mount progress of the section (§0c slots): the kicker fades, K1's title types, the panes reveal (the
   * primary visual), the gauge comes in over the cards' slot and the status bar last, so the pin begins assembled
   * and nothing shows before the content slots start.
   */
  setEntrance(q01: number): void {
    this.entrance = Math.max(0, Math.min(1, q01));
  }

  /** Pin progress p ∈ [0, 1] (unclamped outside while approaching / leaving). */
  apply(p: number): void {
    const pc = clamp01(p);
    const n = STATES.length;
    const s = Math.min(n - 1, Math.floor(pc * n));
    const t = clamp01(pc * n - s);
    this.depth = pc;

    // ---- chrome: kicker, titles, status bar, gauge, progress, ground ----
    const enter = s === 0 ? 1 : easeOutCubic(range(t, 0, 0.12));
    this.kickerOdo.setValue(s === 0 ? 1 : lerp(s, s + 1, enter));
    this.statusLines.forEach((g, i) => {
      g.style.setProperty("--o", (i % n === s ? enter : i % n === s - 1 ? 1 - enter : 0).toFixed(3));
      g.classList.toggle("cur", i % n === s);
    });
    qa(this.el, ".k-gate").forEach((g, i) => {
      g.style.setProperty("--o", i === s ? enter.toFixed(3) : i === s - 1 ? (1 - enter).toFixed(3) : "0");
      g.style.setProperty("--x", i === s ? ((1 - enter) * 4).toFixed(2) : i === s - 1 ? (-enter * 4).toFixed(2) : "0");
    });
    this.titles.forEach((h, i) => {
      const on = i === s ? 1 : i === s - 1 ? 1 - enter : 0;
      h.style.setProperty("--o", on.toFixed(3));
      h.classList.toggle("cur", i === s);
      if (i === s) this.typedTitles[i].set(s === 0 ? Math.max(range(this.entrance, ...SLOTS.title), range(pc, 0, 0.025)) : range(t, 0.02, 0.14));
      else if (i === s - 1) this.typedTitles[i].set(1);
    });
    // first-state mount during the entrance (complete once the pin has started, and for snapshots)
    const ent = this.snapshot !== undefined || pc > 0 ? 1 : this.entrance;
    this.el.style.setProperty("--ent-k", range(ent, ...SLOTS.kicker).toFixed(3));
    const entR = easeOutCubic(range(ent, SLOTS.visual[0], SLOTS.visual[0] + CLIP));
    this.el.style.setProperty("--ent-r", entR.toFixed(3));
    this.el.classList.toggle("ent-open", entR >= 1);
    // text before its box (§0c): the panes' surfaces and seams arrive after their words, the support text after that
    this.el.style.setProperty("--ent-box", range(ent, ...K1_BOX).toFixed(3));
    this.el.style.setProperty("--ent-sup", range(ent, ...K1_SUPPORT).toFixed(3));
    this.el.style.setProperty("--ent-c", range(ent, ...SLOTS.card).toFixed(3));
    this.el.style.setProperty("--ent-s", range(ent, ...SLOTS.status).toFixed(3));

    this.setDepth(pc);
    const ground = s === 0 ? range(t, 0, 0.15) : 1;
    this.el.style.setProperty("--ground", ground.toFixed(3));
    this.el.style.setProperty("--glow", (ground * (0.3 + 0.7 * pc)).toFixed(3));

    // ---- hints: arrive in settle (0.72–0.85), leave in the next enter (0–0.15) ----
    this.hints.forEach((h, i) => {
      let o = 0;
      if (i === s) o = easeOutCubic(range(t, 0.72, 0.85));
      else if (i === s - 1) o = 1 - easeOutCubic(range(t, 0, 0.15));
      h.style.setProperty("--o", o.toFixed(3));
      h.style.setProperty("--draw", o.toFixed(3));
      h.classList.toggle("on", o > 0.01);
    });

    // ---- the two panes ----
    this.applySections(this.session, s, t, enter, pc);
    this.applySections(this.outputs, s, t, enter, pc);
  }

  private applySections(secs: Section[], s: number, t: number, enter: number, pc: number): void {
    secs.forEach((sec, i) => {
      const on = i === s ? 1 : i === s - 1 ? 1 - enter : 0;
      sec.el.style.setProperty("--o", on.toFixed(3));
      sec.el.classList.toggle("on", on > 0.01);
      if (i !== s) {
        if (i === s - 1 && sec.bridge) sec.bridge.typer.set(1);
        return;
      }
      for (const b of sec.beats) {
        // K1's opening content mounts with the entrance (pinned-mount m); once the pin runs it simply stays
        const pre = b.ent !== undefined && pc <= 0 && this.snapshot === undefined;
        const vis = pre ? this.entrance >= b.ent! : t >= b.at;
        b.el.classList.toggle("on", vis);
        if (b.typer) {
          const tp = pre ? range(this.entrance, b.ent!, b.ent! + 0.04) : b.dur > 0 ? range(t, b.at, b.at + b.dur) : 1;
          b.typer.set(vis ? tp : 0);
          b.typer.cursor(vis && tp < 1);
        }
        if (b.odo) b.odo.set(pre ? range(this.entrance, b.ent!, b.ent! + 0.06) : b.dur > 0 && b.ent === undefined ? range(t, b.at, b.at + b.dur) : vis ? 1 : 0);
        if (b.done) b.done.classList.toggle("on", t >= b.at + b.dur + 0.04);
        if (b.exit) {
          const landed = pre ? this.entrance >= b.ent! + 0.02 : t >= b.at + 0.02;
          b.el.style.setProperty("--underline", landed ? "1" : "0");
          b.el.classList.toggle("drawn", landed);
          if (landed) this.flash(b.exit.pane, b.exit.key, b.exit.tint);
        }
      }
      if (sec.bridge) {
        const p = range(t, ...BRIDGE);
        sec.bridge.el.classList.toggle("on", t >= BRIDGE[0]);
        sec.bridge.typer.set(p);
        sec.bridge.typer.cursor(p > 0 && p < 1);
      }
    });
  }

  /** Verdict flash: runs once per exit landing, on a clock; never replays on scroll back. */
  private flash(pane: 0 | 1, key: string, tint: number): void {
    if (this.fired.has(key) || this.snapshot !== undefined) return;
    this.fired.add(key);
    const f = this.flashes[pane];
    const w = window as unknown as { __flashLog?: { key: string; pane: string; tint: string; at: number; shader: string }[] };
    (w.__flashLog ??= []).push({ key, pane: pane === 0 ? "session" : "outputs", tint: tint ? "ingot" : "slag", at: Math.round(performance.now()), shader: f?.pass?.alive ? "gl" : "css-fallback" });
    if (!f?.pass?.alive) return;
    f.start = performance.now();
    f.tint = tint;
  }
}
