package bmc

import (
	"strings"
	"testing"
	"time"

	"dbping/internal/model"

	ipmitypes "github.com/bougou/go-ipmi/pkg/types"
)

func TestSelEventTextThreshold(t *testing.T) {
	cases := []struct {
		offset   uint8
		want     string
		severity string
	}{
		{0, "转为正常", "info"},
		{1, "低于下限警告", "warning"},
		{2, "低于下限严重", "critical"},
		{4, "超过上限警告", "warning"},
		{5, "超过上限严重", "critical"},
		{6, "超过上限不可恢复", "critical"},
	}
	for _, c := range cases {
		text, sev := selEventText(0x01, false, 0x01, c.offset) // 温度 + 阈值事件
		if !strings.Contains(text, c.want) || sev != c.severity {
			t.Errorf("阈值事件 offset=%d: got (%q,%q), want (%q,%q)", c.offset, text, sev, c.want, c.severity)
		}
	}
}

func TestSelEventTextSensorSpecific(t *testing.T) {
	// 电源模块故障
	text, sev := selEventText(0x08, false, 0x6F, 0x01)
	if text != "电源模块故障" || sev != "critical" {
		t.Errorf("PSU 故障: got (%q,%q)", text, sev)
	}
	// 电源冗余恢复
	text, sev = selEventText(0x09, false, 0x6F, 0x02)
	if text != "电源冗余恢复" || sev != "info" {
		t.Errorf("电源冗余恢复: got (%q,%q)", text, sev)
	}
	// 恢复(deassertion)一律 info
	text, sev = selEventText(0x08, true, 0x6F, 0x01)
	if !strings.HasPrefix(text, "恢复:") || sev != "info" {
		t.Errorf("恢复事件: got (%q,%q)", text, sev)
	}
	// 未知传感器类型的特定事件回退
	text, _ = selEventText(0xEE, false, 0x6F, 0x03)
	if !strings.Contains(text, "特定事件") {
		t.Errorf("回退描述: %q", text)
	}
}

func TestDecodeSELRoundtrip(t *testing.T) {
	in := &ipmitypes.SEL{
		RecordID:   0x01A2,
		RecordType: ipmitypes.SELRecordType(0x02),
		Standard: &ipmitypes.SELStandard{
			Timestamp:        time.Unix(1763942400, 0).UTC(),
			SensorType:       ipmitypes.SensorType(0x08), // 电源模块
			SensorNumber:     7,
			EventDir:         ipmitypes.EventDirAssertion,
			EventReadingType: 0x6F,
			EventData:        ipmitypes.EventData{EventData1: 0x01},
		},
	}
	parsed, err := ipmitypes.ParseSEL(in.Pack())
	if err != nil {
		t.Fatalf("ParseSEL 失败: %v", err)
	}
	e := decodeSEL(parsed)
	if e == nil {
		t.Fatal("decodeSEL 返回 nil")
	}
	if e.Sensor != "电源模块 #7" {
		t.Errorf("Sensor = %q", e.Sensor)
	}
	if e.Event != "电源模块故障" || e.Severity != "critical" {
		t.Errorf("Event/Severity = %q / %q", e.Event, e.Severity)
	}
	if e.RecordID != "0x01a2" {
		t.Errorf("RecordID = %q", e.RecordID)
	}
	if !strings.Contains(e.Time, ":") {
		t.Errorf("Time 格式异常: %q", e.Time)
	}
	// OEM 记录 → nil（计入跳过）
	oem := &ipmitypes.SEL{RecordID: 3, RecordType: ipmitypes.SELRecordType(0xC0)}
	if decodeSEL(oem) != nil {
		t.Errorf("OEM 记录应返回 nil")
	}
}

func TestTopSELErrors(t *testing.T) {
	entries := []model.BMCSEL{
		{Sensor: "电源模块 #1", Event: "电源模块故障", Severity: "critical"},
		{Sensor: "电源模块 #1", Event: "电源模块故障", Severity: "critical"},
		{Sensor: "温度 #5", Event: "超过上限严重", Severity: "critical"},
		{Sensor: "风扇 #2", Event: "风扇冗余丢失", Severity: "warning"},
		{Sensor: "温度 #5", Event: "恢复: 超过上限严重", Severity: "info"},
	}
	top := topSELErrors(entries)
	if len(top) != 3 {
		t.Fatalf("Top = %v, 期望 3 项", top)
	}
	if !strings.Contains(top[0], "×2 电源模块") {
		t.Errorf("Top1 = %q（应按次数排序）", top[0])
	}
}

