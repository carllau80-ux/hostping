package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"dbping/internal/model"
)

// hanaProvider SAP HANA 2.0（SAP 官方 go-hdb 纯 Go 驱动）。
type hanaProvider struct{}

func (hanaProvider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := HanaDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "hdb", dsn) }
}

func (f hanaProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1 FROM DUMMY")
}

func (f hanaProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	// 服务器 OS（M_HOST_INFORMATION 键值对，采不到则留空）
	o.OS = QueryString(db, `SELECT "VALUE" FROM M_HOST_INFORMATION WHERE "KEY" = 'os_name'`, "")
	if o.OS != "" {
		if v := QueryString(db, `SELECT "VALUE" FROM M_HOST_INFORMATION WHERE "KEY" = 'os_version'`, ""); v != "" {
			o.OS = strings.TrimSpace(o.OS + " " + v)
		}
	}
	Try(o, "系统信息", func() error {
		if kvs, ok := QueryKV(db, `SELECT SYSTEM_ID, DATABASE_NAME, VERSION, START_TIME, USAGE FROM M_DATABASE`); ok {
			o.Instance = kvs[0].Value + " / " + kvs[1].Value
			o.Version = kvs[2].Value
			o.VersionFull = "HANA " + kvs[2].Value + "（" + kvs[4].Value + "）"
			o.StartupTime = kvs[3].Value
			o.Extra = append(o.Extra, model.KV{Key: "用途", Value: kvs[4].Value})
		}
		return nil
	})
	Try(o, "运行时长", func() error {
		sec := QueryString(db, `SELECT SECONDS_BETWEEN(START_TIME, CURRENT_TIMESTAMP) FROM M_DATABASE`, "0")
		o.Uptime = HumanDuration(parseI64(sec))
		return nil
	})
	// HANA 日志模式（归档对应物）：M_SYSTEM_OVERVIEW
	Try(o, "日志模式", func() error {
		lm := QueryString(db, `SELECT VALUE FROM M_SYSTEM_OVERVIEW
			WHERE SECTION = 'System Information' AND NAME = 'Log Mode'`, "")
		switch strings.ToLower(lm) {
		case "normal":
			o.ArchiveMode = "NORMAL（日志归档已启用）"
		case "overwrite":
			o.ArchiveMode = "OVERWRITE（覆盖写，不归档！）"
		default:
			o.ArchiveMode = lm
		}
		return nil
	})
	Try(o, "服务状态", func() error {
		n := QueryString(db, `SELECT COUNT(*) FROM M_SERVICES WHERE ACTIVE_STATUS = 'YES'`, "0")
		total := QueryString(db, "SELECT COUNT(*) FROM M_SERVICES", "0")
		o.Extra = append(o.Extra, model.KV{Key: "活跃服务", Value: n + " / " + total})
		hosts := QueryString(db, "SELECT COUNT(DISTINCT HOST) FROM M_SERVICES", "1")
		if parseI64(hosts) > 1 {
			o.Cluster = "Scale-Out 集群（" + hosts + " 主机）"
			o.Role = "分布式节点"
		}
		return nil
	})
	Try(o, "租户库", func() error {
		_, rows, err := QueryTable(db, `SELECT DATABASE_NAME, START_TIME FROM M_DATABASES`)
		if err != nil {
			return nil // 单容器部署，无 MDC
		}
		for _, r := range rows {
			o.Extra = append(o.Extra, model.KV{Key: "租户 " + r[0], Value: "启动:" + r[1]})
		}
		if len(rows) > 1 {
			o.Cluster = "MDC 多租户（" + fmt.Sprintf("%d 库", len(rows)) + "）"
		}
		return nil
	})
	Try(o, "连接数", func() error {
		o.ConnCur = QueryString(db, "SELECT COUNT(*) FROM M_CONNECTIONS", "-")
		return nil
	})
	Try(o, "容量", func() error {
		sz := QueryString(db, `SELECT round(SUM(MEMORY_SIZE_IN_TOTAL)/1048576/1024, 2) FROM M_TABLES`, "")
		if sz != "" {
			o.DBSize = sz + " GB（内存占用）"
		}
		tc := QueryString(db, "SELECT COUNT(*) FROM M_TABLES", "")
		o.TableCount = tc
		return nil
	})

	// 归档日志备份路径（开启归档后展示）
	Try(o, "归档路径", func() error {
		if p := QueryString(db, `SELECT VALUE FROM M_INIFILE_CONTENTS
			WHERE KEY = 'basepath_logbackup' AND VALUE != ''`, ""); p != "" {
			o.ArchiveDest = p
		}
		return nil
	})
	if o.Charset == "" {
		o.Charset = "UTF-8（UNICODE）"
	}
	if o.Timezone == "" {
		o.Timezone = "-"
	}
	if o.Role == "" {
		o.Role = "单机"
	}
	if o.Cluster == "" {
		o.Cluster = "单机"
	}
	if o.ReadOnly == "" {
		o.ReadOnly = "-"
	}
	return o, nil
}

