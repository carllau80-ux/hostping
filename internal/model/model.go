// Package model 定义 DBPing 的全部数据模型（前后端共享的 JSON 结构）。
package model

// Conn 一个数据库连接配置。
type Conn struct {
	ID       string `json:"id"`
	Name     string `json:"name"`     // 显示名
	Group    string `json:"group"`    // 分组（生产/测试/项目…）
	Type     string `json:"type"`     // 数据库类型标识（见 provider.TypeInfos）
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"` // 服务名/库名/实例/模式（含义随类型）
	User     string `json:"user"`
	Password string `json:"password"` // 仅内存传输；落盘时由 config 层加密
	Params   string `json:"params"`   // 附加 DSN 参数，如 sslmode=require&application_name=dbping
	Timeout  int    `json:"timeout"`  // 连接/查询超时（秒）
}

// Masked 返回脱敏副本（不给前端回显密码）。
func (c Conn) Masked() Conn {
	c.Password = ""
	return c
}

// Settings 全局设置。
type Settings struct {
	AutoPing     bool   `json:"autoPing"`     // 定时自动巡检
	IntervalSec  int    `json:"intervalSec"`  // 间隔秒数（默认 60）
	PingTimeout  int    `json:"pingTimeout"`  // 单次检查超时秒数（默认 5）
	AdvancedMode bool   `json:"advancedMode"` // 高级模式（启用 SQL 控制台，支持 CRUD）
	AIProvider   string `json:"aiProvider"`   // AI 提供商（zhipu/deepseek/qwen/moonshot/volces/qianfan/custom）
	AIModel      string `json:"aiModel"`      // AI 模型名
	AIBaseURL    string `json:"aiBaseUrl"`    // OpenAI 兼容 API Base URL（custom 时由用户填写）
}

// PingResult 一次健康检查的结果。
type PingResult struct {
	ConnID    string `json:"connId"`
	OK        bool   `json:"ok"`
	Stage     string `json:"stage"`     // 最后到达的阶段: tcp / auth / sql / ok
	ErrorKind string `json:"errorKind"` // network/auth/permission/protocol/timeout/unknown/空
	Error     string `json:"error"`     // 失败原因（含中文分类前缀）
	TCPMs     int64  `json:"tcpMs"`     // TCP 握手耗时
	SQLMs     int64  `json:"sqlMs"`     // SQL 执行耗时
	TotalMs   int64  `json:"totalMs"`
	CheckedAt int64  `json:"checkedAt"` // unix 毫秒
}

// KV 键值对（通用展示单元）。
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Overview 实例概览（元数据采集结果）。
type Overview struct {
	ConnID      string  `json:"connId"`
	Version     string  `json:"version"`     // 短版本号，如 8.0.36
	VersionFull string  `json:"versionFull"` // 完整版本横幅
	Edition     string  `json:"edition"`     // 版本/编辑（企业版等）
	Instance    string  `json:"instance"`    // 实例名/SID/服务名
	Hostname    string  `json:"hostname"`
	Port        int     `json:"port"`
	OS          string  `json:"os"`          // 服务器操作系统（尽量采集）
	StartupTime string  `json:"startupTime"`
	Uptime      string  `json:"uptime"`      // 已运行时长
	Role        string  `json:"role"`        // 主库/从库/备库/单机/未知
	ReadOnly    string  `json:"readOnly"`    // 是/否/未知
	ArchiveMode string  `json:"archiveMode"` // 归档/日志模式（跨库统一口径）
	ArchiveDest string  `json:"archiveDest"` // 归档路径/归档目的地（能查到则展示）
	Charset     string  `json:"charset"`
	Timezone    string  `json:"timezone"`
	DBSize      string  `json:"dbSize"`     // 数据库总大小
	TableCount  string  `json:"tableCount"` // 表数量
	ConnCur     string  `json:"connCur"`    // 当前连接数
	ConnMax     string  `json:"connMax"`    // 最大连接数
	Cluster     string  `json:"cluster"`    // 集群形态：单机/主从/分布式…
	Warnings    []string `json:"warnings"`  // 权限不足等提示
	Extra       []KV     `json:"extra"`     // 类型特有信息
}

