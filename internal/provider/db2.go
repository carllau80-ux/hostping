package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dbping/internal/model"
)

// db2Provider IBM Db2（obaydullahmhs/go-db2，纯 Go DRDA 协议实现，零 cgo）。
type db2Provider struct{}

// Db2DSN go-db2 的 key=value DSN：
// host=..;port=..;database=..;user=..;password=..;timeout=秒;allow_plaintext_auth=true
// 说明：驱动默认禁止"非 SSL + 明文认证"（安全默认），内网测试库普遍是非 SSL，
// 这里显式放行；启用 SSL 时该参数无影响。可通过附加参数 allow_plaintext_auth=false 覆盖。
func Db2DSN(c *model.Conn) string {
	parts := []string{
		"host=" + c.Host,
		"port=" + strconv.Itoa(c.Port),
		"database=" + c.Database,
		"user=" + c.User,
		"password=" + c.Password,
		"timeout=" + strconv.Itoa(timeoutSecs(c)),
		"allow_plaintext_auth=true",
	}
	for k, v := range parseParams(c.Params) {
		switch strings.ToLower(k) {
		case "host", "server", "hostname", "port", "database", "dbname", "db",
			"user", "uid", "username", "password", "pwd":
			continue
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ";")
}

func (db2Provider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := Db2DSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "db2", dsn) }
}

func (f db2Provider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1 FROM SYSIBM.SYSDUMMY1")
}

func (f db2Provider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// 版本与实例（SYSIBMADM 管理视图，无权限则降级）
	if kvs, ok := QueryKV(db, `SELECT INST_NAME, INST_VERSION, HOST_NAME, OS_NAME FROM SYSIBMADM.ENV_INST_INFO`); ok {
		o.Instance = kvs[0].Value
		o.Version = kvs[1].Value
		o.VersionFull = "Db2 " + kvs[1].Value
		o.Hostname = kvs[2].Value
		o.OS = kvs[3].Value
	} else {
		o.Warnings = append(o.Warnings, "SYSIBMADM.ENV_INST_INFO 采集失败（可能权限不足）")
	}
	Try(o, "启动时间", func() error {
		o.StartupTime = QueryString(db, `SELECT TO_CHAR(INST_START_TIME, 'YYYY-MM-DD HH24:MI:SS') FROM SYSIBMADM.ENV_INST_INFO`, "")
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(o.StartupTime), time.Local); err == nil {
			o.Uptime = HumanDuration(int64(time.Since(t).Seconds()))
		}
		return nil
	})
	Try(o, "归档模式", func() error {
		v := QueryString(db, `SELECT VALUE FROM SYSIBMADM.DB_CFG WHERE NAME = 'LOGARCHMETH1'`, "")
		if v == "" {
			return nil
		}
		switch strings.ToUpper(v) {
		case "OFF":
			o.ArchiveMode = "LOGARCHMETH1=OFF 非归档"
		default:
			o.ArchiveMode = "LOGARCHMETH1=" + v
			if strings.HasPrefix(strings.ToUpper(v), "DISK:") {
				o.ArchiveDest = v[5:]
			}
		}
		return nil
	})
	Try(o, "连接数", func() error {
		o.ConnCur = QueryString(db, "SELECT COUNT(*) FROM SYSIBMADM.APPLICATIONS", "-")
		o.ConnMax = QueryString(db, `SELECT VALUE FROM SYSIBMADM.DB_CFG WHERE NAME = 'MAX_COORD_AGENTS'`, "-")
		return nil
	})
	Try(o, "容量", func() error {
		kb := parseI64(QueryString(db, "SELECT SUM(TBSP_TOTAL_SIZE_KB) FROM SYSIBMADM.TBSP_UTILIZATION", "0"))
		if kb > 0 {
			o.DBSize = HumanBytes(kb * 1024)
		}
		o.TableCount = QueryString(db, `SELECT COUNT(*) FROM SYSCAT.TABLES WHERE TYPE = 'T' AND TABSCHEMA NOT LIKE 'SYS%'`, "0")
		return nil
	})
	Try(o, "HADR", func() error {
		_, rows, err := QueryTable(db, "SELECT HADR_STATE, HADR_SYNCMODE FROM SYSIBMADM.HADR_INFO")
		if err == nil && len(rows) > 0 {
			o.Cluster = "HADR 高可用"
			o.Role = "HADR " + rows[0][0]
		}
		return nil
	})

	if o.ArchiveMode == "" {
		o.ArchiveMode = "未知"
	}
	if o.Role == "" {
		o.Role = "单机"
	}
	if o.Cluster == "" {
		o.Cluster = "单机"
	}
	o.Charset = "-"
	o.Timezone = "-"
	o.ReadOnly = "-"
	return o, nil
}

func (f db2Provider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 模式列表
	if _, rows, err := QueryTable(db, `SELECT SCHEMANAME FROM SYSCAT.SCHEMAS
		WHERE SCHEMANAME NOT LIKE 'SYS%' ORDER BY SCHEMANAME`); err == nil {
		for _, r := range rows {
			note := ""
			switch r[0] {
			case "NULLID", "SQLJ", "DB2INST1":
				note = ""
			}
			d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Note: note})
		}
	}
	// 大表 Top 20（CARD 为统计值，未 runstats 时为 -1）
	if _, rows, err := QueryTable(db, `SELECT TABSCHEMA, TABNAME, CARD FROM SYSCAT.TABLES
		WHERE TYPE = 'T' AND TABSCHEMA NOT LIKE 'SYS%'
		ORDER BY CARD DESC FETCH FIRST 20 ROWS ONLY`); err == nil {
		for _, r := range rows {
			rowsStr := r[2]
			if rowsStr == "-1" {
				rowsStr = "-"
			}
			d.Tables = append(d.Tables, model.TableInfo{Schema: r[0], Name: r[1], Rows: rowsStr})
		}
	}
	// 表空间
	if _, rows, err := QueryTable(db, `SELECT TBSP_NAME,
		round(TBSP_TOTAL_SIZE_KB/1024.0, 1) || ' MB',
		round(TBSP_FREE_SIZE_KB/1024.0, 1) || ' MB', TBSP_STATE
		FROM SYSIBMADM.TBSP_UTILIZATION ORDER BY TBSP_NAME`); err == nil {
		for _, r := range rows {
			d.TBS = append(d.TBS, model.TBSInfo{Name: r[0], Size: r[1], Free: r[2], Status: r[3]})
		}
	}
	return d, nil
}

func (f db2Provider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "Db2 @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: QueryString(db, "SELECT INST_VERSION FROM SYSIBMADM.ENV_INST_INFO", "-")},
	)

	// 表空间分支
	if _, rows, err := QueryTable(db, `SELECT TBSP_NAME, TBSP_TYPE,
		round(TBSP_TOTAL_SIZE_KB/1024.0, 1) || ' MB', TBSP_STATE
		FROM SYSIBMADM.TBSP_UTILIZATION ORDER BY TBSP_NAME`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("tbs", "表空间", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			addNode(branch, newTNode(fmt.Sprintf("tbs-%d", i), r[0], r[2]+" · "+r[1]+" · "+r[3], "na"))
		}
	}

	// 模式分支
	if _, rows, err := QueryTable(db, `SELECT SCHEMANAME FROM SYSCAT.SCHEMAS
		WHERE SCHEMANAME NOT LIKE 'SYS%' ORDER BY SCHEMANAME`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("dbs", "模式", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			addNode(branch, newTNode(fmt.Sprintf("db-%d", i), r[0], "", "ok"))
		}
	}
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
