package app

// speedtest.go 测速（纯 Go iperf3 客户端协议）+ 测速/扫描历史的删除与导出。

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"strings"
	"sync"
	"time"

	"dbping/internal/model"
	"dbping/internal/speedtest"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var iperfBusy sync.Mutex

// IPerfStart 连接 iperf3 服务端执行 TCP 带宽测试（上行/下行），逐秒推送进度事件 iperf:progress。
// server: 服务端地址；port: 服务端口（5201）；duration: 测试秒数；omit: 预热秒数；
// parallel: 并行流数；reverse: true=下行（服务端发送）。完成后自动存入测速历史。
func (a *App) IPerfStart(server string, port, duration, omit, parallel int, reverse bool) (map[string]any, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("请输入测速服务器地址")
	}
	if port < 1 || port > 65535 {
		port = 5201
	}
	if !iperfBusy.TryLock() {
		return nil, fmt.Errorf("测速进行中，请等待当前测试完成")
	}
	defer iperfBusy.Unlock()

	res, err := speedtest.RunTest(server, port, duration, omit, parallel, reverse, func(iv speedtest.Interval) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "iperf:progress", map[string]any{
				"sec": iv.Sec, "mbps": iv.MBps, "retrans": iv.Retrans,
				"omitted": iv.Omitted, "total": duration + omit,
			})
		}
	})
	if err != nil {
		return nil, err
	}
	// 测速历史保留
	entry := model.SpeedEntry{
		Time: time.Now().UnixMilli(), Server: server, Port: port,
		Direction: res.Direction, Parallel: parallel, Duration: duration,
		Seconds: res.Seconds, Mbps: res.Mbps, ServerMbps: res.ServerMbps,
		Retrans: res.Retrans, Warning: res.Warning,
	}
	for _, iv := range res.Intervals {
		entry.Intervals = append(entry.Intervals, model.SpeedInterval{
			Sec: iv.Sec, MBps: iv.MBps, Retrans: iv.Retrans, Omitted: iv.Omitted,
		})
	}
	st := a.store()
	st.AddSpeedEntry(entry)
	if err := st.Save(); err != nil {
		log.Printf("[speedtest] 历史保存失败: %v", err)
	}
	return map[string]any{
		"direction": res.Direction, "seconds": res.Seconds, "mbps": res.Mbps,
		"serverMbps": res.ServerMbps, "retrans": res.Retrans,
		"warning": res.Warning, "intervals": res.Intervals, "time": entry.Time,
	}, nil
}

// GetSpeedHistory 返回测速历史（最新在前）。
func (a *App) GetSpeedHistory() []model.SpeedEntry {
	return a.store().ListSpeedHistory()
}

// DeleteSpeedEntry 删除一条测速历史。
func (a *App) DeleteSpeedEntry(t int64) error {
	a.store().DeleteSpeedEntry(t)
	return a.store().Save()
}

// DeleteNmapHistoryEntry 删除一条端口扫描历史。
func (a *App) DeleteNmapHistoryEntry(t int64) error {
	a.store().DeleteNmapHistoryEntry(t)
	return a.store().Save()
}

