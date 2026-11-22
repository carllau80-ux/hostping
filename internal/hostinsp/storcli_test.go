package hostinsp

import (
	"strings"
	"testing"
)

// 仿真 `/cALL show J` 输出（结构按 storcli 实际字段，防御式键名取值）。
const storcliSample = `CLI告警行在前
{
  "Controllers": [{
    "Command Name": "/c0 show",
    "Command Status": {"Status": "Success", "Return Code": 0},
    "Response Data": {
      "Product Name": "LSI MegaRAID 9361-8i",
      "Version Info": {"Firmware Version": "5.130.00-2658", "BIOS Version": "7.13.00.1"},
      "Temperature of Controller": "45C",
      "Memory": "2048 MB",
      "VD List": [
        {"VD": "0", "Name": "data", "Type": "RAID-10", "State": "Optl", "Size": "3.590 TB", "Cac": "RWBD", "Layout": "CONCAT", "Drives": "4"},
        {"VD": "1", "Name": "log", "Type": "RAID-1", "State": "Optl", "Size": "931.0 GB", "Cac": "RA", "Drives": "2"}
      ],
      "PD List": [
        {"E:S": "8:0", "DID": "8", "Model": "ST18000NM000J-2TV103", "State": "Onln", "Size": "1.750 TB", "Intf": "SAS", "Med": "HDD", "SN": "ZL231X4A"},
        {"E:S": "8:1", "DID": "9", "Model": "ST18000NM000J-2TV103", "State": "Onln", "Size": "1.750 TB", "Intf": "SAS", "Med": "HDD", "SN": "ZL231Y7B"}
      ]
    }
  }]
}`

// 仿真 `/cALL /eALL /sALL show J` 明细（键形如 Drive /c0/e8/s0 - Drive Information）。
const storcliDetail = `{
  "Controllers": [{
    "Response Data": {
      "Drive /c0/e8/s0 - Drive Information": {
        "SN": "ZL231X4A-DETAIL",
        "Power-On Hours": "30241"
      },
      "Drive /c0/e8/s1 - Drive Information": {
        "SN": "ZL231Y7B-DETAIL",
        "Power-On Hours": "18210"
      }
    }
  }]
}`

func TestParseStorcliJSON(t *testing.T) {
	rep, err := parseStorcliJSON(storcliSample)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(rep.Controllers) != 1 {
		t.Fatalf("控制器数 = %d, 期望 1", len(rep.Controllers))
	}
	c := rep.Controllers[0]
	if c.Model != "LSI MegaRAID 9361-8i" {
		t.Errorf("Model = %q", c.Model)
	}
	if c.Firmware != "5.130.00-2658" {
		t.Errorf("Firmware = %q", c.Firmware)
	}
	if c.Temp != "45C" {
		t.Errorf("Temp = %q", c.Temp)
	}
	if c.Memory != "2048 MB" {
		t.Errorf("Memory = %q", c.Memory)
	}
	if len(c.VDs) != 2 || c.VDs[0].Type != "RAID-10" || c.VDs[0].Cache != "RWBD" {
		t.Errorf("VDs = %+v", c.VDs)
	}
	if len(c.PDs) != 2 || c.PDs[0].SN != "ZL231X4A" || c.PDs[0].Slot != "8:0" {
		t.Errorf("PDs = %+v", c.PDs)
	}
}

func TestEnrichStorcliPDs(t *testing.T) {
	rep, _ := parseStorcliJSON(storcliSample)
	enrichStorcliPDs(storcliDetail, rep)
	c := rep.Controllers[0]
	if c.PDs[0].Power != "30241" || c.PDs[1].Power != "18210" {
		t.Errorf("通电时长未补齐: %+v", c.PDs)
	}
	if c.PDs[0].SN != "ZL231X4A" {
		t.Errorf("已有 SN 不应被覆盖: %q", c.PDs[0].SN)
	}
}

func TestParseBBUState(t *testing.T) {
	out := `{
		"Controllers": [{
			"Response Data": {"Battery State": "Operational", "Capacity": "95%"}
		}]
	}`
	got := parseBBUState(out)
	if got != "Operational（95%）" {
		t.Errorf("parseBBUState = %q", got)
	}
	if parseBBUState("no json") != "" {
		t.Errorf("非法输入应返回空")
	}
}

