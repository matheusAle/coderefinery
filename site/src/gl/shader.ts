/**
 * Raw WebGL 1 helper (motion-spec §14.4): one full-screen triangle, one
 * fragment shader, no framebuffer. Disabled under prefers-reduced-motion, on
 * navigator.connection.saveData, when WebGL is missing, and when a warm frame
 * renders slower than the guard: the first two frames warm the pipeline, the
 * third is measured (12 ms). The CSS gradient behind the canvas is the
 * fallback in every case; a dropped shader is logged by name.
 */

const VERT = `
attribute vec2 a;
void main(){ gl_Position = vec4(a, 0.0, 1.0); }
`;

const GUARD_MS = 12;

/** Every pass is recorded on `window.__shaderLog` ({ shader, status, firstFrameMs, reason }) so the §14.4 guards can be audited. */
export interface ShaderLogEntry {
  shader: string;
  status: "running" | "disabled";
  firstFrameMs: number | null;
  reason: string | null;
}
const shaderLog: ShaderLogEntry[] = ((window as unknown as { __shaderLog?: ShaderLogEntry[] }).__shaderLog ??= []);
const log = (entry: ShaderLogEntry): void => {
  const i = shaderLog.findIndex((e) => e.shader === entry.shader);
  if (i >= 0) shaderLog[i] = entry;
  else shaderLog.push(entry);
  if (entry.status === "disabled") console.info(`shader "${entry.shader}" disabled: ${entry.reason}`);
};
const WARMUP_FRAMES = 2;

export interface ShaderPass {
  canvas: HTMLCanvasElement;
  uniform(name: string, ...v: number[]): void;
  draw(time: number): void;
  resize(): void;
  destroy(): void;
  readonly alive: boolean;
}

interface NavWithConnection extends Navigator {
  connection?: { saveData?: boolean };
}

export function shadersAllowed(): boolean {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return false;
  const nav = navigator as NavWithConnection;
  if (nav.connection?.saveData) return false;
  return true;
}

export function disabledReason(): string | null {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return "prefers-reduced-motion";
  const nav = navigator as NavWithConnection;
  if (nav.connection?.saveData) return "saveData";
  return null;
}

