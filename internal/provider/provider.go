// Package provider 实现各数据库的探活与元数据采集。
// 统一约束：全部使用纯 Go 驱动（零 cgo），全部为只读 SQL。
package provider

import (
	"context"
	"fmt"
	"sync"

	"dbping/internal/model"
)

// Provider 每类数据库的采集实现。
type Provider interface {
	Ping(ctx context.Context, c *model.Conn) (*model.PingResult, error)
	Overview(ctx context.Context, c *model.Conn) (*model.Overview, error)
	Detail(ctx context.Context, c *model.Conn) (*model.Detail, error)
	Topology(ctx context.Context, c *model.Conn) (*model.Topology, error)
}

var (
	regMu    sync.RWMutex
	families = map[string]Provider{} // family -> provider
)

// Register 注册一个 family 实现。
func Register(family string, p Provider) {
	regMu.Lock()
	defer regMu.Unlock()
	families[family] = p
}

// Get 按 family 取实现。
func Get(family string) (Provider, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	p, ok := families[family]
	if !ok {
		return nil, fmt.Errorf("未注册的数据库类型族: %s", family)
	}
	return p, nil
}

// Families 返回已注册的 family 名单。
func Families() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(families))
	for k := range families {
		out = append(out, k)
	}
	return out
}

// TypeInfos 支持的数据库类型清单（一期尽量全量覆盖，均为纯 Go 驱动）。
var TypeInfos = []model.TypeInfo{
	// ---- MySQL 协议族 ----
	{Type: "mysql", Name: "MySQL", Family: "mysql", Port: 3306, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "🐬", Vendor: "Oracle", Cluster: false},
	{Type: "mariadb", Name: "MariaDB", Family: "mysql", Port: 3306, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "🦭", Vendor: "MariaDB Foundation"},
	{Type: "tidb", Name: "TiDB", Family: "mysql", Port: 4000, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "⚡", Vendor: "PingCAP", Cluster: true},
	{Type: "oceanbase", Name: "OceanBase (MySQL租户)", Family: "mysql", Port: 2881, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "🌊", Vendor: "蚂蚁集团", Cluster: true},
	{Type: "doris", Name: "Apache Doris", Family: "mysql", Port: 9030, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "🔶", Vendor: "Apache", Cluster: true},
	{Type: "starrocks", Name: "StarRocks", Family: "mysql", Port: 9030, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "⭐", Vendor: "StarRocks Inc.", Cluster: true},
	{Type: "gbase8a", Name: "GBase 8a", Family: "mysql", Port: 5258, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "🧮", Vendor: "南大通用", Note: "MySQL 协议兼容"},
	{Type: "polardb", Name: "PolarDB / PolarDB-X", Family: "mysql", Port: 3306, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "☁️", Vendor: "阿里云", Cluster: true},
	{Type: "tdsql", Name: "TDSQL", Family: "mysql", Port: 3306, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "🔗", Vendor: "腾讯云", Cluster: true},
	{Type: "goldendb", Name: "GoldenDB", Family: "mysql", Port: 3306, DBLabel: "数据库（可空）", UserLabel: "用户名", Icon: "🟡", Vendor: "中兴", Cluster: true},

	// ---- PostgreSQL 协议族 ----
	{Type: "pg", Name: "PostgreSQL", Family: "pg", Port: 5432, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🐘", Vendor: "PostgreSQL Global Dev. Group"},
	{Type: "kingbase", Name: "人大金仓 KingbaseES", Family: "pg", Port: 54321, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "👑", Vendor: "人大金仓", Note: "PG 协议兼容，使用 lib/pq"},
	{Type: "opengauss", Name: "openGauss / GaussDB", Family: "pg", Port: 5432, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🔵", Vendor: "华为", Cluster: true, Note: "官方驱动，支持 sha256 认证"},
	{Type: "mogdb", Name: "MogDB", Family: "pg", Port: 5432, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🟢", Vendor: "云和恩墨", Cluster: true},
	{Type: "greenplum", Name: "Greenplum", Family: "pg", Port: 5432, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🟩", Vendor: "VMware/Broadcom", Cluster: true},
	{Type: "cockroach", Name: "CockroachDB", Family: "pg", Port: 26257, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🪳", Vendor: "Cockroach Labs", Cluster: true},
	{Type: "yugabyte", Name: "YugabyteDB", Family: "pg", Port: 5432, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🔺", Vendor: "Yugabyte", Cluster: true},
	{Type: "gbase8c", Name: "GBase 8c", Family: "pg", Port: 5432, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🧾", Vendor: "南大通用", Note: "PG 协议兼容"},
	{Type: "vastbase", Name: "Vastbase G100", Family: "pg", Port: 5432, DBLabel: "数据库（必填）", UserLabel: "用户名", Icon: "🟥", Vendor: "海量数据", Note: "PG 协议兼容"},

	// ---- 独立协议 ----
	{Type: "mssql", Name: "SQL Server", Family: "mssql", Port: 1433, DBLabel: "数据库（默认 master）", UserLabel: "用户名", Icon: "🟦", Vendor: "Microsoft"},
	{Type: "db2", Name: "IBM Db2", Family: "db2", Port: 50000, DBLabel: "数据库名", UserLabel: "用户名", Icon: "🛢", Vendor: "IBM", Note: "纯 Go DRDA 实现（免 cgo/CLI）"},
	{Type: "oracle", Name: "Oracle", Family: "oracle", Port: 1521, DBLabel: "服务名/SID", UserLabel: "用户名（如 system）", Icon: "🅾️", Vendor: "Oracle", Cluster: false, Note: "纯 Go TNS 实现，免 Instant Client"},
	{Type: "oceanbase-oracle", Name: "OceanBase (Oracle租户)", Family: "oracle", Port: 2881, DBLabel: "服务名/租户", UserLabel: "用户名", Icon: "🌊", Vendor: "蚂蚁集团", Note: "实验性：经 ODP 以 Oracle 协议连接"},
	{Type: "dm", Name: "达梦 DM8", Family: "dm", Port: 5236, DBLabel: "模式（可空，默认用户同名模式）", UserLabel: "用户名（如 SYSDBA）", Icon: "🐲", Vendor: "武汉达梦"},
	{Type: "hana", Name: "SAP HANA", Family: "hana", Port: 30015, DBLabel: "租户库（MDC 时必填）", UserLabel: "用户名", Icon: "💠", Vendor: "SAP", Cluster: true},
	{Type: "clickhouse", Name: "ClickHouse", Family: "clickhouse", Port: 9000, DBLabel: "数据库（默认 default）", UserLabel: "用户名（默认 default）", Icon: "🧱", Vendor: "ClickHouse Inc.", Cluster: true},
	{Type: "sybase", Name: "Sybase ASE", Family: "sybase", Port: 5000, DBLabel: "数据库（默认 master）", UserLabel: "用户名", Icon: "🔷", Vendor: "SAP (Sybase)", Note: "FreeTDS/TDS 协议纯 Go 驱动"},

	// ---- 本地文件 / NoSQL ----
	{Type: "sqlite", Name: "SQLite（本地文件）", Family: "sqlite", Port: 0, DBLabel: "数据库文件路径", UserLabel: "", Icon: "🗂", Vendor: "SQLite Consortium", NoSQL: false, Note: "支持打开/创建本地 .db/.sqlite 文件"},
	{Type: "redis", Name: "Redis", Family: "redis", Port: 6379, DBLabel: "DB 编号（默认 0）", UserLabel: "", Icon: "🟥", Vendor: "Redis Ltd.", NoSQL: true, Cluster: true},
	{Type: "mongo", Name: "MongoDB", Family: "mongo", Port: 27017, DBLabel: "认证库 authSource（默认 admin）", UserLabel: "用户名", Icon: "🍃", Vendor: "MongoDB Inc.", NoSQL: true, Cluster: true},
}

// TypeInfo 按类型标识查找。
func TypeInfo(typ string) (model.TypeInfo, error) {
	for _, t := range TypeInfos {
		if t.Type == typ {
			return t, nil
		}
	}
	return model.TypeInfo{}, fmt.Errorf("不支持的数据库类型: %s", typ)
}
