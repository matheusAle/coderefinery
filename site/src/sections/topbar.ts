/**
 * Wordmark + nav (tier 3: static, cursor blink only). Shared by all three routes. On phones (§17.1) the nav is a
 * full-width sheet under the topbar behind a menu button; choosing a link closes it.
 */
import { html } from "../dom";

export type NavKey = "why" | "agents" | "math";

export function topbar(active?: NavKey): string {
  const a = (k: NavKey): string => (active === k ? ' class="active" aria-current="page"' : "");
  return html`<div class="topbar">
    <a class="wordmark" href="/#top" aria-label="coderefinery home">
      <span class="wm-code">code</span><span class="wm-refinery">refinery</span><span class="wm-cursor" aria-hidden="true"></span>
    </a>
    <button class="nav-toggle mono" type="button" aria-expanded="false" aria-controls="primary-nav" aria-label="menu">menu</button>
    <nav class="nav" id="primary-nav" aria-label="Primary">
      <a href="/#why"${a("why")}>why</a>
      <a href="/agents"${a("agents")}>for agents</a>
      <a href="/math"${a("math")}>the math</a>
      <a href="/docs">docs</a>
      <a href="https://github.com/matheusAle/coderefinery" rel="noopener">github</a>
    </nav>
  </div>`;
}

/** The phone menu: the button toggles the sheet; choosing a link, Escape, a tap outside or the card going under the deck closes it. */
export function wireTopbar(root: ParentNode = document): void {
  root.querySelectorAll<HTMLElement>(".topbar").forEach((bar) => {
    const btn = bar.querySelector<HTMLButtonElement>(".nav-toggle");
    if (!btn) return;
    const set = (open: boolean): void => {
      bar.classList.toggle("nav-open", open);
      btn.setAttribute("aria-expanded", String(open));
      btn.textContent = open ? "close" : "menu";
    };
    btn.addEventListener("click", () => set(!bar.classList.contains("nav-open")));
    bar.querySelectorAll(".nav a").forEach((a) => a.addEventListener("click", () => set(false)));
    document.addEventListener("keydown", (e) => e.key === "Escape" && set(false));
    document.addEventListener("click", (e) => {
      if (bar.classList.contains("nav-open") && !bar.contains(e.target as Node)) set(false);
    });
    // the deck covers the card that holds this bar (§17.4): the sheet goes with it
    document.addEventListener("deck:cover", (e) => {
      if (bar.classList.contains("nav-open") && (e.target as Node).contains(bar)) set(false);
    });
  });
}
