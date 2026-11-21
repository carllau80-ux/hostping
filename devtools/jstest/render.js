// render.js 渲染冒烟测试：用假 DOM 加载 app.js，跑 hostReportHTML 全路径。
// 用法: node devtools/jstest/render.js
"use strict";
const fs = require("fs");
const path = require("path");

// 万能元素桩：任何属性访问都返回可用对象
function stubEl() {
  const el = {
    style: {}, dataset: {}, classList: { add() {}, remove() {}, toggle() {}, contains() { return false; } },
    children: [], innerHTML: "", textContent: "", value: "", placeholder: "", title: "", src: "",
    addEventListener() {}, removeEventListener() {}, appendChild(c) { el.children.push(c); },
    querySelector() { return stubEl(); }, querySelectorAll() { return []; },
    closest() { return null; }, focus() {}, blur() {}, close() {}, showModal() {}, remove() {},
    setAttribute() {}, getAttribute() { return null; }, insertAdjacentHTML() {}, click() {},
    onclick: null, onchange: null, oninput: null, disabled: false, hidden: false,
  };
  return el;
}

const handler = {
  get(target, prop) {
    if (prop in target) return target[prop];
    if (prop === "querySelectorAll") return () => [];
    if (prop === "querySelector" || prop === "getElementById") return () => stubEl();
    if (prop === "createElement" || prop === "createDocumentFragment") return () => stubEl();
    if (prop === "addEventListener" || prop === "removeEventListener") return () => {};
    if (prop === "body" || prop === "documentElement") return stubEl();
    return stubEl();
  },
};
global.document = new Proxy({ addEventListener() {}, readyState: "complete" }, handler);
global.window = new Proxy({ location: { reload() {} }, addEventListener() {}, _editHostId: null, _editId: null }, handler);
Object.defineProperty(global, "navigator", { value: { clipboard: { writeText: async () => {} } }, configurable: true });
global.localStorage = { getItem() { return null; }, setItem() {}, removeItem() {} };
global.localStorage.getItem = () => null;
global.requestAnimationFrame = (f) => setTimeout(f, 0);
global.setInterval = () => 0;
global.fetch = async () => ({ ok: true, json: async () => ({}) });

// 加载 app.js（boot 里 API() 为空会直接 return，DOM 全是桩，不会崩）
const vm = require("vm");
const src = fs.readFileSync(path.join(__dirname, "../../frontend/dist/app.js"), "utf8");
vm.runInThisContext(src, { filename: "app.js" });

// 仿真巡检报告（覆盖全部渲染分支）
const report = {
  hostId: "h-1", name: "pve", host: "192.168.31.58", port: 22, user: "root",
  collectedAt: Date.now(),
  osName: "Debian GNU/Linux 12 (bookworm)", osKind: "linux",
  sysInfo: [
    { key: "发行版", value: "Debian GNU/Linux 12 (bookworm)" },
    { key: "内核", value: "6.8.4-2-pve" },
    { key: "__df", value: "Filesystem Type Size Used Avail Use% Mounted on\n/dev/sda1 ext4 100G 20G 80G 20% /" },
    { key: "__ip", value: "lo UNKNOWN 127.0.0.1/8 ::1/128\neth0 UP 192.168.31.58/24" },
  ],
  listenPorts: [
    { port: "22", state: "LISTEN", service: "SSH", process: "sshd", conns: "ESTAB 1" },
    { port: "111", state: "UDP", service: "RPCBind", process: "rpcbind", conns: "-" },
    { port: "5432", state: "LISTEN", service: "PostgreSQL", process: "postgres", conns: "" },
  ],
  topCpu: [
    { pid: "1", user: "root", cpu: "0.1", mem: "0.5", rss: "12345", vsz: "999", etime: "10-01:02:03", start: "Mon Nov 10 16:41:09 2026", cmd: "/sbin/init" },
    { pid: "1284", user: "root", cpu: "32.9", mem: "1.2", rss: "45678", vsz: "888", etime: "5-03:12:00", start: "Mon Nov 16 09:00:00 2026", cmd: "/opt/runstor/bin/rs_client --serve" },
  ],
  topMem: [{ pid: "2", user: "root", cpu: "0.0", mem: "1.0", rss: "22222", vsz: "888", etime: "10-01:02:03", start: "Mon Nov 10 16:41:09 2026", cmd: "kthreadd" }],
  rsProcs: [{
    ProcInfo: {}, pid: "1284", user: "root", cpu: "0.5", mem: "1.2", rss: "45678", etime: "5-03:12:00",
    cmd: "/opt/runstor/bin/rs_client", fdCount: "26",
    openFiles: [
      "anon_inode:[eventpoll]", "/dev/null", "/opt/runstor/db/client/client.db", "/opt/runstor/log/client/rs_client.log",
      "pipe:[512]", "socket:[1277561355]", "socket:[1277998672]", "socket:[1278007316]", "socket:[1278008934]",
    ],
    conns: [
      "ESTAB 0 0 192.168.2.128:43636 192.168.1.193:6102 users:((\"rs_client\",pid=1284,fd=20))",
      "ESTAB 0 0 192.168.2.128:49984 192.168.1.193:6102 users:((\"rs_client\",pid=1284,fd=14))",
      "LISTEN 0 4096 *:36500 *:* users:((\"rs_client\",pid=1284,fd=9))",
      "ESTAB 0 0 [::ffff:127.0.0.1]:36500 [::ffff:127.0.0.1]:46164 users:((\"rs_client\",pid=1284,fd=10))",
      "ESTAB 0 0 [::ffff:127.0.0.1]:36500 [::ffff:127.0.0.1]:40968 users:((\"rs_client\",pid=1284,fd=17))",
    ],
  }],
  security: [
    { key: "SELinux", value: "未安装/未启用" },
    { key: "__iptables", value: "-A INPUT -j ACCEPT" },
    { key: "firewalld", value: "未运行" },
  ],
  sysLog: { errCount: "2", warnCount: "5", topErrors: ["err1", "err2"], topWarns: [], dmesgErrs: ["dmesg err"], source: "journalctl" },
  storcli: {
    controllers: [{
      index: "0", model: "LSI MegaRAID 9361-8i", status: "Success",
      firmware: "5.130.00-2658", temp: "45C", memory: "2048 MB", battery: "Operational（95%）",
      vds: [{ id: "0", name: "data", type: "RAID-10", state: "Optl", size: "3.590 TB", cache: "RWBD" }],
      pds: [{ slot: "8:0", model: "ST18000NM000J", state: "Onln", size: "1.750 TB", sn: "ZL231X4A", power: "30241", detail: "HDD · SAS" }],
    }],
  },
  storcliNote: "",
  zfs: {
    poolCols: ["名称", "大小", "已用", "剩余", "碎片率", "健康状态"],
    pools: [["tank", "20G", "5G", "15G", "0%", "ONLINE"]],
    status: "pool: tank status: ONLINE",
    dsCols: ["数据集", "已用", "可用", "挂载点", "压缩", "压缩比", "重删", "重删比"],
    datasets: [["tank/data", "5G", "15G", "/tank/data", "lz4", "1.85x", "off", "1.00x"]],
  },
  zfsNote: "",
  lsscsi: "[0:0:0:0] disk ATA QEMU HARDDISK\n", lsscsiNote: "",
  lspci: "00:00.0 Host bridge: Intel\n", lspciNote: "",
  reboots: [
    { start: "Fri Nov 21 09:01", end: "-", dur: "still running" },
    { start: "Fri Nov 10 16:41", end: "20:37", dur: "3+20:56" },
  ],
  warnings: ["未发现 rs_/rsa_ 前缀进程"],
};

