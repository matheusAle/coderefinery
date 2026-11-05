/**
 * Typed text (tier 1): `chars_shown = round(len × progress)`; the block cursor
 * blinks on a clock but never moves ahead of the last typed character
 * (motion-spec §5a). Scrolling back deletes characters one by one.
 */

export interface Typer {
  el: HTMLElement;
  set(progress: number): void;
  /** Hide the cursor (the line became a normal `cmd` line after Enter). */
  cursor(show: boolean): void;
  readonly text: string;
}

export function typer(text: string, options: { cursor?: boolean; className?: string } = {}): Typer {
  const el = document.createElement("span");
  el.className = "typed" + (options.className ? " " + options.className : "");
  const body = document.createElement("span");
  const cur = document.createElement("span");
  cur.className = "cursor";
  cur.setAttribute("aria-hidden", "true");
  // the untyped rest, laid out but invisible, so a wrapping line keeps its final height while it types (shown below
  // desktop only, where the stacked states would otherwise grow under the reader's scroll)
  const rest = document.createElement("span");
  rest.className = "typed-rest";
  rest.setAttribute("aria-hidden", "true");
  el.appendChild(body);
  if (options.cursor !== false) el.appendChild(cur);
  el.appendChild(rest);
  el.setAttribute("aria-label", text);
  let shown = -1;

  function set(progress: number): void {
    const p = progress < 0 ? 0 : progress > 1 ? 1 : progress;
    const n = Math.round(text.length * p);
    if (n === shown) return;
    shown = n;
    body.textContent = text.slice(0, n);
    rest.textContent = text.slice(n);
  }
  set(0);

  return {
    el,
    set,
    text,
    cursor(show: boolean) {
      cur.style.display = show ? "" : "none";
    },
  };
}
