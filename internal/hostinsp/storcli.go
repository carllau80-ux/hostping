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
			ctl.CtlStatus = firstNonEmpty(
				nestedString(rd, "Status", "Controller Status"),
				asString(rd["Controller Status"]),
			)
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
			// PD 列表（storcli 新版键名为 "PD LIST"/"EID:Slt"，旧版为 "PD List"/"E:S"，防御式兼容）
			if pds, ok := listAt(rd, "PD LIST", "PD List"); ok {
				for _, p := range pds {
					if pm, ok := p.(map[string]any); ok {
						ctl.PDs = append(ctl.PDs, model.StorcliPD{
							Slot:  firstNonEmpty(asString(pm["EID:Slt"]), asString(pm["E:S"]), asString(pm["Slot"]), asString(pm["Slot Number"])),
							Model: firstNonEmpty(asString(pm["Model"]), asString(pm["Inquiry String"])),
							State: firstNonEmpty(asString(pm["State"]), asString(pm["Fw State"])),
							Size:  firstNonEmpty(asString(pm["Size"]), asString(pm["Drive Size"])),
							SN:    firstNonEmpty(asString(pm["SN"]), asString(pm["Serial Number"]), asString(pm["Serial number"])),
							MediaType: firstNonEmpty(asString(pm["Med"]), asString(pm["Media Type"])),
							Interface: firstNonEmpty(asString(pm["Intf"]), asString(pm["Interface Type"])),
							Detail: joinNonEmpty([]string{
								firstNonEmpty(asString(pm["Media Type"]), asString(pm["Med"])),
								firstNonEmpty(asString(pm["Interface Type"]), asString(pm["Intf"])),
								asString(pm["Temperature"]), asString(pm["Drive Position"]),
							}, " · "),
						})
					}
				}
			}
			// VD 列表（新版 "VD LIST"，旧版 "VD List"；ID 可能形如 "DG/VD"）
			if vds, ok := listAt(rd, "VD LIST", "VD List"); ok {
				for _, v := range vds {
					if vm, ok := v.(map[string]any); ok {
						ctl.VDs = append(ctl.VDs, model.StorcliVD{
							ID:     firstNonEmpty(asString(vm["DG/VD"]), asString(vm["VD"]), asString(vm["VD Id"])),
							Name:   firstNonEmpty(asString(vm["Name"]), asString(vm["NAME"]), asString(vm["VD Name"])),
							Type:   firstNonEmpty(asString(vm["Type"]), asString(vm["TYPE"]), asString(vm["RAID Level"])),
							State:  firstNonEmpty(asString(vm["State"]), asString(vm["STATE"]), asString(vm["VD State"])),
							Size:   firstNonEmpty(asString(vm["Size"]), asString(vm["SIZE"]), asString(vm["Size (MB)"])),
							Cache:  firstNonEmpty(asString(vm["Cache"]), asString(vm["Cac"]), asString(vm["CACHE"]), asString(vm["Current Cache Mode"]), asString(vm["Access"])),
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

// listAt 返回 rd 中第一个存在的数组键（兼容 storcli 新旧键名大小写差异）。
func listAt(rd map[string]any, keys ...string) ([]any, bool) {
	for _, k := range keys {
		if v, ok := rd[k].([]any); ok {
			return v, true
		}
	}
	return nil, false
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
			if pd.Temp == "" {
				pd.Temp = firstNonEmpty(
					asString(dm["Temperature"]),
					asString(dm["Drive Temperature"]),
					asString(dm["Temperature of Drive"]),
				)
			}
			if pd.MediaType == "" {
				pd.MediaType = firstNonEmpty(asString(dm["Med"]), asString(dm["Media Type"]))
			}
			if pd.Interface == "" {
				pd.Interface = firstNonEmpty(asString(dm["Intf"]), asString(dm["Interface Type"]))
			}
			// 明细补齐的字段同步追加到详情串（去重，避免重复展示）
			pd.Detail = joinNonEmpty(dedupWords(append(
				splitWords(pd.Detail),
				pd.MediaType, pd.Interface, pd.Temp,
			)), " · ")
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

// extractJSON 剥离 storcli 输出中的 JSON 体（告警行可能出现在前后）。
func extractJSON(out string) string {
	out = strings.TrimSpace(out)
	if i := strings.IndexByte(out, '{'); i >= 0 {
		if j := strings.LastIndexByte(out, '}'); j > i {
			return out[i : j+1]
		}
	}
	return out
}

// storcliCtlResponse 取第 i 个控制器的 Response Data / Data 映射（旧版本 VD 详情键名为 Data）。
func storcliCtlResponse(out string, i int) map[string]any {
	var root map[string]any
	if json.Unmarshal([]byte(extractJSON(out)), &root) != nil {
		return nil
	}
	ctlAny, ok := root["Controllers"].([]any)
	if !ok || i >= len(ctlAny) {
		return nil
	}
	cm, ok := ctlAny[i].(map[string]any)
	if !ok {
		return nil
	}
	if rd, ok := cm["Response Data"].(map[string]any); ok {
		return rd
	}
	rd, _ := cm["Data"].(map[string]any)
	return rd
}

// parseStorcliShowAllCtl 用 `/cN show all J` 富化单个控制器的固件/温度/内存/控制器状态（仅填空缺字段）。
// 真实输出结构：控制器信息在 Response Data 的嵌套段里 —— "Version"/"HwCfg"/"Basics"/"Status"，
// 温度键为 "ROC temperature(Degree Celsius)"（部分版本括号前带空格）。
func parseStorcliShowAllCtl(out string, c *model.StorcliCtl) {
	if c == nil {
		return
	}
	rd := storcliCtlResponse(out, 0)
	if rd == nil {
		return
	}
	if c.Firmware == "" {
		c.Firmware = firstNonEmpty(
			nestedString(rd, "Version", "Firmware Version"),
			nestedString(rd, "Version Info", "Firmware Version"),
			asString(rd["Firmware Version"]),
			asString(rd["Version of Firmware"]),
			nestedString(rd, "Version Info", "Version of Firmware"),
		)
	}
	if c.Temp == "" {
		c.Temp = normalizeTemp(firstNonEmpty(
			nestedString(rd, "HwCfg", "ROC temperature(Degree Celsius)"),
			nestedString(rd, "HwCfg", "ROC temperature (Degree Celsius)"),
			nestedString(rd, "HwCfg", "ROC Temperature(Degree Celsius)"),
			nestedString(rd, "HwCfg", "ROC temperature"),
			nestedString(rd, "HwCfg", "ROC Temperature"),
			asString(rd["ROC temperature(Degree Celsius)"]),
			asString(rd["ROC Temperature(Degree Celsius)"]),
			asString(rd["ROC temperature"]),
			asString(rd["ROC Temperature"]),
			asString(rd["Temperature of Controller"]),
			asString(rd["Temperature"]),
			asString(rd["Controller Temperature"]),
		))
	}
	if c.Memory == "" {
		c.Memory = firstNonEmpty(
			nestedString(rd, "HwCfg", "On Board Memory Size"),
			nestedString(rd, "HwCfg", "Onboard Memory Size"),
			nestedString(rd, "HwCfg", "Memory Size"),
			nestedString(rd, "HwCfg", "Memory"),
			asString(rd["Memory"]),
			asString(rd["Memory Size"]),
			asString(rd["Cache Memory Size"]),
			asString(rd["Onboard Memory Size"]),
		)
	}
	if c.CtlStatus == "" {
		c.CtlStatus = firstNonEmpty(
			nestedString(rd, "Status", "Controller Status"),
			asString(rd["Controller Status"]),
		)
	}
	if c.Model == "" {
		c.Model = firstNonEmpty(
			nestedString(rd, "Basics", "Model"),
			nestedString(rd, "HwCfg", "Model"),
		)
	}
}

// storcliBBUMap 从 show all 输出的 Response Data 中定位 BBU / CacheVault 信息映射。
// 真实键名："BBU_Info" 为数组（取 [0]），"Cachevault_Info" 为映射；兼容带空格的旧变体。
func storcliBBUMap(rd map[string]any) map[string]any {
	for _, key := range []string{"Cachevault_Info", "Cachevault Info", "Cache Vault Info", "BBU Info", "Battery"} {
		if sm, ok := rd[key].(map[string]any); ok && len(sm) > 0 {
			return sm
		}
	}
	if arr, ok := rd["BBU_Info"].([]any); ok {
		for _, e := range arr {
			if m, ok := e.(map[string]any); ok && len(m) > 0 {
				return m
			}
		}
	}
	return rd
}

// parseBBUDetail 从 `/cN /bbu show all J`（或 CacheVault 的 `/cN /cv show all J`）提取电池详情串。
// CacheVault 用 "Temp" 表示温度、"Capacitance"/"SOH" 表示健康度。
func parseBBUDetail(out string) string {
	rd := storcliCtlResponse(out, 0)
	if rd == nil {
		return ""
	}
	m := storcliBBUMap(rd)
	var segs []string
	add := func(label string, keys ...string) {
		v := firstNonEmpty(mapKeys(m, keys...)...)
		if v != "" {
			segs = append(segs, label+" "+v)
		}
	}
	add("类型", "Battery Type", "Device Name")
	add("制造日期", "MfgDate")
	add("电压", "Voltage")
	add("温度", "Temperature", "Temp")
	add("设计容量", "Design Capacity")
	add("满充容量", "Full Charge Capacity")
	add("剩余容量", "Remaining Capacity", "Current Capacity")
	add("健康度", "State of Health", "SOH")
	add("电容", "Capacitance")
	add("循环次数", "Cycle Count")
	add("下次学习", "Next Learn Time", "Next Learn")
	return strings.Join(segs, " · ")
}

// enrichStorcliVDsCtl 用 `/cN /vALL show all J` 补齐单个控制器 VD 的条带大小与 Span 结构（仅填空缺字段）。
// 详情可能直接在 Response Data 顶层或形如 "VD0 - Detailed Information" 的子映射中，按 ID 匹配、退化为顺序匹配。
func enrichStorcliVDsCtl(out string, c *model.StorcliCtl) {
	if c == nil || len(c.VDs) == 0 {
		return
	}
	rd := storcliCtlResponse(out, 0)
	if rd == nil {
		return
	}
	// 收集所有含条带信息的映射：顶层本身 + 子映射
	var details []map[string]any
	if _, ok := rd["Strip Size"]; ok {
		details = append(details, rd)
	}
	for _, v := range rd {
		if m, ok := v.(map[string]any); ok {
			if _, ok := m["Strip Size"]; ok {
				details = append(details, m)
			}
		}
	}
	// 先按 DG/VD 精确匹配
	matched := make([]bool, len(details))
	for vi := range c.VDs {
		for di, d := range details {
			if matched[di] || asString(d["DG/VD"]) != c.VDs[vi].ID {
				continue
			}
			fillVD(&c.VDs[vi], d)
			matched[di] = true
			break
		}
	}
	// 再把未匹配的详情按顺序填给仍未填的 VD
	vi := 0
	for di := range details {
		if matched[di] {
			continue
		}
		for vi < len(c.VDs) && (c.VDs[vi].Strip != "" || c.VDs[vi].Spans != "") {
			vi++
		}
		if vi >= len(c.VDs) {
			break
		}
		fillVD(&c.VDs[vi], details[di])
		vi++
	}
}

// fillVD 从 VD 详情映射填充条带与 Span 结构。
func fillVD(v *model.StorcliVD, d map[string]any) {
	if v.Strip == "" {
		v.Strip = firstNonEmpty(asString(d["Strip Size"]), asString(d["Strip"]))
	}
	if v.Spans == "" {
		perSpan := firstNonEmpty(asString(d["Number of Drives Per Span"]), asString(d["Drives Per Span"]))
		depth := firstNonEmpty(asString(d["Span Depth"]), asString(d["Spans"]))
		if perSpan != "" && depth != "" {
			v.Spans = perSpan + " 盘 × " + depth + " Span"
		} else {
			v.Spans = firstNonEmpty(perSpan, depth)
		}
	}
}

// normalizeTemp 纯数字温度补 "°C" 单位（键名已注明摄氏度）。
func normalizeTemp(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if isNumeric(s) {
		return s + " °C"
	}
	return s
}

func isNumeric(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return len(s) > 0
}

// mapKeys 返回 m 中第一个非空键值（多键名防御取值）。
func mapKeys(m map[string]any, keys ...string) []string {
	for _, k := range keys {
		if v := asString(m[k]); v != "" {
			return []string{v}
		}
	}
	return nil
}

// splitWords 按 " · " 拆分详情串。
func splitWords(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, " · ")
}

// dedupWords 去掉空串与重复项（保持顺序）。
func dedupWords(vals []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
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
