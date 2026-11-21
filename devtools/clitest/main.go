// clitest 用 DBPing 的 provider/monitor 采集代码做命令行实测（与 GUI 同一代码路径）。
// 用法: go run ./devtools/clitest -type pg -host 192.168.31.58 -port 5432 -user postgres -pass xxx -db postgres [-save]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"dbping/internal/config"
	"dbping/internal/model"
	"dbping/internal/monitor"
	"dbping/internal/provider"
)

func main() {
	typ := flag.String("type", "pg", "数据库类型")
	host := flag.String("host", "", "主机")
	port := flag.Int("port", 0, "端口")
	user := flag.String("user", "", "用户名")
	pass := flag.String("pass", "", "密码")
	db := flag.String("db", "", "数据库/服务名")
	params := flag.String("params", "", "附加参数（如 AUTH TYPE=SYSDBA）")
	name := flag.String("name", "", "显示名（默认 host:port）")
	group := flag.String("group", "", "分组")
	save := flag.Bool("save", false, "将连接写入 DBPing GUI 配置")
	fromCfg := flag.String("from-config", "", "从 DBPing 配置读取连接（按名称或 ID，密码自动解密）并测试")
	sqlText := flag.String("sql", "", "执行 SQL（高级模式引擎：SELECT 自动分页）")
	page := flag.Int("page", 1, "页码")
	size := flag.Int("size", 10, "每页行数")
	tables := flag.Bool("tables", false, "测试数据表元数据分页")
	schema := flag.String("schema", "", "数据表分页的模式筛选")
	filter := flag.String("filter", "", "数据表分页的表名关键字")
	flag.Parse()

	var c *model.Conn
	if *fromCfg != "" {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("读取配置失败:", err)
			os.Exit(1)
		}
		found := false
		for _, conn := range cfg.ListConns() {
			if conn.Name == *fromCfg || conn.ID == *fromCfg {
				cc := conn
				c = &cc
				found = true
				break
			}
		}
		if !found {
			fmt.Println("配置中未找到连接:", *fromCfg)
			os.Exit(1)
		}
		if *name != "" {
			c.Name = *name
		}
	} else {
		if *host == "" {
			fmt.Fprintln(os.Stderr, "需要 -host 或 -from-config")
			os.Exit(2)
		}
		c = &model.Conn{
			Name: or(*name, fmt.Sprintf("%s:%d", *host, *port)),
			Group: *group, Type: *typ, Host: *host, Port: *port,
			Database: *db, User: *user, Password: *pass, Params: *params, Timeout: 5,
		}
	}

	// SQL 执行引擎测试
	if *sqlText != "" {
		r, err := provider.ExecSQL(c, *sqlText, *page, *size)
		if err != nil {
			fmt.Println("ExecSQL 失败:", err)
			os.Exit(1)
		}
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
		return
	}
	// 数据表分页测试
	if *tables {
		tp, err := provider.ListTablesPaged(c, *schema, *filter, *page, *size)
		if err != nil {
			fmt.Println("ListTablesPaged 失败:", err)
			os.Exit(1)
		}
		b, _ := json.MarshalIndent(tp, "", "  ")
		fmt.Println(string(b))
		return
	}

	mon := monitor.New(nil)

	fmt.Println("========== 1. Ping 三级探活 ==========")
	r := mon.PingOne(c)
	fmt.Printf("结果: %v   阶段: %s   TCP: %dms   SQL: %dms\n", okStr(r.OK), r.Stage, r.TCPMs, r.SQLMs)
	if r.Error != "" {
		fmt.Println("错误:", r.Error)
	}
	if !r.OK {
		os.Exit(1)
	}

	fmt.Println("\n========== 2. 实例概览 Overview ==========")
	ov, err := mon.GetOverview(c)
	if err != nil {
		fmt.Println("Overview 失败:", err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(ov, "", "  ")
	fmt.Println(string(b))

	fmt.Println("\n========== 3. 明细 Detail ==========")
	dtl, err := mon.GetDetail(c)
	if err != nil {
		fmt.Println("Detail 失败:", err)
		os.Exit(1)
	}
	printDetail(dtl)

	fmt.Println("\n========== 4. 拓扑 Topology ==========")
	tp, err := mon.GetTopology(c)
	if err != nil {
		fmt.Println("Topology 失败:", err)
		os.Exit(1)
	}
	printTree(tp.Root, 0)

	if *save {
		fmt.Println("\n========== 5. 写入 GUI 配置 ==========")
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("加载配置失败:", err)
			os.Exit(1)
		}
		c.ID = ""
		c.Name = or(*name, fmt.Sprintf("%s(%s)", *host, *typ))
		if err := cfg.UpsertConn(*c); err != nil {
			fmt.Println("写入失败:", err)
			os.Exit(1)
		}
		if err := cfg.Save(); err != nil {
			fmt.Println("保存失败:", err)
			os.Exit(1)
		}
		fmt.Println("已写入配置目录:", cfg.DirPath, "（重启 GUI 后可见）")
	}
}

func printDetail(d *model.Detail) {
	if len(d.Dbs) > 0 {
		fmt.Println("-- 库/模式 --")
		for _, x := range d.Dbs {
			fmt.Printf("  %-20s 大小:%-12s 表数:%-6s %s\n", x.Name, or(x.Size, "-"), or(x.Tables, "-"), x.Note)
		}
	}
	if len(d.Tables) > 0 {
		fmt.Println("-- 大表 Top N --")
		for _, x := range d.Tables {
			fmt.Printf("  %s.%s  行:%-10s 大小:%s\n", or(x.Schema, "-"), x.Name, or(x.Rows, "-"), or(x.Size, "-"))
		}
	}
	if len(d.TBS) > 0 {
		fmt.Println("-- 表空间 --")
		for _, x := range d.TBS {
			fmt.Printf("  %-20s 大小:%-12s 状态:%s\n", x.Name, or(x.Size, "-"), or(x.Status, "-"))
		}
	}
	if len(d.Services) > 0 {
		fmt.Println("-- 集群/服务 --")
		for _, x := range d.Services {
			fmt.Printf("  %-24s 角色:%-16s 状态:%s\n", x.Name, x.Role, x.Status)
		}
	}
	if len(d.Dbs)+len(d.Tables)+len(d.TBS)+len(d.Services) == 0 {
		fmt.Println("(无明细数据)")
	}
}

func printTree(n *model.TNode, depth int) {
	fmt.Printf("%s%s [%s]  %s\n", strings.Repeat("  ", depth), n.Label, n.Status, n.Sub)
	for _, ch := range n.Children {
		printTree(ch, depth+1)
	}
}

func okStr(b bool) string {
	if b {
		return "✓ 正常"
	}
	return "✗ 故障"
}

func or(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
