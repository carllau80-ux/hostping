// Package app Wails 绑定层：向前端暴露全部 API。
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dbping/internal/bmc"
	"dbping/internal/config"
	"dbping/internal/hostinsp"
	"dbping/internal/model"
	"dbping/internal/monitor"
	"dbping/internal/provider"
	"dbping/internal/winicon"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 应用主体。
type App struct {
	ctx        context.Context
	cfg        *config.Store
	mon        *monitor.Monitor
	autoCancel context.CancelFunc
	mu         sync.Mutex
}

// New 创建应用。
func New() *App { return &App{} }

// store 取配置仓库（惰性初始化：前端绑定调用可能早于 OnStartup 完成，
// 此时 a.cfg 尚为 nil，直接解引用会 panic 导致前端拿到空数据）。
func (a *App) store() *config.Store {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		cfg, err := config.Load()
		if err != nil {
			log.Printf("加载配置失败（使用空配置）: %v", err)
			cfg, _ = config.Load()
		}
		a.cfg = cfg
		initFileLog(cfg.DirPath)
		log.Printf("[app] 配置仓库就绪(惰性): %d 个连接", len(cfg.ListConns()))
		if a.mon == nil {
			a.mon = monitor.New(a.onPingResults)
		}
	}
	return a.cfg
}

// mon 取巡检器（随 store 惰性创建）。
func (a *App) monInst() *monitor.Monitor {
	a.store()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mon == nil {
		a.mon = monitor.New(a.onPingResults)
	}
	return a.mon
}

// Startup Wails 启动回调。
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	// 复用惰性初始化（若前端绑定调用早于本回调，store() 已完成加载，此处直接复用，
	// 避免 Store/Monitor 双实例造成状态分裂）
	cfg := a.store()
	initFileLog(cfg.DirPath)
	log.Printf("[app] 启动: %d 个连接, 自动巡检=%v, 高级模式(会话)=%v",
		len(cfg.ListConns()), cfg.Settings.AutoPing, cfg.Settings.AdvancedMode)
	// 主窗口/任务栏图标（窗口类不读取 exe 资源图标，需运行时注入）
	if err := winicon.SetMainWindowIcon(); err != nil {
		log.Printf("设置窗口图标: %v", err)
	}
	if cfg.Settings.AutoPing {
		a.startAuto()
	}
}

// initFileLog 将标准日志路由到 %APPDATA%\DBPing\app.log（超过 2MB 截断）。
func initFileLog(dir string) {
	const maxLog = 2 << 20
	path := filepath.Join(dir, "app.log")
	if st, err := os.Stat(path); err == nil && st.Size() > maxLog {
		_ = os.Truncate(path, 0)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
}

// Shutdown Wails 退出回调。
func (a *App) Shutdown(context.Context) {
	if a.autoCancel != nil {
		a.autoCancel()
	}
}

func (a *App) onPingResults(results []model.PingResult) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "ping:update", results)
	}
}

// ---------- 类型与连接 ----------

// GetTypes 返回支持的数据库类型清单。
func (a *App) GetTypes() []model.TypeInfo { return providerTypeInfos() }

// GetConnections 返回连接列表（密码脱敏）。
func (a *App) GetConnections() []model.Conn {
	out := a.store().ListConns()
	for i := range out {
		out[i] = out[i].Masked()
	}
	return out
}

// GetConfigDir 返回配置目录（便于用户备份）。
func (a *App) GetConfigDir() string { return a.store().DirPath }

// OpenConfigDir 打开配置目录（资源管理器）。
func (a *App) OpenConfigDir() error {
	return exec.Command("explorer", a.store().DirPath).Start()
}

// SaveConnection 新增/更新连接。
func (a *App) SaveConnection(c model.Conn) error {
	if c.Host == "" {
		return fmt.Errorf("主机不能为空")
	}
	if c.Type == "" {
		return fmt.Errorf("数据库类型不能为空")
	}
	if err := a.store().UpsertConn(c); err != nil {
		return err
	}
	return a.store().Save()
}

// DeleteConnection 删除连接。
func (a *App) DeleteConnection(id string) error {
	a.store().DeleteConn(id)
	return a.store().Save()
}

// ---------- 巡检 ----------

// TestConnection 测试一个（可能尚未保存的）连接。
func (a *App) TestConnection(c model.Conn) model.PingResult {
	// 更新已有连接时密码留空 → 取存储的旧密码
	if c.Password == "" && c.ID != "" {
		if old, ok := a.store().GetConn(c.ID); ok {
			c.Password = old.Password
		}
	}
	return a.monInst().PingOne(&c)
}

