// probe.js 探测 4027 端口协议（SSH 会立即发 Banner，RDP 不发）。
"use strict";
const net = require("net");
const s = net.connect(4027, "183.221.243.20");
s.setTimeout(8000);
s.on("connect", () => console.log("TCP_OK"));
s.on("data", (d) => {
  console.log("BANNER:", JSON.stringify(d.toString("latin1").slice(0, 100)));
  s.destroy();
});
s.on("timeout", () => {
  console.log("NO_BANNER_TIMEOUT");
  s.destroy();
});
s.on("error", (e) => console.log("ERR:", e.message));
s.on("close", () => process.exit(0));
