/* HostPing 前端逻辑（无框架，原生 JS；支持中英文与明暗主题） */
"use strict";

const $ = (s) => document.querySelector(s);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const API = () => window.go && window.go.app && window.go.app.App;

/* ---------------- i18n ---------------- */
const I18N = {
  zh: {
    "app.sub": "主机巡检与分析",
    "side.add": "＋ 新建连接", "side.pingAll": "▶ 全部 Ping", "side.settings": "⚙ 自动巡检", "side.exportAll": "⇓ 汇总报告",
    "dash.title": "总览仪表盘", "dash.searchPh": "🔍 搜索名称 / 主机…", "dash.allTypes": "全部类型", "dash.allGroups": "全部分组",
    "st.total": "连接总数", "st.ok": "正常", "st.err": "故障", "st.na": "未检测", "st.warn": "超时/异常",
    "empty.none": "还没有连接", "empty.hint": "点击\"＋ 新建连接\"开始", "empty.noc": "尚无连接，点击左上角 ＋ 新建连接", "empty.nomatch": "没有匹配的连接",
    "detail.back": "← 返回", "detail.ping": "▶ Ping", "detail.edit": "✎ 编辑", "detail.refresh": "⟳ 刷新", "detail.export": "⇓ 导出报告", "detail.del": "✕ 删除", "detail.delConfirm": "确认删除？",
    "tab.overview": "概览", "tab.topology": "拓扑图", "tab.tables": "大表", "tab.tbs": "表空间/文件", "tab.services": "集群/服务", "tab.dbs": "库/模式", "tab.history": "检查历史",    "dlg.newTitle": "新建连接", "dlg.editTitle": "编辑连接", "dlg.type": "数据库类型", "dlg.searchType": "🔍 搜索数据库…",
    "dlg.name": "显示名称", "dlg.namePh": "如：生产-订单库", "dlg.group": "分组", "dlg.groupPh": "如：生产环境",
    "dlg.port": "端口", "dlg.host": "主机", "dlg.hostPh": "IP 或主机名", "dlg.database": "数据库", "dlg.user": "用户名",
    "dlg.role": "连接身份", "dlg.roleNormal": "普通用户",
    "dlg.pass": "密码", "dlg.passKeep": "留空保持原密码", "dlg.params": "附加参数", "dlg.paramsPh": "如 sslmode=require（可空）",
    "dlg.timeout": "超时（秒）", "dlg.test": "🔌 测试", "dlg.save": "保存", "dlg.cancel": "取消",
    "dlg.file": "数据库文件", "dlg.pick": "📂 打开文件", "dlg.create": "🆕 新建文件",
    "dlg.tabBasic": "基本连接", "dlg.tabCert": "TLS 证书",
    "cert.mode": "TLS 模式", "cert.ca": "CA 证书", "cert.client": "客户端证书", "cert.key": "私钥",
    "cert.keypass": "私钥密码", "cert.sni": "服务器名称", "cert.skip": "跳过证书校验",
    "cert.hostcert": "服务器证书", "cert.default": "跟随驱动默认",
    "set.title": "自动巡检设置", "set.enable": "启用定时自动巡检", "set.advanced": "高级模式（启用 SQL 控制台，可执行增删改）", "set.interval": "巡检间隔（秒）", "set.timeout": "单次检查超时（秒）", "set.save": "保存",
    "tab.sql": "SQL 控制台", "tab.tablesFull": "数据表",
    "ov.archiveDest": "归档路径/对端",
    "sql.run": "▶ 执行", "sql.runConfirm": "该语句将修改数据，再次点击确认执行", "sql.limit": "行数/页", "sql.prev": "‹ 上一页", "sql.next": "下一页 ›",
    "sql.affected": "✓ 已执行，影响 {n} 行 · {ms}ms", "sql.ok": "✓ 执行成功 · {ms}ms",
    "sql.pageInfo": "共 {total} 行 · 第 {page} 页（每页 {size} 行）· {ms}ms",
    "sql.pageInfoNoTotal": "第 {page} 页（每页 {size} 行）· {ms}ms",
    "sql.truncated": "结果超过 500 行已截断，请用更精确的条件查询",
    "sql.empty": "（无结果）", "sql.hint": "Ctrl+Enter 执行；多条语句按顺序执行，出错即停止；SELECT 自动分页",
    "sql.multi": "共 {n} 条 · 成功 {ok} 条", "sql.stopped": "第 {i} 条出错，后续语句已停止", "sql.notRun": "未执行",
    "sql.first8": "仅展示前 8 行", "sql.query": "查询 {n} 行",
    "sql.browse": "数据浏览", "sql.browseBtn": "查询", "sql.hist": "历史 SQL", "sql.histClear": "清空",
    "tbl.filterPh": "按表名筛选…",
    "mode.db": "🗄 数据库", "mode.host": "🖥 主机", "side.addHost": "＋ 新建主机",
    "hostdash.title": "主机巡检",
    "host.statTotal": "主机总数", "host.statPending": "未巡检", "host.statOk": "巡检正常",
    "host.empty": "还没有主机，点击「＋ 新建主机」添加 SSH 目标",
    "host.lastInspect": "上次巡检",
    "host.inspect": "🔍 执行巡检", "host.inspecting": "正在巡检", "host.inspectDone": "巡检完成",
    "host.doneAt": "完成时间", "host.notInspected": "尚未执行巡检",
    "host.secSys": "系统软硬件信息", "host.secDisk": "磁盘挂载", "host.secNet": "网络接口",
    "host.secTopCpu": "Top 10 进程（按 CPU）", "host.secTopMem": "Top 10 进程（按内存）",
    "host.secRS": "rs_/rsa_ 进程巡检", "host.noRS": "未发现 rs_/rsa_ 前缀进程",
    "host.openFiles": "打开文件", "host.conns": "网络连接",
    "host.filesUnit": " 项", "host.filesOther": "匿名/事件", "host.filesReg": "普通文件",
    "host.localAddr": "本地地址", "host.remoteHost": "远端主机",
    "host.raidFw": "固件版本", "host.raidTemp": "温度", "host.raidMem": "缓存内存", "host.raidBattery": "电池",
    "host.secRaid": "RAID 卡（storcli64）", "host.controller": "控制器", "host.noRaid": "未检测到 RAID 卡或未安装 storcli64",
    "host.secZfs": "ZFS 存储池", "host.noZfs": "未安装 ZFS",
    "host.newTitle": "新建主机", "host.editTitle": "编辑主机",
    "host.user": "SSH 用户", "host.authType": "认证方式", "host.authPwd": "密码", "host.authKey": "私钥",
    "host.keyPath": "私钥文件", "host.hint": "巡检为只读操作：仅执行 ps/df/ls 等查看命令与 storcli64 /cALL show、zpool list/status、zfs list",
    "pingdlg.title": "全部 Ping", "pingdlg.queued": "排队中", "pingdlg.testing": "检测中…",
    "dlg.close": "关闭",
    "pg.pager": { prev: "‹ 上一页", next: "下一页 ›", page: "第 {p} 页 / 共 {t} 条" },
    "ov.version": "版本", "ov.instance": "实例", "ov.hostname": "主机名", "ov.role": "角色", "ov.readonly": "只读",
    "ov.os": "操作系统",
    "ov.archive": "归档/日志模式", "ov.charset": "字符集", "ov.tz": "时区", "ov.startup": "启动时间", "ov.uptime": "已运行",
    "ov.dbsize": "数据库大小", "ov.objects": "对象数量", "ov.conns": "连接数（当前/最大）", "ov.cluster": "集群形态",
    "ov.tips": "采集提示（权限不足的视图已跳过）", "ov.archiveHint": "开启归档/日志参考命令",
    "yes": "是", "no": "否",
    "topo.expandAll": "⤢ 全部展开", "topo.collapseAll": "⤡ 全部收起", "topo.expandTip": "双击行也可展开/收起",
    "err.title": "连接失败", "err.detail": "原始错误", "err.sugT": "排查建议", "err.copy": "复制详情", "err.copied": "已复制到剪贴板", "err.close": "关闭",
    "err.failedList": "以下连接检查失败：",
    "errk.network": "网络不通", "errk.auth": "认证失败", "errk.permission": "权限不足", "errk.timeout": "超时",
    "errk.protocol": "协议不兼容", "errk.config": "配置错误", "errk.unknown": "失败",
    "err.sug.network": "1) 确认 IP 与端口正确、数据库服务已启动；2) 检查防火墙/安全组是否放行该端口；3) 数据库可能仅监听本机（如 PG listen_addresses、MySQL bind-address、Oracle 监听器），需开启远程监听。",
    "err.sug.auth": "1) 核对用户名与密码；2) MongoDB 确认 authSource（默认 admin）；3) Oracle 用户名通常需大写（除非建号时加引号）；4) 检查账号是否锁定或未授予登录权限。",
    "err.sug.permission": "1) 使用具备系统视图权限的账号（Oracle 需 SELECT ANY DICTIONARY 或 DBA 角色；SQL Server 需 VIEW SERVER STATE；PG 建议可读 pg_catalog）；2) 权限不足时 HostPing 会自动降级采集，仅部分信息缺失。",
    "err.sug.timeout": "1) 网络质量差或服务器负载高，可在连接设置中调大超时时间；2) 检查跨网段路由/VPN 稳定性。",
    "err.sug.protocol": "1) 数据库版本过旧或协议与驱动不兼容；2) 部分国产库需调整服务端兼容参数（如 GBase 8a 握手差异、MySQL caching_sha2）。",
    "err.sug.config": "1) 核对服务名/SID/库名/模式名是否正确；2) Oracle 用 lsnrctl status 检查监听器注册的服务名。",
    "err.sug.unknown": "请查看下方原始错误信息定位问题；必要时检查 HostPing 日志与网络环境。",
    "layer.instance": "实例层", "layer.cluster": "集群/复制层", "layer.db": "数据库层", "layer.tbs": "表空间层", "layer.table": "表层",
    "th.schema": "模式", "th.name": "名称", "th.rows": "行数(估算)", "th.size": "大小", "th.comment": "备注",
    "th.free": "剩余", "th.status": "状态", "th.files": "文件", "th.role": "角色", "th.detail": "详情", "th.host": "主机",
    "th.tables": "表数", "th.charset": "字符集", "th.note": "备注", "th.time": "时间", "th.result": "结果", "th.tcp": "TCP(ms)", "th.sql": "SQL(ms)", "th.desc": "说明",
    "hist.records": "记录数", "hist.rate": "可用率（本次会话）", "hist.avgsql": "最近平均 SQL 延迟",
    "auto.on": "自动巡检：开启（每 {n}s）", "auto.off": "自动巡检：关闭",
    "toast.saved": "已保存", "toast.saveFail": "保存失败: ", "toast.loadFail": "数据加载失败: ", "toast.api": "后端 API 未就绪",
    "toast.refreshed": "已重新连接（缓存已刷新）",
    "toast.welcome": "欢迎使用 HostPing！先新建一个数据库连接", "toast.deleted": "已删除",
    "toast.pingOk": "Ping 正常　TCP {t}ms · SQL {s}ms", "toast.pingFail": "Ping 失败",
    "toast.done": "巡检完成：{ok}/{n} 正常", "toast.checkFail": "巡检失败: ", "toast.exportOk": "报告已导出：", "toast.exportFail": "导出失败: ",
    "toast.exportCancel": "导出失败: ", "toast.scriptErr": "脚本错误: ", "toast.asyncErr": "异步错误: ",
    "toast.setSaved": "设置已保存", "toast.noDetail": "无数据（需指定数据库）", "toast.noTbs": "无表空间信息（该数据库类型可能不适用）",
    "toast.noSvc": "无集群/复制信息（单机或权限不足）", "toast.noData": "无数据", "toast.noHist": "暂无检查记录，点击\"▶ Ping\"或开启自动巡检",
    "toast.loadErr": "加载失败：",
    "welcome.empty": "还没有连接",
    "app.sub2": "主机巡检与分析", "logo.title": "HostPing",
    "mode.db": "🗄 数据库", "mode.host": "🖥 主机", "side.addHost": "＋ 新建主机",
    "hostdash.title": "主机巡检",
    "host.statTotal": "主机总数", "host.statPending": "未巡检", "host.statOk": "巡检正常",
    "host.empty": "还没有主机，点击「＋ 新建主机」添加 SSH 目标",
    "host.lastInspect": "上次巡检",
    "host.inspect": "🔍 执行巡检", "host.inspecting": "正在巡检", "host.inspectDone": "巡检完成",
    "host.doneAt": "完成时间", "host.notInspected": "尚未执行巡检",
    "host.secSys": "系统软硬件信息", "host.secDisk": "磁盘挂载", "host.secNet": "网络接口",
    "host.secTopCpu": "Top 10 进程（按 CPU）", "host.secTopMem": "Top 10 进程（按内存）",
    "host.secRS": "rs_/rsa_ 进程巡检", "host.noRS": "未发现 rs_/rsa_ 前缀进程",
    "host.openFiles": "打开文件", "host.conns": "网络连接",
    "host.filesUnit": " 项", "host.filesOther": "匿名/事件", "host.filesReg": "普通文件",
    "host.localAddr": "本地地址", "host.remoteHost": "远端主机",
    "host.raidFw": "固件版本", "host.raidTemp": "温度", "host.raidMem": "缓存内存", "host.raidBattery": "电池",
    "host.secRaid": "RAID 卡（storcli64）", "host.controller": "控制器", "host.noRaid": "未检测到 RAID 卡或未安装 storcli64",
    "host.secZfs": "ZFS 存储池", "host.noZfs": "未安装 ZFS",
    "host.newTitle": "新建主机", "host.editTitle": "编辑主机",
    "host.user": "SSH 用户", "host.authType": "认证方式", "host.authPwd": "密码", "host.authKey": "私钥",
    "host.keyPath": "私钥文件", "host.hint": "巡检为只读操作：仅执行 ps/df/ls 等查看命令与 storcli64 /cALL show、zpool list/status、zfs list",
    "host.testConn": "🔌 测试连接",
    "host.secSecurity": "安全状态（SELinux/防火墙）",
    "host.port": "端口", "host.state": "状态", "host.service": "服务", "host.process": "进程",
    "detail.backShort": "返回",
    "host.secLog": "系统日志检查与分析", "host.logErrors": "错误条目", "host.logWarns": "警告条目",
    "host.logSource": "日志来源", "host.dmesg": "内核日志（dmesg）", "host.recentErrors": "最近错误日志",
    "host.lsscsi": "SCSI 设备（lsscsi）", "host.lspci": "PCI 设备（lspci）",
    "host.inspectBtn": "执行巡检",
    "host.secPorts": "端口监听", "host.connCount": "连接数",
    "host.badgeWarn": "正常（有警告）",
    "host.secReboot": "重启记录（最近 10 次）", "host.rbStart": "启动时间", "host.rbEnd": "结束时间",
    "host.rbDur": "本次运行时长", "host.rbRunning": "运行中", "host.rbNone": "未采集到重启记录（last 命令不可用或无记录）",
    "host.step.sysinfo": "系统信息", "host.step.ports": "端口监听", "host.step.top": "Top 进程", "host.step.rs": "rs_/rsa_ 进程",
    "host.step.syslog": "系统日志", "host.step.security": "安全状态", "host.step.storcli": "RAID（storcli）", "host.step.zfs": "ZFS",
    "host.step.lsscsi": "SCSI 设备", "host.step.lspci": "PCI 设备", "host.step.reboot": "重启记录",
    "ai.cfgTitle": "🤖 AI 分析设置", "ai.provider": "提供商", "ai.model": "模型名", "ai.baseUrl": "API Base URL",
    "ai.key": "API Key", "ai.keyKeep": "留空保持原 Key", "ai.custom": "自定义（OpenAI 兼容）",
    "ai.local": "本地模型（Ollama / LM Studio）",
    "ai.hint": "云端大模型均为 OpenAI 兼容接口，选择提供商后自动预填模型名；本地模型（Ollama 默认 http://localhost:11434/v1）无需 API Key；Key 在本机加密存储，不会上传。",
    "ai.analyze": "🤖 AI 分析", "ai.analyzing": "AI 分析中…", "ai.noReport": "请先执行巡检生成报告",
    "ai.done": "AI 分析完成", "ai.section": "AI 分析报告", "ai.export": "⇓ 导出 HTML",
    "ai.tab": "🤖 AI 分析", "ai.history": "历史分析（点击回看）", "ai.elapsed": "已用时", "ai.pleaseWait": "大模型生成需要一些时间，请稍候…",
    "host.tabReport": "巡检报告", "host.tabHist": "历史", "host.backToList": "返回历史列表", "host.histEmpty": "暂无历史巡检记录",
    "side.settingsLabel": "设置",
    "spd.title": "🚀 测速", "spd.newTitle": "测速（iperf3）", "spd.server": "测速服务器", "spd.port": "服务端口",
    "spd.duration": "测试时长（秒）", "spd.parallel": "并行流数", "spd.dir": "测试方向",
    "spd.up": "上行（本机 → 服务端）", "spd.down": "下行（服务端 → 本机）",
    "spd.hint": "在目标服务器运行 iperf3 -s 后即可测速；纯 Go 客户端实现，无需本机安装 iperf3",
    "spd.start": "开始测速", "spd.running": "测速中，请稍候…", "spd.sec": "时间", "spd.rate": "速率",
    "spd.omitted": "预热", "spd.sentLabel": "上行", "spd.recvLabel": "下行", "spd.serverSide": "对端实测", "spd.retrans": "重传",
    "spd.history": "测速历史（点击回看）", "spd.noHistory": "暂无测速记录", "spd.export": "导出报告", "spd.del": "删除", "spd.deleted": "已删除",
    "nmap.title": "🔎 端口扫描", "nmap.newTitle": "端口扫描（类 nmap）",
    "nmap.target": "目标主机", "nmap.targetPh": "IP 或主机名，如 192.168.1.10",
    "nmap.portSpec": "端口范围", "nmap.portSpecPh": "如 22,80,443 或 1-1024",
    "nmap.scanBtn": "开始扫描", "nmap.scanning": "扫描中…",
    "nmap.done": "扫描完成：{ok} 个开放 / 共 {n} 个端口",
    "nmap.os": "系统推测", "nmap.osBasis": "依据", "nmap.osNone": "未能推测目标系统（ICMP 被过滤且无服务 Banner）",
    "nmap.history": "扫描记录（点击回看）", "nmap.noHistory": "暂无扫描记录",
    "nmap.hint": "从本机直接发起 TCP 连接探测与系统推测，无需登录目标主机；最多 2000 个端口",
  },
  en: {
    "app.sub": "Host & DB Inspection & Analysis",
    "side.add": "＋ New Connection", "side.pingAll": "▶ Ping All", "side.settings": "⚙ Auto Check", "side.exportAll": "⇓ Summary",
    "dash.title": "Dashboard", "dash.searchPh": "🔍 Search name / host…", "dash.allTypes": "All Types", "dash.allGroups": "All Groups",
    "st.total": "Connections", "st.ok": "OK", "st.err": "Failed", "st.na": "Unchecked", "st.warn": "Timeout",
    "empty.none": "No connections yet", "empty.hint": "Click \"＋ New Connection\" to start", "empty.noc": "No connections yet — click ＋ New Connection", "empty.nomatch": "No matching connections",
    "detail.back": "← Back", "detail.ping": "▶ Ping", "detail.edit": "✎ Edit", "detail.refresh": "⟳ Refresh", "detail.export": "⇓ Export", "detail.del": "✕ Delete", "detail.delConfirm": "Confirm delete?",
    "tab.overview": "Overview", "tab.topology": "Topology", "tab.tables": "Top Tables", "tab.tbs": "Tablespaces", "tab.services": "Cluster/Services", "tab.dbs": "Databases", "tab.history": "History",
    "dlg.newTitle": "New Connection", "dlg.editTitle": "Edit Connection", "dlg.type": "Database Type", "dlg.searchType": "🔍 Search database…",
    "dlg.name": "Display Name", "dlg.namePh": "e.g. PROD-orders", "dlg.group": "Group", "dlg.groupPh": "e.g. Production",
    "dlg.port": "Port", "dlg.host": "Host", "dlg.hostPh": "IP or hostname", "dlg.database": "Database", "dlg.user": "Username",
    "dlg.role": "Connect as", "dlg.roleNormal": "Normal user",
    "dlg.pass": "Password", "dlg.passKeep": "Leave blank to keep", "dlg.params": "Extra Params", "dlg.paramsPh": "e.g. sslmode=require (optional)",
    "dlg.timeout": "Timeout (s)", "dlg.test": "Test", "dlg.save": "Save", "dlg.cancel": "Cancel",
    "dlg.file": "Database file", "dlg.pick": "📂 Open file", "dlg.create": "🆕 New file",
    "dlg.tabBasic": "Basic", "dlg.tabCert": "TLS & Cert",
    "cert.mode": "TLS mode", "cert.ca": "CA certificate", "cert.client": "Client certificate", "cert.key": "Private key",
    "cert.keypass": "Key password", "cert.sni": "Server name", "cert.skip": "Skip verify",
    "cert.hostcert": "Server certificate", "cert.default": "Driver default",
    "set.title": "Auto Check Settings", "set.enable": "Enable scheduled auto check", "set.advanced": "Advanced mode (SQL console with CRUD)", "set.interval": "Interval (seconds)", "set.timeout": "Per-check timeout (s)", "set.save": "Save",
    "tab.sql": "SQL Console", "tab.tablesFull": "Tables",
    "ov.archiveDest": "Archive Destination / Peer",
    "sql.run": "▶ Run", "sql.runConfirm": "This statement modifies data — click again to confirm", "sql.limit": "Rows/page", "sql.prev": "‹ Prev", "sql.next": "Next ›",
    "sql.affected": "✓ Executed, {n} rows affected · {ms}ms", "sql.ok": "✓ Executed · {ms}ms",
    "sql.pageInfo": "{total} rows · page {page} ({size}/page) · {ms}ms",
    "sql.pageInfoNoTotal": "page {page} ({size}/page) · {ms}ms",
    "sql.truncated": "Result truncated to 500 rows — refine your query",
    "sql.empty": "(no results)", "sql.hint": "Ctrl+Enter to run; multiple statements run in order, stop on first error; SELECT auto-paginated",
    "sql.multi": "{n} statements · {ok} ok", "sql.stopped": "failed at #{i}, remaining skipped", "sql.notRun": "not executed",
    "sql.first8": "showing first 8 rows", "sql.query": "{n} rows",
    "sql.browse": "Browse data", "sql.browseBtn": "Query", "sql.hist": "History", "sql.histClear": "Clear",
    "tbl.filterPh": "Filter by table name…",
    "mode.db": "🗄 Databases", "mode.host": "🖥 Hosts", "side.addHost": "＋ New Host",
    "hostdash.title": "Host Inspection",
    "host.statTotal": "Hosts", "host.statPending": "Pending", "host.statOk": "OK",
    "host.empty": "No hosts yet — click ＋ New Host to add an SSH target",
    "host.lastInspect": "Last inspect",
    "host.inspect": "🔍 Inspect", "host.inspecting": "Inspecting", "host.inspectDone": "Inspection done",
    "host.doneAt": "Finished at", "host.notInspected": "Not inspected yet",
    "host.secSys": "System HW/SW Info", "host.secDisk": "Disk Mounts", "host.secNet": "Network Interfaces",
    "host.secTopCpu": "Top 10 Processes (CPU)", "host.secTopMem": "Top 10 Processes (Memory)",
    "host.secRS": "rs_/rsa_ Process Inspection", "host.noRS": "No rs_/rsa_ prefixed processes found",
    "host.openFiles": "Open files", "host.conns": "Network connections",
    "host.filesUnit": "", "host.filesOther": "anon/epoll", "host.filesReg": "Regular files",
    "host.localAddr": "Local", "host.remoteHost": "Remote host",
    "host.raidFw": "Firmware", "host.raidTemp": "Temp", "host.raidMem": "Cache memory", "host.raidBattery": "Battery",
    "host.secRaid": "RAID Controller (storcli64)", "host.controller": "Controller", "host.noRaid": "No RAID card or storcli64 not installed",
    "host.secZfs": "ZFS Pools", "host.noZfs": "ZFS not installed",
    "host.newTitle": "New Host", "host.editTitle": "Edit Host",
    "host.user": "SSH User", "host.authType": "Auth Type", "host.authPwd": "Password", "host.authKey": "Private Key",
    "host.keyPath": "Private key file", "host.hint": "Inspection is read-only: only ps/df/ls style commands and storcli64 /cALL show, zpool list/status, zfs list",
    "pingdlg.title": "Ping All", "pingdlg.queued": "Queued", "pingdlg.testing": "Testing…",
    "dlg.close": "Close",
    "ov.version": "Version", "ov.instance": "Instance", "ov.hostname": "Hostname", "ov.role": "Role", "ov.readonly": "Read Only",
    "ov.os": "Server OS",
    "ov.archive": "Archive/Log Mode", "ov.charset": "Charset", "ov.tz": "Timezone", "ov.startup": "Startup Time", "ov.uptime": "Uptime",
    "ov.dbsize": "DB Size", "ov.objects": "Objects", "ov.conns": "Connections (cur/max)", "ov.cluster": "Cluster",
    "ov.tips": "Collection notes (views skipped due to privileges)", "ov.archiveHint": "How to enable archiving (examples)",
    "yes": "Yes", "no": "No",
    "topo.expandAll": "⤢ Expand all", "topo.collapseAll": "⤡ Collapse all", "topo.expandTip": "Double-click a row to toggle too",
    "err.title": "Connection Failed", "err.detail": "Raw error", "err.sugT": "Troubleshooting", "err.copy": "Copy details", "err.copied": "Copied to clipboard", "err.close": "Close",
    "err.failedList": "The following connections failed:",
    "errk.network": "Network", "errk.auth": "Authentication", "errk.permission": "Permission", "errk.timeout": "Timeout",
    "errk.protocol": "Protocol", "errk.config": "Configuration", "errk.unknown": "Failed",
    "err.sug.network": "1) Verify IP/port and that the service is running; 2) Check firewall/security group rules; 3) The server may listen on localhost only (e.g. PG listen_addresses, MySQL bind-address, Oracle listener) — enable remote listening.",
    "err.sug.auth": "1) Check username/password; 2) For MongoDB verify authSource (default admin); 3) Oracle usernames are usually UPPERCASE unless quoted; 4) Check if the account is locked or lacks CONNECT privilege.",
    "err.sug.permission": "1) Use an account with dictionary privileges (Oracle: SELECT ANY DICTIONARY or DBA; SQL Server: VIEW SERVER STATE; PG: readable pg_catalog); 2) HostPing degrades gracefully when views are inaccessible.",
    "err.sug.timeout": "1) Poor network or high server load — increase the timeout in connection settings; 2) Check cross-segment routing/VPN stability.",
    "err.sug.protocol": "1) Database version too old or protocol incompatible with the driver; 2) Some databases need server-side compatibility params (e.g. GBase 8a handshake, MySQL caching_sha2).",
    "err.sug.config": "1) Verify service name/SID/database/schema; 2) For Oracle check registered services via lsnrctl status.",
    "err.sug.unknown": "See the raw error below; check the HostPing log and network environment if needed.",
    "layer.instance": "Instance", "layer.cluster": "Cluster/Replication", "layer.db": "Databases", "layer.tbs": "Tablespaces", "layer.table": "Tables",
    "th.schema": "Schema", "th.name": "Name", "th.rows": "Rows (est.)", "th.size": "Size", "th.comment": "Comment",
    "th.free": "Free", "th.status": "Status", "th.files": "Files", "th.role": "Role", "th.detail": "Detail", "th.host": "Host",
    "th.tables": "Tables", "th.charset": "Charset", "th.note": "Note", "th.time": "Time", "th.result": "Result", "th.tcp": "TCP(ms)", "th.sql": "SQL(ms)", "th.desc": "Detail",
    "hist.records": "Records", "hist.rate": "Availability (session)", "hist.avgsql": "Avg SQL Latency",
    "auto.on": "Auto check: on (every {n}s)", "auto.off": "Auto check: off",
    "toast.saved": "Saved", "toast.saveFail": "Save failed: ", "toast.loadFail": "Load failed: ", "toast.api": "Backend API not ready",
    "toast.refreshed": "Reconnected (cache refreshed)",
    "toast.welcome": "Welcome to HostPing! Create a connection to start", "toast.deleted": "Deleted",
    "toast.pingOk": "Ping OK　TCP {t}ms · SQL {s}ms", "toast.pingFail": "Ping failed",
    "toast.done": "Check done: {ok}/{n} OK", "toast.checkFail": "Check failed: ", "toast.exportOk": "Report exported: ", "toast.exportFail": "Export failed: ",
    "toast.exportCancel": "Export failed: ", "toast.scriptErr": "Script error: ", "toast.asyncErr": "Async error: ",
    "toast.setSaved": "Settings saved", "toast.noDetail": "No data (database required)", "toast.noTbs": "No tablespace info (N/A for this type)",
    "toast.noSvc": "No cluster/replication info (standalone or no privilege)", "toast.noData": "No data", "toast.noHist": "No history yet — click \"▶ Ping\" or enable auto check",
    "toast.loadErr": "Load failed: ",
    "welcome.empty": "No connections yet",
    "app.sub2": "Host & DB Inspection & Analysis", "logo.title": "HostPing",
    "mode.db": "🗄 Databases", "mode.host": "🖥 Hosts", "side.addHost": "＋ New Host",
    "hostdash.title": "Host Inspection",
    "host.statTotal": "Hosts", "host.statPending": "Pending", "host.statOk": "OK",
    "host.empty": "No hosts yet — click ＋ New Host to add an SSH target",
    "host.lastInspect": "Last inspect",
    "host.inspect": "🔍 Inspect", "host.inspecting": "Inspecting", "host.inspectDone": "Inspection done",
    "host.doneAt": "Finished at", "host.notInspected": "Not inspected yet",
    "host.secSys": "System HW/SW Info", "host.secDisk": "Disk Mounts", "host.secNet": "Network Interfaces",
    "host.secTopCpu": "Top 10 Processes (CPU)", "host.secTopMem": "Top 10 Processes (Memory)",
    "host.secRS": "rs_/rsa_ Process Inspection", "host.noRS": "No rs_/rsa_ prefixed processes found",
    "host.openFiles": "Open files", "host.conns": "Network connections",
    "host.filesUnit": "", "host.filesOther": "anon/epoll", "host.filesReg": "Regular files",
    "host.localAddr": "Local", "host.remoteHost": "Remote host",
    "host.raidFw": "Firmware", "host.raidTemp": "Temp", "host.raidMem": "Cache memory", "host.raidBattery": "Battery",
    "host.secRaid": "RAID Controller (storcli64)", "host.controller": "Controller", "host.noRaid": "No RAID card or storcli64 not installed",
    "host.secZfs": "ZFS Pools", "host.noZfs": "ZFS not installed",
    "host.newTitle": "New Host", "host.editTitle": "Edit Host",
    "host.user": "SSH User", "host.authType": "Auth Type", "host.authPwd": "Password", "host.authKey": "Private Key",
    "host.keyPath": "Private key file", "host.hint": "Inspection is read-only: only ps/df/ls style commands and storcli64 /cALL show, zpool list/status, zfs list",
    "host.testConn": "🔌 Test connection",
    "host.secSecurity": "Security (SELinux/Firewall)",
    "host.port": "Port", "host.state": "State", "host.service": "Service", "host.process": "Process",
    "host.secLog": "System Log Analysis", "host.logErrors": "Errors", "host.logWarns": "Warnings",
    "host.logSource": "Source", "host.dmesg": "Kernel Log (dmesg)",
    "host.inspectBtn": "Inspect",
    "host.secPorts": "Listening Ports", "host.connCount": "Connections",
    "host.badgeWarn": "OK (warnings)",
    "host.secReboot": "Reboots (last 10)", "host.rbStart": "Boot time", "host.rbEnd": "Shutdown",
    "host.rbDur": "Uptime", "host.rbRunning": "running", "host.rbNone": "No reboot records (last unavailable or empty)",
    "host.step.sysinfo": "System info", "host.step.ports": "Listening ports", "host.step.top": "Top processes", "host.step.rs": "rs_/rsa_ procs",
    "host.step.syslog": "System log", "host.step.security": "Security", "host.step.storcli": "RAID (storcli)", "host.step.zfs": "ZFS",
    "host.step.lsscsi": "SCSI devices", "host.step.lspci": "PCI devices", "host.step.reboot": "Reboots",
    "ai.cfgTitle": "🤖 AI Analysis Settings", "ai.provider": "Provider", "ai.model": "Model", "ai.baseUrl": "API Base URL",
    "ai.key": "API Key", "ai.keyKeep": "Leave empty to keep", "ai.custom": "Custom (OpenAI compatible)",
    "ai.local": "Local models (Ollama / LM Studio)",
    "ai.hint": "Cloud providers use OpenAI-compatible APIs with pre-filled model names; local models (Ollama default http://localhost:11434/v1) need no API key; keys are stored encrypted on this machine.",
    "ai.analyze": "🤖 AI Analysis", "ai.analyzing": "Analyzing…", "ai.noReport": "Run an inspection first",
    "ai.done": "AI analysis done", "ai.section": "AI Analysis Report", "ai.export": "⇓ Export HTML",
    "ai.tab": "🤖 AI Analysis", "ai.history": "History (click to review)", "ai.elapsed": "Elapsed", "ai.pleaseWait": "LLM generation takes a while, please wait…",
    "host.tabReport": "Inspection Report", "host.tabHist": "History", "host.backToList": "Back to history list", "host.histEmpty": "No inspection history yet",
    "side.settingsLabel": "Settings",
    "spd.title": "🚀 Speed Test", "spd.newTitle": "Speed Test (iperf3)", "spd.server": "Server", "spd.port": "Port",
    "spd.duration": "Duration (s)", "spd.parallel": "Parallel streams", "spd.dir": "Direction",
    "spd.up": "Upload (this PC → server)", "spd.down": "Download (server → this PC)",
    "spd.hint": "Run `iperf3 -s` on the target server; pure-Go client, no local iperf3 needed",
    "spd.start": "Start", "spd.running": "Testing, please wait…", "spd.sec": "Time", "spd.rate": "Rate",
    "spd.omitted": "omit", "spd.sentLabel": "upload", "spd.recvLabel": "download", "spd.serverSide": "server-side", "spd.retrans": "Retransmits",
    "spd.history": "Speed test history (click to review)", "spd.noHistory": "No speed test records yet", "spd.export": "Export report", "spd.del": "Delete", "spd.deleted": "Deleted",
    "nmap.title": "🔎 Port Scan", "nmap.newTitle": "Port Scan (nmap-like)",
    "nmap.target": "Target Host", "nmap.targetPh": "IP or hostname, e.g. 192.168.1.10",
    "nmap.portSpec": "Port Range", "nmap.portSpecPh": "e.g. 22,80,443 or 1-1024",
    "nmap.scanBtn": "Scan", "nmap.scanning": "Scanning…",
    "nmap.done": "Scan done: {ok} open / {n} ports",
    "nmap.os": "OS guess", "nmap.osBasis": "basis", "nmap.osNone": "OS undetectable (ICMP filtered, no service banner)",
    "nmap.history": "Scan history (click to review)", "nmap.noHistory": "No scan history yet",
    "nmap.hint": "TCP connect probe and OS guess launched from this machine; no login required on the target; up to 2000 ports",
  },
};
let LANG = localStorage.getItem("dbping.lang")
  || ((navigator.language || "zh-CN").toLowerCase().startsWith("zh") ? "zh" : "en");
