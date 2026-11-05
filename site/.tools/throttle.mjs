// CPU throttling on every page target of a CDP browser (a rough stand-in for a mid-range phone).
// usage: node .tools/throttle.mjs <ws-browser-url> <rate>   (rate 1 = off); stays attached while it runs
const [url, rate = "4"] = process.argv.slice(2);
const http = url.replace(/^ws/, "http").replace(/\/devtools\/browser\/.*/, "");
const targets = await (await fetch(`${http}/json/list`)).json();
for (const t of targets.filter((t) => t.type === "page" && t.url.startsWith("http"))) {
  const ws = new WebSocket(t.webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener("open", r));
  ws.send(JSON.stringify({ id: 1, method: "Emulation.setCPUThrottlingRate", params: { rate: Number(rate) } }));
  console.log(t.url, "cpu ×", rate);
}
setInterval(() => {}, 1 << 30);
