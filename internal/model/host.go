package model

// SSHHost 一个 SSH 主机巡检目标。
// Password/KeyPassphrase 不入 JSON，走 passwords 加密侧表（host ID 加 h- 前缀）。
type SSHHost struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Group         string `json:"group"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	AuthType      string `json:"authType"` // password | key
	Password      string `json:"password"`
	KeyPath       string `json:"keyPath"`
	KeyPassphrase string `json:"keyPassphrase"`
	Timeout       int    `json:"timeout"` // 单命令超时（秒）
}

// PortInfo 端口条目（监听采集 / nmap 扫描共用）。
type PortInfo struct {
	Port    string `json:"port"`
	State   string `json:"state"`           // LISTEN / UNCONN / 开放 / 关闭
	Service string `json:"service"`         // 识别的服务名
	Process string `json:"process"`         // 监听进程
	Conns   string `json:"conns,omitempty"` // 连接数摘要（如 ESTAB 3 · TIME-WAIT 5）
}

// NmapResult 类 nmap 端口扫描结果（含系统推测）。
type NmapResult struct {
	Target  string     `json:"target"`
	OSGuess string     `json:"osGuess"` // Windows / Linux/Unix / 网络设备 / 未知
	OSBasis string     `json:"osBasis"` // 推测依据（TTL / Banner / 端口特征）
	Ports   []PortInfo `json:"ports"`
}

// NmapHistoryItem 一次类 nmap 扫描的持久化记录。
type NmapHistoryItem struct {
	Target    string     `json:"target"`
	PortSpec  string     `json:"portSpec"` // 原始端口范围输入（回填用）
	Time      int64      `json:"time"`     // UnixMilli
	OSGuess   string     `json:"osGuess"`
	OSBasis   string     `json:"osBasis"`
	OpenCount int        `json:"openCount"`
	Total     int        `json:"total"`
	Ports     []PortInfo `json:"ports"`
}

// Masked 返回脱敏副本（不给前端回显敏感字段）。
func (h SSHHost) Masked() SSHHost {
	h.Password = ""
	h.KeyPassphrase = ""
	return h
}

// ProcInfo 进程信息。
type ProcInfo struct {
	PID   string `json:"pid"`
	User  string `json:"user"`
	CPU   string `json:"cpu"`
	MEM   string `json:"mem"`
	RSS   string `json:"rss"`
	VSZ   string `json:"vsz"`
	ETime string `json:"etime"` // 运行时长
	Start string `json:"start"` // 启动时间
	Cmd   string `json:"cmd"`
}

// RSProc rs_/rsa_ 前缀进程的详细巡检结果。
type RSProc struct {
	ProcInfo
	FDCount   string   `json:"fdCount"`   // 打开文件描述符数量
	OpenFiles []string `json:"openFiles"` // 打开文件列表（截断）
	Conns     []string `json:"conns"`     // 网络连接（截断）
}

// StorcliVD storcli 虚拟磁盘条目。
type StorcliVD struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`   // RAID 级别（RAID-5 等）
	State  string `json:"state"`
	Size   string `json:"size"`
	Cache  string `json:"cache"`  // 缓存模式（RWBD / RA / WB 等）
	Detail string `json:"detail"`
}

// StorcliPD storcli 物理磁盘条目。
type StorcliPD struct {
	Slot   string `json:"slot"`
	Model  string `json:"model"`
	State  string `json:"state"`
	Size   string `json:"size"`
	SN     string `json:"sn"`    // 序列号
	Power  string `json:"power"` // 通电时长
	Detail string `json:"detail"`
}

// StorcliCtl 一个 RAID 控制器。
type StorcliCtl struct {
	Index    string      `json:"index"`
	Model    string      `json:"model"`
	Status   string      `json:"status"`
	Firmware string      `json:"firmware"` // 固件版本
	Temp     string      `json:"temp"`     // 控制器温度
	Memory   string      `json:"memory"`   // 缓存内存
	Battery  string      `json:"battery"`  // 电池状态
	VDs      []StorcliVD `json:"vds"`
	PDs      []StorcliPD `json:"pds"`
}

// StorcliReport storcli64 巡检结果。
type StorcliReport struct {
	Controllers []StorcliCtl `json:"controllers"`
}

// ZFSReport ZFS 巡检结果。
type ZFSReport struct {
	Pools    [][]string `json:"pools"`
	PoolCols []string   `json:"poolCols"`
	Status   string     `json:"status"`
	Datasets [][]string `json:"datasets"`
	DSectCols []string  `json:"dsCols"`
}