// PingOne 检查单个已保存连接。
func (a *App) PingOne(id string) (model.PingResult, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return model.PingResult{}, fmt.Errorf("连接不存在")
	}
	return a.monInst().PingOne(&c), nil
}

// PingAll 并发检查全部连接。
func (a *App) PingAll() []model.PingResult {
	return a.monInst().PingAll(a.store().ListConns())
}

// GetLatestResults 返回每连接最近一次巡检结果。
func (a *App) GetLatestResults() map[string]model.PingResult { return a.monInst().Latest() }

// GetHistory 返回检查历史。
func (a *App) GetHistory(id string) []model.PingResult { return a.monInst().GetHistory(id) }

// ---------- 元数据 ----------

// GetOverview 采集实例概览。
func (a *App) GetOverview(id string) (*model.Overview, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return nil, fmt.Errorf("连接不存在")
	}
	return a.monInst().GetOverview(&c)
}

// GetDetail 采集明细。
func (a *App) GetDetail(id string) (*model.Detail, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return nil, fmt.Errorf("连接不存在")
	}
	return a.monInst().GetDetail(&c)
}

// GetTopology 采集拓扑。
func (a *App) GetTopology(id string) (*model.Topology, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return nil, fmt.Errorf("连接不存在")
	}
	return a.monInst().GetTopology(&c)
}

// ---------- 高级模式：SQL 控制台与表分页 ----------

// ExecSQL 执行 SQL（SELECT 自动 COUNT+分页封装；DML/DDL 返回影响行数）。
func (a *App) ExecSQL(id, sqlText string, page, size int) (*model.SQLResult, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return nil, fmt.Errorf("连接不存在")
	}
	return provider.ExecSQL(&c, sqlText, page, size)
}

// ListTables 数据表元数据分页（按数据库/模式 + 表名关键字筛选）。
func (a *App) ListTables(id, schema, filter string, page, size int) (*model.TablePage, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return nil, fmt.Errorf("连接不存在")
	}
	return provider.ListTablesPaged(&c, schema, filter, page, size)
}

// ListSchemas 数据库/模式列表（数据表页签筛选用）。
func (a *App) ListSchemas(id string) ([]string, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return nil, fmt.Errorf("连接不存在")
	}
	return provider.ListSchemas(&c)
}

// PickSQLiteFile 选择/创建 SQLite 本地文件，返回路径。
func (a *App) PickSQLiteFile(create bool) (string, error) {
	if create {
		return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "创建 SQLite 数据库文件",
			DefaultFilename: "new.db",
			Filters:         []runtime.FileFilter{{DisplayName: "SQLite 数据库", Pattern: "*.db;*.sqlite;*.sqlite3"}},
		})
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "打开 SQLite 数据库文件",
		Filters: []runtime.FileFilter{{DisplayName: "SQLite 数据库", Pattern: "*.db;*.sqlite;*.sqlite3"}},
	})
}

// PickCertFile 选择证书/私钥文件（kind: ca=CA或服务器证书, cert=客户端证书, key=私钥）。
func (a *App) PickCertFile(kind string) (string, error) {
	all := []runtime.FileFilter{{DisplayName: "证书/私钥文件", Pattern: "*.pem;*.crt;*.cer;*.der;*.key;*.p12;*.pfx"}}
	var filters []runtime.FileFilter
	switch kind {
	case "ca":
		filters = []runtime.FileFilter{
			{DisplayName: "CA/服务器证书", Pattern: "*.pem;*.crt;*.cer;*.der"},
			all[0],
		}
	case "key":
		filters = []runtime.FileFilter{
			{DisplayName: "私钥文件", Pattern: "*.key;*.pem;*.pfx;*.p12"},
			all[0],
		}
	default:
		filters = all
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "选择证书文件",
		Filters: filters,
	})
}

// ---------- 设置 ----------

// GetSettings 返回设置。
func (a *App) GetSettings() model.Settings { return a.store().Settings }

// SaveSettings 保存设置（并按需重启自动巡检）。
func (a *App) SaveSettings(s model.Settings) error {
	a.store().SaveSettings(s)
	if a.autoCancel != nil {
		a.autoCancel()
		a.autoCancel = nil
	}
	if s.AutoPing {
		a.startAuto()
	}
	return a.store().Save()
}

func (a *App) startAuto() {
	ctx, cancel := context.WithCancel(context.Background())
	a.autoCancel = cancel
	interval := a.store().Settings.IntervalSec
	if interval <= 0 {
		interval = 60
	}
	go a.monInst().RunAutoLoop(ctx, func() []model.Conn { return a.store().ListConns() }, interval)
}

