package hostinsp

import (
	"fmt"
	"strings"
	"time"

	"dbping/internal/model"
)

// ProgressFn 巡检进度回调（section: 当前节名; done/total: 已完成/总节数）。
type ProgressFn func(section string, done, total int)

const (
	rsMaxProcs    = 20 // rs_/rsa_ 进程采集上限
	openFilesCap  = 50 // 单进程打开文件列表上限
)

// Inspect 对指定主机执行完整巡检（顺序采集，逐节降级为 Warning）。
// 认证凭据直接取自 h.Password（密码认证）或 h.KeyPassphrase（私钥认证）。
func Inspect(h *model.SSHHost, progress ProgressFn) (*model.HostReport, error) {
	password := h.Password
	if h.AuthType == "key" {
		password = h.KeyPassphrase
	}
	cli, err := Connect(h, password, h.KeyPassphrase)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	sections := []string{"sysinfo", "ports", "top", "rs", "syslog", "security", "storcli", "zfs", "lsscsi", "lspci", "reboot"}
	total := len(sections)
	done := 0
	step := func(name string, fn func()) {
		if progress != nil {
			progress(name, done, total)
		}
		fn()
		done++
		if progress != nil {
			progress(name, done, total)
		}
	}

	r := &model.HostReport{
		HostID:      h.ID,
		Name:        h.Name,
		Host:        h.Host,
		Port:        h.Port,
		User:        h.User,
		CollectedAt: time.Now().UnixMilli(),
	}
	cmdTimeout := time.Duration(h.Timeout) * time.Second
	if cmdTimeout <= 0 {
		cmdTimeout = 20 * time.Second
	}
	run := func(cmd string) string {
		out, err := cli.Run(cmd, cmdTimeout)
		if err != nil {
			return ""
		}
		return out
	}

	step("sysinfo", func() { collectSysInfo(cli, r, run) })
	step("ports", func() { collectListenPorts(r, run) })
	step("top", func() { collectTopProcs(r, run) })
	step("rs", func() { collectRSProcs(r, run) })
	step("syslog", func() { r.SysLog = collectSysLog(run) })
	step("security", func() { r.Security = collectSecurity(run) })
	step("storcli", func() { collectStorcli(cli, r, run) })
	step("zfs", func() { collectZFS(cli, r, run) })
	step("lsscsi", func() { collectLsscsi(cli, r, run) })
	step("lspci", func() { collectLspci(cli, r, run) })
	step("reboot", func() { collectReboots(r, run) })

	return r, nil
}

// ---------- 重启记录（last -x reboot，最近 10 次） ----------

func collectReboots(r *model.HostReport, run func(string) string) {
	out := run("last -x reboot 2>/dev/null | head -11")
	weekdays := map[string]bool{"Mon": true, "Tue": true, "Wed": true, "Thu": true, "Fri": true, "Sat": true, "Sun": true}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		fs := strings.Fields(line)
		if len(fs) < 6 || fs[0] != "reboot" {
			continue
		}
		// reboot system boot [kernel] weekday mon date time ...
		startIdx := 4
		if weekdays[fs[3]] {
			startIdx = 3
		}
		if len(fs) < startIdx+4 {
			continue
		}
		rb := model.RebootInfo{Start: strings.Join(fs[startIdx:startIdx+4], " ")}
		rest := fs[startIdx+4:]
		if len(rest) > 0 && rest[0] == "still" {
			rb.End = "-"
			rb.Dur = "still running"
		} else if len(rest) >= 3 && rest[0] == "-" {
			rb.End = rest[1]
			rb.Dur = strings.Trim(rest[2], "()")
		} else {
			rb.End = "-"
			rb.Dur = strings.Join(rest, " ")
		}
		r.Reboots = append(r.Reboots, rb)
		if len(r.Reboots) >= 10 {
			break
		}
	}
	if len(r.Reboots) == 0 {
		warn(r, "未采集到重启记录（last 命令不可用或无记录）")
	}
}

