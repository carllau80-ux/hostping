package app

// ai.go AI 分析：巡检数据摘要 → 国内主流大模型（OpenAI 兼容接口）→ Markdown → 格式化 HTML。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"dbping/internal/model"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AIProviders 内置提供商 → 默认 Base URL / 默认模型（前端下拉据此预填，后端仅用 BaseURL+Model 调用）。
var AIProviders = map[string][2]string{
	"zhipu":    {"https://open.bigmodel.cn/api/paas/v4", "glm-4-flash"},
	"deepseek": {"https://api.deepseek.com", "deepseek-chat"},
	"qwen":     {"https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen-plus"},
	"moonshot": {"https://api.moonshot.cn/v1", "moonshot-v1-8k"},
	"volces":   {"https://ark.cn-beijing.volces.com/api/v3", "doubao-1.5-pro-32k"},
	"qianfan":  {"https://qianfan.baidubce.com/v2", "ernie-4.0-8k-latest"},
}

// GetVersion 返回应用版本号。
func (a *App) GetVersion() string { return AppVersion }

// GetAIConfig 返回 AI 配置（Key 不回传，仅告知是否已配置）。
func (a *App) GetAIConfig() map[string]any {
	st := a.store()
	return map[string]any{
		"provider": st.Settings.AIProvider,
		"model":    st.Settings.AIModel,
		"baseUrl":  st.Settings.AIBaseURL,
		"hasKey":   st.AIKey() != "",
	}
}

// SaveAIConfig 保存 AI 配置（apiKey 留空表示保持原值）。
func (a *App) SaveAIConfig(provider, modelName, baseURL, apiKey string) error {
	st := a.store()
	st.SaveSettings(model.Settings{
		AutoPing:     st.Settings.AutoPing,
		AdvancedMode: st.Settings.AdvancedMode,
		IntervalSec:  st.Settings.IntervalSec,
		PingTimeout:  st.Settings.PingTimeout,
		AIProvider:   provider,
		AIModel:      modelName,
		AIBaseURL:    baseURL,
	})
	if apiKey != "" {
		st.SetAIKey(apiKey)
	}
	if err := st.Save(); err != nil {
		return err
	}
	log.Printf("[ai] 配置已保存: provider=%s model=%s", provider, modelName)
	return nil
}

// AIAnalyzeHost 分析主机巡检报告（reportJSON 为前端传来的 HostReport JSON）。
// 返回 {markdown, html, model, elapsedMs}。
func (a *App) AIAnalyzeHost(reportJSON string) (map[string]any, error) {
	base, key, modelName, err := a.aiConfig()
	if err != nil {
		return nil, err
	}
	var r model.HostReport
	if err := json.Unmarshal([]byte(reportJSON), &r); err != nil {
		return nil, fmt.Errorf("巡检报告数据无效: %w", err)
	}
	prompt := hostReportDigest(&r)
	start := time.Now()
	md, err := aiChat(base, key, modelName, aiSystemPrompt("host"), prompt, 180*time.Second)
	if err != nil {
		log.Printf("[ai] 分析失败 %s: %v", r.Name, err)
		return nil, err
	}
	elapsed := time.Since(start).Milliseconds()
	log.Printf("[ai] 分析完成 %s: model=%s, %dms, %d 字", r.Name, modelName, elapsed, len([]rune(md)))
	return map[string]any{
		"markdown":  md,
		"html":      mdToHTML(md),
		"model":     modelName,
		"elapsedMs": elapsed,
		"time":      time.Now().UnixMilli(),
	}, nil
}

// AIAnalyzeOverview 分析数据库巡检概览（ovJSON 为前端传来的 Overview JSON，id 用于补充连接信息）。
func (a *App) AIAnalyzeOverview(id, ovJSON string) (map[string]any, error) {
	base, key, modelName, err := a.aiConfig()
	if err != nil {
		return nil, err
	}
	var ov model.Overview
	if err := json.Unmarshal([]byte(ovJSON), &ov); err != nil {
		return nil, fmt.Errorf("概览数据无效: %w", err)
	}
	c, connOK := a.store().GetConn(id)
	prompt := overviewDigest(c, connOK, &ov)

	start := time.Now()
	md, err := aiChat(base, key, modelName, aiSystemPrompt("db"), prompt, 180*time.Second)
	if err != nil {
		log.Printf("[ai] 数据库分析失败: %v", err)
		return nil, err
	}
	elapsed := time.Since(start).Milliseconds()
	log.Printf("[ai] 数据库分析完成: model=%s, %dms, %d 字", modelName, elapsed, len([]rune(md)))
	return map[string]any{
		"markdown":  md,
		"html":      mdToHTML(md),
		"model":     modelName,
		"elapsedMs": elapsed,
		"time":      time.Now().UnixMilli(),
	}, nil
}

