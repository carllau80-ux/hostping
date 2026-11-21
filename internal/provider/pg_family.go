package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"dbping/internal/model"
)

// pgFamily PostgreSQL 协议兼容族：
// PostgreSQL / KingbaseES / openGauss / MogDB / Greenplum / CockroachDB / YugabyteDB / GBase 8c / Vastbase。
type pgFamily struct{}

// pgDriver 返回类型对应的 database/sql 驱动名。
func pgDriver(typ string) string {
	switch typ {
	case "kingbase":
		return "postgres" // lib/pq，金仓官方推荐路线
	case "opengauss":
		return "opengauss" // openGauss 官方连接器（支持 sha256 认证）
	default:
		return "pgx"
	}
}

func (pgFamily) open(c *model.Conn) func() (*sql.DB, error) {
	drv := pgDriver(c.Type)
	dsn := PGDSN(c, drv)
	return func() (*sql.DB, error) { return OpenSQL(c, drv, dsn) }
}

// Ping 三级探活。
func (f pgFamily) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1")
}

// Overview 实例概览。
func (f pgFamily) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// 版本横幅: "PostgreSQL 15.4 ..." / "openGauss 5.0.0 ..." / "CockroachDB CCL v23.1 ..."
	banner := QueryString(db, "SELECT version()", "-")
	o.VersionFull = banner
	if fs := strings.Fields(banner); len(fs) >= 2 {
		o.Version = fs[1]
	}
	// 服务器 OS：从横幅解析 "on x86_64-pc-linux-gnu"
	o.OS = osFromBanner(banner)

	Try(o, "实例", func() error {
		o.Instance = QueryString(db, "SELECT current_setting('server_version')", "-")
		o.Hostname = QueryString(db, "SELECT inet_server_addr()", "")
		return nil
	})
	Try(o, "启动时间", func() error {
		started := QueryString(db, "SELECT pg_postmaster_start_time()", "")
		if s := strings.TrimSpace(started); s != "" {
			var t time.Time
			var err error
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.99999-07", "2006-01-02 15:04:05"} {
				if t, err = time.Parse(layout, s); err == nil {
					break
				}
			}
			if err == nil {
				o.StartupTime = t.Format("2006-01-02 15:04:05")
				o.Uptime = HumanDuration(int64(time.Since(t).Seconds()))
			} else {
				o.StartupTime = s
			}
		}
		return nil
	})
	Try(o, "角色", func() error {
		inRec := QueryString(db, "SELECT pg_is_in_recovery()", "f")
		if inRec == "t" || inRec == "true" {
			o.Role = "备库（恢复中/只读）"
		} else {
			o.Role = "主库（可写）"
		}
		// 有复制从属时标注
		n := QueryString(db, "SELECT count(*) FROM pg_stat_replication", "0")
		if parseI64(n) > 0 && o.Role == "主库（可写）" {
			o.Role = fmt.Sprintf("主库（%d 个复制从属）", parseI64(n))
			o.Cluster = "主从复制"
		}
		return nil
	})
	Try(o, "只读", func() error {
		ro := QueryString(db, "SHOW transaction_read_only", "off")
		if strings.EqualFold(ro, "on") {
			o.ReadOnly = "是"
		} else {
			o.ReadOnly = "否"
		}
		return nil
	})
	Try(o, "归档模式", func() error {
		am := QueryString(db, "SHOW archive_mode", "off")
		wl := QueryString(db, "SHOW wal_level", "-")
		switch {
		case strings.EqualFold(am, "on"):
			o.ArchiveMode = "归档已开启（archive_mode=on）"
		case strings.EqualFold(wl, "replica") || strings.EqualFold(wl, "logical"):
			o.ArchiveMode = "归档未开启（仅 WAL: " + wl + "）"
		default:
			o.ArchiveMode = "未开启"
		}
		// 归档命令/目的地（开启归档后展示）
		if cmd := QueryString(db, "SHOW archive_command", ""); cmd != "" && cmd != "(none)" {
			o.ArchiveDest = cmd
		}
		// 备库：显示主库连接配置（流复制对端）
		if inRec := QueryString(db, "SELECT pg_is_in_recovery()", "f"); inRec == "t" || inRec == "true" {
			if ci := QueryString(db, "SHOW primary_conninfo", ""); ci != "" && ci != "(none)" {
				o.ArchiveDest = strings.TrimSpace(o.ArchiveDest + " ｜ 对端: " + ci)
			}
		}
		return nil
	})
	Try(o, "字符集", func() error {
		o.Charset = QueryString(db, "SHOW server_encoding", "-")
		o.Timezone = QueryString(db, "SHOW TimeZone", "-")
		return nil
	})
	Try(o, "连接数", func() error {
		o.ConnCur = QueryString(db, "SELECT count(*) FROM pg_stat_activity", "0")
		o.ConnMax = QueryString(db, "SHOW max_connections", "-")
		return nil
	})
	Try(o, "容量", func() error {
		if c.Database != "" {
			o.DBSize = QueryString(db, "SELECT pg_size_pretty(pg_database_size(current_database()))", "-")
			o.TableCount = QueryString(db,
				"SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema','pg_toast','sys','sysaudit','sys_catalog')", "0")
		} else {
			o.DBSize = "-"
			o.TableCount = "-"
		}
		return nil
	})

	// 类型专有
	switch c.Type {
	case "opengauss", "mogdb":
		Try(o, "集群节点", func() error {
			_, rows, err := QueryTable(db, "SELECT node_name, node_type, node_status FROM pgxc_node")
			if err != nil {
				return err
			}
			if len(rows) > 1 {
				o.Cluster = "分布式（" + fmt.Sprintf("%d 节点", len(rows)) + "）"
			}
			return nil
		})
	case "greenplum":
		o.Cluster = "MPP 集群"
		o.Role = "Master/Segment 架构"
	case "cockroach", "yugabyte", "tidb":
		o.Cluster = "分布式"
	case "kingbase":
		o.Extra = append(o.Extra, model.KV{Key: "兼容模式", Value: "PG 协议（lib/pq 驱动）"})
	}
	return o, nil
}