func warn(r *model.HostReport, msg string) {
	r.Warnings = append(r.Warnings, msg)
}

// ---------- 系统软硬件信息 ----------

func collectSysInfo(c *SSHClient, r *model.HostReport, run func(string) string) {
	kv := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			r.SysInfo = append(r.SysInfo, model.KV{Key: k, Value: strings.TrimSpace(v)})
		}
	}
	// 发行版
	osRelease := run(`cat /etc/os-release 2>/dev/null`)
	pretty := pickField(osRelease, "PRETTY_NAME=")
	if pretty == "" && osRelease == "" {
		pretty = run("uname -o 2>/dev/null")
	}
	if pretty != "" {
		r.OSName = strings.Trim(pretty, `"`)
	}
	kv("发行版", strings.Trim(pretty, `"`))
	kv("内核", firstField(run("uname -r")))
	kv("架构", firstField(run("uname -m")))
	kv("主机名", firstLine(run("hostname")))
	// CPU
	cpuModel := firstMatch(run("lscpu 2>/dev/null"), "Model name:")
	if cpuModel == "" {
		cpuModel = firstLine(run(`grep -m1 "model name" /proc/cpuinfo 2>/dev/null | cut -d: -f2`))
	}
	kv("CPU 型号", strings.TrimSpace(cpuModel))
	kv("CPU 核数", firstField(run("nproc 2>/dev/null")))
	// 内存
	memTotal := pickField(run("cat /proc/meminfo 2>/dev/null"), "MemTotal:")
	memAvail := pickField(run("cat /proc/meminfo 2>/dev/null"), "MemAvailable:")
	kv("内存总量", memTotal)
	kv("内存可用", memAvail)
	// 运行时间
	kv("最近启动", firstLine(run("uptime -s 2>/dev/null")))
	kv("运行时长", firstLine(run("uptime -p 2>/dev/null")))
	// 磁盘挂载（含文件系统类型）
	dfOut := run("df -hT -x tmpfs -x devtmpfs -x squashfs 2>/dev/null")
	kv("磁盘挂载", fmt.Sprintf("%d 个挂载点", countLines(dfOut)-1))
	r.SysInfo = append(r.SysInfo, model.KV{Key: "__df", Value: dfOut})
	// 网络
	ipOut := run("ip -brief addr 2>/dev/null")
	if ipOut == "" {
		ipOut = run("hostname -I 2>/dev/null")
		kv("IP 地址", strings.TrimSpace(ipOut))
	} else {
		r.SysInfo = append(r.SysInfo, model.KV{Key: "__ip", Value: ipOut})
	}
	if r.OSKind == "" {
		r.OSKind = "linux"
	}
}

// ---------- Top N 进程（含启动时间与运行时长） ----------

func collectTopProcs(r *model.HostReport, run func(string) string) {
	// etime=运行时长, lstart=完整启动时间
	out := run(`ps -eo pid,user,pcpu,pmem,rss,vsz,etime,lstart,args --sort=-%cpu 2>/dev/null | head -11`)
	r.TopCPU = parsePSLong(out, 10)
	out2 := run(`ps -eo pid,user,pcpu,pmem,rss,vsz,etime,lstart,args --sort=-rss 2>/dev/null | head -11`)
	r.TopMem = parsePSLong(out2, 10)
}

// parsePSLong 解析 `ps -eo pid,user,pcpu,pmem,rss,vsz,etime,lstart,args` 输出。
// lstart 占 5 个字段（如 "Mon Nov 10 16:41:09 2026"）。
func parsePSLong(out string, max int) []model.ProcInfo {
	var list []model.ProcInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		fs := strings.Fields(line)
		if len(fs) < 12 {
			continue
		}
		if fs[0] == "PID" {
			continue
		}
		// 过滤 ps 自身（通过 SSH 执行时排序参数行会短暂占据榜首）
		full := strings.Join(fs[12:], " ")
		if strings.Contains(full, "--sort") || strings.Contains(full, "| head") {
			continue
		}
		list = append(list, model.ProcInfo{
			PID:   fs[0],
			User:  fs[1],
			CPU:   fs[2],
			MEM:   fs[3],
			RSS:   fs[4],
			VSZ:   fs[5],
			ETime: fs[6],
			Start: strings.Join(fs[7:12], " "),
			Cmd:   full,
		})
		if len(list) >= max {
			break
		}
	}
	return list
}

