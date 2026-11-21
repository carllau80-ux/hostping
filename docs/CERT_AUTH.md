# 证书认证支持调研（基于 DBPing 内置驱动源码验证）

> 调研方法：逐一检查 DBPing 实际引入的纯 Go 驱动源码（模块缓存内 grep），确认 TLS/证书参数的真实支持能力，
> 而非厂商文档泛述。结论分三层：**服务端能力**（数据库本身）、**驱动能力**（我们用的纯 Go 驱动）、**DBPing 现状**（附加参数能否直通）。

## 一、总矩阵

| 数据库 | 服务端证书认证 | 认证等级 | 驱动支持（源码验证） | DBPing 现状 |
|---|---|---|---|---|
| **PostgreSQL 系**（PG/GP/Cockroach/Yugabyte/GBase8c/Vastbase） | ★★★ 最强 | sslmode 六级：disable/allow/prefer/require/**verify-ca/verify-full**；客户端证书登录（pg_hba `cert` 方法，CN 映射用户） | pgx/lib-pq 完整：sslmode、sslrootcert、sslcert、sslkey、sslpassword | ✅ 附加参数直通即用 |
| **人大金仓** | ★★★ | 同 PG | lib/pq 同上 | ✅ 直通 |
| **openGauss/GaussDB/MogDB** | ★★★ | 同 PG，另支持**国密 TLCP** 证书 | openGauss 官方 connector（lib/pq 系） | ✅ 直通 |
| **Oracle**（TCPS） | ★★★ | 服务端验证；**双向 TLS（wallet）**；**证书即身份**（DN 映射数据库用户） | go-ora：`SSL=true`、`SSL VERIFY`、`WALLET`（PKCS12 路径）、`WALLET PASSWORD` | ✅ 直通 |
| **MySQL 系**（MySQL/MariaDB/TiDB/OB-MySQL/PolarDB/TDSQL/GoldenDB） | ★★☆ | REQUIRE NONE/**SSL/X509/SUBJECT/ISSUER/CIPHER** 五级（X509 即强制客户端证书） | go-sql-driver：`tls=true/skip-verify/preferred`；**客户端证书需代码注册**（RegisterTLSConfig） | ⚠️ 单向 TLS 直通；客户端证书需代码接线 |
| **达梦 DM8** | ★★☆ | sslEncrypt 分级（1/2/4：认证/加密/双向）；**支持客户端证书** | dm 驱动：`sslEncrypt`、`sslCertPath`、`sslKeyPath`、`sslFilesPath`（兼容大写 SSL_CERT_PATH） | ✅ 直通 |
| **MongoDB** | ★★★ | 服务端验证（tlsCAFile）；**双向 TLS**；**MONGODB-X509 证书即登录身份**；副本集成员间证书 | mongo-driver：`tls=true`、`tlsCAFile`、`tlsCertificateKeyFile`、`tlsCertificateKeyFilePassword`、`allowInvalidCertificates`、`authMechanism=MONGODB-X509` | ✅ URI 参数直通 |
| **ClickHouse** | ★★☆ | 服务端验证；服务端可配 X.509 用户（common_name 映射） | clickhouse-go：DSN `secure=true`、`skip_verify=true`；**客户端证书需代码设置**（Options.TLS.Certificates） | ⚠️ 单向直通；客户端证书需代码 |
| **SAP HANA** | ★★☆ | 服务端验证；服务端支持 X.509 SSO 登录 | go-hdb：`TLSRootCAFile`、`TLSServerName`、`TLSInsecureSkipVerify`（仅服务端验证三级）；**客户端证书未通过 DSN 暴露** | ⚠️ 单向直通；双向需上游驱动支持 |
| **Redis 6+** | ★☆☆ | 传输层 mTLS（服务端可要求客户端证书）；但认证仍走密码/ACL（证书不做身份） | go-redis：`Options.TLSConfig`（代码级）；URL `rediss://` 自动启用 + `skip_verify` | ❌ DBPing 尚未接线 TLS（需代码） |
| **SQL Server** | ★☆☆ | 仅传输加密三档：`encrypt=disable/required/strict`（TDS8.0）；**无客户端证书登录**（Windows/SQL 认证） | go-mssqldb：`encrypt`、`trustservercertificate`、`certificate`（主机证书校验）、`hostnameincertificate` | ✅ 直通（仅加密与服务器证书校验） |
| **Sybase ASE** | ★☆☆ | 服务端支持证书（trusted.txt），可配双向 | thda/tds：仅 `ssl=on`（纯加密，无验证等级、无客户端证书） | ⚠️ 仅加密 |
| **SQLite** | — | 本地文件，不适用 | — | — |
| **Doris/StarRocks** | ★☆☆ | FE 端口可开 TLS（enable_ssl）；客户端证书支持有限 | MySQL 协议同上 | 同 MySQL 系 |
| **OceanBase (Oracle租户)** | ★★☆ | TCPS 同 Oracle | go-ora 同上 | 同 Oracle |

## 二、DBPing 启用方式（今天即可用，填"附加参数"即可）

| 场景 | 附加参数示例 |
|---|---|
| PG 系双向证书 + 严格校验 | `sslmode=verify-full&sslrootcert=C:\certs\ca.pem&sslcert=C:\certs\client.crt&sslkey=C:\certs\client.key` |
| PG 系仅校验服务器证书 | `sslmode=verify-ca&sslrootcert=C:\certs\ca.pem` |
| Oracle TCPS + wallet | `SSL=TRUE&SSL VERIFY=FALSE&WALLET=C:\certs\wallet&WALLET PASSWORD=xxx` |
| 达梦双向证书 | `sslEncrypt=4&sslCertPath=C:\certs\client-cert.pem&sslKeyPath=C:\certs\client-key.pem` |
| MySQL 强制加密（跳过验证） | `tls=skip-verify` |
| MongoDB X.509 登录 | `tls=true&tlsCAFile=C:\certs\ca.pem&tlsCertificateKeyFile=C:\certs\client.pem`（配合认证机制 X509） |
| HANA 单向 TLS | `TLSRootCAFile=C:\certs\trust.pem&TLSServerName=hana01` |
| SQL Server 强加密 | `encrypt=strict`（2022 TDS8.0）或 `encrypt=true&trustservercertificate=true` |
| ClickHouse 加密 | `secure=true`（跳过验证加 `skip_verify=true`） |

> 密码类含特殊字符时注意转义；证书文件路径建议用绝对路径。

## 三、差距与后续可做的接线（代码级）

| 差距 | 说明 | 工作量 |
|---|---|---|
| MySQL 系客户端证书 | 需要 `mysql.RegisterTLSConfig("dbping", &tls.Config{Certificates: ...})`，把 `tlsCert/tlsKey/tlsCA` 约定参数注册为命名配置后 DSN 引用 | 小 |
| Redis TLS | redisProvider 目前未设置 `Options.TLSConfig`；可约定 `ssl=true&sslCert=&sslKey=&sslCA=` 参数 | 小 |
| ClickHouse 客户端证书 | DSN 无证书参数，需要改用 Options 编程式构建（放弃纯 DSN） | 中 |
| HANA 双向 | go-hdb DSN 未暴露客户端证书，需上游支持或 fork | 依赖上游 |
| Sybase 双向 | thda/tds 未实现，依赖上游 | 依赖上游 |
| 统一证书 UI | 连接对话框可加"证书"页：CA/客户端证书/私钥三个文件选择器（映射到各家参数） | 中 |

## 四、结论

- **证书认证能力最强（等级最细）**：PostgreSQL 系（六级 sslmode + cert 登录）、Oracle TCPS（wallet + DN 映射）、MongoDB（X.509 即身份）。
- **支持双向证书**：PG 系、Oracle、MongoDB、达梦、MySQL 系（服务端 X509 + 驱动代码）、ClickHouse（代码）。
- **仅传输加密、无证书登录**：SQL Server、Redis（mTLS 但身份仍靠密码）、Sybase（驱动限制）。
- **DBPing 今天已可用**：PG 系、Oracle、达梦、MongoDB、HANA、SQL Server、ClickHouse（单向）——全部通过"附加参数"直通，无需改代码；MySQL/Redis/ClickHouse 的客户端证书需要上面第三节的少量接线。
