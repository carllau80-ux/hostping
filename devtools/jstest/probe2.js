// probe2.js 确认 4027 是否 RDP（X.224 协商）+ 扫描常见管理端口。
"use strict";
const net = require("net");

// 1) RDP X.224 连接请求
function probeRDP() {
  return new Promise((resolve) => {
    const s = net.connect(4027, "183.221.243.20");
    s.setTimeout(6000);
    let out = "";
    s.on("connect", () => {
      // X.224 CR TPDU (RDP negotiation request)
      s.write(Buffer.from("030000130ee000000000000100080003000000", "hex"));
    });
    s.on("data", (d) => { out = d.toString("hex"); s.destroy(); });
    s.on("timeout", () => { s.destroy(); });
    s.on("error", (e) => { out = "ERR:" + e.message; });
    s.on("close", () => resolve(out));
  });
}

// 2) 端口连通性
function probePort(port) {
  return new Promise((resolve) => {
    const s = net.connect(port, "183.221.243.20");
    s.setTimeout(4000);
    s.on("connect", () => { s.destroy(); resolve("OPEN"); });
    s.on("timeout", () => { s.destroy(); resolve("TIMEOUT"); });
    s.on("error", () => resolve("CLOSED/FILTERED"));
  });
}

(async () => {
  const rdp = await probeRDP();
  console.log("4027 X.224 响应:", rdp ? rdp.slice(0, 60) : "(无响应)");
  if (rdp.startsWith("030000130ed")) console.log("=> 确认是 RDP（X.224 Connection Confirm）");
  for (const p of [22, 135, 445, 5985, 5986]) {
    console.log(`端口 ${p}:`, await probePort(p));
  }
  process.exit(0);
})();
