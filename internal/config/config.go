// Package config 配置持久化：JSON + AES-GCM 密码加密（密钥存本机用户目录）。
package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dbping/internal/model"
)

// Store 配置仓库。
type Store struct {
	mu           sync.Mutex
	dir          string
	DirPath      string
	Settings     model.Settings
	Conns        []model.Conn
	Hosts        []model.SSHHost
	BMCHosts     []model.BMCHost                     // BMC 带外连接（IPMI）
	NmapHistory  []model.NmapHistoryItem             // 类 nmap 扫描记录（最新在前，最多 50 条）
	SpeedHistory []model.SpeedEntry                  // 测速记录（最新在前，最多 50 条）
	HostHistory  map[string][]model.HostHistoryEntry // 主机巡检历史（hostID → 最新在前，每台最多 10 条）
	BMCHistory   map[string][]model.BMCHistoryEntry  // BMC 巡检历史（bmcID → 最新在前，每台最多 10 条）
	DBAIHistory  []model.DBAIEntry                   // 数据库 AI 分析历史（最新在前，最多 20 条）
	aiKeyEnc     string                              // AI API Key（密文，存 passwords["ai-key"]）
	deleted      map[string]bool                     // 已删除连接 ID（墓碑）：Save 合并磁盘数据时跳过，防止删除的连接"复活"
	key          []byte
}

const (
	cfgFile = "config.json"
	keyFile = "secret.key"
)

// userDirFn 可在测试中替换，便于隔离验证 Load/Save 行为。
var userDirFn = os.UserConfigDir

// Load 打开（或初始化）配置仓库。
func Load() (*Store, error) {
	base, err := userDirFn()
	if err != nil {
		base = "."
	}
	dir := filepath.Join(base, "DBPing")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, DirPath: dir, deleted: map[string]bool{}}
	s.Settings = model.Settings{AutoPing: false, IntervalSec: 60, PingTimeout: 5}

	key, err := loadOrCreateKey(filepath.Join(dir, keyFile))
	if err != nil {
		return nil, fmt.Errorf("初始化密钥失败: %w", err)
	}
	s.key = key

	data, err := os.ReadFile(filepath.Join(dir, cfgFile))
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[config] 首次运行，尚未有配置文件")
			return s, nil
		}
		log.Printf("[config] 读取配置失败: %v", err)
		return s, nil
	}
	var raw struct {
		Settings     model.Settings                      `json:"settings"`
		Conns        []model.Conn                        `json:"conns"`
		Hosts        []model.SSHHost                     `json:"hosts"`
		BMCHosts     []model.BMCHost                     `json:"bmcs"`
		NmapHistory  []model.NmapHistoryItem             `json:"nmapHistory"`
		SpeedHistory []model.SpeedEntry                  `json:"speedHistory"`
		HostHistory  map[string][]model.HostHistoryEntry `json:"hostHistory"`
		BMCHistory   map[string][]model.BMCHistoryEntry  `json:"bmcHistory"`
		DBAIHistory  []model.DBAIEntry                   `json:"dbAiHistory"`
		Passwords    map[string]string                   `json:"passwords"` // connID -> 加密串（含主机 h-/hk- 前缀键）
		Deleted      []string                            `json:"deleted"`   // 已删除连接 ID（墓碑）
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		// 配置损坏：留档并尝试从备份恢复
		log.Printf("[config] 配置解析失败，已留档: %v", err)
		stamp := time.Now().Format("20060102_150405")
		_ = os.Rename(filepath.Join(dir, cfgFile), filepath.Join(dir, "config.corrupt-"+stamp+".json"))
			if bak, err2 := os.ReadFile(filepath.Join(dir, "config.bak.json")); err2 == nil {
				var raw2 struct {
					Settings     model.Settings                      `json:"settings"`
					Conns        []model.Conn                        `json:"conns"`
					Hosts        []model.SSHHost                     `json:"hosts"`
					BMCHosts     []model.BMCHost                     `json:"bmcs"`
					NmapHistory  []model.NmapHistoryItem             `json:"nmapHistory"`
					SpeedHistory []model.SpeedEntry                  `json:"speedHistory"`
					HostHistory  map[string][]model.HostHistoryEntry `json:"hostHistory"`
					BMCHistory   map[string][]model.BMCHistoryEntry  `json:"bmcHistory"`
					DBAIHistory  []model.DBAIEntry                   `json:"dbAiHistory"`
					Passwords    map[string]string                   `json:"passwords"`
					Deleted      []string                            `json:"deleted"`
				}
			if json.Unmarshal(bak, &raw2) == nil {
				raw = raw2
				log.Printf("[config] 已从 config.bak.json 恢复 %d 个连接", len(raw.Conns))
			}
		}
	}
	for _, id := range raw.Deleted {
		s.deleted[id] = true
	}
	s.Settings = raw.Settings
	// SQL 控制台是会话级开关：每次启动强制关闭，不在配置中生效/持久化
	s.Settings.AdvancedMode = false
	if s.Settings.IntervalSec <= 0 {
		s.Settings.IntervalSec = 60
	}
	if s.Settings.PingTimeout <= 0 {
		s.Settings.PingTimeout = 5
	}
	s.Conns = raw.Conns
	s.NmapHistory = raw.NmapHistory
	s.SpeedHistory = raw.SpeedHistory
	if raw.HostHistory != nil {
		s.HostHistory = raw.HostHistory
	} else {
		s.HostHistory = map[string][]model.HostHistoryEntry{}
	}
	s.DBAIHistory = raw.DBAIHistory
	s.aiKeyEnc = raw.Passwords["ai-key"]
	for i := range s.Conns {
		if enc, ok := raw.Passwords[s.Conns[i].ID]; ok {
			s.Conns[i].Password = s.Decrypt(enc)
		}
	}
	// SSH 主机（密码/口令复用 passwords 侧表，键为 h-<hostID> 与 hk-<hostID>）
	s.Hosts = raw.Hosts
	for i := range s.Hosts {
		if enc, ok := raw.Passwords["h-"+s.Hosts[i].ID]; ok {
			s.Hosts[i].Password = s.Decrypt(enc)
		}
		if enc, ok := raw.Passwords["hk-"+s.Hosts[i].ID]; ok {
			s.Hosts[i].KeyPassphrase = s.Decrypt(enc)
		}
	}
	// BMC 连接（密码键 b-<bmcID>）
	s.BMCHosts = raw.BMCHosts
	for i := range s.BMCHosts {
		if enc, ok := raw.Passwords["b-"+s.BMCHosts[i].ID]; ok {
			s.BMCHosts[i].Password = s.Decrypt(enc)
		}
	}
	if raw.BMCHistory != nil {
		s.BMCHistory = raw.BMCHistory
	} else {
		s.BMCHistory = map[string][]model.BMCHistoryEntry{}
	}
	log.Printf("[config] 加载完成: %d 个连接, %d 个主机, %d 个 BMC", len(s.Conns), len(s.Hosts), len(s.BMCHosts))
	return s, nil
}

