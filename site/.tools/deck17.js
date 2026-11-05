// Round 17 deck probes (compact and desktop geometry). Inject with `agent-browser eval "$(cat .tools/deck17.js)"`,
// then call window.D17.<probe>() (each returns a JSON string).
(() => {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const frame = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  const units = () => [...document.querySelectorAll("main .deck-card, main > .card")];
  const face = (u) => u.querySelector(":scope > .card-face") || u;
  const nat = (el) => {
    const m = document.querySelector("main");
    m.classList.add("deck-measure");
    const y = el.getBoundingClientRect().top + scrollY;
    m.classList.remove("deck-measure");
    return y;
  };
  const go = async (y) => {
    scrollTo({ top: y, behavior: "instant" });
    await frame();
    await sleep(30);
  };
  const label = (u) => {
    const k = u.querySelector(".kicker, .dive-kicker");
    return (k ? k.textContent.trim().slice(0, 26) : u.className.split(" ")[0]) + "";
  };
  const mat = (el) => {
    const t = getComputedStyle(el).transform;
    if (!t || t === "none") return { s: 1, ty: 0 };
    const m = new DOMMatrix(t);
    return { s: +m.a.toFixed(4), ty: +m.f.toFixed(2) };
  };
  const dimOf = (u) => +getComputedStyle(face(u).querySelector(":scope > .deck-dim") || document.body).opacity || 0;

  // every card's top and bottom reachable: at scroll = natural top the face top is at 0; at natural top + face − vh
  // the face bottom sits at the viewport bottom and the next card has not come in yet
  async function reach() {
    const vh = innerHeight;
    const us = units();
    const out = [];
    for (let i = 0; i < us.length; i++) {
      const u = us[i];
      const f = face(u);
      const T = nat(u);
      const fh = f.offsetHeight;
      await go(T);
      const topAt = Math.round(f.getBoundingClientRect().top);
      const yb = T + fh - vh;
      await go(Math.min(yb, document.documentElement.scrollHeight - vh));
      const r = f.getBoundingClientRect();
      const nxt = us[i + 1] ? Math.round(us[i + 1].getBoundingClientRect().top) : null;
      // the end of the hold: the next card is about to appear, this one is still pinned by its bottom
      let holdEnd = null;
      if (us[i + 1]) {
        await go(nat(us[i + 1]) - vh - 1);
        holdEnd = Math.round(f.getBoundingClientRect().bottom);
      }
      const hold = us[i + 1] ? Math.round(nat(us[i + 1]) - vh - yb) : 0;
      out.push({ i, card: label(u), h: fh, tall: fh > vh + 1, topAt, bottomAt: Math.round(r.bottom), holdEnd, hold, vh, nextTop: nxt, ok: Math.abs(topAt) <= 1 && Math.abs(r.bottom - vh) <= 1 && (holdEnd === null || Math.abs(holdEnd - vh) <= 1) && (nxt === null || nxt >= vh - 1) && getComputedStyle(u).visibility !== "hidden" });
    }
    return JSON.stringify({ all: out.every((o) => o.ok), out });
  }

  // deck motion while card j comes in over card j−1: samples of the covered and the incoming face
  async function motion(j, steps = [0, 0.09, 0.18, 0.3, 0.5, 0.75, 0.95, 1]) {
    const vh = innerHeight;
    const us = units();
    const inc = us[j];
    const cov = us[j - 1];
    const T = nat(inc);
    const rows = [];
    for (const f of steps) {
      // f: the incoming top's travel from the viewport bottom (0) to the top (1)
      await go(T - vh + f * vh);
      const a = mat(face(cov));
      const b = mat(face(inc));
      rows.push({
        f,
        inTop: Math.round(inc.getBoundingClientRect().top),
        cov: { s: a.s, ty: a.ty, dim: +dimOf(cov).toFixed(3), vis: getComputedStyle(cov).visibility, origin: getComputedStyle(face(cov)).transformOrigin, wc: face(cov).style.willChange },
        inc: { s: b.s, dim: +dimOf(inc).toFixed(3), glow: inc.style.getPropertyValue("--seam-glow"), wc: face(inc).style.willChange },
      });
    }
    return JSON.stringify({ covered: label(cov), coveredH: face(cov).offsetHeight, incoming: label(inc), vh, rows });
  }

  // at n positions over the page: hits of a 5 × 9 elementFromPoint grid on a hidden card (must be 0), how many
  // covered-but-visible cards (≤ 1), hidden cards' computed visibility, and document width
  async function probe(n = 60) {
    const vh = innerHeight;
    const H = document.documentElement.scrollHeight - vh;
    const us = units();
    let bad = 0;
    let maxCovered = 0;
    let visLeak = 0;
    let wide = 0;
    const where = [];
    for (let k = 0; k <= n; k++) {
      await go(Math.round((H * k) / n));
      const hidden = us.filter((u) => u.dataset.hidden);
      maxCovered = Math.max(maxCovered, us.filter((u) => u.dataset.covered && !u.dataset.hidden).length);
      for (const u of hidden) if (getComputedStyle(u).visibility !== "hidden" || !u.inert) visLeak++;
      for (let x = 0; x < 5; x++)
        for (let y = 0; y < 9; y++) {
          const e = document.elementFromPoint(((x + 0.5) * innerWidth) / 5, ((y + 0.5) * vh) / 9);
          const u = e && e.closest(".deck-card, main > .card");
          if (u && u.dataset.hidden) {
            bad++;
            if (where.length < 5) where.push({ y: scrollY, card: label(u) });
          }
        }
      if (document.documentElement.scrollWidth !== innerWidth) wide++;
    }
    return JSON.stringify({ positions: n + 1, hitsOnHidden: bad, maxCoveredVisible: maxCovered, hiddenNotHiddenOrInert: visLeak, widthOverflow: wide, where });
  }

  // the page bottom: last card + footer only (grid probe at max scroll and 50 px steps above it)
  async function bottom() {
    const vh = innerHeight;
    const H = document.documentElement.scrollHeight - vh;
    const us = units();
    const last = us[us.length - 1];
    const out = [];
    for (const d of [300, 200, 100, 50, 0]) {
      await go(H - d);
      const foreign = new Set();
      for (let x = 0; x < 5; x++)
        for (let y = 0; y < 12; y++) {
          const e = document.elementFromPoint(((x + 0.5) * innerWidth) / 5, ((y + 0.5) * vh) / 12);
          if (!e) continue;
          const u = e.closest(".deck-card, main > .card");
          if (e.closest("footer")) continue;
          if (u === last) continue;
          foreign.add(u ? label(u) : e.tagName + "." + e.className);
        }
      const visibleEarlier = us.slice(0, -1).filter((u) => getComputedStyle(u).visibility !== "hidden").map(label);
      out.push({ fromBottom: d, foreign: [...foreign], visibleEarlier, lastTop: Math.round(last.getBoundingClientRect().top), footerTop: Math.round(document.querySelector("footer").getBoundingClientRect().top) });
    }
    return JSON.stringify(out);
  }

  // scroll the whole page at `px` per frame and record frame intervals (rAF deltas)
  async function frames(px = 30) {
    await go(0);
    const H = document.documentElement.scrollHeight - innerHeight;
    const d = [];
    let last = performance.now();
    let prevY = -1;
    let still = 0;
    await new Promise((res) => {
      const step = (t) => {
        if (t - last > 17.5 && d.length) (window.D17.slow ??= []).push([Math.round(scrollY), Math.round(t - last)]);
        d.push(t - last);
        last = t;
        still = scrollY === prevY ? still + 1 : 0;
        prevY = scrollY;
        if (scrollY >= H - 1 || still > 5) return res();
        scrollTo({ top: scrollY + px, behavior: "instant" });
        requestAnimationFrame(step);
      };
      requestAnimationFrame(step);
    });
    d.shift();
    const s = [...d].sort((a, b) => a - b);
    const q = (p) => +s[Math.min(s.length - 1, Math.floor(p * s.length))].toFixed(1);
    return JSON.stringify({ end: Math.round(scrollY), H: Math.round(H), frames: d.length, p50: q(0.5), p95: q(0.95), p99: q(0.99), worst: +s[s.length - 1].toFixed(1), over16: d.filter((x) => x > 17.5).length, shaders: (window.__shaderLog || []).map((l) => l.shader + ":" + l.status) });
  }

  // document width at 12 scroll positions
  async function widths() {
    const H = document.documentElement.scrollHeight - innerHeight;
    const out = [];
    for (let k = 0; k < 12; k++) {
      await go(Math.round((H * k) / 11));
      out.push(document.documentElement.scrollWidth - innerWidth);
    }
    return JSON.stringify({ ok: out.every((v) => v === 0), diffs: out });
  }

  // scroll so card j's top is at fraction f of its slide (0 = viewport bottom, 1 = top)
  async function at(j, f) {
    const us = units();
    await go(nat(us[j]) - innerHeight + f * innerHeight);
    return JSON.stringify({ y: scrollY, card: label(us[j]) });
  }
  // scroll so card j is pinned by its bottom and card j+1's top is at fraction f of the viewport
  async function pinned(j, f) {
    const us = units();
    await go(nat(us[j + 1]) - innerHeight + f * innerHeight);
    return JSON.stringify({ y: scrollY, card: label(us[j]), next: label(us[j + 1]), bottom: Math.round(face(us[j]).getBoundingClientRect().bottom) });
  }
  const list = () => JSON.stringify(units().map((u, i) => ({ i, card: label(u), h: face(u).offsetHeight, nat: Math.round(nat(u)) })));

  // frames() outlasts a CDP eval on long pages: start it, then poll D17.result
  const startFrames = (px = 30) => {
    window.D17.result = null;
    frames(px).then((r) => (window.D17.result = r));
    return "started";
  };
  window.D17 = { reach, motion, probe, bottom, frames, startFrames, widths, at, pinned, list, nat, result: null };
  return "D17 ready";
})();