// 新版 storcli（007.x）键名：VD LIST / PD LIST / EID:Slt / DG/VD / TYPE（大写）/ Cache。
const storcliSampleNew = `{
  "Controllers": [{
    "Command Status": {"Status": "Success"},
    "Response Data": {
      "Product Name": "AVAGO MegaRAID SAS 9364-8i",
      "VD LIST": [
        {"DG/VD": "0/0", "TYPE": "RAID-5", "State": "Optl", "Access": "RW", "Consist": "Yes", "Cache": "RWBD", "Name": "data", "Size": "10.913 TB"}
      ],
      "PD LIST": [
        {"EID:Slt": "8:0", "DID": 8, "State": "Onln", "DG": 0, "Intf": "SAS", "Med": "HDD", "SeSz": "512B", "Model": "ST12000NM001G-2M103", "Size": "10.913 TB"},
        {"EID:Slt": "8:1", "DID": 9, "State": "Onln", "DG": 0, "Intf": "SAS", "Med": "SSD", "Model": "MTFDDAV960TGA", "Size": "894.25 GB"}
      ]
    }
  }]
}`

func TestParseStorcliJSONNewKeys(t *testing.T) {
	rep, err := parseStorcliJSON(storcliSampleNew)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	c := rep.Controllers[0]
	if c.Model != "AVAGO MegaRAID SAS 9364-8i" {
		t.Errorf("Model = %q", c.Model)
	}
	if len(c.VDs) != 1 || c.VDs[0].ID != "0/0" || c.VDs[0].Type != "RAID-5" || c.VDs[0].Cache != "RWBD" {
		t.Errorf("VDs = %+v", c.VDs)
	}
	if len(c.PDs) != 2 {
		t.Fatalf("PDs = %+v", c.PDs)
	}
	p := c.PDs[0]
	if p.Slot != "8:0" || p.MediaType != "HDD" || p.Interface != "SAS" || p.Model != "ST12000NM001G-2M103" {
		t.Errorf("PD[0] = %+v", p)
	}
	if c.PDs[1].MediaType != "SSD" {
		t.Errorf("PD[1].MediaType = %q", c.PDs[1].MediaType)
	}
}

// `/c0 show all J` 真实结构：控制器信息在嵌套段 Version / HwCfg / Status / Basics。
const storcliShowAll = `{
  "Controllers": [{
    "Response Data": {
      "Basics": {"Controller": 0, "Model": "AVAGO MegaRAID SAS 9364-8i", "Current System Date/time": "11/21/2026, 22:00:00"},
      "Version": {"BIOS Version": "6.34.00.0", "Firmware Version": "4.660.00-8211"},
      "Status": {"Controller Status": "Optimal", "BBU Status": 0},
      "HwCfg": {"On Board Memory Size": "2048 MB", "ROC temperature(Degree Celsius)": 47, "Backend Port Count": 8}
    }
  }]
}`

func TestParseStorcliShowAllCtl(t *testing.T) {
	rep, _ := parseStorcliJSON(storcliSampleNew)
	c := &rep.Controllers[0]
	parseStorcliShowAllCtl(storcliShowAll, c)
	if c.Firmware != "4.660.00-8211" {
		t.Errorf("Firmware = %q", c.Firmware)
	}
	if c.Temp != "47 °C" {
		t.Errorf("Temp = %q", c.Temp)
	}
	if c.Memory != "2048 MB" {
		t.Errorf("Memory = %q", c.Memory)
	}
	if c.CtlStatus != "Optimal" {
		t.Errorf("CtlStatus = %q", c.CtlStatus)
	}
	// 已有字段不覆盖：旧值保留
	rep2, _ := parseStorcliJSON(storcliSample)
	c2 := &rep2.Controllers[0]
	parseStorcliShowAllCtl(storcliShowAll, c2)
	if c2.Firmware != "5.130.00-2658" || c2.Temp != "45C" {
		t.Errorf("已有字段被覆盖: fw=%q temp=%q", c2.Firmware, c2.Temp)
	}
}

func TestNormalizeTemp(t *testing.T) {
	if normalizeTemp("47") != "47 °C" {
		t.Errorf("normalizeTemp(47) = %q", normalizeTemp("47"))
	}
	if normalizeTemp("45C") != "45C" {
		t.Errorf("normalizeTemp(45C) = %q", normalizeTemp("45C"))
	}
	if normalizeTemp("") != "" {
		t.Errorf("normalizeTemp(empty) = %q", normalizeTemp(""))
	}
}

// `/c0 /bbu show all J` 真实结构：BBU_Info 为数组。
const storcliBBUAll = `{
  "Controllers": [{
    "Response Data": {
      "Status": {"BBU Status": 0},
      "BBU_Info": [{
        "Battery Type": "CVPM02",
        "MfgDate": "Week 28 of 2016",
        "Voltage": "9.08 V",
        "Temperature": "31 C",
        "Full Charge Capacity": "357 mAh",
        "Remaining Capacity": "340 mAh",
        "State of Health": "100.00%",
        "Next Learn Time": "2026-12-01 02:00:00"
      }]
    }
  }]
}`