// osFromBanner 从 PG 系版本横幅解析服务器 OS（"on x86_64-pc-linux-gnu"）。
func osFromBanner(banner string) string {
	i := strings.Index(banner, " on ")
	if i < 0 {
		return ""
	}
	rest := banner[i+4:]
	end := strings.IndexAny(rest, ",;")
	if end >= 0 {
		rest = rest[:end]
	}
	rest = strings.TrimSpace(rest)
	// 形如 "x86_64-pc-linux-gnu, 64-bit" 或 "x86_64-pc-linux-gnu"
	parts := strings.Split(rest, "-")
	if len(parts) < 3 {
		return ""
	}
	arch, osName := parts[0], parts[2]
	osName = strings.TrimSuffix(osName, "gnu")
	switch strings.ToLower(osName) {
	case "linux":
		osName = "Linux"
	case "win", "mingw", "windows":
		osName = "Windows"
	case "darwin":
		osName = "macOS"
	case "freebsd":
		osName = "FreeBSD"
	default:
		osName = strings.Title(osName)
	}
	return strings.TrimSpace(osName + " " + arch)
}

// Detail 库/表/表空间/服务明细。
func (f pgFamily) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 库列表
	if _, rows, err := QueryTable(db, `SELECT datname,
		CASE WHEN pg_database_size(datname) > 0 THEN pg_size_pretty(pg_database_size(datname)) ELSE '-' END,
		pg_encoding_to_char(encoding)
		FROM pg_database WHERE datallowconn ORDER BY datname`); err == nil {
		for _, r := range rows {
			note := ""
			if strings.HasPrefix(r[0], "template") || r[0] == "postgres" {
				note = "（系统库）"
			}
			d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Size: r[1], Charset: r[2], Note: note})
		}
		// 当前库补充表数（PG 无法跨库统计，仅当前连接库可得）
		if c.Database != "" {
			tc := QueryString(db, `SELECT count(*) FROM pg_tables WHERE schemaname NOT IN
				('pg_catalog','information_schema','pg_toast','sys','sysaudit','sys_catalog')`, "")
			if tc != "" {
				for i := range d.Dbs {
					if d.Dbs[i].Name == c.Database {
						d.Dbs[i].Tables = tc
					}
				}
			}
		}
	}

	// 当前库 Top 表
	if c.Database != "" {
		if _, rows, err := QueryTable(db, `SELECT n.nspname, c.relname,
			CASE WHEN c.reltuples >= 0 THEN round(c.reltuples)::bigint ELSE 0 END,
			pg_size_pretty(pg_total_relation_size(c.oid))
			FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relkind = 'r' AND n.nspname NOT IN
			('pg_catalog','information_schema','pg_toast','sys','sysaudit','sys_catalog','crdb_internal','pg_extension')
			ORDER BY pg_total_relation_size(c.oid) DESC LIMIT 20`); err == nil {
			for _, r := range rows {
				d.Tables = append(d.Tables, model.TableInfo{
					Schema: r[0], Name: r[1], Rows: r[2], Size: r[3],
				})
			}
		}
	}

	// 表空间（大小 + 存放路径 + 状态）
	if _, rows, err := QueryTable(db, `SELECT spcname, pg_size_pretty(pg_tablespace_size(oid)),
		COALESCE(pg_tablespace_location(oid), '')
		FROM pg_tablespace ORDER BY spcname`); err == nil {
		for _, r := range rows {
			d.TBS = append(d.TBS, model.TBSInfo{
				Name: r[0], Size: r[1], Files: orDefault(r[2], "-"),
				Status: "normal", Free: "-（由文件系统决定）",
			})
		}
	}

	// 集群服务
	d.Services = f.services(db, c)
	return d, nil
}

