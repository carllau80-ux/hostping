package provider

import (
	"context"
	"database/sql"
	"fmt"

	"dbping/internal/model"
)

// chProvider ClickHouse（clickhouse-go/v2 原生 TCP 协议，纯 Go）。
type chProvider struct{}

func (chProvider) open(c *model.Conn) func() (*sql.DB, error) {
	dsn := CHDSN(c)
	return func() (*sql.DB, error) { return OpenSQL(c, "clickhouse", dsn) }
}

func (f chProvider) Ping(_ context.Context, c *model.Conn) (*model.PingResult, error) {
	return PingGeneric(c, f.open(c), "SELECT 1")
}

func (f chProvider) Overview(_ context.Context, c *model.Conn) (*model.Overview, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	o := &model.Overview{ConnID: c.ID, Port: c.Port}

	o.Version = QueryString(db, "SELECT version()", "-")
	o.VersionFull = "ClickHouse " + o.Version

	Try(o, "主机名", func() error {
		o.Hostname = QueryString(db, "SELECT hostName()", "")
		return nil
	})
	Try(o, "运行时长", func() error {
		o.Uptime = HumanDuration(parseI64(QueryString(db, "SELECT uptime()", "0")))
		return nil
	})
	Try(o, "连接数", func() error {
		o.ConnCur = QueryString(db, `SELECT value FROM system.metrics WHERE metric = 'TCPConnection'`, "-")
		return nil
	})
	Try(o, "容量", func() error {
		sz := QueryString(db, `SELECT formatReadableSize(sum(bytes_on_disk)) FROM system.parts WHERE active`, "-")
		o.DBSize = sz
		tc := QueryString(db, `SELECT count() FROM system.tables
			WHERE database NOT IN ('system','INFORMATION_SCHEMA','information_schema')`, "0")
		o.TableCount = tc
		return nil
	})
	Try(o, "集群", func() error {
		n := QueryString(db, "SELECT count(DISTINCT cluster) FROM system.clusters", "0")
		if parseI64(n) > 0 {
			o.Cluster = fmt.Sprintf("%s 个集群（system.clusters）", n)
			o.Role = "分布式"
		}
		return nil
	})

	o.ArchiveMode = "-" // CH 为 LSM/日志型存储，无 WAL 归档概念
	o.Charset = "UTF-8"
	o.Timezone = QueryString(db, "SELECT timeZone()", "-")
	o.ReadOnly = "-"
	if o.Role == "" {
		o.Role = "单机"
	}
	if o.Cluster == "" {
		o.Cluster = "单机"
	}
	return o, nil
}

func (f chProvider) Detail(_ context.Context, c *model.Conn) (*model.Detail, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	d := &model.Detail{}

	// 库列表（表数 + 大小）
	if _, rows, err := QueryTable(db, `SELECT database, count() FROM system.tables
		GROUP BY database ORDER BY database`); err == nil {
		tablesByDb := map[string]string{}
		for _, r := range rows {
			tablesByDb[r[0]] = r[1]
		}
		if _, rows, err := QueryTable(db, `SELECT database, formatReadableSize(sum(bytes_on_disk))
			FROM system.parts WHERE active GROUP BY database`); err == nil {
			for _, r := range rows {
				d.Dbs = append(d.Dbs, model.DbInfo{Name: r[0], Tables: tablesByDb[r[0]], Size: r[1]})
			}
		} else {
			for k, v := range tablesByDb {
				d.Dbs = append(d.Dbs, model.DbInfo{Name: k, Tables: v})
			}
		}
	}

	// 大表 Top 20
	if _, rows, err := QueryTable(db, `SELECT database, table, formatReadableSize(sum(bytes_on_disk)), formatReadableQuantity(sum(rows))
		FROM system.parts WHERE active
		GROUP BY database, table ORDER BY sum(bytes_on_disk) DESC LIMIT 20`); err == nil {
		for _, r := range rows {
			d.Tables = append(d.Tables, model.TableInfo{Schema: r[0], Name: r[1], Size: r[2], Rows: r[3]})
		}
	}

	// 磁盘（表空间对应物）
	if _, rows, err := QueryTable(db, `SELECT name, path, formatReadableSize(total_space), formatReadableSize(free_space)
		FROM system.disks ORDER BY name`); err == nil {
		for _, r := range rows {
			d.TBS = append(d.TBS, model.TBSInfo{Name: r[0], Files: r[1], Size: r[2], Free: r[3], Status: "-"})
		}
	}

	// 集群拓扑（shard/replica）
	if cols, rows, err := QueryTable(db, `SELECT cluster, shard_num, replica_num, host_name, port, errors_count
		FROM system.clusters ORDER BY cluster, shard_num, replica_num`); err == nil {
		for _, r := range rows {
			m := mapCols(cols, r)
			d.Services = append(d.Services, model.SvcInfo{
				Name:   m["host_name"] + ":" + m["port"],
				Role:   m["cluster"] + " 分片" + m["shard_num"] + " 副本" + m["replica_num"],
				Status: "运行中",
				Host:   m["host_name"],
				Detail: "错误数 " + m["errors_count"],
			})
		}
	}
	return d, nil
}

