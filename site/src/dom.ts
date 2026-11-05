/** Tiny HTML helpers. Copy is passed through `esc` so the verbatim text is safe. */

export function esc(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** Preserve runs of spaces in copy (the reference uses &nbsp; columns). */
export function pre(s: string): string {
  return esc(s).replace(/ {2,}/g, (m) => "&nbsp;".repeat(m.length));
}

export function html(strings: TemplateStringsArray, ...values: unknown[]): string {
  let out = "";
  strings.forEach((s, i) => {
    out += s;
    if (i < values.length) {
      const v = values[i];
      out += Array.isArray(v) ? v.join("") : String(v ?? "");
    }
  });
  return out;
}

export function fromHTML(markup: string): HTMLElement {
  const t = document.createElement("template");
  t.innerHTML = markup.trim();
  return t.content.firstElementChild as HTMLElement;
}

export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text !== undefined) e.textContent = text;
  return e;
}

export function q<T extends HTMLElement = HTMLElement>(root: ParentNode, sel: string): T {
  const r = root.querySelector<T>(sel);
  if (!r) throw new Error(`missing element: ${sel}`);
  return r;
}

export function qa<T extends HTMLElement = HTMLElement>(root: ParentNode, sel: string): T[] {
  return Array.from(root.querySelectorAll<T>(sel));
}

/**
 * Running prose with inline code (§0b "variables and values in prose are code"): `backticked` spans become
 * <code class="ic">; everything else is escaped.
 */
export function prose(s: string): string {
  return s
    .split("`")
    .map((part, i) => (i % 2 ? `<code class="ic">${esc(part)}</code>` : esc(part)))
    .join("");
}
