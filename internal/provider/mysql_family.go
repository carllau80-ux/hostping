package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"dbping/internal/model"
)

// mysqlFamily MySQL 协议兼容族：
// MySQL / MariaDB / TiDB / OceanBase(MySQL) / Doris / StarRocks / GBase 8a / PolarDB / TDSQL / GoldenDB。
type mysqlFamily struct{}

func (mysqlFamily) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := MySQLDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "mysql", dsn) }
}

// Ping 三级探活。
func (f mysqlFamily) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1")
}

// Overview 实例概览。
func (f mysqlFamily) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// 版本
	o.Version = QueryString(db, "SELECT VERSION()", "-")
	comment := QueryString(db, "SELECT @@version_comment", "")
	o.VersionFull = strings.TrimSpace(o.Version + " " + comment)
	if c.Type == "tidb" {
		if tv := QueryString(db, "SELECT tidb_version()", ""); tv != "-" {
			o.VersionFull = tv
			o.Version = firstLine(tv)
		}
	}
	if c.Type == "oceanbase" || c.Type == "goldendb" {
		if ob := QueryString(db, "SELECT ob_version()", ""); ob != "-" && ob != "" {
			o.VersionFull = "OceanBase " + ob
			o.Version = ob
		}
	}

	Try(o, "主机名", func() error {
		o.Hostname = QueryString(db, "SELECT @@hostname", "")
		return nil
	})
	Try(o, "服务器OS", func() error {
		if kvs, ok := QueryKV(db, "SELECT @@version_compile_os, @@version_compile_machine"); ok && len(kvs) >= 2 {
			o.OS = strings.TrimSpace(kvs[0].Value + " " + kvs[1].Value)
		}
		return nil
	})
	Try(o, "运行时长", func() error {
		up := QueryString(db, "SHOW GLOBAL STATUS LIKE 'Uptime'", "")
		if sec, ok := statusValue(up); ok {
			o.Uptime = HumanDuration(sec)
		}
		return nil
	})
	Try(o, "字符集", func() error {
		o.Charset = QueryString(db, "SELECT @@character_set_server", "-")
		o.Timezone = QueryString(db, "SELECT @@system_time_zone", "-")
		return nil
	})
	Try(o, "连接数", func() error {
		o.ConnCur = QueryString(db, "SHOW GLOBAL STATUS LIKE 'Threads_connected'", "0")
		if v, ok := statusValue(o.ConnCur); ok {
			o.ConnCur = fmt.Sprintf("%d", v)
		}
		o.ConnMax = QueryString(db, "SELECT @@max_connections", "-")
		return nil
	})
	Try(o, "归档/Binlog", func() error {
		lb := QueryString(db, "SELECT @@log_bin", "0")
		if lb == "1" {
			o.ArchiveMode = "Binlog 已开启（binlog）"
			// binlog 文件落盘目录（归档路径）
			if base := QueryString(db, "SELECT @@log_bin_basename", ""); base != "" && base != "-" {
				if i := strings.LastIndex(base, "/"); i > 0 {
					o.ArchiveDest = base[:i+1]
				} else if i := strings.LastIndex(base, `\`); i > 0 {
					o.ArchiveDest = base[:i+1]
				} else {
					o.ArchiveDest = base
				}
			}
		} else {
			o.ArchiveMode = "Binlog 未开启"
		}
		return nil
	})
	Try(o, "只读", func() error {
		ro := QueryString(db, "SELECT @@read_only", "0")
		if ro == "1" {
			o.ReadOnly = "是"
		} else {
			o.ReadOnly = "否"
		}
		return nil
	})
	Try(o, "容量", func() error {
		where := "table_schema NOT IN ('mysql','information_schema','performance_schema','sys','oceanbase')"
		if c.Database != "" && c.Type == "mysql" {
			where = "table_schema = '" + sqlEscape(c.Database) + "'"
		}
		sz := QueryString(db, "SELECT SUM(data_length+index_length) FROM information_schema.TABLES WHERE "+where, "0")
		o.DBSize = HumanBytes(parseI64(sz))
		tc := QueryString(db, "SELECT COUNT(*) FROM information_schema.TABLES WHERE "+where, "0")
		o.TableCount = tc
		return nil
	})
	Try(o, "存储引擎", func() error {
		if eng := QueryString(db, "SELECT @@default_storage_engine", ""); eng != "" && eng != "-" {
			o.Extra = append(o.Extra, model.KV{Key: "默认存储引擎", Value: eng})
		}
		return nil
	})

	f.roleAndCluster(db, c, o)
	return o, nil
}

// roleAndCluster 主从/集群角色判定。
func (f mysqlFamily) roleAndCluster(db *sql.DB, c *model.Conn, o *model.Overview) {
	switch c.Type {
	case "tidb":
		o.Cluster = "分布式（TiDB/PD/TiKV）"
		o.Role = "分布式集群"
	case "oceanbase", "polardb", "tdsql", "goldendb", "doris", "starrocks":
		o.Cluster = "分布式集群"
		o.Role = "集群节点"
	default:
		// 主从检测：优先 SHOW REPLICA STATUS（8.0.22+），回落 SHOW SLAVE STATUS
		isSlave := false
		for _, q := range []string{"SHOW REPLICA STATUS", "SHOW SLAVE STATUS"} {
			if cols, rows, err := QueryTable(db, q); err == nil && len(rows) > 0 {
				ioRunning := pickRow(cols, rows[0], "Slave_IO_Running", "Replica_IO_Running")
				sqlRunning := pickRow(cols, rows[0], "Slave_SQL_Running", "Replica_SQL_Running")
				masterHost := pickRow(cols, rows[0], "Master_Host", "Source_Host")
				if strings.EqualFold(ioRunning, "Yes") && strings.EqualFold(sqlRunning, "Yes") {
					o.Role = "从库（复制中，来源 " + masterHost + "）"
					o.Cluster = "主从复制"
					isSlave = true
				} else {
					o.Role = "从库（复制异常）"
					o.Cluster = "主从复制"
					isSlave = true
				}
				break
			}
		}
		if !isSlave {
			if n := QueryString(db, "SHOW GLOBAL STATUS LIKE 'Rpl_semi_sync_master_clients'", "0"); parseI64(n) > 0 {
				o.Cluster = "主从复制（半同步 " + n + " 从）"
				o.Role = "主库"
			}
		}
		if o.Role == "" {
			o.Role = "单机"
		}
		if o.Cluster == "" {
			o.Cluster = "单机"
		}
	}
}

// Detail 库/表/表空间/服务明细。
func (f mysqlFamily) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 库列表 + 各库大小/表数
	if _, rows, err := QueryTable(db, `SELECT table_schema, COUNT(*), SUM(data_length+index_length)
		FROM information_schema.TABLES GROUP BY table_schema ORDER BY 3 DESC`); err == nil {
		for _, r := range rows {
			d.Dbs = append(d.Dbs, model.DbInfo{
				Name:   r[0],
				Tables: fmt.Sprintf("%d", parseI64(r[1])),
				Size:   HumanBytes(parseI64(r[2])),
				Note:   systemDBNote(r[0]),
			})
		}
	} else {
		d.Overview = &model.Overview{Warnings: []string{"库列表采集失败: " + shortErr(err)}}
	}

	// 当前库 Top 表
	if c.Database != "" {
		q := fmt.Sprintf(`SELECT table_name, IFNULL(table_rows,0),
			IFNULL(data_length+index_length,0), IFNULL(table_comment,'')
			FROM information_schema.TABLES WHERE table_schema='%s'
			ORDER BY (data_length+index_length) DESC LIMIT 20`, sqlEscape(c.Database))
		if _, rows, err := QueryTable(db, q); err == nil {
			for _, r := range rows {
				d.Tables = append(d.Tables, model.TableInfo{
					Name: r[0], Rows: fmt.Sprintf("%d", parseI64(r[1])),
					Size: HumanBytes(parseI64(r[2])), Comment: r[3],
				})
			}
		}
	}

	// 集群服务/节点
	d.Services = f.services(db, c)
	return d, nil
}

// services 集群组件/复制成员采集。
func (f mysqlFamily) services(db *sql.DB, c *model.Conn) []model.SvcInfo {
	var out []model.SvcInfo
	add := func(name, role, status, host, detail string) {
		out = append(out, model.SvcInfo{Name: name, Role: role, Status: status, Host: host, Detail: detail})
	}
	switch c.Type {
	case "tidb":
		if cols, rows, err := QueryTable(db, `SELECT TYPE, INSTANCE, VERSION, START_TIME FROM information_schema.CLUSTER_INFO`); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				add(m["INSTANCE"], m["TYPE"], "运行中", hostOf(m["INSTANCE"]), m["VERSION"]+" 启动:"+m["START_TIME"])
			}
		}
		if cols, rows, err := QueryTable(db, `SELECT STORE_ID, ADDRESS, STATE, CAPACITY, AVAILABLE FROM information_schema.TIKV_STORE_STATUS`); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				add(m["ADDRESS"], "TiKV Store#"+m["STORE_ID"], m["STATE"], m["ADDRESS"], "容量 "+m["CAPACITY"]+" 可用 "+m["AVAILABLE"])
			}
		}
	case "oceanbase":
		if cols, rows, err := QueryTable(db, `SELECT SVR_IP, SVR_PORT, ZONE, STATUS, WITH_ROOTSERVER, BUILD_VERSION FROM oceanbase.DBA_OB_SERVERS`); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				role := "OBServer"
				if m["WITH_ROOTSERVER"] == "YES" || m["WITH_ROOTSERVER"] == "1" {
					role += "（RootServer）"
				}
				add(m["SVR_IP"]+":"+m["SVR_PORT"], role, m["STATUS"], m["SVR_IP"], "Zone "+m["ZONE"]+" "+m["BUILD_VERSION"])
			}
		}
		if cols, rows, err := QueryTable(db, `SELECT TENANT_NAME, TENANT_TYPE, STATUS FROM oceanbase.DBA_OB_TENANTS`); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				add(m["TENANT_NAME"], "租户（"+m["TENANT_TYPE"]+"）", m["STATUS"], "", "")
			}
		}
	case "doris", "starrocks":
		if cols, rows, err := QueryTable(db, "SHOW FRONTENDS"); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				host := orDefault(m["Host"], m["IP"])
				status := "正常"
				if !strings.EqualFold(orDefault(m["Alive"], "true"), "true") && m["Alive"] != "1" {
					status = "异常"
				}
				add(host, "FE（"+orDefault(m["Role"], "-")+"）", status, host, "启动:"+orDefault(m["LastStartTime"], "-"))
			}
		}
		if cols, rows, err := QueryTable(db, "SHOW BACKENDS"); err == nil {
			for _, r := range rows {
				m := mapCols(cols, r)
				host := orDefault(m["Host"], m["IP"])
				status := "正常"
				if !strings.EqualFold(orDefault(m["Alive"], "true"), "true") && m["Alive"] != "1" {
					status = "异常"
				}
				add(host, "BE", status, host, "表数:"+orDefault(m["TabletNum"], "-")+" 容量:"+orDefault(m["TotalCapacity"], "-"))
			}
		}
	default:
		// MySQL/MariaDB：主从复制成员（8.0.22+ 用 REPLICA，旧版/兼容系用 SLAVE）
		for _, q := range []string{"SHOW REPLICA STATUS", "SHOW SLAVE STATUS"} {
			if cols, rows, err := QueryTable(db, q); err == nil && len(rows) > 0 {
				m := mapCols(cols, rows[0])
				master := orDefault(m["master_host"], m["source_host"], "-")
				mport := orDefault(m["master_port"], m["source_port"], "")
				peer := master
				if mport != "" {
					peer = master + ":" + mport
				}
				muser := orDefault(m["master_user"], m["source_user"], "")
				if muser != "" {
					peer += "（用户 " + muser + "）"
				}
				behind := orDefault(m["seconds_behind_source"], m["seconds_behind_master"], "")
				add(peer, "复制源（Master）", "复制中", master,
					"IO:"+orDefault(m["slave_io_running"], m["replica_io_running"], "-")+
						" SQL:"+orDefault(m["slave_sql_running"], m["replica_sql_running"], "-")+
						orDefault(map[bool]string{true: " · 延迟 " + behind + "s", false: ""}[behind != "" && behind != "NULL"], ""),
					)
				break
			}
		}
		// 半同步从库数量
		if n := QueryString(db, "SHOW GLOBAL STATUS LIKE 'Rpl_semi_sync_master_clients'", "0"); parseI64(n) > 0 {
			add("-", "半同步从库", "运行中", "", "数量: "+n)
		}
	}
	return out
}

// Topology 拓扑树：实例 → 复制/集群 + 数据库（→表）。
func (f mysqlFamily) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ver := QueryString(db, "SELECT VERSION()", "")
	root := newTNode("root", c.Name, c.Type+" @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: ver},
		model.KV{Key: "地址", Value: c.Host + ":" + itoa(c.Port)},
	)

	// 集群/复制分支
	svcs := f.services(db, c)
	if len(svcs) > 0 {
		branch := addNode(root, newTNode("cluster", "集群/复制拓扑", fmt.Sprintf("%d 个组件", len(svcs)), "na"))
		for i, s := range svcs {
			st := "ok"
			if strings.Contains(s.Status, "异常") || strings.Contains(s.Status, "异常") || s.Status == "STOPPED" {
				st = "err"
			}
			addNode(branch, newTNode(fmt.Sprintf("svc-%d", i), s.Name, s.Role+" · "+s.Status, st,
				model.KV{Key: "详情", Value: s.Detail}))
		}
	}

	// 数据库分支
	branch := addNode(root, newTNode("dbs", "数据库", "", "na"))
	if _, rows, err := QueryTable(db, `SELECT table_schema, COUNT(*), SUM(data_length+index_length)
		FROM information_schema.TABLES GROUP BY table_schema ORDER BY 3 DESC`); err == nil {
		for i, r := range rows {
			name, cnt, size := r[0], r[1], HumanBytes(parseI64(r[2]))
			status := "ok"
			note := systemDBNote(name)
			if note != "" {
				status = "na"
			}
			addNode(branch, newTNode(fmt.Sprintf("db-%d", i), name,
				fmt.Sprintf("%s 表 · %s %s", cnt, size, note), status))
		}
	}

	// 当前库表空间维度（InnoDB 不适用，OceanBase 租户/表组略）——保持树精简
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}

// ---------- 小工具 ----------

func sqlEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(s)
}

// parseI64 解析数字字符串（容忍小数/空白/单位后缀）。
func parseI64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	end := 0
	seenDot := false
	for i, ch := range s {
		if ch >= '0' && ch <= '9' {
			end = i + 1
			continue
		}
		if ch == '.' && !seenDot {
			seenDot = true
			continue
		}
		break
	}
	if end == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(s[:end], 64)
	return int64(f)
}

// statusValue 从 "Uptime  12345" 样式取数值。
func statusValue(s string) (int64, bool) {
	fs := strings.Fields(s)
	if len(fs) < 2 {
		return 0, false
	}
	return parseI64(fs[len(fs)-1]), true
}

// mapCols 行转 map（列名 -> 值，列名统一小写）。
func mapCols(cols []string, row []string) map[string]string {
	m := make(map[string]string, len(cols))
	for i, c := range cols {
		if i < len(row) {
			m[strings.ToLower(c)] = row[i]
		}
	}
	return m
}

func pickRow(cols []string, row []string, names ...string) string {
	m := mapCols(cols, row)
	for _, n := range names {
		if v, ok := m[strings.ToLower(n)]; ok && v != "" {
			return v
		}
	}
	return ""
}

func orDefault(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func hostOf(instance string) string {
	if i := strings.LastIndex(instance, ":"); i > 0 {
		return instance[:i]
	}
	return instance
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func itoa(p int) string { return fmt.Sprintf("%d", p) }

func systemDBNote(name string) string {
	switch strings.ToLower(name) {
	case "mysql", "sys", "performance_schema", "information_schema", "oceanbase":
		return "（系统库）"
	default:
		return ""
	}
}
