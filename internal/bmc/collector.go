package bmc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"dbping/internal/model"

	ipmi "github.com/bougou/go-ipmi/pkg/client"
	ipmitypes "github.com/bougou/go-ipmi/pkg/types"
)

// ProgressFn 巡检进度回调（section: 当前节名; done/total: 已完成/总节数），仿 hostinsp.ProgressFn。
type ProgressFn func(section string, done, total int)

// sections 巡检节（与前端 i18n key bmc.step.<name> 对应）。
var sections = []string{"connect", "device", "chassis", "sensors", "hw", "fru", "sel"}

// TestBMC 测试 BMC 连接（连接 + GetDeviceID），返回 {ok, detail} 供前端展示。
func TestBMC(h *model.BMCHost) map[string]any {
	cli, err := connect(h)
	if err != nil {
		return map[string]any{"ok": false, "detail": err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout(h))
	defer func() { cancel(); cli.Close(ctx) }()
	dev, err := cli.GetDeviceID(ctx)
	if err != nil {
		return map[string]any{"ok": false, "detail": "已连接但 GetDeviceID 失败：" + shortErr(err)}
	}
	return map[string]any{"ok": true, "detail": fmt.Sprintf("%s · 固件 %s · IPMI %s",
		manufacturerName(dev.ManufacturerID), firmwareStr(dev.MajorFirmwareRevision, dev.MinorFirmwareRevision),
		ipmiVersionStr(dev.MajorIPMIVersion, dev.MinorIPMIVersion))}
}

// InspectBMC 对 BMC 执行完整带外巡检（连接失败直接返回错误；其余节失败降级为 Note/Warnings）。
func InspectBMC(h *model.BMCHost, progress ProgressFn) (*model.BMCReport, error) {
	r := &model.BMCReport{
		HostID: h.ID, Name: h.Name, Host: h.Host, Port: h.Port, User: h.User,
		CollectedAt: time.Now().UnixMilli(),
	}
	cli, err := connect(h)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout2())
	defer func() { cancel(); cli.Close(ctx) }()

	done := 0
	total := len(sections) - 1 // connect 已完成
	done++
	if progress != nil {
		progress("connect", done, total)
	}
	step := func(name string, fn func(*ipmi.Client, *model.BMCReport)) {
		if progress != nil {
			progress(name, done, total)
		}
		fn(cli, r)
		done++
		if progress != nil {
			progress(name, done, total)
		}
	}
	step("device", collectDevice)
	step("chassis", collectChassis)
	step("sensors", collectSensors)
	step("hw", collectHardware)
	step("fru", collectFRU)
	step("sel", collectSEL)
	r.Health, r.HealthDetail = computeHealth(r)
	return r, nil
}

