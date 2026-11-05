/**
 * Typed titles (§0b): kickers and headlines type like a command at a prompt,
 * scrubbed by scroll over their slot. The full text stays in the DOM (each
 * character in its own span) so word wrapping is pre-computed and lines never
 * reflow; untyped characters are transparent. The block cursor is a
 * pseudo-element on the last typed character (no layout change), blinks on a
 * clock, and disappears when the title is complete.
 */

export interface TypedTitle {
  set(progress: number): void;
  readonly length: number;
}

export function typedTitle(el: HTMLElement): TypedTitle {
  if (el.dataset.typed === "1") {
    const existing = Array.from(el.querySelectorAll<HTMLElement>(".ch"));
    return make(existing);
  }
  const chars: HTMLElement[] = [];
  const walk = (node: Node): void => {
    if (node.nodeType === Node.TEXT_NODE) {
      const text = node.textContent ?? "";
      const frag = document.createDocumentFragment();
      for (const ch of text) {
        const s = document.createElement("span");
        s.className = "ch";
        s.textContent = ch;
        chars.push(s);
        frag.appendChild(s);
      }
      node.parentNode?.replaceChild(frag, node);
    } else if (node.nodeType === Node.ELEMENT_NODE && (node as Element).tagName !== "BR") {
      Array.from(node.childNodes).forEach(walk);
    }
  };
  const label = el.textContent ?? "";
  Array.from(el.childNodes).forEach(walk);
  el.dataset.typed = "1";
  el.setAttribute("aria-label", label);
  el.classList.add("typed-title");
  return make(chars);
}

function make(chars: HTMLElement[]): TypedTitle {
  let shown = -1;
  let cur = -1;
  const n = chars.length;
  return {
    length: n,
    set(progress) {
      const p = progress < 0 ? 0 : progress > 1 ? 1 : progress;
      const k = Math.round(n * p);
      if (k === shown) return;
      // only touch the characters that changed
      const lo = Math.min(shown < 0 ? 0 : shown, k);
      const hi = Math.max(shown < 0 ? 0 : shown, k);
      for (let i = lo; i < hi; i++) chars[i]?.classList.toggle("on", i < k);
      if (shown < 0) for (let i = 0; i < n; i++) chars[i].classList.toggle("on", i < k);
      shown = k;
      if (cur >= 0) chars[cur]?.classList.remove("cur");
      cur = k > 0 && k < n ? k - 1 : -1;
      if (cur >= 0) chars[cur].classList.add("cur");
    },
  };
}
