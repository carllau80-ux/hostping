// viewcheck.js 校验主机详情 Tab 三视图渲染（report/ai/hist）。
"use strict";
const fs = require("fs");
const path = require("path");
function stubEl() {
  const el = {
    style: {}, dataset: {}, classList: { add() {}, remove() {}, toggle() {}, contains() { return false; } },
    innerHTML: "", textContent: "", value: "",
    addEventListener() {}, querySelector() { return stubEl(); }, querySelectorAll() { return [] },
    appendChild() {}, onclick: null, focus() {}, close() {}, showModal() {}, remove() {}, click() {},
  };
  return el;
}
const elCache = new Map();
function cachedEl(sel) {
  if (!elCache.has(sel)) elCache.set(sel, stubEl());
  return elCache.get(sel);
}
const handler = {
  get(t, p) {
    if (p in t) return t[p];
    if (p === "querySelectorAll") return () => [];
    if (p === "querySelector" || p === "getElementById") return (sel) => cachedEl(String(sel));
    if (p === "addEventListener" || p === "removeEventListener") return () => {};
    if (p === "body" || p === "documentElement") return cachedEl("body");
    return cachedEl(String(p));
  },
};
global.document = new Proxy({ addEventListener() {}, readyState: "complete" }, handler);
global.window = new Proxy({ location: { reload() {} }, addEventListener() {} }, handler);
global.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
Object.defineProperty(global, "navigator", { value: { clipboard: { writeText: async () => {} } }, configurable: true });
const vm = require("vm");
vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../../frontend/dist/app.js"), "utf8"), { filename: "app.js" });

// 造数据：主机 + 报告 + AI 结果 + 历史
const host = { id: "h-1", name: "pve", host: "192.168.31.58", port: 22, user: "root", authType: "password" };
S.hosts = [host];
S.hostSel = "h-1";
S.hostReports["h-1"] = {
  hostId: "h-1", name: "pve", osName: "Debian 12", collectedAt: Date.now(),
  sysInfo: [{ key: "内核", value: "6.8" }], warnings: [],
};
S.aiReports["h-1"] = { markdown: "## 结论\n正常", html: "<h3>结论</h3><p>正常</p>", model: "glm-4-flash", elapsedMs: 3200, time: Date.now() };
S.hostHistory = [
  { time: Date.now(), report: S.hostReports["h-1"], ai: { html: "<p>AI 结论</p>", model: "glm-4-flash", time: Date.now() } },
  { time: Date.now() - 86400000, report: { osName: "Debian 11" }, ai: null },
];

let fail = 0;
function check(view, musts) {
  S.hostView = view; S.hostHistIdx = -1;
  renderHostView();
  const body = document.querySelector("#hostBody");
  const html = body ? body.innerHTML : "(no body)";
  for (const s of musts) {
    if (!html.includes(s)) { console.error(`[${view}] 缺失: ${s}`); fail++; }
  }
}
check("report", ["内核", "6.8"]);
check("ai", ["AI 分析报告", "glm-4-flash", "结论"]);
S.hostView = "hist"; renderHostView();
let body = document.querySelector("#hostBody");
if (!body.innerHTML.includes("Debian 12") || !body.innerHTML.includes("Debian 11")) { console.error("[hist] 列表缺失"); fail++; }
// 历史详情（选中第一条）
S.hostHistIdx = 0; renderHostView();
body = document.querySelector("#hostBody");
for (const s of ["返回历史列表", "AI 结论", "6.8"]) {
  if (!body.innerHTML.includes(s)) { console.error("[hist详情] 缺失:", s); fail++; }
}
if (fail === 0) console.log("主机 Tab 三视图渲染校验通过 ✓");
else { console.error("失败项:", fail); process.exit(1); }