function t(key, vars) {
  let s = (I18N[LANG] && I18N[LANG][key]) || I18N.zh[key] || key;
  if (vars) for (const k of Object.keys(vars)) s = s.replaceAll("{" + k + "}", vars[k]);
  return s;
}

/* ---------------- 主题 ---------------- */
let THEME = localStorage.getItem("dbping.theme") || "light";
function applyTheme() {
  document.documentElement.dataset.theme = THEME;
  $("#btnTheme").textContent = THEME === "dark" ? "☀️" : "🌙";
  localStorage.setItem("dbping.theme", THEME);
}
function applyLang() {
  document.documentElement.lang = LANG === "zh" ? "zh-CN" : "en";
  $("#btnLang").textContent = LANG === "zh" ? "🌐 EN" : "🌐 中文";
  localStorage.setItem("dbping.lang", LANG);
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const v = t(el.dataset.i18n);
    if (el.childNodes.length && el.childNodes[0].nodeType === 3) el.childNodes[0].textContent = v;
    else el.textContent = v;
  });
  document.querySelectorAll("[data-i18n-ph]").forEach((el) => { el.placeholder = t(el.dataset.i18nPh); });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => { el.title = t(el.dataset.i18nTitle); });
  $("#filterType").options[0].textContent = t("dash.allTypes");
  $("#filterGroup").options[0].textContent = t("dash.allGroups");
}

/* ---------------- 全局状态 ---------------- */
const S = {
  types: [], conns: [], latest: {}, settings: null,
  sel: null, tab: "overview",
  search: "", fType: "", fGroup: "",
  detailCache: {}, topoChart: null, histChart: null,
  deleteArmed: null, pinging: false,
  // 主机巡检模式
  mode: "db", hosts: [], hostSel: null, hostReports: {}, hostBusy: false, hostDeleteArmed: null,
  aiReports: {}, dbAIReports: {}, aiBusy: false,
  hostView: "report", hostHistIdx: -1, hostHistory: [],
  iperfBusy: false,
};

/* ---------------- 工具 ---------------- */
function toast(msg, kind = "") {
  const el = document.createElement("div");
  el.className = "toast " + kind;
  el.textContent = msg;
  $("#toasts").appendChild(el);
  setTimeout(() => el.remove(), 3600);
}
function typeOf(t2) { return S.types.find((x) => x.type === t2) || { icon: "🗄", name: t2, port: 0, dbLabel: "Database", userLabel: "Username", note: "", vendor: "" }; }
function statusOf(id) {
  const r = S.latest[id];
  if (!r) return "na";
  return r.ok ? "ok" : (r.errorKind === "timeout" ? "warn" : "err");
}
function fmtLat(r) { return r && r.ok ? `TCP ${r.tcpMs}ms · SQL ${r.sqlMs}ms` : "—"; }

/* ---------------- 错误对话框 ---------------- */
let _lastErrText = "";
function kindLabel(kind) { return t("errk." + (kind || "unknown")); }

function copyText(s) {
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(s);
  }
  const ta = document.createElement("textarea");
  ta.value = s;
  document.body.appendChild(ta);
  ta.select();
  document.execCommand("copy");
  ta.remove();
  return Promise.resolve();
}

// showError 以对话框展示失败详情：错误分类 + 原始错误 + 按类型的排查建议
function showError(titleText, info) {
  const kind = info.kind || "unknown";
  $("#errTitle").textContent = titleText;
  $("#errMeta").textContent = [info.meta, info.stage ? (LANG === "zh" ? "阶段: " + info.stage : "stage: " + info.stage) : ""].filter(Boolean).join("　·　");
  $("#errKind").textContent = kindLabel(kind);
  $("#errMsg").textContent = info.msg || "-";
  $("#errSug").textContent = t("err.sug." + kind);
  _lastErrText = [titleText, info.meta, kindLabel(kind), info.msg, "—", t("err.sug." + kind)]
    .filter(Boolean).join("\n");
  $("#dlgError").showModal();
}

/* ---------------- 数据加载 ---------------- */
async function reload() {
  const A = API(); if (!A) { toast(t("toast.api"), "err"); return; }
  S.types = (await A.GetTypes()) || [];
  S.conns = (await A.GetConnections()) || [];
  S.hosts = (await A.GetSSHHosts()) || [];
  S.latest = (await A.GetLatestResults()) || {};
  S.settings = (await A.GetSettings()) || { autoPing: false, intervalSec: 60, pingTimeout: 5 };
  renderAll();
}

/* ---------------- 模式切换 ---------------- */
function switchMode(mode) {
  S.mode = mode;
  $("#modeDb").classList.toggle("active", mode === "db");
  $("#modeHost").classList.toggle("active", mode === "host");
  $("#btnAdd").style.display = mode === "db" ? "" : "none";
  $("#btnAddHost").style.display = mode === "host" ? "" : "none";
  $("#connList").style.display = mode === "db" ? "" : "none";
  $("#hostList").style.display = mode === "host" ? "" : "none";
  // 视图切换
  $("#dashboard").style.display = mode === "db" ? "" : "none";
  $("#hostDash").style.display = mode === "host" ? "" : "none";
  $("#detail").style.display = "none";
  $("#hostDetail").style.display = "none";
  S.sel = null; S.hostSel = null;
  if (mode === "host") { renderHostList(); renderHostDash(); }
  renderAutoState();
}

/* ---------------- 主机巡检 ---------------- */
function hostState(id) {
  S.hostReports[id] = S.hostReports[id] || null;
  return S.hostReports[id];
}

function renderHostList() {
  const list = $("#hostList");
  if (!S.hosts.length) {
    list.innerHTML = `<div class="empty" style="color:#7488ab;padding:30px 8px;font-size:12px">${LANG === "zh" ? "还没有主机" : "No hosts yet"}<br>${LANG === "zh" ? "点击「＋ 新建主机」开始" : "Click ＋ New Host"}</div>`;
    return;
  }
  list.innerHTML = S.hosts.map((h) => `
    <div class="conn-item ${S.hostSel === h.id ? "active" : ""}" data-id="${esc(h.id)}">
      <span class="dot ${hostLastStatus(h.id)}"></span>
      <span class="c-icon">🖥</span>
      <div style="flex:1;min-width:0">
        <div class="c-name">${esc(h.name)}</div>
        <div class="c-host">${esc(h.user)}@${esc(h.host)}</div>
      </div>
      <button class="item-edit" data-id="${esc(h.id)}" title="${esc(t("detail.edit"))}">✎</button>
    </div>`).join("");
  list.querySelectorAll(".conn-item").forEach((el) => {
    el.onclick = () => showHostDetail(el.dataset.id);
    el.ondblclick = () => openHostDialog(el.dataset.id);
  });
  list.querySelectorAll(".item-edit").forEach((el) => {
    el.onclick = (e) => { e.stopPropagation(); openHostDialog(el.dataset.id); };
  });
}

function hostLastStatus(id) {
  const r = S.hostReports[id];
  if (!r) return "na";
  return r.warnings && r.warnings.length >= 3 ? "warn" : "ok";
}

function renderHostDash() {
  const cards = $("#hostCards");
  const stats = { total: S.hosts.length, ok: 0, inspected: 0 };
  for (const h of S.hosts) if (S.hostReports[h.id]) { stats.inspected++; if (hostLastStatus(h.id) === "ok") stats.ok++; }
  const statRow = $("#hostStatRow");
  if (statRow) statRow.innerHTML = `
    <div class="stat"><b>${stats.total}</b><span>${t("host.statTotal")}</span></div>
    <div class="stat na"><b>${stats.total - stats.inspected}</b><span>${t("host.statPending")}</span></div>
    <div class="stat ok"><b>${stats.ok}</b><span>${t("host.statOk")}</span></div>`;
  if (!S.hosts.length) {
    cards.innerHTML = `<div class="empty" style="grid-column:1/-1"><div class="big">🖥</div>${t("host.empty")}</div>`;
    return;
  }
  cards.innerHTML = S.hosts.map((h) => {
    const r = S.hostReports[h.id];
    const last = r ? new Date(r.collectedAt).toLocaleString() : "—";
    return `<div class="host-card" data-id="${esc(h.id)}">
      <div class="top"><span>🖥</span><span class="h-name">${esc(h.name)}</span><span class="h-type">${esc(h.group || "")}</span></div>
      <div class="host">${esc(h.user)}@${esc(h.host)}:${esc(h.port)}</div>
      <div class="h-meta"><span class="h-last">${t("host.lastInspect")}: ${esc(last)}</span></div>
      <div class="host-actions">
        <button class="btn small h-inspect" data-id="${esc(h.id)}">${t("host.inspect")}</button>
        <button class="btn small h-export" data-id="${esc(h.id)}">${t("detail.export")}</button>
        <button class="btn small h-edit" data-id="${esc(h.id)}">${t("detail.edit")}</button>
      </div>
    </div>`;
  }).join("");
  cards.querySelectorAll(".host-card").forEach((el) => { el.onclick = () => showHostDetail(el.dataset.id); });
  cards.querySelectorAll(".h-inspect").forEach((el) => { el.onclick = (e) => { e.stopPropagation(); inspectHost(el.dataset.id); }; });
  cards.querySelectorAll(".h-export").forEach((el) => { el.onclick = (e) => { e.stopPropagation(); exportHostReport(el.dataset.id); }; });
  cards.querySelectorAll(".h-edit").forEach((el) => { el.onclick = (e) => { e.stopPropagation(); openHostDialog(el.dataset.id); }; });
}

async function showHostDetail(id) {
  S.hostSel = id;
  const h = S.hosts.find((x) => x.id === id); if (!h) return;
  $("#hostDash").style.display = "none";
  $("#hostDetail").style.display = "block";
  $("#hName").textContent = h.name;
  $("#hMeta").textContent = `${h.user}@${h.host}:${h.port} · ${h.authType === "key" ? t("host.authKey") : t("host.authPwd")}`;
  $("#hStatus").textContent = "● " + (hostLastStatus(id) === "ok" ? t("st.ok") : t("st.na"));
  $("#hStatus").className = "status-badge " + hostLastStatus(id);
  // 重置到巡检报告 Tab 并加载历史
  S.hostView = "report"; S.hostHistIdx = -1;
  document.querySelectorAll("#hostTabs .tab").forEach((x) => x.classList.toggle("active", (x.dataset.htab || "report") === "report"));
  renderHostView();
  renderHostList();
  // 主机模式下侧边栏仍在，需隐藏数据库仪表盘
  $("#dashboard").style.display = "none";
  $("#hostDash").style.display = "none";
  try {
    const A = API();
    S.hostHistory = (A && await A.GetHostHistory(id)) || [];
    if (S.hostView === "hist") renderHostView();
  } catch { S.hostHistory = []; }
}

// renderHostView 主机详情按 Tab 渲染：巡检报告 / AI 分析 / 历史。
function renderHostView() {
  const body = $("#hostBody"); if (!body) return;
  const id = S.hostSel;
  const r = S.hostReports[id];
  if (S.hostView === "ai") {
    const ai = S.aiReports[id];
    body.innerHTML = ai
      ? aiSectionHTML(ai, "host")
      : `<div class="empty"><div class="big">🤖</div>${t("ai.noReport")}<br><br>
         ${r ? `<button class="btn primary" onclick="aiAnalyzeHost('${esc(id)}')">${t("ai.analyze")}</button>` : ""}</div>`;
    return;
  }
  if (S.hostView === "hist") {
    if (S.hostHistIdx >= 0 && S.hostHistory[S.hostHistIdx]) {
      const e = S.hostHistory[S.hostHistIdx];
      body.innerHTML = `<div style="margin:4px 0 10px"><button class="btn small" onclick="hostHistBack()">← ${t("host.backToList")}</button>
        <span class="dim" style="font-size:12px;margin-left:10px">${new Date(e.time).toLocaleString()} · ${esc((e.report && e.report.osName) || "")}</span></div>` +
        aiSectionHTML(e.ai && { html: e.ai.html, model: e.ai.model, elapsedMs: 0, time: e.ai.time }, "host") + hostReportHTML(e.report);
      return;
    }
    if (!(S.hostHistory || []).length) {
      body.innerHTML = `<div class="empty"><div class="big">🕘</div>${t("host.histEmpty")}</div>`;
      return;
    }
    body.innerHTML = `<table class="data"><tr><th>${t("th.time")}</th><th>${t("host.secSys")}</th><th>AI</th></tr>` +
      S.hostHistory.map((e, i) => `<tr class="hist-row" data-i="${i}" style="cursor:pointer">
        <td>${new Date(e.time).toLocaleString()}</td>
        <td>${esc((e.report && e.report.osName) || "-")}</td>
        <td>${e.ai ? `<span style="color:var(--brand)">🤖 ${esc(e.ai.model || "")}</span>` : `<span class="dim">—</span>`}</td>
      </tr>`).join("") + `</table>`;
    body.querySelectorAll(".hist-row").forEach((el) => {
      el.onclick = () => { S.hostHistIdx = +el.dataset.i; renderHostView(); };
    });
    return;
  }
  // 默认：巡检报告
  if (!r) {
    body.innerHTML = `<div class="empty"><div class="big">🔍</div>${t("host.notInspected")}<br><br>
      <button class="btn primary" onclick="inspectHost('${esc(id)}')">${t("host.inspect")}</button></div>`;
    return;
  }
  body.innerHTML = hostReportHTML(r);
}

// hostHistBack 从历史详情返回历史列表（全局函数）。
function hostHistBack() { S.hostHistIdx = -1; renderHostView(); }

// hostSetActiveTab 切换主机详情 Tab（全局函数，供分析完成后跳转）。
function hostSetActiveTab(name) {
  const el = document.querySelector(`#hostTabs .tab[data-htab="${name}"]`);
  if (el) el.click();
}

// aiSectionHTML AI 分析报告区块（置于巡检报告最上方；kind 决定导出目标）。
function aiSectionHTML(ai, kind) {
  if (!ai) return "";
  const fn = kind === "db" ? "aiExportDB()" : "aiExportHost()";
  return `<div class="ai-box">
    <div class="ai-head">
      <span class="ai-ico">🤖</span><b>${t("ai.section")}</b>
      <span class="dim ai-meta">${esc(ai.model || "")} · ${Math.round((ai.elapsedMs || 0) / 1000)}s · ${new Date(ai.time || Date.now()).toLocaleString()}</span>
      <button class="btn small" onclick="${fn}">${t("ai.export")}</button>
    </div>
    <div class="ai-body">${ai.html || ""}</div>
  </div>`;
}

// aiProgressOpen/Close AI 分析进度弹窗（带耗时计时）。
let aiTimer = null;
function aiProgressOpen() {
  const d = $("#dlgAIProgress");
  if (!d) return;
  const el = $("#aiElapsed");
  const t0 = Date.now();
  if (el) el.textContent = "0s";
  if (aiTimer) clearInterval(aiTimer);
  aiTimer = setInterval(() => {
    if (el) el.textContent = Math.round((Date.now() - t0) / 1000) + "s";
  }, 1000);
  d.showModal();
}
function aiProgressClose() {
  if (aiTimer) { clearInterval(aiTimer); aiTimer = null; }
  const d = $("#dlgAIProgress");
  if (d && d.open) d.close();
}