// Save 持久化（密码单独加密存储；写前合并磁盘最新数据防止覆盖丢失；原子写入）。
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1) 读取磁盘上的最新配置（其它实例或外部修改的连接要保留，避免覆盖丢失）
	diskConns := map[string]model.Conn{}
	diskPasswords := map[string]string{}
	diskDeleted := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join(s.dir, cfgFile)); err == nil {
		var disk struct {
			Conns     []model.Conn      `json:"conns"`
			Passwords map[string]string `json:"passwords"`
			Deleted   []string          `json:"deleted"`
		}
		if json.Unmarshal(data, &disk) == nil {
			for _, c := range disk.Conns {
				diskConns[c.ID] = c
			}
			diskPasswords = disk.Passwords
			for _, id := range disk.Deleted {
				diskDeleted[id] = true
			}
		}
	}
	// 合并双方墓碑：本实例删除的、以及其它实例已删除的
	for id := range diskDeleted {
		s.deleted[id] = true
	}

	// 2) 内存侧状态（密码加密落盘）
	inMem := map[string]bool{}
	passwords := map[string]string{}
	conns := make([]model.Conn, 0, len(s.Conns))
	for _, c := range s.Conns {
		inMem[c.ID] = true
		cc := c
		cc.Password = ""
		conns = append(conns, cc)
		if c.Password != "" {
			passwords[c.ID] = s.Encrypt(c.Password)
		} else if pw, ok := diskPasswords[c.ID]; ok {
			// 内存中密码为空（未改动）→ 沿用磁盘密文
			passwords[c.ID] = pw
		}
	}
	// SSH 主机密码（键 h-<id> 与 hk-<id>）
	hostPasswords := map[string]string{}
	for _, h := range s.Hosts {
		if h.Password != "" {
			hostPasswords["h-"+h.ID] = s.Encrypt(h.Password)
		} else if pw, ok := diskPasswords["h-"+h.ID]; ok {
			hostPasswords["h-"+h.ID] = pw
		}
		if h.KeyPassphrase != "" {
			hostPasswords["hk-"+h.ID] = s.Encrypt(h.KeyPassphrase)
		} else if pw, ok := diskPasswords["hk-"+h.ID]; ok {
			hostPasswords["hk-"+h.ID] = pw
		}
	}

	// BMC 连接密码（键 b-<bmcID>，命名空间隔离）
	for _, b := range s.BMCHosts {
		if b.Password != "" {
			passwords["b-"+b.ID] = s.Encrypt(b.Password)
		} else if pw, ok := diskPasswords["b-"+b.ID]; ok {
			passwords["b-"+b.ID] = pw
		}
	}

	// 3) 磁盘上有、内存中没有、且未被删除的连接 → 原样保留（含密文）
	for id, dc := range diskConns {
		if !inMem[id] && !s.deleted[id] {
			conns = append(conns, dc)
			if pw, ok := diskPasswords[id]; ok {
				passwords[id] = pw
			}
		}
	}
	// SSH 主机密码并入密码表（键 h-<id> / hk-<id> 命名空间隔离）
	for k, v := range hostPasswords {
		passwords[k] = v
	}
	// AI API Key（独立键，无密码输入时保留磁盘原值）
	if s.aiKeyEnc != "" {
		passwords["ai-key"] = s.aiKeyEnc
	} else if pw, ok := diskPasswords["ai-key"]; ok {
		passwords["ai-key"] = pw
	}
	// 清理墓碑：仅保留仍可能复活的（磁盘上仍存在该 ID）
	keptDeleted := make([]string, 0, len(s.deleted))
	for id := range s.deleted {
		if _, onDisk := diskConns[id]; onDisk || len(keptDeleted) < 200 {
			keptDeleted = append(keptDeleted, id)
		}
	}

	raw := struct {
		Version      int                                 `json:"version"`
		Settings     model.Settings                      `json:"settings"`
		Conns        []model.Conn                        `json:"conns"`
		Hosts        []model.SSHHost                     `json:"hosts"`
		BMCHosts     []model.BMCHost                     `json:"bmcs,omitempty"`
		NmapHistory  []model.NmapHistoryItem             `json:"nmapHistory,omitempty"`
		SpeedHistory []model.SpeedEntry                  `json:"speedHistory,omitempty"`
		HostHistory  map[string][]model.HostHistoryEntry `json:"hostHistory,omitempty"`
		BMCHistory   map[string][]model.BMCHistoryEntry  `json:"bmcHistory,omitempty"`
		DBAIHistory  []model.DBAIEntry                   `json:"dbAiHistory,omitempty"`
		Passwords    map[string]string                   `json:"passwords"`
		Deleted      []string                            `json:"deleted,omitempty"`
	}{Version: 1, Settings: s.Settings, Conns: conns, Hosts: s.Hosts, BMCHosts: s.BMCHosts, NmapHistory: s.NmapHistory, SpeedHistory: s.SpeedHistory, HostHistory: s.HostHistory, BMCHistory: s.BMCHistory, DBAIHistory: s.DBAIHistory, Passwords: passwords, Deleted: keptDeleted}
	// 清理内存中的墓碑集合为落盘集合
	s.deleted = map[string]bool{}
	for _, id := range keptDeleted {
		s.deleted[id] = true
	}
	// advancedMode 为会话级开关，落盘时强制关闭，保证下次启动为默认关闭状态
	raw.Settings.AdvancedMode = false

	// 4) 备份旧配置（留一份最近可回滚版本），再原子写入
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	cur := filepath.Join(s.dir, cfgFile)
	if prev, err := os.ReadFile(cur); err == nil {
		_ = os.WriteFile(filepath.Join(s.dir, "config.bak.json"), prev, 0o600)
	}
	tmp := filepath.Join(s.dir, cfgFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, cur); err != nil {
		return err
	}
	log.Printf("[config] 保存完成: %d 个连接, %d 个主机（含合并磁盘保留项）", len(conns), len(s.Hosts))
	return nil
}