// TableInfo 表清单条目。
type TableInfo struct {
	Schema  string `json:"schema"`
	Name    string `json:"name"`
	Rows    string `json:"rows"`
	Size    string `json:"size"`
	Comment string `json:"comment"`
}

// TBSInfo 表空间/文件组/磁盘条目。
type TBSInfo struct {
	Name   string `json:"name"`
	Size   string `json:"size"`
	Free   string `json:"free"`
	Used   string `json:"used"`   // 已用百分比或已用大小
	Status string `json:"status"`
	Files  string `json:"files"`
}

// DbInfo 数据库/模式/租户条目。
type DbInfo struct {
	Name    string `json:"name"`
	Size    string `json:"size"`
	Tables  string `json:"tables"`
	Charset string `json:"charset"`
	Note    string `json:"note"`
}

// SvcInfo 服务/节点条目（HANA 服务、OB 节点、TiDB 组件、复制成员…）。
type SvcInfo struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
	Host   string `json:"host"`
	Detail string `json:"detail"`
}

// Detail 单个连接的完整详情（总览 + 列表数据）。
type Detail struct {
	Overview *Overview   `json:"overview"`
	Dbs      []DbInfo    `json:"dbs"`      // 库/模式/租户
	Tables   []TableInfo `json:"tables"`   // Top N 大表
	TBS      []TBSInfo   `json:"tbs"`      // 表空间/文件组/磁盘
	Services []SvcInfo   `json:"services"` // 集群节点/服务/复制成员
}

// TNode 拓扑树节点。
type TNode struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Sub      string   `json:"sub,omitempty"`   // 副标题（类型/版本等）
	Status   string   `json:"status"`          // ok / warn / err / na
	KVs      []KV     `json:"kvs,omitempty"`   // 指标（tooltip 展示）
	Children []*TNode `json:"children,omitempty"`
}

// Topology 拓扑结构（树形）。
type Topology struct {
	ConnID string `json:"connId"`
	Root   *TNode `json:"root"`
}

// SQLResult SQL 控制台执行结果。
type SQLResult struct {
	Columns   []string   `json:"columns"`
	Rows      [][]string `json:"rows"`
	Affected  int64      `json:"affected"`
	IsQuery   bool       `json:"isQuery"`
	Total     int64      `json:"total"` // SELECT 分页时的总行数（-1=未获取）
	Page      int        `json:"page"`
	PageSize  int        `json:"pageSize"`
	Truncated bool       `json:"truncated"` // 原样执行且超过行数上限被截断
	ElapsedMs int64      `json:"elapsedMs"`
}

// TablePage 数据表元数据分页。
type TablePage struct {
	Total  int64      `json:"total"`
	Page   int        `json:"page"`
	Size   int        `json:"size"`
	Schema string     `json:"schema"`
	Cols   []string   `json:"cols"`
	Rows   [][]string `json:"rows"`
}

// TypeList 给前端的类型清单。
type TypeList struct {
	Types []TypeInfo `json:"types"`
}

// TypeInfo 数据库类型元数据（驱动 UI 动态表单与展示）。
type TypeInfo struct {
	Type      string `json:"type"`
	Name      string `json:"name"`      // 显示名
	Family    string `json:"family"`    // 实现族
	Port      int    `json:"port"`      // 默认端口
	DBLabel   string `json:"dbLabel"`   // “数据库”字段含义
	UserLabel string `json:"userLabel"` // “用户名”字段含义（空=隐藏）
	Icon      string `json:"icon"`
	Cluster   bool   `json:"cluster"` // 支持分布式拓扑
	NoSQL     bool   `json:"nosql"`
	Vendor    string `json:"vendor"`
	Note      string `json:"note"` // 备注（实验性等）
}