// ---------- 导出 ----------

// ---------- SSH 主机巡检 ----------

// GetSSHHosts 返回 SSH 主机列表（脱敏）。
func (a *App) GetSSHHosts() []model.SSHHost {
	out := a.store().ListHosts()
	return out
}

// SaveSSHHost 新增/更新 SSH 主机（密码/口令留空表示保持不变）。
func (a *App) SaveSSHHost(h model.SSHHost) error {
	if h.Host == "" {
		return fmt.Errorf("主机不能为空")
	}
	if err := a.store().UpsertHost(h); err != nil {
		return err
	}
	log.Printf("[host] 保存 SSH 主机: %s (%s:%d)", h.Name, h.Host, h.Port)
	return a.store().Save()
}

// DeleteSSHHost 删除 SSH 主机。
func (a *App) DeleteSSHHost(id string) error {
	a.store().DeleteHost(id)
	log.Printf("[host] 删除 SSH 主机: %s", id)
	return a.store().Save()
}

// TestSSHHost 测试 SSH 连通性（连接 + 执行 echo，返回主机名/系统摘要）。
func (a *App) TestSSHHost(h model.SSHHost) map[string]any {
	// 密码/口令留空时沿用已存主机的凭据（编辑场景保持原密码语义）
	if h.ID != "" {
		if old, ok := a.store().GetHost(h.ID); ok {
			if h.Password == "" {
				h.Password = old.Password
			}
			if h.KeyPassphrase == "" {
				h.KeyPassphrase = old.KeyPassphrase
			}
			if h.KeyPath == "" {
				h.KeyPath = old.KeyPath
			}
		}
	}
	if h.Timeout <= 0 {
		h.Timeout = 15
	}
	password := h.Password
	if h.AuthType == "key" {
		password = h.KeyPassphrase
	}
	cli, err := hostinsp.Connect(&h, password, h.KeyPassphrase)
	if err != nil {
		return map[string]any{"ok": false, "detail": err.Error()}
	}
	defer cli.Close()
	out, err := cli.Run("echo ok && hostname && head -1 /etc/os-release 2>/dev/null", 15*time.Second)
	if err != nil {
		return map[string]any{"ok": false, "detail": err.Error()}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	detail := "SSH OK"
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "ok" {
			continue
		}
		if l != "" {
			detail += " · " + l
		}
	}
	return map[string]any{"ok": true, "detail": detail}
}

// InspectHost 执行主机巡检（同步，逐节推送进度事件）。
func (a *App) InspectHost(id string) (*model.HostReport, error) {
	h, ok := a.store().GetHost(id)
	if !ok {
		return nil, fmt.Errorf("主机不存在")
	}
	report, err := hostinsp.Inspect(&h, func(section string, done, total int) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "host:progress", map[string]any{
				"section": section, "done": done, "total": total,
			})
		}
	})
	if err != nil {
		log.Printf("[host] 巡检失败 %s: %v", h.Name, err)
		return nil, err
	}
	// 巡检结果历史保留（最新在前，每台最多 10 条）
	st := a.store()
	st.AddHostHistory(id, *report)
	if err := st.Save(); err != nil {
		log.Printf("[host] 巡检历史保存失败: %v", err)
	}
	log.Printf("[host] 巡检完成 %s: OS=%s, rs进程=%d, 警告=%d",
		h.Name, report.OSName, len(report.RSProcs), len(report.Warnings))
	return report, nil
}

// ---------- BMC 带外巡检（IPMI） ----------

// GetBMCs 返回 BMC 连接列表（脱敏）。
func (a *App) GetBMCs() []model.BMCHost {
	return a.store().ListBMCs()
}

// SaveBMC 新增/更新 BMC 连接（密码留空表示保持不变）。
func (a *App) SaveBMC(b model.BMCHost) error {
	if b.Host == "" {
		return fmt.Errorf("BMC 地址不能为空")
	}
	if err := a.store().UpsertBMC(b); err != nil {
		return err
	}
	log.Printf("[bmc] 保存 BMC 连接: %s (%s:%d)", b.Name, b.Host, b.Port)
	return a.store().Save()
}

// DeleteBMC 删除 BMC 连接。
func (a *App) DeleteBMC(id string) error {
	a.store().DeleteBMC(id)
	log.Printf("[bmc] 删除 BMC 连接: %s", id)
	return a.store().Save()
}