// aiAnalyzeHost 将当前巡检报告交给大模型分析。
async function aiAnalyzeHost(id) {
  const A = API(); if (!A || S.aiBusy) return;
  const r = S.hostReports[id];
  if (!r) { toast(t("ai.noReport"), "err"); return; }
  const btn = $("#btnHostAI");
  if (btn) { btn.disabled = true; btn.textContent = "⏳ " + t("ai.analyzing"); }
  S.aiBusy = true;
  aiProgressOpen();
  try {
    const res = await A.AIAnalyzeHost(JSON.stringify(r));
    S.aiReports[id] = res;
    // 历史保留：将 AI 结果挂到对应的巡检历史
    try { await A.AttachHostAI(id, JSON.stringify(r), res.markdown); } catch { }
    S.hostView = "ai"; S.hostHistIdx = -1;
    hostSetActiveTab("ai");
    toast(t("ai.done"), "ok");
  } catch (e) {
    showError(t("err.title"), { meta: `AI · ${id}`, kind: "unknown", msg: String(e) });
  } finally {
    S.aiBusy = false;
    aiProgressClose();
    if (btn) { btn.disabled = false; btn.textContent = t("ai.analyze"); }
  }
}

// ---------- 测速（iperf3） ----------
async function iperfStart() {
  const A = API(); if (!A || S.iperfBusy) return;
  const server = $("#ipServer").value.trim();
  const port = parseInt($("#ipPort").value) || 5201;
  const duration = parseInt($("#ipDuration").value) || 10;
  const parallel = parseInt($("#ipParallel").value) || 1;
  const reverse = $("#ipDir").value === "down";
  const box = $("#ipResult");
  if (!server) { toast(LANG === "zh" ? "请输入测速服务器地址" : "Enter server address", "err"); return; }
  S.iperfBusy = true;
  $("#ipBtnGo").disabled = true;
  $("#ipStatus").textContent = t("spd.running");
  box.innerHTML = `<table class="data" id="ipTbl"><tr><th>${t("spd.sec")}</th><th>${t("spd.rate")}</th></tr></table>`;
  try {
    const res = await A.IPerfStart(server, port, duration, 0, parallel, reverse);
    iperfRenderSummary(server, res);
    await loadIperfHistory();
  } catch (e) {
    box.innerHTML = warnBoxHTML(String(e));
  } finally {
    S.iperfBusy = false;
    $("#ipBtnGo").disabled = false;
    $("#ipStatus").textContent = "";
  }
}

// loadIperfHistory 加载并渲染测速历史（点击回看 / 删除 / 导出）。
async function loadIperfHistory() {
  const A = API(); if (!A) return;
  const box = $("#ipHistory");
  if (!box) return;
  try {
    const items = (await A.GetSpeedHistory()) || [];
    if (!items.length) {
      box.innerHTML = `<div class="dim" style="font-size:11.5px;margin-top:8px">${t("spd.noHistory")}</div>`;
      return;
    }
    box.innerHTML = `<div style="display:flex;align-items:center;gap:8px;margin:10px 0 2px">
        <span class="dim" style="font-size:12px;flex:1">${t("spd.history")}</span>
        <button class="btn small" id="ipHistExport">⇓ ${t("spd.export")}</button>
      </div>` +
      items.map((it, i) => `
        <div class="nh-item" data-i="${i}">
          <span class="dim" style="font-size:11px;flex:none">${new Date(it.time).toLocaleString()}</span>
          <b style="flex:none">${esc(it.server)}:${esc(it.port)}</b>
          <span class="dim" style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${it.direction === "up" ? t("spd.sentLabel") : t("spd.recvLabel")} · ${it.parallel} ${t("spd.parallel")}</span>
          <span style="flex:none;color:var(--ok)">${Number(it.mbps || 0).toFixed(1)} Mbps</span>
          <button class="nh-del" data-t="${it.time}" title="${t("spd.del")}">✕</button>
        </div>`).join("");
    $("#ipHistExport").onclick = async () => {
      try {
        const p = await A.ExportSpeedHistory();
        if (p) toast(t("toast.exportOk") + p, "ok");
      } catch (e) { if (String(e).indexOf("cancel") < 0) toast(t("toast.exportFail") + e, "err"); }
    };
    box.querySelectorAll(".nh-item").forEach((el) => {
      el.onclick = (e) => {
        if (e.target.closest(".nh-del")) return;
        const it = items[+el.dataset.i]; if (!it) return;
        iperfRenderEntry(it);
      };
    });
    box.querySelectorAll(".nh-del").forEach((el) => {
      el.onclick = async (e) => {
        e.stopPropagation();
        try {
          await A.DeleteSpeedEntry(Number(el.dataset.t));
          await loadIperfHistory();
          toast(t("spd.deleted"), "ok");
        } catch (err) { toast(String(err), "err"); }
      };
    });
  } catch { box.innerHTML = ""; }
}

// iperfRenderEntry 回看一条历史测速（汇总 + 每秒区间）。
function iperfRenderEntry(it) {
  const box = $("#ipResult"); if (!box) return;
  const up = it.direction === "up";
  const warn = it.warning ? `<div class="warnbox" style="padding:6px 10px;font-size:11.5px;margin:0 0 8px">${esc(it.warning)}</div>` : "";
  let rows = "";
  for (const iv of it.intervals || []) {
    const tag = iv.omitted ? ` <span class="dim">(${t("spd.omitted")})</span>` : "";
    rows += `<tr><td>${iv.sec}s${tag}</td><td>${Number(iv.mbps).toFixed(1)} Mbps</td></tr>`;
  }
  box.innerHTML =
    `<div class="dim" style="font-size:11.5px;margin-bottom:6px">${esc(it.server)}:${esc(it.port)} · ${new Date(it.time).toLocaleString()}</div>` +
    warn +
    `<div class="host-log-stats">
      <div class="stat ok"><b>${Number(it.mbps || 0).toFixed(1)}</b><span>Mbps ${up ? t("spd.sentLabel") : t("spd.recvLabel")}</span></div>
      <div class="stat"><b>${Number(it.serverMbps || 0).toFixed(1)}</b><span>Mbps ${t("spd.serverSide")}</span></div>
      <div class="stat"><b>${it.retrans >= 0 ? it.retrans : "-"}</b><span>${t("spd.retrans")}</span></div>
    </div>` +
    `<table class="data"><tr><th>${t("spd.sec")}</th><th>${t("spd.rate")}</th></tr>` + rows + `</table>`;
}

// iperfAppendInterval 追加一行实时区间速率。
function iperfAppendInterval(p) {
  const tbl = $("#ipTbl"); if (!tbl) return;
  const omitted = p.omitted ? ` <span class="dim">(${t("spd.omitted")})</span>` : "";
  tbl.insertAdjacentHTML("beforeend",
    `<tr><td>${p.sec}s${omitted}</td><td>${Number(p.mbps).toFixed(1)} Mbps</td></tr>`);
}

// iperfRenderSummary 在区间表上方插入汇总卡片（保留实时区间行）。
function iperfRenderSummary(server, res) {
  const box = $("#ipResult"); if (!box) return;
  const up = res.direction === "up";
  const hdr = `<div class="dim" style="font-size:11.5px;margin-bottom:6px">${esc(server)} · ${up ? t("spd.up") : t("spd.down")} · ${Math.round((res.seconds || 0) * 10) / 10}s</div>`;
  const warn = res.warning ? `<div class="warnbox" style="padding:6px 10px;font-size:11.5px;margin:0 0 8px">${esc(res.warning)}</div>` : "";
  const stats = `<div class="host-log-stats">
    <div class="stat ok"><b>${Number(res.mbps || 0).toFixed(1)}</b><span>Mbps ${up ? t("spd.sentLabel") : t("spd.recvLabel")}</span></div>
    <div class="stat"><b>${Number(res.serverMbps || 0).toFixed(1)}</b><span>Mbps ${t("spd.serverSide")}</span></div>
    <div class="stat"><b>${res.retrans >= 0 ? res.retrans : "-"}</b><span>${t("spd.retrans")}</span></div>
  </div>`;
  const wrap = document.createElement("div");
  wrap.innerHTML = warn + hdr + stats;
  box.insertBefore(wrap, box.firstChild);
}

// aiExportHost 导出主机 AI 分析 HTML 报告（全局函数，供区块内 onclick 调用）。
async function aiExportHost() {
  const A = API(); if (!A) return;
  const id = S.hostSel;
  const r = S.hostReports[id], ai = S.aiReports[id];
  if (!r || !ai) return;
  try {
    const p = await A.AIExportHostAnalysis(JSON.stringify(r), ai.markdown);
    if (p) toast(t("toast.exportOk") + p, "ok");
  } catch (e) { if (String(e).indexOf("cancel") < 0) toast(t("toast.exportFail") + e, "err"); }
}

// aiAnalyzeDB 将当前数据库连接的巡检概览交给大模型分析。
async function aiAnalyzeDB() {
  const A = API(); if (!A || S.aiBusy || !S.sel) return;
  const btn = $("#btnAI");
  if (btn) { btn.disabled = true; btn.textContent = "⏳ " + t("ai.analyzing"); }
  S.aiBusy = true;
  aiProgressOpen();
  try {
    const ov = await A.GetOverview(S.sel);
    const ovJSON = JSON.stringify(ov);
    const res = await A.AIAnalyzeOverview(S.sel, ovJSON);
    S.dbAIReports[S.sel] = res;
    // 历史保留（含概览快照）
    try { await A.AddDBAIAnalysis(S.sel, ovJSON, res.markdown); } catch { }
    await loadTab("ai", false);
    const tabEl = document.querySelector('#tabs .tab[data-tab="ai"]');
    if (tabEl && !tabEl.classList.contains("active")) tabEl.click();
    toast(t("ai.done"), "ok");
  } catch (e) {
    showError(t("err.title"), { meta: `AI · ${connName(S.sel)}`, kind: "unknown", msg: String(e) });
  } finally {
    S.aiBusy = false;
    aiProgressClose();
    if (btn) { btn.disabled = false; btn.textContent = t("ai.analyze"); }
  }
}

// aiExportDB 导出数据库 AI 分析 HTML 报告（全局函数，供区块内 onclick 调用）。
async function aiExportDB() {
  const A = API(); if (!A) return;
  const id = S.sel;
  const ai = S.dbAIReports[id];
  const cache = cacheFor(id);
  if (!ai || !cache.overview) return;
  try {
    const p = await A.AIExportOverviewAnalysis(id, JSON.stringify(cache.overview), ai.markdown);
    if (p) toast(t("toast.exportOk") + p, "ok");
  } catch (e) { if (String(e).indexOf("cancel") < 0) toast(t("toast.exportFail") + e, "err"); }
}

async function inspectHost(id) {
  const A = API(); if (!A || S.hostBusy) return;
  const h = S.hosts.find((x) => x.id === id); if (!h) return;
  S.hostBusy = true;
  // 确保进入主机详情视图
  if (S.hostSel !== id) showHostDetail(id);
  const prog = $("#hostProg");
  if (prog) prog.innerHTML = `<span class="host-prog"><span class="spin"></span>${t("host.inspecting")}…</span>` + progressBarHTML(0, 0);
  $("#hStatus").textContent = "● " + t("host.inspecting");
  $("#hStatus").className = "status-badge warn";
  try {
    const r = await A.InspectHost(id);
    S.hostReports[id] = r;
    if (prog) prog.textContent = `${t("host.doneAt")}: ${new Date(r.collectedAt).toLocaleTimeString()}`;
    // 后端已自动存入历史；切回巡检报告 Tab 展示
    S.hostView = "report"; S.hostHistIdx = -1;
    hostSetActiveTab("report");
    try { S.hostHistory = (await A.GetHostHistory(id)) || []; } catch { }
    renderHostView();
    toast(t("host.inspectDone"), "ok");
  } catch (e) {
    showError(t("err.title"), { meta: `${h.name} · ${h.host}`, kind: "unknown", msg: String(e) });
    if (prog) prog.textContent = "";
  } finally {
    S.hostBusy = false;
    renderHostList(); renderHostDash();
    // 状态徽标同步为最终状态（修复：完成后仍显示"正在巡检"）
    const hb = $("#hStatus");
    if (hb) {
      const st = hostLastStatus(id);
      hb.textContent = "● " + (st === "ok" ? t("st.ok") : st === "warn" ? t("host.badgeWarn") : t("st.na"));
      hb.className = "status-badge " + st;
    }
  }
}

// progressBarHTML 百分比进度条（done/total）。
function progressBarHTML(done, total) {
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  return `<div class="pbar-row"><div class="pbar"><div class="pbar-fill" style="width:${pct}%"></div></div><span class="dim" style="font-size:11.5px;flex:none">${pct}% (${done}/${total})</span></div>`;
}

async function exportHostReport(id) {
  const A = API(); if (!A) return;
  try {
    // 综合导出：当前巡检报告 + 对应 AI 分析（同一份 HTML，目录跳转）
    const r = S.hostReports[id];
    const ai = S.aiReports[id];
    const p = await A.ExportHostReport(id, r ? JSON.stringify(r) : "", ai ? ai.markdown : "");
    if (p) toast(t("toast.exportOk") + p, "ok");
  } catch (e) { if (String(e).indexOf("cancel") < 0) toast(t("toast.exportFail") + e, "err"); }
}

function osIcon(osName) {
  const s = String(osName || "").toLowerCase();
  if (s.includes("debian")) return "🌀";
  if (s.includes("ubuntu")) return "🟠";
  if (s.includes("centos") || s.includes("rocky") || s.includes("alma")) return "⛑";
  if (s.includes("red hat") || s.includes("rhel")) return "🎩";
  if (s.includes("fedora")) return "🎩";
  if (s.includes("windows")) return "🪟";
  if (s.includes("proxmox") || s.includes("pve")) return "🖥";
  if (s.includes("opensuse") || s.includes("suse")) return "🦎";
  if (s.includes("arch")) return "🏹";
  if (s.includes("kali")) return "🐉";
  if (s.includes("linux") || s.includes("unix") || s.includes("aix") || s.includes("hp-ux")) return "🐧";
  return "🖥";
}

function hostReportHTML(r) {
  let html = "";
  // OS 图标 + 系统信息（左右布局）
  const icon = osIcon(r.osName);
  html += `<h2 style="color:var(--brand);font-size:15px;margin:14px 0 8px">${icon} ${t("host.secSys")}</h2>`;
  html += `<table class="data host-lr">`;
  for (const kv of r.sysInfo || []) {
    if (kv.key === "__df" || kv.key === "__ip") continue;
    html += `<tr><th>${esc(kv.key)}</th><td>${esc(kv.value)}</td></tr>`;
  }
  html += `</table>`;
  // PCI / SCSI 设备（放在磁盘挂载之前，PCI 在前）
  if (r.lspci && r.lspci !== "") {
    html += `<h2 style="color:var(--brand);font-size:14px;margin:14px 0 6px">${t("host.lspci")}</h2><pre class="sql-input" style="min-height:60px;max-height:200px;overflow:auto">${esc(r.lspci)}</pre>`;
  } else if (r.lspciNote) html += `<div class="dim" style="font-size:12px;margin:6px 0">${esc(r.lspciNote)}</div>`;
  if (r.lsscsi && r.lsscsi !== "") {
    html += `<h2 style="color:var(--brand);font-size:14px;margin:14px 0 6px">${t("host.lsscsi")}</h2><pre class="sql-input" style="min-height:60px;max-height:200px;overflow:auto">${esc(r.lsscsi)}</pre>`;
  } else if (r.lsscsiNote) html += `<div class="dim" style="font-size:12px;margin:6px 0">${esc(r.lsscsiNote)}</div>`;
  // 磁盘挂载
  for (const kv of r.sysInfo || []) {
    if (kv.key === "__df") html += `<h2 style="color:var(--brand);font-size:14px;margin:14px 0 6px">${t("host.secDisk")}</h2>` + plainTableHTML(kv.value, ["文件系统", "类型", "容量", "已用", "可用", "已用%", "挂载点"]);
  }
  // 网络接口（表格，状态前置图标）
  for (const kv of r.sysInfo || []) {
    if (kv.key === "__ip") {
      html += `<h2 style="color:var(--brand);font-size:14px;margin:14px 0 6px">${t("host.secNet")}</h2>`;
      html += `<table class="data"><tr><th>接口</th><th>状态</th><th>地址</th></tr>`;
      for (const rawLine of kv.value.split("\n")) {
        const line = rawLine.trim();
        if (!line) continue;
        const fs = line.split(/\s+/);
        html += `<tr><td>${esc(fs[0] || "")}</td><td>${netStateIcon(fs[1] || "")} ${esc(fs[1] || "")}</td><td>${esc(fs.slice(2).join(" "))}</td></tr>`;
      }
      html += `</table>`;
    }
  }
  // 端口监听（全部监听端口 + 进程 + 连接数 + 状态）
  if ((r.listenPorts || []).length) {
    html += `<h2 style="color:var(--brand);font-size:14px;margin:14px 0 6px">${t("host.secPorts")}</h2>`;
    html += `<table class="data"><tr><th>${t("host.port")}</th><th>${t("host.service")}</th><th>${t("host.process")}</th><th>${t("host.state")}</th><th>${t("host.connCount")}</th></tr>`;
    for (const p of r.listenPorts) {
      html += `<tr><td><b>${esc(p.port)}</b></td><td>${esc(p.service)}</td><td>${esc(p.process)}</td><td>${esc(p.state)}</td><td class="dim" style="font-size:11.5px">${esc(p.conns || "-")}</td></tr>`;
    }
    html += `</table>`;
  }
  // RAID
  if (r.storcli && (r.storcli.controllers || []).length) {
    html += `<h2 style="color:var(--brand);font-size:14px;margin:14px 0 6px">${t("host.secRaid")}</h2>`;
    for (const c of r.storcli.controllers) {
      html += `<div class="kv-item" style="margin-bottom:4px"><div class="k">${t("host.controller")} ${esc(c.index)} — ${esc(c.model)}</div><div class="v">${t("host.state")}: ${esc(c.status || "-")}</div></div>`;
      const ctlBits = [];
      if (c.firmware) ctlBits.push(`${t("host.raidFw")}: ${esc(c.firmware)}`);
      if (c.temp) ctlBits.push(`${t("host.raidTemp")}: ${esc(c.temp)}`);
      if (c.memory) ctlBits.push(`${t("host.raidMem")}: ${esc(c.memory)}`);
      if (c.battery) ctlBits.push(`${t("host.raidBattery")}: ${esc(c.battery)}`);
      if (ctlBits.length) html += `<div class="dim" style="font-size:11.5px;margin:0 0 6px">${ctlBits.join(" · ")}</div>`;
      if ((c.vds || []).length) html += simpleTableHTML(["VD", "名称", "级别", "状态", "容量", "缓存"], c.vds.map((v) => [v.id, v.name, v.type, v.state, v.size, v.cache || "-"]));
      if ((c.pds || []).length) html += simpleTableHTML(["槽位", "型号", "状态", "容量", "SN", "通电时长", "详情"], c.pds.map((p) => [p.slot, p.model, p.state, p.size, p.sn || "-", p.power || "-", p.detail]));
    }
  } else html += `<div class="empty" style="padding:8px;font-size:12px">${esc(r.storcliNote || t("host.noRaid"))}</div>`;
  // ZFS
  if (r.zfs && (r.zfs.pools || []).length) {
    html += `<h2 style="color:var(--brand);font-size:14px;margin:14px 0 6px">${t("host.secZfs")}</h2>`;
    html += simpleTableHTML(r.zfs.poolCols || ["名称", "大小", "已用", "剩余", "碎片率", "健康状态"], r.zfs.pools);
    if (r.zfs.status) html += `<pre class="sql-input" style="min-height:80px;max-height:200px;overflow:auto">${esc(r.zfs.status)}</pre>`;
    if ((r.zfs.datasets || []).length) html += simpleTableHTML(r.zfs.dsCols || ["数据集", "已用", "可用", "挂载点"], r.zfs.datasets);
  } else html += `<div class="empty" style="padding:8px;font-size:12px">${esc(r.zfsNote || t("host.noZfs"))}</div>`;
  // 安全状态（左右布局）——放在 Top 进程之前
  html += `<h2 style="color:var(--brand);font-size:15px;margin:16px 0 8px">${t("host.secSecurity")}</h2>`;
  html += `<table class="data host-lr">`;
  for (const kv of r.security || []) {
    if (kv.key.startsWith("__")) {
      html += `</table><div class="pre-block">${esc(kv.value)}</div><table class="data host-lr">`;
      continue;
    }
    html += `<tr><th>${esc(kv.key)}</th><td>${esc(kv.value)}</td></tr>`;
  }
  html += `</table>`;
  // Top CPU / Top Mem（含启动时间与运行时长）
  html += `<h2 style="color:var(--brand);font-size:15px;margin:16px 0 8px">${t("host.secTopCpu")}</h2>` + procTableHTML(r.topCpu || []);
  html += `<h2 style="color:var(--brand);font-size:15px;margin:16px 0 8px">${t("host.secTopMem")}</h2>` + procTableHTML(r.topMem || []);
  // rs_/rsa_（格式化：文件分类计数 + 连接聚合表格）
  html += `<h2 style="color:var(--brand);font-size:15px;margin:16px 0 8px">${t("host.secRS")}</h2>`;
  if (!(r.rsProcs || []).length) html += `<div class="empty">${t("host.noRS")}</div>`;
  for (const p of r.rsProcs || []) {
    html += `<div class="kv-item" style="margin-bottom:8px">
      <div class="k">${esc(p.cmd)} <span class="dim">(PID ${esc(p.pid)} · ${esc(p.user)})</span></div>
      <div class="v">CPU ${esc(p.cpu)}% · MEM ${esc(p.mem)}% · 内存 ${esc(kbToMB(p.rss))} · 运行 ${esc(p.etime)} · FD ${esc(p.fdCount)}</div>
      ${fmtOpenFiles(p.openFiles)}
      ${fmtConns(p.conns)}
    </div>`;
  }
  // 警告
  if ((r.warnings || []).length) html += `<div class="warnbox"><b>${t("ov.tips")}</b><br>` + r.warnings.map(esc).join("<br>") + `</div>`;
  // 系统日志（最后）
  if (r.sysLog) {
    html += `<h2 style="color:var(--brand);font-size:15px;margin:16px 0 8px">${t("host.secLog")}</h2>`;
    html += `<div class="host-log-stats">
      <div class="stat err"><b>${esc(r.sysLog.errCount)}</b><span>${t("host.logErrors")} (24h)</span></div>
      <div class="stat warn"><b>${esc(r.sysLog.warnCount)}</b><span>${t("host.logWarns")} (24h)</span></div>
      <div class="stat"><span class="dim" style="font-size:11px">${t("host.logSource")}: ${esc(r.sysLog.source)}</span></div>
    </div>`;
    if ((r.sysLog.topErrors || []).length) {
      html += `<div class="dim" style="font-size:12px;margin:8px 0 4px">${t("host.recentErrors")}</div><div class="pre-block" style="max-height:200px">`;
      html += r.sysLog.topErrors.map((l) => esc(l)).join("\n");
      html += `</div>`;
    }
    if ((r.sysLog.dmesgErrs || []).length) {
      html += `<div class="dim" style="font-size:12px;margin:8px 0 4px">${t("host.dmesg")}</div><div class="pre-block" style="max-height:120px">`;
      html += r.sysLog.dmesgErrs.map((l) => esc(l)).join("\n");
      html += `</div>`;
    }
  }
  // 重启记录（最近 10 次，最后）
  html += `<h2 style="color:var(--brand);font-size:15px;margin:16px 0 8px">${t("host.secReboot")}</h2>`;
  if (!(r.reboots || []).length) {
    html += `<div class="dim" style="font-size:12px;margin:6px 0">${esc(r.rebootsNote || t("host.rbNone"))}</div>`;
  } else {
    html += `<table class="data"><tr><th>#</th><th>${t("host.rbStart")}</th><th>${t("host.rbEnd")}</th><th>${t("host.rbDur")}</th></tr>`;
    r.reboots.forEach((rb, i) => {
      const dur = /still/i.test(rb.dur || "") ? t("host.rbRunning") : esc(rb.dur || "-");
      html += `<tr><td class="dim">${i + 1}</td><td>${esc(rb.start || "-")}</td><td>${rb.end === "-" ? `<span class="dim">—</span>` : esc(rb.end || "-")}</td><td>${dur}</td></tr>`;
    });
    html += `</table>`;
  }
  return html;
}

// netStateIcon 网络接口状态前置图标。
function netStateIcon(state) {
  const s = String(state || "").toUpperCase();
  if (s === "UP") return "🟢";
  if (s === "DOWN") return "🔴";
  return "⚪";
}