export function createPass(host: HTMLElement, frag: string, name = "shader"): ShaderPass | null {
  const why = disabledReason();
  if (why) {
    log({ shader: name, status: "disabled", firstFrameMs: null, reason: why });
    return null;
  }
  const canvas = document.createElement("canvas");
  canvas.className = "gl";
  canvas.setAttribute("aria-hidden", "true");
  const gl = canvas.getContext("webgl", {
    alpha: true,
    antialias: false,
    depth: false,
    stencil: false,
    premultipliedAlpha: true,
    powerPreference: "low-power",
  });
  if (!gl) {
    log({ shader: name, status: "disabled", firstFrameMs: null, reason: "no WebGL" });
    return null;
  }

  const compile = (type: number, src: string): WebGLShader | null => {
    const s = gl.createShader(type);
    if (!s) return null;
    gl.shaderSource(s, src);
    gl.compileShader(s);
    if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
      console.warn(`shader "${name}"`, gl.getShaderInfoLog(s));
      gl.deleteShader(s);
      return null;
    }
    return s;
  };
  const vs = compile(gl.VERTEX_SHADER, VERT);
  const fs = compile(gl.FRAGMENT_SHADER, frag);
  const prog = gl.createProgram();
  if (!vs || !fs || !prog) return null;
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
    console.warn(`program "${name}"`, gl.getProgramInfoLog(prog));
    return null;
  }
  gl.useProgram(prog);
  const buf = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, buf);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
  const loc = gl.getAttribLocation(prog, "a");
  gl.enableVertexAttribArray(loc);
  gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);
  gl.disable(gl.DEPTH_TEST);
  gl.enable(gl.BLEND);
  gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);

  const uniforms = new Map<string, number[]>();
  const locs = new Map<string, WebGLUniformLocation | null>();
  const uloc = (n: string): WebGLUniformLocation | null => {
    if (!locs.has(n)) locs.set(n, gl.getUniformLocation(prog, n));
    return locs.get(n) ?? null;
  };

  host.appendChild(canvas);
  log({ shader: name, status: "running", firstFrameMs: null, reason: null });
  let alive = true;
  let frames = 0;
  const ro = new ResizeObserver(() => pass.resize());
  ro.observe(host);

  const pass: ShaderPass = {
    canvas,
    get alive() {
      return alive;
    },
    uniform(n, ...v) {
      uniforms.set(n, v);
    },
    resize() {
      // backing store = the host's CSS rect × devicePixelRatio (capped at 2), viewport follows every change
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      const r = host.getBoundingClientRect();
      const w = Math.max(1, Math.round(r.width * dpr));
      const h = Math.max(1, Math.round(r.height * dpr));
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
        gl.viewport(0, 0, w, h);
      }
    },
    draw(time) {
      if (!alive) return;
      const measure = frames === WARMUP_FRAMES;
      const t0 = measure ? performance.now() : 0;
      pass.resize();
      gl.useProgram(prog);
      gl.clearColor(0, 0, 0, 0);
      gl.clear(gl.COLOR_BUFFER_BIT);
      const r = uloc("u_res");
      if (r) gl.uniform2f(r, canvas.width, canvas.height);
      const t = uloc("u_time");
      if (t) gl.uniform1f(t, time);
      for (const [n, v] of uniforms) {
        const l = uloc(n);
        if (!l) continue;
        if (v.length === 1) gl.uniform1f(l, v[0]);
        else if (v.length === 2) gl.uniform2f(l, v[0], v[1]);
        else if (v.length === 3) gl.uniform3f(l, v[0], v[1], v[2]);
        else gl.uniform4f(l, v[0], v[1], v[2], v[3]);
      }
      gl.drawArrays(gl.TRIANGLES, 0, 3);
      if (frames < WARMUP_FRAMES) gl.flush();
      if (measure) {
        gl.finish();
        const dt = performance.now() - t0;
        if (dt > GUARD_MS) {
          log({ shader: name, status: "disabled", firstFrameMs: dt, reason: `warm frame ${dt.toFixed(1)} ms > ${GUARD_MS} ms (CSS fallback)` });
          pass.destroy();
        } else log({ shader: name, status: "running", firstFrameMs: dt, reason: null });
      }
      frames++;
    },
    destroy() {
      if (!alive) return;
      alive = false;
      ro.disconnect();
      canvas.remove();
      const ext = gl.getExtension("WEBGL_lose_context");
      ext?.loseContext();
    },
  };
  allPasses.push({ pass, name });
  // the §14.4 guard runs for every pass at mount, not on its first on-screen frame (D23): if nothing has drawn by
  // the next animation frame (the furnace and the flashes sit off-screen until the dive), warm the pipeline and
  // time the third frame now, then clear, so `__shaderLog` has firstFrameMs for every shader from the start
  // (a pass that drew a frame or two and then paused, like the discreet field under a card that has its own, is
  // completed the same way; its uniforms are real then, so the frame stays)
  requestAnimationFrame(() =>
    requestAnimationFrame(() => {
      if (!alive || frames > WARMUP_FRAMES) return;
      const blank = frames === 0;
      while (alive && frames <= WARMUP_FRAMES) pass.draw(performance.now() / 1000);
      if (alive && blank) {
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
      }
    }),
  );
  return pass;
}

/** Shared rAF loop so every pass runs on one clock. */
const loops = new Set<(time: number) => void>();
let running = false;

/**
 * Frame budget (§17.3): on phones and tablets, while the page is being scrolled, if the median frame over the last
 * 90 scrolling frames is above 18 ms, every pass drops to its static CSS fallback for the session (logged).
 */
const allPasses: { pass: ShaderPass; name: string }[] = [];
let lastFrame = 0;
let lastScroll = -1e9;
const frames: number[] = [];
let budgetBlown = false;
window.addEventListener("scroll", () => (lastScroll = performance.now()), { passive: true });
function watchBudget(now: number): void {
  const dt = now - lastFrame;
  lastFrame = now;
  if (budgetBlown || dt <= 0 || dt > 200 || now - lastScroll > 250) return;
  if (!window.matchMedia("(max-width: 1023px)").matches) return;
  frames.push(dt);
  if (frames.length < 90) return;
  const sorted = [...frames].sort((a, b) => a - b);
  frames.length = 0;
  if (sorted[45] <= 18) return;
  budgetBlown = true;
  for (const { pass, name } of allPasses) {
    if (!pass.alive) continue;
    pass.destroy();
    log({ shader: name, status: "disabled", firstFrameMs: null, reason: `scroll frames median ${sorted[45].toFixed(1)} ms > 18 ms (CSS fallback)` });
  }
}

function tick(now: number): void {
  if (loops.size === 0) {
    running = false;
    return;
  }
  watchBudget(now);
  const t = now / 1000;
  for (const f of loops) f(t);
  requestAnimationFrame(tick);
}
export function onFrame(fn: (time: number) => void): () => void {
  loops.add(fn);
  if (!running) {
    running = true;
    requestAnimationFrame(tick);
  }
  return () => loops.delete(fn);
}