// ---------- 连接管理 ----------

// ListConns 返回全部连接（保证非 nil，避免 JSON 序列化为 null）。
func (s *Store) ListConns() []model.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Conn, len(s.Conns))
	copy(out, s.Conns)
	return out
}

// GetConn 按 ID 查找。
func (s *Store) GetConn(id string) (model.Conn, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.Conns {
		if c.ID == id {
			return c, true
		}
	}
	return model.Conn{}, false
}

// UpsertConn 新增/更新连接（更新时密码留空表示保持不变）。
func (s *Store) UpsertConn(c model.Conn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.Name == "" {
		c.Name = c.Host
	}
	if c.Timeout <= 0 {
		c.Timeout = 5
	}
	// 重新添加同 ID 连接时清除墓碑
	if c.ID != "" {
		delete(s.deleted, c.ID)
	}
	for i := range s.Conns {
		if s.Conns[i].ID == c.ID {
			if c.Password == "" {
				c.Password = s.Conns[i].Password
			}
			s.Conns[i] = c
			return nil
		}
	}
	if c.ID == "" {
		c.ID = newID()
	}
	s.Conns = append(s.Conns, c)
	return nil
}

// DeleteConn 删除连接（同时记录墓碑，防止保存合并时从磁盘复活）。
func (s *Store) DeleteConn(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Conns {
		if s.Conns[i].ID == id {
			s.Conns = append(s.Conns[:i], s.Conns[i+1:]...)
			s.deleted[id] = true
			return true
		}
	}
	return false
}