function procTableHTML(list) {
  if (!list.length) return `<div class="empty">${t("sql.empty")}</div>`;
  // rs_/rsa_ 前缀进程标蓝以示区别
  const isRS = (cmd) => {
    const base = (cmd || "").split(/\s+/)[0].split(/[\\/]/).pop();
    return base.startsWith("rs_") || base.startsWith("rsa_");
  };
  return `<table class="data"><tr><th>PID</th><th>用户</th><th>CPU%</th><th>MEM%</th><th>内存(MB)</th><th>启动时间</th><th>运行时长</th><th>命令</th></tr>` +
    list.map((p) => {
      const cmdShort = p.cmd && p.cmd.length > 60 ? p.cmd.slice(0, 60) + "…" : (p.cmd || "");
      const cmdHtml = isRS(p.cmd)
        ? `<span style="color:var(--brand);font-weight:600" title="${esc(p.cmd || "")}">${esc(cmdShort)}</span>`
        : `<span title="${esc(p.cmd || "")}">${esc(cmdShort)}</span>`;
      return `<tr><td>${esc(p.pid)}</td><td>${esc(p.user)}</td><td>${esc(p.cpu)}</td><td>${esc(p.mem)}</td><td>${esc(kbToMB(p.rss))}</td><td>${esc(p.start || "-")}</td><td>${esc(p.etime || "-")}</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${cmdHtml}</td></tr>`;
    }).join("") + `</table>`;
}

// kbToMB 进程 RSS（KB）转 MB 显示。
function kbToMB(rss) {
  const n = parseFloat(rss);
  if (isNaN(n) || n <= 0) return "-";
  return (n / 1024).toFixed(1) + " MB";
}

function simpleTableHTML(cols, rows) {
  return `<table class="data"><tr>${cols.map((c) => `<th>${esc(c)}</th>`).join("")}</tr>` +
    rows.map((r) => `<tr>${r.map((v) => `<td>${esc(v)}</td>`).join("")}</tr>`).join("") + `</table>`;
}

// fmtOpenFiles rs_/rsa_ 打开文件格式化：普通文件列出，socket/pipe/匿名计数。
function fmtOpenFiles(files) {
  if (!files || !files.length) return "";
  const reg = [], sock = [], pipe = [], other = [];
  for (const f of files) {
    if (/^socket:\[/.test(f)) sock.push(f);
    else if (/^pipe:\[/.test(f)) pipe.push(f);
    else if (/^anon_inode:|eventpoll|^\/dev\/null$|^\/dev\/urandom$|^\/dev\/random$/.test(f)) other.push(f);
    else reg.push(f);
  }
  let head = `${t("host.openFiles")} ${files.length}${t("host.filesUnit")}`;
  if (sock.length) head += ` · socket ×${sock.length}`;
  if (pipe.length) head += ` · pipe ×${pipe.length}`;
  if (other.length) head += ` · ${t("host.filesOther")} ×${other.length}`;
  let html = `<div class="dim" style="font-size:11.5px;margin-top:6px">${esc(head)}</div>`;
  if (reg.length) {
    html += `<div class="dim" style="font-size:11.5px;margin-top:2px;user-select:text">${t("host.filesReg")}: ${esc(reg.slice(0, 15).join(" · "))}${reg.length > 15 ? " …" : ""}</div>`;
  }
  return html;
}

// fmtConns rs_/rsa_ 网络连接格式化：解析 ss 行并按方向聚合。
// 入站（本地端口在 LISTEN 集合中）按 状态+本地 聚合，出站按 状态+远端 聚合。
function fmtConns(conns) {
  // 拆分地址为 [主机, 端口]（兼容 [IPv6]:port / IPv4:port / *:*）
  const splitAddr = (a) => {
    if (a.startsWith("[")) {
      const j = a.indexOf("]");
      if (j > 0) return [a.slice(0, j + 1), a.slice(j + 2)];
    }
    const i = a.lastIndexOf(":");
    return i > 0 ? [a.slice(0, i), a.slice(i + 1)] : [a, ""];
  };
  const rows = [];
  for (const c of conns || []) {
    const m = c.match(/^(\S+)\s+\d+\s+\d+\s+(\S+)\s+(\S+)/);
    if (!m) continue;
    rows.push({ st: m[1], local: m[2], remote: m[3] });
  }
  if (!rows.length) return "";
  const listenPorts = new Set(rows.filter((r) => r.st === "LISTEN").map((r) => splitAddr(r.local)[1]));
  const agg = new Map();
  for (const r of rows) {
    const [lh, lp] = splitAddr(r.local);
    const [rh, rp] = splitAddr(r.remote);
    let key, e;
    if (r.st === "LISTEN") {
      key = "L|" + r.local;
      e = agg.get(key) || { st: r.st, local: r.local, remote: rh, n: 0, remotes: new Set(), locals: new Set() };
    } else if (listenPorts.has(lp)) {
      // 入站数据连接：本地端口固定，远端端口为临时端口
      key = "I|" + r.st + "|" + r.local + "|" + rh;
      e = agg.get(key) || { st: r.st, local: r.local, remote: rh, n: 0, remotes: new Set([rp]), locals: new Set() };
    } else {
      // 出站连接：远端固定，本地端口为临时端口
      key = "O|" + r.st + "|" + rh + "|" + rp;
      e = agg.get(key) || { st: r.st, local: lh, remote: rh + ":" + rp, n: 0, remotes: new Set(), locals: new Set() };
    }
    e.n++;
    e.locals.add(r.local);
    e.remotes.add(r.remote);
    agg.set(key, e);
  }
  let html = `<table class="data" style="margin-top:4px"><tr><th>${t("host.state")}</th><th>${t("host.localAddr")}</th><th>${t("host.remoteHost")}</th><th>${t("host.connCount")}</th></tr>`;
  for (const e of agg.values()) {
    const local = e.locals.size > 1 ? [...e.locals].map((l) => splitAddr(l)[0]).filter((v, i, a) => a.indexOf(v) === i).join(" / ") : [...e.locals][0] || "-";
    html += `<tr><td>${esc(e.st)}</td><td>${esc(local)}</td><td>${esc(e.remote)}</td><td>${e.n}</td></tr>`;
  }
  html += `</table>`;
  return html;
}

function plainTableHTML(text, headers) {
  const lines = text.split("\n").filter((l) => l.trim());
  return simpleTableHTML(headers, lines.slice(1).map((l) => l.split(/\s+/)));
}

/* ---------------- 主机对话框 ---------------- */
function openHostDialog(id) {
  window._editHostId = id || null;
  if (id) {
    const h = S.hosts.find((x) => x.id === id);
    $("#dlgHostTitle").textContent = t("host.editTitle");
    $("#hFName").value = h.name; $("#hFGroup").value = h.group || "";
    $("#hFHost").value = h.host; $("#hFPort").value = h.port;
    $("#hFUser").value = h.user; $("#hFAuth").value = h.authType || "password";
    $("#hFPassword").value = ""; $("#hFKeyPath").value = h.keyPath || "";
    $("#hFKeyPass").value = ""; $("#hFTimeout").value = h.timeout || 15;
  } else {
    $("#dlgHostTitle").textContent = t("host.newTitle");
    $("#hFName").value = ""; $("#hFGroup").value = ""; $("#hFHost").value = "";
    $("#hFPort").value = 22; $("#hFUser").value = "root"; $("#hFAuth").value = "password";
    $("#hFPassword").value = ""; $("#hFKeyPath").value = ""; $("#hFKeyPass").value = ""; $("#hFTimeout").value = 15;
  }
  applyHostAuthVisibility();
  $("#dlgHost").showModal();
}

function applyHostAuthVisibility() {
  const isKey = $("#hFAuth").value === "key";
  $("#hLbPwd").style.display = isKey ? "none" : "";
  $("#hLbKeyPath").style.display = isKey ? "" : "none";
  $("#hLbKeyPass").style.display = isKey ? "" : "none";
}

function collectHostForm() {
  const h = { id: window._editHostId || "" };
  h.name = $("#hFName").value.trim() || $("#hFHost").value.trim();
  h.group = $("#hFGroup").value.trim();
  h.host = $("#hFHost").value.trim();
  h.port = parseInt($("#hFPort").value) || 22;
  h.user = $("#hFUser").value.trim();
  h.authType = $("#hFAuth").value;
  h.password = $("#hFPassword").value;
  h.keyPath = $("#hFKeyPath").value.trim();
  h.keyPassphrase = $("#hFKeyPass").value;
  h.timeout = parseInt($("#hFTimeout").value) || 15;
  return h;
}

async function saveHost() {
  const A = API(); if (!A) return;
  const editId = window._editHostId;
  const h = collectHostForm();
  try {
    await A.SaveSSHHost(h);
    window._editHostId = null;
    $("#dlgHost").close();
    toast(t("toast.saved"), "ok");
    await reload();
    switchMode("host");
    if (editId && S.hostSel === editId) showHostDetail(editId);
  } catch (e) { toast(t("toast.saveFail") + e, "err"); }
}

async function testSSHConn() {
  const A = API(); if (!A) return;
  const h = collectHostForm();
  const el = $("#hostTestResult");
  if (el) { el.className = ""; el.textContent = LANG === "zh" ? "测试中…" : "Testing…"; }
  try {
    const r = await A.TestSSHHost(h);
    if (r && r.ok) {
      // 成功结果直接显示在对话框内（toast 会被对话框背景虚化）
      const msg = `✓ ${LANG === "zh" ? "SSH 连接成功" : "SSH connected"}　${r.detail || ""}`;
      if (el) { el.className = "ok"; el.textContent = msg; }
    } else {
      const detail = r ? (r.detail || "failed") : "unknown";
      if (el) { el.className = "err"; el.textContent = "✗ " + detail; }
      showError(t("err.title"), { meta: `${h.name || "-"} · ${h.host}:${h.port}`, kind: "network", msg: detail });
    }
  } catch (e) {
    if (el) { el.className = "err"; el.textContent = "✗ " + String(e); }
    showError(t("err.title"), { meta: `${h.name || "-"} · ${h.host}:${h.port}`, kind: "unknown", msg: String(e) });
  }
}

async function deleteHost(id) {
  const A = API(); if (!A) return;
  if (S.hostDeleteArmed !== id) {
    S.hostDeleteArmed = id;
    $("#btnHostDelete").textContent = t("detail.delConfirm");
    setTimeout(() => { S.hostDeleteArmed = null; const b = $("#btnHostDelete"); if (b) b.textContent = t("detail.del"); }, 2500);
    return;
  }
  S.hostDeleteArmed = null;
  await A.DeleteSSHHost(id);
  delete S.hostReports[id];
  delete S.aiReports[id];
  $("#btnHostDelete").textContent = t("detail.del");
  toast(t("toast.deleted"), "ok");
  S.hostSel = null;
  switchMode("host");
  await reload();
}

/* ---------------- 渲染：侧边栏 ---------------- */
function renderSidebar() {
  const list = $("#connList");
  const groups = {};
  for (const c of S.conns) {
    const g = c.group || (LANG === "zh" ? "未分组" : "Ungrouped");
    (groups[g] = groups[g] || []).push(c);
  }
  let html = "";
  for (const g of Object.keys(groups).sort()) {
    html += `<div class="conn-group">${esc(g.toUpperCase())} (${groups[g].length})</div>`;
    for (const c of groups[g]) {
      const ti = typeOf(c.type);
      html += `<div class="conn-item ${S.sel === c.id ? "active" : ""}" data-id="${esc(c.id)}">
        <span class="dot ${statusOf(c.id)}"></span>
        <span class="c-icon">${esc(ti.icon)}</span>
        <div style="flex:1;min-width:0">
          <div class="c-name">${esc(c.name)}</div>
          <div class="c-host">${esc(c.host)}:${esc(c.port)}</div>
        </div>
        <button class="item-edit" data-id="${esc(c.id)}" title="${esc(t("detail.edit"))}">✎</button></div>`;
    }
  }
  if (!S.conns.length) html = `<div class="empty" style="color:var(--side-dim);padding:30px 8px;font-size:12px">${t("empty.none")}<br>${t("empty.hint")}</div>`;
  list.innerHTML = html;
  list.querySelectorAll(".conn-item").forEach((el) => {
    el.onclick = () => showDetail(el.dataset.id); // 单击：显示缓存（无缓存才采集）
    el.ondblclick = async () => {                 // 双击：重新连接（清缓存重采）
      await showDetail(el.dataset.id, { refresh: true });
      toast(t("toast.refreshed"), "ok");
    };
  });
  list.querySelectorAll(".item-edit").forEach((el) => {
    el.onclick = (e) => { e.stopPropagation(); openConnDialog(el.dataset.id); };
  });
}

/* ---------------- 渲染：仪表盘 ---------------- */
function filteredConns() {
  const q = S.search.trim().toLowerCase();
  return S.conns.filter((c) => {
    if (S.fType && c.type !== S.fType) return false;
    if (S.fGroup && (c.group || "ungrouped") !== S.fGroup) return false;
    if (q && !(c.name + c.host + (c.database || "")).toLowerCase().includes(q)) return false;
    return true;
  });
}

function renderStats() {
  const total = S.conns.length;
  let ok = 0, err = 0, na = 0;
  for (const c of S.conns) {
    const st = statusOf(c.id);
    if (st === "ok") ok++; else if (st === "na") na++; else err++;
  }
  $("#statRow").innerHTML = `
    <div class="stat"><b>${total}</b><span>${t("st.total")}</span></div>
    <div class="stat ok"><b>${ok}</b><span>${t("st.ok")}</span></div>
    <div class="stat err"><b>${err}</b><span>${t("st.err")}</span></div>
    <div class="stat na"><b>${na}</b><span>${t("st.na")}</span></div>`;
}

function renderCards() {
  const cards = $("#cards");
  const list = filteredConns();
  if (!list.length) {
    cards.innerHTML = `<div class="empty" style="grid-column:1/-1"><div class="big">🗃</div>
      ${S.conns.length ? t("empty.nomatch") : t("empty.noc")}</div>`;
    return;
  }
  cards.innerHTML = list.map((c) => {
    const ti = typeOf(c.type);
    const st = statusOf(c.id);
    const r = S.latest[c.id];
    const badge = { ok: t("st.ok"), warn: t("st.warn"), err: t("st.err"), na: t("st.na") }[st];
    const errLine = r && !r.ok ? `<div class="lat" style="color:var(--err);margin-top:8px;font-size:11px">${esc(r.error || "").slice(0, 90)}</div>` : "";
    return `<div class="card" data-id="${esc(c.id)}">
      <div class="top"><span class="big-icon">${esc(ti.icon)}</span>
        <span class="c-name">${esc(c.name)}</span><span class="c-type">${esc(ti.name)}</span></div>
      <div class="host">${esc(c.host)}:${esc(c.port)}${c.database ? " / " + esc(c.database) : ""}${c.group ? " · " + esc(c.group) : ""}</div>
      <div class="meta">
        <span class="status-badge ${st}">${badge}</span>
        <span class="lat">${esc(fmtLat(r))}</span>
        <button class="card-edit" data-id="${esc(c.id)}" title="${esc(t("detail.edit"))}">✎ 编辑</button>
      </div>${errLine}</div>`;
  }).join("");
  cards.querySelectorAll(".card").forEach((el) => {
    el.onclick = () => showDetail(el.dataset.id);
    el.ondblclick = () => openConnDialog(el.dataset.id);
  });
  cards.querySelectorAll(".card-edit").forEach((el) => {
    el.onclick = (e) => { e.stopPropagation(); openConnDialog(el.dataset.id); };
  });
}

function renderFilters() {
  const ts = [...new Set(S.conns.map((c) => c.type))];
  $("#filterType").innerHTML = `<option value="">${t("dash.allTypes")}</option>` +
    ts.map((ty) => `<option value="${esc(ty)}" ${S.fType === ty ? "selected" : ""}>${esc(typeOf(ty).name)}</option>`).join("");
  const gs = [...new Set(S.conns.map((c) => c.group || (LANG === "zh" ? "未分组" : "Ungrouped")))];
  $("#filterGroup").innerHTML = `<option value="">${t("dash.allGroups")}</option>` +
    gs.map((g) => `<option value="${esc(g)}" ${S.fGroup === g ? "selected" : ""}>${esc(g)}</option>`).join("");
}

function renderAll() {
  // 各渲染环节独立防崩：单块失败不影响整体连接列表展示
  const safe = (name, fn) => { try { fn(); } catch (e) { console.error(name, e); toast(name + ": " + e, "err"); } };
  safe("renderSidebar", renderSidebar);
  safe("renderStats", renderStats);
  safe("renderFilters", renderFilters);
  safe("renderCards", renderCards);
  safe("renderAutoState", renderAutoState);
  safe("updateAdvTabs", updateAdvTabs);
  if (S.mode === "host") safe("renderHostDash", renderHostDash);
}

// 高级模式页签显隐（左下角 ⌨ 开关控制）
function updateAdvTabs() {
  const on = S.settings && S.settings.advancedMode;
  document.querySelectorAll(".adv-only").forEach((el) => { el.style.display = on ? "" : "none"; });
  const btn = $("#btnAdvanced");
  if (btn) btn.classList.toggle("on", !!on);
  if (!on && S.tab === "sql") loadTab("overview");
}

/* ---------------- 数据表元数据分页（按库/模式分组 + 表查询） ---------------- */
function tblState() {
  S.tblByConn = S.tblByConn || {};
  S.tblByConn[S.sel] = S.tblByConn[S.sel] || { schema: "", page: 1, size: 50, schemas: null, qry: null };
  return S.tblByConn[S.sel];
}

// 按方言生成带引号的表标识符
function quoteIdent(family, schema, table) {
  switch (family) {
    case "mysql": return "`" + schema + "`.`" + table + "`";
    case "mssql": return "[" + schema + "].[" + table + "]";
    case "clickhouse": return schema + "." + table;
    case "sqlite": return '"' + table + '"';
    case "sybase": return schema ? schema + ".." + table : table;
    case "db2": return '"' + schema.toUpperCase() + '"."' + table.toUpperCase() + '"';
    default: return '"' + schema + '"."' + table + '"';
  }
}

/* ---------------- 通用错误块（可选中 + 一键复制） ---------------- */
function warnBoxHTML(msg) {
  return `<div class="warnbox selectable"><div class="wb-row">
    <div class="wb-msg">${esc(msg)}</div>
    <button class="btn small warn-copy" type="button">${LANG === "zh" ? "复制" : "Copy"}</button>
  </div></div>`;
}
// 全局委托：点击 .warn-copy 复制所在错误块文本
document.addEventListener("click", (e) => {
  const btn = e.target.closest(".warn-copy");
  if (!btn) return;
  const box = btn.closest(".warnbox");
  if (box) {
    copyText(box.querySelector(".wb-msg")?.textContent || box.textContent)
      .then(() => toast(t("err.copied"), "ok"))
      .catch(() => { });
  }
});

async function renderTablesPage(force) {
  const A = API(); if (!A || !S.sel) return;
  const st = tblState();
  // 首次载入模式/数据库列表
  if (!st.schemas) {
    try { st.schemas = (await A.ListSchemas(S.sel)) || []; } catch (e) { st.schemas = []; }
    if (!st.schemas.includes(st.schema)) st.schema = st.schemas[0] || "";
  }
  if (force) st.page = 1;
  let tp;
  try {
    tp = await A.ListTables(S.sel, st.schema, st.filter || "", st.page, st.size);
  } catch (e) {
    $("#tabBody").innerHTML = warnBoxHTML(t("toast.loadErr") + String(e));
    return;
  }
  st._rows = tp.rows || [];
  const totalPages = tp.total > 0 ? Math.max(1, Math.ceil(tp.total / st.size)) : null;
  let html = `<div class="topo-toolbar">
    <label class="dim" style="font-size:12px">${t("tab.dbs")}
      <select id="tblSchema" style="border:1px solid var(--line);border-radius:6px;padding:3px 6px;background:var(--panel);color:var(--txt);max-width:200px">
        ${(st.schemas || [""]).map((s) => `<option ${st.schema === s ? "selected" : ""}>${esc(s)}</option>`).join("")}
      </select>
    </label>
    <input id="tblFilter" value="${esc(st.filter || "")}" placeholder="${t("tbl.filterPh")}"
      style="border:1px solid var(--line);border-radius:6px;padding:4px 8px;background:var(--panel);color:var(--txt);font-size:12px;width:200px" />
    <span class="dim" style="font-size:12px">${t("sql.pageInfo", { total: tp.total, page: tp.page, size: st.size, ms: 0 }).replace(" · 0ms", "")}</span>
    <label class="dim" style="font-size:12px">${t("sql.limit")}
      <select id="tblSize" style="border:1px solid var(--line);border-radius:6px;padding:3px 6px;background:var(--panel);color:var(--txt)">
        ${[10, 50, 100].map((n) => `<option ${st.size === n ? "selected" : ""}>${n}</option>`).join("")}
      </select>
    </label>
    <span style="flex:1"></span>
    <button class="btn small" id="tblPrev" ${st.page <= 1 ? "disabled" : ""}>${t("sql.prev")}</button>
    <button class="btn small" id="tblNext" ${totalPages && st.page >= totalPages ? "disabled" : ""}>${t("sql.next")}</button>
  </div>`;
  if (!tp.rows || !tp.rows.length) {
    html += `<div class="empty">${t("sql.empty")}</div>`;
  } else {
    html += `<div class="tbl-list-area"><table class="data"><tr>${(tp.cols || []).map((c) => `<th>${esc(c)}</th>`).join("")}<th></th></tr>` +
      tp.rows.map((r, i) => `<tr>${r.map((v) => `<td>${esc(v)}</td>`).join("")}<td><button class="btn small tbl-q" data-i="${i}">${t("sql.browseBtn")}</button></td></tr>`).join("") + `</table></div>`;
  }
  html += `<div id="tblBrowse" class="tbl-browse-area"></div>`;
  $("#tabBody").innerHTML = html;

  $("#tblSchema").onchange = (e) => { st.schema = e.target.value; st.page = 1; st.qry = null; renderTablesPage(); };
  const sizeSel = $("#tblSize");
  if (sizeSel) sizeSel.onchange = (e) => { st.size = parseInt(e.target.value) || 50; st.page = 1; st.qry = null; renderTablesPage(); };
  // 表名关键字筛选（防抖 300ms，跨页生效）
  const filt = $("#tblFilter");
  let debounceTimer = null;
  if (filt) filt.oninput = () => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      st.filter = filt.value.trim();
      st.page = 1;
      renderTablesPage();
      const f2 = $("#tblFilter");
      if (f2) { f2.focus(); f2.setSelectionRange(f2.value.length, f2.value.length); }
    }, 300);
  };
  const prev = $("#tblPrev"), next = $("#tblNext");
  if (prev) prev.onclick = () => { if (st.page > 1) { st.page--; renderTablesPage(); } };
  if (next) next.onclick = () => { st.page++; renderTablesPage(); };
  document.querySelectorAll(".tbl-q").forEach((el) => {
    el.onclick = () => {
      const i = parseInt(el.dataset.i);
      const tname = familyTableName(tp.rows, i, st.schema, tp.cols);
      st.qry = { ident: tname, page: 1 };
      runTableQuery();
    };
  });
  if (st.qry) runTableQuery();
}

// 从行数据取表名（各族列表第一列均为表名，模式由下拉框选定）
function familyTableName(rows, i, schema, cols) {
  const fam = typeOf(S.conns.find((x) => x.id === S.sel)?.type || "").family;
  const r = rows[i] || [];
  const tbl = r[0] || "";
  return quoteIdent(fam, schema, tbl);
}

// 表数据分页浏览（内嵌在数据表页签下方）
async function runTableQuery() {
  const A = API(); if (!A || !S.sel) return;
  const st = tblState();
  if (!st.qry) return;
  const box = $("#tblBrowse");
  if (!box) return;
  box.innerHTML = `<div class="loading"><span class="spin"></span>${LANG === "zh" ? "查询中…" : "Querying…"}</div>`;
  try {
    const r = await A.ExecSQL(S.sel, "SELECT * FROM " + st.qry.ident, st.qry.page, st.size);
    const fam = typeOf(S.conns.find((x) => x.id === S.sel)?.type || "").family;
    const noOffset = fam === "sybase"; // ASE 无 OFFSET：仅第一页
    let html = `<div class="topo-toolbar" style="margin-top:10px">
      <b style="font-size:13px">${t("sql.browse")}：${esc(st.qry.ident)}</b>
      <span class="dim" style="font-size:12px">${esc(r.total >= 0 ? t("sql.pageInfo", { total: r.total, page: r.page, size: r.pageSize, ms: r.elapsedMs }) : t("sql.pageInfoNoTotal", { page: r.page, size: r.pageSize, ms: r.elapsedMs }))}</span>
      ${noOffset ? `<span class="dim" style="font-size:11px">${LANG === "zh" ? "（该数据库不支持偏移分页，仅显示前 N 行）" : "(no OFFSET support; first N rows only)"}</span>` : ""}
      <span style="flex:1"></span>
      <button class="btn small" id="qbPrev" ${r.page <= 1 || noOffset ? "disabled" : ""}>${t("sql.prev")}</button>
      <button class="btn small" id="qbNext" ${noOffset || (r.total >= 0 && r.page * r.pageSize >= r.total) || (r.rows || []).length < r.pageSize ? "disabled" : ""}>${t("sql.next")}</button>
    </div>`;
    if (!r.rows || !r.rows.length) html += `<div class="empty">${t("sql.empty")}</div>`;
    else html += `<div class="sql-result"><table class="data"><tr>${(r.columns || []).map((c) => `<th>${esc(c)}</th>`).join("")}</tr>` +
      r.rows.map((row) => `<tr>${row.map((v) => `<td>${esc(v)}</td>`).join("")}</tr>`).join("") + `</table></div>`;
    box.innerHTML = html;
    const prev = $("#qbPrev"), next = $("#qbNext");
    if (prev) prev.onclick = () => { if (st.qry.page > 1) { st.qry.page--; runTableQuery(); } };
    if (next) next.onclick = () => { st.qry.page++; runTableQuery(); };
  } catch (e) {
    box.innerHTML = warnBoxHTML(t("toast.loadErr") + String(e));
  }
}

