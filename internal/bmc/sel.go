package bmc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"dbping/internal/model"

	ipmi "github.com/bougou/go-ipmi/pkg/client"
	ipmitypes "github.com/bougou/go-ipmi/pkg/types"
)

// maxSELEntries 单次巡检保留的最新 SEL 条数（超长日志滚动保留，避免超大日志拖垮 UI）。
const maxSELEntries = 1000

// maxSELWalked 最多遍历的 SEL 条数（SEL 链从最旧开始，需遍历到尾部才能取到最新记录）。
const maxSELWalked = 4000

// collectSEL 拉取并解码 SEL（System Event Log），按时间倒序展示，规则化分析。
// SEL 链按时间从最旧到最新排列：滚动窗口保留最新 maxSELEntries 条。
func collectSEL(cli *ipmi.Client, r *model.BMCReport) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	info, err := cli.GetSELInfo(ctx)
	if err != nil {
		r.SELNote = "SEL 信息读取失败：" + shortErr(err)
		return
	}
	rep := &model.BMCSELReport{Total: fmt.Sprintf("%d", info.Entries)}
	if info.Entries == 0 {
		rep.Analysis = "SEL 为空（无历史事件）"
		r.SEL = rep
		return
	}
	kept := make([]model.BMCSEL, 0, maxSELEntries)
	recordID := uint16(0)
	walked, skip, truncated := 0, 0, false
	for walked < maxSELWalked {
		entry, err := cli.GetSELEntry(ctx, 0, recordID)
		if err != nil {
			rep.Note = joinNote(rep.Note, "遍历中断于记录 0x"+hex4(recordID)+"："+shortErr(err))
			break
		}
		walked++
		sel, err := ipmitypes.ParseSEL(entry.Data)
		if err != nil {
			skip++
		} else if e := decodeSEL(sel); e != nil {
			kept = append(kept, *e)
			if len(kept) > maxSELEntries { // 滚动窗口：只保留最新 N 条
				kept = kept[1:]
				truncated = true
			}
		} else {
			skip++
		}
		if entry.NextRecordID == 0xFFFF {
			break
		}
		recordID = entry.NextRecordID
	}
	if truncated {
		rep.Note = joinNote(rep.Note, fmt.Sprintf("日志较大，已遍历 %d 条并保留最新 %d 条", walked, len(kept)))
	}
	if skip > 0 {
		rep.Note = joinNote(rep.Note, fmt.Sprintf("跳过 %d 条无法解析/OEM 记录", skip))
	}
	rep.Entries = kept
	// 按时间倒序（SEL RecordID 不保证时序）
	sort.Slice(rep.Entries, func(i, j int) bool {
		return rep.Entries[i].Time > rep.Entries[j].Time
	})
	for _, e := range rep.Entries {
		switch e.Severity {
		case "critical":
			rep.CriticalCount++
		case "warning":
			rep.WarningCount++
		default:
			rep.InfoCount++
		}
	}
	rep.TopErrors = topSELErrors(rep.Entries)
	rep.Analysis = analyzeSEL(rep)
	r.SEL = rep
}

// decodeSEL 解码单条 SEL 记录（仅标准记录；OEM 返回 nil 计为跳过）。
func decodeSEL(sel *ipmitypes.SEL) *model.BMCSEL {
	if sel == nil || sel.Standard == nil {
		return nil
	}
	st := sel.Standard
	event, severity := selEventText(uint8(st.SensorType), st.EventDir == ipmitypes.EventDirDeassertion, uint8(st.EventReadingType), uint8(st.EventData.EventData1&0x0F))
	return &model.BMCSEL{
		Time:     st.Timestamp.Format("2006-01-02 15:04:05"),
		RecordID: "0x" + hex4(sel.RecordID),
		Sensor:   fmt.Sprintf("%s #%d", sensorTypeName(uint8(st.SensorType)), st.SensorNumber),
		Event:    event,
		Severity: severity,
	}
}

// topSELErrors 非恢复事件按描述聚合计数，取 Top 5。
func topSELErrors(entries []model.BMCSEL) []string {
	count := map[string]int{}
	for _, e := range entries {
		if e.Severity == "info" || strings.HasPrefix(e.Event, "恢复:") {
			continue
		}
		key := e.Sensor + " " + e.Event
		count[key]++
	}
	type kv struct {
		k string
		n int
	}
	var list []kv
	for k, n := range count {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	var out []string
	for i, e := range list {
		if i >= 5 {
			break
		}
		out = append(out, fmt.Sprintf("×%d %s", e.n, e.k))
	}
	return out
}

// analyzeSEL 规则化分析文本（供报告与 AI 上下文）。
func analyzeSEL(rep *model.BMCSELReport) string {
	if len(rep.Entries) == 0 {
		return "SEL 无可解析记录"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "SEL 共 %s 条：严重 %d / 警告 %d / 信息 %d", rep.Total, rep.CriticalCount, rep.WarningCount, rep.InfoCount)
	// 最近一次严重事件
	for _, e := range rep.Entries {
		if e.Severity == "critical" {
			fmt.Fprintf(&b, "；最近严重事件：%s %s %s", e.Time, e.Sensor, e.Event)
			break
		}
	}
	// 最近 24h 事件量
	day := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	recent := 0
	for _, e := range rep.Entries {
		if strings.HasPrefix(e.Time, day) && e.Severity != "info" {
			recent++
		}
	}
	if recent > 0 {
		fmt.Fprintf(&b, "；最近 24h 异常事件 %d 条，建议关注", recent)
	}
	if len(rep.TopErrors) > 0 {
		fmt.Fprintf(&b, "；高频错误：%s", strings.Join(rep.TopErrors, "；"))
	}
	return b.String()
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + "；" + b
}