// connect 建立 RMCP+ 会话；cipher suite 17（默认）失败后回退 3（HMAC-SHA1 无加密，兼容老 BMC）。
func connect(h *model.BMCHost) (*ipmi.Client, error) {
	port := h.Port
	if port <= 0 {
		port = 623
	}
	timeout := time.Duration(h.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var lastErr error
	for _, suite := range []ipmitypes.CipherSuiteID{ipmitypes.CipherSuiteID17, ipmitypes.CipherSuiteID3} {
		cli, err := ipmi.NewClient(h.Host, port, h.User, h.Password)
		if err != nil {
			return nil, classifyConnErr(h, err)
		}
		cli.WithTimeout(timeout)
		cli.WithCipherSuiteID(suite)
		ctx, cancel := context.WithTimeout(context.Background(), timeout+5*time.Second)
		err = cli.Connect(ctx)
		cancel()
		if err == nil {
			return cli, nil
		}
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = cli.Close(closeCtx)
		closeCancel()
		lastErr = err
		if errors.Is(err, ipmi.ErrRAKPAuthentication) {
			break // 认证失败，换套件无意义
		}
	}
	return nil, classifyConnErr(h, lastErr)
}

// classifyConnErr 连接错误中文分类（网络不通/认证失败/超时），超时时用 RMCP Presence Ping 精确定位。
func classifyConnErr(h *model.BMCHost, err error) error {
	if err == nil {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", h.Host, defaultPort(h.Port))
	msg := shortErr(err)
	switch {
	case errors.Is(err, ipmi.ErrRAKPAuthentication):
		return fmt.Errorf("BMC 认证失败（%s）：用户名或密码错误（%s）", addr, msg)
	case errors.Is(err, context.DeadlineExceeded) || isTimeoutNetErr(err):
		return classifyTimeout(h, addr)
	default:
		var nerr net.Error
		if errors.As(err, &nerr) && nerr.Timeout() {
			return classifyTimeout(h, addr)
		}
		return fmt.Errorf("BMC 连接失败（%s）：网络不通或端口错误（%s）", addr, msg)
	}
}

// classifyTimeout 超时细分：RMCP ping 有回包=会话层问题；无回包=网络层不可达。
func classifyTimeout(h *model.BMCHost, addr string) error {
	// 两次探测，各 2 秒，区分 BMC 完全无响应与 RMCP+ 会话建立失败
	if rmcpPing(h.Host, defaultPort(h.Port), 2*time.Second) || rmcpPing(h.Host, defaultPort(h.Port), 2*time.Second) {
		return fmt.Errorf("BMC 有响应但 RMCP+ 会话建立失败（%s）：可能为权限/密码套件限制，或 BMC 忙碌，请稍后重试", addr)
	}
	return fmt.Errorf("BMC 无响应（%s）：请检查 1) IP 是否为 BMC 管理口地址而非操作系统 IP；2) 防火墙是否放行 UDP 623；3) BMC 是否开启了 IPMI over LAN——Dell iDRAC 需在 iDRAC Settings → Connectivity/IPMI Settings 中启用，华为/浪潮 iBMC 类似", addr)
}

func isTimeoutNetErr(err error) bool {
	var nerr net.Error
	if errors.As(err, &nerr) {
		return nerr.Timeout()
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout") ||
		strings.Contains(strings.ToLower(err.Error()), "i/o timeout")
}

func opTimeout(h *model.BMCHost) time.Duration {
	t := time.Duration(h.Timeout) * time.Second
	if t <= 0 {
		return 10 * time.Second
	}
	return t
}

func defaultPort(p int) int {
	if p <= 0 {
		return 623
	}
	return p
}

// ---------- 各节采集 ----------

func collectDevice(cli *ipmi.Client, r *model.BMCReport) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout2())
	defer cancel()
	dev, err := cli.GetDeviceID(ctx)
	if err != nil {
		r.DeviceNote = "GetDeviceID 失败：" + shortErr(err)
		return
	}
	d := &model.BMCDeviceInfo{
		DeviceID:       fmt.Sprintf("%d", dev.DeviceID),
		ManufacturerID: dev.ManufacturerID,
		Manufacturer:   manufacturerName(dev.ManufacturerID),
		ProductID:      dev.ProductID,
		Firmware:       firmwareStr(dev.MajorFirmwareRevision, dev.MinorFirmwareRevision),
		IPMIVersion:    ipmiVersionStr(dev.MajorIPMIVersion, dev.MinorIPMIVersion),
	}
	if len(dev.AuxiliaryFirmwareRevision) > 0 {
		var parts []string
		for _, b := range dev.AuxiliaryFirmwareRevision {
			parts = append(parts, hex8(uint32(b)))
		}
		d.AuxFirmware = strings.Join(parts, " ")
	}
	// 自检结果（防御式，失败不影响整体）
	if st, err := cli.GetSelfTestResults(ctx); err == nil {
		d.SelfTest = selfTestStr(st.Byte1, st.Byte2)
	}
	// 产品型号多存于 FRU，此处仅显示产品 ID
	d.Product = "产品ID 0x" + hex4(dev.ProductID)
	r.Device = d
}

func collectChassis(cli *ipmi.Client, r *model.BMCReport) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout2())
	defer cancel()
	st, err := cli.GetChassisStatus(ctx)
	if err != nil {
		r.ChassisNote = "GetChassisStatus 失败：" + shortErr(err)
		return
	}
	power := "关机"
	if st.PowerIsOn {
		power = "上电"
	}
	r.Chassis = append(r.Chassis, model.KV{Key: "电源状态", Value: power})
	if st.PowerControlFault {
		r.Chassis = append(r.Chassis, model.KV{Key: "电源控制故障", Value: "是（未能进入目标电源状态）"})
	}
	if st.PowerFault {
		r.Chassis = append(r.Chassis, model.KV{Key: "电源子系统故障", Value: "是"})
	}
	if st.PowerOverload {
		r.Chassis = append(r.Chassis, model.KV{Key: "电源过载", Value: "是（已因过载关机）"})
	}
	if st.InterLock {
		r.Chassis = append(r.Chassis, model.KV{Key: "面板联锁", Value: "激活（机箱面板联锁导致关机）"})
	}
	if st.CollingFanFault {
		r.Chassis = append(r.Chassis, model.KV{Key: "散热风扇故障", Value: "是"})
	}
	if st.DriveFault {
		r.Chassis = append(r.Chassis, model.KV{Key: "驱动器故障", Value: "是"})
	}
	if st.ChassisIntrusionActive {
		r.Chassis = append(r.Chassis, model.KV{Key: "机箱入侵", Value: "是（机箱盖曾被打开）"})
	}
	// 上次电源事件
	last := "无"
	switch {
	case st.ACFailed:
		last = "交流电失效(AC Failed)"
	case st.LastPowerDownByPowerFault:
		last = "电源故障导致下电"
	case st.LastPowerDownByPowerOverload:
		last = "电源过载导致下电"
	case st.LastPowerDownByPowerInterlockActivated:
		last = "面板联锁导致下电"
	case st.LastPowerOnByCommand:
		last = "命令上电"
	}
	r.Chassis = append(r.Chassis, model.KV{Key: "上次电源事件", Value: last})
	if st.FrontPanelLockoutActive {
		r.Chassis = append(r.Chassis, model.KV{Key: "前面板锁定", Value: "激活（电源/复位按钮禁用）"})
	}
}

