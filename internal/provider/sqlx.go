package provider

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"dbping/internal/model"

	_ "gitee.com/chunanyong/dm"                         // 驱动名: dm
	_ "gitcode.com/opengauss/openGauss-connector-go-pq" // 驱动名: opengauss
	_ "github.com/ClickHouse/clickhouse-go/v2"          // 驱动名: clickhouse
	_ "github.com/SAP/go-hdb/driver"                    // 驱动名: hdb
	_ "github.com/jackc/pgx/v5/stdlib"                  // 驱动名: pgx
	_ "github.com/lib/pq"                               // 驱动名: postgres
	_ "github.com/microsoft/go-mssqldb"                 // 驱动名: sqlserver
	_ "github.com/sijms/go-ora/v2"                      // 驱动名: oracle（纯 Go TNS）
	_ "github.com/thda/tds"                             // 驱动名: tds/syb（Sybase ASE，FreeTDS 协议纯 Go 实现）
	_ "modernc.org/sqlite"                              // 驱动名: sqlite（纯 Go）
	_ "github.com/go-db2/go-db2"                        // 驱动名: db2（纯 Go DRDA，有 live server 测试）

	"github.com/go-sql-driver/mysql"
)

// OpenSQL 打开一个 database/sql 连接（小连接池，巡检专用）。
func OpenSQL(c *model.Conn, driver, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute)
	return db, nil
}

// PingGeneric 三级探活通用流程：TCP → 认证 → SQL。
// open 返回已打开的 *sql.DB；probe 为探活 SQL。
func PingGeneric(c *model.Conn, open func() (*sql.DB, error), probe string) (*model.PingResult, error) {
	res := &model.PingResult{ConnID: c.ID, Stage: "tcp", CheckedAt: time.Now().UnixMilli()}
	start := time.Now()

	tcpMs, err := DialTCP(c)
	if err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.TCPMs = tcpMs

	res.Stage = "auth"
	db, err := open()
	if err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	defer db.Close()

	ctx, cancel := DBCtx(c)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}

	res.Stage = "sql"
	s := time.Now()
	var one int
	if err := db.QueryRowContext(ctx, probe).Scan(&one); err != nil {
		res.ErrorKind, res.Error = ClassifyError(err)
		return res, nil
	}
	res.SQLMs = time.Since(s).Milliseconds()
	if res.SQLMs <= 0 {
		res.SQLMs = 1
	}
	res.Stage = "ok"
	res.OK = true
	res.TotalMs = time.Since(start).Milliseconds()
	return res, nil
}

// ---------- DSN 构造 ----------

// mysqlTLSRegistered 记录已注册的 TLS 配置名（RegisterTLSConfig 重复注册会 panic）。
var mysqlTLSRegistered sync.Map

// applyMySQLTLS 处理 DBPing 证书约定参数（sslCert/sslKey/sslCA/sslServerName/sslSkipVerify）：
// 存在客户端证书时构建 tls.Config 并以稳定名称注册，返回应写入 cfg.TLSConfig 的名称与剔除约定参数后的剩余参数。
func applyMySQLTLS(p map[string]string) (tlsName string, rest map[string]string) {
	rest = map[string]string{}
	cert, key, ca := p["sslCert"], p["sslKey"], p["sslCA"]
	skip := strings.EqualFold(p["sslSkipVerify"], "true")
	sn := p["sslServerName"]
	for _, k := range []string{"sslCert", "sslKey", "sslCA", "sslServerName", "sslSkipVerify"} {
		delete(p, k)
	}
	if cert == "" || key == "" {
		for k, v := range p {
			rest[k] = v
		}
		return "", p
	}
	cfg := &tls.Config{}
	if cer, err := tls.LoadX509KeyPair(cert, key); err == nil {
		cfg.Certificates = []tls.Certificate{cer}
	} else {
		// 证书加载失败则回退为不启用客户端证书，连接错误会由探活阶段给出
		for k, v := range p {
			rest[k] = v
		}
		return "", p
	}
	if ca != "" {
		if pem, err := os.ReadFile(ca); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(pem) {
				cfg.RootCAs = pool
			}
		}
	}
	if skip {
		cfg.InsecureSkipVerify = true
	}
	if sn != "" {
		cfg.ServerName = sn
	}
	name := fmt.Sprintf("dbping-%x", sha256.Sum256([]byte(cert+"|"+key+"|"+ca+"|"+sn+fmt.Sprint(skip))))
	if _, loaded := mysqlTLSRegistered.LoadOrStore(name, true); !loaded {
		_ = mysql.RegisterTLSConfig(name, cfg)
	}
	return name, p
}

// MySQLDSN go-sql-driver/mysql 的 DSN（用 Config 结构避免特殊字符转义问题）。
func MySQLDSN(c *model.Conn) string {
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = netAddr(c)
	cfg.DBName = c.Database
	sec := timeoutDur(c)
	cfg.Timeout = sec
	cfg.ReadTimeout = sec * 3
	cfg.WriteTimeout = sec
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["charset"] = "utf8mb4"
	p := parseParams(c.Params)
	tlsName, p := applyMySQLTLS(p)
	if tlsName != "" {
		cfg.TLSConfig = tlsName
	}
	for k, v := range p {
		if k == "charset" || k == "timeout" || k == "readTimeout" || k == "writeTimeout" {
			continue
		}
		cfg.Params[k] = v
	}
	return cfg.FormatDSN()
}

func netAddr(c *model.Conn) string {
	if c.Port <= 0 {
		return c.Host
	}
	return c.Host + ":" + strconv.Itoa(c.Port)
}