// ---------- rs_/rsa_ 进程 ----------

func collectRSProcs(r *model.HostReport, run func(string) string) {
	out := run(`ps -eo pid,user,pcpu,pmem,rss,vsz,etime,args 2>/dev/null`)
	pids := []model.RSProc{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		fs := strings.Fields(line)
		if len(fs) < 8 {
			continue
		}
		if fs[0] == "PID" {
			continue
		}
		base := baseCmd(fs[7])
		if !strings.HasPrefix(base, "rs_") && !strings.HasPrefix(base, "rsa_") {
			continue
		}
		pids = append(pids, model.RSProc{ProcInfo: model.ProcInfo{
			PID:   fs[0],
			User:  fs[1],
			CPU:   fs[2],
			MEM:   fs[3],
			RSS:   fs[4],
			VSZ:   fs[5],
			ETime: fs[6],
			Cmd:   strings.Join(fs[7:], " "),
		}})
		if len(pids) >= rsMaxProcs {
			break
		}
	}
	for i := range pids {
		p := &pids[i]
		pid := p.PID
		fdCount := strings.TrimSpace(run(fmt.Sprintf("ls /proc/%s/fd 2>/dev/null | wc -l", pid)))
		p.FDCount = orDash(fdCount)
		files := run(fmt.Sprintf("ls -l /proc/%s/fd 2>/dev/null | awk '{print $NF}' | sort -u | head -%d", pid, openFilesCap))
		for _, line := range strings.Split(files, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				p.OpenFiles = append(p.OpenFiles, line)
			}
		}
		conns := run(fmt.Sprintf("ss -tanp 2>/dev/null | grep \"pid=%s,\" | head -20", pid))
		for _, line := range strings.Split(conns, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				p.Conns = append(p.Conns, line)
			}
		}
	}
	r.RSProcs = pids
	if len(pids) == 0 {
		warn(r, "未发现 rs_/rsa_ 前缀进程（可能服务未运行或以其它方式命名）")
	}
}

