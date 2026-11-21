package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"dbping/internal/model"
)

// dmProvider 达梦 DM8（gitee.com/chunanyong/dm，官方驱动镜像，纯 Go）。
// 系统视图体系与 Oracle 类似：V$VERSION / V$INSTANCE / V$DATABASE / DBA_*。
type dmProvider struct{}

func (dmProvider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := DmDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "dm", dsn) }
}

func (f dmProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1 FROM DUAL")
}

func (f dmProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	if banner := QueryString(db, "SELECT BANNER FROM V$VERSION WHERE ROWNUM = 1", ""); banner != "" {
		o.VersionFull = banner
		if fs := strings.Fields(banner); len(fs) >= 2 {
			o.Version = fs[len(fs)-2]
		}
	}

	// 服务器 OS（V$SYSTEMINFO，采不到则留空）
	o.OS = QueryString(db, "SELECT OS_NAME FROM V$SYSTEMINFO", "")

	Try(o, "实例信息", func() error {
		if kvs, ok := QueryKV(db, `SELECT INSTANCE_NAME, HOST_NAME, STATUS$, STARTUP_TIME FROM V$INSTANCE`); ok {
			o.Instance = kvs[0].Value
			o.Hostname = kvs[1].Value
			o.Extra = append(o.Extra, model.KV{Key: "实例状态", Value: kvs[2].Value})
			o.StartupTime = kvs[3].Value
			if t, err := time.Parse("2006-01-02 15:04:05.000000", strings.TrimSpace(kvs[3].Value)); err == nil {
				o.Uptime = HumanDuration(int64(time.Since(t).Seconds()))
			}
		}
		return nil
	})
	Try(o, "数据库状态", func() error {
		if kvs, ok := QueryKV(db, `SELECT NAME, OPEN_MODE, ARCH_MODE FROM V$DATABASE`); ok {
			o.Extra = append(o.Extra, model.KV{Key: "库名", Value: kvs[0].Value})
			o.Extra = append(o.Extra, model.KV{Key: "打开模式", Value: kvs[1].Value})
			switch strings.ToUpper(kvs[2].Value) {
			case "Y", "ARCHIVELOG":
				o.ArchiveMode = "ARCHIVELOG 归档模式"
			case "N", "NOARCHIVELOG":
				o.ArchiveMode = "NOARCHIVELOG 非归档"
			default:
				o.ArchiveMode = kvs[2].Value
			}
		}
		return nil
	})
	Try(o, "字符集", func() error {
		// SF_GET_UNICODE_FLAG(): 0=GBK 1=UTF-8
		u := QueryString(db, "SELECT SF_GET_UNICODE_FLAG()", "")
		switch u {
		case "0":
			o.Charset = "GBK"
		case "1":
			o.Charset = "UTF-8"
		case "2":
			o.Charset = "EUC-KR"
		default:
			o.Charset = u
		}
		return nil
	})
	Try(o, "会话数", func() error {
		o.ConnCur = QueryString(db, "SELECT COUNT(*) FROM V$SESSION", "-")
		o.ConnMax = QueryString(db, `SELECT PARA_VALUE FROM V$DM_INI WHERE PARA_NAME = 'MAX_SESSIONS'`, "-")
		return nil
	})
	Try(o, "容量", func() error {
		sz := QueryString(db, `SELECT round(SUM(TOTAL_SIZE)*PAGE()/1048576/1024, 2) FROM V$TABLESPACE`, "")
		if sz != "" {
			o.DBSize = sz + " GB（表空间）"
		} else if sz2 := QueryString(db, `SELECT round(SUM(BYTES)/1048576/1024, 2) FROM DBA_DATA_FILES`, ""); sz2 != "" {
			o.DBSize = sz2 + " GB（数据文件）"
		}
		tc := QueryString(db, "SELECT COUNT(*) FROM DBA_TABLES", "")
		if tc != "" {
			o.TableCount = tc
		}
		return nil
	})

	// 归档目的地（V$DM_ARCH_INI）
	Try(o, "归档路径", func() error {
		_, rows, err := QueryTable(db, `SELECT ARCH_DEST FROM V$DM_ARCH_INI WHERE ARCH_DEST IS NOT NULL`)
		if err == nil && len(rows) > 0 {
			dests := make([]string, 0, len(rows))
			for _, r := range rows {
				dests = append(dests, r[0])
			}
			o.ArchiveDest = strings.Join(dests, " ; ")
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
	return o, nil
}

func (f dmProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 表空间：大小 + 剩余 + 状态 + 文件（DBA_* 视图，聚合到表空间行）
	tbsAgg := map[string]*model.TBSInfo{}
	var order []string
	if _, rows, err := QueryTable(db, `SELECT TABLESPACE_NAME, STATUS FROM DBA_TABLESPACES ORDER BY TABLESPACE_NAME`); err == nil {
		for _, r := range rows {
			tbsAgg[r[0]] = &model.TBSInfo{Name: r[0], Status: r[1], Size: "-", Free: "-", Files: "-"}
			order = append(order, r[0])
		}
	}
	if _, rows, err := QueryTable(db, `SELECT TABLESPACE_NAME, round(SUM(BYTES)/1048576, 1) FROM DBA_DATA_FILES GROUP BY TABLESPACE_NAME`); err == nil {
		for _, r := range rows {
			if t, ok := tbsAgg[r[0]]; ok {
				t.Size = r[1] + " MB"
			} else {
				order = append(order, r[0])
				tbsAgg[r[0]] = &model.TBSInfo{Name: r[0], Status: "-", Size: r[1] + " MB", Free: "-", Files: "-"}
			}
		}
	}
	if _, rows, err := QueryTable(db, `SELECT TABLESPACE_NAME, round(SUM(BYTES)/1048576, 1) FROM DBA_FREE_SPACE GROUP BY TABLESPACE_NAME`); err == nil {
		for _, r := range rows {
			if t, ok := tbsAgg[r[0]]; ok {
				t.Free = r[1] + " MB"
			}
		}
	}
	filesByTbs := map[string][]string{}
	if _, rows, err := QueryTable(db, `SELECT TABLESPACE_NAME, FILE_NAME FROM DBA_DATA_FILES ORDER BY TABLESPACE_NAME`); err == nil {
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

	// 大表 Top 20
	if _, rows, err := QueryTable(db, `SELECT * FROM (
			SELECT OWNER, SEGMENT_NAME, round(SUM(BYTES)/1048576, 1) MB
			FROM DBA_SEGMENTS WHERE SEGMENT_TYPE = 'TABLE'
			GROUP BY OWNER, SEGMENT_NAME ORDER BY SUM(BYTES) DESC
		) WHERE ROWNUM <= 20`); err == nil {
		for _, r := range rows {
			d.Tables = append(d.Tables, model.TableInfo{Schema: r[0], Name: r[1], Size: r[2] + " MB"})
		}
	}

	// 模式/用户列表 + 各模式表数
	if _, rows, err := QueryTable(db, `SELECT USERNAME FROM DBA_USERS ORDER BY USERNAME`); err == nil {
		counts := map[string]string{}
		if _, rows, err := QueryTable(db, `SELECT OWNER, COUNT(*) FROM DBA_TABLES GROUP BY OWNER`); err == nil {
			for _, r := range rows {
				counts[r[0]] = r[1]
			}
		}
		for _, r := range rows {
			d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Tables: counts[r[0]]})
		}
	}
	return d, nil
}

func (f dmProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "DM8 @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: firstLine(QueryString(db, "SELECT BANNER FROM V$VERSION WHERE ROWNUM = 1", "-"))},
		model.KV{Key: "归档模式", Value: QueryString(db, "SELECT ARCH_MODE FROM V$DATABASE", "未知")},
	)

	// 表空间分支（含数据文件）
	if _, rows, err := QueryTable(db, `SELECT TABLESPACE_NAME, round(SUM(BYTES)/1048576/1024, 2) || ' GB'
		FROM DBA_DATA_FILES GROUP BY TABLESPACE_NAME ORDER BY TABLESPACE_NAME`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("tbs", "表空间", fmt.Sprintf("%d 个", len(rows)), "na"))
		filesByTbs := map[string][]string{}
		if _, frows, err := QueryTable(db, `SELECT TABLESPACE_NAME,
			FILE_NAME || '  (' || round(BYTES/1048576, 1) || ' MB)'
			FROM DBA_DATA_FILES ORDER BY TABLESPACE_NAME`); err == nil {
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

	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