/* ---------------- SQL 控制台（高级模式） ---------------- */
function sqlState() {
  S.sqlByConn = S.sqlByConn || {};
  S.sqlByConn[S.sel] = S.sqlByConn[S.sel] || { text: "", size: 200, page: 1, armed: false };
  return S.sqlByConn[S.sel];
}

function renderSQLConsole() {
  const st = sqlState();
  const hist = loadSQLHist();
  $("#tabBody").innerHTML = `
    <div class="sql-toolbar">
      <button class="btn primary small" id="sqlRun">${t("sql.run")}</button>
      <label class="dim" style="font-size:12px">${t("sql.limit")}
        <select id="sqlSize" style="border:1px solid var(--line);border-radius:6px;padding:3px 6px;background:var(--panel);color:var(--txt)">
          ${[50, 100, 200, 500].map((n) => `<option ${st.size === n ? "selected" : ""}>${n}</option>`).join("")}
        </select>
      </label>
      <label class="dim" style="font-size:12px">${t("sql.hist")}
        <select id="sqlHist" style="border:1px solid var(--line);border-radius:6px;padding:3px 6px;background:var(--panel);color:var(--txt);max-width:260px">
          <option value="">— ${hist.length} —</option>
          ${hist.map((h, i) => `<option value="${i}">${esc(h.sql.replace(/\s+/g, " ").slice(0, 60))}</option>`).join("")}
        </select>
      </label>
      <button class="btn small" id="sqlHistClear">${t("sql.histClear")}</button>
      <span class="dim" style="font-size:11px">${t("sql.hint")}</span>
    </div>
    <textarea id="sqlInput" class="sql-input" spellcheck="false" placeholder="SELECT * FROM ...">${esc(st.text)}</textarea>
    <div id="sqlResult"></div>`;
  const input = $("#sqlInput");
  input.oninput = () => { st.text = input.value; st.armed = false; };
  input.onkeydown = (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter") { e.preventDefault(); runSQL(1); }
  };
  $("#sqlSize").onchange = (e) => { st.size = parseInt(e.target.value) || 200; st.page = 1; };
  $("#sqlHist").onchange = (e) => {
    const h = loadSQLHist();
    const item = h[parseInt(e.target.value)];
    if (item) { st.text = item.sql; input.value = item.sql; }
    e.target.value = "";
  };
  $("#sqlHistClear").onclick = () => {
    localStorage.removeItem("dbping.sqlhist");
    renderSQLConsole();
  };
  $("#sqlRun").onclick = () => {
    if (st.armed) { runSQL(1); return; }
    const stmts = splitSQLStatements(st.text || "");
    const isRead = stmts.length > 0 && stmts.every((s) => {
      const u = s.toUpperCase();
      return u.startsWith("SELECT") || u.startsWith("SHOW") || u.startsWith("PRAGMA") ||
        u.startsWith("EXPLAIN") || u.startsWith("WITH") || u.startsWith("DESC");
    });
    if (!isRead) {
      st.armed = true;
      $("#sqlRun").textContent = t("sql.runConfirm");
      setTimeout(() => { st.armed = false; const b = $("#sqlRun"); if (b) b.textContent = t("sql.run"); }, 3000);
      return;
    }
    runSQL(1);
  };
  if (st.last) renderSQLResult(st.last);
}

/* ---------------- SQL 历史（localStorage 持久化） ---------------- */
function loadSQLHist() {
  try { return JSON.parse(localStorage.getItem("dbping.sqlhist") || "[]"); } catch { return []; }
}
function pushSQLHist(sql) {
  const s = String(sql || "").trim();
  if (!s) return;
  const h = loadSQLHist();
  if (h.length && h[0].sql === s) { h[0].ts = Date.now(); } // 连续相同不重复
  else h.unshift({ sql: s, ts: Date.now() });
  if (h.length > 100) h.length = 100;
  try { localStorage.setItem("dbping.sqlhist", JSON.stringify(h)); } catch { }
  refreshSQLHistSelect();
}
function refreshSQLHistSelect() {
  const sel = document.getElementById("sqlHist");
  if (!sel) return;
  const hist = loadSQLHist();
  sel.innerHTML = `<option value="">— ${hist.length} —</option>` +
    hist.map((h, i) => `<option value="${i}">${esc(h.sql.replace(/\s+/g, " ").slice(0, 60))}</option>`).join("");
}

// 按分号切分多条语句（容忍字符串/注释内的分号）
function splitSQLStatements(text) {
  const out = [];
  let cur = "", inS = false, inD = false, inLC = false, inBC = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i], nx = text[i + 1];
    if (inLC) { cur += ch; if (ch === "\n") inLC = false; continue; }
    if (inBC) { cur += ch; if (ch === "*" && nx === "/") { cur += nx; i++; inBC = false; } continue; }
    if (inS) { cur += ch; if (ch === "'") { if (nx === "'") { cur += nx; i++; } else inS = false; } continue; }
    if (inD) { cur += ch; if (ch === '"') inD = false; continue; }
    if (ch === "-" && nx === "-") { inLC = true; cur += ch; continue; }
    if (ch === "/" && nx === "*") { inBC = true; cur += ch; continue; }
    if (ch === "'") { inS = true; cur += ch; continue; }
    if (ch === '"') { inD = true; cur += ch; continue; }
    if (ch === ";") { if (cur.trim()) out.push(cur.trim()); cur = ""; continue; }
    cur += ch;
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

async function runSQL(page) {
  const A = API(); if (!A || !S.sel) return;
  const st = sqlState();
  st.armed = false;
  const btn = $("#sqlRun"); if (btn) btn.textContent = t("sql.run");
  const out = $("#sqlResult");
  const stmts = splitSQLStatements(st.text || "");
  if (!stmts.length) { if (out) out.innerHTML = `<div class="empty">${t("sql.empty")}</div>`; return; }

  // 单条语句：走完整分页流程
  if (stmts.length === 1) {
    pushSQLHist(stmts[0]);
    st.page = page || 1;
    if (out) out.innerHTML = `<div class="loading"><span class="spin"></span>${LANG === "zh" ? "执行中…" : "Running…"}</div>`;
    try {
      const r = await A.ExecSQL(S.sel, stmts[0], st.page, st.size);
      st.last = r;
      renderSQLResult(r);
    } catch (e) {
      if (out) out.innerHTML = warnBoxHTML(String(e));
    }
    return;
  }

  // 多条语句：按顺序执行，出错即停止
  st.page = 1;
  st.last = null;
  for (const s of stmts) pushSQLHist(s);
  if (out) out.innerHTML = `<div class="loading"><span class="spin"></span>${LANG === "zh" ? "执行中…" : "Running…"}</div>`;
  const blocks = [];
  let failIdx = -1;
  const perPage = Math.min(st.size, 50);
  for (let i = 0; i < stmts.length; i++) {
    try {
      const r = await A.ExecSQL(S.sel, stmts[i], 1, perPage);
      blocks.push({ idx: i, stmt: stmts[i], r });
    } catch (e) {
      failIdx = i;
      blocks.push({ idx: i, stmt: stmts[i], err: String(e) });
      break;
    }
  }
  renderMultiResult(stmts, blocks, failIdx);
}

function renderMultiResult(stmts, blocks, failIdx) {
  const out = $("#sqlResult"); if (!out) return;
  const okCount = blocks.filter((b) => !b.err).length;
  let html = `<div class="topo-toolbar"><span class="dim" style="font-size:12px">` +
    (LANG === "zh"
      ? `共 ${stmts.length} 条 · 成功 ${okCount} 条` + (failIdx >= 0 ? ` · ${t("sql.stopped", { i: failIdx + 1 })}` : "")
      : t("sql.multi", { n: stmts.length, ok: okCount }) + (failIdx >= 0 ? ` · ${t("sql.stopped", { i: failIdx + 1 })}` : "")) +
    `</span></div>`;

  html += blocks.map((b) => {
    const head = `<div class="stmt-head ${b.err ? "err" : "ok"}">#${b.idx + 1} ${b.err ? "✗ 失败" : "✓ 成功"}</div>`;
    const code = `<pre class="stmt-sql ${b.err ? "err" : ""}">${esc(b.stmt)}</pre>`;
    if (b.err) {
      return `<div class="stmt-block">${head}${code}<div class="stmt-err">${esc(b.err)}</div></div>`;
    }
    const r = b.r;
    if (r.isQuery) {
      const n = (r.rows || []).length;
      let info = `<div class="stmt-info">${t("sql.query", { n: r.total >= 0 ? r.total : n })} · ${r.elapsedMs}ms` +
        (n > 8 ? ` · ${t("sql.first8")}` : "") + `</div>`;
      let tbl = "";
      if (n > 0) {
        tbl = `<div class="sql-result mini"><table class="data"><tr>${(r.columns || []).map((c) => `<th>${esc(c)}</th>`).join("")}</tr>` +
          r.rows.slice(0, 8).map((row) => `<tr>${row.map((v) => `<td>${esc(v)}</td>`).join("")}</tr>`).join("") + `</table></div>`;
      }
      return `<div class="stmt-block">${head}${code}${info}${tbl}</div>`;
    }
    return `<div class="stmt-block">${head}${code}<div class="stmt-info">${t("sql.affected", { n: r.affected, ms: r.elapsedMs })}</div></div>`;
  }).join("");

  if (failIdx >= 0 && failIdx + 1 < stmts.length) {
    html += `<div class="dim" style="font-size:12px;padding:4px 2px">${t("sql.notRun")}: ` +
      stmts.slice(failIdx + 1).map((s, i) => `#${failIdx + 2 + i}`).join(", ") + `</div>`;
  }
  out.innerHTML = html;
}

function renderSQLResult(r) {
  const out = $("#sqlResult"); if (!out || !r) return;
  const st = sqlState();
  let html = "";
  if (r.isQuery) {
    const info = r.total >= 0
      ? t("sql.pageInfo", { total: r.total, page: r.page, size: r.pageSize, ms: r.elapsedMs })
      : t("sql.pageInfoNoTotal", { page: r.page, size: r.pageSize, ms: r.elapsedMs });
    html += `<div class="topo-toolbar">
      <span class="dim" style="font-size:12px">${esc(info)}</span>
      <span style="flex:1"></span>
      <button class="btn small" id="sqlPrev" ${r.page <= 1 ? "disabled" : ""}>${t("sql.prev")}</button>
      <button class="btn small" id="sqlNext" ${(r.total >= 0 && r.page * r.pageSize >= r.total) || (r.rows || []).length < r.pageSize ? "disabled" : ""}>${t("sql.next")}</button>
    </div>`;
    if (r.truncated) html += `<div class="warnbox">${t("sql.truncated")}</div>`;
    if (!r.rows || !r.rows.length) {
      html += `<div class="empty">${t("sql.empty")}</div>`;
    } else {
      html += `<div class="sql-result"><table class="data"><tr>${(r.columns || []).map((c) => `<th>${esc(c)}</th>`).join("")}</tr>` +
        r.rows.map((row) => `<tr>${row.map((v) => `<td>${esc(v)}</td>`).join("")}</tr>`).join("") + `</table></div>`;
    }
  } else {
    html = `<div class="warnbox" style="border-color:#86efac;background:#f0fdf4;color:var(--ok)">${t("sql.affected", { n: r.affected, ms: r.elapsedMs })}</div>`;
  }
  out.innerHTML = html;
  const prev = $("#sqlPrev"), next = $("#sqlNext");
  if (prev) prev.onclick = () => { if (r.page > 1) runSQL(r.page - 1); };
  if (next) next.onclick = () => runSQL(r.page + 1);
}

function renderAutoState() {
  const on = S.settings && S.settings.autoPing;
  const btn = document.querySelector("#btnSettings");
  if (btn) btn.classList.toggle("on", !!on);
}

/* ---------------- Ping 事件 ---------------- */
function onPingUpdate(results) {
  if (!Array.isArray(results)) return;
  for (const r of results) {
    S.latest[r.connId] = r;
    if (!r.ok && !S.pinging) toast(`【${connName(r.connId)}】${(r.error || "").slice(0, 80)}`, "err");
  }
  renderSidebar(); renderStats(); renderCards();
  if (S.sel && S.tab === "history") loadTab("history", true);
  if (S.sel) updateDetailStatus();
}
function connName(id) { const c = S.conns.find((x) => x.id === id); return c ? c.name : id; }

/* ---------------- 详情页 ---------------- */
// 缓存策略：每个连接独立缓存（overview/detail/topo）；
// 左侧单击显示缓存信息（无缓存才首次采集），双击=重新连接（清缓存重采）
function cacheFor(id) {
  S.detailCache[id] = S.detailCache[id] || {};
  return S.detailCache[id];
}

async function showDetail(id, opts = {}) {
  const refresh = !!opts.refresh;
  if (refresh) delete S.detailCache[id];
  S.sel = id; S.tab = "overview";
  $("#dashboard").style.display = "none";
  $("#detail").style.display = "block";
  const c = S.conns.find((x) => x.id === id);
  if (!c) return;
  const ti = typeOf(c.type);
  $("#dIcon").textContent = ti.icon;
  $("#dName").textContent = c.name;
  $("#dMeta").textContent = detailMeta();
  document.querySelectorAll(".tab").forEach((el) => el.classList.toggle("active", el.dataset.tab === "overview"));
  updateDetailStatus();
  await loadTab("overview", refresh);
  renderSidebar();
}
function backToDash() {
  S.sel = null;
  $("#detail").style.display = "none";
  $("#dashboard").style.display = "block";
  if (S.topoChart) { S.topoChart.dispose(); S.topoChart = null; }
  if (S.histChart) { S.histChart.dispose(); S.histChart = null; }
  renderSidebar();
}
function updateDetailStatus() {
  const st = statusOf(S.sel);
  const r = S.latest[S.sel];
  const badge = $("#dStatus");
  badge.className = "status-badge " + st;
  const label = { ok: t("st.ok"), warn: t("st.warn"), err: t("st.err"), na: t("st.na") }[st];
  badge.textContent = "● " + label + (r && r.ok ? `  ${r.tcpMs}/${r.sqlMs}ms` : "");
}

async function loadTab(tab, force) {
  const A = API(); if (!A || !S.sel) return;
  const body = $("#tabBody");
  const cache = cacheFor(S.sel);
  const fromCache = (k) => !!cache[k] && !force;
  body.innerHTML = `<div class="loading"><span class="spin"></span>${LANG === "zh" ? "加载中…" : "Loading…"}</div>`;
  try {
    if (tab === "overview") {
      if (!fromCache("overview")) cache.overview = await A.GetOverview(S.sel);
      body.innerHTML = overviewHTML(cache.overview);
    } else if (tab === "ai") {
      const ai = S.dbAIReports[S.sel];
      let histHTML = "";
      try {
        const items = (await A.GetDBAIHistory()) || [];
        if (items.length) {
          histHTML = `<div class="dim" style="font-size:12px;margin:14px 0 4px">${t("ai.history")}</div>` +
            items.map((e, i) => `<div class="nh-item dbai-row" data-i="${i}">
              <span class="dim" style="font-size:11px;flex:none">${new Date(e.time).toLocaleString()}</span>
              <b style="flex:none">${esc(e.connName || e.connId || "-")}</b>
              <span class="dim" style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(e.connType || "")}</span>
              <span style="flex:none">🤖 ${esc(e.ai && e.ai.model || "")}</span>
            </div>`).join("");
        }
      } catch { }
      body.innerHTML = (ai
        ? aiSectionHTML(ai, "db")
        : `<div class="empty"><div class="big">🤖</div>${t("ai.noReport")}<br><br>
           <button class="btn primary" onclick="aiAnalyzeDB()">${t("ai.analyze")}</button></div>`) + histHTML;
      body.querySelectorAll(".dbai-row").forEach((el) => {
        el.onclick = async () => {
          try {
            const A2 = API(); if (!A2) return;
            const items = (await A2.GetDBAIHistory()) || [];
            const e = items[+el.dataset.i];
            if (!e) return;
            body.innerHTML = `<div style="margin:4px 0 10px"><button class="btn small" onclick="loadTab('ai')">&larr; ${t("host.backToList")}</button>
              <span class="dim" style="font-size:12px;margin-left:10px">${new Date(e.time).toLocaleString()} · ${esc(e.connName || "")}</span></div>
              <div class="ai-box"><div class="ai-head"><span class="ai-ico">🤖</span><b>${t("ai.section")}</b>
              <span class="dim ai-meta">${esc(e.ai && e.ai.model || "")} · ${new Date(e.time).toLocaleString()}</span></div>
              <div class="ai-body">${(e.ai && e.ai.html) || ""}</div></div>`;
          } catch { }
        };
      });
    } else if (tab === "topology") {
      if (!fromCache("topo")) cache.topo = await A.GetTopology(S.sel);
      body.innerHTML = `
        <div class="layer-chips" id="layerChips"></div>
        <div class="topo-toolbar">
          <button class="btn small" id="btnTreeExpand">${t("topo.expandAll")}</button>
          <button class="btn small" id="btnTreeCollapse">${t("topo.collapseAll")}</button>
          <span class="dim" style="font-size:11px">${t("topo.expandTip")}</span>
        </div>
        <div class="xtree-wrap" id="topoTree"></div>`;
      renderTopoTree(cache.topo);
    } else if (tab === "tablespage") {
      body.innerHTML = `<div class="loading"><span class="spin"></span>${LANG === "zh" ? "加载中…" : "Loading…"}</div>`;
      await renderTablesPage(force);
    } else if (tab === "sql") {
      renderSQLConsole();
    } else if (tab === "history") {
      const hist = (await A.GetHistory(S.sel)) || []; // 历史记录保持实时
      body.innerHTML = historyHTML(hist);
      renderHistChart(hist);
    } else {
      if (!fromCache("detail")) cache.detail = await A.GetDetail(S.sel);
      const d = cache.detail || {};
      if (tab === "tbs") body.innerHTML = tbsHTML(d.tbs);
      else if (tab === "services") body.innerHTML = servicesHTML(d.services);
      else if (tab === "dbs") body.innerHTML = dbsHTML(d.dbs);
    }
    S.tab = tab;
  } catch (e) {
    body.innerHTML = warnBoxHTML(t("toast.loadErr") + String(e));
  }
}

/* ---------------- 概览（大图标 + 列表 + 归档命令提示） ---------------- */
const DB_SVG = (accent) => `<svg viewBox="0 0 64 64">
  <defs><linearGradient id="dbg${accent}" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="#3b82f6"/><stop offset="1" stop-color="#06b6d4"/></linearGradient></defs>
  <rect x="2" y="2" width="60" height="60" rx="14" fill="url(#dbg${accent})"/>
  <path d="M17 22c0-4.4 6.7-8 15-8s15 3.6 15 8v20c0 4.4-6.7 8-15 8s-15-3.6-15-8V22z" fill="#f8fafc"/>
  <ellipse cx="32" cy="22" rx="15" ry="6.5" fill="#dbeafe"/>
  <path d="M17 32c2.6 2.7 8 4.5 15 4.5s12.4-1.8 15-4.5" stroke="#cbd5e1" stroke-width="1.6" fill="none"/>
  <path d="M17 40c2.6 2.7 8 4.5 15 4.5s12.4-1.8 15-4.5" stroke="#cbd5e1" stroke-width="1.6" fill="none"/>
</svg>`;

// 各数据库开启归档/日志的参考命令（注释随界面语言切换，SQL 本身不变）
const ARCH_HINTS = {
  zh: {
    pg: ["# postgresql.conf", "archive_mode = on", "archive_command = 'test ! -f /archive/%f && cp %p /archive/%f'", "", "ALTER SYSTEM SET archive_mode = 'on';", "# 重启生效", "systemctl restart postgresql", "", "-- 手动切换 WAL 测试归档", "SELECT pg_switch_wal();"],
    kingbase: ["# kingbase.conf", "archive_mode = on", "archive_command = 'cp %p /archive/%f'", "", "# 重启生效后验证", "SHOW archive_mode;"],
    opengauss: ["# postgresql.conf（openGauss）", "archive_mode = on", "archive_command = 'cp %p /archive/%f'", "", "gs_ctl restart -D /data/opengauss", "", "-- 验证", "SHOW archive_mode;"],
    mysql: ["# my.cnf [mysqld]", "server_id = 1", "log_bin = mysql-bin", "binlog_format = ROW", "", "# 重启后验证", "SHOW VARIABLES LIKE 'log_bin';"],
    oracle: ["-- Oracle 开启归档（需 MOUNT 状态）", "SHUTDOWN IMMEDIATE;", "STARTUP MOUNT;", "ALTER DATABASE ARCHIVELOG;", "ALTER DATABASE OPEN;", "", "-- 验证", "ARCHIVE LOG LIST;", "ALTER SYSTEM SWITCH LOGFILE;"],
    dm: ["-- 达梦开启归档", "ALTER DATABASE MOUNT;", "ALTER DATABASE ARCHIVELOG;", "ALTER DATABASE OPEN;", "", "-- 或配置 dm.ini: ARCH_INI=1 + 归档配置", "SELECT ARCH_MODE FROM V$DATABASE;"],
    hana: ["-- HANA 日志模式切换", "ALTER SYSTEM LOG MODE NORMAL;     -- 归档", "ALTER SYSTEM LOG MODE OVERWRITE;  -- 覆盖写(不归档)", "", "-- 验证", "SELECT VALUE FROM M_SYSTEM_OVERVIEW", "WHERE NAME = 'Log Mode';"],
    mssql: ["-- SQL Server 切换为完整恢复模式", "ALTER DATABASE [MyDB] SET RECOVERY FULL;", "", "-- 首次全备后日志链生效", "BACKUP DATABASE [MyDB] TO DISK = '...';", "BACKUP LOG [MyDB] TO DISK = '...';"],
    redis: ["# RDB: redis.conf", "save 900 1", "save 300 10", "", "# AOF 运行时开启", "CONFIG SET appendonly yes", "CONFIG REWRITE"],
    mongo: ["// MongoDB: 副本集自动启用 oplog", "// journal 默认开启（WiredTiger）", "", "// 验证副本集", "db.adminCommand({ replSetGetStatus: 1 })"],
    clickhouse: ["-- ClickHouse 无 WAL 归档概念", "-- 备份工具示例", "clickhouse-backup create", "clickhouse-backup list"],
    oceanbase: ["-- OceanBase 日志归档", "ALTER SYSTEM SET ARCHIVELOG = true;", "", "-- 查看归档进度", "SELECT * FROM oceanbase.CDB_OB_ARCHIVELOG;"],
    generic: ["-- 参考官方文档开启归档/日志归档"],
  },
  en: {
    pg: ["# postgresql.conf", "archive_mode = on", "archive_command = 'test ! -f /archive/%f && cp %p /archive/%f'", "", "ALTER SYSTEM SET archive_mode = 'on';", "# restart to take effect", "systemctl restart postgresql", "", "-- manually switch a WAL to test archiving", "SELECT pg_switch_wal();"],
    kingbase: ["# kingbase.conf", "archive_mode = on", "archive_command = 'cp %p /archive/%f'", "", "# verify after restart", "SHOW archive_mode;"],
    opengauss: ["# postgresql.conf (openGauss)", "archive_mode = on", "archive_command = 'cp %p /archive/%f'", "", "gs_ctl restart -D /data/opengauss", "", "-- verify", "SHOW archive_mode;"],
    mysql: ["# my.cnf [mysqld]", "server_id = 1", "log_bin = mysql-bin", "binlog_format = ROW", "", "# verify after restart", "SHOW VARIABLES LIKE 'log_bin';"],
    oracle: ["-- Enable Oracle archiving (requires MOUNT state)", "SHUTDOWN IMMEDIATE;", "STARTUP MOUNT;", "ALTER DATABASE ARCHIVELOG;", "ALTER DATABASE OPEN;", "", "-- verify", "ARCHIVE LOG LIST;", "ALTER SYSTEM SWITCH LOGFILE;"],
    dm: ["-- Enable DM archiving", "ALTER DATABASE MOUNT;", "ALTER DATABASE ARCHIVELOG;", "ALTER DATABASE OPEN;", "", "-- or dm.ini: ARCH_INI=1 + archive config file", "SELECT ARCH_MODE FROM V$DATABASE;"],
    hana: ["-- HANA log mode switch", "ALTER SYSTEM LOG MODE NORMAL;     -- archiving", "ALTER SYSTEM LOG MODE OVERWRITE;  -- overwrite (no archive)", "", "-- verify", "SELECT VALUE FROM M_SYSTEM_OVERVIEW", "WHERE NAME = 'Log Mode';"],
    mssql: ["-- Switch SQL Server to FULL recovery model", "ALTER DATABASE [MyDB] SET RECOVERY FULL;", "", "-- log chain becomes valid after the first full backup", "BACKUP DATABASE [MyDB] TO DISK = '...';", "BACKUP LOG [MyDB] TO DISK = '...';"],
    redis: ["# RDB: redis.conf", "save 900 1", "save 300 10", "", "# enable AOF at runtime", "CONFIG SET appendonly yes", "CONFIG REWRITE"],
    mongo: ["// MongoDB: replica sets enable the oplog automatically", "// journal is on by default (WiredTiger)", "", "// verify replica set", "db.adminCommand({ replSetGetStatus: 1 })"],
    clickhouse: ["-- ClickHouse has no WAL-archiving concept", "-- backup tool example", "clickhouse-backup create", "clickhouse-backup list"],
    oceanbase: ["-- OceanBase log archiving", "ALTER SYSTEM SET ARCHIVELOG = true;", "", "-- check archive progress", "SELECT * FROM oceanbase.CDB_OB_ARCHIVELOG;"],
    generic: ["-- Refer to vendor documentation to enable archiving"],
  },
};
function archHintFor(type) {
  const H = ARCH_HINTS[LANG] || ARCH_HINTS.zh;
  if (H[type]) return H[type];
  const fam = typeOf(type).family;
  if (fam === "pg" && H.pg) return H.pg;
  if (fam === "mysql" && H.mysql) return H.mysql;
  if (fam === "oracle" && H.oracle) return H.oracle;
  return H.generic;
}

