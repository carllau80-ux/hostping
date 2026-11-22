// Package bmc 通过 IPMI 2.0（RMCP+，UDP 623）对服务器 BMC 做带外巡检（纯 Go）。
package bmc

// ipmiManufacturers IPMI Manufacturer ID（IANA 企业号）→ 名称（常见厂商）。
var ipmiManufacturers = map[uint32]string{
	2:     "IBM",
	11:    "HP/HPE",
	343:   "Intel",
	674:   "Dell",
	918:   "浪潮 Inspur",
	19046: "联想 Lenovo",
	20181: "华为 Huawei",
	10876: "Supermicro",
	20942: "新华三 H3C",
	37257: "超聚变",
}

// manufacturerName 厂商名（未知时返回 "厂商ID 0x…"）。
func manufacturerName(id uint32) string {
	if n, ok := ipmiManufacturers[id]; ok {
		return n
	}
	return "厂商ID 0x" + hex8(id)
}

// sensorTypeCN IPMI 传感器类型码 → 中文名（常见类型，未收录的回退 "类型码 0x.."）。
var sensorTypeCN = map[uint8]string{
	0x01: "温度", 0x02: "电压", 0x03: "电流", 0x04: "风扇",
	0x05: "机箱", 0x06: "物理安全", 0x07: "处理器", 0x08: "电源模块",
	0x09: "电源冗余", 0x0A: "处理器电源", 0x0B: "电源单元", 0x0C: "内存",
	0x0D: "驱动槽", 0x0E: "后面板", 0x0F: "系统板", 0x10: "扩展板",
	0x11: "外设板", 0x12: "系统事件", 0x13: "严重中断", 0x14: "总线异常",
	0x15: "固件错误", 0x16: "引导错误", 0x17: "文件系统错误", 0x18: "关机",
	0x19: "挂起", 0x1A: "重启", 0x1B: "OS 停机", 0x1C: "OS 启动",
	0x1D: "OS 运行", 0x1E: "OS 加载", 0x1F: "严重停机", 0x20: "状态",
	0x25: "实体存在", 0x29: "电池", 0x2B: "版本变更", 0x2C: "FRU 状态",
	0xC2: "系统电源", 0xC3: "系统探测",
}

func sensorTypeName(code uint8) string {
	if n, ok := sensorTypeCN[code]; ok {
		return n
	}
	return "类型码 0x" + hex8(uint32(code))
}

// thresholdEventCN 阈值事件（事件类型 0x01）偏移 → 描述与严重级别。
var thresholdEventCN = map[uint8][2]string{
	0: {"转为正常", "info"},
	1: {"低于下限警告(LNC)", "warning"},
	2: {"低于下限严重(LCR)", "critical"},
	3: {"低于下限不可恢复(LNR)", "critical"},
	4: {"超过上限警告(UNC)", "warning"},
	5: {"超过上限严重(UCR)", "critical"},
	6: {"超过上限不可恢复(UNR)", "critical"},
}

// sensorSpecificEvents 传感器特定事件（事件类型 0x6F）偏移表：按常见传感器类型分组。
var sensorSpecificEvents = map[uint8]map[uint8][2]string{
	0x04: { // 风扇
		0: {"风扇冗余丢失", "warning"},
		1: {"风扇冗余恢复", "info"},
		2: {"转速低于下限严重", "critical"},
		3: {"转速低于下限不可恢复", "critical"},
	},
	0x07: { // 处理器
		0: {"处理器内部错误(IERR)", "critical"},
		1: {"处理器热跳闸", "critical"},
		2: {"处理器已存在", "info"},
		3: {"处理器已禁用", "warning"},
	},
	0x08: { // 电源模块
		0: {"电源模块已存在", "info"},
		1: {"电源模块故障", "critical"},
		2: {"电源模块预测故障", "warning"},
		3: {"输入丢失(AC 断电)", "critical"},
		4: {"输入丢失但仍在输出(整流失效)", "critical"},
		5: {"输入丢失且无输出", "critical"},
		6: {"电源配置无效", "critical"},
	},
	0x09: { // 电源冗余
		0: {"电源冗余完全丢失", "critical"},
		1: {"电源冗余降级", "warning"},
		2: {"电源冗余恢复", "info"},
	},
	0x0C: { // 内存
		0: {"可纠正 ECC 错误", "warning"},
		1: {"不可纠正 ECC 错误", "critical"},
		2: {"奇偶校验错误", "critical"},
		3: {"可纠正 ECC 达到记录上限", "warning"},
		5: {"内存已存在", "info"},
	},
	0x0D: { // 驱动槽
		0: {"驱动器已存在", "info"},
		1: {"驱动器故障", "critical"},
		2: {"驱动器预测故障", "warning"},
		3: {"驱动器在位异常", "warning"},
	},
	0x13: { // 严重中断
		0: {"NMI", "critical"},
		1: {"总线严重错误", "critical"},
		2: {"固件严重错误", "critical"},
	},
	0x25: { // 实体存在
		0: {"实体已存在", "info"},
		1: {"实体不存在", "warning"},
	},
	0x29: { // 电池
		0: {"电池丢失", "warning"},
		1: {"电池低于阈值", "warning"},
	},
}

// selEventText 解码 SEL 标准记录的事件描述与严重级别。
// eventDir: false=断言(assertion), true=恢复(deassertion)。
func selEventText(sensorType uint8, eventDir bool, eventType, offset uint8) (string, string) {
	if eventDir { // 恢复事件一律按 info 汇报（原事件严重级别已解除）
		t, _ := assertText(sensorType, eventType, offset)
		return "恢复: " + t, "info"
	}
	return assertText(sensorType, eventType, offset)
}

// assertText 断言事件文本与级别。
func assertText(sensorType, eventType, offset uint8) (string, string) {
	switch eventType {
	case 0x01: // 阈值事件
		if m, ok := thresholdEventCN[offset]; ok {
			return m[0], m[1]
		}
	case 0x02, 0x03, 0x04, 0x05: // 离散/数字状态类
		if offset == 0 {
			return "状态正常", "info"
		}
		return "状态异常(偏移 0x" + hex8(uint32(offset)) + ")", "warning"
	case 0x6F: // 传感器特定
		if m, ok := sensorSpecificEvents[sensorType]; ok {
			if e, ok := m[offset]; ok {
				return e[0], e[1]
			}
		}
		return sensorTypeName(sensorType) + " 特定事件(偏移 0x" + hex8(uint32(offset)) + ")", "warning"
	}
	return "事件类型 0x" + hex8(uint32(eventType)) + " 偏移 0x" + hex8(uint32(offset)), "warning"
}

// hex8/hex16/hex4 小写十六进制（无前缀）。
func hex8(v uint32) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[(v>>4)&0xF], digits[v&0xF]})
}

func hex4(v uint16) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 4)
	for i := 3; i >= 0; i-- {
		out[i] = digits[v&0xF]
		v >>= 4
	}
	return string(out)
}

func hex16(v uint32) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = digits[v&0xF]
		v >>= 4
	}
	return string(out)
}
