// conncheck.js 定向校验 fmtConns 聚合（用户提供的真实 rs_client ss 输出形态）。
"use strict";
const fs = require("fs");
const path = require("path");
function stubEl() {
  const el = {
    style: {}, dataset: {}, classList: { add() {}, remove() {}, toggle() {}, contains() { return false; } },
    innerHTML: "", textContent: "", value: "",
    addEventListener() {}, querySelector() { return stubEl(); }, querySelectorAll() { return [] },
    appendChild() {}, onclick: null, focus() {}, close() {}, showModal() {}, remove() {},
  };
  return el;
}
const handler = {
  get(t, p) {
    if (p in t) return t[p];
    if (p === "querySelectorAll") return () => [];
    if (p === "querySelector" || p === "getElementById") return () => stubEl();
    if (p === "addEventListener" || p === "removeEventListener") return () => {};
    if (p === "body" || p === "documentElement") return stubEl();
    return stubEl();
  },
};
global.document = new Proxy({ addEventListener() {}, readyState: "complete" }, handler);
global.window = new Proxy({ location: { reload() {} }, addEventListener() {} }, handler);
global.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
Object.defineProperty(global, "navigator", { value: { clipboard: { writeText: async () => {} } }, configurable: true });
const vm = require("vm");
vm.runInThisContext(fs.readFileSync(path.join(__dirname, "../../frontend/dist/app.js"), "utf8"), { filename: "app.js" });

const conns = [
  `ESTAB 0 0 192.168.2.128:43636 192.168.1.193:6102 users:(("rs_client",pid=1284,fd=20))`,
  `ESTAB 0 0 192.168.2.128:49984 192.168.1.193:6102 users:((\"rs_client\",pid=1284,fd=14))`,
  `LISTEN 0 4096 *:36500 *:* users:((\"rs_client\",pid=1284,fd=9))`,
  `ESTAB 0 0 [::ffff:127.0.0.1]:36500 [::ffff:127.0.0.1]:46164 users:((\"rs_client\",pid=1284,fd=10))`,
  `ESTAB 0 0 [::ffff:127.0.0.1]:36500 [::ffff:127.0.0.1]:40968 users:((\"rs_client\",pid=1284,fd=17))`,
  `ESTAB 0 0 [::ffff:127.0.0.1]:36500 [::ffff:127.0.0.1]:42332 users:((\"rs_client\",pid=1284,fd=49))`,
  `ESTAB 0 0 [240e:39d:21:37d1:6d88:ce4a:6c46:6c8d]:59948 [240e:39d:21:37d0:f973:c1d3:ed02:8e2a]:6102 users:((\"rs_client\",pid=1284,fd=18))`,
];
const html = fmtConns(conns);
console.log(html.replace(/<\/tr>/g, "</tr>\n"));
const rows = (html.match(/<tr>/g) || []).length - 1; // 减表头
console.log("--- 聚合后行数:", rows, "(原始连接:", conns.length, ")");
if (rows !== 4) { console.error("聚合结果不符合预期（期望 4 行）"); process.exit(1); }
if (html.indexOf("[240e:39d:21:37d1:6d88:ce4a:6c46:6c8d]:59948") < 0) { console.error("IPv6 地址被错误截断"); process.exit(1); }
console.log("聚合校验通过 ✓");