// ExportSpeedHistory 导出测速历史 HTML 报告（含每秒区间明细），返回保存路径。
func (a *App) ExportSpeedHistory() (string, error) {
	entries := a.store().ListSpeedHistory()
	if len(entries) == 0 {
		return "", fmt.Errorf("暂无测速历史")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出测速历史",
		DefaultFilename: fmt.Sprintf("HostPing测速历史_%s.html", time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	var b strings.Builder
	b.WriteString(reportHead("HostPing 测速历史报告"))
	b.WriteString(fmt.Sprintf("<h1>🚀 测速历史报告</h1><div class='sub'>共 %d 条记录 · 生成于 %s</div>",
		len(entries), time.Now().Format("2006-01-02 15:04:05")))
	// 目录
	b.WriteString("<div style='background:#eef2f9;border:1px solid #d5dfef;border-radius:8px;padding:10px 14px;margin:0 0 18px;font-size:13.5px'><b>目录：</b> ")
	for i, e := range entries {
		fmt.Fprintf(&b, "<a href='#spd-%d'>%d. %s</a> &nbsp;", i+1, i+1, html.EscapeString(shortTime(e.Time)))
	}
	b.WriteString("</div>")
	for i, e := range entries {
		dir := "上行"
		if e.Direction == "down" {
			dir = "下行"
		}
		fmt.Fprintf(&b, "<div id='spd-%d'><h2>🚀 %s · %s:%d（%s · %d 流 · %d 秒）</h2>", i+1,
			html.EscapeString(shortTime(e.Time)), html.EscapeString(e.Server), e.Port, dir, e.Parallel, e.Duration)
		if e.Warning != "" {
			fmt.Fprintf(&b, "<div class='warn'>%s</div>", html.EscapeString(e.Warning))
		}
		fmt.Fprintf(&b, "<table class='kv'>"+
			"<tr><th>本端实测</th><td>%.1f Mbps</td></tr>"+
			"<tr><th>对端实测</th><td>%s</td></tr>"+
			"<tr><th>重传</th><td>%s</td></tr></table>",
			e.Mbps, html.EscapeString(orFloat(e.ServerMbps)), html.EscapeString(orRetrans(e.Retrans)))
		b.WriteString("<h3>每秒速率</h3><table><tr><th>时间</th><th>速率</th><th>备注</th></tr>")
		for _, iv := range e.Intervals {
			note := ""
			if iv.Omitted {
				note = "预热（不计入汇总）"
			}
			fmt.Fprintf(&b, "<tr><td>%ds</td><td>%.1f Mbps</td><td>%s</td></tr>", iv.Sec, iv.MBps, note)
		}
		b.WriteString("</table></div>")
	}
	b.WriteString(reportFoot())
	if err := writeHTML(path, b.String()); err != nil {
		return "", err
	}
	log.Printf("[speedtest] 历史已导出: %s", path)
	return path, nil
}

// ExportNmapHistory 导出端口扫描历史 HTML 报告，返回保存路径。
func (a *App) ExportNmapHistory() (string, error) {
	entries := a.store().ListNmapHistory()
	if len(entries) == 0 {
		return "", fmt.Errorf("暂无扫描历史")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出端口扫描历史",
		DefaultFilename: fmt.Sprintf("HostPing端口扫描历史_%s.html", time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	var b strings.Builder
	b.WriteString(reportHead("HostPing 端口扫描历史报告"))
	b.WriteString(fmt.Sprintf("<h1>🔎 端口扫描历史报告</h1><div class='sub'>共 %d 条记录 · 生成于 %s</div>",
		len(entries), time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("<div style='background:#eef2f9;border:1px solid #d5dfef;border-radius:8px;padding:10px 14px;margin:0 0 18px;font-size:13.5px'><b>目录：</b> ")
	for i, e := range entries {
		fmt.Fprintf(&b, "<a href='#scan-%d'>%d. %s</a> &nbsp;", i+1, i+1, html.EscapeString(shortTime(e.Time)))
	}
	b.WriteString("</div>")
	for i, e := range entries {
		fmt.Fprintf(&b, "<div id='scan-%d'><h2>🔎 %s（端口范围 %s）</h2>", i+1,
			html.EscapeString(e.Target), html.EscapeString(orDefaultStr(e.PortSpec, "-")))
		fmt.Fprintf(&b, "<div class='sub'>%s · 系统推测：%s · 开放 %d / 共 %d 端口</div>",
			html.EscapeString(shortTime(e.Time)), html.EscapeString(orDefaultStr(e.OSGuess, "未知")),
			e.OpenCount, e.Total)
		if e.OSBasis != "" {
			fmt.Fprintf(&b, "<div class='dim'>推测依据：%s</div>", html.EscapeString(e.OSBasis))
		}
		if len(e.Ports) > 0 {
			b.WriteString("<h3>开放端口</h3><table><tr><th>端口</th><th>服务</th></tr>")
			for _, p := range e.Ports {
				fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td></tr>", html.EscapeString(p.Port), html.EscapeString(p.Service))
			}
			b.WriteString("</table>")
		} else {
			b.WriteString("<div class='na'>指定范围内无开放端口</div>")
		}
		b.WriteString("</div>")
	}
	b.WriteString(reportFoot())
	if err := writeHTML(path, b.String()); err != nil {
		return "", err
	}
	log.Printf("[nmap] 历史已导出: %s", path)
	return path, nil
}

func shortTime(t int64) string {
	return time.UnixMilli(t).Format("01-02 15:04")
}

func orFloat(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f Mbps", v)
}

func orRetrans(v int64) string {
	if v < 0 {
		return "-"
	}
	return fmt.Sprintf("%d", v)
}

var _ = json.Marshal