// aiConfig 校验并返回 AI 调用参数（本地/自定义提供商允许空 Key）。
func (a *App) aiConfig() (base, key, modelName string, err error) {
	st := a.store()
	base = strings.TrimRight(st.Settings.AIBaseURL, "/")
	key = st.AIKey()
	modelName = st.Settings.AIModel
	prov := st.Settings.AIProvider
	needKey := prov != "local" && prov != "custom"
	switch {
	case base == "" || modelName == "":
		return "", "", "", fmt.Errorf("未配置 AI：请先在「⚙ 设置 → AI 分析设置」中选择提供商并填写 API Key")
	case needKey && key == "":
		return "", "", "", fmt.Errorf("提供商 %s 需要 API Key，请先在设置中填写", prov)
	}
	return base, key, modelName, nil
}

// AttachHostAI 将 AI 分析结果挂到主机巡检历史（按 CollectedAt 匹配），并持久化。
func (a *App) AttachHostAI(id, reportJSON, markdown string) error {
	st := a.store()
	var r model.HostReport
	if err := json.Unmarshal([]byte(reportJSON), &r); err != nil {
		return fmt.Errorf("巡检报告数据无效: %w", err)
	}
	ai := model.AIAnalysis{
		Markdown: markdown,
		HTML:     mdToHTML(markdown),
		Model:    st.Settings.AIModel,
		Time:     time.Now().UnixMilli(),
	}
	st.AttachHostAI(id, r, ai)
	return st.Save()
}

// GetHostHistory 返回主机巡检历史（最新在前，含已保存的 AI 分析）。
func (a *App) GetHostHistory(id string) []model.HostHistoryEntry {
	return a.store().GetHostHistory(id)
}

// AddDBAIAnalysis 保存数据库 AI 分析到历史（含概览快照），并持久化。
func (a *App) AddDBAIAnalysis(id, ovJSON, markdown string) error {
	st := a.store()
	ai := model.AIAnalysis{
		Markdown: markdown,
		HTML:     mdToHTML(markdown),
		Model:    st.Settings.AIModel,
		Time:     time.Now().UnixMilli(),
	}
	e := model.DBAIEntry{Time: ai.Time, ConnID: id, OverviewJSON: ovJSON, AI: ai}
	if c, ok := st.GetConn(id); ok {
		e.ConnName = c.Name
		e.ConnType = c.Type
	}
	st.AddDBAIEntry(e)
	return st.Save()
}

// GetDBAIHistory 返回数据库 AI 分析历史（最新在前）。
func (a *App) GetDBAIHistory() []model.DBAIEntry {
	return a.store().GetDBAIHistory()
}