func (f chProvider) Topology(_ context.Context, c *model.Conn) (*model.Topology, error) {
	db, err := f.open(c)()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	root := newTNode("root", c.Name, "ClickHouse @ "+c.Host+":"+itoa(c.Port), "ok",
		model.KV{Key: "版本", Value: QueryString(db, "SELECT version()", "-")},
		model.KV{Key: "地址", Value: c.Host + ":" + itoa(c.Port)},
	)

	// 集群分支：cluster → shard → replica
	if cols, rows, err := QueryTable(db, `SELECT cluster, shard_num, replica_num, host_name, port, is_local
		FROM system.clusters ORDER BY cluster, shard_num, replica_num`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("clusters", "集群拓扑", "system.clusters", "na"))
		clusterNodes := map[string]*model.TNode{}
		shardNodes := map[string]*model.TNode{}
		for i, r := range rows {
			m := mapCols(cols, r)
			cl, ok := clusterNodes[m["cluster"]]
			if !ok {
				cl = addNode(branch, newTNode("cl-"+m["cluster"], "集群 "+m["cluster"], "", "ok"))
				clusterNodes[m["cluster"]] = cl
			}
			key := m["cluster"] + "-" + m["shard_num"]
			sh, ok := shardNodes[key]
			if !ok {
				sh = addNode(cl, newTNode("sh-"+key, "分片 "+m["shard_num"], "", "na"))
				shardNodes[key] = sh
			}
			st := "na"
			if m["is_local"] == "1" {
				st = "ok"
			}
			addNode(sh, newTNode(fmt.Sprintf("rp-%d", i), m["host_name"]+":"+m["port"],
				"副本 "+m["replica_num"], st))
		}
	}

	// 磁盘分支（数据文件落盘位置）
	if _, rows, err := QueryTable(db, `SELECT name, path,
		formatReadableSize(total_space), formatReadableSize(free_space)
		FROM system.disks ORDER BY name`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("tbs", "磁盘", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			addNode(branch, newTNode(fmt.Sprintf("tbs-%d", i), r[0],
				r[2]+" / 剩余 "+r[3], "na", model.KV{Key: "路径", Value: r[1]}))
		}
	}

	// 库 → 大表
	if _, rows, err := QueryTable(db, `SELECT database, count() FROM system.tables
		WHERE database NOT IN ('system','INFORMATION_SCHEMA','information_schema')
		GROUP BY database ORDER BY database`); err == nil && len(rows) > 0 {
		branch := addNode(root, newTNode("dbs", "数据库", fmt.Sprintf("%d 个", len(rows)), "na"))
		for i, r := range rows {
			n := addNode(branch, newTNode(fmt.Sprintf("db-%d", i), r[0], r[1]+" 表", "ok"))
			if _, trows, err := QueryTable(db, fmt.Sprintf(`SELECT table, formatReadableSize(sum(bytes_on_disk))
				FROM system.parts WHERE active AND database = '%s'
				GROUP BY table ORDER BY sum(bytes_on_disk) DESC LIMIT 10`, sqlEscape(r[0]))); err == nil {
				for j, tr := range trows {
					addNode(n, newTNode(fmt.Sprintf("db-%d-t-%d", i, j), tr[0], tr[1], "na"))
				}
			}
		}
	}
	return &model.Topology{ConnID: c.ID, Root: root}, nil
}