// ---------- SSH 主机 CRUD ----------

// ListHosts 返回全部 SSH 主机（脱敏）。
func (s *Store) ListHosts() []model.SSHHost {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.SSHHost, len(s.Hosts))
	copy(out, s.Hosts)
	for i := range out {
		out[i] = out[i].Masked()
	}
	return out
}

// GetHost 按 ID 查找 SSH 主机（含密码/口令，Load/Upsert 时已解密入内存）。
func (s *Store) GetHost(id string) (model.SSHHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range s.Hosts {
		if h.ID == id {
			return h, true
		}
	}
	return model.SSHHost{}, false
}

// UpsertHost 新增/更新 SSH 主机。
func (s *Store) UpsertHost(h model.SSHHost) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h.Name == "" {
		h.Name = h.Host
	}
	if h.Timeout <= 0 {
		h.Timeout = 15
	}
	if h.ID != "" {
		delete(s.deleted, "h-"+h.ID)
	}
	for i := range s.Hosts {
		if s.Hosts[i].ID == h.ID {
			// 密码/口令留空表示保持不变
			if h.Password == "" {
				h.Password = s.Hosts[i].Password
			}
			if h.KeyPassphrase == "" {
				h.KeyPassphrase = s.Hosts[i].KeyPassphrase
			}
			s.Hosts[i] = h
			return nil
		}
	}
	if h.ID == "" {
		h.ID = "h" + newID()
	}
	s.Hosts = append(s.Hosts, h)
	return nil
}

// DeleteHost 删除 SSH 主机（记录墓碑防复活）。
func (s *Store) DeleteHost(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Hosts {
		if s.Hosts[i].ID == id {
			s.Hosts = append(s.Hosts[:i], s.Hosts[i+1:]...)
			s.deleted["h-"+id] = true
			return true
		}
	}
	return false
}

// ---------- BMC 连接 CRUD ----------

// ListBMCs 返回全部 BMC 连接（脱敏）。
func (s *Store) ListBMCs() []model.BMCHost {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.BMCHost, len(s.BMCHosts))
	copy(out, s.BMCHosts)
	for i := range out {
		out[i] = out[i].Masked()
	}
	return out
}

// GetBMC 按 ID 查找 BMC 连接（含密码，Load/Upsert 时已解密入内存）。
func (s *Store) GetBMC(id string) (model.BMCHost, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.BMCHosts {
		if b.ID == id {
			return b, true
		}
	}
	return model.BMCHost{}, false
}

