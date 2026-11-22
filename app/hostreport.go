package app

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"dbping/internal/model"
)

// renderHostReport 生成主机巡检 HTML 报告。
// renderCombinedHost 主机综合报告：巡检报告 + AI 分析（带目录锚点跳转）。
func renderCombinedHost(h model.SSHHost, r *model.HostReport, aiMarkdown, aiModel string) string {
	var b strings.Builder
	b.WriteString(reportHead("HostPing 主机综合报告 — " + html.EscapeString(h.Name)))
	hasAI := strings.TrimSpace(aiMarkdown) != ""
	b.WriteString("<div style=\"background:#eef2f9;border:1px solid #d5dfef;border-radius:8px;padding:10px 14px;margin:0 0 18px;font-size:13.5px\"><b>目录：</b> <a href=\"#sec-inspect\">📋 巡检报告</a>")
	if hasAI {
		b.WriteString(" · <a href=\"#sec-ai\">🤖 AI 分析报告</a>")
	}
	b.WriteString("</div>")
	b.WriteString("<div id=\"sec-inspect\">" + bodyOnly(renderHostReport(h, r)) + "</div>")
	if hasAI {
		b.WriteString("<div id=\"sec-ai\"><h1>🤖 AI 分析报告 — " + html.EscapeString(h.Name) + "</h1>")
		fmt.Fprintf(&b, "<div class='sub'>分析模型：%s · %s</div>", html.EscapeString(aiModel), time.Now().Format("2006-01-02 15:04:05"))
		b.WriteString(mdToHTML(aiMarkdown))
		b.WriteString("</div>")
	}
	b.WriteString(reportFoot())
	return b.String()
}

