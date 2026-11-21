package provider

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"dbping/internal/model"

	"github.com/redis/go-redis/v9"
)

// redisProvider Redis（go-redis v9，RESP 协议纯 Go）。
type redisProvider struct{}

func (redisProvider) client(c *model.Conn) *redis.Client {
	dbIdx := 0
	if c.Database != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(c.Database)); err == nil {
			dbIdx = n
		}
	}
	return redis.NewClient(&redis.Options{
		Addr:        net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Password:    c.Password,
		DB:          dbIdx,
		DialTimeout: timeoutDur(c),
		ReadTimeout: timeoutDur(c) * 2,
	})
}

// parseInfo 将 INFO 文本解析为 map。
func parseInfo(s string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(s, "\r\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, ':'); i > 0 {
			m[line[:i]] = line[i+1:]
		}
	}
	return m
}

func (redisProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	res := &model.PingResult{ConnID: c.ID, Stage: "tcp", CheckedAt: time.Now().UnixMilli()}
	start := time.Now()
	tcpMs, err := DialTCP(c)
	if err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.TCPMs = tcpMs

	ctx, cancel := DBCtx(c)
	defer cancel()
	rdb := redisProvider{}.client(c)
	defer rdb.Close()

	res.Stage = "auth"
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.Stage = "sql"
	s := time.Now()
	if _, err := rdb.Info(ctx, "server").Result(); err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.SQLMs = time.Since(s).Milliseconds()
	if res.SQLMs <= 0 {
		res.SQLMs = 1
	}
	res.Stage = "ok"
	res.OK = true
	res.TotalMs = time.Since(start).Milliseconds()
	return res, nil
}

func (redisProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	ctx, cancel := DBCtx(c)
	defer cancel()
	rdb := redisProvider{}.client(c)
	defer rdb.Close()

	info, err := rdb.Info(ctx, "server", "clients", "memory", "persistence", "replication", "keyspace", "cluster").Result()
	if err != nil {
		return nil, err
	}
	m := parseInfo(info)
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	o.Version = m["redis_version"]
	o.VersionFull = fmt.Sprintf("Redis %s（%s · %s）", m["redis_version"], m["redis_mode"], m["os"])
	o.Instance = m["redis_mode"]
	o.OS = m["os"]
	o.Hostname = orDefault(m["hostname"], m["os"])
	if up, err := strconv.ParseInt(m["uptime_in_seconds"], 10, 64); err == nil {
		o.Uptime = HumanDuration(up)
	}

	// 角色：主从/集群/单机
	switch {
	case m["cluster_enabled"] == "1":
		o.Cluster = "Cluster 分片集群"
		o.Role = "集群节点"
	case m["role"] == "master" && m["connected_slaves"] != "" && m["connected_slaves"] != "0":
		o.Cluster = "主从复制（" + m["connected_slaves"] + " 从）"
		o.Role = "主库"
	case m["role"] == "master":
		o.Cluster = "单机"
		o.Role = "主库"
	case m["role"] == "slave" || m["role"] == "replica":
		o.Cluster = "主从复制"
		o.Role = "从库（主: " + m["master_host"] + ":" + m["master_port"] + "）"
	}

	// 持久化（归档对应物）
	parts := []string{}
	if m["rdb_last_bgsave_status"] != "" {
		parts = append(parts, "RDB:"+m["rdb_last_bgsave_status"])
	}
	if m["aof_enabled"] == "1" {
		parts = append(parts, "AOF:已开启")
	} else if m["aof_enabled"] == "0" {
		parts = append(parts, "AOF:未开启")
	}
	o.ArchiveMode = strings.Join(parts, " · ")

	o.Charset = "-"
	o.Timezone = "-"
	o.DBSize = m["used_memory_human"] + "（内存）"
	o.ConnCur = m["connected_clients"]
	o.ConnMax = m["maxclients"]
	o.ReadOnly = "-"

	// 键总数
	totalKeys := int64(0)
	for k, v := range m {
		if strings.HasPrefix(k, "db") && strings.Contains(v, "keys=") {
			if i := strings.Index(v, "keys="); i >= 0 {
				rest := v[i+5:]
				if j := strings.Index(rest, ","); j >= 0 {
					rest = rest[:j]
				}
				totalKeys += parseI64(rest)
			}
		}
	}
	if totalKeys > 0 {
		o.TableCount = fmt.Sprintf("%d 个键", totalKeys)
	} else {
		o.TableCount = "0 个键"
	}
	o.Extra = append(o.Extra,
		model.KV{Key: "运行模式", Value: m["redis_mode"]},
		model.KV{Key: "内存峰值", Value: m["used_memory_peak_human"]},
		model.KV{Key: "mem_allocator", Value: m["mem_allocator"]},
	)
	return o, nil
}

