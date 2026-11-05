// Real touch swipe through CDP (touchStart, 10 touchMoves, touchEnd) on the first http page target.
// usage: node .tools/swipe.mjs <ws-browser-url> <x1> <y1> <x2> <y2>
const [url, x1, y1, x2, y2] = process.argv.slice(2).map((v, i) => (i ? Number(v) : v));
const http = url.replace(/^ws/, "http").replace(/\/devtools\/browser\/.*/, "");
const t = (await (await fetch(`${http}/json/list`)).json()).find((t) => t.type === "page" && t.url.startsWith("http"));
const ws = new WebSocket(t.webSocketDebuggerUrl);
await new Promise((r) => ws.addEventListener("open", r));
let id = 0;
const send = (method, params = {}) =>
  new Promise((res) => {
    const my = ++id;
    ws.addEventListener("message", function h(ev) {
      const m = JSON.parse(ev.data);
      if (m.id === my) { ws.removeEventListener("message", h); res(m); }
    });
    ws.send(JSON.stringify({ id: my, method, params }));
  });
const pt = (x, y) => [{ x, y, radiusX: 4, radiusY: 4, force: 1, id: 1 }];
await send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: pt(x1, y1) });
for (let i = 1; i <= 10; i++) {
  await send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: pt(x1 + ((x2 - x1) * i) / 10, y1 + ((y2 - y1) * i) / 10) });
  await new Promise((r) => setTimeout(r, 16));
}
await send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
console.log("swipe", x1, y1, "→", x2, y2);
ws.close();
