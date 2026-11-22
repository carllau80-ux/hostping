package model

// BMCHost BMC 带外连接配置（IPMI 2.0，RMCP+，UDP 623）。
type BMCHost struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Group    string `json:"group"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"` // 仅经 Wails 绑定层传递，不入盘（存 passwords 侧表加密）
	Timeout  int    `json:"timeout"`  // 秒
}

// Masked 返回脱敏副本（密码清空）。
func (h BMCHost) Masked() BMCHost {
	h.Password = ""
	return h
}

// BMCDeviceInfo BMC 自身信息。
type BMCDeviceInfo struct {
	DeviceID       string `json:"deviceId"`
	Manufacturer   string `json:"manufacturer"`
	ManufacturerID uint32 `json:"manufacturerId"`
	Product        string `json:"product"`
	ProductID      uint16 `json:"productId"`
	Firmware       string `json:"firmware"`
	IPMIVersion    string `json:"ipmiVersion"`
	AuxFirmware    string `json:"auxFirmware"`
	SelfTest       string `json:"selfTest"`
}

// BMCSensor 一个传感器读数。
type BMCSensor struct {
	Name   string `json:"name"`
	Number string `json:"number"`
	Kind   string `json:"kind"`   // 类型（温度/电压/风扇/电源…，按传感器类型码映射）
	Value  string `json:"value"`  // 已换算读数串（模拟量带单位；离散量显示状态）
	Status string `json:"status"` // nominal / warning / critical / unknown
	Range  string `json:"range"`  // 阈值范围串（模拟量，如 "低 -5 / 高 60"）
}

// BMCSEL 一条 SEL（系统事件日志）记录。
type BMCSEL struct {
	Time     string `json:"time"`
	RecordID string `json:"recordId"`
	Sensor   string `json:"sensor"` // 传感器类型（中文）+ 编号
	Event    string `json:"event"`  // 事件描述（含断言/恢复方向）
	Severity string `json:"severity"`
}

// BMCSELReport SEL 采集与规则化分析结果。
type BMCSELReport struct {
	Total         string   `json:"total"` // 日志总条数（BMC 侧）
	Entries       []BMCSEL `json:"entries"`
	CriticalCount int      `json:"criticalCount"`
	WarningCount  int      `json:"warningCount"`
	InfoCount     int      `json:"infoCount"`
	TopErrors     []string `json:"topErrors"`
	Analysis      string   `json:"analysis"`
	Note          string   `json:"note"` // 截断/跳过说明
}

// BMCReport BMC 带外巡检报告。
type BMCReport struct {
	HostID      string `json:"hostId"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	CollectedAt int64  `json:"collectedAt"`

	Device  *BMCDeviceInfo `json:"device,omitempty"`
	DeviceNote string     `json:"deviceNote"`
	Chassis []KV           `json:"chassis"`
	ChassisNote string    `json:"chassisNote"`
	Hardware []KV         `json:"hardware"` // 硬件配置（CPU/内存/电源/风扇/盘位统计 + FRU 设备清单）
	HardwareNote string   `json:"hardwareNote"`
	Sensors []BMCSensor    `json:"sensors"`
	SensorsNote string    `json:"sensorsNote"`
	FRU     []KV           `json:"fru"`
	FRUNote string         `json:"fruNote"`
	SEL     *BMCSELReport  `json:"sel,omitempty"`
	SELNote string         `json:"selNote"`

	Health       string   `json:"health"`       // 正常 / 警告 / 严重 / 未知
	HealthDetail string   `json:"healthDetail"` // 判定依据
	Warnings     []string `json:"warnings"`
}

// BMCHistoryEntry BMC 巡检历史条目。
type BMCHistoryEntry struct {
	Time   int64       `json:"time"` // 巡检完成时间（UnixMilli）
	Report BMCReport   `json:"report"`
	AI     *AIAnalysis `json:"ai"` // 对应的 AI 分析（未分析时为空）
}
