package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"dbping/internal/model"
)

// contextTimeout 为连接构造带超时的 ctx；未设置时默认 5 秒。
func contextTimeout(c *model.Conn) (context.Context, context.CancelFunc) {
	sec := c.Timeout
	if sec <= 0 {
		sec = 5
	}
	return context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
}

// DBCtx 为已建立的连接返回用于查询的 ctx（超时稍长，默认 10 秒）。
func DBCtx(c *model.Conn) (context.Context, context.CancelFunc) {
	sec := c.Timeout * 2
	if sec <= 0 {
		sec = 10
	}
	if sec > 60 {
		sec = 60
	}
	return context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
}

// DialTCP 三级探活第一级：TCP 端口探测，返回耗时毫秒。
func DialTCP(c *model.Conn) (int64, error) {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeoutDur(c))
	if err != nil {
		return 0, err
	}
	ms := time.Since(start).Milliseconds()
	conn.Close()
	if ms <= 0 {
		ms = 1
	}
	return ms, nil
}

func timeoutDur(c *model.Conn) time.Duration {
	sec := c.Timeout
	if sec <= 0 {
		sec = 5
	}
	return time.Duration(sec) * time.Second
}

// ClassifyError 将底层错误归类为中文提示 + 错误类别。
func ClassifyError(err error) (kind, msg string) {
	if err == nil {
		return "", ""
	}
	s := strings.ToLower(err.Error())
	switch {
	case containsAny(s, "access denied", "ora-01017", "password authentication failed", "login failed for user",
		"authentication failed", "28000", "auth", "鉴权", "password", "wrong password", "noauth", "bad credentials"):
		return "auth", "[认证失败] 用户名或密码错误，或未授予登录权限: " + err.Error()
	case containsAny(s, "ora-00942", "ora-01031", "insufficient privileges", "permission denied", "42501",
		"privilege", "not authorized", "authorization", "denied to user", "denied"):
		return "permission", "[权限不足] 账号缺少系统视图/元数据查询权限: " + err.Error()
	case containsAny(s, "forcibly closed", "handshake", "invalid packet", "unexpected packet", "bad packet", "wsarecv", "broken pipe", "malformed"):
		// TCP 已建立但读握手包阶段被断开：端口上大概率不是目标数据库服务
		return "protocol", "[协议不匹配] TCP 已连通但握手失败（该端口上可能不是此数据库服务，请核对端口）: " + err.Error()
	case containsAny(s, "connection refused", "no such host", "lookup", "i/o timeout", "dial tcp", "network",
		"unreachable", "connectex", "actively refused", "getaddrinfo", "eof", "reset"):
		return "network", "[网络不通] 无法建立 TCP 连接（地址/端口/防火墙）: " + err.Error()
	case containsAny(s, "timeout", "deadline exceeded", "canceled"):
		return "timeout", "[超时] 连接或执行超时，请检查网络或调大超时: " + err.Error()
	case containsAny(s, "unknown database", "does not exist", "unknowndatabase", "invalid username/password",
		"ora-12514", "ora-12505", "ora-12154", "unknown schema"):
		return "config", "[配置错误] 数据库/服务名不存在: " + err.Error()
	case containsAny(s, "unsupported", "protocol", "driver: bad connection"):
		return "protocol", "[协议不兼容] 服务端协议与驱动不匹配（版本差异）: " + err.Error()
	default:
		return "unknown", "[失败] " + err.Error()
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// QueryString 执行返回 1 行 1 列的查询，空结果返回 def。
func QueryString(db *sql.DB, q string, def string) string {
	var s sql.NullString
	if err := db.QueryRow(q).Scan(&s); err != nil || !s.Valid {
		return def
	}
	if strings.TrimSpace(s.String) == "" {
		return def
	}
	return s.String
}

// QueryKV 执行返回 1 行多列的查询，列名->值。
func QueryKV(db *sql.DB, q string) ([]model.KV, bool) {
	rows, err := db.Query(q)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, false
	}
	if !rows.Next() {
		return nil, false
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, false
	}
	out := make([]model.KV, 0, len(cols))
	for i, c := range cols {
		out = append(out, model.KV{Key: c, Value: cellString(vals[i])})
	}
	return out, true
}

// QueryTable 通用取表：返回列名 + 全部行的字符串化结果。
func QueryTable(db *sql.DB, q string) (cols []string, rows [][]string, err error) {
	rs, err := db.Query(q)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	cols, err = rs.Columns()
	if err != nil {
		return nil, nil, err
	}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return cols, rows, err
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			row[i] = cellString(v)
		}
		rows = append(rows, row)
	}
	return cols, rows, rs.Err()
}

func cellString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// HumanBytes 字节数转可读大小。
func HumanBytes(b int64) string {
	switch {
	case b >= 1<<40:
		return fmt.Sprintf("%.2f TB", float64(b)/(1<<40))
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// HumanDuration 秒数转已运行时长。
func HumanDuration(sec int64) string {
	if sec <= 0 {
		return "-"
	}
	d := time.Duration(sec) * time.Second
	days := int64(d.Hours()) / 24
	hours := int64(d.Hours()) % 24
	mins := int64(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分", hours, mins)
	}
	return fmt.Sprintf("%d分钟", mins)
}

// Try 捕获采集子步骤错误：失败时把原因登记为 warning 而非整体失败。
func Try(o *model.Overview, view string, fn func() error) {
	if err := fn(); err != nil {
		o.Warnings = append(o.Warnings, view+" 采集失败: "+shortErr(err))
	}
}

func shortErr(err error) string {
	s := err.Error()
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// Ignore 在某些驱动不支持指定视图时静默跳过。
func Ignore(err error) bool { return err != nil }

var ErrSkipped = errors.New("skipped")
