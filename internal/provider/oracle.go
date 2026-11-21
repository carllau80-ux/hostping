package provider

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"dbping/internal/model"
)

// oracleProvider Oracle（go-ora 纯 Go TNS 驱动，免 Instant Client）。
// 特点：普通业务账号常无 v$/dba_ 权限，全部查询走优雅降级并登记 warning。
type oracleProvider struct{}

func (oracleProvider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := OracleDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "oracle", dsn) }
}

func (f oracleProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1 FROM DUAL")
}

func (f oracleProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// 版本横幅（v$version）
	if banner := QueryString(db, "SELECT banner FROM v$version WHERE ROWNUM = 1", ""); banner != "" {
		o.VersionFull = banner
		// "Oracle Database 19c Enterprise Edition Release 19.0.0.0.0 - Production"
		if m := regexp.MustCompile(`Release (\S+)`).FindStringSubmatch(banner); len(m) >= 2 {
			o.Version = m[1]
		} else if m := regexp.MustCompile(`(\d+[cde]?)\s`).FindStringSubmatch(banner); len(m) >= 2 {
			o.Version = m[1]
		}
	} else {
		o.Warnings = append(o.Warnings, "无 v$version 权限，版本信息缺失")
	}

	// 服务器 OS：v$database.platform_name（如 "Linux x86 64-bit"）
	o.OS = QueryString(db, "SELECT platform_name FROM v$database", "")

	Try(o, "实例信息", func() error {
		if kvs, ok := QueryKV(db, `SELECT instance_name, host_name, status,
			TO_CHAR(startup_time, 'YYYY-MM-DD HH24:MI:SS') AS startup_time
			FROM v$instance`); ok {
			o.Instance = kvs[0].Value
			o.Hostname = kvs[1].Value
			o.Extra = append(o.Extra, model.KV{Key: "实例状态", Value: kvs[2].Value})
			o.StartupTime = kvs[3].Value
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(kvs[3].Value), time.Local); err == nil {
				o.Uptime = HumanDuration(int64(time.Since(t).Seconds()))
			}
		}
		return nil
	})
	// 数据库状态（不查 cdb 列：Oracle 10g/11g 的 v$database 无该列）
	Try(o, "数据库状态", func() error {
		if kvs, ok := QueryKV(db, `SELECT name, open_mode, log_mode FROM v$database`); ok {
			o.Extra = append(o.Extra, model.KV{Key: "DB名称", Value: kvs[0].Value})
			o.Extra = append(o.Extra, model.KV{Key: "打开模式", Value: kvs[1].Value})
			// 归档模式（Oracle 核心关注项）
			switch strings.ToUpper(kvs[2].Value) {
			case "ARCHIVELOG":
				o.ArchiveMode = "ARCHIVELOG 归档模式"
			case "NOARCHIVELOG":
				o.ArchiveMode = "NOARCHIVELOG 非归档"
			default:
				o.ArchiveMode = kvs[2].Value
			}
		}
		return nil
	})
	// CDB 多租户（12c+，10g/11g 无 cdb 列则静默跳过）
	if cdb := QueryString(db, "SELECT cdb FROM v$database", ""); strings.EqualFold(cdb, "YES") {
		o.Cluster = "CDB 多租户架构"
	}
	// 归档目的地（开启归档后展示路径；DataGuard 远端也会出现在这里）
	// 注意：Oracle 中 '' 即 NULL，不能写 destination != ''（会把全部行过滤掉）
	Try(o, "归档路径", func() error {
		_, rows, err := QueryTable(db, `SELECT destination FROM v$archive_dest
			WHERE status = 'VALID' AND destination IS NOT NULL
			ORDER BY dest_id`)
		if err == nil && len(rows) > 0 {
			dests := make([]string, 0, len(rows))
			for _, r := range rows {
				dests = append(dests, r[0])
			}
			o.ArchiveDest = strings.Join(dests, " ; ")
		}
		return nil
	})
	Try(o, "字符集", func() error {
		o.Charset = QueryString(db, `SELECT value FROM nls_database_parameters
			WHERE parameter = 'NLS_CHARACTERSET'`, "-")
		return nil
	})
	Try(o, "会话数", func() error {
		o.ConnCur = QueryString(db, "SELECT count(*) FROM v$session", "-")
		o.ConnMax = QueryString(db, `SELECT value FROM v$parameter WHERE name = 'processes'`, "-") + " (processes)"
		return nil
	})
	Try(o, "容量", func() error {
		sz := QueryString(db, `SELECT round(SUM(bytes)/1048576/1024, 2) FROM dba_data_files`, "")
		if sz != "" {
			o.DBSize = sz + " GB（数据文件）"
		}
		tc := QueryString(db, "SELECT count(*) FROM dba_tables", "")
		if tc != "" {
			o.TableCount = tc
		}
		return nil
	})
	Try(o, "PDB 列表", func() error {
		_, rows, err := QueryTable(db, "SELECT name, open_mode FROM v$pdbs ORDER BY name")
		if err != nil {
			return nil // 非 CDB，静默
		}
		for _, r := range rows {
			o.Extra = append(o.Extra, model.KV{Key: "PDB " + r[0], Value: r[1]})
		}
		return nil
	})

	if o.ArchiveMode == "" {
		o.ArchiveMode = "未知（无 v$database 权限）"
		o.Warnings = append(o.Warnings, "归档模式需要 v$database 查询权限（SELECT ANY DICTIONARY 或 DBA 角色）")
	}
	if o.Role == "" {
		o.Role = "-"
	}
	if o.Cluster == "" {
		o.Cluster = "单机"
	}
	return o, nil
}