func renderHostReport(h model.SSHHost, r *model.HostReport) string {
	var b strings.Builder
	b.WriteString(reportHead("HostPing 主机巡检报告 — " + html.EscapeString(h.Name)))
	b.WriteString(fmt.Sprintf("<h1>🖥 主机巡检 — %s</h1><div class='sub'>%s@%s:%d · %s</div>",
		html.EscapeString(h.Name), html.EscapeString(h.User),
		html.EscapeString(h.Host), h.Port, time.Now().Format("2006-01-02 15:04:05")))

	if r != nil {
		// 系统信息
		b.WriteString("<h2>系统信息</h2><table class='kv'>")
		if r.OSName != "" {
			fmt.Fprintf(&b, "<tr><th>操作系统</th><td>%s</td></tr>", html.EscapeString(r.OSName))
		}
		for _, kv := range r.SysInfo {
			k := kv.Key
			if strings.HasPrefix(k, "__") {
				continue // df/ip 大文本块单独渲染
			}
			fmt.Fprintf(&b, "<tr><th>%s</th><td>%s</td></tr>", html.EscapeString(k), html.EscapeString(kv.Value))
		}
		b.WriteString("</table>")
		// lspci / lsscsi（放在磁盘挂载之前，PCI 在前）
		if strings.TrimSpace(r.Lspci) != "" {
			b.WriteString("<h2>PCI 设备（lspci）</h2><div class='pre-block'>" + html.EscapeString(r.Lspci) + "</div>")
		}
		if strings.TrimSpace(r.Lsscsi) != "" {
			b.WriteString("<h2>SCSI 设备（lsscsi）</h2><div class='pre-block'>" + html.EscapeString(r.Lsscsi) + "</div>")
		}
		// RAID / ZFS（放在 SCSI 设备之后，与存储链路相邻）
		writeRAIDSection(&b, r)
		writeZFSSection(&b, r)
		// df 表格
		for _, kv := range r.SysInfo {
			if kv.Key == "__df" {
				b.WriteString("<h2>磁盘挂载</h2>")
				b.WriteString(renderPlainTable(kv.Value, []string{"文件系统", "类型", "容量", "已用", "可用", "已用%", "挂载点"}))
			}
		}
		// 网络接口
		for _, kv := range r.SysInfo {
			if kv.Key == "__ip" {
				b.WriteString("<h2>网络接口</h2>")
				b.WriteString(renderNetIfTable(kv.Value))
			}
		}

		// 监听端口
		b.WriteString("<h2>端口监听</h2>")
		if len(r.ListenPorts) > 0 {
			b.WriteString("<table><tr><th>端口</th><th>服务</th><th>进程</th><th>状态</th><th>连接数</th></tr>")
			for _, p := range r.ListenPorts {
				fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
					html.EscapeString(p.Port), html.EscapeString(p.Service), html.EscapeString(p.Process),
					html.EscapeString(p.State), html.EscapeString(p.Conns))
			}
			b.WriteString("</table>")
		} else {
			b.WriteString("<div class='na'>未采集到监听端口</div>")
		}

		// 安全状态
		b.WriteString("<h2>安全状态</h2><table class='kv'>")
		for _, kv := range r.Security {
			if strings.HasPrefix(kv.Key, "__") {
				b.WriteString("</table><div class='pre-block'>" + html.EscapeString(kv.Value) + "</div><table class='kv'>")
				continue
			}
			fmt.Fprintf(&b, "<tr><th>%s</th><td>%s</td></tr>", html.EscapeString(kv.Key), html.EscapeString(kv.Value))
		}
		b.WriteString("</table>")

		// 系统日志（移到最后统一渲染）

		// Top CPU
		b.WriteString("<h2>Top 10 进程（按 CPU）</h2>")
		b.WriteString(procTable(r.TopCPU))
		b.WriteString("<h2>Top 10 进程（按内存）</h2>")
		b.WriteString(procTable(r.TopMem))

		// rs_/rsa_ 进程
		b.WriteString("<h2>rs_/rsa_ 进程巡检</h2>")
		if len(r.RSProcs) == 0 {
			b.WriteString("<div class='na'>未发现 rs_/rsa_ 前缀进程</div>")
		} else {
			for _, p := range r.RSProcs {
				fmt.Fprintf(&b, "<h3 style='color:#2563eb'>%s (PID %s, 用户 %s)</h3>", html.EscapeString(baseName(p.Cmd)), html.EscapeString(p.PID), html.EscapeString(p.User))
				b.WriteString(fmt.Sprintf("<table class='kv'><tr><th>CPU%%</th><td>%s</td></tr><tr><th>内存%%</th><td>%s</td></tr><tr><th>内存(MB)</th><td>%s</td></tr><tr><th>运行时长</th><td>%s</td></tr><tr><th>打开文件数</th><td>%s</td></tr></table>",
					html.EscapeString(p.CPU), html.EscapeString(p.MEM), html.EscapeString(kbToMB(p.RSS)), html.EscapeString(p.ETime), html.EscapeString(p.FDCount)))
				if len(p.OpenFiles) > 0 {
					b.WriteString("<div class='pre-block'>打开文件：<br>" + html.EscapeString(strings.Join(p.OpenFiles, "\n")) + "</div>")
				}
				if len(p.Conns) > 0 {
					b.WriteString("<div class='pre-block'>网络连接：<br>" + html.EscapeString(strings.Join(p.Conns, "\n")) + "</div>")
				}
			}
		}

		// RAID / ZFS 已移至 SCSI 设备之后（writeRAIDSection / writeZFSSection）

		// 警告
		if len(r.Warnings) > 0 {
			b.WriteString("<div class='warn'>")
			for _, w := range r.Warnings {
				b.WriteString("⚠ " + html.EscapeString(w) + "<br>")
			}
			b.WriteString("</div>")
		}

		// 系统日志（最后）
		if r.SysLog != nil {
			b.WriteString("<h2>系统日志（24h）</h2>")
			fmt.Fprintf(&b, "<div class='dim'>错误 %s 条 · 警告 %s 条 · 来源：%s</div>",
				html.EscapeString(r.SysLog.ErrCount), html.EscapeString(r.SysLog.WarnCount), html.EscapeString(r.SysLog.Source))
			if len(r.SysLog.TopErrors) > 0 {
				b.WriteString("<div class='pre-block'>" + html.EscapeString(strings.Join(r.SysLog.TopErrors, "\n")) + "</div>")
			}
			if len(r.SysLog.DmesgErrs) > 0 {
				b.WriteString("<div class='dim'>dmesg 错误：</div><div class='pre-block'>" + html.EscapeString(strings.Join(r.SysLog.DmesgErrs, "\n")) + "</div>")
			}
		}

		// 重启记录（最后）
		b.WriteString("<h2>重启记录（最近 10 次）</h2>")
		if len(r.Reboots) > 0 {
			b.WriteString("<table><tr><th>#</th><th>启动时间</th><th>结束时间</th><th>本次运行时长</th></tr>")
			for i, rb := range r.Reboots {
				end := rb.End
				if end == "-" || end == "" {
					end = "—"
				}
				dur := rb.Dur
				if strings.Contains(strings.ToLower(dur), "still") {
					dur = "运行中"
				}
				fmt.Fprintf(&b, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
					i+1, html.EscapeString(rb.Start), html.EscapeString(end), html.EscapeString(orDefaultStr(dur, "-")))
			}
			b.WriteString("</table>")
		} else {
			b.WriteString("<div class='na'>未采集到重启记录</div>")
		}
	}

	b.WriteString(reportFoot())
	return b.String()
}