// TestBMC 测试 BMC 连通性（连接 + GetDeviceID；密码留空沿用已存凭据）。
func (a *App) TestBMC(b model.BMCHost) map[string]any {
	if b.ID != "" {
		if old, ok := a.store().GetBMC(b.ID); ok && b.Password == "" {
			b.Password = old.Password
		}
	}
	return bmc.TestBMC(&b)
}

// InspectBMC 执行 BMC 带外巡检（同步，逐节推送进度事件）。
func (a *App) InspectBMC(id string) (*model.BMCReport, error) {
	b, ok := a.store().GetBMC(id)
	if !ok {
		return nil, fmt.Errorf("BMC 连接不存在")
	}
	report, err := bmc.InspectBMC(&b, func(section string, done, total int) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "bmc:progress", map[string]any{
				"section": section, "done": done, "total": total,
			})
		}
	})
	if err != nil {
		log.Printf("[bmc] 巡检失败 %s: %v", b.Name, err)
		return nil, err
	}
	// 巡检结果历史保留（最新在前，每台最多 10 条）
	st := a.store()
	st.AddBMCHistory(id, *report)
	if err := st.Save(); err != nil {
		log.Printf("[bmc] 巡检历史保存失败: %v", err)
	}
	log.Printf("[bmc] 巡检完成 %s: 健康=%s, 传感器=%d", b.Name, report.Health, len(report.Sensors))
	return report, nil
}

// GetBMCHistory 返回 BMC 巡检历史（最新在前，每台最多 10 条）。
func (a *App) GetBMCHistory(id string) []model.BMCHistoryEntry {
	return a.store().GetBMCHistory(id)
}

// ExportBMCReport 导出 BMC 综合 HTML 报告（巡检 + AI 分析，带目录跳转），返回保存路径。
// reportJSON/aiMarkdown 为前端当前展示的报告与 AI 分析（可空；为空时重新采集巡检）。
func (a *App) ExportBMCReport(id, reportJSON, aiMarkdown string) (string, error) {
	b, ok := a.store().GetBMC(id)
	if !ok {
		return "", fmt.Errorf("BMC 连接不存在")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出 BMC 综合报告（巡检 + AI）",
		DefaultFilename: fmt.Sprintf("HostPing BMC报告_%s_%s.html", b.Name, time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	// 优先用前端传入的当前报告（避免重复巡检）；无则重新采集
	var report *model.BMCReport
	if strings.TrimSpace(reportJSON) != "" {
		r := model.BMCReport{}
		if json.Unmarshal([]byte(reportJSON), &r) == nil && r.CollectedAt > 0 {
			report = &r
		}
	}
	if report == nil {
		report, err = bmc.InspectBMC(&b, nil)
		if err != nil {
			return "", err
		}
	}
	if err := writeHTML(path, renderCombinedBMC(b, report, aiMarkdown, a.store().Settings.AIModel)); err != nil {
		return "", err
	}
	log.Printf("[bmc] 综合报告已导出: %s", path)
	return path, nil
}

// NmapScan 类 nmap 端口扫描（并发 TCP 连接探测 + TTL/Banner 系统推测，从本机直接连目标）。
func (a *App) NmapScan(target string, portSpec string) (*model.NmapResult, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("目标主机不能为空")
	}
	ports, err := hostinsp.ParsePortSpec(portSpec)
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("端口列表为空")
	}
	var mu sync.Mutex
	var results []model.PortInfo
	done := 0
	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	for _, p := range ports {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", target, port), 3*time.Second)
			pi := model.PortInfo{Port: fmt.Sprintf("%d", port)}
			if err == nil {
				pi.State = "开放"
				pi.Service = hostinsp.ServiceName(port)
				conn.Close()
			} else {
				pi.State = "关闭"
			}
			mu.Lock()
			results = append(results, pi)
			done++
			d := done
			mu.Unlock()
			// 逐端口推送扫描进度
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "nmap:progress", map[string]any{"done": d, "total": len(ports)})
			}
		}(p)
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool {
		x, _ := strconv.Atoi(results[i].Port)
		y, _ := strconv.Atoi(results[j].Port)
		return x < y
	})
	// 开放端口与 Banner → 系统推测
	var openPorts []int
	for _, pi := range results {
		if pi.State == "开放" {
			n, _ := strconv.Atoi(pi.Port)
			openPorts = append(openPorts, n)
		}
	}
	banners := map[int]string{}
	var bwg sync.WaitGroup
	var bmu sync.Mutex
	for _, p := range hostinsp.BannerProbePorts(openPorts) {
		bwg.Add(1)
		go func(port int) {
			defer bwg.Done()
			b := hostinsp.GrabBanner(target, port, 1500*time.Millisecond)
			if b != "" {
				bmu.Lock()
				banners[port] = b
				bmu.Unlock()
			}
		}(p)
	}
	bwg.Wait()
	ttl, ttlOK := hostinsp.ICMPTTL(target, 2*time.Second)
	guess, basis := hostinsp.GuessOS(openPorts, banners, ttl, ttlOK)
	res := &model.NmapResult{
		Target:  target,
		OSGuess: guess,
		OSBasis: basis,
		Ports:   results,
	}
	// 记录扫描结果（持久化，最新在前，最多 50 条）
	a.store().AddNmapHistory(model.NmapHistoryItem{
		Target:    target,
		PortSpec:  strings.TrimSpace(portSpec),
		Time:      time.Now().UnixMilli(),
		OSGuess:   guess,
		OSBasis:   basis,
		OpenCount: len(openPorts),
		Total:     len(results),
		Ports:     results,
	})
	if err := a.store().Save(); err != nil {
		log.Printf("[nmap] 扫描记录保存失败: %v", err)
	}
	log.Printf("[nmap] 扫描完成 %s: 开放 %d/%d, OS推测=%s", target, len(openPorts), len(results), guess)
	return res, nil
}

