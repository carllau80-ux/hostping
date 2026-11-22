// bmctest BMC 带外巡检命令行验证工具（无 UI，直接调用 internal/bmc）。
// 用法：go run ./devtools/bmctest -host 192.168.2.100 -user ADMIN -pass admin123 [-port 623] [-timeout 10]
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"dbping/internal/bmc"
	"dbping/internal/model"
)

func main() {
	host := flag.String("host", "", "BMC IP 地址")
	port := flag.Int("port", 623, "BMC 端口（IPMI 默认 623）")
	user := flag.String("user", "ADMIN", "BMC 用户名")
	pass := flag.String("pass", "", "BMC 密码")
	timeout := flag.Int("timeout", 10, "超时秒数")
	flag.Parse()
	if *host == "" || *pass == "" {
		fmt.Println("用法: bmctest -host <BMC IP> -user <用户> -pass <密码> [-port 623] [-timeout 10]")
		os.Exit(1)
	}
	h := &model.BMCHost{ID: "cli", Name: "CLI 验证", Host: *host, Port: *port, User: *user, Password: *pass, Timeout: *timeout}

	// 先做连接测试
	fmt.Printf("--- 连接测试 ---\n")
	tr := bmc.TestBMC(h)
	ok, _ := tr["ok"].(bool)
	detail, _ := tr["detail"].(string)
	if !ok {
		fmt.Printf("  失败: %s\n", detail)
		os.Exit(2)
	}
	fmt.Printf("  OK · %s\n", detail)

	start := time.Now()
	r, err := bmc.InspectBMC(h, nil)
	if err != nil {
		fmt.Printf("巡检失败: %v\n", err)
		os.Exit(3)
	}
	fmt.Printf("\n巡检完成（%.1fs）\n健康: %s（%s）\n", time.Since(start).Seconds(), r.Health, r.HealthDetail)

	if r.Device != nil {
		fmt.Printf("\n--- BMC 设备 ---\n  厂商 %s · 产品 %s\n  固件 %s · IPMI %s · 自检 %s\n",
			r.Device.Manufacturer, r.Device.Product, r.Device.Firmware, r.Device.IPMIVersion, r.Device.SelfTest)
	} else if r.DeviceNote != "" {
		fmt.Printf("\n--- BMC 设备 ---\n  %s\n", r.DeviceNote)
	}
	if len(r.Chassis) > 0 {
		fmt.Printf("\n--- 机箱状态 ---\n")
		for _, kv := range r.Chassis {
			fmt.Printf("  %s: %s\n", kv.Key, kv.Value)
		}
	}
	if len(r.Sensors) > 0 {
		fmt.Printf("\n--- 传感器（%d） ---\n", len(r.Sensors))
		for _, s := range r.Sensors {
			fmt.Printf("  [%s/%s] %s = %s · 状态 %s · %s\n", s.Kind, s.Number, s.Name, s.Value, s.Status, s.Range)
		}
	} else if r.SensorsNote != "" {
		fmt.Printf("\n--- 传感器 ---\n  %s\n", r.SensorsNote)
	}
	if len(r.FRU) > 0 {
		fmt.Printf("\n--- FRU ---\n")
		for _, kv := range r.FRU {
			if kv.Key == "__fru" {
				fmt.Printf("  ◆ %s\n", kv.Value)
			} else {
				fmt.Printf("  %s: %s\n", kv.Key, kv.Value)
			}
		}
	} else if r.FRUNote != "" {
		fmt.Printf("\n--- FRU ---\n  %s\n", r.FRUNote)
	}
	if r.SEL != nil {
		fmt.Printf("\n--- SEL（共 %s 条：严重 %d / 警告 %d / 信息 %d） ---\n", r.SEL.Total, r.SEL.CriticalCount, r.SEL.WarningCount, r.SEL.InfoCount)
		if r.SEL.Analysis != "" {
			fmt.Printf("  规则分析: %s\n", r.SEL.Analysis)
		}
		for _, e := range r.SEL.Entries {
			fmt.Printf("  %s [%s] %s: %s\n", e.Time, e.Severity, e.Sensor, e.Event)
		}
		if r.SEL.Note != "" {
			fmt.Printf("  备注: %s\n", r.SEL.Note)
		}
	} else if r.SELNote != "" {
		fmt.Printf("\n--- SEL ---\n  %s\n", r.SELNote)
	}
	if len(r.Warnings) > 0 {
		fmt.Printf("\n--- 警告 ---\n")
		for _, w := range r.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}
}
