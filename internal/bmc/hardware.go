package bmc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"dbping/internal/model"

	ipmi "github.com/bougou/go-ipmi/pkg/client"
)

// collectHardware 从 BMC 读取硬件配置：按传感器类型统计 CPU/内存/电源/风扇/盘位，
// 并列出 SDR 中的 FRU 设备（DIMM/PSU/背板等）与 BMC 管理控制器。依赖 collectSensors 先行。
func collectHardware(cli *ipmi.Client, r *model.BMCReport) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sdrs, err := cli.GetSDRs(ctx)
	if err != nil {
		r.HardwareNote = "硬件配置读取失败（SDR 仓库不可用）：" + shortErr(err)
		return
	}
	var fruDevices []string
	for _, sdr := range sdrs {
		if sdr == nil || sdr.RecordHeader == nil {
			continue
		}
		switch {
		case sdr.FRUDeviceLocator != nil:
			name := strings.TrimSpace(string(sdr.FRUDeviceLocator.DeviceIDBytes))
			if name != "" {
				fruDevices = append(fruDevices, name)
			}
		case sdr.MgmtControllerDeviceLocator != nil:
			name := strings.TrimSpace(string(sdr.MgmtControllerDeviceLocator.DeviceIDBytes))
			if name != "" {
				fruDevices = append(fruDevices, name+"（管理控制器）")
			}
		}
	}
	r.Hardware = summarizeHardware(r.Sensors, fruDevices)
	if len(r.Hardware) == 0 {
		r.HardwareNote = "未从 BMC 读取到硬件配置信息（无传感器与 FRU 定位记录）"
	}
}

// hardwareStats 硬件部件统计行（类型码 → 中文名 → 数量与状态摘要）。
var hardwareStats = []struct {
	code   uint8
	name   string
	noun   string // 量词
}{
	{0x07, "处理器", "颗"},
	{0x0C, "内存", "条"},
	{0x08, "电源模块", "个"},
	{0x09, "电源冗余", ""},
	{0x04, "风扇", "个"},
	{0x0D, "驱动槽", "个"},
	{0x05, "机箱面板", ""},
	{0x06, "物理安全", ""},
}

// summarizeHardware 由传感器结果与 FRU 定位器生成硬件配置 KV 行（纯函数，可测）。
func summarizeHardware(sensors []model.BMCSensor, fruDevices []string) []model.KV {
	var rows []model.KV
	for _, st := range hardwareStats {
		var matched []model.BMCSensor
		for _, s := range sensors {
			if kindCode(s.Kind) == st.code || (st.code == 0x07 && strings.HasPrefix(s.Kind, "处理器")) {
				matched = append(matched, s)
			}
		}
		if len(matched) == 0 {
			continue
		}
		crit, warn := 0, 0
		for _, s := range matched {
			switch s.Status {
			case "critical":
				crit++
			case "warning":
				warn++
			}
		}
		summary := fmt.Sprintf("%d %s", len(matched), st.noun)
		if st.noun == "" {
			summary = fmt.Sprintf("%d 项", len(matched))
		}
		if crit > 0 {
			summary += fmt.Sprintf("（严重 %d", crit)
			if warn > 0 {
				summary += fmt.Sprintf("，警告 %d", warn)
			}
			summary += "）"
		} else if warn > 0 {
			summary += fmt.Sprintf("（警告 %d）", warn)
		}
		rows = append(rows, model.KV{Key: st.name, Value: summary})
	}
	if len(fruDevices) > 0 {
		sort.Strings(fruDevices)
		rows = append(rows, model.KV{Key: "FRU 设备", Value: fmt.Sprintf("%d 个：%s", len(fruDevices), strings.Join(fruDevices, "、"))})
	}
	return rows
}

// kindCode 从中文类型名反查传感器类型码（sensorTypeName 的逆映射）。
func kindCode(name string) uint8 {
	for code, cn := range sensorTypeCN {
		if cn == name {
			return code
		}
	}
	return 0
}