// collectSensors 遍历 SDR 仓库，读全部传感器（阈值型按 LNC/LCR/LNR/UNC/UCR/UNR 判级，离散型按激活状态）。
func collectSensors(cli *ipmi.Client, r *model.BMCReport) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sdrs, err := cli.GetSDRs(ctx)
	if err != nil {
		r.SensorsNote = "SDR 仓库读取失败：" + shortErr(err)
		return
	}
	readCtx, readCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer readCancel()
	skip := 0
	for _, sdr := range sdrs {
		if sdr == nil || sdr.RecordHeader == nil {
			continue
		}
		s := parseSensorSDR(cli, readCtx, sdr)
		if s == nil {
			skip++
			continue
		}
		r.Sensors = append(r.Sensors, *s)
	}
	if len(r.Sensors) == 0 {
		r.SensorsNote = "SDR 仓库为空或无可用传感器"
		return
	}
	if skip > 0 {
		r.SensorsNote = fmt.Sprintf("另有 %d 个传感器读取失败/不支持（已跳过）", skip)
	}
}

// computeHealth 健康汇总：传感器 critical 或 SEL critical → 严重；任一 warning → 警告；否则正常。
func computeHealth(r *model.BMCReport) (string, string) {
	var crit, warn []string
	for _, s := range r.Sensors {
		switch s.Status {
		case "critical":
			crit = append(crit, "传感器 "+s.Name+" "+s.Value)
		case "warning":
			warn = append(warn, "传感器 "+s.Name+" "+s.Value)
		}
	}
	if r.SEL != nil {
		if r.SEL.CriticalCount > 0 {
			crit = append(crit, fmt.Sprintf("SEL 严重事件 %d 条", r.SEL.CriticalCount))
		}
		if r.SEL.WarningCount > 0 {
			warn = append(warn, fmt.Sprintf("SEL 警告事件 %d 条", r.SEL.WarningCount))
		}
	}
	switch {
	case len(crit) > 0:
		return "严重", strings.Join(crit, "；") + detailSuffix(warn)
	case len(warn) > 0:
		return "警告", strings.Join(warn, "；")
	default:
		if len(r.Sensors) == 0 {
			return "未知", "无传感器数据"
		}
		return "正常", "传感器与事件日志均无告警"
	}
}

func detailSuffix(warn []string) string {
	if len(warn) == 0 {
		return ""
	}
	return "；另有警告：" + strings.Join(warn, "；")
}

// ---------- 小工具 ----------

func opTimeout2() time.Duration { return 10 * time.Second }

func firmwareStr(major uint8, minorBCD uint8) string {
	return fmt.Sprintf("%d.%d", major, bcdToDec(minorBCD))
}

func ipmiVersionStr(major, minor uint8) string {
	return fmt.Sprintf("%d.%d", major, minor)
}

// bcdToDec BCD 码转十进制（固件次版本为 BCD 编码）。
func bcdToDec(b uint8) int {
	return int((b>>4)&0xF)*10 + int(b&0xF)
}

// selfTestStr 自检结果（IPMI 规范：0x55=无错误）。
func selfTestStr(b1, b2 uint8) string {
	switch b1 {
	case 0x55:
		if b2 == 0x00 {
			return "自检通过"
		}
		return "自检通过（存在轻微失效项）"
	case 0x56:
		return "自检未通过（固件损坏/镜像不完整）"
	case 0x57:
		return "自检未通过（功能性硬件失效）"
	case 0x58:
		return "自检未通过（存在致命失效，详细码见子命令）"
	default:
		return fmt.Sprintf("自检返回 0x%02X 0x%02X", b1, b2)
	}
}

func shortErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSpace(s)
}