func baseCmd(args string) string {
	fs := strings.Fields(args)
	if len(fs) == 0 {
		return ""
	}
	p := fs[0]
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// ---------- storcli64 RAID ----------

func collectStorcli(c *SSHClient, r *model.HostReport, run func(string) string) {
	// storcli64 路径固定为 /opt/MegaRAID/storcli/storcli64（避免 PATH 差异取到错误版本）
	bin := "/opt/MegaRAID/storcli/storcli64"
	if strings.TrimSpace(run("test -x "+bin+" && echo ok")) != "ok" {
		r.StorcliNote = "未检测到 RAID 卡或未安装 storcli64（/opt/MegaRAID/storcli/storcli64 不存在）"
		return
	}
	rawShow := run(bin + " /cALL show J 2>/dev/null")
	if strings.TrimSpace(rawShow) == "" {
		r.StorcliNote = "storcli64 已安装但未返回数据（可能无 RAID 卡或权限不足）"
		return
	}
	rep, err := parseStorcliJSON(rawShow)
	if err != nil {
		r.StorcliNote = "storcli64 输出解析失败：" + shortErr(err)
		return
	}
	// 原始输出收集（诊断用：完整呈现 storcli 各命令输出）
	var raw strings.Builder
	addRaw := func(cmd, out string) {
		if strings.TrimSpace(out) == "" {
			return
		}
		s := strings.TrimSpace(out)
		if len(s) > 200*1024 {
			s = s[:200*1024] + "\n…（超长截断）"
		}
		fmt.Fprintf(&raw, "=== %s ===\n%s\n", cmd, s)
	}
	addRaw(bin+" /cALL show J", rawShow)
	// PD 摘要缺信息时，用明细 JSON 补齐 SN / 通电时长 / 温度 / 介质 / 接口
	rawPD := ""
	needEnrich := false
	for _, ctl := range rep.Controllers {
		for _, pd := range ctl.PDs {
			if pd.SN == "" || pd.Power == "" || pd.Temp == "" || pd.MediaType == "" || pd.Interface == "" {
				needEnrich = true
				break
			}
		}
	}
	if needEnrich {
		rawPD = run(bin + " /cALL /eALL /sALL show J 2>/dev/null")
		enrichStorcliPDs(rawPD, rep)
		addRaw(bin+" /cALL /eALL /sALL show J", rawPD)
	}
	// show all 富化控制器固件 / ROC 温度 / 内存 / 控制器状态（摘要不含时；show all 仅保证单控制器形式）
	if anyCtlFieldEmpty(rep) && len(rep.Controllers) > 0 {
		rawShowAll := run(fmt.Sprintf("%s /c%d show all J 2>/dev/null", bin, 0))
		parseStorcliShowAllCtl(rawShowAll, &rep.Controllers[0])
		addRaw(fmt.Sprintf("%s /c0 show all J", bin), rawShowAll)
		for i := 1; i < len(rep.Controllers); i++ {
			o := run(fmt.Sprintf("%s /c%d show all J 2>/dev/null", bin, i))
			parseStorcliShowAllCtl(o, &rep.Controllers[i])
			addRaw(fmt.Sprintf("%s /c%d show all J", bin, i), o)
		}
	}
	// 摘要通常不含电池：单独查 BBU / CacheVault 详情（9364-8i 等卡用 CacheVault，storcli 仍走 /bbu 对象，部分版本需 /cv）
	for i := range rep.Controllers {
		ctl := &rep.Controllers[i]
		if ctl.Battery == "" || ctl.BatteryDetail == "" {
			bb := run(fmt.Sprintf("%s /c%d /bbu show all J 2>/dev/null", bin, i))
			addRaw(fmt.Sprintf("%s /c%d /bbu show all J", bin, i), bb)
			if ctl.Battery == "" {
				ctl.Battery = parseBBUState(bb)
			}
			if ctl.BatteryDetail == "" {
				ctl.BatteryDetail = parseBBUDetail(bb)
			}
			if ctl.Battery == "" && ctl.BatteryDetail == "" {
				cv := run(fmt.Sprintf("%s /c%d /cv show all J 2>/dev/null", bin, i))
				addRaw(fmt.Sprintf("%s /c%d /cv show all J", bin, i), cv)
				ctl.Battery = parseBBUState(cv)
				ctl.BatteryDetail = parseBBUDetail(cv)
			}
		}
	}
	// VD 条带大小与 Span 结构（摘要 VD LIST 不含）
	needVD := false
	for _, ctl := range rep.Controllers {
		for _, v := range ctl.VDs {
			if v.Strip == "" || v.Spans == "" {
				needVD = true
				break
			}
		}
	}
	if needVD {
		for i := range rep.Controllers {
			o := run(fmt.Sprintf("%s /c%d /vALL show all J 2>/dev/null", bin, i))
			enrichStorcliVDsCtl(o, &rep.Controllers[i])
			addRaw(fmt.Sprintf("%s /c%d /vALL show all J", bin, i), o)
		}
	}
	rep.Raw = raw.String()
	r.Storcli = rep
}

// anyCtlFieldEmpty 任一控制器仍有未填的固件/温度/内存。
func anyCtlFieldEmpty(rep *model.StorcliReport) bool {
	for _, c := range rep.Controllers {
		if c.Firmware == "" || c.Temp == "" || c.Memory == "" {
			return true
		}
	}
	return len(rep.Controllers) == 0
}

// parseBBUState 从 `/cN /bbu show all J` 提取电池状态摘要（兼容 BBU_Info 数组 / Cachevault_Info 映射）。
func parseBBUState(out string) string {
	rd := storcliCtlResponse(out, 0)
	if rd == nil {
		return ""
	}
	m := storcliBBUMap(rd)
	state := firstNonEmpty(
		asString(m["Battery State"]),
		asString(m["State"]),
		asString(rd["Battery State"]),
		asString(rd["Battery Status"]),
		nestedString(rd, "Battery", "Battery State"),
	)
	if state == "" {
		return ""
	}
	// BBU 用容量百分比，CacheVault 用电容（Capacitance）/健康度（SOH）表征
	cap := firstNonEmpty(
		asString(m["Capacitance"]),
		asString(m["State of Health"]),
		asString(m["SOH"]),
		asString(m["Capacity"]),
		asString(m["Full Charge Capacity"]),
		asString(m["Battery Capacity"]),
		asString(rd["Capacity"]),
		asString(rd["Full Charge Capacity"]),
		asString(rd["Battery Capacity"]),
	)
	if cap != "" {
		return state + "（" + cap + "）"
	}
	return state
}

// ---------- ZFS ----------

func collectZFS(c *SSHClient, r *model.HostReport, run func(string) string) {
	if strings.TrimSpace(run("command -v zpool 2>/dev/null")) == "" {
		r.ZFSNote = "未安装 ZFS（zpool 命令不存在）"
		return
	}
	// 池：显式指定 -o 列序，避免不同版本 zpool list 默认列数/顺序不同导致表格错位
	rep := &model.ZFSReport{PoolCols: []string{"名称", "大小", "已用", "剩余", "碎片率", "使用率", "重删", "健康状态"}}
	rep.Pools = parseStorcliLines(run("zpool list -H -o name,size,alloc,free,frag,cap,dedup,health 2>/dev/null"), 8)
	rep.Status = run("zpool status -v 2>/dev/null")
	// 数据集：基础 4 列 + 属性（压缩算法/压缩比/重删/重删比）按名称合并
	rep.DSectCols = []string{"数据集", "已用", "可用", "挂载点", "压缩算法", "压缩比", "重删", "重删比"}
	base := parseStorcliLines(run("zfs list -H -o name,used,avail,mountpoint 2>/dev/null"), 4)
	props := parseStorcliLines(run("zfs get -H -o name,property,value compress,compressratio,dedup,dedupratio 2>/dev/null"), 3)
	rep.Datasets = mergeZFSDatasets(base, props)
	if len(rep.Pools) == 0 {
		r.ZFSNote = "ZFS 已安装但未创建存储池"
	}
	r.ZFS = rep
}

// mergeZFSDatasets 将 zfs get 属性行（名称/属性/值）按数据集名合并进 zfs list 基础行，组成 8 列表。
// compress 列即压缩算法（off = 未开启；lz4/zstd… = 开启及算法），dedup 列为是否开启重删。
func mergeZFSDatasets(base, props [][]string) [][]string {
	byName := map[string]map[string]string{}
	for _, p := range props {
		if len(p) < 3 {
			continue
		}
		m, ok := byName[p[0]]
		if !ok {
			m = map[string]string{}
			byName[p[0]] = m
		}
		m[p[1]] = strings.TrimSpace(strings.Join(p[2:], "\t"))
	}
	var rows [][]string
	for _, b := range base {
		if len(b) < 4 {
			continue
		}
		m := byName[b[0]]
		get := func(k string) string {
			if m == nil {
				return ""
			}
			return m[k]
		}
		rows = append(rows, []string{
			b[0], b[1], b[2], b[3],
			get("compress"), get("compressratio"), get("dedup"), get("dedupratio"),
		})
	}
	return rows
}