const html = hostReportHTML(report);
const mustHave = [
  "网络接口", "eth0", "🟢 UP",              // 网络接口表格（状态前置图标）
  "端口监听", "sshd", "ESTAB 1",            // 监听端口表格
  "安全状态", "SELinux", "-A INPUT -j ACCEPT", // 安全状态（含 __ 前缀 pre-block）
  "系统日志", "journalctl",                  // 系统日志
  "SCSI 设备", "PCI 设备",                   // lspci/lsscsi
  "Top 10", "/sbin/init", "启动时间",        // 进程表
  "rs_/rsa_",                               // rs 进程
  // RAID 格式化
  "LSI MegaRAID 9361-8i", "固件版本: 5.130.00-2658", "温度: 45C", "缓存内存: 2048 MB", "电池: Operational",
  "RAID-10", "RWBD",                        // VD 级别/缓存
  "ZL231X4A", "30241",                      // PD SN/通电时长
  "lz4", "1.85x", "off", "1.00x",           // ZFS 数据集压缩/重删
  "socket ×4", "pipe ×1", "普通文件",        // rs 打开文件分类
  "192.168.1.193", "ESTAB",                 // rs 连接聚合
  "重启记录", "Fri Nov 21 09:01", "运行中",   // 重启记录（最后）
];
// rs_/rsa_ 进程蓝色标识：topCpu 中 rs_client 标蓝、/sbin/init 不标蓝
const blue = `color:var(--brand);font-weight:600`;
if (!html.includes(`${blue}" title="/opt/runstor/bin/rs_client`)) { console.error("rs 进程未标蓝"); fail++; }
if (html.includes(`${blue}" title="/sbin/init"`)) { console.error("普通进程被误标蓝"); fail++; }
let fail = 0;
for (const s of mustHave) {
  if (!html.includes(s)) { console.error("缺失:", s); fail++; }
}
// 顺序断言：PCI < SCSI < 磁盘挂载 < 网络接口 < 端口监听 < 安全状态 < Top < rs < 系统日志(最后)
const order = ["PCI 设备", "SCSI 设备", "磁盘挂载", "网络接口", "端口监听", "安全状态", "Top 10 进程", "rs_/rsa_", "系统日志", "重启记录"];
let pos = -1;
for (const s of order) {
  const i = html.indexOf(s);
  if (i < 0 || i <= pos) { console.error("顺序错误:", s, "at", i, "prev", pos); fail++; }
  else pos = i;
}
if (fail === 0) {
  console.log("渲染冒烟测试通过 ✓  输出长度:", html.length);
} else {
  console.error("失败项:", fail);
  process.exit(1);
}
