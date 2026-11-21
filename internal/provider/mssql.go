package provider

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dbping/internal/model"
)

// mssqlProvider Microsoft SQL Server。
type mssqlProvider struct{}

func (mssqlProvider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := MSSQLDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "sqlserver", dsn) }
}

func (f mssqlProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1")
}

func (f mssqlProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// @@VERSION 第一行: "Microsoft SQL Server 2019 (RTM-CU25) 15.0.4335.1 ..."
	banner := QueryString(db, "SELECT @@VERSION", "-")
	o.VersionFull = firstLine(banner)
	if fs := strings.Fields(firstLine(banner)); len(fs) >= 5 {
		o.Version = fs[4]
	}
	// 服务器 OS：横幅中 " on Windows Server 2019 Datacenter 10.0 ..."
	o.OS = osFromMssqlBanner(firstLine(banner))

	Try(o, "实例属性", func() error {
		o.Instance = QueryString(db, "SELECT CAST(SERVERPROPERTY('ServerName') AS varchar(128))", "-")
		o.Edition = QueryString(db, "SELECT CAST(SERVERPROPERTY('Edition') AS varchar(128))", "-")
		o.Hostname = QueryString(db, "SELECT CAST(SERVERPROPERTY('MachineName') AS varchar(128))", "-")
		lvl := QueryString(db, "SELECT CAST(SERVERPROPERTY('ProductLevel') AS varchar(64))", "")
		if lvl != "" {
			o.Extra = append(o.Extra, model.KV{Key: "补丁级别", Value: lvl})
		}
		hadr := QueryString(db, "SELECT CAST(SERVERPROPERTY('IsHadrEnabled') AS varchar(8))", "0")
		if hadr == "1" {
			o.Cluster = "AlwaysOn 可用组已启用"
			o.Role = "AG 成员"
		}
		return nil
	})
	Try(o, "启动时间", func() error {
		started := QueryString(db, "SELECT CONVERT(varchar(19), sqlserver_start_time, 120) FROM sys.dm_os_sys_info", "")
		o.StartupTime = started
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(started), time.Local); err == nil {
			o.Uptime = HumanDuration(int64(time.Since(t).Seconds()))
		}
		return nil
	})
	Try(o, "恢复模式", func() error {
		rm := ""
		if c.Database != "" {
			rm = QueryString(db, "SELECT recovery_model_desc FROM sys.databases WHERE name='"+sqlEscape(c.Database)+"'", "")
		}
		if rm == "" {
			rm = QueryString(db, "SELECT recovery_model_desc FROM sys.databases WHERE database_id=DB_ID()", "-")
		}
		switch strings.ToLower(rm) {
		case "full":
			o.ArchiveMode = "FULL 完整恢复（支持日志备份/时点恢复）"
		case "bulk_logged":
			o.ArchiveMode = "BULK_LOGGED 大容量日志恢复"
		case "simple":
			o.ArchiveMode = "SIMPLE 简单恢复（无日志归档）"
		default:
			o.ArchiveMode = rm
		}
		return nil
	})
	Try(o, "字符集/排序规则", func() error {
		o.Charset = QueryString(db, "SELECT CAST(SERVERPROPERTY('Collation') AS varchar(64))", "-")
		o.Timezone = "-"
		return nil
	})
	Try(o, "连接数", func() error {
		o.ConnCur = QueryString(db, "SELECT count(*) FROM sys.dm_exec_sessions WHERE is_user_process = 1", "0")
		o.ConnMax = QueryString(db, "SELECT CAST(@@MAX_CONNECTIONS AS varchar(16))", "-")
		return nil
	})
	Try(o, "容量", func() error {
		sz := QueryString(db, "SELECT CAST(SUM(CAST(size AS bigint))*8/1024.0/1024 AS varchar(32))+' GB' FROM sys.master_files WHERE database_id = DB_ID()", "-")
		o.DBSize = sz
		return nil
	})
	Try(o, "数据库数量", func() error {
		o.TableCount = QueryString(db, "SELECT count(*) FROM sys.databases", "0") + " 个库"
		return nil
	})
	if o.Role == "" {
		o.Role = "单机"
	}
	if o.Cluster == "" {
		o.Cluster = "单机"
	}
	return o, nil
}