// GetNmapHistory 返回类 nmap 扫描记录（最新在前）。
func (a *App) GetNmapHistory() []model.NmapHistoryItem {
	return a.store().ListNmapHistory()
}

// ExportHostReport 导出主机综合 HTML 报告（巡检 + AI 分析，带目录跳转），返回保存路径。
// reportJSON/aiMarkdown 为前端当前展示的报告与 AI 分析（可空；为空时重新采集巡检）。
func (a *App) ExportHostReport(id, reportJSON, aiMarkdown string) (string, error) {
	h, ok := a.store().GetHost(id)
	if !ok {
		return "", fmt.Errorf("主机不存在")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出主机综合报告（巡检 + AI）",
		DefaultFilename: fmt.Sprintf("HostPing综合报告_%s_%s.html", h.Name, time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	// 优先用前端传入的当前报告（避免重复巡检）；无则重新采集
	var report *model.HostReport
	if strings.TrimSpace(reportJSON) != "" {
		r := model.HostReport{}
		if json.Unmarshal([]byte(reportJSON), &r) == nil && r.CollectedAt > 0 {
			report = &r
		}
	}
	if report == nil {
		report, err = hostinsp.Inspect(&h, nil)
		if err != nil {
			return "", err
		}
	}
	if err := writeHTML(path, renderCombinedHost(h, report, aiMarkdown, a.store().Settings.AIModel)); err != nil {
		return "", err
	}
	log.Printf("[host] 综合报告已导出: %s", path)
	return path, nil
}
func (a *App) ExportReport(id string, aiMarkdown string) (string, error) {
	c, ok := a.store().GetConn(id)
	if !ok {
		return "", fmt.Errorf("连接不存在")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出数据库综合报告（巡检 + AI）",
		DefaultFilename: fmt.Sprintf("HostPing综合报告_%s_%s.html", c.Name, time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	ov, _ := a.monInst().GetOverview(&c)
	dtl, _ := a.monInst().GetDetail(&c)
	topo, _ := a.monInst().GetTopology(&c)
	hist := a.monInst().GetHistory(id)
	html := renderCombinedDB(c, ov, dtl, topo, hist, aiMarkdown, a.store().Settings.AIModel)
	if err := writeHTML(path, html); err != nil {
		return "", err
	}
	return path, nil
}

// ExportReportAll 导出全部连接汇总报告（含每个连接的实例概览），返回保存路径。
func (a *App) ExportReportAll() (string, error) {
	conns := a.store().ListConns()
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出汇总巡检报告",
		DefaultFilename: fmt.Sprintf("HostPing汇总报告_%s.html", time.Now().Format("20060102_150405")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML 报告", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}

	// 并发采集概览（12 并发上限；已缓存的 ping 状态直接取用）
	rows := make([]*sumRow, len(conns))
	latest := a.monInst().Latest()
	sem := make(chan struct{}, 12)
	var wg sync.WaitGroup
	for i := range conns {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rw := &sumRow{conn: conns[idx]}
			if r, ok := latest[conns[idx].ID]; ok {
				rw.res, rw.checked = r, true
			}
			if ov, err := a.monInst().GetOverview(&conns[idx]); err == nil {
				rw.ov = ov
			}
			rows[idx] = rw
		}(i)
	}
	wg.Wait()

	html := renderSummaryReport(rows, time.Now())
	if err := writeHTML(path, html); err != nil {
		return "", err
	}
	return path, nil
}


