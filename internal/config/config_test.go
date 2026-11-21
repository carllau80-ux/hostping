package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dbping/internal/model"
)

// TestAdvancedModeNotPersisted 配置文件里即使残留 advancedMode=true，
// Load 也必须强制为 false（SQL 控制台为会话级开关，缺省启动关闭），Save 落盘同样强制 false。
func TestAdvancedModeNotPersisted(t *testing.T) {
	// 隔离的用户配置目录
	dir := t.TempDir()
	old := userDirFn
	userDirFn = func() (string, error) { return dir, nil }
	defer func() { userDirFn = old }()

	// 预置一份含 advancedMode=true 的旧配置与密钥（Store 实际目录为 dir/DBPing）
	cfgDir := filepath.Join(dir, "DBPing")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":1,"settings":{"autoPing":false,"advancedMode":true,"intervalSec":60,"pingTimeout":5},"conns":[],"passwords":{}}`
	if err := os.WriteFile(filepath.Join(cfgDir, cfgFile), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, keyFile), key, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if s.Settings.AdvancedMode {
		t.Fatalf("启动时 advancedMode 必须强制为 false")
	}
	if s.Settings.AutoPing {
		t.Fatalf("其它设置项应原样保留（fixture 中 autoPing=false）")
	}
	if s.Settings.IntervalSec != 60 {
		t.Fatalf("其它设置项应原样保留（intervalSec=60）")
	}

	// 会话中打开再保存 → 落盘也必须为 false
	s.Settings.AdvancedMode = true
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(s.DirPath, cfgFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `"advancedMode":true`) {
		t.Fatalf("落盘不应包含 advancedMode=true: %s", string(out))
	}
	var check struct {
		Settings struct {
			AdvancedMode bool `json:"advancedMode"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(out, &check); err != nil {
		t.Fatal(err)
	}
	if check.Settings.AdvancedMode {
		t.Fatalf("落盘的 advancedMode 应为 false")
	}
}

// TestDeleteDoesNotResurrect 删除连接后，Save 的磁盘合并逻辑不得把它加回来；
// 重启（再次 Load）也应保持已删除状态；即使磁盘被其它实例写回，墓碑也要挡住。
func TestDeleteDoesNotResurrect(t *testing.T) {
	dir := t.TempDir()
	old := userDirFn
	userDirFn = func() (string, error) { return dir, nil }
	defer func() { userDirFn = old }()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	// Store 实际目录为 dir/DBPing
	cfgDir := filepath.Join(dir, "DBPing")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, keyFile), key, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertConn(model.Conn{ID: "conn-b", Name: "B", Type: "sqlite", Host: "C:\\x.db"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	// 删除 B 并保存
	if !s.DeleteConn("conn-b") {
		t.Fatal("DeleteConn 应返回 true")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(s.DirPath, cfgFile))
	if strings.Contains(string(out), `"conn-b"`) && strings.Contains(string(out), `"conns"`) {
		// conns 段不应再有 conn-b（deleted 段允许出现）
		var chk struct {
			Conns []struct {
				ID string `json:"id"`
			} `json:"conns"`
		}
		if json.Unmarshal(out, &chk) == nil {
			for _, c := range chk.Conns {
				if c.ID == "conn-b" {
					t.Fatalf("保存后 conns 中不应再包含已删除的 conn-b")
				}
			}
		}
	}

	// 模拟重启：Load 后仍不应出现
	s2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range s2.ListConns() {
		if c.ID == "conn-b" {
			t.Fatalf("重启后已删除的连接不应复活")
		}
	}

	// 模拟另一个实例把 conn-b 写回磁盘（不带墓碑），本实例 Save 仍应剔除它
	disk := map[string]any{
		"version":   1,
		"settings":  map[string]any{"autoPing": false, "advancedMode": false, "intervalSec": 60, "pingTimeout": 5},
		"conns":     []map[string]any{{"id": "conn-b", "name": "B", "type": "sqlite", "host": "C:\\x.db"}},
		"passwords": map[string]any{},
	}
	d, _ := json.Marshal(disk)
	if err := os.WriteFile(filepath.Join(s.DirPath, cfgFile), d, 0o600); err != nil {
		t.Fatal(err)
	}
	// 用当前实例（内存中无 conn-b、有墓碑）再保存
	if err := s2.Save(); err != nil {
		t.Fatal(err)
	}
	out2, _ := os.ReadFile(filepath.Join(s.DirPath, cfgFile))
	var chk2 struct {
		Conns []struct {
			ID string `json:"id"`
		} `json:"conns"`
	}
	if err := json.Unmarshal(out2, &chk2); err != nil {
		t.Fatal(err)
	}
	for _, c := range chk2.Conns {
		if c.ID == "conn-b" {
			t.Fatalf("其它实例写回的已删除连接不应复活（墓碑失效）; 文件内容=%s", string(out2))
		}
	}
}
func TestSaveMergesDiskConns(t *testing.T) {
	dir := t.TempDir()
	old := userDirFn
	userDirFn = func() (string, error) { return dir, nil }
	defer func() { userDirFn = old }()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	// Store 实际目录为 dir/DBPing
	cfgDir := filepath.Join(dir, "DBPing")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, keyFile), key, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// 内存中只有一个连接 A
	if err := s.UpsertConn(model.Conn{ID: "conn-a", Name: "A", Type: "pg", Host: "h1", Port: 5432}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	// 模拟另一个实例/外部修改：磁盘上多出连接 B（含密文占位密码）
	disk := map[string]any{
		"version":   1,
		"settings":  map[string]any{"autoPing": false, "advancedMode": false, "intervalSec": 60, "pingTimeout": 5},
		"conns":     []map[string]any{{"id": "conn-a", "name": "A", "type": "pg", "host": "h1", "port": 5432}, {"id": "conn-b", "name": "B", "type": "mysql", "host": "h2", "port": 3306}},
		"passwords": map[string]any{"conn-b": "cipher-text-b"},
	}
	d, _ := json.Marshal(disk)
	if err := os.WriteFile(filepath.Join(s.DirPath, cfgFile), d, 0o600); err != nil {
		t.Fatal(err)
	}

	// 内存侧（旧状态）再保存：磁盘上的 conn-b 必须保留
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(s.DirPath, cfgFile))
	if err != nil {
		t.Fatal(err)
	}
	var check struct {
		Conns []struct {
			ID string `json:"id"`
		} `json:"conns"`
		Passwords map[string]string `json:"passwords"`
	}
	if err := json.Unmarshal(out, &check); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range check.Conns {
		ids[c.ID] = true
	}
	if !ids["conn-a"] || !ids["conn-b"] {
		t.Fatalf("保存后应同时包含 conn-a 与 conn-b（合并磁盘数据）: %v", ids)
	}
	if check.Passwords["conn-b"] != "cipher-text-b" {
		t.Fatalf("conn-b 的密文密码应原样保留")
	}
}