function hintSpan(type) {
  const lines = archHintFor(type);
  return `<span class="hint-i" title="${t("ov.archiveHint")}">ⓘ<div class="tip">` +
    `<div class="tt">${t("ov.archiveHint")}</div><pre>${esc(lines.join("\n"))}</pre></div></span>`;
}

function overviewHTML(ov) {
  if (!ov) return `<div class="empty">${t("toast.noData")}</div>`;
  const c = S.conns.find((x) => x.id === S.sel) || {};
  const ti = typeOf(c.type);
  const st = statusOf(S.sel);
  const idd = "g" + Math.abs(hashStr(c.type || "x"));
  const ro = ov.readOnly === "是" || String(ov.readOnly).toLowerCase() === "yes" ? t("yes")
    : (ov.readOnly === "否" || String(ov.readOnly).toLowerCase() === "no" ? t("no") : ov.readOnly);

  const heroBadges = [];
  if (ov.role && ov.role !== "-") heroBadges.push(`<span class="b">${t("ov.role")}: <b>${esc(LVAL(ov.role))}</b></span>`);
  if (ov.cluster && ov.cluster !== "-") heroBadges.push(`<span class="b">${t("ov.cluster")}: <b>${esc(LVAL(ov.cluster))}</b></span>`);
  if (ov.startupTime && ov.uptime) heroBadges.push(`<span class="b">${t("ov.uptime")}: <b>${esc(ov.uptime)}</b></span>`);

  const row = (k, v, extra) => (v && v !== "-" && v !== "")
    ? `<tr><th>${k}${extra || ""}</th><td>${esc(v)}</td></tr>` : "";
  let rows = "";
  rows += row(t("ov.version"), ov.versionFull);
  rows += row(t("ov.instance"), ov.instance);
  rows += row(t("ov.hostname"), ov.hostname);
  rows += row(t("ov.os"), ov.os);
  rows += row(t("ov.archive"), LVAL(ov.archiveMode), hintSpan(c.type));
  rows += row(t("ov.archiveDest"), LVAL(ov.archiveDest));
  rows += row(t("ov.charset"), ov.charset);
  rows += row(t("ov.tz"), ov.timezone);
  rows += row(t("ov.startup"), ov.startupTime);
  rows += row(t("ov.readonly"), ro);
  rows += row(t("ov.dbsize"), ov.dbSize);
  rows += row(t("ov.objects"), ov.tableCount);
  rows += row(t("ov.conns"), ov.connCur && ov.connMax ? `${ov.connCur} / ${ov.connMax}` : ov.connCur);
  rows += row(t("ov.cluster"), LVAL(ov.cluster));
  for (const kv of ov.extra || []) rows += row(esc(LKEY(kv.key)), LVAL(kv.value));

  let html = `
  <div class="ov-hero">
    <div class="ov-icon">${DB_SVG(idd)}<span class="ov-dot ${st}"></span>${ov.os ? `<span class="ov-os" title="${esc(ov.os)}">${osEmoji(ov.os)}</span>` : ""}</div>
    <div class="ov-id">
      <div class="ov-title">${esc(ti.icon)} ${esc(ti.name)} ${esc(ov.version)}<span class="chip">${esc(c.name || "")}</span></div>
      <div class="ov-sub">${esc(c.host)}:${esc(c.port)}${c.database ? " / " + esc(c.database) : ""} · ${esc(c.group || "")}</div>
      <div class="ov-badges">${heroBadges.join("")}</div>
    </div>
  </div>
  <table class="ov-list">${rows}</table>`;
  if (ov.warnings && ov.warnings.length) {
    html += `<div class="warnbox"><b>${t("ov.tips")}</b><br>` + ov.warnings.map(esc).join("<br>") + `</div>`;
  }
  return html;
}
function hashStr(s) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return h;
}

// osEmoji 按操作系统字符串给出标识图标
function osEmoji(os) {
  const s = String(os).toLowerCase();
  if (s.includes("win")) return "🪟";
  if (s.includes("darwin") || s.includes("macos") || s.includes("mac os")) return "🍎";
  if (s.includes("linux") || s.includes("unix") || s.includes("aix") || s.includes("hp-ux") || s.includes("solaris")) return "🐧";
  return "🖥";
}

/* ---------------- 后端返回值本地化（英文模式） ----------------
 * 后端采集值为固定中文措辞（我们可控），用词典映射为英文；中文模式原样返回。 */
const ZH2EN_VALUES = [
  // 归档/日志模式
  [/^归档未开启（仅 WAL: (.+)）$/, (m) => `Archiving off (WAL: ${m[1]})`],
  [/^归档已开启（archive_mode=on）$/, () => "Archiving on (archive_mode=on)"],
  [/^归档未开启$/, () => "Archiving off"],
  [/^Binlog 已开启（binlog）$/, () => "Binlog enabled"],
  [/^Binlog 未开启$/, () => "Binlog disabled"],
  [/^ARCHIVELOG 归档模式$/, () => "ARCHIVELOG"],
  [/^NOARCHIVELOG 非归档$/, () => "NOARCHIVELOG"],
  [/^NORMAL（日志归档已启用）$/, () => "NORMAL (archiving enabled)"],
  [/^OVERWRITE（覆盖写，不归档！）$/, () => "OVERWRITE (no archiving!)"],
  [/^FULL 完整恢复（支持日志备份\/时点恢复）$/, () => "FULL recovery (log backup / PITR)"],
  [/^BULK_LOGGED 大容量日志恢复$/, () => "BULK_LOGGED recovery"],
  [/^SIMPLE 简单恢复（无日志归档）$/, () => "SIMPLE recovery (no log archiving)"],
  [/^未知（无 v\$database 权限）$/, () => "Unknown (no v$database privilege)"],
  // 角色
  [/^主库（可写）$/, () => "Primary (writable)"],
  [/^备库（恢复中\/只读）$/, () => "Standby (recovering / read-only)"],
  [/^主库（(\d+) 个复制从属）$/, (m) => `Primary (${m[1]} replicas)`],
  [/^从库（复制中，来源 (.+)）$/, (m) => `Replica (syncing from ${m[1]})`],
  [/^从库（复制异常）$/, () => "Replica (replication broken)"],
  [/^主库$/, () => "Primary"],
  [/^从库$/, () => "Replica"],
  [/^单机$/, () => "Standalone"],
  [/^未知$/, () => "Unknown"],
  // 集群形态
  [/^分布式（TiDB\/PD\/TiKV）$/, () => "Distributed (TiDB/PD/TiKV)"],
  [/^分布式集群$/, () => "Distributed cluster"],
  [/^集群节点$/, () => "Cluster node"],
  [/^主从复制（半同步 (\d+) 从）$/, (m) => `Primary-Replica (semi-sync ${m[1]})`],
  [/^主从复制（(\d+) 从）$/, (m) => `Primary-Replica (${m[1]} replicas)`],
  [/^主从复制$/, () => "Primary-Replica"],
  [/^CDB 多租户架构$/, () => "CDB multitenant"],
  [/^MDC 多租户（(\d+) 库）$/, (m) => `MDC multitenant (${m[1]} DBs)`],
  [/^Scale-Out 集群（(\d+) 主机）$/, (m) => `Scale-Out cluster (${m[1]} hosts)`],
  [/^分布式（(\d+) 节点）$/, (m) => `Distributed (${m[1]} nodes)`],
  [/^MPP 集群$/, () => "MPP cluster"],
  [/^分布式$/, () => "Distributed"],
  [/^副本集 (.+)$/, (m) => `Replica set ${m[1]}`],
  [/^Cluster 分片集群$/, () => "Cluster (sharded)"],
  [/^Master\/Segment 架构$/, () => "Master/Segment architecture"],
  [/^(\d+) 个集群（system\.clusters）$/, (m) => `${m[1]} clusters (system.clusters)`],
  // 服务角色/状态
  [/^复制源（Master）$/, () => "Master (source)"],
  [/^半同步从库$/, () => "Semi-sync replicas"],
  [/^复制从属（(.+)）$/, (m) => `Replica (${m[1]})`],
  [/^从库（主: (.+)）$/, (m) => `Replica (master: ${m[1]})`],
  [/^OBServer（RootServer）$/, () => "OBServer (RootServer)"],
  [/^租户（(.+)）$/, (m) => `Tenant (${m[1]})`],
  [/^FE（(.+)）$/, (m) => `FE (${m[1]})`],
  [/^CN 协调节点$/, () => "CN (coordinator)"],
  [/^DN 数据节点$/, () => "DN (data node)"],
  [/^GTM 备机$/, () => "GTM standby"],
  [/^Segment（Primary）$/, () => "Segment (Primary)"],
  [/^Segment（Mirror）$/, () => "Segment (Mirror)"],
  [/^主节点$/, () => "Master"],
  [/^从节点$/, () => "Replica node"],
  [/^副本集成员$/, () => "Replica set member"],
  [/^TiKV Store#(.+)$/, (m) => `TiKV Store#${m[1]}`],
  [/^运行中$/, () => "Running"],
  [/^停止$/, () => "Stopped"],
  [/^正常$/, () => "Normal"],
  [/^复制中$/, () => "Replicating"],
  [/^复制异常$/, () => "Replication broken"],
  [/^Master$/, () => "Master"],
  [/^OBServer$/, () => "OBServer"],
  [/^GTM$/, () => "GTM"],
  [/^BE$/, () => "BE"],
  // 拓扑分支名
  [/^集群\/复制拓扑$/, () => "Cluster / Replication"],
  [/^集群拓扑$/, () => "Cluster Topology"],
  [/^表空间$/, () => "Tablespaces"],
  [/^数据库$/, () => "Databases"],
  [/^数据文件（当前库）$/, () => "Data Files (current db)"],
  [/^磁盘\/卷$/, () => "Disks / Volumes"],
  [/^磁盘$/, () => "Disks"],
  [/^数据目录$/, () => "Data directory"],
  [/^持久化$/, () => "Persistence"],
  [/^RDB 快照$/, () => "RDB snapshot"],
  [/^AOF 追加日志$/, () => "AOF log"],
  [/^键空间$/, () => "Keyspace"],
  [/^大表 Top 15$/, () => "Top 15 Tables"],
  [/^当前库大表 Top 15$/, () => "Top 15 Tables (current db)"],
  [/^PDB 多租户$/, () => "PDB Multitenant"],
  [/^服务拓扑$/, () => "Service Topology"],
  [/^主机 (.+)$/, (m) => `Host ${m[1]}`],
  [/^Schema$/, () => "Schemas"],
  [/^（无键）$/, () => "(no keys)"],
];

// 词级替换（组合串内出现的中文词）
function lvalTokens(s) {
  return String(s)
    .replace(/（系统库）/g, "(system db)")
    .replace(/（系统用户）/g, "(system user)")
    .replace(/（系统模式）/g, "(system schema)")
    .replace(/（空库）/g, "(empty db)")
    .replace(/个组件/g, "components")
    .replace(/个成员/g, "members")
    .replace(/个服务/g, "services")
    .replace(/个文件/g, "files")
    .replace(/个库/g, "DBs")
    .replace(/剩余 /g, "free ")
    .replace(/数据文件目录/g, "datafile dir")
    .replace(/数据文件/g, "datafile")
    .replace(/AOF:已开启/g, "AOF:enabled")
    .replace(/AOF:未开启/g, "AOF:disabled")
    .replace(/状态:/g, "status: ")
    .replace(/启动:/g, "started: ")
    .replace(/表数:/g, "tables: ")
    .replace(/^(\d+) 表$/, "$1 tables")
    .replace(/^(\d+) 个$/, "$1 items");
}

const ZH2EN_KEYS = {
  "默认存储引擎": "Default engine",
  "补丁级别": "Patch level",
  "实例状态": "Instance status",
  "DB名称": "DB name",
  "库名": "DB name",
  "打开模式": "Open mode",
  "用途": "Usage",
  "数据库数量": "Databases",
  "运行模式": "Mode",
  "内存峰值": "Memory peak",
  "活跃服务": "Active services",
  "路径": "Path",
  "详情": "Detail",
};

// LVAL：英文模式下把后端中文值映射为英文；中文模式原样返回
function LVAL(s) {
  if (LANG !== "en" || s == null) return s;
  let out = String(s);
  for (const [re, fn] of ZH2EN_VALUES) {
    const m = out.match(re);
    if (m) { out = fn(m); return out; }
  }
  return lvalTokens(out);
}
function LKEY(s) {
  if (LANG !== "en" || s == null) return s;
  const m = String(s).match(/^租户 (.+)$/);
  if (m) return `Tenant ${m[1]}`;
  return ZH2EN_KEYS[s] || s;
}

/* ---------------- 明细表格 ---------------- */
function tablesHTML(tables) {
  if (!tables || !tables.length) return `<div class="empty">${t("toast.noDetail")}</div>`;
  return `<table class="data"><tr><th>${t("th.schema")}</th><th>${t("th.name")}</th><th>${t("th.rows")}</th><th>${t("th.size")}</th><th>${t("th.comment")}</th></tr>` +
    tables.map((x) => `<tr><td>${esc(x.schema)}</td><td>${esc(x.name)}</td><td>${esc(x.rows)}</td><td>${esc(x.size)}</td><td>${esc(x.comment)}</td></tr>`).join("") + `</table>`;
}
function tbsHTML(tbs) {
  if (!tbs || !tbs.length) return `<div class="empty">${t("toast.noTbs")}</div>`;
  return `<table class="data"><tr><th>${t("th.name")}</th><th>${t("th.size")}</th><th>${t("th.free")}</th><th>${t("th.status")}</th><th>${t("th.files")}</th></tr>` +
    tbs.map((x) => `<tr><td>${esc(x.name)}</td><td>${esc(x.size)}</td><td>${esc(x.free)}</td><td>${esc(x.status)}</td><td style="word-break:break-all">${esc(x.files)}</td></tr>`).join("") + `</table>`;
}
function servicesHTML(svcs) {
  if (!svcs || !svcs.length) return `<div class="empty">${t("toast.noSvc")}</div>`;
  return `<table class="data"><tr><th>${t("th.name")}</th><th>${t("th.role")}</th><th>${t("th.status")}</th><th>${t("th.host")}</th><th>${t("th.detail")}</th></tr>` +
    svcs.map((s) => `<tr><td>${esc(s.name)}</td><td>${esc(LVAL(s.role))}</td>
      <td>${/异常|FAIL|停止|offline/i.test(s.status) ? `<span style="color:var(--err)">${esc(LVAL(s.status))}</span>` : esc(LVAL(s.status))}</td>
      <td>${esc(s.host)}</td><td>${esc(LVAL(s.detail))}</td></tr>`).join("") + `</table>`;
}
function dbsHTML(dbs) {
  if (!dbs || !dbs.length) return `<div class="empty">${t("toast.noData")}</div>`;
  return `<table class="data"><tr><th>${t("th.name")}</th><th>${t("th.size")}</th><th>${t("th.tables")}</th><th>${t("th.charset")}</th><th>${t("th.note")}</th></tr>` +
    dbs.map((d) => `<tr><td>${esc(d.name)}</td><td>${esc(d.size)}</td><td>${esc(d.tables)}</td><td>${esc(d.charset)}</td><td>${esc(LVAL(d.note))}</td></tr>`).join("") + `</table>`;
}

function historyHTML(hist) {
  if (!hist || !hist.length) return `<div class="empty">${t("toast.noHist")}</div>`;
  const okCount = hist.filter((h) => h.ok).length;
  const rate = ((okCount / hist.length) * 100).toFixed(1);
  let html = `<div class="hist-chart" id="histChart"></div>
  <div class="hist-stats">
    <div class="stat"><b>${hist.length}</b><span>${t("hist.records")}</span></div>
    <div class="stat ok"><b>${rate}%</b><span>${t("hist.rate")}</span></div>
    <div class="stat"><b>${avgSql(hist)}<small style="font-size:11px"> ms</small></b><span>${t("hist.avgsql")}</span></div>
  </div>
  <table class="data"><tr><th>${t("th.time")}</th><th>${t("th.result")}</th><th>${t("th.tcp")}</th><th>${t("th.sql")}</th><th>${t("th.desc")}</th></tr>`;
  for (const r of [...hist].reverse().slice(0, 50)) {
    html += `<tr><td>${new Date(r.checkedAt).toLocaleString()}</td>
      <td>${r.ok ? `<span style="color:var(--ok)">${t("st.ok")}</span>` : `<span style="color:var(--err)">${t("st.err")}</span>`}</td>
      <td>${r.tcpMs}</td><td>${r.sqlMs}</td><td style="max-width:340px;word-break:break-all">${esc(r.error || "")}</td></tr>`;
  }
  return html + `</table>`;
}
function avgSql(hist) {
  const oks = hist.filter((h) => h.ok);
  if (!oks.length) return "-";
  return Math.round(oks.reduce((s, h) => s + h.sqlMs, 0) / oks.length);
}

/* ---------------- ECharts（历史曲线） ---------------- */
function cssVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || "#b6c2d4";
}

/* ---------------- 拓扑：资源管理器式树 ---------------- */
// 浅色系节点图标（SVG，随主题淡雅显示）
function topoIconSVG(kind, size) {
  const s = size || 14;
  const wrap = (inner) => `<svg class="xi" width="${s}" height="${s}" viewBox="0 0 16 16">${inner}</svg>`;
  switch (kind) {
    case "folder":
      return wrap(`<path d="M1.5 4.2c0-.6.4-1 1-1h3.4l1.3 1.4h6.3c.6 0 1 .4 1 1v6.2c0 .6-.4 1-1 1h-11c-.6 0-1-.4-1-1V4.2z" fill="#dbe7f5" stroke="#a9c2de" stroke-width="0.9"/>`);
    case "db":
      return wrap(`<ellipse cx="8" cy="3.6" rx="5.6" ry="2.2" fill="#d5ecf7" stroke="#9cc6dd" stroke-width="0.9"/><path d="M2.4 3.6v8.8c0 1.2 2.5 2.2 5.6 2.2s5.6-1 5.6-2.2V3.6" fill="#e5f2fa" stroke="#9cc6dd" stroke-width="0.9"/><path d="M2.4 8c0 1.2 2.5 2.2 5.6 2.2s5.6-1 5.6-2.2" fill="none" stroke="#b9d8e8" stroke-width="0.8"/>`);
    case "table":
      return wrap(`<rect x="2" y="2.5" width="12" height="11" rx="1.2" fill="#e9f0f8" stroke="#aec6dc" stroke-width="0.9"/><path d="M2 5.6h12M2 8.9h12M6 5.6v7.9M10 5.6v7.9" stroke="#c4d6e8" stroke-width="0.8"/>`);
    case "file":
      return wrap(`<path d="M3.5 1.8h6L13 5.3v8.4c0 .4-.3.7-.7.7H3.7c-.4 0-.7-.3-.7-.7V2.5c0-.4.3-.7.7-.7z" fill="#e9f0f8" stroke="#aec6dc" stroke-width="0.9"/><path d="M9.5 1.8v3.5H13" fill="#d5e3f2" stroke="#aec6dc" stroke-width="0.8"/><path d="M5.3 8h5.4M5.3 10.4h5.4" stroke="#c4d6e8" stroke-width="0.8"/>`);
    case "disk":
      return wrap(`<circle cx="8" cy="8" r="6" fill="#dfeaf6" stroke="#a9c2de" stroke-width="0.9"/><circle cx="8" cy="8" r="2" fill="#ffffff" stroke="#a9c2de" stroke-width="0.8"/><path d="M8 2v3.4M8 10.6V14M2 8h3.4M10.6 8H14" stroke="#c3d6ea" stroke-width="0.7"/>`);
    case "gear":
      return wrap(`<path d="M6.6 1.6h2.8l.4 1.7 1.5.9 1.7-.6 1.4 2.4-1.3 1.2v1.6l1.3 1.2-1.4 2.4-1.7-.6-1.5.9-.4 1.7H6.6l-.4-1.7-1.5-.9-1.7.6L1.6 9.9l1.3-1.2V7.1L1.6 5.9 3 3.5l1.7.6 1.5-.9.4-1.6z" fill="#e2ebf5" stroke="#a9c2de" stroke-width="0.8"/><circle cx="8" cy="7.9" r="2" fill="#ffffff" stroke="#a9c2de" stroke-width="0.8"/>`);
    case "host":
      return wrap(`<rect x="2" y="3" width="12" height="8" rx="1" fill="#e6edf6" stroke="#a9c2de" stroke-width="0.9"/><path d="M5.5 13.5h5" stroke="#a9c2de" stroke-width="1.3"/>`);
    case "cluster":
      return wrap(`<path d="M8 6.4L5.2 9.6M8 6.4l2.8 3.2M6.4 11h3.2" stroke="#b7cbe2" stroke-width="0.9"/><circle cx="8" cy="4.4" r="2.3" fill="#dbe7f5" stroke="#a9c2de" stroke-width="0.9"/><circle cx="4.6" cy="11.2" r="2.3" fill="#e2eef8" stroke="#a9c2de" stroke-width="0.9"/><circle cx="11.4" cy="11.2" r="2.3" fill="#e2eef8" stroke="#a9c2de" stroke-width="0.9"/>`);
    default:
      return wrap(`<circle cx="8" cy="8" r="5.5" fill="#e6edf6" stroke="#a9c2de" stroke-width="0.9"/>`);
  }
}

function topoKind(n) {
  const id = n.id || "";
  const hasKids = n.children && n.children.length;
  if (id === "root") return "db";
  if (hasKids) return (id === "cluster" || id === "clusters") ? "cluster" : "folder";
  if (/^tbs-\d+-f/.test(id)) return "file"; // 数据文件
  if (/^tbs-/.test(id) || id === "rdb" || id === "aof" || /^tbs/.test(id)) return "disk";
  if (/^host-/.test(id)) return "host";
  if (/^sh-/.test(id)) return "cluster";
  if (/^rp-|^svc-|^m-|^n-|^s-/.test(id)) return "gear";
  if (/^sch-/.test(id)) return "folder";
  if (/^db-\d+-c-|^db-\d+-t-|^t-|^\w+-t-/.test(id)) return "table";
  if (/^db-\d+$|^db\d|^db-empty/.test(id)) return "db";
  return "table";
}

function topoTreeHTML(nodes, depth, rootKind) {
  return `<ul class="xtree">${nodes.map((n) => {
    const has = n.children && n.children.length;
    const open = depth < 2; // 默认展开到第二层
    const kvs = (n.kvs || []).map((kv) => esc(LKEY(kv.key) + ": " + LVAL(kv.value))).join("　");
    const isRoot = depth === 0;
    return `<li class="xnode">
      <div class="xrow" data-depth="${depth}">
        ${has ? `<span class="xarrow ${open ? "open" : ""}">${open ? "−" : "+"}</span>` : `<span class="xleaf-sp"></span>`}
        <span class="xicon">${topoIconSVG(topoKind(n), isRoot ? 16 : 14)}</span>
        <span class="xlabel">${esc(LVAL(n.label))}</span>
        ${n.sub && !isRoot ? `<span class="xsub">${esc(LVAL(n.sub))}</span>` : ""}
        ${kvs ? `<span class="xkvs dim">${kvs}</span>` : ""}
        <span class="dot ${n.status || "na"}"></span>
      </div>
      ${has ? `<ul class="xchildren" style="${open ? "" : "display:none"}">${topoTreeHTML(n.children, depth + 1, rootKind)}</ul>` : ""}
    </li>`;
  }).join("")}</ul>`;
}

