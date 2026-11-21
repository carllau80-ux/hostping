// Package monitor 巡检调度器：并发 Ping 全部连接 + 定时自动巡检 + 历史记录。
package monitor

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"dbping/internal/model"
	"dbping/internal/provider"
)

// Monitor 巡检器。
type Monitor struct {
	mu      sync.Mutex
	history map[string][]model.PingResult // connID -> 最近 N 条
	loaded  bool

	onChange func([]model.PingResult) // 每轮巡检结束回调（用于推送前端事件）
}

const maxHistoryPerConn = 100

// New 创建巡检器。
func New(onChange func([]model.PingResult)) *Monitor {
	return &Monitor{history: map[string][]model.PingResult{}, onChange: onChange}
}

// PingOne 检查单个连接。
func (m *Monitor) PingOne(c *model.Conn) model.PingResult {
	res := m.ping(c)
	m.appendHistory(res)
	return res
}

// PingAll 并发检查全部连接（worker 池并发 12），返回全部结果。
func (m *Monitor) PingAll(conns []model.Conn) []model.PingResult {
	results := make([]model.PingResult, len(conns))
	sem := make(chan struct{}, 12)
	var wg sync.WaitGroup
	for i := range conns {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = m.ping(&conns[idx])
		}(i)
	}
	wg.Wait()
	m.mu.Lock()
	for _, r := range results {
		if r.ConnID != "" {
			m.pushHistoryLocked(r)
		}
	}
	m.mu.Unlock()
	if m.onChange != nil {
		go m.onChange(results)
	}
	return results
}

// ping 执行单个检查（含 panic 保护）。
func (m *Monitor) ping(c *model.Conn) (res model.PingResult) {
	res.ConnID = c.ID
	defer func() {
		if r := recover(); r != nil {
			res.OK = false
			res.Stage = "unknown"
			res.ErrorKind = "unknown"
			res.Error = fmt.Sprintf("[内部错误] 采集器异常: %v", r)
		}
	}()
	p, err := provider.Get(familyOf(c.Type))
	if err != nil {
		res.Stage = "unknown"
		res.ErrorKind = "config"
		res.Error = err.Error()
		res.CheckedAt = time.Now().UnixMilli()
		return res
	}
	r, err := p.Ping(context.Background(), c)
	if err != nil {
		res.Stage = "unknown"
		res.ErrorKind = "unknown"
		res.Error = "[内部错误] " + err.Error()
		res.CheckedAt = time.Now().UnixMilli()
		return res
	}
	return *r
}

// GetOverview 采集概览。
func (m *Monitor) GetOverview(c *model.Conn) (*model.Overview, error) {
	p, err := provider.Get(familyOf(c.Type))
	if err != nil {
		return nil, err
	}
	return p.Overview(context.Background(), c)
}

// GetDetail 采集明细（库/表/表空间/服务）。
func (m *Monitor) GetDetail(c *model.Conn) (*model.Detail, error) {
	p, err := provider.Get(familyOf(c.Type))
	if err != nil {
		return nil, err
	}
	return p.Detail(context.Background(), c)
}

// GetTopology 采集拓扑。
func (m *Monitor) GetTopology(c *model.Conn) (*model.Topology, error) {
	p, err := provider.Get(familyOf(c.Type))
	if err != nil {
		return nil, err
	}
	return p.Topology(context.Background(), c)
}

// RunAutoLoop 定时自动巡检循环（interval<=0 时退出）。
func (m *Monitor) RunAutoLoop(ctx context.Context, getConns func() []model.Conn, intervalSec int) {
	t := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			conns := getConns()
			if len(conns) == 0 {
				continue
			}
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("自动巡检异常: %v", r)
					}
				}()
				m.PingAll(conns)
			}()
		}
	}
}

// ---------- 历史 ----------

func (m *Monitor) appendHistory(r model.PingResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pushHistoryLocked(r)
}

func (m *Monitor) pushHistoryLocked(r model.PingResult) {
	h := append(m.history[r.ConnID], r)
	if len(h) > maxHistoryPerConn {
		h = h[len(h)-maxHistoryPerConn:]
	}
	m.history[r.ConnID] = h
}

// GetHistory 返回某连接的检查历史（时间正序，保证非 nil）。
func (m *Monitor) GetHistory(connID string) []model.PingResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.PingResult, len(m.history[connID]))
	copy(out, m.history[connID])
	sort.Slice(out, func(i, j int) bool { return out[i].CheckedAt < out[j].CheckedAt })
	return out
}

// Latest 返回每连接最近一次结果。
func (m *Monitor) Latest() map[string]model.PingResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]model.PingResult{}
	for id, h := range m.history {
		if len(h) > 0 {
			out[id] = h[len(h)-1]
		}
	}
	return out
}

// familyOf 由类型标识推 family（类型注册表反查）。
func familyOf(typ string) string {
	if ti, err := provider.TypeInfo(typ); err == nil {
		return ti.Family
	}
	return ""
}
