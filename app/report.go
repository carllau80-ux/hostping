package app

import (
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"dbping/internal/model"
	"dbping/internal/provider"
)

// providerTypeInfos 前端需要的类型清单。
func providerTypeInfos() []model.TypeInfo { return provider.TypeInfos }

// renderReport 生成单连接巡检报告 HTML。
// renderCombinedDB 数据库综合报告：巡检报告 + AI 分析（带目录锚点跳转）。
func renderCombinedDB(c model.Conn, ov *model.Overview, dtl *model.Detail, topo *model.Topology, hist []model.PingResult, aiMarkdown, aiModel string) string {
	var b strings.Builder
	b.WriteString(reportHead("HostPing 数据库综合报告 — " + html.EscapeString(c.Name)))
	hasAI := strings.TrimSpace(aiMarkdown) != ""
	b.WriteString("<div style=\"background:#eef2f9;border:1px solid #d5dfef;border-radius:8px;padding:10px 14px;margin:0 0 18px;font-size:13.5px\"><b>目录：</b> <a href=\"#sec-inspect\">📋 巡检报告</a>")
	if hasAI {
		b.WriteString(" · <a href=\"#sec-ai\">🤖 AI 分析报告</a>")
	}
	b.WriteString("</div>")
	b.WriteString("<div id=\"sec-inspect\">" + bodyOnly(renderReport(c, ov, dtl, topo, hist)) + "</div>")
	if hasAI {
		b.WriteString("<div id='sec-ai'><h1>🤖 AI 分析报告 — " + html.EscapeString(c.Name) + "</h1>")
		fmt.Fprintf(&b, "<div class='sub'>分析模型：%s · %s</div>", html.EscapeString(aiModel), time.Now().Format("2006-01-02 15:04:05"))
		b.WriteString(mdToHTML(aiMarkdown))
		b.WriteString("</div>")
	}
	b.WriteString(reportFoot())
	return b.String()
}

// bodyOnly 从完整 HTML 文档中提取 body 内部内容（用于嵌入综合报告）。
func bodyOnly(doc string) string {
	i := strings.Index(doc, "<body")
	if i < 0 {
		return doc
	}
	j := strings.Index(doc[i:], ">")
	if j < 0 {
		return doc
	}
	start := i + j + 1
	end := strings.LastIndex(doc, "</body>")
	if end <= start {
		return doc
	}
	return doc[start:end]
}

func renderReport(c model.Conn, ov *model.Overview, dtl *model.Detail, topo *model.Topology, hist []model.PingResult) string {
	var b strings.Builder
	b.WriteString(reportHead("HostPing 巡检报告 — " + html.EscapeString(c.Name)))
	b.WriteString(fmt.Sprintf("<h1>%s %s</h1><div class='sub'>%s · %s:%d · %s</div>",
		typeIcon(c.Type), html.EscapeString(c.Name), html.EscapeString(typeName(c.Type)),
		html.EscapeString(c.Host), c.Port, time.Now().Format("2006-01-02 15:04:05")))

	if ov != nil {
		b.WriteString("<h2>实例概览</h2>")
		b.WriteString(overviewTable(ov))
	}

	if dtl != nil {
		if len(dtl.Services) > 0 {
			b.WriteString("<h2>集群/服务</h2>" + svcTable(dtl.Services))
		}
		if len(dtl.TBS) > 0 {
			b.WriteString("<h2>表空间/文件</h2>" + tbsTable(dtl.TBS))
		}
		if len(dtl.Tables) > 0 {
			b.WriteString("<h2>大表 Top N</h2>" + tableTable(dtl.Tables))
		}
	}

	if len(hist) > 0 {
		b.WriteString("<h2>最近检查记录</h2><table><tr><th>时间</th><th>结果</th><th>TCP(ms)</th><th>SQL(ms)</th><th>说明</th></tr>")
		for _, r := range hist {
			okStr := "<span class='ok'>正常</span>"
			detail := ""
			if !r.OK {
				okStr = "<span class='err'>故障</span>"
				detail = html.EscapeString(r.Error)
			}
			b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
				time.UnixMilli(r.CheckedAt).Format("01-02 15:04:05"), okStr, r.TCPMs, r.SQLMs, detail))
		}
		b.WriteString("</table>")
	}

	// 拓扑树（纯 HTML 缩进树）
	if topo != nil && topo.Root != nil {
		b.WriteString("<h2>拓扑结构</h2><div class='tree'>")
		renderTreeNode(&b, topo.Root, 0)
		b.WriteString("</div>")
	}

	b.WriteString(reportFoot())
	return b.String()
}

