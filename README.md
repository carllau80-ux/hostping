# DBPing — 多数据库巡检与拓扑工具

Go 语言开发的数据库健康检查与元数据采集桌面工具：**纯 Go 驱动（零 cgo）、单可执行文件、Windows 优先**。

![Go](https://img.shields.io/badge/Go-1.26-00ADD8) ![Driver](https://img.shields.io/badge/驱动-100%25_纯Go-blue) ![Build](https://img.shields.io/badge/产物-单exe-lightgrey)

## 功能

- **三级 Ping 探活**：TCP 端口 → 认证登录 → SQL 探活，分别统计延迟，失败原因中文分类（网络不通/认证失败/权限不足/超时…）
- **元数据采集**：版本、实例、主备角色、只读、**归档/日志模式**（附各库开启归档的参考命令提示）、字符集、启动时间、库大小、表数量、连接数、Top N 大表、表空间/数据文件、集群节点
- **拓扑展示**：自上而下分层树形图（实例层 → 集群/复制层 → 数据库层 → 表空间层 → 表层，ECharts 离线渲染）；分布式库（TiDB/OceanBase/ClickHouse/Doris 等）展示集群组件拓扑
- **概览页**：大数据库图标 + 列表式信息展示
- **连接管理**：分组、搜索筛选，带原生图标的数据库类型名录（按厂商分组、可搜索），密码 AES-GCM 本地加密存储
- **定时自动巡检** + 检查历史（可用率、延迟曲线）
- **巡检报告导出**（单连接 HTML / 全部汇总 HTML）
- **界面**：中英文一键切换、明/暗主题、应用图标

## 支持的数据库（27 类，全部纯 Go 驱动，免装客户端）

| 协议族 | 数据库 |
|---|---|
| MySQL 协议 | MySQL、MariaDB、TiDB、OceanBase(MySQL租户)、Apache Doris、StarRocks、GBase 8a、PolarDB/PolarDB-X、TDSQL、GoldenDB |
| PostgreSQL 协议 | PostgreSQL、人大金仓 KingbaseES、openGauss/GaussDB、MogDB、Greenplum、CockroachDB、YugabyteDB、GBase 8c、Vastbase G100 |
| 独立协议 | SQL Server、Oracle（11g~23ai，免 Instant Client）、达梦 DM8、SAP HANA 2.0、ClickHouse |
| NoSQL | Redis、MongoDB |

> Oracle 租户经 ODP 连接 OceanBase(Oracle模式) 为实验性支持；GBase 8s / 神通 / DB2 无纯 Go 驱动，暂不支持。

## 运行

双击 `bin/DBPing.exe` 即可（无需安装，配置保存在 `%APPDATA%\DBPing\`）。

- 系统要求：Windows 10 21H2+ / Windows 11（自带 WebView2 运行时）
- 所有采集 SQL 均为只读；Oracle/达梦 建议使用具备 `v$`/`dba_` 视图权限的账号（权限不足时自动降级采集并提示）

## 开发构建

```bash
go build -tags desktop,production -ldflags "-s -w -H windowsgui" -o bin/DBPing.exe .
```

无 Node/npm 依赖（前端为免构建原生 HTML/JS + 本地 ECharts）。

## 代码结构

```
main.go                    Wails 入口（前端资源嵌入）
app/                       Wails 绑定层 + HTML 巡检报告生成
internal/
  model/                   数据模型（前后端共享 JSON 结构）
  provider/                采集实现（核心扩展点）
    provider.go            接口 + 27 类数据库类型注册表
    sqlx.go                DSN 构造 + 驱动导入 + 三级探活通用流程
    mysql_family.go        MySQL 协议族（10 类）
    pg_family.go           PostgreSQL 协议族（9 类）
    mssql/oracle/dm/hana   独立协议四类
    clickhouse/redis/mongo 三类
  config/                  配置持久化 + AES-GCM 密码加密
  monitor/                 并发巡检调度 + 定时循环 + 历史
frontend/dist/             原生 HTML/JS 前端 + echarts.min.js
```

新增一个数据库 = 在 `provider.TypeInfos` 注册一条类型 +（必要时）加一个查询方言分支。