func (f mssqlProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 库列表（大小来自 master_files，可能因权限失败）
	if _, rows, err := QueryTable(db, `SELECT d.name, d.state_desc, d.recovery_model_desc,
		CAST(SUM(CAST(mf.size AS bigint))*8/1024.0/1024 AS varchar(32)) + ' GB' AS dbsize
		FROM sys.databases d LEFT JOIN sys.master_files mf ON mf.database_id = d.database_id
		GROUP BY d.name, d.state_desc, d.recovery_model_desc ORDER BY d.name`); err == nil {
		for _, r := range rows {
			note := ""
			if r[1] != "ONLINE" {
				note = "（" + r[1] + "）"
			}
			d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Size: r[3], Note: note + " 恢复模式:" + r[2]})
		}
	}

	// 当前库补充表数（SQL Server 无法跨库统计）
	dn := QueryString(db, "SELECT DB_NAME()", "")
	tc := QueryString(db, "SELECT count(*) FROM sys.tables", "")
	if dn != "" && tc != "" {
		for i := range d.Dbs {
			if d.Dbs[i].Name == dn {
				d.Dbs[i].Tables = tc
			}
		}
	}

	// 当前库 Top 表
	if _, rows, err := QueryTable(db, `SELECT TOP 20 s.name, t.name,
		SUM(CASE WHEN p.index_id IN (0,1) THEN p.rows ELSE 0 END),
		CAST(SUM(au.total_pages)*8/1024.0 AS varchar(32)) + ' MB'
		FROM sys.tables t
		JOIN sys.schemas s ON s.schema_id = t.schema_id
		JOIN sys.partitions p ON p.object_id = t.object_id
		JOIN sys.allocation_units au ON au.container_id = p.partition_id
		GROUP BY s.name, t.name ORDER BY SUM(au.total_pages) DESC`); err == nil {
		for _, r := range rows {
			d.Tables = append(d.Tables, model.TableInfo{Schema: r[0], Name: r[1], Rows: r[2], Size: r[3]})
		}
	}

	// 文件组（SQL Server 的"表空间"对应物）
	if _, rows, err := QueryTable(db, `SELECT fg.name, COUNT(df.name),
		CAST(SUM(CAST(df.size AS bigint))*8/1024.0/1024 AS varchar(32)) + ' GB'
		FROM sys.filegroups fg LEFT JOIN sys.database_files df ON df.data_space_id = fg.data_space_id
		GROUP BY fg.name ORDER BY fg.name`); err == nil {
		for _, r := range rows {
			d.TBS = append(d.TBS, model.TBSInfo{Name: r[0], Files: r[1] + " 个文件", Size: r[2], Status: "-"})
		}
	}

	// 数据文件（含已用/剩余空间）
	if _, rows, err := QueryTable(db, `SELECT name, type_desc, physical_name,
		CAST(CAST(size AS bigint)*8/1024.0/1024 AS varchar(32)),
		CAST(FILEPROPERTY(name, 'SpaceUsed')*8/1024.0/1024 AS varchar(32)),
		state_desc
		FROM sys.database_files ORDER BY type_desc, name`); err == nil {
		for _, r := range rows {
			sizeGB, usedGB := parseSizeGB(r[3]), parseSizeGB(r[4])
			freeGB := sizeGB - usedGB
			d.TBS = append(d.TBS, model.TBSInfo{
				Name:   r[0],
				Size:   fmt.Sprintf("%.2f GB", sizeGB),
				Free:   fmt.Sprintf("%.2f GB", freeGB),
				Used:   fmt.Sprintf("%.0f%%", pct(usedGB, sizeGB)),
				Status: r[1] + " · " + r[5],
				Files:  r[2],
			})
		}
	}
	return d, nil
}

// parseSizeGB 解析 "1.50"/"1.50 GB" 风格数值。
func parseSizeGB(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "GB"))
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func pct(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total * 100
}

// osFromMssqlBanner 从 @@VERSION 横幅解析 OS（" on Windows Server 2019 Datacenter 10.0 ..."）。
func osFromMssqlBanner(banner string) string {
	i := strings.Index(banner, " on ")
	if i < 0 {
		return ""
	}
	rest := banner[i+4:]
	if cut := strings.IndexAny(rest, ",;"); cut >= 0 {
		rest = rest[:cut]
	}
	// 去掉尾部版本号 "10.0 <build> ..."
	re := regexp.MustCompile(`\s\d+\.\d+.*$`)
	return strings.TrimSpace(re.ReplaceAllString(strings.TrimSpace(rest), ""))
}

func (f mssqlProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "SQL Server @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: firstLine(QueryString(db, "SELECT @@VERSION", "-"))},
		model.KV{Key: "实例", Value: QueryString(db, "SELECT CAST(SERVERPROPERTY('ServerName') AS varchar(128))", "-")},
	)

	// 数据库分支（含恢复模式与状态）
	if _, rows, err := QueryTable(db, `SELECT name, state_desc, recovery_model_desc
		FROM sys.databases ORDER BY name`); err == nil {
		branch := addNode(root, newTNode("dbs", "数据库", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			status := "ok"
			if r[1] != "ONLINE" {
				status = "warn"
			}
			addNode(branch, newTNode(fmt.Sprintf("db-%d", i), r[0],
				r[2]+" · "+r[1], status))
		}
	}

	// 当前库数据文件分支
	if _, rows, err := QueryTable(db, `SELECT name, type_desc, physical_name,
		CAST(CAST(size AS bigint)*8/1024.0/1024 AS varchar(32)) + ' GB', state_desc
		FROM sys.database_files ORDER BY type_desc, name`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("tbs", "数据文件（当前库）", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			addNode(branch, newTNode(fmt.Sprintf("tbs-%d", i), r[0],
				r[3]+" · "+r[1]+" · "+r[4], "na", model.KV{Key: "路径", Value: r[2]}))
		}
	}

	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