// sumRow 汇总报告的一行（连接 + 概览 + 最近检查结果）。
type sumRow struct {
	conn    model.Conn
	ov      *model.Overview
	res     model.PingResult
	checked bool
}

// overviewTable 生成实例概览 kv 表（单连接报告与汇总报告共用）。
func overviewTable(ov *model.Overview) string {
	if ov == nil {
		return "<div class='na'>（概览采集失败或无数据）</div>"
	}
	var b strings.Builder
	b.WriteString("<table class='kv'>")
	row := func(k, v string) {
		if v != "" && v != "-" {
			fmt.Fprintf(&b, "<tr><th>%s</th><td>%s</td></tr>", html.EscapeString(k), html.EscapeString(v))
		}
	}
	row("版本", ov.VersionFull)
	row("实例", ov.Instance)
	row("主机", ov.Hostname)
	row("操作系统", ov.OS)
	row("角色", ov.Role)
	row("只读", ov.ReadOnly)
	row("归档/日志模式", ov.ArchiveMode)
	row("字符集", ov.Charset)
	row("时区", ov.Timezone)
	row("启动时间", ov.StartupTime)
	row("已运行", ov.Uptime)
	row("数据库大小", ov.DBSize)
	row("对象数量", ov.TableCount)
	row("连接数（当前/最大）", ov.ConnCur+" / "+ov.ConnMax)
	row("集群形态", ov.Cluster)
	for _, kv := range ov.Extra {
		row(kv.Key, kv.Value)
	}
	b.WriteString("</table>")
	if len(ov.Warnings) > 0 {
		b.WriteString("<div class='warn'>")
		for _, w := range ov.Warnings {
			b.WriteString("⚠ " + html.EscapeString(w) + "<br>")
		}
		b.WriteString("</div>")
	}
	return b.String()
}

// renderSummaryReport 汇总报告：连接总表 + 每个连接的实例概览。
func renderSummaryReport(rows []*sumRow, generated time.Time) string {
	var b strings.Builder
	b.WriteString(reportHead("HostPing 巡检汇总报告"))
	b.WriteString(fmt.Sprintf("<h1>数据库巡检汇总</h1><div class='sub'>共 %d 个连接 · %s</div>",
		len(rows), generated.Format("2006-01-02 15:04:05")))

	// 第一部分：状态总表（连接名带锚点链接，跳到对应明细）
	b.WriteString("<h2>检查状态总表</h2>")
	b.WriteString("<table><tr><th>连接</th><th>类型</th><th>地址</th><th>状态</th><th>TCP延迟</th><th>SQL延迟</th><th>说明</th></tr>")
	for i, r := range rows {
		status, detail := "<span class='na'>未检测</span>", ""
		if r.checked {
			if r.res.OK {
				status = "<span class='ok'>正常</span>"
			} else {
				status = "<span class='err'>故障</span>"
				detail = html.EscapeString(r.res.Error)
			}
			fmt.Fprintf(&b, "<tr><td><a class='lnk' href='#conn-%d'>%s</a></td><td>%s</td><td>%s:%d</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
				i, html.EscapeString(r.conn.Name), html.EscapeString(typeName(r.conn.Type)),
				html.EscapeString(r.conn.Host), r.conn.Port, status, r.res.TCPMs, r.res.SQLMs, detail)
			continue
		}
		fmt.Fprintf(&b, "<tr><td><a class='lnk' href='#conn-%d'>%s</a></td><td>%s</td><td>%s:%d</td><td>%s</td><td>-</td><td>-</td><td></td></tr>",
			i, html.EscapeString(r.conn.Name), html.EscapeString(typeName(r.conn.Type)),
			html.EscapeString(r.conn.Host), r.conn.Port, status)
	}
	b.WriteString("</table>")

	// 第二部分：每个连接的实例概览
	b.WriteString("<h2>实例概览明细</h2>")
	for i, r := range rows {
		fmt.Fprintf(&b, "<h3 id='conn-%d'>%s %s <span class='dim small'>%s · %s:%d</span></h3>",
			i, typeIcon(r.conn.Type), html.EscapeString(r.conn.Name),
			html.EscapeString(typeName(r.conn.Type)), html.EscapeString(r.conn.Host), r.conn.Port)
		if r.checked && !r.res.OK {
			fmt.Fprintf(&b, "<div class='err-line'>连接故障：%s</div>", html.EscapeString(r.res.Error))
		}
		b.WriteString(overviewTable(r.ov))
	}

	b.WriteString(reportFoot())
	return b.String()
}