func (f hanaProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// Schema 列表
	if _, rows, err := QueryTable(db, `SELECT SCHEMA_NAME, COUNT(*)
		FROM M_TABLES GROUP BY SCHEMA_NAME ORDER BY 2 DESC LIMIT 30`); err == nil {
		for _, r := range rows {
			note := ""
			if strings.HasPrefix(r[0], "SAP") || strings.HasPrefix(r[0], "_SYS") {
				note = "（系统模式）"
			}
			d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Tables: r[1], Note: note})
		}
	}

	// 大表 Top 20
	if _, rows, err := QueryTable(db, `SELECT SCHEMA_NAME, TABLE_NAME, RECORD_COUNT,
		round(MEMORY_SIZE_IN_TOTAL/1048576, 1) || ' MB'
		FROM M_TABLES ORDER BY MEMORY_SIZE_IN_TOTAL DESC LIMIT 20`); err == nil {
		for _, r := range rows {
			d.Tables = append(d.Tables, model.TableInfo{Schema: r[0], Name: r[1], Rows: r[2], Size: r[3]})
		}
	}

	// 卷/磁盘（HANA 的"表空间"对应物）：容量 + 剩余 + 路径
	if _, rows, err := QueryTable(db, `SELECT HOST, USAGE_TYPE, PATH,
		round(TOTAL_SIZE/1048576/1024, 2) || ' GB',
		round(USED_SIZE/1048576/1024, 2) || ' GB',
		round((TOTAL_SIZE - USED_SIZE)/1048576/1024, 2) || ' GB'
		FROM M_DISKS WHERE USAGE_TYPE IN ('DATA','LOG') ORDER BY HOST, USAGE_TYPE`); err == nil {
		for _, r := range rows {
			d.TBS = append(d.TBS, model.TBSInfo{
				Name: r[0] + " · " + r[1], Size: r[3], Used: r[4], Free: r[5], Files: r[2], Status: "ok",
			})
		}
	}

	// 服务列表（集群拓扑）
	if cols, rows, err := QueryTable(db, `SELECT HOST, SERVICE_NAME, PORT, ACTIVE_STATUS, PROCESS_TYPE
		FROM M_SERVICES ORDER BY HOST, SERVICE_NAME`); err == nil {
		for _, r := range rows {
			m := mapCols(cols, r)
			st := "运行中"
			if !strings.EqualFold(m["ACTIVE_STATUS"], "YES") {
				st = "停止"
			}
			d.Services = append(d.Services, model.SvcInfo{
				Name:   m["HOST"] + ":" + m["PORT"],
				Role:   m["SERVICE_NAME"],
				Status: st,
				Host:   m["HOST"],
				Detail: "进程类型 " + m["PROCESS_TYPE"],
			})
		}
	}
	return d, nil
}

func (f hanaProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "HANA @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: QueryString(db, "SELECT VERSION FROM M_DATABASE", "-")},
		model.KV{Key: "日志模式", Value: QueryString(db, `SELECT VALUE FROM M_SYSTEM_OVERVIEW
			WHERE SECTION = 'System Information' AND NAME = 'Log Mode'`, "-")},
	)

	// 磁盘/卷分支（数据/日志文件落盘位置）
	if _, rows, err := QueryTable(db, `SELECT HOST, USAGE_TYPE, PATH,
		round(TOTAL_SIZE/1048576/1024, 2) || ' GB',
		round((TOTAL_SIZE - USED_SIZE)/1048576/1024, 2) || ' GB'
		FROM M_DISKS WHERE USAGE_TYPE IN ('DATA','LOG') ORDER BY HOST, USAGE_TYPE`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("tbs", "磁盘/卷", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			addNode(branch, newTNode(fmt.Sprintf("tbs-%d", i), r[0]+" · "+r[1],
				r[3]+" / 剩余 "+r[4], "na", model.KV{Key: "路径", Value: r[2]}))
		}
	}

	// 主机/服务分支（Scale-Out 拓扑）
	if cols, rows, err := QueryTable(db, `SELECT HOST, SERVICE_NAME, PORT, ACTIVE_STATUS
		FROM M_SERVICES ORDER BY HOST, SERVICE_NAME`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("svc", "服务拓扑", fmt.Sprintf("%d 个服务", len(rows)), "na"))
		byHost := map[string]*model.TNode{}
		for i, r := range rows {
			m := mapCols(cols, r)
			hostNode, ok := byHost[m["HOST"]]
			if !ok {
				hostNode = addNode(branch, newTNode("host-"+m["HOST"], "主机 "+m["HOST"], "", "ok"))
				byHost[m["HOST"]] = hostNode
			}
			st := "ok"
			sub := ":" + m["PORT"]
			if !strings.EqualFold(m["ACTIVE_STATUS"], "YES") {
				st = "err"
				sub += "（停止）"
			}
			addNode(hostNode, newTNode(fmt.Sprintf("svc-%d", i), m["SERVICE_NAME"], sub, st))
		}
	}

	// Schema 分支
	if _, rows, err := QueryTable(db, `SELECT SCHEMA_NAME, COUNT(*) FROM M_TABLES
		GROUP BY SCHEMA_NAME ORDER BY 2 DESC LIMIT 15`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("schemas", "Schema", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			addNode(branch, newTNode(fmt.Sprintf("sch-%d", i), r[0], r[1]+" 表", "na"))
		}
	}
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