func (f oracleProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 表空间：大小 + 剩余 + 状态（数据文件聚合到表空间行）
	tbsAgg := map[string]*model.TBSInfo{}
	var order []string
	if _, rows, err := QueryTable(db, `SELECT tablespace_name, status, contents FROM dba_tablespaces ORDER BY tablespace_name`); err == nil {
		for _, r := range rows {
			tbsAgg[r[0]] = &model.TBSInfo{Name: r[0], Status: r[1] + " · " + r[2], Size: "-", Free: "-", Files: "-"}
			order = append(order, r[0])
		}
	}
	if _, rows, err := QueryTable(db, `SELECT tablespace_name, round(SUM(bytes)/1048576/1024, 2) FROM dba_data_files GROUP BY tablespace_name`); err == nil {
		for _, r := range rows {
			if t, ok := tbsAgg[r[0]]; ok {
				t.Size = r[1] + " GB"
			} else {
				order = append(order, r[0])
				tbsAgg[r[0]] = &model.TBSInfo{Name: r[0], Status: "-", Size: r[1] + " GB", Free: "-", Files: "-"}
			}
		}
	}
	if _, rows, err := QueryTable(db, `SELECT tablespace_name, round(SUM(bytes)/1048576/1024, 2) FROM dba_free_space GROUP BY tablespace_name`); err == nil {
		for _, r := range rows {
			if t, ok := tbsAgg[r[0]]; ok {
				t.Free = r[1] + " GB"
			}
		}
	}
	filesByTbs := map[string][]string{}
	if _, rows, err := QueryTable(db, `SELECT tablespace_name, file_name FROM dba_data_files ORDER BY tablespace_name, file_id`); err == nil {
		for _, r := range rows {
			filesByTbs[r[0]] = append(filesByTbs[r[0]], r[1])
		}
	}
	for _, name := range order {
		t := tbsAgg[name]
		if fs := filesByTbs[name]; len(fs) > 0 {
			t.Files = fmt.Sprintf("%d 个文件", len(fs))
			show := fs
			if len(show) > 2 {
				show = show[:2]
				t.Files += "（" + strings.Join(show, " ; ") + " …）"
			} else {
				t.Files += "（" + strings.Join(show, " ; ") + "）"
			}
		}
		d.TBS = append(d.TBS, *t)
	}
	if len(order) == 0 {
		// 降级：user 视图
		if _, rows, err := QueryTable(db, `SELECT tablespace_name, status, contents FROM user_tablespaces`); err == nil {
			for _, r := range rows {
				d.TBS = append(d.TBS, model.TBSInfo{Name: r[0], Status: r[1] + " · " + r[2], Size: "-", Free: "-", Files: "-"})
			}
		}
	}

	// 大表 Top 20（dba_segments，无权限则 user_segments）
	if _, rows, err := QueryTable(db, `SELECT * FROM (
			SELECT owner, segment_name, round(SUM(bytes)/1048576, 1) mb
			FROM dba_segments WHERE segment_type IN ('TABLE','TABLE PARTITION')
			GROUP BY owner, segment_name ORDER BY SUM(bytes) DESC
		) WHERE ROWNUM <= 20`); err == nil {
		for _, r := range rows {
			d.Tables = append(d.Tables, model.TableInfo{Schema: r[0], Name: r[1], Size: r[2] + " MB"})
		}
	} else if _, rows, err := QueryTable(db, `SELECT * FROM (
			SELECT segment_name, round(SUM(bytes)/1048576, 1) mb
			FROM user_segments WHERE segment_type IN ('TABLE','TABLE PARTITION')
			GROUP BY segment_name ORDER BY SUM(bytes) DESC
		) WHERE ROWNUM <= 20`); err == nil {
		for _, r := range rows {
			d.Tables = append(d.Tables, model.TableInfo{Name: r[0], Size: r[1] + " MB"})
		}
	}

	// 模式列表 + 各模式表数
	if _, rows, err := QueryTable(db, `SELECT username FROM dba_users ORDER BY username`); err == nil {
		counts := map[string]string{}
		if _, rows, err := QueryTable(db, `SELECT owner, count(*) FROM dba_tables GROUP BY owner`); err == nil {
			for _, r := range rows {
				counts[r[0]] = r[1]
			}
		} else if _, rows, err := QueryTable(db, `SELECT owner, count(*) FROM all_tables GROUP BY owner`); err == nil {
			for _, r := range rows {
				counts[r[0]] = r[1]
			}
		}
		for _, r := range rows {
			note := ""
			switch strings.ToUpper(r[0]) {
			case "SYS", "SYSTEM", "SYSAUX", "OUTLN", "DBSNMP", "APPQOSSYS", "XDB", "ORDSYS":
				note = "（系统用户）"
			}
			d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Tables: counts[r[0]], Note: note})
		}
	}
	return d, nil
}

