package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"dbping/internal/model"
)

// sybaseProvider SAP Sybase ASE（thda/tds 纯 Go FreeTDS/TDS 协议实现，零 cgo）。
type sybaseProvider struct{}

func (sybaseProvider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := SybaseDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "tds", dsn) }
}

func (f sybaseProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1")
}

func (f sybaseProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// 版本横幅: "Adaptive Server Enterprise/16.0 SP03 PL01/EBP 16.0 SP03 ..."
	banner := QueryString(db, "SELECT @@version", "-")
	o.VersionFull = firstLine(banner)
	if m := strings.SplitN(banner, "/", 2); len(m) >= 2 {
		if fs := strings.Fields(m[1]); len(fs) >= 1 {
			o.Version = fs[0]
		}
	}

	Try(o, "实例信息", func() error {
		o.Instance = QueryString(db, "SELECT @@servername", "-")
		o.Hostname = QueryString(db, "SELECT @@hostname", "")
		return nil
	})
	Try(o, "连接数", func() error {
		o.ConnCur = QueryString(db, "SELECT COUNT(*) FROM master..sysprocesses", "-")
		o.ConnMax = QueryString(db, `SELECT value_in_use FROM master..sysconfigures
			WHERE name = 'number of user connections'`, "-") + " (user connections)"
		return nil
	})
	Try(o, "对象数量", func() error {
		o.TableCount = QueryString(db, "SELECT COUNT(*) FROM sysobjects WHERE type = 'U'", "0")
		return nil
	})
	Try(o, "字符集", func() error {
		o.Charset = QueryString(db, `SELECT cs.name FROM master..sysconfigures c,
			master..syscharsets cs WHERE c.name = 'default character set id'
			AND cs.id = c.value`, "")
		return nil
	})

	Try(o, "日志模式", func() error {
		// ASE 的"日志模式"对应物：trunc log on chkpt（检查点自动截断事务日志）
		v := QueryString(db, `SELECT CAST(DATABASEPROPERTY(DB_NAME(), 'trunc log on chkpt') AS varchar(10))`, "")
		switch strings.TrimSpace(v) {
		case "1":
			o.ArchiveMode = "trunc log on chkpt=true（日志自动截断，非归档）"
			o.ArchiveDest = "-（无归档日志；依赖 dump transaction 备份）"
		case "0":
			o.ArchiveMode = "trunc log on chkpt=false（日志保留，需手工 dump transaction）"
			o.ArchiveDest = "-（无归档日志；依赖 dump transaction 备份）"
		default:
			o.ArchiveMode = "未知（DATABASEPROPERTY 不可用）"
		}
		return nil
	})

	o.ArchiveMode = orDefault(o.ArchiveMode, "-（ASE 无在线归档日志，采用 dump 备份）")
	o.Charset = orDefault(o.Charset, "-")
	o.Timezone = "-"
	o.ReadOnly = "-"
	o.Role = "单机"
	o.Cluster = "单机"
	o.Extra = append(o.Extra, model.KV{Key: "兼容协议", Value: "FreeTDS/TDS（纯 Go 驱动）"})
	return o, nil
}

func (f sybaseProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 库列表
	if _, rows, err := QueryTable(db, `SELECT name FROM master..sysdatabases ORDER BY name`); err == nil {
		for _, r := range rows {
			note := ""
			switch strings.ToLower(r[0]) {
			case "master", "model", "tempdb", "sybsystemprocs", "sybsystemdb":
				note = "（系统库）"
			}
			d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Note: note})
		}
	}
	// 当前库表数
	tc := QueryString(db, "SELECT COUNT(*) FROM sysobjects WHERE type = 'U'", "")
	if tc != "" && c.Database != "" {
		for i := range d.Dbs {
			if strings.EqualFold(d.Dbs[i].Name, c.Database) {
				d.Dbs[i].Tables = tc
			}
		}
	}
	// 大表 Top 20（sysindexes 行数估算）
	if _, rows, err := QueryTable(db, `SELECT o.name, i.rowcnt(i.doampg)
		FROM sysindexes i, sysobjects o
		WHERE o.id = i.id AND i.indid IN (0,1) AND o.type = 'U'
		ORDER BY i.rowcnt(i.doampg) DESC`); err == nil {
		for i, r := range rows {
			if i >= 20 {
				break
			}
			d.Tables = append(d.Tables, model.TableInfo{Name: r[0], Rows: r[1]})
		}
	}
	return d, nil
}

func (f sybaseProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "Sybase ASE @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: firstLine(QueryString(db, "SELECT @@version", "-"))},
	)

	// 数据库分支
	if _, rows, err := QueryTable(db, `SELECT name FROM master..sysdatabases ORDER BY name`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("dbs", "数据库", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			note := ""
			status := "ok"
			switch strings.ToLower(r[0]) {
			case "master", "model", "tempdb", "sybsystemprocs", "sybsystemdb":
				note = "（系统库）"
				status = "na"
			}
			addNode(branch, newTNode(fmt.Sprintf("db-%d", i), r[0], note, status))
		}
	}

	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
