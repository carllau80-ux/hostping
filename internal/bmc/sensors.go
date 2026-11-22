package bmc

import (
	"context"
	"fmt"
	"strings"

	"dbping/internal/model"

	ipmi "github.com/bougou/go-ipmi/pkg/client"
	sensorpkg "github.com/bougou/go-ipmi/pkg/command/sensor"
	ipmitypes "github.com/bougou/go-ipmi/pkg/types"
)

// unitCN IPMI 基本单位码 → 中文单位（未收录回退空）。
var unitCN = map[uint8]string{
	0x00: "", 0x01: "°C", 0x02: "°F", 0x03: "K", 0x04: "V", 0x05: "A",
	0x06: "W", 0x08: "C", 0x0D: "ms", 0x12: "m", 0x13: "rpm", 0x16: "Hz",
	0x24: "%", 0x30: "L",
}

// parseSensorSDR 解析单个传感器 SDR 并读取实时读数；不支持读取的返回 nil。
func parseSensorSDR(cli *ipmi.Client, ctx context.Context, sdr *ipmitypes.SDR) *model.BMCSensor {
	var (
		name       string
		sensorNum  uint8
		typeCode   uint8
		analog     bool
		full       *ipmitypes.SDRFull
		thresholds string
	)
	switch {
	case sdr.Full != nil:
		full = sdr.Full
		name = string(full.IDStringBytes)
		sensorNum = uint8(full.SensorNumber)
		typeCode = uint8(full.SensorType)
		analog = full.HasAnalogReading()
		thresholds = thresholdRangeStr(full)
	case sdr.Compact != nil:
		c := sdr.Compact
		name = string(c.IDStringBytes)
		sensorNum = uint8(c.SensorNumber)
		typeCode = uint8(c.SensorType)
	default: // EventOnly / 定位类等无读数
		return nil
	}
	if name == "" {
		name = "传感器 " + fmt.Sprintf("%d", sensorNum)
	}

	reading, err := cli.GetSensorReading(ctx, sensorNum)
	if err != nil {
		return nil // 读取失败按跳过处理
	}

	s := &model.BMCSensor{
		Name:   name,
		Number: fmt.Sprintf("%d", sensorNum),
		Kind:   sensorTypeName(typeCode),
	}
	switch {
	case reading.ReadingUnavailable:
		s.Status = "unknown"
		s.Value = "不可用"
	case analog && full != nil:
		s.Value = fmt.Sprintf("%.2f %s", full.ConvertReading(reading.Reading), analogUnit(full))
		s.Status = "nominal"
		s.Range = thresholds
	default: // 离散量
		if hasActiveState(reading) {
			s.Value = "状态激活: " + activeStateStr(reading)
			s.Status = "warning"
		} else {
			s.Value = "读数 0x" + hex8(uint32(reading.Reading))
			s.Status = "nominal"
		}
	}
	// 阈值标志位优先于默认判定（阈值型与离散型响应均可能携带）
	s.Status = sensorStatusFromFlags(reading, s.Status)
	return s
}

// sensorStatusFromFlags 按读数阈值标志位判定状态（优先级：critical > warning > 默认值）。
func sensorStatusFromFlags(reading *sensorpkg.GetSensorReadingResponse, fallback string) string {
	if reading.Above_UNR || reading.Above_UCR || reading.Below_LNR || reading.Below_LCR {
		return "critical"
	}
	if reading.Above_UNC || reading.Below_LNC {
		if fallback != "critical" {
			return "warning"
		}
	}
	return fallback
}

// thresholdRangeStr 模拟量可读阈值范围串（如 "低警告 -5.00 / 高严重 60.00"）。
func thresholdRangeStr(full *ipmitypes.SDRFull) string {
	var lows, highs string
	for _, t := range []struct {
		code ipmitypes.SensorThresholdType
		mark string
	}{
		{ipmitypes.SensorThresholdType_LNC, "低警告"},
		{ipmitypes.SensorThresholdType_LCR, "低严重"},
		{ipmitypes.SensorThresholdType_UNC, "高警告"},
		{ipmitypes.SensorThresholdType_UCR, "高严重"},
	} {
		v := full.ThresholdValueStr(t.code)
		if v == "" || v == "unspecified" {
			continue
		}
		if t.code == ipmitypes.SensorThresholdType_LNC || t.code == ipmitypes.SensorThresholdType_LCR {
			lows = appendSep(lows, t.mark+" "+v)
		} else {
			highs = appendSep(highs, t.mark+" "+v)
		}
	}
	return appendSep(lows, highs)
}

func appendSep(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " / " + b
}

// analogUnit 模拟量单位（BaseUnit 映射，未知回退空）。
func analogUnit(full *ipmitypes.SDRFull) string {
	return unitCN[uint8(full.SensorUnit.BaseUnit)]
}

// hasActiveState 离散量是否存在激活状态位。
func hasActiveState(reading *sensorpkg.GetSensorReadingResponse) bool {
	return len(reading.ActiveStates.TrueEvents()) > 0
}

// activeStateStr 激活状态位串。
func activeStateStr(reading *sensorpkg.GetSensorReadingResponse) string {
	events := reading.ActiveStates.TrueEvents()
	var parts []string
	for _, e := range events {
		parts = append(parts, fmt.Sprintf("0x%02X", e))
	}
	return strings.Join(parts, ",")
}
