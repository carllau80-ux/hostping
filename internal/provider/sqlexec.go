package provider

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dbping/internal/model"
)

// SQLiteDSN modernc/sqlite 的 DSN 即文件路径（可带查询参数）。
func SQLiteDSN(c *model.Conn) string {
	path := strings.TrimSpace(c.Host)
	if path == "" {
		path = c.Database
	}
	if strings.HasPrefix(strings.ToLower(path), "file:") {
		return path
	}
	q := url.Values{}
	for k, v := range parseParams(c.Params) {
		q.Set(k, v)
	}
	if len(q) > 0 {
		return path + "?" + q.Encode()
	}
	return path
}

// SybaseDSN thda/tds 的 URL DSN：tds://user:pass@host:port/db?charset=utf8
func SybaseDSN(c *model.Conn) string {
	u := &url.URL{
		Scheme: "tds",
		User:   url.UserPassword(c.User, c.Password),
		Host:   netAddr(c),
	}
	if c.Database != "" {
		u.Path = "/" + c.Database
	}
	q := url.Values{}
	q.Set("charset", "utf8")
	q.Set("loginTimeout", strconv.Itoa(timeoutSecs(c)))
	q.Set("readTimeout", strconv.Itoa(timeoutSecs(c)*3))
	for k, v := range parseParams(c.Params) {
		if k == "charset" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// DSNFor 返回连接对应的 (driver, dsn)；redis/mongo 不支持 SQL 执行。
func DSNFor(c *model.Conn) (string, string, error) {
	ti, err := TypeInfo(c.Type)
	if err != nil {
		return "", "", err
	}
	switch ti.Family {
	case "mysql":
		return "mysql", MySQLDSN(c), nil
	case "pg":
		drv := pgDriver(c.Type)
		return drv, PGDSN(c, drv), nil
	case "mssql":
		return "sqlserver", MSSQLDSN(c), nil
	case "oracle":
		return "oracle", OracleDSN(c), nil
	case "dm":
		return "dm", DmDSN(c), nil
	case "hana":
		return "hdb", HanaDSN(c), nil
	case "clickhouse":
		return "clickhouse", CHDSN(c), nil
	case "sqlite":
		return "sqlite", SQLiteDSN(c), nil
	case "sybase":
		return "tds", SybaseDSN(c), nil
	case "db2":
		return "db2", Db2DSN(c), nil
	default:
		return "", "", fmt.Errorf("该数据库类型（%s）暂不支持 SQL 控制台", ti.Name)
	}
}

// ExecSQL 高级模式 SQL 执行：SELECT 自动 COUNT + 分页封装；非查询返回影响行数。
func ExecSQL(c *model.Conn, sqlText string, page, size int) (*model.SQLResult, error) {
	driver, dsn, err := DSNFor(c)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 1000 {
		size = 200
	}
	db, err := OpenSQL(c, driver, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx, cancel := DBCtx(c)
	defer cancel()

	start := time.Now()
	trimmed := strings.TrimSpace(sqlText)
	trimmed = strings.TrimSuffix(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil, fmt.Errorf("SQL 语句为空")
	}
	upper := strings.ToUpper(trimmed)
	res := &model.SQLResult{Page: page, PageSize: size}

	switch {
	case strings.HasPrefix(upper, "SELECT"):
		// 分页：COUNT 总数 + 方言化分页封装
		ti, _ := TypeInfo(c.Type)
		total := int64(-1)
		if cntSQL := countSQL(c.Type, trimmed); cntSQL != "" {
			if v := QueryString(db, cntSQL, ""); v != "" {
				total = parseI64(v)
			}
		}
		q := wrapPaged(c.Type, trimmed, (page-1)*size, size)
		cols, rows, err2 := QueryTable(db, q)
		if err2 != nil {
			return nil, err2
		}
		res.IsQuery = true
		res.Columns = cols
		res.Rows = rows
		res.Total = total
		_ = ti
	default:
		// SHOW / PRAGMA / EXPLAIN / WITH 等按查询原样执行（行数封顶 500）
		if strings.HasPrefix(upper, "SHOW") || strings.HasPrefix(upper, "PRAGMA") ||
			strings.HasPrefix(upper, "EXPLAIN") || strings.HasPrefix(upper, "WITH") ||
			strings.HasPrefix(upper, "EXEC") || strings.HasPrefix(upper, "SP_") ||
			strings.HasPrefix(upper, "DESC") {
			cols, rows, err2 := QueryTable(db, trimmed)
			if err2 != nil {
				return nil, err2
			}
			if len(rows) > 500 {
				rows = rows[:500]
				res.Truncated = true
			}
			res.IsQuery = true
			res.Columns = cols
			res.Rows = rows
			res.Total = int64(len(rows))
			res.ElapsedMs = time.Since(start).Milliseconds()
			return res, nil
		}
		// DML / DDL
		r, err2 := db.ExecContext(ctx, trimmed)
		if err2 != nil {
			return nil, err2
		}
		aff, _ := r.RowsAffected()
		res.Affected = aff
	}
	res.ElapsedMs = time.Since(start).Milliseconds()
	return res, nil
}

// countSQL 派生表计数（不支持的方言返回空串）。
func countSQL(typ, sqlText string) string {
	switch TypeInfoFamily(typ) {
	case "sybase":
		return "" // ASE 派生表分页支持有限，不取总数
	default:
		return "SELECT COUNT(*) FROM (" + sqlText + ") t_"
	}
}

// wrapPaged 按方言把 SELECT 包装为分页查询。
func wrapPaged(typ, sqlText string, offset, size int) string {
	switch TypeInfoFamily(typ) {
	case "mssql":
		return fmt.Sprintf("SELECT * FROM (%s) t_ ORDER BY (SELECT NULL) OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", sqlText, offset, size)
	case "oracle":
		return fmt.Sprintf("SELECT * FROM (SELECT t_.*, ROWNUM rn_ FROM (%s) t_ WHERE ROWNUM <= %d) WHERE rn_ > %d", sqlText, offset+size, offset)
	case "sybase":
		// ASE 不支持派生表/ OFFSET：注入 TOP 限制行数（仅第一页有数据，翻页由前端提示）
		upper := strings.ToUpper(strings.TrimSpace(sqlText))
		if strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper[6:], " TOP") {
			return "SELECT TOP " + itoaRaw(size) + " " + sqlText[len("SELECT"):]
		}
		return sqlText
	default: // mysql / pg系 / dm / hana / clickhouse / sqlite 均支持 LIMIT..OFFSET
		return fmt.Sprintf("SELECT * FROM (%s) t_ LIMIT %d OFFSET %d", sqlText, size, offset)
	}
}

func itoaRaw(n int) string { return fmt.Sprintf("%d", n) }

// TypeInfoFamily 轻量 family 查询（避免循环依赖 monitor）。
func TypeInfoFamily(typ string) string {
	if ti, err := TypeInfo(typ); err == nil {
		return ti.Family
	}
	return ""
}

// ListSchemas 数据库/模式列表（供数据表页签按库筛选）。
func ListSchemas(c *model.Conn) ([]string, error) {
	driver, dsn, err := DSNFor(c)
	if err != nil {
		return nil, err
	}
	db, err := OpenSQL(c, driver, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	family := TypeInfoFamily(c.Type)
	var q string
	switch family {
	case "mysql":
		q = `SELECT schema_name FROM information_schema.schemata
			WHERE schema_name NOT IN ('mysql','information_schema','performance_schema','sys','oceanbase')
			ORDER BY schema_name`
	case "pg":
		q = `SELECT nspname FROM pg_namespace
			WHERE nspname NOT IN ('pg_catalog','information_schema','pg_toast','sys','sysaudit','sys_catalog','crdb_internal')
			ORDER BY nspname`
	case "mssql":
		q = "SELECT name FROM sys.schemas ORDER BY name"
	case "oracle", "dm":
		q = "SELECT username FROM dba_users ORDER BY username"
	case "hana":
		q = "SELECT SCHEMA_NAME FROM SCHEMAS ORDER BY SCHEMA_NAME"
	case "clickhouse":
		q = `SELECT database FROM system.databases
			WHERE database NOT IN ('system','INFORMATION_SCHEMA','information_schema') ORDER BY database`
	case "db2":
		q = "SELECT SCHEMANAME FROM SYSCAT.SCHEMAS ORDER BY SCHEMANAME"
	case "sybase":
		q = "SELECT name FROM master..sysdatabases ORDER BY name"
	case "sqlite":
		return []string{"main"}, nil
	default:
		return nil, fmt.Errorf("该数据库类型暂不支持模式列表")
	}
	_, rows, err := QueryTable(db, q)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r[0])
	}
	return out, nil
}

// ListTablesPaged 数据表元数据分页（可按数据库/模式 + 表名关键字筛选）。
func ListTablesPaged(c *model.Conn, schema, filter string, page, size int) (*model.TablePage, error) {
	driver, dsn, err := DSNFor(c)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 500 {
		size = 50
	}
	db, err := OpenSQL(c, driver, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tp := &model.TablePage{Page: page, Size: size, Schema: schema}

	family := TypeInfoFamily(c.Type)
	var cntSQL, pageSQL string
	offset := (page - 1) * size
	sc := sqlEscape(schema)
	kw := sqlEscape(strings.TrimSpace(filter))
	like := func(col string) string {
		if kw == "" {
			return ""
		}
		return " AND " + fmt.Sprintf(col, "'%"+kw+"%'")
	}
	switch family {
	case "mysql":
		cntSQL = "SELECT COUNT(*) FROM information_schema.TABLES WHERE table_schema = '" + sc + "'" + like("table_name LIKE %s")
		pageSQL = fmt.Sprintf(`SELECT table_name, IFNULL(table_rows,'-'),
			CONCAT(IFNULL(ROUND((data_length+index_length)/1024,1),0),' KB'), IFNULL(table_comment,'')
			FROM information_schema.TABLES WHERE table_schema = '%s'%s
			ORDER BY table_name LIMIT %d OFFSET %d`, sc, like("table_name LIKE %s"), size, offset)
	case "pg":
		cntSQL = "SELECT COUNT(*) FROM pg_tables WHERE schemaname = '" + sc + "'" + like("tablename LIKE %s")
		pageSQL = fmt.Sprintf(`SELECT tablename,
			pg_size_pretty(pg_total_relation_size('"' || schemaname || '"."' || tablename || '"'))
			FROM pg_tables WHERE schemaname = '%s'%s
			ORDER BY tablename LIMIT %d OFFSET %d`, sc, like("tablename LIKE %s"), size, offset)
	case "mssql":
		cntSQL = "SELECT COUNT(*) FROM sys.tables t JOIN sys.schemas s ON s.schema_id = t.schema_id WHERE s.name = '" + sc + "'" + like("t.name LIKE %s")
		pageSQL = fmt.Sprintf(`SELECT t.name FROM sys.tables t JOIN sys.schemas s ON s.schema_id = t.schema_id
			WHERE s.name = '%s'%s ORDER BY t.name
			OFFSET %d ROWS FETCH NEXT %d ROWS ONLY`, sc, like("t.name LIKE %s"), offset, size)
	case "oracle":
		cntSQL = "SELECT COUNT(*) FROM dba_tables WHERE owner = '" + sc + "'" + like("UPPER(table_name) LIKE UPPER(%s)")
		pageSQL = fmt.Sprintf(`SELECT * FROM (SELECT table_name FROM dba_tables
			WHERE owner = '%s'%s ORDER BY table_name) WHERE ROWNUM <= %d`, sc, like("UPPER(table_name) LIKE UPPER(%s)"), offset+size)
	case "dm":
		cntSQL = "SELECT COUNT(*) FROM DBA_TABLES WHERE OWNER = '" + sc + "'" + like("UPPER(TABLE_NAME) LIKE UPPER(%s)")
		pageSQL = fmt.Sprintf(`SELECT * FROM (SELECT TABLE_NAME FROM DBA_TABLES
			WHERE OWNER = '%s'%s ORDER BY TABLE_NAME) WHERE ROWNUM <= %d`, sc, like("UPPER(TABLE_NAME) LIKE UPPER(%s)"), offset+size)
	case "hana":
		cntSQL = "SELECT COUNT(*) FROM M_TABLES WHERE SCHEMA_NAME = '" + sc + "'" + like("UPPER(TABLE_NAME) LIKE UPPER(%s)")
		pageSQL = fmt.Sprintf(`SELECT TABLE_NAME, RECORD_COUNT FROM M_TABLES
			WHERE SCHEMA_NAME = '%s'%s ORDER BY TABLE_NAME LIMIT %d OFFSET %d`, sc, like("UPPER(TABLE_NAME) LIKE UPPER(%s)"), size, offset)
	case "clickhouse":
		cntSQL = "SELECT COUNT(*) FROM system.tables WHERE database = '" + sc + "'" + like("name LIKE %s")
		pageSQL = fmt.Sprintf(`SELECT name FROM system.tables WHERE database = '%s'%s
			ORDER BY name LIMIT %d OFFSET %d`, sc, like("name LIKE %s"), size, offset)
	case "db2":
		cntSQL = "SELECT COUNT(*) FROM SYSCAT.TABLES WHERE TYPE = 'T' AND TABSCHEMA = '" + strings.ToUpper(sc) + "'" + like("UPPER(TABNAME) LIKE UPPER(%s)")
		pageSQL = fmt.Sprintf(`SELECT TABNAME, CARD FROM SYSCAT.TABLES
			WHERE TYPE = 'T' AND TABSCHEMA = '%s'%s ORDER BY TABNAME
			OFFSET %d ROWS FETCH FIRST %d ROWS ONLY`, strings.ToUpper(sc), like("UPPER(TABNAME) LIKE UPPER(%s)"), offset, size)
	case "sqlite":
		cntSQL = "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'" + like("name LIKE %s")
		pageSQL = fmt.Sprintf(`SELECT name FROM sqlite_master
			WHERE type = 'table' AND name NOT LIKE 'sqlite_%s'%s
			ORDER BY name LIMIT %d OFFSET %d`, "%", like("name LIKE %s"), size, offset)
	case "sybase":
		// ASE 无通用 OFFSET：取 TOP(page*size) 后在内存切片
		cntSQL = "SELECT COUNT(*) FROM sysobjects WHERE type = 'U'" + like("name LIKE %s")
		pageSQL = fmt.Sprintf("SELECT TOP %d name FROM sysobjects WHERE type = 'U'%s ORDER BY name", page*size, like("name LIKE %s"))
	default:
		return nil, fmt.Errorf("该数据库类型暂不支持数据表分页")
	}

	if cntSQL != "" {
		tp.Total = parseI64(QueryString(db, cntSQL, "0"))
	}
	cols, rows, err := QueryTable(db, pageSQL)
	if err != nil {
		return nil, err
	}
	// 统一列名
	switch family {
	case "mysql":
		cols = []string{"表名", "行数(估算)", "大小", "备注"}
	case "pg":
		cols = []string{"表名", "大小"}
	case "mssql", "oracle", "dm":
		cols = []string{"表名"}
	case "hana":
		cols = []string{"表名", "行数"}
	case "db2":
		cols = []string{"表名", "行数(统计)"}
	case "clickhouse", "sqlite", "sybase":
		cols = []string{"表名"}
	}
	// sybase/oracle/dm 的 ROWNUM/TOP 截取本页
	if family == "sybase" || family == "oracle" || family == "dm" {
		if offset < len(rows) {
			end := offset + size
			if end > len(rows) {
				end = len(rows)
			}
			rows = rows[offset:end]
		} else {
			rows = nil
		}
	}
	tp.Cols = cols
	tp.Rows = rows
	return tp, nil
}
