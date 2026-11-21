package app

import (
	"strings"
	"testing"
	"time"

	"dbping/internal/model"
)

func TestMdToHTML(t *testing.T) {
	md := "## 总体结论\n\n**正常**，系统运行平稳。\n\n## 关键发现\n\n- 内存使用率 `62%`，正常\n- 磁盘 `/` 使用 `85%`，**建议关注**\n\n| 项目 | 数值 |\n|---|---|\n| CPU | 32.9% |\n| 内存 | 2.3 GB |\n\n```bash\ndf -h\n```\n\n> 引用：数据来自巡检\n\n1. 第一步\n2. 第二步\n"
	h := mdToHTML(md)
	musts := []string{
		"<h3>总体结论</h3>",
		"<strong>正常</strong>",
		"<ul><li>内存使用率 <code>62%</code>",
		"<strong>建议关注</strong>",
		"<table><thead><tr><th>项目</th><th>数值</th></tr></thead><tbody><tr><td>CPU</td><td>32.9%</td>",
		"<pre><code>df -h\n</code></pre>",
		"<blockquote>引用：数据来自巡检</blockquote>",
		"<ol><li>第一步</li><li>第二步</li></ol>",
	}
	for _, s := range musts {
		if !strings.Contains(h, s) {
			t.Errorf("缺少: %q\n实际输出: %s", s, h)
		}
	}
	if strings.Contains(h, "**") || strings.Contains(h, "```") {
		t.Errorf("Markdown 标记残留: %s", h)
	}
}

func TestMdToHTMLEscape(t *testing.T) {
	h := mdToHTML("文本 <script>alert(1)</script> 与 **粗体**")
	if strings.Contains(h, "<script>") {
		t.Errorf("HTML 未转义: %s", h)
	}
	if !strings.Contains(h, "&lt;script&gt;") {
		t.Errorf("转义结果缺失: %s", h)
	}
}

func TestHostReportDigest(t *testing.T) {
	r := &model.HostReport{
		Name: "pve", Host: "192.168.31.58", Port: 22, User: "root",
		OSName: "Debian 12",
		SysInfo: []model.KV{
			{Key: "CPU 核数", Value: "4"},
			{Key: "__df", Value: "Filesystem Type Size Used\n/dev/sda1 ext4 100G 20G"},
		},
		ListenPorts: []model.PortInfo{{Port: "22", State: "LISTEN", Service: "SSH", Process: "sshd", Conns: "ESTAB 1"}},
		TopCPU:      []model.ProcInfo{{PID: "1", User: "root", CPU: "32.9", MEM: "0.2", RSS: "589904", Cmd: "systemd"}},
		Storcli: &model.StorcliReport{Controllers: []model.StorcliCtl{{
			Index: "0", Model: "9361-8i", Status: "Success", Firmware: "5.13", Temp: "45C", Memory: "2048 MB", Battery: "Operational",
			VDs: []model.StorcliVD{{ID: "0", Type: "RAID-10", State: "Optl", Size: "3.5 TB", Cache: "RWBD"}},
			PDs: []model.StorcliPD{{Slot: "8:0", Model: "ST18000", State: "Onln", Size: "1.75 TB", SN: "ZL1", Power: "30241"}},
		}}},
		Warnings: []string{"未发现 rs_/rsa_ 前缀进程"},
	}
	d := hostReportDigest(r)
	musts := []string{"主机巡检数据：pve", "Debian 12", "CPU 核数：4", "磁盘挂载", "/dev/sda1", "监听端口", "576.1 MB", "RAID 控制器 0", "5.13", "45C", "ZL1", "30241", "巡检警告"}
	for _, s := range musts {
		if !strings.Contains(d, s) {
			t.Errorf("摘要缺少: %q", s)
		}
	}
}

func TestSplitMDRow(t *testing.T) {
	cols := splitMDRow("| a | b | c |")
	if len(cols) != 3 || cols[0] != "a" || cols[2] != "c" {
		t.Errorf("splitMDRow = %v", cols)
	}
	if !allDashCols(splitMDRow("|---|:---:|")) {
		t.Errorf("分隔行未识别")
	}
}

func TestRenderCombinedHost(t *testing.T) {
	h := model.SSHHost{Name: "pve", Host: "192.168.31.58", Port: 22, User: "root"}
	r := &model.HostReport{Name: "pve", Host: "192.168.31.58", OSName: "Debian 12", CollectedAt: time.Now().UnixMilli()}
	// 带 AI：应含目录两链接与两个锚点
	doc := renderCombinedHost(h, r, "## 结论\n正常", "glm-4-flash")
	musts := []string{
		`href="#sec-inspect"`, `href="#sec-ai"`, `id="sec-inspect"`, `id="sec-ai"`,
		"AI 分析报告", "glm-4-flash", "<h3>结论</h3>",
		".pre-block{", // 导出报告内嵌样式：pre-block 换行显示
	}
	for _, s := range musts {
		if !strings.Contains(doc, s) {
			t.Errorf("综合报告缺少: %q", s)
		}
	}
	if strings.Count(doc, "<body") != 1 {
		t.Errorf("bodyOnly 未正确剥离内层文档: body 标签数 %d", strings.Count(doc, "<body"))
	}
	// 无 AI：只有巡检链接
	doc2 := renderCombinedHost(h, r, "", "")
	if strings.Contains(doc2, `href="#sec-ai"`) || strings.Contains(doc2, `id="sec-ai"`) {
		t.Errorf("无 AI 时不应有 AI 锚点")
	}
	if !strings.Contains(doc2, `href="#sec-inspect"`) {
		t.Errorf("缺少巡检锚点")
	}
}

func TestBodyOnly(t *testing.T) {
	doc := "<!DOCTYPE html><html><head><style>p{}</style></head><body><h1>标题</h1><p>内容</p></body></html>"
	got := bodyOnly(doc)
	if strings.Contains(got, "<body") || strings.Contains(got, "<head") || strings.Contains(got, "</body>") {
		t.Errorf("bodyOnly 残留外壳: %s", got)
	}
	if !strings.Contains(got, "<h1>标题</h1>") {
		t.Errorf("bodyOnly 内容缺失: %s", got)
	}
}