function renderTopoTree(topo) {
  // 层级指示（按层面展示）
  const chips = $("#layerChips");
  if (chips) {
    chips.innerHTML = ["layer.instance", "layer.cluster", "layer.db", "layer.tbs", "layer.table"]
      .map((k) => `<span class="lc"><i></i>${t(k)}</span>`)
      .join(`<span class="arrow">▸</span>`);
  }
  const el = $("#topoTree");
  if (!el || !topo || !topo.root) return;
  el.innerHTML = topoTreeHTML([topo.root], 0, "db");
  // 展开/收起交互（+ / -）
  el.querySelectorAll(".xrow").forEach((row) => {
    const kids = row.parentElement.querySelector(":scope > .xchildren");
    if (!kids) return;
    row.onclick = () => {
      const open = kids.style.display !== "none";
      kids.style.display = open ? "none" : "";
      const arrow = row.querySelector(".xarrow");
      if (arrow) { arrow.classList.toggle("open", !open); arrow.textContent = open ? "+" : "−"; }
    };
    row.ondblclick = row.onclick;
  });
  const setAll = (open) => {
    el.querySelectorAll(".xchildren").forEach((u) => { u.style.display = open ? "" : "none"; });
    el.querySelectorAll(".xarrow").forEach((a) => { a.classList.toggle("open", open); a.textContent = open ? "−" : "+"; });
  };
  $("#btnTreeExpand").onclick = () => setAll(true);
  $("#btnTreeCollapse").onclick = () => setAll(false);
}

function renderHistChart(hist) {
  const el = document.getElementById("histChart");
  if (!el || !hist || !hist.length) return;
  if (S.histChart) S.histChart.dispose();
  S.histChart = echarts.init(el);
  const txt = cssVar("--txt");
  const line = cssVar("--line2");
  S.histChart.setOption({
    tooltip: { trigger: "axis" },
    legend: { data: ["SQL", "TCP"], textStyle: { color: txt } },
    grid: { left: 50, right: 20, top: 36, bottom: 30 },
    xAxis: { type: "category", data: hist.map((h) => new Date(h.checkedAt).toLocaleTimeString()), axisLabel: { color: txt }, axisLine: { lineStyle: { color: line } } },
    yAxis: { type: "value", name: "ms", axisLabel: { color: txt }, splitLine: { lineStyle: { color: line } } },
    series: [
      { name: "SQL", type: "line", smooth: true, showSymbol: false,
        data: hist.map((h) => (h.ok ? h.sqlMs : null)), itemStyle: { color: "#2563eb" },
        areaStyle: { color: "#2563eb14" } },
      { name: "TCP", type: "line", smooth: true, showSymbol: false,
        data: hist.map((h) => h.tcpMs), itemStyle: { color: "#94a3b8" } },
    ],
  });
}

/* ---------------- 连接对话框（图标名录下拉） ---------------- */
// Oracle 特权连接：AUTH TYPE=SYSDBA/SYSOPER 以附加参数形式存储在 params 中
function roleFromParams(p) {
  const m = String(p || "").match(/AUTH\s*TYPE=([^&]+)/i);
  return m ? m[1].trim().toUpperCase() : "";
}
function stripAuthType(p) {
  let s = String(p || "");
  while (/(^|&)\s*AUTH\s*TYPE=[^&]*/i.test(s)) {
    s = s.replace(/(^|&)\s*AUTH\s*TYPE=[^&]*/i, "$1");
  }
  return s.replace(/&&+/g, "&").replace(/^&+|&+$/g, "").trim();
}

/* ---------------- TLS 证书适配（连接对话框证书页） ---------------- */
// 各族的 TLS 模式与字段可见性；参数以各自驱动约定写入 params
const CERT_SPECS = {
  mysql: {
    modes: [["default", "default"], ["preferred", "preferred"], ["required", "required (verify)"], ["skipverify", "required (skip verify)"], ["client", "双向证书 / mTLS"]],
    ca: true, cert: true, key: true, pass: false, sni: true, skip: false,
  },
  pg: {
    modes: [["default", "default"], ["disable", "disable"], ["prefer", "prefer"], ["require", "require"], ["verify-ca", "verify-ca"], ["verify-full", "verify-full"]],
    ca: true, cert: true, key: true, pass: true, sni: false, skip: false,
  },
  dm: {
    modes: [["default", "default"], ["0", "0 不加密"], ["1", "1 SSL 加密"], ["2", "2 证书签名校验"], ["4", "4 仅证书认证"]],
    ca: false, cert: true, key: true, pass: false, sni: false, skip: false,
  },
  mssql: {
    modes: [["default", "default"], ["disable", "disable"], ["required", "required"], ["strict", "strict (TDS 8.0)"]],
    ca: true, cert: false, key: false, pass: false, sni: false, skip: true, caLabel: "cert.hostcert",
  },
};
const CERT_KEYS_BY_FAMILY = {
  mysql: ["sslCert", "sslKey", "sslCA", "sslServerName", "sslSkipVerify", "tls"],
  pg: ["sslmode", "sslrootcert", "sslcert", "sslkey", "sslpassword"],
  dm: ["sslEncrypt", "sslCertPath", "sslKeyPath", "sslFilesPath"],
  mssql: ["encrypt", "trustservercertificate", "certificate"],
};

function stripKeys(family, params) {
  const keys = CERT_KEYS_BY_FAMILY[family];
  if (!keys) return params;
  let s = String(params || "");
  for (const k of keys) {
    const re = new RegExp("(^|&)" + k.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "=[^&]*", "gi");
    while (re.test(s)) s = s.replace(re, "$1");
  }
  return s.replace(/&&+/g, "&").replace(/^&+|&+$/g, "").trim();
}

function readCertForm() {
  return {
    mode: $("#certMode").value,
    ca: $("#certCA").value.trim(),
    cert: $("#certFile").value.trim(),
    key: $("#certKey").value.trim(),
    pass: $("#certKeyPass").value,
    sni: $("#certSNI").value.trim(),
    skip: $("#certSkip").value === "true",
  };
}

// JS 版 params 解析（"a=1&b=2" → 对象；供证书表单回显用）
function parseParams(s) {
  const out = {};
  s = String(s || "").trim();
  if (s.startsWith("?")) s = s.slice(1);
  for (const kv of s.split("&")) {
    if (!kv) continue;
    const i = kv.indexOf("=");
    if (i < 0) out[kv] = "";
    else out[kv.slice(0, i)] = kv.slice(i + 1);
  }
  return out;
}

function composeCertParams(family, st) {
  if (!CERT_SPECS[family] || st.mode === "" || st.mode === "default") return "";
  const kv = (k, v) => k + "=" + v;
  switch (family) {
    case "mysql":
      if (st.mode === "client") {
        let p = kv("sslCert", st.cert) + "&" + kv("sslKey", st.key);
        if (st.ca) p += "&" + kv("sslCA", st.ca);
        if (st.sni) p += "&" + kv("sslServerName", st.sni);
        if (st.skip) p += "&" + kv("sslSkipVerify", "true");
        return p;
      }
      if (st.mode === "preferred") return kv("tls", "preferred");
      if (st.mode === "required") return kv("tls", "true");
      if (st.mode === "skipverify") return kv("tls", "skip-verify");
      return "";
    case "pg": {
      let p = kv("sslmode", st.mode);
      if (st.ca) p += "&" + kv("sslrootcert", st.ca);
      if (st.cert) p += "&" + kv("sslcert", st.cert);
      if (st.key) p += "&" + kv("sslkey", st.key);
      if (st.pass) p += "&" + kv("sslpassword", st.pass);
      return p;
    }
    case "dm": {
      let p = kv("sslEncrypt", st.mode);
      if (st.cert) p += "&" + kv("sslCertPath", st.cert);
      if (st.key) p += "&" + kv("sslKeyPath", st.key);
      return p;
    }
    case "mssql": {
      let p = kv("encrypt", st.mode);
      if (st.skip) p += "&" + kv("trustservercertificate", "true");
      if (st.ca) p += "&" + kv("certificate", st.ca);
      return p;
    }
  }
  return "";
}

function fillCertForm(family, params) {
  const spec = CERT_SPECS[family];
  if (!spec) return;
  const m = parseParams(params || "");
  const g = (k) => m[k] || "";
  let mode = "default";
  if (family === "mysql") {
    if (g("sslCert")) mode = "client";
    else if (g("tls") === "preferred") mode = "preferred";
    else if (g("tls") === "true") mode = "required";
    else if (g("tls") === "skip-verify") mode = "skipverify";
  } else if (family === "pg") {
    if (g("sslmode")) mode = g("sslmode");
  } else if (family === "dm") {
    if (g("sslEncrypt")) mode = g("sslEncrypt");
  } else if (family === "mssql") {
    if (g("encrypt")) mode = g("encrypt");
  }
  $("#certMode").value = spec.modes.some((x) => x[0] === mode) ? mode : "default";
  $("#certCA").value = family === "mssql" ? g("certificate") : (family === "pg" ? g("sslrootcert") : g("sslCA"));
  $("#certFile").value = family === "dm" ? g("sslCertPath") : g("sslcert") || g("sslCert");
  $("#certKey").value = family === "dm" ? g("sslKeyPath") : g("sslkey") || g("sslKey");
  $("#certKeyPass").value = g("sslpassword");
  $("#certSNI").value = g("sslServerName");
  $("#certSkip").value = (family === "mssql" ? g("trustservercertificate") : g("sslSkipVerify")).toLowerCase() === "true" ? "true" : "";
  updateCertHint();
}

function applyCertSpec(family) {
  const spec = CERT_SPECS[family];
  $("#connTabs").style.display = spec ? "" : "none";
  if (!spec) {
    $("#connTabBasic").style.display = "";
    $("#connTabCert").style.display = "none";
    return;
  }
  // 模式选项
  $("#certMode").innerHTML = spec.modes.map(([v, label]) =>
    `<option value="${esc(v)}">${esc(v === "default" ? t("cert.default") : label)}</option>`).join("");
  // 字段可见性
  $("#lbCertCA").style.display = spec.ca ? "" : "none";
  $("#certCALabel").textContent = t(spec.caLabel || "cert.ca");
  $("#lbCertFile").style.display = spec.cert ? "" : "none";
  $("#lbCertKey").style.display = spec.key ? "" : "none";
  $("#lbCertKeyPass").style.display = spec.pass ? "" : "none";
  $("#lbCertSNI").style.display = spec.sni ? "" : "none";
  $("#lbCertSkip").style.display = spec.skip ? "" : "none";
  // 模式相关 hint（随选择联动）
  S._certFamily = family;
  $("#certMode").onchange = updateCertHint;
  updateCertHint();
}

// 各 TLS 模式的场景说明与配置要求（随界面语言切换）
const MODE_HINTS = {
  zh: {
    mysql: {
      default: "跟随驱动默认：不额外配置 TLS，走普通连接。",
      preferred: "场景：过渡期。先尝试 TLS，服务器不支持则回退明文。配置要求：无（存在降级/中间人风险，不建议长期使用）。",
      required: "场景：强制加密并校验服务器身份。配置要求：服务端已开启 TLS（have_ssl=YES）；服务器为自签证书时，需在「CA 证书」提供签发其证书的 CA 文件。",
      skipverify: "场景：强制加密但不校验服务器身份。配置要求：无（加密有效，但可被中间人冒充，仅建议测试环境）。",
      client: "场景：双向证书（mTLS），配合服务端 REQUIRE X509/SUBJECT/ISSUER 可实现证书登录。配置要求：客户端证书+私钥必填；「CA 证书」用于校验服务器；「服务器名称」须与服务器证书 CN/SAN 一致。",
    },
    pg: {
      default: "跟随驱动默认（未设置 sslmode 时驱动按 prefer 处理：尝试 TLS，失败回退明文）。",
      disable: "场景：关闭 TLS，明文连接。配置要求：无（仅限可信内网）。",
      prefer: "场景：优先 TLS，服务器不支持则回退明文。配置要求：无（存在降级风险）。",
      require: "场景：强制加密，但不校验服务器证书。配置要求：服务端 ssl=on（中间人风险，生产建议 verify-ca）。",
      "verify-ca": "场景：强制加密并校验服务器证书是否由可信 CA 签发（不校验主机名）。配置要求：「CA 证书」必填（sslrootcert）。",
      "verify-full": "场景：最严格——校验 CA 且校验主机名，防中间人。配置要求：「CA 证书」必填，且连接主机名须与证书 CN/SAN 一致；配合 pg_hba 的 cert 认证 + 客户端证书可免密登录。",
    },
    dm: {
      default: "跟随驱动默认（不启用 SSL）。",
      "0": "场景：不加密，明文连接。配置要求：无。",
      "1": "场景：SSL 加密通信。配置要求：提供客户端证书+私钥；服务端 dm.ini 已配置 SSL（证书/密钥）。",
      "2": "场景：证书签名校验（对通信报文签名）。配置要求：提供签名证书文件；服务端已开启相应校验。",
      "4": "场景：仅证书认证（校验服务器身份）。配置要求：提供客户端证书+私钥；具体语义以达梦安全手册为准。",
    },
    mssql: {
      default: "跟随驱动默认（新版本驱动默认强制加密并校验证书）。",
      disable: "场景：关闭加密，明文传输（含登录口令，不推荐）。配置要求：无。",
      required: "场景：强制加密。配置要求：默认校验服务器证书——自签证书可勾选「跳过证书校验」，或在「服务器证书」提供服务器证书文件。",
      strict: "场景：TDS 8.0 严格加密（SQL Server 2022+），TLS 先于 TDS 握手，等同 HTTPS 模式。配置要求：服务器证书已配置在 SQL Server 上；客户端需「服务器证书」或勾选跳过校验。",
    },
  },
  en: {
    mysql: {
      default: "Driver default: plain connection unless TLS is configured.",
      preferred: "Use case: transition period. Try TLS first, fall back to plaintext. Requirement: none (downgrade/MITM risk — not recommended long-term).",
      required: "Use case: force encryption with server verification. Requirement: server TLS enabled (have_ssl=YES); for self-signed servers provide its CA in the CA field.",
      skipverify: "Use case: force encryption without verifying the server. Requirement: none (encrypted but spoofable — testing only).",
      client: "Use case: mutual TLS; with server-side REQUIRE X509/SUBJECT/ISSUER enables certificate login. Requirement: client cert + key mandatory; CA verifies the server; server name must match cert CN/SAN.",
    },
    pg: {
      default: "Driver default (no sslmode set → prefer: try TLS, fall back to plaintext).",
      disable: "Use case: no TLS, plaintext. Requirement: none (trusted LAN only).",
      prefer: "Use case: TLS first, fall back to plaintext. Requirement: none (downgrade risk).",
      require: "Use case: force TLS without verifying the server certificate. Requirement: server ssl=on (MITM risk — prefer verify-ca in production).",
      "verify-ca": "Use case: force TLS and verify the server cert is signed by a trusted CA (hostname not checked). Requirement: CA certificate mandatory (sslrootcert).",
      "verify-full": "Use case: strictest — verify CA chain and hostname. Requirement: CA certificate mandatory, hostname must match cert CN/SAN; with pg_hba cert auth + client cert enables passwordless login.",
    },
    dm: {
      default: "Driver default (SSL off).",
      "0": "Use case: no encryption, plaintext. Requirement: none.",
      "1": "Use case: SSL encrypted communication. Requirement: client cert + key; server dm.ini SSL configured.",
      "2": "Use case: packet signature verification. Requirement: signature certificate files; server-side check enabled.",
      "4": "Use case: certificate-only authentication (verify server). Requirement: client cert + key; see DM security manual for details.",
    },
    mssql: {
      default: "Driver default (recent drivers force encryption with certificate verification).",
      disable: "Use case: no encryption — plaintext including passwords (not recommended). Requirement: none.",
      required: "Use case: force encryption. Requirement: server certificate verified by default — tick skip-verify for self-signed, or provide the server certificate file.",
      strict: "Use case: TDS 8.0 strict encryption (SQL Server 2022+) — TLS before TDS handshake, HTTPS-like. Requirement: server cert installed on SQL Server; client needs the server certificate or skip-verify.",
    },
  },
};

function updateCertHint() {
  const family = S._certFamily;
  const mode = $("#certMode") ? $("#certMode").value : "";
  const bank = MODE_HINTS[LANG] || MODE_HINTS.zh;
  const hint = bank[family] && bank[family][mode];
  $("#certHint").textContent = hint || "";
}

function comboDisplay(ti) {
  $("#fTypeIcon").textContent = ti.icon;
  $("#fTypeName").textContent = ti.name;
}
function buildComboList(filter) {
  const q = (filter || "").trim().toLowerCase();
  const groups = {};
  for (const ti of S.types) {
    if (q && !(ti.name + ti.type + (ti.vendor || "") + (ti.note || "")).toLowerCase().includes(q)) continue;
    (groups[ti.vendor || ""] = groups[ti.vendor || ""] || []).push(ti);
  }
  let html = "";
  for (const v of Object.keys(groups).sort()) {
    if (v) html += `<div class="combo-vendor">${esc(v.toUpperCase())}</div>`;
    for (const ti of groups[v]) {
      html += `<div class="combo-item" data-type="${esc(ti.type)}">
        <span class="ci-icon">${esc(ti.icon)}</span>
        <span class="ci-name">${esc(ti.name)}</span>
        <span class="ci-note">${esc(ti.note || (ti.cluster ? "cluster" : ""))}</span></div>`;
    }
  }
  if (!html) html = `<div class="empty" style="padding:18px">—</div>`;
  $("#fTypeList").innerHTML = html;
  $("#fTypeList").querySelectorAll(".combo-item").forEach((el) => {
    el.onclick = (e) => {
      e.preventDefault();
      $("#fType").value = el.dataset.type;
      comboDisplay(typeOf(el.dataset.type));
      closeCombo();
      onTypeChange();
    };
  });
}
function openCombo() { $("#fTypePanel").style.display = ""; $("#typeCombo").classList.add("open"); $("#fTypeSearch").value = ""; buildComboList(""); $("#fTypeSearch").focus(); }
function closeCombo() { $("#fTypePanel").style.display = "none"; $("#typeCombo").classList.remove("open"); }

function onTypeChange() {
  const ti = typeOf($("#fType").value);
  $("#fPort").value = ti.port || "";
  const isSqlite = ti.family === "sqlite";
  $("#portWrap").style.display = isSqlite ? "none" : "";
  $("#sqliteBtns").style.display = isSqlite ? "flex" : "none";
  $("#lbPass").style.display = isSqlite ? "none" : "";
  $("#hostLabelText").textContent = isSqlite ? t("dlg.file") : t("dlg.host");
  if (isSqlite && !$("#fHost").value) $("#fHost").value = "";
  $("#dbLabelText").textContent = LANG === "zh" ? ti.dbLabel : (t("dlg.database") + (ti.dbLabel ? ` · ${ti.dbLabel}` : ""));
  $("#lbUser").style.display = ti.userLabel ? "" : "none";
  $("#userLabelText").textContent = LANG === "zh" ? ti.userLabel : (t("dlg.user") + (ti.userLabel ? ` · ${ti.userLabel}` : ""));
  // Oracle 系显示"连接身份"（SYSDBA/SYSOPER 特权连接）
  $("#lbRole").style.display = ti.family === "oracle" ? "" : "none";
  // 证书页（mysql/pg/mssql/dm 族）
  applyCertSpec(CERT_SPECS[ti.family] ? ti.family : null);
  // 默认回到基本连接页
  switchConnTab("basic");
  $("#typeHint").textContent = [ti.vendor, ti.note].filter(Boolean).join(" · ") || "";
}

function switchConnTab(which) {
  document.querySelectorAll("#connTabs .ctab").forEach((el) =>
    el.classList.toggle("active", el.dataset.ctab === which));
  $("#connTabBasic").style.display = which === "basic" ? "" : "none";
  $("#connTabCert").style.display = which === "cert" ? "" : "none";
}

function openConnDialog(id) {
  const A = API();
  window._editId = id || null;
  // 隐藏的基础参数（证书/连接身份等 UI 组装会保留并重组；高级模式下可在输入框手动编辑）
  const baseConn = id ? S.conns.find((x) => x.id === id) : null;
  S._baseParams = baseConn ? (baseConn.params || "") : "";
  $("#groupList").innerHTML = [...new Set(S.conns.map((c) => c.group).filter(Boolean))]
    .map((g) => `<option value="${esc(g)}">`).join("");
  $("#testResult").textContent = "";
  $("#testResult").className = "";
  closeCombo();
  switchConnTab("basic");
  if (id) {
    const c = S.conns.find((x) => x.id === id);
    $("#dlgConnTitle").textContent = t("dlg.editTitle");
    $("#fType").value = c.type; comboDisplay(typeOf(c.type));
    $("#fName").value = c.name; $("#fGroup").value = c.group || "";
    $("#fHost").value = c.host; $("#fPort").value = c.port; $("#fDatabase").value = c.database || "";
    $("#fUser").value = c.user || ""; $("#fPassword").value = "";
    const role = roleFromParams(c.params);
    $("#fRole").value = role === "SYSDBA" || role === "SYSOPER" ? role.toLowerCase() : "normal";
    $("#fTimeout").value = c.timeout || 5;
    window._rawParams = c.params;
  } else {
    $("#dlgConnTitle").textContent = t("dlg.newTitle");
    const first = S.types[0] || typeOf("");
    $("#fType").value = first.type; comboDisplay(first);
    $("#fName").value = ""; $("#fGroup").value = "";
    $("#fHost").value = ""; $("#fDatabase").value = ""; $("#fUser").value = "";
    $("#fPassword").value = ""; $("#fTimeout").value = 5;
    window._rawParams = "";
  }
  onTypeChange();
  // 证书表单从“原始参数”解析（fParams 已剥离 UI 管理键，避免回显丢失）
  if (CERT_SPECS[typeOf($("#fType").value).family]) {
    fillCertForm(typeOf($("#fType").value).family, window._rawParams);
  }
  $("#dlgConn").showModal();
}

function collectConnForm() {
  const ti = typeOf($("#fType").value);
  const c = { id: window._editId || "" };
  c.type = $("#fType").value;
  c.name = $("#fName").value.trim() || $("#fHost").value.trim();
  c.group = $("#fGroup").value.trim();
  c.host = $("#fHost").value.trim();
  c.port = parseInt($("#fPort").value) || ti.port;
  c.database = $("#fDatabase").value.trim();
  c.user = $("#fUser").value.trim();
  c.password = $("#fPassword").value;
  // 参数组装：基础参数（来自已存连接，静默保留）剥离 UI 管理键后重组
  let params = stripAuthType(stripKeys(ti.family, S._baseParams || ""));
  if (CERT_SPECS[ti.family]) {
    const cp = composeCertParams(ti.family, readCertForm());
    if (cp) params = params ? cp + "&" + params : cp;
  }
  if (ti.family === "oracle") {
    const role = $("#fRole").value;
    if (role === "sysdba" || role === "sysoper") {
      params = params ? params + "&AUTH TYPE=" + role.toUpperCase() : "AUTH TYPE=" + role.toUpperCase();
    }
  }
  c.params = params;
  c.timeout = parseInt($("#fTimeout").value) || 5;
  return c;
}

async function testConn() {
  const A = API(); if (!A) return;
  const c = collectConnForm();
  const el = $("#testResult");
  el.className = ""; el.textContent = LANG === "zh" ? "测试中…" : "Testing…";
  const r = await A.TestConnection(c);
  if (r.ok) {
    el.className = "ok";
    el.textContent = `✓ ${LANG === "zh" ? "连接正常" : "OK"}　TCP ${r.tcpMs}ms · SQL ${r.sqlMs}ms`;
  } else {
    el.className = "err";
    el.textContent = "✗ " + (r.error || "failed");
    showError(t("err.title"), {
      meta: `${c.name || "-"} · ${typeOf(c.type).name} · ${c.host}:${c.port}${c.database ? "/" + c.database : ""}`,
      kind: r.errorKind, msg: r.error, stage: r.stage,
    });
  }
}

async function saveConn() {
  const A = API(); if (!A) return;
  const editId = window._editId;
  const c = collectConnForm();
  try {
    await A.SaveConnection(c);
    window._editId = null;
    $("#dlgConn").close();
    toast(t("toast.saved"), "ok");
    if (editId) delete S.detailCache[editId]; // 配置已变，缓存作废
    await reload();
    if (editId && S.sel === editId) await showDetail(editId, { refresh: true });
  } catch (e) { toast(t("toast.saveFail") + e, "err"); }
}

/* ---------------- 设置 ---------------- */
// AI_PROVIDERS 内置大模型 → 图标 / 名称 / 默认 Base URL / 默认模型
// （选择提供商时自动预填模型名；local/custom 支持空 Key）
const AI_PROVIDERS = {
  zhipu:    { name: "智谱 GLM", icon: "🧠", base: "https://open.bigmodel.cn/api/paas/v4", model: "glm-4-flash" },
  deepseek: { name: "DeepSeek", icon: "🐳", base: "https://api.deepseek.com", model: "deepseek-chat" },
  qwen:     { name: "通义千问 Qwen", icon: "☁️", base: "https://dashscope.aliyuncs.com/compatible-mode/v1", model: "qwen-plus" },
  moonshot: { name: "Kimi（月之暗面）", icon: "🌙", base: "https://api.moonshot.cn/v1", model: "moonshot-v1-8k" },
  volces:   { name: "豆包（火山方舟）", icon: "🌋", base: "https://ark.cn-beijing.volces.com/api/v3", model: "doubao-1.5-pro-32k" },
  qianfan:  { name: "百度千帆 ERNIE", icon: "🔵", base: "https://qianfan.baidubce.com/v2", model: "ernie-4.0-8k-latest" },
  local:    { name: "", icon: "🏠", base: "http://localhost:11434/v1", model: "qwen2.5:7b" },
  custom:   { name: "", icon: "🛠️", base: "", model: "" },
};

