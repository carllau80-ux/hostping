package hostinsp

import "testing"

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