func procTable(list []model.ProcInfo) string {
	if len(list) == 0 {
		return "<div class='na'>无数据</div>"
	}
	var b strings.Builder
	b.WriteString("<table><tr><th>PID</th><th>用户</th><th>CPU%</th><th>内存%</th><th>内存(MB)</th><th>启动时间</th><th>运行时长</th><th>命令</th></tr>")
	for _, p := range list {
		cmd := html.EscapeString(p.Cmd)
		// rs_/rsa_ 前缀进程标蓝以示区别
		if base := baseName(p.Cmd); strings.HasPrefix(base, "rs_") || strings.HasPrefix(base, "rsa_") {
			cmd = "<span style='color:#2563eb;font-weight:600'>" + cmd + "</span>"
		}
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(p.PID), html.EscapeString(p.User), html.EscapeString(p.CPU),
			html.EscapeString(p.MEM), html.EscapeString(kbToMB(p.RSS)), html.EscapeString(p.Start),
			html.EscapeString(p.ETime), cmd)
	}
	b.WriteString("</table>")
	return b.String()
}

// kbToMB 进程 RSS（KB）转 MB 显示。
func kbToMB(rss string) string {
	n, err := strconv.ParseFloat(strings.TrimSpace(rss), 64)
	if err != nil || n <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f MB", n/1024)
}

// renderNetIfTable 渲染 `ip -brief addr` 输出为 接口/状态/地址 表格（状态前置图标）。
func renderNetIfTable(text string) string {
	var b strings.Builder
	b.WriteString("<table><tr><th>接口</th><th>状态</th><th>地址</th></tr>")
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		fs := strings.Fields(line)
		if len(fs) == 0 {
			continue
		}
		addrs := strings.Join(fs[2:], " ")
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s %s</td><td>%s</td></tr>",
			html.EscapeString(fs[0]), html.EscapeString(netStateIcon(fs[1])), html.EscapeString(orDashStr(fs[1])), html.EscapeString(addrs))
	}
	b.WriteString("</table>")
	return b.String()
}

// netStateIcon 网络接口状态前置图标。
func netStateIcon(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "UP":
		return "🟢"
	case "DOWN":
		return "🔴"
	default:
		return "⚪"
	}
}

func orDashStr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func renderPlainTable(text string, headers []string) string {
	var b strings.Builder
	b.WriteString("<table><tr>")
	for _, c := range headers {
		fmt.Fprintf(&b, "<th>%s</th>", html.EscapeString(c))
	}
	b.WriteString("</tr>")
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "Filesystem") || strings.HasPrefix(line, "文件系统") {
			continue
		}
		fs := strings.Fields(line)
		b.WriteString("<tr>")
		for _, f := range fs {
			fmt.Fprintf(&b, "<td>%s</td>", html.EscapeString(f))
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</table>")
	return b.String()
}

func baseName(cmd string) string {
	fs := strings.Fields(cmd)
	if len(fs) == 0 {
		return ""
	}
	p := fs[0]
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		p = p[i+1:]
	}
	return p
}

func orDefaultStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// writeRAIDSection 渲染 RAID 卡（storcli64）分节（SCSI 设备之后）。
func writeRAIDSection(b *strings.Builder, r *model.HostReport) {
	b.WriteString("<h2>RAID 卡（storcli64）</h2>")
	if r.Storcli != nil && len(r.Storcli.Controllers) > 0 {
		for _, c := range r.Storcli.Controllers {
			fmt.Fprintf(b, "<h3>控制器 %s — %s（状态 %s）</h3>", html.EscapeString(c.Index), html.EscapeString(c.Model), html.EscapeString(orDefaultStr(c.Status, "-")))
			var ctlBits []string
			if c.CtlStatus != "" {
				ctlBits = append(ctlBits, "控制器状态: "+c.CtlStatus)
			}
			if c.Firmware != "" {
				ctlBits = append(ctlBits, "固件版本: "+c.Firmware)
			}
			if c.Temp != "" {
				ctlBits = append(ctlBits, "温度: "+c.Temp)
			}
			if c.Memory != "" {
				ctlBits = append(ctlBits, "缓存内存: "+c.Memory)
			}
			if c.Battery != "" {
				ctlBits = append(ctlBits, "电池: "+c.Battery)
			}
			if len(ctlBits) > 0 {
				fmt.Fprintf(b, "<div class='dim'>%s</div>", html.EscapeString(strings.Join(ctlBits, " · ")))
			}
			if c.BatteryDetail != "" {
				fmt.Fprintf(b, "<div class='dim'>电池/CacheVault 详情：%s</div>", html.EscapeString(c.BatteryDetail))
			}
			if len(c.VDs) > 0 {
				b.WriteString("<div class='dim'>RAID 组（VD）：</div><table><tr><th>VD</th><th>名称</th><th>级别</th><th>状态</th><th>容量</th><th>缓存</th><th>条带</th><th>Span 结构</th></tr>")
				for _, v := range c.VDs {
					fmt.Fprintf(b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
						html.EscapeString(v.ID), html.EscapeString(v.Name), html.EscapeString(v.Type), html.EscapeString(v.State), html.EscapeString(v.Size), html.EscapeString(orDefaultStr(v.Cache, "-")),
						html.EscapeString(orDefaultStr(v.Strip, "-")), html.EscapeString(orDefaultStr(v.Spans, "-")))
				}
				b.WriteString("</table>")
			}
			if len(c.PDs) > 0 {
				b.WriteString("<div class='dim'>物理磁盘（PD）：</div><table><tr><th>槽位</th><th>型号</th><th>状态</th><th>容量</th><th>介质</th><th>接口</th><th>温度</th><th>SN</th><th>通电时长</th><th>详情</th></tr>")
				for _, p := range c.PDs {
					fmt.Fprintf(b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
						html.EscapeString(p.Slot), html.EscapeString(p.Model), html.EscapeString(p.State), html.EscapeString(p.Size),
						html.EscapeString(orDefaultStr(p.MediaType, "-")), html.EscapeString(orDefaultStr(p.Interface, "-")), html.EscapeString(orDefaultStr(p.Temp, "-")),
						html.EscapeString(orDefaultStr(p.SN, "-")), html.EscapeString(orDefaultStr(p.Power, "-")), html.EscapeString(p.Detail))
				}
				b.WriteString("</table>")
			}
		}
	} else {
		b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.StorcliNote, "未检测到 RAID 卡")) + "</div>")
	}
	// storcli 原始输出（完整，诊断用；可折叠）
	if r.Storcli != nil && strings.TrimSpace(r.Storcli.Raw) != "" {
		b.WriteString("<details style='margin:8px 0 0'><summary style='cursor:pointer;color:#2563eb;font-size:13px'>storcli 原始输出（完整）</summary><div class='pre-block'>" + html.EscapeString(r.Storcli.Raw) + "</div></details>")
	}
}

// writeZFSSection 渲染 ZFS 分节（SCSI 设备之后）。
func writeZFSSection(b *strings.Builder, r *model.HostReport) {
	b.WriteString("<h2>ZFS</h2>")
	if r.ZFS != nil {
		if len(r.ZFS.Pools) > 0 {
			b.WriteString("<div class='dim'>存储池：</div><table>")
			b.WriteString("<tr>")
			for _, c := range r.ZFS.PoolCols {
				fmt.Fprintf(b, "<th>%s</th>", html.EscapeString(c))
			}
			b.WriteString("</tr>")
			for _, row := range r.ZFS.Pools {
				b.WriteString("<tr>")
				for _, v := range row {
					fmt.Fprintf(b, "<td>%s</td>", html.EscapeString(v))
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</table>")
		}
		if strings.TrimSpace(r.ZFS.Status) != "" {
			b.WriteString("<div class='dim'>zpool status：</div><div class='pre-block'>" + html.EscapeString(r.ZFS.Status) + "</div>")
		}
		if len(r.ZFS.Datasets) > 0 {
			b.WriteString("<div class='dim'>数据集：</div><table><tr>")
			for _, c := range r.ZFS.DSectCols {
				fmt.Fprintf(b, "<th>%s</th>", html.EscapeString(c))
			}
			b.WriteString("</tr>")
			for _, row := range r.ZFS.Datasets {
				b.WriteString("<tr>")
				for _, v := range row {
					fmt.Fprintf(b, "<td>%s</td>", html.EscapeString(v))
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</table>")
		}
	} else {
		b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.ZFSNote, "未检测到 ZFS")) + "</div>")
	}
}