// CacheVault 真实结构：Cachevault_Info 映射，温度键为 Temp。
const storcliCVAll = `{
  "Controllers": [{
    "Response Data": {
      "Cachevault_Info": {
        "Device Name": "CVPM02",
        "State": "Optimal",
        "Temp": "28 C",
        "Capacitance": "100%"
      }
    }
  }]
}`

func TestParseBBUDetail(t *testing.T) {
	got := parseBBUDetail(storcliBBUAll)
	for _, want := range []string{"类型 CVPM02", "制造日期 Week 28 of 2016", "满充容量 357 mAh", "剩余容量 340 mAh", "健康度 100.00%", "下次学习 2026-12-01 02:00:00"} {
		if !strings.Contains(got, want) {
			t.Errorf("BBU 详情缺 %q: %q", want, got)
		}
	}
	cv := parseBBUDetail(storcliCVAll)
	for _, want := range []string{"类型 CVPM02", "温度 28 C", "电容 100%"} {
		if !strings.Contains(cv, want) {
			t.Errorf("CacheVault 详情缺 %q: %q", want, cv)
		}
	}
	if parseBBUDetail("no json") != "" {
		t.Errorf("非法输入应返回空")
	}
}

func TestParseBBUStateCacheVault(t *testing.T) {
	got := parseBBUState(storcliCVAll)
	if got != "Optimal（100%）" {
		t.Errorf("parseBBUState(CV) = %q", got)
	}
}

// PD 明细含温度/介质/接口（补齐结构化字段且详情串不重复）。
const storcliDetailTemp = `{
  "Controllers": [{
    "Response Data": {
      "Drive /c0/e8/s0 - Drive Information": {"SN": "ZL231X4A", "Power-On Hours": "30241", "Temperature": "34 C"}
    }
  }]
}`

func TestEnrichStorcliPDsTemp(t *testing.T) {
	rep, _ := parseStorcliJSON(storcliSample)
	c := rep.Controllers[0]
	enrichStorcliPDs(storcliDetailTemp, rep)
	p := c.PDs[0]
	if p.Temp != "34 C" {
		t.Errorf("Temp = %q", p.Temp)
	}
	if p.Power != "30241" {
		t.Errorf("Power = %q", p.Power)
	}
	if p.MediaType != "HDD" || p.Interface != "SAS" {
		t.Errorf("介质/接口未解析: %q / %q", p.MediaType, p.Interface)
	}
	// 详情串不应重复出现 HDD/SAS
	if strings.Count(p.Detail, "HDD") != 1 || strings.Count(p.Detail, "SAS") != 1 {
		t.Errorf("详情串重复: %q", p.Detail)
	}
}

// `/c0 /vALL show all J`：条带信息在 Response Data 顶层（新版）与 Data 键包装（旧版）两种形态。
const storcliVDAllNew = `{
  "Controllers": [{
    "Response Data": {
      "DG/VD": "0/0",
      "Strip Size": "128 KB",
      "Number of Drives Per Span": "5",
      "Span Depth": "1"
    }
  }]
}`

const storcliVDAllOld = `{
  "Controllers": [{
    "Data": {
      "DG/VD": "0/0",
      "Strip Size": "64 KB",
      "Number of Drives Per Span": "3",
      "Span Depth": "1"
    }
  }]
}`

func TestEnrichStorcliVDsCtl(t *testing.T) {
	rep, _ := parseStorcliJSON(storcliSampleNew)
	v := &rep.Controllers[0].VDs[0]
	enrichStorcliVDsCtl(storcliVDAllNew, &rep.Controllers[0])
	if v.Strip != "128 KB" || v.Spans != "5 盘 × 1 Span" {
		t.Errorf("Strip/Spans = %q / %q", v.Strip, v.Spans)
	}
	rep2, _ := parseStorcliJSON(storcliSampleNew)
	v2 := &rep2.Controllers[0].VDs[0]
	enrichStorcliVDsCtl(storcliVDAllOld, &rep2.Controllers[0])
	if v2.Strip != "64 KB" || v2.Spans != "3 盘 × 1 Span" {
		t.Errorf("旧版 Data 包装未解析: %q / %q", v2.Strip, v2.Spans)
	}
	enrichStorcliVDsCtl("no json", &rep.Controllers[0])
	if v.Strip != "128 KB" {
		t.Errorf("非法输入不应清空已有值")
	}
}