// AIExportOverviewAnalysis 将数据库 AI 分析导出为格式化 HTML 报告，返回保存路径。
func (a *App) AIExportOverviewAnalysis(id, ovJSON, markdown string) (string, error) {
	st := a.store()
	c, ok := st.GetConn(id)
	name := "数据库"
	if ok {
		name = c.Name
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出 AI 分析报告",
		DefaultFilename: fmt.Sprintf("HostPing_AI分析_%s_%s.html", sanitizeName(name), time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	var b strings.Builder
	b.WriteString(reportHead("HostPing AI 分析报告 — " + html.EscapeString(name)))
	sub := name
	if ok {
		sub = fmt.Sprintf("%s · %s (%s:%d/%s)", name, c.Type, c.Host, c.Port, c.Database)
	}
	b.WriteString(fmt.Sprintf("<h1>🤖 AI 分析报告 — %s</h1><div class='sub'>巡检对象 %s · 分析模型 %s · %s</div>",
		html.EscapeString(name), html.EscapeString(sub), html.EscapeString(st.Settings.AIModel), time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString(mdToHTML(markdown))
	b.WriteString(reportFoot())
	if err := writeHTML(path, b.String()); err != nil {
		return "", err
	}
	log.Printf("[ai] 数据库分析报告已导出: %s", path)
	return path, nil
}

// AIExportHostAnalysis 将 AI 分析导出为格式化 HTML 报告，返回保存路径。
func (a *App) AIExportHostAnalysis(reportJSON, markdown string) (string, error) {
	st := a.store()
	var r model.HostReport
	_ = json.Unmarshal([]byte(reportJSON), &r) // 名称仅用于文件名，失败也可继续
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出 AI 分析报告",
		DefaultFilename: fmt.Sprintf("HostPing_AI分析_%s_%s.html", sanitizeName(r.Name), time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	var b strings.Builder
	b.WriteString(reportHead("HostPing AI 分析报告 — " + html.EscapeString(r.Name)))
	b.WriteString(fmt.Sprintf("<h1>🤖 AI 分析报告 — %s</h1><div class='sub'>巡检对象 %s (%s) · 分析模型 %s · %s</div>",
		html.EscapeString(r.Name), html.EscapeString(r.Name), html.EscapeString(r.Host),
		html.EscapeString(st.Settings.AIModel), time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString(mdToHTML(markdown))
	b.WriteString(reportFoot())
	if err := writeHTML(path, b.String()); err != nil {
		return "", err
	}
	log.Printf("[ai] 分析报告已导出: %s", path)
	return path, nil
}

// ---------- 大模型调用（OpenAI 兼容 chat/completions） ----------

func aiSystemPrompt(kind string) string {
	if kind == "db" {
		return "你是资深数据库运维专家，负责分析 HostPing 巡检工具采集的数据库巡检概览数据（JSON 格式，字段含义：version 版本、role 主库/从库/备库、archiveMode 归档模式、charset 字符集、connCur/connMax 当前与最大连接数、dbSize 数据库大小、tableCount 表数量、cluster 集群形态、extra 类型特有信息等）。" +
			"请基于数据输出一份简体中文分析报告，使用 Markdown 格式，结构如下：" +
			"## 总体结论（正常 / 关注 / 风险，一句话说明）；" +
			"## 关键发现（按严重程度排序，引用具体数值作为依据）；" +
			"## 逐项分析（版本与生命周期、主备与归档配置、连接数水位、容量与表规模、字符集与时区、类型特有项等，仅分析有数据的字段，无数据的字段跳过）；" +
			"## 处置建议（可执行的措施清单）。\n" +
			"严格要求：只依据给出的数据分析，不得编造；常见关注点包括：连接数接近上限、未开启归档的主库、字符集不常见、长期未重启、警告提示等；语气专业简洁。"
	}
	return "你是资深系统运维与数据库专家，负责分析 HostPing 巡检工具采集的主机巡检数据。" +
		"请基于提供的数据输出一份简体中文分析报告，使用 Markdown 格式，结构如下：" +
		"## 总体结论（正常 / 关注 / 风险，一句话说明）；" +
		"## 关键发现（按严重程度排序，引用具体数值作为依据，无异常则说明检查项均正常）；" +
		"## 逐项分析（系统资源、磁盘、网络端口、RAID、ZFS、安全、系统日志、重启记录等，仅分析有数据的项，无数据的项跳过）；" +
		"## 处置建议（可执行的措施清单）。\n" +
		"严格要求：只依据给出的数据分析，不得编造；数值异常需说明判断依据（如内存使用率、磁盘水位、Optl/Onln 状态、错误日志条数）；" +
		"语气专业简洁，避免空话。"
}

// overviewDigest 数据库巡检概览摘要（连接信息 + 概览 JSON）。
func overviewDigest(c model.Conn, ok bool, ov *model.Overview) string {
	var b strings.Builder
	title := "未知连接"
	typ := "-"
	if ok {
		title = fmt.Sprintf("%s (%s:%d/%s)", c.Name, c.Host, c.Port, c.Database)
		typ = c.Type
	}
	fmt.Fprintf(&b, "# 数据库巡检概览：%s\n- 类型：%s\n- 连接：%s\n\n", title, typ, title)
	j, _ := json.MarshalIndent(ov, "", "  ")
	s := string(j)
	if len(s) > 8000 {
		s = s[:8000] + "\n（数据过长已截断）"
	}
	b.WriteString("```json\n" + s + "\n```\n")
	if len(ov.Warnings) > 0 {
		b.WriteString("\n巡检警告：\n")
		for _, w := range ov.Warnings {
			b.WriteString("- " + w + "\n")
		}
	}
	return b.String()
}

// aiChat 调用 OpenAI 兼容 chat/completions 接口（key 为空时不带鉴权头，适配本地模型）。
func aiChat(base, key, model, system, user string, timeout time.Duration) (string, error) {
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.3,
		"stream":      false,
	}
	buf, _ := json.Marshal(reqBody)
	url := base + "/chat/completions"
	hreq, err := http.NewRequest("POST", url, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	hreq.Header.Set("Content-Type", "application/json")
	if key != "" {
		hreq.Header.Set("Authorization", "Bearer "+key)
	}
	cli := &http.Client{Timeout: timeout}
	resp, err := cli.Do(hreq)
	if err != nil {
		return "", fmt.Errorf("请求大模型失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("大模型返回 %d: %s", resp.StatusCode, excerpt(string(body), 300))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("大模型错误: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("大模型未返回内容")
	}
	return out.Choices[0].Message.Content, nil
}

// ---------- 巡检报告摘要（发给大模型的紧凑文本） ----------

const digestCap = 12000 // 摘要字符上限（适配 8k 上下文模型）

func hostReportDigest(r *model.HostReport) string {
	var b strings.Builder
	w := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
	}
	w("# 主机巡检数据：%s (%s:%d，SSH 用户 %s)\n\n", r.Name, r.Host, r.Port, r.User)
	w("- 操作系统：%s\n", r.OSName)
	for _, kv := range r.SysInfo {
		if strings.HasPrefix(kv.Key, "__") {
			continue
		}
		w("- %s：%s\n", kv.Key, kv.Value)
	}
	for _, kv := range r.SysInfo {
		switch kv.Key {
		case "__df":
			w("\n## 磁盘挂载\n```\n%s\n```\n", trimText(kv.Value, 1200))
		case "__ip":
			w("\n## 网络接口\n```\n%s\n```\n", trimText(kv.Value, 800))
		}
	}
	if len(r.ListenPorts) > 0 {
		w("\n## 监听端口\n| 端口 | 状态 | 服务 | 进程 | 连接 |\n|---|---|---|---|---|\n")
		for _, p := range r.ListenPorts {
			w("| %s | %s | %s | %s | %s |\n", p.Port, p.State, p.Service, p.Process, p.Conns)
		}
	}
	if r.Storcli != nil {
		for _, c := range r.Storcli.Controllers {
		w("\n## RAID 控制器 %s（%s，状态 %s）\n", c.Index, c.Model, c.Status)
		w("- 命令状态：%s · 控制器状态：%s · 固件：%s · 温度：%s · 缓存内存：%s · 电池：%s\n", orAI(c.Status), orAI(c.CtlStatus), orAI(c.Firmware), orAI(c.Temp), orAI(c.Memory), orAI(c.Battery))
		if c.BatteryDetail != "" {
			w("- 电池/CacheVault 详情：%s\n", c.BatteryDetail)
		}
		for _, v := range c.VDs {
			w("- VD %s 名称=%s 级别=%s 状态=%s 容量=%s 缓存=%s 条带=%s Span=%s\n", v.ID, v.Name, v.Type, v.State, v.Size, v.Cache, v.Strip, v.Spans)
		}
		for _, p := range c.PDs {
			w("- PD %s 型号=%s 状态=%s 容量=%s 介质=%s 接口=%s 温度=%s SN=%s 通电=%s小时 %s\n", p.Slot, p.Model, p.State, p.Size, p.MediaType, p.Interface, p.Temp, p.SN, p.Power, p.Detail)
		}
		}
	} else if r.StorcliNote != "" {
		w("\n## RAID\n- %s\n", r.StorcliNote)
	}
	if r.ZFS != nil && len(r.ZFS.Pools) > 0 {
		w("\n## ZFS 存储池\n| %s |\n", strings.Join(r.ZFS.PoolCols, " | "))
		for _, row := range r.ZFS.Pools {
			w("| %s |\n", strings.Join(row, " | "))
		}
		if len(r.ZFS.Datasets) > 0 {
			w("\n### 数据集\n| %s |\n", strings.Join(r.ZFS.DSectCols, " | "))
			for _, row := range r.ZFS.Datasets {
				w("| %s |\n", strings.Join(row, " | "))
			}
		}
	} else if r.ZFSNote != "" {
		w("\n## ZFS\n- %s\n", r.ZFSNote)
	}
	if len(r.TopCPU) > 0 {
		w("\n## Top 进程（按 CPU，内存为 RSS 换算 MB）\n")
		for _, p := range r.TopCPU {
			w("- PID %s %s CPU %s%% MEM %s%% 内存 %s 启动 %s 运行 %s：%s\n", p.PID, p.User, p.CPU, p.MEM, kbToMB(p.RSS), p.Start, p.ETime, trimText(p.Cmd, 120))
		}
	}
	if len(r.TopMem) > 0 {
		w("\n## Top 进程（按内存）\n")
		for _, p := range r.TopMem {
			w("- PID %s %s CPU %s%% MEM %s%% 内存 %s：%s\n", p.PID, p.User, p.CPU, p.MEM, kbToMB(p.RSS), trimText(p.Cmd, 120))
		}
	}
	for _, p := range r.RSProcs {
		w("\n## rs_/rsa_ 进程 %s (PID %s, %s)\n", baseName(p.Cmd), p.PID, p.User)
		w("- CPU %s%% MEM %s%% 内存 %s 运行 %s FD %s\n", p.CPU, p.MEM, kbToMB(p.RSS), p.ETime, p.FDCount)
		if len(p.OpenFiles) > 0 {
			w("- 打开文件：%s\n", trimText(strings.Join(p.OpenFiles, " ; "), 600))
		}
		if len(p.Conns) > 0 {
			w("- 网络连接：\n")
			for _, c := range p.Conns {
				w("  - %s\n", trimText(c, 160))
			}
		}
	}
	if r.Security != nil {
		w("\n## 安全状态\n")
		for _, kv := range r.Security {
			if strings.HasPrefix(kv.Key, "__") {
				w("```\n%s\n```\n", trimText(kv.Value, 600))
				continue
			}
			w("- %s：%s\n", kv.Key, kv.Value)
		}
	}
	if r.SysLog != nil {
		w("\n## 系统日志（24h，来源 %s）\n- 错误 %s 条 · 警告 %s 条\n", r.SysLog.Source, r.SysLog.ErrCount, r.SysLog.WarnCount)
		for _, l := range r.SysLog.TopErrors {
			w("  - %s\n", trimText(l, 160))
		}
		if len(r.SysLog.DmesgErrs) > 0 {
			w("- dmesg 错误：\n")
			for _, l := range r.SysLog.DmesgErrs {
				w("  - %s\n", trimText(l, 160))
			}
		}
	}
	if len(r.Reboots) > 0 {
		w("\n## 重启记录（最近 %d 次）\n", len(r.Reboots))
		for i, rb := range r.Reboots {
			w("- %d. 启动 %s 结束 %s 时长 %s\n", i+1, rb.Start, rb.End, rb.Dur)
		}
	}
	if len(r.Warnings) > 0 {
		w("\n## 巡检警告\n")
		for _, warn := range r.Warnings {
			w("- %s\n", warn)
		}
	}
	out := b.String()
	if len(out) > digestCap {
		out = out[:digestCap] + "\n\n（数据过长已截断）"
	}
	return out
}

func orAI(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func trimText(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func excerpt(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func sanitizeName(name string) string {
	repl := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return repl.Replace(strings.TrimSpace(name))
}

// ---------- Markdown → HTML（轻量转换，覆盖 LLM 常用语法） ----------

var (
	reH      = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	reUl     = regexp.MustCompile(`^\s*[-*]\s+(.*)$`)
	reOl     = regexp.MustCompile(`^\s*\d+[.)]\s+(.*)$`)
	reBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reCode   = regexp.MustCompile("`([^`]+)`")
	reTrimLn = regexp.MustCompile(`^\s*\|?[\s:|-]+\|?\s*$`)
)

// mdToHTML 将 LLM 输出的 Markdown 转为 HTML 片段（标题/列表/表格/代码块/加粗/行内代码/引用）。
func mdToHTML(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var b strings.Builder
	inCode, inTable := false, false
	tableHasHeader := false
	listTag := "" // "ul" / "ol"
	para := ""
	closePara := func() {
		if para != "" {
			b.WriteString("<p>" + inlineMD(para) + "</p>")
			para = ""
		}
	}
	closeList := func() {
		if listTag != "" {
			b.WriteString("</" + listTag + ">")
			listTag = ""
		}
	}
	closeTable := func() {
		if inTable {
			if !tableHasHeader && b.Len() > 0 {
				// 纯数据表也包一层（无表头则不做特殊处理）
			}
			b.WriteString("</tbody></table>")
			inTable, tableHasHeader = false, false
		}
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		// 代码块
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			closePara()
			closeList()
			closeTable()
			if inCode {
				b.WriteString("</code></pre>")
				inCode = false
			} else {
				b.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			b.WriteString(html.EscapeString(raw) + "\n")
			continue
		}
		// 表格
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			closePara()
			closeList()
			cols := splitMDRow(strings.TrimSpace(line))
			if reTrimLn.MatchString(line) || allDashCols(cols) {
				if inTable && !tableHasHeader {
					// 已写入表头行
				}
				tableHasHeader = true // 分隔行之后是数据行
				continue
			}
			if !inTable {
				b.WriteString("<table><thead><tr>")
				for _, c := range cols {
					b.WriteString("<th>" + inlineMD(c) + "</th>")
				}
				b.WriteString("</tr></thead><tbody>")
				inTable, tableHasHeader = true, false
				continue
			}
			b.WriteString("<tr>")
			for _, c := range cols {
				b.WriteString("<td>" + inlineMD(c) + "</td>")
			}
			b.WriteString("</tr>")
			continue
		}
		closeTable()
		// 标题
		if m := reH.FindStringSubmatch(line); m != nil {
			closePara()
			closeList()
			level := len(m[1]) + 1 // # → h2
			if level > 6 {
				level = 6
			}
			b.WriteString(fmt.Sprintf("<h%d>%s</h%d>", level, inlineMD(m[2]), level))
			continue
		}
		// 引用
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			closePara()
			closeList()
			b.WriteString("<blockquote>" + inlineMD(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ">"))) + "</blockquote>")
			continue
		}
		// 列表
		if m := reUl.FindStringSubmatch(line); m != nil {
			closePara()
			if listTag != "ul" {
				closeList()
				b.WriteString("<ul>")
				listTag = "ul"
			}
			b.WriteString("<li>" + inlineMD(m[1]) + "</li>")
			continue
		}
		if m := reOl.FindStringSubmatch(line); m != nil {
			closePara()
			if listTag != "ol" {
				closeList()
				b.WriteString("<ol>")
				listTag = "ol"
			}
			b.WriteString("<li>" + inlineMD(m[1]) + "</li>")
			continue
		}
		// 空行 / 普通文本
		if strings.TrimSpace(line) == "" {
			closePara()
			closeList()
			continue
		}
		// 分隔线
		if reTrimLn.MatchString(line) && strings.Contains(line, "---") {
			closePara()
			closeList()
			b.WriteString("<hr/>")
			continue
		}
		if para != "" {
			para += " " + strings.TrimSpace(line)
		} else {
			para = strings.TrimSpace(line)
		}
	}
	closePara()
	closeList()
	closeTable()
	if inCode {
		b.WriteString("</code></pre>")
	}
	return b.String()
}

// inlineMD 行内格式：加粗 + 行内代码（先转义 HTML）。
func inlineMD(s string) string {
	s = html.EscapeString(s)
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reCode.ReplaceAllString(s, "<code>$1</code>")
	return s
}

func splitMDRow(line string) []string {
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func allDashCols(cols []string) bool {
	if len(cols) == 0 {
		return false
	}
	for _, c := range cols {
		t := strings.TrimSpace(c)
		if t != "" && !strings.HasPrefix(t, ":") && !strings.HasPrefix(t, "-") {
			return false
		}
	}
	return true
}