// SysLogReport 系统日志检查结果。
type SysLogReport struct {
	ErrCount  string   `json:"errCount"`
	WarnCount string   `json:"warnCount"`
	TopErrors []string `json:"topErrors"`
	TopWarns  []string `json:"topWarns"`
	DmesgErrs []string `json:"dmesgErrs"`
	Source    string   `json:"source"`
}

// AIAnalysis 一次 AI 分析结果（Markdown + 预渲染 HTML）。
type AIAnalysis struct {
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
	Model    string `json:"model"`
	Time     int64  `json:"time"`
}

// HostHistoryEntry 主机巡检历史条目（巡检报告 + 对应的 AI 分析）。
type HostHistoryEntry struct {
	Time   int64        `json:"time"`   // 巡检完成时间（UnixMilli）
	Report HostReport   `json:"report"` // 完整巡检报告
	AI     *AIAnalysis  `json:"ai"`     // 对应的 AI 分析（未分析时为空）
}

// SpeedInterval 测速每秒区间。
type SpeedInterval struct {
	Sec     int     `json:"sec"`     // 区间序号（1 起）
	MBps    float64 `json:"mbps"`    // 该秒速率（Mbps）
	Retrans int64   `json:"retrans"` // 重传
	Omitted bool    `json:"omitted"` // 预热阶段
}

// SpeedEntry 测速历史条目。
type SpeedEntry struct {
	Time       int64           `json:"time"`  // UnixMilli
	Server     string          `json:"server"`
	Port       int             `json:"port"`
	Direction  string          `json:"direction"` // up / down
	Parallel   int             `json:"parallel"`
	Duration   int             `json:"duration"`
	Seconds    float64         `json:"seconds"`
	Mbps       float64         `json:"mbps"`
	ServerMbps float64         `json:"serverMbps"`
	Retrans    int64           `json:"retrans"`
	Warning    string          `json:"warning,omitempty"`
	Intervals  []SpeedInterval `json:"intervals"`
}

// ScanEntry 端口扫描历史条目。
type ScanEntry struct {
	Time      int64      `json:"time"` // UnixMilli
	Target    string     `json:"target"`
	PortSpec  string     `json:"portSpec"`
	OSGuess   string     `json:"osGuess"`
	OSBasis   string     `json:"osBasis"`
	OpenCount int        `json:"openCount"`
	Total     int        `json:"total"`
	Ports     []PortInfo `json:"ports"`
}

// DBAIEntry 数据库 AI 分析历史条目（含概览快照，可回看）。
type DBAIEntry struct {
	Time         int64      `json:"time"`
	ConnID       string     `json:"connId"`
	ConnName     string     `json:"connName"`
	ConnType     string     `json:"connType"`
	OverviewJSON string     `json:"overviewJson"` // 概览快照
	AI           AIAnalysis `json:"ai"`
}

// RebootInfo 一次重启记录。
type RebootInfo struct {
	Start string `json:"start"` // 启动时间
	End   string `json:"end"`   // 关机时间（- 表示仍在运行）
	Dur   string `json:"dur"`   // 本次运行时长
}

// HostReport 主机巡检报告。
type HostReport struct {
	HostID      string       `json:"hostId"`
	Name        string       `json:"name"`
	Host        string       `json:"host"`
	Port        int          `json:"port"`
	User        string       `json:"user"`
	CollectedAt int64        `json:"collectedAt"`
	OSName      string       `json:"osName"` // 发行版 + 内核 + 架构
	OSKind      string       `json:"osKind"` // linux / unknown
	SysInfo     []KV         `json:"sysInfo"`
	TopCPU      []ProcInfo   `json:"topCpu"`
	TopMem      []ProcInfo   `json:"topMem"`
	RSProcs     []RSProc     `json:"rsProcs"`
	SysLog      *SysLogReport `json:"sysLog,omitempty"`
	SysLogNote  string       `json:"sysLogNote"`
	Security    []KV         `json:"security"`
	ListenPorts []PortInfo   `json:"listenPorts"` // 全部监听端口（TCP/UDP，含进程与连接数）
	Storcli     *StorcliReport `json:"storcli,omitempty"`
	StorcliNote string       `json:"storcliNote"`
	ZFS         *ZFSReport   `json:"zfs,omitempty"`
	ZFSNote     string       `json:"zfsNote"`
	Lsscsi      string       `json:"lsscsi"`
	LsscsiNote  string       `json:"lsscsiNote"`
	Lspci       string       `json:"lspci"`
	LspciNote   string       `json:"lspciNote"`
	Reboots     []RebootInfo `json:"reboots"` // 最近 10 次重启记录
	Warnings    []string     `json:"warnings"`
}
