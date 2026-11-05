// Enable touch emulation (pointer: coarse, hover: none, touch events) on every page target of a CDP browser.
// usage: node .tools/touch.mjs <ws-browser-url> [on|off]
const [url, mode = "on"] = process.argv.slice(2);
const http = url.replace(/^ws/, "http").replace(/\/devtools\/browser\/.*/, "");
const targets = await (await fetch(`${http}/json/list`)).json();
for (const t of targets.filter((t) => t.type === "page" && t.url.startsWith("http"))) {
  const ws = new WebSocket(t.webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener("open", r));
  let id = 0;
  const send = (method, params = {}) =>
    new Promise((res) => {
      const my = ++id;
      ws.addEventListener("message", function h(ev) {
        const m = JSON.parse(ev.data);
        if (m.id === my) {
          ws.removeEventListener("message", h);
          res(m);
        }
      });
      ws.send(JSON.stringify({ id: my, method, params }));
    });
  const on = mode === "on";
  await send("Emulation.setTouchEmulationEnabled", { enabled: on, maxTouchPoints: on ? 5 : 0 });
  await send("Emulation.setEmitTouchEventsForMouse", { enabled: on, configuration: "mobile" });
  console.log(t.url, "touch", mode);
  if (mode !== "on") ws.close();
}
// touch emulation lives as long as this CDP session: stay attached (kill the process to stop)
if (mode === "on") setInterval(() => {}, 1 << 30);