// parseParams 将 "a=1&b=2" 解析为 map。
func parseParams(s string) map[string]string {
	out := map[string]string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	if strings.HasPrefix(s, "?") {
		s = s[1:]
	}
	for _, kv := range strings.Split(s, "&") {
		if kv == "" {
			continue
		}
		p := strings.SplitN(kv, "=", 2)
		if len(p) == 2 {
			out[p[0]] = p[1]
		} else {
			out[p[0]] = ""
		}
	}
	return out
}

// PGDSN PostgreSQL 族 DSN。driver: pgx / postgres(lib/pq) / opengauss。
func PGDSN(c *model.Conn, driver string) string {
	db := c.Database
	if db == "" {
		db = "postgres"
	}
	if driver == "opengauss" {
		// openGauss connector（lib/pq 系）：key=value 形式
		parts := []string{
			"host=" + c.Host,
			"port=" + strconv.Itoa(c.Port),
			"user=" + pqEscape(c.User),
			"password=" + pqEscape(c.Password),
			"dbname=" + pqEscape(db),
			"sslmode=disable",
			"connect_timeout=" + strconv.Itoa(timeoutSecs(c)),
		}
		for k, v := range parseParams(c.Params) {
			if k == "host" || k == "port" || k == "user" || k == "password" || k == "dbname" {
				continue
			}
			parts = append(parts, k+"="+pqEscape(v))
		}
		return strings.Join(parts, " ")
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   netAddr(c),
		Path:   "/" + db,
	}
	q := url.Values{}
	q.Set("sslmode", "disable")
	q.Set("connect_timeout", strconv.Itoa(timeoutSecs(c)))
	q.Set("application_name", "dbping")
	for k, v := range parseParams(c.Params) {
		if k == "sslmode" && v == "" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// pqEscape lib/pq 风格 DSN 值转义。
func pqEscape(v string) string {
	if v == "" {
		return "''"
	}
	if strings.ContainsAny(v, " '\\") {
		return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(v) + "'"
	}
	return v
}

func timeoutSecs(c *model.Conn) int {
	if c.Timeout <= 0 {
		return 5
	}
	return c.Timeout
}

// MSSQLDSN go-mssqldb 的 URL DSN。
func MSSQLDSN(c *model.Conn) string {
	u := &url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(c.User, c.Password),
		Host:   netAddr(c),
	}
	q := url.Values{}
	if c.Database != "" {
		q.Set("database", c.Database)
	}
	q.Set("connection timeout", strconv.Itoa(timeoutSecs(c)))
	q.Set("dial timeout", strconv.Itoa(timeoutSecs(c)))
	for k, v := range parseParams(c.Params) {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// OracleDSN go-ora 的 DSN：oracle://user:pass@host:port/service。
func OracleDSN(c *model.Conn) string {
	u := &url.URL{
		Scheme: "oracle",
		User:   url.UserPassword(c.User, c.Password),
		Host:   netAddr(c),
	}
	if c.Database != "" {
		u.Path = "/" + c.Database
	}
	q := url.Values{}
	for k, v := range parseParams(c.Params) {
		q.Set(k, v)
	}
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// DmDSN 达梦 DSN：dm://user:pass@host:port?schema=xxx。
func DmDSN(c *model.Conn) string {
	u := &url.URL{
		Scheme: "dm",
		User:   url.UserPassword(c.User, c.Password),
		Host:   netAddr(c),
	}
	q := url.Values{}
	if c.Database != "" {
		q.Set("schema", c.Database)
	}
	for k, v := range parseParams(c.Params) {
		if k == "schema" && v == "" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// HanaDSN go-hdb DSN：hdb://user:pass@host:port。
func HanaDSN(c *model.Conn) string {
	u := &url.URL{
		Scheme: "hdb",
		User:   url.UserPassword(c.User, c.Password),
		Host:   netAddr(c),
	}
	q := url.Values{}
	for k, v := range parseParams(c.Params) {
		q.Set(k, v)
	}
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// CHDSN ClickHouse DSN。
func CHDSN(c *model.Conn) string {	u := &url.URL{
		Scheme: "clickhouse",
		User:   url.UserPassword(c.User, c.Password),
		Host:   netAddr(c),
	}
	if c.Database != "" {
		u.Path = "/" + c.Database
	}
	q := url.Values{}
	q.Set("dial_timeout", fmt.Sprintf("%ds", timeoutSecs(c)))
	q.Set("ping_timeout", fmt.Sprintf("%ds", timeoutSecs(c)))
	for k, v := range parseParams(c.Params) {
		if k == "dial_timeout" || k == "ping_timeout" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ---------- 拓扑节点构造 ----------

func newTNode(id, label, sub, status string, kvs ...model.KV) *model.TNode {
	if status == "" {
		status = "na"
	}
	return &model.TNode{ID: id, Label: label, Sub: sub, Status: status, KVs: kvs}
}

func addNode(parent *model.TNode, n *model.TNode) *model.TNode {
	parent.Children = append(parent.Children, n)
	return n
}

// humanUptimeSecs 由启动时间字符串（Go 解析失败则返回空）计算已运行时长。
func humanUptimeSecs(started string, now time.Time) string {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(started), time.Local)
	if err != nil {
		return ""
	}
	sec := int64(now.Sub(t).Seconds())
	if sec < 0 {
		return "-"
	}
	return HumanDuration(sec)
}

// ctxOf 提供家族实现使用的短 ctx。
func ctxOf(c *model.Conn) (context.Context, context.CancelFunc) { return DBCtx(c) }
