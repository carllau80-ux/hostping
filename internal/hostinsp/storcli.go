package hostinsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"dbping/internal/model"
)

// parseStorcliJSON 解析 `storcli64 /cALL show J` 的 JSON 输出（防御式解析）。
// 结构大致为：
// {"Controllers":[{"Command Status":{"Status":"Success",...},
//   "Response Data":{"Version Info":{...},"Installed VD List"?...,"PD List":[...],"VD List":[...]}}]}
func parseStorcliJSON(out string) (*model.StorcliReport, error) {
	out = strings.TrimSpace(out)
	// storcli 可能输出告警行在 JSON 前/后：无论是否以 { 开头，都取第一个 { 到最后一个 }
	if i := strings.IndexByte(out, '{'); i >= 0 {
		if j := strings.LastIndexByte(out, '}'); j > i {
			out = out[i : j+1]
		}
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(out), &root); err != nil {
		return nil, err
	}
	rep := &model.StorcliReport{}
	ctlAny, ok := root["Controllers"].([]any)
	if !ok {
		return rep, nil
	}
	for i, c := range ctlAny {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		ctl := model.StorcliCtl{Index: fmt.Sprintf("%d", i+1)}
		if cs, ok := cm["Command Status"].(map[string]any); ok {
			ctl.Status = asString(cs["Status"])
		}
		rd, _ := cm["Response Data"].(map[string]any)
		if rd != nil {
			ctl.Model = firstNonEmpty(
				asString(rd["Product Name"]),
				asString(rd["Controller Model"]),
				asString(rd["Model"]),
			)
			// 控制器固件 / 温度 / 内存 / 电池（不同版本字段名有差异，防御式多键取值）
			ctl.Firmware = firstNonEmpty(
				asString(rd["Firmware Version"]),
				asString(rd["Version of Firmware"]),
				nestedString(rd, "Version Info", "Firmware Version"),
				nestedString(rd, "Version Info", "Version of Firmware"),
			)
			ctl.Temp = firstNonEmpty(
				asString(rd["Temperature"]),
				asString(rd["Temperature of Controller"]),
				asString(rd["ROC temperature"]),
				asString(rd["ROC Temperature"]),
				asString(rd["Controller Temperature"]),
			)
			ctl.Memory = firstNonEmpty(
				asString(rd["Memory"]),
				asString(rd["Memory Size"]),
				asString(rd["Cache Memory Size"]),
				asString(rd["Onboard Memory Size"]),
			)
			ctl.Battery = firstNonEmpty(
				asString(rd["Battery State"]),
				asString(rd["Battery Status"]),
				nestedString(rd, "Battery", "Battery State"),
				nestedString(rd, "Battery", "Status"),
			)
			// PD 列表
			if pds, ok := rd["PD List"].([]any); ok {
				for _, p := range pds {
					if pm, ok := p.(map[string]any); ok {
						ctl.PDs = append(ctl.PDs, model.StorcliPD{
							Slot:  firstNonEmpty(asString(pm["E:S"]), asString(pm["Slot"]), asString(pm["Slot Number"])),
							Model: firstNonEmpty(asString(pm["Model"]), asString(pm["Inquiry String"])),
							State: firstNonEmpty(asString(pm["State"]), asString(pm["Fw State"])),
							Size:  firstNonEmpty(asString(pm["Size"]), asString(pm["Drive Size"])),
							SN:    firstNonEmpty(asString(pm["SN"]), asString(pm["Serial Number"]), asString(pm["Serial number"])),
							Detail: joinNonEmpty([]string{
								asString(pm["Media Type"]), asString(pm["Interface Type"]),
								asString(pm["Temperature"]), asString(pm["Drive Position"]),
							}, " · "),
						})
					}
				}
			}
			// VD 列表
			if vds, ok := rd["VD List"].([]any); ok {
				for _, v := range vds {
					if vm, ok := v.(map[string]any); ok {
						ctl.VDs = append(ctl.VDs, model.StorcliVD{
							ID:     firstNonEmpty(asString(vm["VD"]), asString(vm["VD Id"])),
							Name:   firstNonEmpty(asString(vm["Name"]), asString(vm["VD Name"])),
							Type:   firstNonEmpty(asString(vm["Type"]), asString(vm["RAID Level"])),
							State:  firstNonEmpty(asString(vm["State"]), asString(vm["VD State"])),
							Size:   firstNonEmpty(asString(vm["Size"]), asString(vm["Size (MB)"])),
							Cache:  firstNonEmpty(asString(vm["Cac"]), asString(vm["Cache"]), asString(vm["Current Cache Mode"]), asString(vm["Access"])),
							Detail: joinNonEmpty([]string{asString(vm["Layout"]), asString(vm["Stripes"]), asString(vm["Drives"])}, " · "),
						})
					}
				}
			}
		}
		rep.Controllers = append(rep.Controllers, ctl)
	}
	return rep, nil
}