// services 复制/集群成员。
func (f pgFamily) services(db *sql.DB, c *model.Conn) []model.SvcInfo {
	var out []model.SvcInfo
	add := func(name, role, status, host, detail string) {
		out = append(out, model.SvcInfo{Name: name, Role: role, Status: status, Host: host, Detail: detail})
	}
	switch c.Type {
	case "greenplum":
		if cols, rows, err := QueryTable(db, `SELECT hostname, address, port, role, status, content
			FROM gp_segment_configuration ORDER BY content, role`); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				role := "Segment"
				if m["content"] == "-1" {
					role = "Master"
				}
				if m["role"] == "m" {
					role += "（Primary）"
				} else {
					role += "（Mirror）"
				}
				st := "正常"
				if !strings.EqualFold(m["status"], "u") {
					st = "异常"
				}
				add(m["address"]+":"+m["port"], role, st, m["hostname"], "content="+m["content"])
			}
		}
	case "opengauss", "mogdb":
		if cols, rows, err := QueryTable(db, `SELECT node_name, node_type, node_status, node_host, node_port
			FROM pgxc_node ORDER BY node_name`); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				role := map[string]string{"C": "CN 协调节点", "D": "DN 数据节点", "G": "GTM", "S": "GTM 备机"}[m["node_type"]]
				if role == "" {
					role = "节点"
				}
				add(m["node_name"], role, m["node_status"], m["node_host"]+":"+m["node_port"], "")
			}
		}
	default:
		// 流复制从属（含端口，即"对端连接配置"）
		if cols, rows, err := QueryTable(db, `SELECT application_name, client_addr, client_port, state, sync_state
			FROM pg_stat_replication`); err == nil {
			for i, r := range rows {
				m := mapCols(cols, r)
				peer := orDefault(m["client_addr"], "-")
				if m["client_port"] != "" && m["client_port"] != "-1" {
					peer += ":" + m["client_port"]
				}
				st := "复制中"
				if m["state"] != "streaming" {
					st = m["state"]
				}
				add(orDefault(m["application_name"], peer, fmt.Sprintf("从属%d", i+1)),
					"复制从属（"+m["sync_state"]+"）", st, peer, "对端: "+peer)
			}
		}
	}
	return out
}

// Topology 拓扑树。
func (f pgFamily) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	banner := QueryString(db, "SELECT version()", "")
	root := newTNode("root", c.Name, c.Type+" @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: firstLine(banner)},
		model.KV{Key: "地址", Value: c.Host + ":" + itoa(c.Port)},
	)

	// 集群/复制分支
	svcs := f.services(db, c)
	if len(svcs) > 0 {
		branch := addNode(root, newTNode("cluster", "集群/复制拓扑", fmt.Sprintf("%d 个成员", len(svcs)), "na"))
		for i, s := range svcs {
			st := "ok"
			if strings.Contains(s.Status, "异常") {
				st = "err"
			}
			addNode(branch, newTNode(fmt.Sprintf("svc-%d", i), s.Name, s.Role+" · "+s.Status, st,
				model.KV{Key: "详情", Value: s.Detail}))
		}
	}

	// 表空间分支（含数据文件目录）
	if _, rows, err := QueryTable(db, `SELECT spcname, pg_size_pretty(pg_tablespace_size(oid)),
		COALESCE(pg_tablespace_location(oid), '')
		FROM pg_tablespace ORDER BY spcname`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("tbs", "表空间", fmt.Sprintf("%d 个", len(rows)), "na"))
		if dd := QueryString(db, "SHOW data_directory", ""); dd != "" {
			addNode(branch, newTNode("tbs-dd", "数据目录", dd, "na"))
		}
		for i, r := range rows {
			n := addNode(branch, newTNode(fmt.Sprintf("tbs-%d", i), r[0], r[1], "na"))
			if r[2] != "" {
				addNode(n, newTNode(fmt.Sprintf("tbs-%d-f", i), r[2], "数据文件目录", "na"))
			}
		}
	}

	// 数据库分支
	if _, rows, err := QueryTable(db, `SELECT datname,
		CASE WHEN pg_database_size(datname) > 0 THEN pg_size_pretty(pg_database_size(datname)) ELSE '-' END
		FROM pg_database WHERE datallowconn ORDER BY datname`); err == nil {
		branch := addNode(root, newTNode("dbs", "数据库", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			note := ""
			status := "ok"
			if strings.HasPrefix(r[0], "template") || r[0] == "postgres" {
				note = "（系统库）"
				status = "na"
			}
			addNode(branch, newTNode(fmt.Sprintf("db-%d", i), r[0], r[1]+" "+note, status))
		}
	}

	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