function aiProvName(v) {
  if (v === "local") return t("ai.local");
  if (v === "custom") return t("ai.custom");
  return (AI_PROVIDERS[v] || {}).name || v;
}

// buildAICombo 渲染 AI 提供商下拉清单（带图标）。
function buildAICombo() {
  const list = $("#aiComboList");
  list.innerHTML = Object.keys(AI_PROVIDERS).map((k) =>
    `<div class="combo-item" data-v="${k}">
      <span class="ci-icon">${AI_PROVIDERS[k].icon}</span>
      <span class="ci-name">${esc(aiProvName(k))}</span>
    </div>`).join("");
}
// 事件委托：列表重建后依然有效；先收起面板再应用选择（保证关闭）
function bindAIComboList() {
  $("#aiComboList").addEventListener("click", (e) => {
    const item = e.target.closest(".combo-item");
    if (!item) return;
    e.preventDefault();
    e.stopPropagation();
    closeAICombo();
    try { setAIProvider(item.dataset.v); } catch { }
  });
}
function setAIProvider(v) {
  const p = AI_PROVIDERS[v]; if (!p) return;
  $("#aiProvider").value = v;
  $("#aiProvIcon").textContent = p.icon;
  $("#aiProvName").textContent = aiProvName(v);
  $("#aiLbBase").style.display = (v === "custom" || v === "local") ? "" : "none";
  $("#aiModel").value = p.model;
  $("#aiBase").value = p.base;
}
function openAICombo() {
  buildAICombo();
  $("#aiComboPanel").style.display = "";
  $("#aiCombo").classList.add("open");
}
function closeAICombo() {
  $("#aiComboPanel").style.display = "none";
  $("#aiCombo").classList.remove("open");
}

async function openSettings() {
  const A = API(); if (!A) return;
  S.settings = await A.GetSettings();
  $("#sAuto").checked = S.settings.autoPing;
  $("#sInterval").value = S.settings.intervalSec || 60;
  $("#sTimeout").value = S.settings.pingTimeout || 5;
  // AI 配置
  try {
    const ai = await A.GetAIConfig();
    const prov = AI_PROVIDERS[ai.provider] ? ai.provider : "zhipu";
    setAIProvider(prov);
    if (ai.model) $("#aiModel").value = ai.model;
    if (ai.baseUrl) $("#aiBase").value = ai.baseUrl;
    $("#aiKey").value = "";
    const needKey = prov !== "local" && prov !== "custom";
    $("#aiKey").placeholder = needKey
      ? (ai.hasKey ? (LANG === "zh" ? "已配置（输入覆盖）" : "Configured (type to replace)") : (LANG === "zh" ? "输入 API Key" : "Enter API Key"))
      : (LANG === "zh" ? "本地模型通常无需 Key" : "Usually not required for local models");
  } catch { }
  try { $("#cfgDir").textContent = (LANG === "zh" ? `版本 v${S.version || "?"} · 配置目录：` : `Version v${S.version || "?"} · Config dir: `) + (await A.GetConfigDir()); } catch { }
  $("#dlgSettings").showModal();
}

async function saveSettings() {
  const A = API(); if (!A) return;
  await A.SaveSettings({
    autoPing: $("#sAuto").checked,
    advancedMode: S.settings ? !!S.settings.advancedMode : false,
    intervalSec: parseInt($("#sInterval").value) || 60,
    pingTimeout: parseInt($("#sTimeout").value) || 5,
  });
  // AI 配置（key 留空 = 保持原值；local/custom 使用可编辑的 BaseURL）
  const prov = $("#aiProvider").value;
  const base = (prov === "custom" || prov === "local") ? $("#aiBase").value.trim() : AI_PROVIDERS[prov].base;
  let model = $("#aiModel").value.trim() || AI_PROVIDERS[prov].model;
  try {
    await A.SaveAIConfig(prov, model, base, $("#aiKey").value.trim());
  } catch (e) { toast(t("toast.saveFail") + e, "err"); }
  $("#dlgSettings").close();
  toast(t("toast.setSaved"), "ok");
  await reload();
}

/* ---------------- 事件绑定 ---------------- */
function bind() {
  $("#btnAdd").onclick = () => openConnDialog(null);
  $("#btnPingAll").onclick = pingAll;
  $("#btnSettings").onclick = openSettings;
  $("#autoState").onclick = openSettings;
  $("#btnSaveSettings").onclick = saveSettings;
  $("#btnSettingsCancel").onclick = () => $("#dlgSettings").close();
  $("#btnExportAll").onclick = exportAll;
  $("#btnAdvanced").onclick = async () => {
    const A = API(); if (!A || !S.settings) return;
    S.settings.advancedMode = !S.settings.advancedMode;
    try { await A.SaveSettings(S.settings); } catch (e) { toast(String(e), "err"); return; }
    updateAdvTabs();
    toast(S.settings.advancedMode
      ? (LANG === "zh" ? "高级模式已开启（SQL 控制台可用）" : "Advanced mode ON (SQL console enabled)")
      : (LANG === "zh" ? "高级模式已关闭" : "Advanced mode OFF"), "ok");
  };
  $("#btnLang").onclick = () => {
    LANG = LANG === "zh" ? "en" : "zh";
    applyLang(); renderAll();
    if (S.sel) { $("#dMeta").textContent = detailMeta(); loadTab(S.tab, true); }
  };
  $("#btnTheme").onclick = () => {
    THEME = THEME === "dark" ? "light" : "dark";
    applyTheme();
    if (S.sel) loadTab(S.tab, true); // 图表配色随主题重绘
  };

  // 类型下拉
  $("#fTypeBtn").onclick = (e) => {
    e.stopPropagation();
    $("#fTypePanel").style.display === "none" ? openCombo() : closeCombo();
  };
  $("#fTypeSearch").onclick = (e) => e.stopPropagation();
  $("#fTypeSearch").oninput = (e) => buildComboList(e.target.value);
  $("#fTypeSearch").onkeydown = (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      const first = document.querySelector("#fTypeList .combo-item");
      if (first) first.click(); // 复用条目点击：选择并关闭下拉
    }
    if (e.key === "Escape") closeCombo();
  };
  document.addEventListener("click", (e) => {
    if (!$("#typeCombo").contains(e.target)) closeCombo();
  });
  // SQLite 文件选择/创建
  $("#btnPickOpen").onclick = async (e) => {
    e.stopPropagation();
    const A = API(); if (!A) return;
    try { const p = await A.PickSQLiteFile(false); if (p) { $("#fHost").value = p; $("#fName").value = $("#fName").value || p.split(/[\\/]/).pop(); } } catch {}
  };
  $("#btnPickCreate").onclick = async (e) => {
    e.stopPropagation();
    const A = API(); if (!A) return;
    try { const p = await A.PickSQLiteFile(true); if (p) { $("#fHost").value = p; $("#fName").value = $("#fName").value || p.split(/[\\/]/).pop(); } } catch {}
  };
  // 证书/私钥文件选择
  $("#btnPickCA").onclick = async () => {
    const A = API(); if (!A) return;
    try { const p = await A.PickCertFile("ca"); if (p) $("#certCA").value = p; } catch {}
  };
  $("#btnPickCert").onclick = async () => {
    const A = API(); if (!A) return;
    try { const p = await A.PickCertFile("cert"); if (p) $("#certFile").value = p; } catch {}
  };
  $("#btnPickKey").onclick = async () => {
    const A = API(); if (!A) return;
    try { const p = await A.PickCertFile("key"); if (p) $("#certKey").value = p; } catch {}
  };

  // 对话框内 Tab：基本连接 / TLS 证书
  document.querySelectorAll("#connTabs .ctab").forEach((el) => {
    el.onclick = () => switchConnTab(el.dataset.ctab);
  });
  $("#btnPingDlgClose").onclick = () => $("#dlgPing").close();

  // ---------- 测速（iperf3） ----------
  $("#btnIperf").onclick = async () => {
    $("#ipResult").innerHTML = "";
    $("#ipStatus").textContent = "";
    $("#dlgIperf").showModal();
    await loadIperfHistory();
  };
  $("#ipBtnClose").onclick = () => { if (!S.iperfBusy) $("#dlgIperf").close(); };
  $("#ipBtnGo").onclick = iperfStart;

  // ---------- 主机巡检模式 ----------
  $("#modeDb").onclick = () => switchMode("db");
  $("#modeHost").onclick = () => switchMode("host");
  $("#btnAddHost").onclick = () => openHostDialog(null);
  $("#hFAuth").onchange = applyHostAuthVisibility;
  $("#hBtnPickKey").onclick = async () => {
    const A = API(); if (!A) return;
    try { const p = await A.PickCertFile("key"); if (p) $("#hFKeyPath").value = p; } catch {}
  };
  $("#hBtnTest").onclick = testSSHConn;
  $("#hBtnSave").onclick = saveHost;
  $("#hBtnCancel").onclick = () => { window._editHostId = null; $("#dlgHost").close(); };
  $("#btnHostBack").onclick = () => { $("#hostDetail").style.display = "none"; $("#hostDash").style.display = ""; renderHostDash(); };
  $("#btnInspect").onclick = () => inspectHost(S.hostSel);
  $("#btnHostAI").onclick = () => aiAnalyzeHost(S.hostSel);
  $("#btnHostExport").onclick = () => exportHostReport(S.hostSel);
  $("#btnHostEdit").onclick = () => openHostDialog(S.hostSel);
  $("#btnHostDelete").onclick = () => deleteHost(S.hostSel);
  // ---------- 类 nmap 端口扫描 ----------
  $("#btnNmapScan").onclick = async () => {
    $("#nmapResult").innerHTML = "";
    $("#nmapTarget").value = ""; $("#nmapPortSpec").value = "";
    $("#dlgNmap").showModal();
    await loadNmapHistory();
  };
  $("#nmapBtnClose").onclick = () => $("#dlgNmap").close();
  $("#nmapBtnGo").onclick = async () => {
    const A = API(); if (!A) return;
    const target = $("#nmapTarget").value.trim();
    const spec = $("#nmapPortSpec").value.trim();
    const box = $("#nmapResult");
    if (!target) { toast(LANG === "zh" ? "请输入目标主机" : "Enter target host", "err"); return; }
    if (!spec) { toast(LANG === "zh" ? "请输入端口范围" : "Enter port range", "err"); return; }
    S.nmapBusy = true;
    box.innerHTML = `<div class="loading"><span class="spin"></span>${t("nmap.scanning")}</div>` + progressBarHTML(0, 0);
    try {
      const r = await A.NmapScan(target, spec);
      renderNmapResult(r);
      await loadNmapHistory();
    } catch (e) {
      box.innerHTML = warnBoxHTML(String(e));
    } finally {
      S.nmapBusy = false;
    }
  };

  // renderNmapResult 渲染一次扫描结果（实时扫描与历史记录共用）。
  function renderNmapResult(r) {
    const box = $("#nmapResult");
    const ports = r.ports || [];
    const open = ports.filter((p) => p.state === "开放");
    const fmt = (s) => s || "-";
    const osLine = r.osGuess && r.osGuess !== "未知"
      ? `<div style="margin-bottom:8px;font-size:13px"><b>🖥 ${t("nmap.os")}:</b> ${esc(r.osGuess)} <span class="dim" style="font-size:11.5px">(${t("nmap.osBasis")}: ${esc(r.osBasis || "-")})</span></div>`
      : `<div class="dim" style="margin-bottom:8px;font-size:12px">${t("nmap.osNone")}</div>`;
    box.innerHTML =
      `<div class="dim" style="font-size:11.5px;margin-bottom:6px">${esc(r.target || "")} · ${t("nmap.done", { ok: open.length, n: ports.length })}${r.time ? " · " + new Date(r.time).toLocaleString() : ""}</div>` +
      osLine +
      (open.length
        ? `<table class="data"><tr><th>${t("host.port")}</th><th>${t("host.state")}</th><th>${t("host.service")}</th></tr>` +
          open.map((p) => `<tr><td>${esc(p.port)}</td><td><span style="color:var(--ok)">● ${esc(p.state)}</span></td><td>${esc(fmt(p.service))}</td></tr>`).join("") + `</table>`
        : `<div class="empty" style="padding:10px;font-size:12.5px">${LANG === "zh" ? "指定范围内无开放端口" : "No open ports in the specified range"}</div>`);
  }

  // loadNmapHistory 加载并渲染扫描记录（点击回看 / 删除 / 导出）。
  async function loadNmapHistory() {
    const A = API(); if (!A) return;
    const box = $("#nmapHistory");
    if (!box) return;
    try {
      const items = (await A.GetNmapHistory()) || [];
      if (!items.length) {
        box.innerHTML = `<div class="dim" style="font-size:11.5px;margin-top:8px">${t("nmap.noHistory")}</div>`;
        return;
      }
      box.innerHTML = `<div style="display:flex;align-items:center;gap:8px;margin:10px 0 2px">
          <span class="dim" style="font-size:12px;flex:1">${t("nmap.history")}</span>
          <button class="btn small" id="nmapHistExport">⇓ ${t("spd.export")}</button>
        </div>` +
        items.map((it, i) => `
          <div class="nh-item" data-i="${i}">
            <span class="dim" style="font-size:11px;flex:none">${new Date(it.time).toLocaleString()}</span>
            <b style="flex:none">${esc(it.target)}</b>
            <span class="dim" style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">🖥 ${esc(it.osGuess || "-")}</span>
            <span style="flex:none;color:var(--ok)">${it.openCount}/${it.total}</span>
            <button class="mini nh-del" data-t="${it.time}" title="${t("spd.del")}">✕</button>
          </div>`).join("");
      $("#nmapHistExport").onclick = async () => {
        try {
          const p = await A.ExportNmapHistory();
          if (p) toast(t("toast.exportOk") + p, "ok");
        } catch (e) { if (String(e).indexOf("cancel") < 0) toast(t("toast.exportFail") + e, "err"); }
      };
      box.querySelectorAll(".nh-item").forEach((el) => {
        el.onclick = (e) => {
          if (e.target.closest(".nh-del")) return;
          const it = items[+el.dataset.i]; if (!it) return;
          // 回填原目标与端口范围
          $("#nmapTarget").value = it.target || "";
          $("#nmapPortSpec").value = it.portSpec || "";
          renderNmapResult(it);
        };
      });
      box.querySelectorAll(".nh-del").forEach((el) => {
        el.onclick = async (e) => {
          e.stopPropagation();
          try {
            await A.DeleteNmapHistoryEntry(Number(el.dataset.t));
            await loadNmapHistory();
            toast(t("spd.deleted"), "ok");
          } catch (err) { toast(String(err), "err"); }
        };
      });
    } catch { box.innerHTML = ""; }
  }

  $("#searchBox").oninput = (e) => { S.search = e.target.value; renderCards(); };
  $("#filterType").onchange = (e) => { S.fType = e.target.value; renderCards(); };
  $("#filterGroup").onchange = (e) => { S.fGroup = e.target.value; renderCards(); };

  $("#btnBack").onclick = backToDash;
  $("#btnEdit").onclick = () => openConnDialog(S.sel);
  $("#btnAI").onclick = () => aiAnalyzeDB();
  $("#btnTest").onclick = testConn;
  $("#btnSaveConn").onclick = saveConn;
  $("#btnCancelConn").onclick = () => { window._editId = null; $("#dlgConn").close(); };
  $("#btnErrClose").onclick = () => $("#dlgError").close();
  $("#btnErrCopy").onclick = async () => {
    try { await copyText(_lastErrText); toast(t("err.copied"), "ok"); }
    catch (e) { toast(String(e), "err"); }
  };

  $("#btnPingOne").onclick = async () => {
    const A = API(); if (!A || !S.sel) return;
    const btn = $("#btnPingOne"); btn.disabled = true;
    try {
      const r = await A.PingOne(S.sel);
      S.latest[S.sel] = r;
      if (r.ok) {
        toast(t("toast.pingOk", { t: r.tcpMs, s: r.sqlMs }), "ok");
      } else {
        const c = S.conns.find((x) => x.id === S.sel) || {};
        showError(t("err.title"), {
          meta: `${c.name || "-"} · ${typeOf(c.type).name} · ${c.host}:${c.port}`,
          kind: r.errorKind, msg: r.error, stage: r.stage,
        });
      }
      updateDetailStatus(); renderSidebar(); renderStats(); renderCards();
      if (S.tab === "history") loadTab("history", true);
    } finally { btn.disabled = false; }
  };
  $("#btnRefresh").onclick = () => { S.detailCache = {}; loadTab(S.tab, true); };
  $("#btnExport").onclick = async () => {
    const A = API(); if (!A || !S.sel) return;
    try {
      // 综合导出：巡检报告 + AI 分析（若已分析）
      const ai = S.dbAIReports[S.sel];
      const p = await A.ExportReport(S.sel, ai ? ai.markdown : "");
      if (p) toast(t("toast.exportOk") + p, "ok");
    } catch (e) { if (String(e).indexOf("cancel") < 0) toast(t("toast.exportFail") + e, "err"); }
  };
  $("#btnDelete").onclick = async () => {
    const A = API(); if (!A || !S.sel) return;
    if (S.deleteArmed !== S.sel) {
      S.deleteArmed = S.sel;
      $("#btnDelete").textContent = t("detail.delConfirm");
      setTimeout(() => { S.deleteArmed = null; $("#btnDelete").textContent = t("detail.del"); }, 2500);
      return;
    }
    await A.DeleteConnection(S.sel);
    toast(t("toast.deleted"), "ok");
    S.deleteArmed = null; $("#btnDelete").textContent = t("detail.del");
    backToDash(); await reload();
  };

  document.querySelectorAll("#tabs .tab").forEach((el) => {
    el.onclick = () => {
      document.querySelectorAll("#tabs .tab").forEach((x) => x.classList.remove("active"));
      el.classList.add("active");
      loadTab(el.dataset.tab);
    };
  });
  // 主机详情 Tab：巡检报告 / AI 分析 / 历史
  document.querySelectorAll("#hostTabs .tab").forEach((el) => {
    el.onclick = () => {
      document.querySelectorAll("#hostTabs .tab").forEach((x) => x.classList.remove("active"));
      el.classList.add("active");
      S.hostView = el.dataset.htab || "report";
      S.hostHistIdx = -1;
      renderHostView();
    };
  });
}

function detailMeta() {
  const c = S.conns.find((x) => x.id === S.sel);
  if (!c) return "";
  const ti = typeOf(c.type);
  return `${ti.name} · ${c.host}:${c.port}${c.database ? " / " + c.database : ""}`;
}

// 全部 Ping：弹窗逐连接动态显示过程（6 并发）
function pingRowHTML(r, i) {
  const ti = typeOf(r.conn.type);
  let st;
  if (r.status === "queued") st = `<span class="dim">${t("pingdlg.queued")}</span>`;
  else if (r.status === "testing") st = `<span class="spin"></span>${t("pingdlg.testing")}`;
  else if (r.status === "ok") st = `<span class="ok">✓</span><span class="dim">TCP ${r.res.tcpMs}ms · SQL ${r.res.sqlMs}ms</span>`;
  else st = `<span class="err">✗</span><span class="dim" title="${esc(r.res ? r.res.error : (r.err || ""))}">${esc((r.res ? r.res.error : r.err || "").slice(0, 46))}</span>`;
  return `<div class="ping-row" id="pingrow-${i}">
    <span class="dot ${r.status === "ok" ? "ok" : r.status === "err" ? "err" : ""}"></span>
    <span>${ti.icon}</span>
    <span class="p-name">${esc(r.conn.name)}</span>
    <span class="p-host">${esc(r.conn.host)}:${esc(r.conn.port)}</span>
    <span class="p-st">${st}</span>
  </div>`;
}

function updatePingRow(rows, i) {
  const el = document.getElementById("pingrow-" + i);
  if (el) el.outerHTML = pingRowHTML(rows[i], i);
}

function updatePingProgress(rows, ok, done) {
  const p = document.getElementById("pingProg");
  if (p) p.textContent = `${done}/${rows.length}`;
  const d = document.getElementById("pingDone");
  if (d && done >= rows.length) d.textContent = t("toast.done", { ok, n: rows.length });
}

async function pingAll() {
  const A = API(); if (!A || !S.conns.length) { toast(LANG === "zh" ? "没有可检测的连接" : "No connections", "err"); return; }
  const btn = $("#btnPingAll");
  btn.disabled = true; btn.textContent = "⏳"; S.pinging = true;
  const rows = S.conns.map((c) => ({ conn: c, status: "queued" }));
  $("#pingRows").innerHTML = rows.map((r, i) => pingRowHTML(r, i)).join("");
  $("#pingProg").textContent = `0/${rows.length}`;
  $("#pingDone").textContent = "";
  $("#dlgPing").showModal();
  S.pinging = true;

  let ok = 0, done = 0, idx = 0;
  async function worker() {
    while (idx < rows.length) {
      const my = idx++;
      const r = rows[my];
      r.status = "testing";
      updatePingRow(rows, my);
      try {
        const res = await A.PingOne(r.conn.id);
        S.latest[r.conn.id] = res;
        r.res = res;
        r.status = res.ok ? "ok" : "err";
        if (res.ok) ok++;
      } catch (e) { r.err = String(e); r.status = "err"; }
      done++;
      updatePingRow(rows, my);
      updatePingProgress(rows, ok, done);
      renderStats(); // 顶部统计实时刷新
    }
  }
  const workers = [];
  for (let i = 0; i < Math.min(6, rows.length); i++) workers.push(worker());
  await Promise.all(workers);
  S.pinging = false;
  toast(t("toast.done", { ok, n: rows.length }), ok === rows.length ? "ok" : "err");
  renderSidebar(); renderStats(); renderCards();
  if (S.sel) updateDetailStatus();
  if (btn) { btn.disabled = false; btn.textContent = "📡"; }
}

async function exportAll() {
  const A = API(); if (!A) return;
  try {
    const p = await A.ExportReportAll();
    if (p) toast(t("toast.exportOk") + p, "ok");
  } catch (e) { if (String(e).indexOf("cancel") < 0) toast(t("toast.exportFail") + e, "err"); }
}

/* ---------------- 启动 ---------------- */
window.addEventListener("resize", () => {
  if (S.topoChart) S.topoChart.resize();
  if (S.histChart) S.histChart.resize();
});

(async function boot() {
  const A = API();
  if (!A) {
    document.body.innerHTML = `<div class="empty"><div class="big">⚠️</div>${LANG === "zh" ? "请在 HostPing 桌面应用中打开本页面（缺少 Wails 绑定）" : "Open this page inside the HostPing desktop app (Wails bindings missing)"}</div>`;
    return;
  }
  window.addEventListener("error", (e) => toast(t("toast.scriptErr") + (e.message || e.type), "err"));
  window.addEventListener("unhandledrejection", (e) => toast(t("toast.asyncErr") + (e.reason || e.type), "err"));
  // 版本号显示（侧边栏 Logo + 设置对话框）
  try {
    S.version = await A.GetVersion();
    const badge = $("#verBadge");
    if (badge) badge.textContent = "v" + S.version;
  } catch { }
  applyTheme();
  applyLang();
  bind();
  try {
    await reload();
  } catch (e) {
    toast(t("toast.loadFail") + e, "err");
  }
  if (window.runtime) {
    window.runtime.EventsOn("ping:update", onPingUpdate);
    // 后端启动期错误（如配置损坏）以对话框呈现
    window.runtime.EventsOn("app:error", (msg) => showError(t("err.title"), { meta: msg, kind: "unknown", msg: String(msg) }));
    // 主机巡检逐节进度（百分比进度条）
    window.runtime.EventsOn("host:progress", (p) => {
      if (!S.hostBusy || !p) return;
      const prog = $("#hostProg"); if (!prog) return;
      prog.innerHTML = `<span class="host-prog"><span class="spin"></span>${esc(t("host.step." + (p.section || "")))}</span>` + progressBarHTML(p.done || 0, p.total || 0);
    });
    // 类 nmap 扫描逐端口进度（百分比进度条）
    window.runtime.EventsOn("nmap:progress", (p) => {
      if (!S.nmapBusy || !p) return;
      const box = $("#nmapResult"); if (!box) return;
      box.innerHTML = `<div class="loading"><span class="spin"></span>${t("nmap.scanning")}</div>` + progressBarHTML(p.done || 0, p.total || 0);
    });
    // 测速逐秒进度
    window.runtime.EventsOn("iperf:progress", (p) => {
      if (!S.iperfBusy || !p) return;
      const box = $("#ipResult"); if (!box) return;
      iperfAppendInterval(p);
    });
  }
  if (!S.conns.length) toast(t("toast.welcome"), "ok");
  // 缺省 tab：主机
  switchMode("host");
  $("#aiComboBtn").onclick = () => {
    const panel = $("#aiComboPanel");
    if (panel.style.display === "none") openAICombo(); else closeAICombo();
  };
  bindAIComboList();
  document.addEventListener("click", (e) => {
    const combo = $("#aiCombo");
    if (combo && !combo.contains(e.target)) closeAICombo();
  });
})();

