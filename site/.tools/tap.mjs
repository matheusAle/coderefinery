// Real touch tap through CDP (touchStart + touchEnd at x,y) on the first http page target.
// usage: node .tools/tap.mjs <ws-browser-url> <x> <y>
const [url, x, y] = process.argv.slice(2);
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
const p = { x: +x, y: +y, radiusX: 4, radiusY: 4, force: 1 };
await send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [p] });
await new Promise((r) => setTimeout(r, 60));
await send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
console.log("tap", x, y, t.url);
ws.close();
