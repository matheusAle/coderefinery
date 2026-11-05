/**
 * Odometer: a template such as "+441 %", "3–120×", "50.5" or "36/36" is split
 * into digit runs and static glyphs. Each digit run rolls as one number from
 * 0 to its value; a column only moves while the column below it is on its
 * last unit, so the right-most digit settles last (motion-spec §9).
 *
 * Accessibility / copy-paste: the DOM holds the real number once, in a
 * visually hidden span; the rolling strips and static glyphs are drawn with
 * CSS pseudo-elements on aria-hidden spans, so innerText and the accessibility
 * tree read `441`, never `0 1 2 3 …`.
 */

interface DigitRun {
  value: number;
  cols: HTMLElement[];
}

export interface Odometer {
  el: HTMLElement;
  set(progress: number): void;
  setValue(v: number): void;
  retarget(template: string): void;
}

export interface OdometerOptions {
  static?: boolean;
  hideLeadingZeros?: boolean;
}

export function odometer(template: string, options: OdometerOptions = {}): Odometer {
  const el = document.createElement("span");
  el.className = "odo";
  const sr = document.createElement("span");
  sr.className = "odo-sr";
  const vis = document.createElement("span");
  vis.className = "odo-v";
  vis.setAttribute("aria-hidden", "true");
  el.append(sr, vis);
  let runs: DigitRun[] = [];
  let tpl = template;

  function build(t: string): void {
    tpl = t;
    sr.textContent = t;
    vis.textContent = "";
    runs = [];
    const re = /(\d+(?:\.\d+)?)|([^\d]+)/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(t))) {
      if (m[1]) {
        const cols: HTMLElement[] = [];
        for (const ch of m[1]) {
          const g = document.createElement("span");
          if (ch === ".") {
            g.className = "g";
            g.dataset.g = ".";
          } else {
            g.className = "d";
            cols.push(g);
          }
          vis.appendChild(g);
        }
        runs.push({ value: Number(m[1].replace(".", "")), cols });
      } else {
        const g = document.createElement("span");
        g.className = "g";
        g.dataset.g = m[2];
        vis.appendChild(g);
      }
    }
  }

  function applyRun(run: DigitRun, v: number): void {
    const n = run.cols.length;
    for (let i = 0; i < n; i++) {
      const place = Math.pow(10, n - 1 - i);
      const whole = Math.floor(v / place) % 10;
      const lower = v - Math.floor(v / place) * place;
      const frac = place === 1 ? lower : Math.max(0, lower - (place - 1));
      run.cols[i].style.setProperty("--v", (whole + frac).toFixed(3));
      if (options.hideLeadingZeros) {
        const leading = i < n - 1 && v < place - 0.5;
        run.cols[i].style.visibility = leading ? "hidden" : "";
      }
    }
  }

  function set(progress: number): void {
    const p = progress < 0 ? 0 : progress > 1 ? 1 : progress;
    for (const run of runs) applyRun(run, run.value * p);
  }

  build(template);
  set(options.static ? 1 : 0);

  return {
    el,
    set,
    setValue(v: number) {
      if (!runs[0]) return;
      applyRun(runs[0], Math.max(0, v));
      // the readable number follows the value for value-driven odometers (gutters, counters)
      const decimals = (tpl.match(/\d+\.(\d+)/) ?? [])[1]?.length ?? 0;
      sr.textContent = tpl.replace(/\d+(?:\.\d+)?/, (Math.round(v) / Math.pow(10, decimals)).toFixed(decimals));
    },
    retarget(t: string) {
      build(t);
    },
  };
}
