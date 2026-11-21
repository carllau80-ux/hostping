package provider

// register.go 将全部 family 实现注册进注册表。

func init() {
	Register("mysql", mysqlFamily{})
	Register("pg", pgFamily{})
	Register("mssql", mssqlProvider{})
	Register("oracle", oracleProvider{})
	Register("dm", dmProvider{})
	Register("hana", hanaProvider{})
	Register("clickhouse", chProvider{})
	Register("redis", redisProvider{})
	Register("mongo", mongoProvider{})
	Register("sybase", sybaseProvider{})
	Register("sqlite", sqliteProvider{})
	Register("db2", db2Provider{})
}
