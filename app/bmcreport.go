package app

import (
	"fmt"
	"html"
	"strings"
	"time"

	"dbping/internal/model"
)

// renderCombinedBMC 生成 BMC 带外巡检综合报告（巡检 + AI，带目录锚点跳转）。
func renderCombinedBMC(h model.BMCHost, r *model.BMCReport, aiMarkdown, aiModel string) string {
	var b strings.Builder
	b.WriteString(reportHead("HostPing BMC 综合报告 — " + html.EscapeString(h.Name)))
	hasAI := strings.TrimSpace(aiMarkdown) != ""
	b.WriteString("<div style=\"background:#eef2f9;border:1px solid #d5dfef;border-radius:8px;padding:10px 14px;margin:0 0 18px;font-size:13.5px\"><b>目录：</b> <a href=\"#sec-bmc\">📋 巡检报告</a>")
	if hasAI {
		b.WriteString(" · <a href=\"#sec-ai\">🤖 AI 分析报告</a>")
	}
	b.WriteString("</div>")
	b.WriteString("<div id=\"sec-bmc\">" + bodyOnly(renderBMCReport(h, r)) + "</div>")
	if hasAI {
		b.WriteString("<div id=\"sec-ai\"><h1>🤖 AI 分析报告 — " + html.EscapeString(h.Name) + "</h1>")
		fmt.Fprintf(&b, "<div class='sub'>分析模型：%s · %s</div>", html.EscapeString(aiModel), time.Now().Format("2006-01-02 15:04:05"))
		b.WriteString(mdToHTML(aiMarkdown))
		b.WriteString("</div>")
	}
	b.WriteString(reportFoot())
	return b.String()
}