func TestAnalyzeSEL(t *testing.T) {
	rep := &model.BMCSELReport{
		Total: "120", CriticalCount: 2, WarningCount: 1, InfoCount: 97,
		Entries: []model.BMCSEL{
			{Time: "2026-11-22 10:00:00", Sensor: "温度 #5", Event: "超过上限严重", Severity: "critical"},
		},
		TopErrors: []string{"×2 电源模块 #1 电源模块故障"},
	}
	analysis := analyzeSEL(rep)
	for _, want := range []string{"共 120 条", "严重 2", "最近严重事件：2026-11-22 10:00:00 温度 #5", "高频错误"} {
		if !strings.Contains(analysis, want) {
			t.Errorf("分析缺 %q:\n%s", want, analysis)
		}
	}
}

func TestComputeHealth(t *testing.T) {
	// 传感器严重 → 严重
	r := &model.BMCReport{Sensors: []model.BMCSensor{{Name: "CPU Temp", Value: "98 °C", Status: "critical"}}}
	health, detail := computeHealth(r)
	if health != "严重" || !strings.Contains(detail, "CPU Temp") {
		t.Errorf("health=%q detail=%q", health, detail)
	}
	// 仅警告 → 警告
	r = &model.BMCReport{Sensors: []model.BMCSensor{{Name: "Fan", Value: "300 rpm", Status: "warning"}}}
	if health, _ = computeHealth(r); health != "警告" {
		t.Errorf("health=%q", health)
	}
	// SEL 严重事件 → 严重
	r = &model.BMCReport{Sensors: []model.BMCSensor{{Name: "Temp", Value: "40 °C", Status: "nominal"}},
		SEL: &model.BMCSELReport{CriticalCount: 1}}
	if health, _ = computeHealth(r); health != "严重" {
		t.Errorf("SEL critical → health=%q", health)
	}
	// 无数据 → 未知
	if health, _ = computeHealth(&model.BMCReport{}); health != "未知" {
		t.Errorf("空数据 health=%q", health)
	}
	// 全正常 → 正常
	r = &model.BMCReport{Sensors: []model.BMCSensor{{Name: "Temp", Value: "40 °C", Status: "nominal"}}}
	if health, detail := computeHealth(r); health != "正常" {
		t.Errorf("health=%q detail=%q", health, detail)
	}
}

func TestManufacturerName(t *testing.T) {
	if manufacturerName(20181) != "华为 Huawei" {
		t.Errorf("华为映射失败")
	}
	if !strings.Contains(manufacturerName(0x1234), "0x") {
		t.Errorf("未知厂商应回退 ID 显示")
	}
}

func TestSelfTestStr(t *testing.T) {
	if selfTestStr(0x55, 0x00) != "自检通过" {
		t.Errorf("自检通过文案错误")
	}
	if !strings.Contains(selfTestStr(0x57, 0x00), "硬件失效") {
		t.Errorf("自检失败文案错误")
	}
}

func TestBCD(t *testing.T) {
	if bcdToDec(0x21) != 21 {
		t.Errorf("BCD 0x21 = %d", bcdToDec(0x21))
	}
	if firmwareStr(4, 0x66) != "4.66" {
		t.Errorf("固件串 = %q", firmwareStr(4, 0x66))
	}
}

func TestSummarizeHardware(t *testing.T) {
	sensors := []model.BMCSensor{
		{Kind: "处理器", Name: "CPU1 Status", Status: "nominal", Value: "存在"},
		{Kind: "处理器", Name: "CPU2 Status", Status: "nominal", Value: "存在"},
		{Kind: "内存", Name: "DIMM1", Status: "nominal"},
		{Kind: "内存", Name: "DIMM2", Status: "warning"},
		{Kind: "电源模块", Name: "PSU1", Status: "nominal"},
		{Kind: "电源模块", Name: "PSU2", Status: "critical"},
		{Kind: "风扇", Name: "Fan1", Status: "nominal"},
	}
	rows := summarizeHardware(sensors, []string{"System Board", "DIMM Slot 1"})
	get := func(key string) string {
		for _, kv := range rows {
			if kv.Key == key {
				return kv.Value
			}
		}
		return ""
	}
	if get("处理器") != "2 颗" {
		t.Errorf("处理器 = %q", get("处理器"))
	}
	if get("内存") != "2 条（警告 1）" {
		t.Errorf("内存 = %q", get("内存"))
	}
	if !strings.Contains(get("电源模块"), "严重 1") {
		t.Errorf("电源模块 = %q", get("电源模块"))
	}
	if get("风扇") != "1 个" {
		t.Errorf("风扇 = %q", get("风扇"))
	}
	if !strings.Contains(get("FRU 设备"), "2 个：") {
		t.Errorf("FRU 设备 = %q", get("FRU 设备"))
	}
	// 无传感器 → 空结果
	if rows := summarizeHardware(nil, nil); len(rows) != 0 {
		t.Errorf("空输入应返回空")
	}
}