// UpsertBMC 新增/更新 BMC 连接（密码留空表示保持不变）。
func (s *Store) UpsertBMC(b model.BMCHost) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.Name == "" {
		b.Name = b.Host
	}
	if b.Timeout <= 0 {
		b.Timeout = 10
	}
	if b.ID != "" {
		delete(s.deleted, "b-"+b.ID)
	}
	for i := range s.BMCHosts {
		if s.BMCHosts[i].ID == b.ID {
			if b.Password == "" {
				b.Password = s.BMCHosts[i].Password
			}
			s.BMCHosts[i] = b
			return nil
		}
	}
	if b.ID == "" {
		b.ID = "b" + newID()
	}
	s.BMCHosts = append(s.BMCHosts, b)
	return nil
}

// DeleteBMC 删除 BMC 连接（记录墓碑防复活）。
func (s *Store) DeleteBMC(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.BMCHosts {
		if s.BMCHosts[i].ID == id {
			s.BMCHosts = append(s.BMCHosts[:i], s.BMCHosts[i+1:]...)
			s.deleted["b-"+id] = true
			return true
		}
	}
	return false
}

// AddBMCHistory 追加 BMC 巡检历史（最新在前，每台最多 10 条）。调用方随后 Save。
func (s *Store) AddBMCHistory(bmcID string, r model.BMCReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.BMCHistory == nil {
		s.BMCHistory = map[string][]model.BMCHistoryEntry{}
	}
	list := s.BMCHistory[bmcID]
	list = append([]model.BMCHistoryEntry{{Time: time.Now().UnixMilli(), Report: r}}, list...)
	if len(list) > 10 {
		list = list[:10]
	}
	s.BMCHistory[bmcID] = list
}

// GetBMCHistory 返回 BMC 巡检历史（最新在前）。
func (s *Store) GetBMCHistory(bmcID string) []model.BMCHistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.BMCHistoryEntry, len(s.BMCHistory[bmcID]))
	copy(out, s.BMCHistory[bmcID])
	return out
}

// SaveSettings 更新设置。
func (s *Store) SaveSettings(st model.Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.IntervalSec <= 0 {
		st.IntervalSec = 60
	}
	if st.PingTimeout <= 0 {
		st.PingTimeout = 5
	}
	s.Settings = st
}

// ---------- AI 配置 ----------

// AIKey 返回 AI API Key（明文，仅后端调用）。
func (s *Store) AIKey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aiKeyEnc == "" {
		return ""
	}
	return s.Decrypt(s.aiKeyEnc)
}

// SetAIKey 保存 AI API Key（加密落盘；调用方随后调用 Save 持久化）。
func (s *Store) SetAIKey(plain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if plain == "" {
		s.aiKeyEnc = ""
		return
	}
	s.aiKeyEnc = s.Encrypt(plain)
}

// ---------- 类 nmap 扫描记录 ----------

// AddNmapHistory 追加一条扫描记录（最新在前，最多保留 50 条）。调用方随后调用 Save 持久化。
func (s *Store) AddNmapHistory(item model.NmapHistoryItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NmapHistory = append([]model.NmapHistoryItem{item}, s.NmapHistory...)
	if len(s.NmapHistory) > 50 {
		s.NmapHistory = s.NmapHistory[:50]
	}
}

// ListNmapHistory 返回全部扫描记录（最新在前）。
func (s *Store) ListNmapHistory() []model.NmapHistoryItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.NmapHistoryItem, len(s.NmapHistory))
	copy(out, s.NmapHistory)
	return out
}

// DeleteNmapHistoryEntry 删除指定时间的扫描记录。
func (s *Store) DeleteNmapHistoryEntry(t int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.NmapHistory {
		if s.NmapHistory[i].Time == t {
			s.NmapHistory = append(s.NmapHistory[:i], s.NmapHistory[i+1:]...)
			return true
		}
	}
	return false
}

// ---------- 测速历史 ----------

// AddSpeedEntry 追加测速记录（最新在前，最多保留 50 条）。调用方随后调用 Save 持久化。
func (s *Store) AddSpeedEntry(e model.SpeedEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SpeedHistory = append([]model.SpeedEntry{e}, s.SpeedHistory...)
	if len(s.SpeedHistory) > 50 {
		s.SpeedHistory = s.SpeedHistory[:50]
	}
}

// ListSpeedHistory 返回全部测速记录（最新在前）。
func (s *Store) ListSpeedHistory() []model.SpeedEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.SpeedEntry, len(s.SpeedHistory))
	copy(out, s.SpeedHistory)
	return out
}