// renderBMCReport 渲染 BMC 巡检报告主体。
func renderBMCReport(h model.BMCHost, r *model.BMCReport) string {
	var b strings.Builder
	b.WriteString(reportHead("HostPing BMC 巡检报告 — " + html.EscapeString(h.Name)))
	b.WriteString(fmt.Sprintf("<h1>🔌 BMC 带外巡检 — %s</h1><div class='sub'>%s@%s:%d · %s</div>",
		html.EscapeString(h.Name), html.EscapeString(h.User),
		html.EscapeString(h.Host), h.Port, time.Now().Format("2006-01-02 15:04:05")))
	if r != nil {
		// 健康徽标
		cls, label := "na", "未知"
		switch r.Health {
		case "正常":
			cls, label = "ok", "正常"
		case "警告":
			cls, label = "warn", "警告"
		case "严重":
			cls, label = "err", "严重"
		}
		fmt.Fprintf(&b, "<h2>总体健康：<span class='%s'>%s</span></h2><div class='dim'>%s</div>", cls, label, html.EscapeString(orDefaultStr(r.HealthDetail, "-")))

		// BMC 设备
		b.WriteString("<h2>BMC 设备</h2>")
		if r.Device != nil {
			b.WriteString("<table class='kv'>")
			writeKVRow(&b, "厂商", r.Device.Manufacturer)
			fmt.Fprintf(&b, "<tr><th>产品</th><td>%s (ID 0x%s)</td></tr>", html.EscapeString(r.Device.Product), html.EscapeString(hexID(uint32(r.Device.ProductID), 4)))
			writeKVRow(&b, "固件版本", r.Device.Firmware)
			writeKVRow(&b, "IPMI 版本", r.Device.IPMIVersion)
			writeKVRow(&b, "自检结果", r.Device.SelfTest)
			writeKVRow(&b, "辅助固件", r.Device.AuxFirmware)
			b.WriteString("</table>")
		} else {
			b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.DeviceNote, "未采集到 BMC 设备信息")) + "</div>")
		}

		// 机箱状态
		b.WriteString("<h2>机箱状态</h2>")
		if len(r.Chassis) > 0 {
			b.WriteString("<table class='kv'>")
			for _, kv := range r.Chassis {
				writeKVRow(&b, kv.Key, kv.Value)
			}
			b.WriteString("</table>")
		} else {
			b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.ChassisNote, "未采集到机箱状态")) + "</div>")
		}

		// 硬件配置
		b.WriteString("<h2>硬件配置</h2>")
		if len(r.Hardware) > 0 {
			b.WriteString("<table class='kv'>")
			for _, kv := range r.Hardware {
				writeKVRow(&b, kv.Key, kv.Value)
			}
			b.WriteString("</table>")
		} else {
			b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.HardwareNote, "未采集到硬件配置")) + "</div>")
		}

		// 传感器
		fmt.Fprintf(&b, "<h2>传感器读数（%d）</h2>", len(r.Sensors))
		if len(r.Sensors) > 0 {
			b.WriteString("<table><tr><th>类型</th><th>名称</th><th>读数</th><th>状态</th><th>阈值</th></tr>")
			for _, s := range r.Sensors {
				scls := "na"
				stext := s.Status
				switch s.Status {
				case "nominal":
					scls, stext = "ok", "正常"
				case "warning":
					scls, stext = "warn", "警告"
				case "critical":
					scls, stext = "err", "严重"
				case "unknown":
					scls, stext = "na", "不可用"
				}
				fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td><span class='%s'>%s</span></td><td class='dim'>%s</td></tr>",
					html.EscapeString(s.Kind), html.EscapeString(s.Name), html.EscapeString(s.Value), scls, stext, html.EscapeString(orDefaultStr(s.Range, "-")))
			}
			b.WriteString("</table>")
			if r.SensorsNote != "" {
				fmt.Fprintf(&b, "<div class='dim'>%s</div>", html.EscapeString(r.SensorsNote))
			}
		} else {
			b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.SensorsNote, "未采集到传感器数据")) + "</div>")
		}

		// FRU
		b.WriteString("<h2>FRU 硬件清单</h2>")
		if len(r.FRU) > 0 {
			b.WriteString("<table class='kv'>")
			for _, kv := range r.FRU {
				if kv.Key == "__fru" {
					fmt.Fprintf(&b, "<tr><th class='fru-dev'>%s</th><td></td></tr>", html.EscapeString(kv.Value))
				} else {
					writeKVRow(&b, kv.Key, kv.Value)
				}
			}
			b.WriteString("</table>")
			if r.FRUNote != "" {
				fmt.Fprintf(&b, "<div class='dim'>%s</div>", html.EscapeString(r.FRUNote))
			}
		} else {
			b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.FRUNote, "未采集到 FRU 信息")) + "</div>")
		}

		// SEL
		b.WriteString("<h2>SEL 系统事件日志</h2>")
		if r.SEL != nil {
			fmt.Fprintf(&b, "<div class='dim'>共 %s 条：严重 <span class='err'>%d</span> · 警告 <span class='warn'>%d</span> · 信息 <span class='ok'>%d</span></div>",
				html.EscapeString(r.SEL.Total), r.SEL.CriticalCount, r.SEL.WarningCount, r.SEL.InfoCount)
			if len(r.SEL.TopErrors) > 0 {
				b.WriteString("<div class='dim'>高频错误：</div><div class='pre-block'>" + html.EscapeString(strings.Join(r.SEL.TopErrors, "\n")) + "</div>")
			}
			if r.SEL.Analysis != "" {
				fmt.Fprintf(&b, "<div class='dim'>规则分析：%s</div>", html.EscapeString(r.SEL.Analysis))
			}
			if len(r.SEL.Entries) > 0 {
				b.WriteString("<table><tr><th>时间</th><th>记录ID</th><th>传感器</th><th>事件</th><th>级别</th></tr>")
				for _, e := range r.SEL.Entries {
					ecls, etext := "na", e.Severity
					switch e.Severity {
					case "critical":
						ecls, etext = "err", "严重"
					case "warning":
						ecls, etext = "warn", "警告"
					case "info":
						ecls, etext = "ok", "信息"
					}
					fmt.Fprintf(&b, "<tr><td>%s</td><td class='dim'>%s</td><td>%s</td><td>%s</td><td><span class='%s'>%s</span></td></tr>",
						html.EscapeString(e.Time), html.EscapeString(e.RecordID), html.EscapeString(e.Sensor), html.EscapeString(e.Event), ecls, etext)
				}
				b.WriteString("</table>")
			}
			if r.SEL.Note != "" {
				fmt.Fprintf(&b, "<div class='dim'>%s</div>", html.EscapeString(r.SEL.Note))
			}
		} else {
			b.WriteString("<div class='na'>" + html.EscapeString(orDefaultStr(r.SELNote, "未采集到 SEL 日志")) + "</div>")
		}

		// 巡检警告
		if len(r.Warnings) > 0 {
			b.WriteString("<div class='warn'>")
			for _, w := range r.Warnings {
				b.WriteString("⚠ " + html.EscapeString(w) + "<br>")
			}
			b.WriteString("</div>")
		}
	}
	b.WriteString(reportFoot())
	return b.String()
}

func writeKVRow(b *strings.Builder, key, val string) {
	if strings.TrimSpace(val) == "" {
		return
	}
	fmt.Fprintf(b, "<tr><th>%s</th><td>%s</td></tr>", html.EscapeString(key), html.EscapeString(val))
}

// hexID 十六进制 ID 串（固定位数，无前缀）。
func hexID(v uint32, width int) string {
	const digits = "0123456789abcdef"
	out := make([]byte, width)
	for i := width - 1; i >= 0; i-- {
		out[i] = digits[v&0xF]
		v >>= 4
	}
	return string(out)
}