func (f oracleProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "Oracle @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: firstLine(QueryString(db, "SELECT banner FROM v$version WHERE ROWNUM = 1", "-"))},
		model.KV{Key: "归档模式", Value: QueryString(db, "SELECT log_mode FROM v$database", "未知")},
	)

	// PDB 分支（CDB 架构）
	if _, rows, err := QueryTable(db, "SELECT name, open_mode FROM v$pdbs ORDER BY name"); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("pdbs", "PDB 多租户", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			st := "ok"
			if !strings.EqualFold(r[1], "READ WRITE") {
				st = "warn"
			}
			addNode(branch, newTNode(fmt.Sprintf("pdb-%d", i), r[0], r[1], st))
		}
	}

	// 表空间分支（含数据文件）
	if _, rows, err := QueryTable(db, `SELECT t.tablespace_name, round(SUM(df.bytes)/1048576/1024, 2) || ' GB'
		FROM dba_tablespaces t JOIN dba_data_files df ON df.tablespace_name = t.tablespace_name
		GROUP BY t.tablespace_name ORDER BY t.tablespace_name`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("tbs", "表空间", fmt.Sprintf("%d 个", len(rows)), "na"))
		filesByTbs := map[string][]string{}
		if _, frows, err := QueryTable(db, `SELECT tablespace_name,
			file_name || '  (' || round(bytes/1048576/1024, 2) || ' GB)'
			FROM dba_data_files ORDER BY tablespace_name, file_id`); err == nil {
			for _, fr := range frows {
				filesByTbs[fr[0]] = append(filesByTbs[fr[0]], fr[1])
			}
		}
		for i, r := range rows {
			n := addNode(branch, newTNode(fmt.Sprintf("tbs-%d", i), r[0], r[1], "na"))
			for j, fn := range filesByTbs[r[0]] {
				addNode(n, newTNode(fmt.Sprintf("tbs-%d-f-%d", i, j), fn, "数据文件", "na"))
			}
		}
	}

	// 表空间分支（含数据文件）
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
