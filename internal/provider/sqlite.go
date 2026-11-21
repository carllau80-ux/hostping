package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"dbping/internal/model"
)

// sqliteProvider SQLite 本地文件（modernc.org/sqlite 纯 Go 实现，零 cgo）。
type sqliteProvider struct{}

func (sqliteProvider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := SQLiteDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "sqlite", dsn) }
}

// Ping SQLite 为本地文件：无 TCP 阶段，仅 打开/认证 与 SQL 两级。
func (f sqliteProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	res := &model.PingResult{ConnID: c.ID, Stage: "auth", CheckedAt: time.Now().UnixMilli()}
	start := time.Now()
	db, err := f.open(c)()
	if err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	defer db.Close()
	ctx, cancel := DBCtx(c)
	defer cancel()

	s := time.Now()
	var one int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.Stage = "ok"
	res.OK = true
	res.SQLMs = time.Since(s).Milliseconds()
	res.TotalMs = time.Since(start).Milliseconds()
	return res, nil
}

func (f sqliteProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: 0}

	o.Version = QueryString(db, "SELECT sqlite_version()", "-")
	o.VersionFull = "SQLite " + o.Version
	o.Instance = "SQLite 文件"
	o.Hostname = SQLiteDSN(c) // 文件路径
	o.OS = "-"

	Try(o, "容量", func() error {
		pages := parseI64(QueryString(db, "PRAGMA page_count", "0"))
		psize := parseI64(QueryString(db, "PRAGMA page_size", "0"))
		if pages > 0 && psize > 0 {
			o.DBSize = HumanBytes(pages * psize)
			o.Extra = append(o.Extra, model.KV{Key: "页数/页大小", Value: fmt.Sprintf("%d / %d B", pages, psize)})
		}
		return nil
	})
	Try(o, "日志模式", func() error {
		jm := QueryString(db, "PRAGMA journal_mode", "-")
		switch strings.ToLower(jm) {
		case "wal":
			o.ArchiveMode = "WAL（写前日志）"
		case "delete":
			o.ArchiveMode = "DELETE（回滚日志）"
		default:
			o.ArchiveMode = jm
		}
		return nil
	})
	Try(o, "对象数量", func() error {
		o.TableCount = QueryString(db, `SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`, "0")
		return nil
	})
	Try(o, "完整性", func() error {
		r := QueryString(db, "PRAGMA integrity_check", "-")
		o.Extra = append(o.Extra, model.KV{Key: "integrity_check", Value: r})
		return nil
	})

	o.Charset = "UTF-8"
	o.Timezone = "-"
	o.ReadOnly = "-"
	o.Role = "本地文件"
	o.Cluster = "单机"
	o.ConnCur = "-"
	o.ConnMax = "-"
	return o, nil
}

func (f sqliteProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 表清单（前 50 个，附行数）
	if _, rows, err := QueryTable(db, `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`); err == nil {
		for i, r := range rows {
			entry := model.DbInfo{Name: r[0]}
			if i < 30 {
				entry.Tables = QueryString(db, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, strings.ReplaceAll(r[0], `"`, `""`)), "-")
			}
			d.Dbs = append(d.Dbs, entry)
			if i < 20 {
				d.Tables = append(d.Tables, model.TableInfo{Name: r[0]})
			}
		}
	}
	// 文件信息
	d.TBS = append(d.TBS, model.TBSInfo{
		Name: "数据库文件", Files: SQLiteDSN(c),
		Size:  QueryString(db, `SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`, "-"),
		Free:  "-",
		Status: QueryString(db, "PRAGMA journal_mode", "-"),
	})
	return d, nil
}

func (f sqliteProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "SQLite · "+SQLiteDSN(c), "ok",
		model.KV{Key: "版本", Value: QueryString(db, "SELECT sqlite_version()", "-")},
	)

	if _, rows, err := QueryTable(db, `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("dbs", "数据表", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			if i >= 30 {
				addNode(branch, newTNode("db-more", fmt.Sprintf("…共 %d 个表", len(rows)), "", "na"))
				break
			}
			cnt := QueryString(db, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, strings.ReplaceAll(r[0], `"`, `""`)), "-")
			addNode(branch, newTNode(fmt.Sprintf("db-%d", i), r[0], cnt+" 行", "ok"))
		}
	}
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