func (redisProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	ctx, cancel := DBCtx(c)
	defer cancel()
	rdb := redisProvider{}.client(c)
	defer rdb.Close()
	d := &model.Detail{}

	info, err := rdb.Info(ctx, "keyspace", "replication", "cluster").Result()
	if err != nil {
		return nil, err
	}
	m := parseInfo(info)

	// 键空间（dbN → 键数）
	for k, v := range m {
		if strings.HasPrefix(k, "db") && strings.Contains(v, "keys=") {
			kv := strings.SplitN(v, ",", 2)
			keys := strings.TrimPrefix(kv[0], "keys=")
			d.Dbs = append(d.Dbs, model.DbInfo{Name: k, Note: v, Tables: keys})
		}
	}

	// 复制/集群成员
	if m["cluster_enabled"] == "1" {
		if nodes, err := rdb.ClusterNodes(ctx).Result(); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(nodes), "\n") {
				fs := strings.Fields(strings.TrimSpace(line))
				if len(fs) < 3 {
					continue
				}
				addr, flags := fs[1], fs[2]
				role := "节点"
				status := "运行中"
				switch {
				case strings.Contains(flags, "master"):
					role = "主节点"
				case strings.Contains(flags, "slave") || strings.Contains(flags, "replica"):
					role = "从节点"
				case strings.Contains(flags, "fail"):
					status = "FAIL"
				}
				d.Services = append(d.Services, model.SvcInfo{Name: addr, Role: role, Status: status, Host: addr, Detail: flags})
			}
		}
	} else {
		for i := 0; i < 10; i++ {
			v, ok := m[fmt.Sprintf("slave%d", i)]
			if !ok {
				break
			}
			fields := map[string]string{}
			for _, kv := range strings.Split(v, ",") {
				if p := strings.SplitN(kv, "=", 2); len(p) == 2 {
					fields[p[0]] = p[1]
				}
			}
			st := fields["state"]
			d.Services = append(d.Services, model.SvcInfo{
				Name:   fields["ip"] + ":" + fields["port"],
				Role:   "从库",
				Status: st,
				Host:   fields["ip"],
				Detail: "offset=" + fields["offset"],
			})
		}
	}
	return d, nil
}

func (redisProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	ctx, cancel := DBCtx(c)
	defer cancel()
	rdb := redisProvider{}.client(c)
	defer rdb.Close()

	info, err := rdb.Info(ctx, "server", "keyspace", "replication", "cluster").Result()
	if err != nil {
		return nil, err
	}
	m := parseInfo(info)

	root := newTNode("root", c.Name, "Redis @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: m["redis_version"]},
		model.KV{Key: "模式", Value: m["redis_mode"]},
	)

	// 持久化分支
	branch := addNode(root, newTNode("persist", "持久化", "", "na"))
	rdbStatus := m["rdb_last_bgsave_status"]
	addNode(branch, newTNode("rdb", "RDB 快照", "最近保存: "+orDefault(rdbStatus, "-"), statusOf(rdbStatus, "ok")))
	addNode(branch, newTNode("aof", "AOF 追加日志", orDefault(map[bool]string{true: "已开启", false: "未开启"}[m["aof_enabled"] == "1"], "-"),
		statusOf(m["aof_enabled"], "na")))

	// 集群/复制分支
	switch {
	case m["cluster_enabled"] == "1":
		br := addNode(root, newTNode("cluster", "Cluster 集群", "", "ok"))
		if nodes, err := rdb.ClusterNodes(ctx).Result(); err == nil {
			for i, line := range strings.Split(strings.TrimSpace(nodes), "\n") {
				fs := strings.Fields(strings.TrimSpace(line))
				if len(fs) < 3 {
					continue
				}
				st := "ok"
				if strings.Contains(fs[2], "fail") {
					st = "err"
				}
				addNode(br, newTNode(fmt.Sprintf("n-%d", i), fs[1], fs[2], st))
			}
		}
	case m["connected_slaves"] != "" && m["connected_slaves"] != "0":
		br := addNode(root, newTNode("repl", "主从复制", m["connected_slaves"]+" 个从库", "ok"))
		for i := 0; i < 10; i++ {
			v, ok := m[fmt.Sprintf("slave%d", i)]
			if !ok {
				break
			}
			fields := map[string]string{}
			for _, kv := range strings.Split(v, ",") {
				if p := strings.SplitN(kv, "=", 2); len(p) == 2 {
					fields[p[0]] = p[1]
				}
			}
			st := "ok"
			if fields["state"] != "online" {
				st = "warn"
			}
			addNode(br, newTNode(fmt.Sprintf("s-%d", i), fields["ip"]+":"+fields["port"], fields["state"], st))
		}
	}

	// 键空间分支
	br := addNode(root, newTNode("dbs", "键空间", "", "na"))
	n := 0
	for k, v := range m {
		if strings.HasPrefix(k, "db") && strings.Contains(v, "keys=") {
			addNode(br, newTNode("db-"+k, k, v, "na"))
			n++
		}
	}
	if n == 0 {
		addNode(br, newTNode("db-empty", "（无键）", "", "na"))
	}
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}

func statusOf(v, normal string) string {
	if v == "" {
		return "na"
	}
	if strings.EqualFold(v, normal) || strings.EqualFold(v, "1") || strings.EqualFold(v, "ok") {
		return "ok"
	}
	return "warn"
}