// nestedString 从 rd[sub][key] 取字符串（防御式）。
func nestedString(rd map[string]any, sub, key string) string {
	sm, ok := rd[sub].(map[string]any)
	if !ok {
		return ""
	}
	return asString(sm[key])
}

// enrichStorcliPDs 用 `/cALL /eALL /sALL show J` 明细补齐 PD 的 SN 与通电时长。
// 明细 JSON 的 Response Data 中每个盘的键形如 "Drive /c0/e252/s0 - Drive Information"。
func enrichStorcliPDs(out string, rep *model.StorcliReport) {
	if rep == nil {
		return
	}
	out = strings.TrimSpace(out)
	if i := strings.IndexByte(out, '{'); i >= 0 {
		if j := strings.LastIndexByte(out, '}'); j > i {
			out = out[i : j+1]
		}
	}
	var root map[string]any
	if json.Unmarshal([]byte(out), &root) != nil {
		return
	}
	ctlAny, ok := root["Controllers"].([]any)
	if !ok || len(ctlAny) == 0 {
		return
	}
	cm, ok := ctlAny[0].(map[string]any)
	if !ok {
		return
	}
	rd, _ := cm["Response Data"].(map[string]any)
	if rd == nil {
		return
	}
	// E:S → 明细
	bySlot := map[string]map[string]any{}
	for key, v := range rd {
		if !strings.Contains(key, "Drive /") {
			continue
		}
		dm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		// 从键 "Drive /c0/e252/s0 ..." 提取 e252/s0 → "252:0"
		es := extractESFromKey(key)
		if es != "" {
			bySlot[es] = dm
		}
	}
	if len(bySlot) == 0 {
		return
	}
	for ci := range rep.Controllers {
		for pi := range rep.Controllers[ci].PDs {
			pd := &rep.Controllers[ci].PDs[pi]
			dm, ok := bySlot[pd.Slot]
			if !ok {
				continue
			}
			if pd.SN == "" {
				pd.SN = firstNonEmpty(asString(dm["SN"]), asString(dm["Serial Number"]), asString(dm["Serial number"]))
			}
			if pd.Power == "" {
				pd.Power = firstNonEmpty(
					asString(dm["Power-On Hours"]),
					asString(dm["Power On Hours"]),
					asString(dm["Power-on Hours"]),
					asString(dm["Hours"]),
				)
			}
		}
	}
}

// extractESFromKey 从 "Drive /c0/e252/s0 - Drive Information" 提取 "252:0"。
func extractESFromKey(key string) string {
	i := strings.Index(key, "/c")
	if i < 0 {
		return ""
	}
	rest := key[i+2:] // c0/e252/s0...
	pe := strings.Index(rest, "/e")
	ps := strings.Index(rest, "/s")
	if pe < 0 || ps < 0 || ps < pe {
		return ""
	}
	enc := rest[pe+2 : ps]
	slot := rest[ps+2:]
	if j := strings.IndexAny(slot, " \t-"); j >= 0 {
		slot = slot[:j]
	}
	return enc + ":" + slot
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func joinNonEmpty(vals []string, sep string) string {
	var out []string
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, sep)
}

// parseStorcliLines 解析 -H 风格制表符分隔文本为行数组（空行/列数不足跳过）。
func parseStorcliLines(text string, minCols int) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		var fs []string
		for _, f := range strings.Split(line, "\t") {
			fs = append(fs, strings.TrimSpace(f))
		}
		if len(fs) < minCols {
			continue
		}
		rows = append(rows, fs)
	}
	return rows
}
