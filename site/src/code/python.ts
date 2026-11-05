/**
 * Python source as syntax-coloured HTML (§13.1 syntax roles: keywords syn-kw, strings syn-str, numbers syn-num,
 * function names after `def` in chalk, comments in ash, decorators syn-dec). Used by the /math code panes.
 */
import { esc } from "../dom";

export type TokenKind = "kw" | "str" | "num" | "fn" | "cm" | "dec" | "txt";
export interface Token {
  kind: TokenKind;
  text: string;
}

const KEYWORDS = new Set([
  "def", "return", "if", "elif", "else", "for", "in", "and", "or", "not", "None", "assert", "import", "from", "class", "True", "False",
]);

export function tokenize(text: string): Token[] {
  const out: Token[] = [];
  const re = /(#.*$)|(@[\w.]+)|('(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*")|(\b\d+(?:\.\d+)?\b)|(\b[A-Za-z_]\w*\b)|(\s+|[^\w\s'"#@]+)/g;
  let m: RegExpExecArray | null;
  let prevWord = "";
  while ((m = re.exec(text))) {
    if (m[1]) out.push({ kind: "cm", text: m[1] });
    else if (m[2]) out.push({ kind: "dec", text: m[2] });
    else if (m[3]) out.push({ kind: "str", text: m[3] });
    else if (m[4]) out.push({ kind: "num", text: m[4] });
    else if (m[5]) {
      const w = m[5];
      if (KEYWORDS.has(w)) out.push({ kind: "kw", text: w });
      else if (prevWord === "def") out.push({ kind: "fn", text: w });
      else out.push({ kind: "txt", text: w });
      prevWord = w;
      continue;
    } else out.push({ kind: "txt", text: m[6] });
    if (m[5] === undefined && !/^\s+$/.test(m[0])) prevWord = "";
  }
  return out;
}

export function renderPython(text: string): string {
  return tokenize(text)
    .map((t) => (t.kind === "txt" ? esc(t.text) : `<span class="syn-${t.kind}">${esc(t.text)}</span>`))
    .join("");
}