// DeleteSpeedEntry 删除指定时间的测速记录。
func (s *Store) DeleteSpeedEntry(t int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.SpeedHistory {
		if s.SpeedHistory[i].Time == t {
			s.SpeedHistory = append(s.SpeedHistory[:i], s.SpeedHistory[i+1:]...)
			return true
		}
	}
	return false
}

// ---------- 巡检与 AI 分析历史 ----------

// AddHostHistory 追加主机巡检历史（最新在前，每台最多 10 条）。调用方随后调用 Save 持久化。
func (s *Store) AddHostHistory(hostID string, r model.HostReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.HostHistory == nil {
		s.HostHistory = map[string][]model.HostHistoryEntry{}
	}
	entry := model.HostHistoryEntry{Time: r.CollectedAt, Report: r}
	list := append([]model.HostHistoryEntry{entry}, s.HostHistory[hostID]...)
	if len(list) > 10 {
		list = list[:10]
	}
	s.HostHistory[hostID] = list
}

// AttachHostAI 将 AI 分析挂到匹配 CollectedAt 的巡检历史；无匹配则新增条目。
func (s *Store) AttachHostAI(hostID string, r model.HostReport, ai model.AIAnalysis) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.HostHistory == nil {
		s.HostHistory = map[string][]model.HostHistoryEntry{}
	}
	list := s.HostHistory[hostID]
	for i := range list {
		if list[i].Report.CollectedAt == r.CollectedAt {
			list[i].AI = &ai
			s.HostHistory[hostID] = list
			return
		}
	}
	entry := model.HostHistoryEntry{Time: time.Now().UnixMilli(), Report: r, AI: &ai}
	list = append([]model.HostHistoryEntry{entry}, list...)
	if len(list) > 10 {
		list = list[:10]
	}
	s.HostHistory[hostID] = list
}

// AttachBMCAI 将 AI 分析挂到匹配 CollectedAt 的 BMC 巡检历史；无匹配则新增条目。
func (s *Store) AttachBMCAI(bmcID string, r model.BMCReport, ai model.AIAnalysis) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.BMCHistory == nil {
		s.BMCHistory = map[string][]model.BMCHistoryEntry{}
	}
	list := s.BMCHistory[bmcID]
	for i := range list {
		if list[i].Report.CollectedAt == r.CollectedAt {
			list[i].AI = &ai
			s.BMCHistory[bmcID] = list
			return
		}
	}
	entry := model.BMCHistoryEntry{Time: time.Now().UnixMilli(), Report: r, AI: &ai}
	list = append([]model.BMCHistoryEntry{entry}, list...)
	if len(list) > 10 {
		list = list[:10]
	}
	s.BMCHistory[bmcID] = list
}

// GetHostHistory 返回主机巡检历史（最新在前）。
func (s *Store) GetHostHistory(hostID string) []model.HostHistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.HostHistoryEntry, len(s.HostHistory[hostID]))
	copy(out, s.HostHistory[hostID])
	return out
}

// AddDBAIEntry 追加数据库 AI 分析历史（最新在前，最多 20 条）。调用方随后调用 Save 持久化。
func (s *Store) AddDBAIEntry(e model.DBAIEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DBAIHistory = append([]model.DBAIEntry{e}, s.DBAIHistory...)
	if len(s.DBAIHistory) > 20 {
		s.DBAIHistory = s.DBAIHistory[:20]
	}
}

// GetDBAIHistory 返回数据库 AI 分析历史（最新在前）。
func (s *Store) GetDBAIHistory() []model.DBAIEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.DBAIEntry, len(s.DBAIHistory))
	copy(out, s.DBAIHistory)
	return out
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// ---------- 密码加密（AES-256-GCM） ----------

func loadOrCreateKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil && len(data) == 32 {
		return data, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt AES-GCM 加密，输出 "enc:v1:<base64>"。
func (s *Store) Encrypt(plain string) string {
	if plain == "" {
		return ""
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return ""
	}
	enc := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return "enc:v1:" + base64.StdEncoding.EncodeToString(enc)
}

// Decrypt 解密；非加密串或解密失败返回空。
func (s *Store) Decrypt(enc string) string {
	if !strings.HasPrefix(enc, "enc:v1:") {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(enc, "enc:v1:"))
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	if len(data) < gcm.NonceSize() {
		return ""
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return ""
	}
	return string(plain)
}

var ErrNotFound = errors.New("not found")

// fingerprintSHA256 供将来做机器绑定扩展（预留）。
func fingerprintSHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