func renderTreeNode(b *strings.Builder, n *model.TNode, depth int) {
	cls := map[string]string{"ok": "ok", "warn": "warn", "err": "err"}[n.Status]
	if cls == "" {
		cls = "na"
	}
	b.WriteString(fmt.Sprintf("<div class='tnode d%d'><span class='dot %s'></span><b>%s</b> <span class='dim'>%s</span></div>",
		depth, cls, html.EscapeString(n.Label), html.EscapeString(n.Sub)))
	for _, ch := range n.Children {
		renderTreeNode(b, ch, depth+1)
	}
}

func svcTable(svcs []model.SvcInfo) string {
	var b strings.Builder
	b.WriteString("<table><tr><th>名称</th><th>角色</th><th>状态</th><th>主机</th><th>详情</th></tr>")
	for _, s := range svcs {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(s.Name), html.EscapeString(s.Role), html.EscapeString(s.Status),
			html.EscapeString(s.Host), html.EscapeString(s.Detail)))
	}
	b.WriteString("</table>")
	return b.String()
}

func tbsTable(tbs []model.TBSInfo) string {
	var b strings.Builder
	b.WriteString("<table><tr><th>名称</th><th>大小</th><th>剩余</th><th>状态</th><th>文件</th></tr>")
	for _, t := range tbs {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(t.Name), html.EscapeString(t.Size), html.EscapeString(t.Free),
			html.EscapeString(t.Status), html.EscapeString(t.Files)))
	}
	b.WriteString("</table>")
	return b.String()
}

func tableTable(ts []model.TableInfo) string {
	var b strings.Builder
	b.WriteString("<table><tr><th>模式</th><th>表名</th><th>行数</th><th>大小</th><th>备注</th></tr>")
	for _, t := range ts {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(t.Schema), html.EscapeString(t.Name), html.EscapeString(t.Rows),
			html.EscapeString(t.Size), html.EscapeString(t.Comment)))
	}
	b.WriteString("</table>")
	return b.String()
}

func typeIcon(typ string) string {
	if ti, err := provider.TypeInfo(typ); err == nil {
		return ti.Icon
	}
	return "🗄"
}

func typeName(typ string) string {
	if ti, err := provider.TypeInfo(typ); err == nil {
		return ti.Name
	}
	return typ
}

func reportHead(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8">
<title>%s</title><style>
body{font-family:"Microsoft YaHei",system-ui,sans-serif;max-width:980px;margin:24px auto;padding:0 16px;color:#1f2937;background:#f8fafc}
h1{font-size:22px;border-bottom:2px solid #2563eb;padding-bottom:8px}
h2{font-size:16px;color:#2563eb;margin-top:28px}
.sub{color:#6b7280;margin-bottom:16px}
table{border-collapse:collapse;width:100%%;margin:8px 0;background:#fff;font-size:13px}
th,td{border:1px solid #e5e7eb;padding:6px 10px;text-align:left}
th{background:#f1f5f9;font-weight:600}
table.kv th{width:220px;background:#f8fafc;color:#475569}
h3{font-size:15px;margin:24px 0 4px}
h3 .small{font-size:12px;font-weight:400}
.dim{color:#6b7280}
.err-line{color:#dc2626;font-size:13px;margin:6px 0}
a.lnk{color:#2563eb;text-decoration:none;border-bottom:1px dashed #93c5fd}
a.lnk:hover{border-bottom-style:solid}
.ok{color:#16a34a;font-weight:600}.err{color:#dc2626;font-weight:600}.warn{color:#d97706}.na{color:#94a3b8}
.dim{color:#94a3b8;font-size:12px}
.tree{background:#fff;border:1px solid #e5e7eb;padding:12px;font-size:13px;line-height:1.9}
.dot{display:inline-block;width:9px;height:9px;border-radius:50%%;margin-right:6px}
.dot.ok{background:#22c55e}.dot.err{background:#ef4444}.dot.warn{background:#f59e0b}.dot.na{background:#cbd5e1}
.warn{background:#fffbeb;border:1px solid #fde68a;padding:10px;margin:10px 0;font-size:13px;color:#92400e;border-radius:6px}
.pre-block{font-family:Consolas,"Courier New",monospace;font-size:12px;line-height:1.6;background:#f1f5f9;border:1px solid #e5e7eb;border-radius:8px;padding:9px 11px;white-space:pre-wrap;word-break:break-all;margin:6px 0 12px;user-select:text}
.footer{margin-top:32px;color:#9ca3af;font-size:12px;text-align:center}
</style></head><body>`, html.EscapeString(title))
}

func reportFoot() string {
	return fmt.Sprintf("<div class='footer'>HostPing v%s 主机巡检与分析 · 生成于 %s</div></body></html>",
		AppVersion, time.Now().Format("2006-01-02 15:04:05"))
}

func writeHTML(path, htmlStr string) error {
	return os.WriteFile(path, []byte(htmlStr), 0o644)
}




