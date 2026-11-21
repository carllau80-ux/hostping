// hosttest 主机巡检端到端验证工具。
// 用法: go run ./devtools/hosttest -host 192.168.31.58 -user root -pass xxx [-key path] [-keypass p]
//
//	或: go run ./devtools/hosttest -from-config [名称/IP关键字]  （直接取本机 GUI 已保存的主机）
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"dbping/internal/config"
	"dbping/internal/hostinsp"
	"dbping/internal/model"
)

func main() {
	host := flag.String("host", "", "主机")
	port := flag.Int("port", 22, "端口")
	user := flag.String("user", "root", "SSH 用户")
	pass := flag.String("pass", "", "密码")
	keyPath := flag.String("key", "", "私钥路径")
	keyPass := flag.String("keypass", "", "私钥口令")
	timeout := flag.Int("timeout", 15, "超时秒")
	fromConfig := flag.String("from-config", "", "从本机配置取主机（可选名称/IP 关键字）")
	flag.Parse()

	var h *model.SSHHost
	keyword := *fromConfig // -from-config [关键字] 或位置参数
	if flag.NArg() > 0 {
		keyword = flag.Arg(0)
	}
	if *fromConfig != "" || keyword != "" && *host == "" {
		st, err := config.Load()
		if err != nil {
			fmt.Println("读取配置失败:", err)
			os.Exit(1)
		}
		for i := range st.Hosts {
			hh := &st.Hosts[i]
			if keyword == "*" || strings.Contains(hh.Name, keyword) || strings.Contains(hh.Host, keyword) {
				h = hh
				break
			}
		}
		if h == nil {
			fmt.Println("配置中未找到匹配主机:", keyword, "（共", len(st.Hosts), "台）")
			os.Exit(1)
		}
		if h.Password == "" && h.KeyPath == "" {
			fmt.Println("该主机无已保存凭据")
			os.Exit(1)
		}
	} else {
		if *host == "" {
			fmt.Fprintln(os.Stderr, "需要 -host 或 -from-config")
			os.Exit(2)
		}
		h = &model.SSHHost{
			ID: "test", Name: *host, Host: *host, Port: *port,
			User: *user, AuthType: "password", Password: *pass,
			KeyPath: *keyPath, KeyPassphrase: *keyPass, Timeout: *timeout,
		}
		if *keyPath != "" {
			h.AuthType = "key"
		}
	}
	r, err := hostinsp.Inspect(h, nil)
	if err != nil {
		fmt.Println("巡检失败:", err)
		os.Exit(1)
	}
	fmt.Println("OS:", r.OSName)
	fmt.Println("--- 系统信息 ---")
	for _, kv := range r.SysInfo {
		if strings.HasPrefix(kv.Key, "__") {
			continue
		}
		fmt.Printf("  %s: %s\n", kv.Key, kv.Value)
	}
	fmt.Printf("--- Top CPU (%d) ---\n", len(r.TopCPU))
	for _, p := range r.TopCPU {
		fmt.Printf("  PID %s CPU %s%% MEM %s%%  %s\n", p.PID, p.CPU, p.MEM, firstN(p.Cmd, 60))
	}
	fmt.Printf("--- Top MEM (%d) ---\n", len(r.TopMem))
	for _, p := range r.TopMem {
		fmt.Printf("  PID %s MEM %s%% RSS %s  %s\n", p.PID, p.MEM, p.RSS, firstN(p.Cmd, 60))
	}
	fmt.Printf("--- rs_/rsa_ 进程 (%d) ---\n", len(r.RSProcs))
	for _, p := range r.RSProcs {
		fmt.Printf("  PID %s USER %s ETime %s FD %s  %s\n    打开文件 %d 条, 网络连接 %d 条\n",
			p.PID, p.User, p.ETime, p.FDCount, firstN(p.Cmd, 70), len(p.OpenFiles), len(p.Conns))
	}
	fmt.Printf("--- storcli ---\n  %s (控制器 %d)\n", orEmpty(r.StorcliNote), ctlCount(r.Storcli))
	fmt.Printf("--- 安全状态 ---\n")
	for _, kv := range r.Security {
		fmt.Printf("  %s: %s\n", kv.Key, kv.Value)
	}
	fmt.Printf("--- 监听端口 (%d) ---\n", len(r.ListenPorts))
	for _, p := range r.ListenPorts {
		fmt.Printf("  端口 %-6s %-6s 服务:%-14s 进程:%s  连接:%s\n", p.Port, p.State, p.Service, p.Process, p.Conns)
	}
	fmt.Printf("--- ZFS ---\n  %s (池 %d)\n", orEmpty(r.ZFSNote), poolCount(r.ZFS))
	fmt.Printf("--- 重启记录 (%d) ---\n", len(r.Reboots))
	for i, rb := range r.Reboots {
		fmt.Printf("  %d. 启动 %s · 结束 %s · 时长 %s\n", i+1, rb.Start, rb.End, rb.Dur)
	}
	if len(r.Warnings) > 0 {
		fmt.Println("--- 警告 ---")
		for _, w := range r.Warnings {
			fmt.Println("  ⚠", w)
		}
	}
}

func ctlCount(r *model.StorcliReport) int {
	if r == nil {
		return 0
	}
	return len(r.Controllers)
}

func poolCount(r *model.ZFSReport) int {
	if r == nil {
		return 0
	}
	return len(r.Pools)
}

func orEmpty(s string) string {
	if s == "" {
		return "(无备注)"
	}
	return s
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

