package hostinsp

// security.go 安全状态（SELinux/AppArmor/防火墙）与系统日志检查。

import (
	"fmt"
	"strings"

	"dbping/internal/model"
)

// collectSecurity 采集 SELinux / 防火墙状态（KV 列表，__ 前缀为大文本块）。
func collectSecurity(run func(string) string) []model.KV {
	var out []model.KV
	kv := func(k, v string) {
		out = append(out, model.KV{Key: k, Value: orDash(v)})
	}
	// SELinux
	se := strings.TrimSpace(run("getenforce 2>/dev/null"))
	switch se {
	case "Enforcing", "Permissive":
		kv("SELinux", se)
	case "":
		kv("SELinux", "未安装/未启用")
	default:
		kv("SELinux", se)
	}
	// AppArmor（Debian/Ubuntu 常用）
	aa := strings.TrimSpace(run(`aa-status 2>/dev/null | grep -m1 "profiles are loaded"`))
	if aa != "" {
		kv("AppArmor", aa)
	}
	// firewalld
	fw := strings.TrimSpace(run("systemctl is-active firewalld 2>/dev/null"))
	if fw == "active" {
		kv("firewalld", "运行中")
		rules := strings.TrimSpace(run("firewall-cmd --list-all 2>/dev/null | head -14"))
		if rules != "" {
			out = append(out, model.KV{Key: "__firewalld", Value: rules})
		}
	} else {
		kv("firewalld", "未运行")
	}
	// ufw
	ufw := strings.TrimSpace(run("ufw status 2>/dev/null | head -1"))
	if strings.Contains(strings.ToLower(ufw), "active") {
		kv("ufw", "已启用")
		out = append(out, model.KV{Key: "__ufw", Value: strings.TrimSpace(run("ufw status 2>/dev/null | head -20"))})
	} else if ufw != "" {
		kv("ufw", "未启用")
	}
	// iptables 规则行数
	ipLines := strings.TrimSpace(run("iptables -S 2>/dev/null | wc -l"))
	if ipLines != "" {
		kv("iptables", ipLines+" 条规则")
	} else {
		kv("iptables", "不可用")
	}
	// nftables
	nft := strings.TrimSpace(run("nft list ruleset 2>/dev/null | wc -l"))
	switch {
	case nft == "":
		kv("nftables", "不可用")
	case nft == "0":
		kv("nftables", "空规则集")
	default:
		kv("nftables", nft+" 行规则")
	}
	return out
}

// collectSysLog 检查系统日志：优先 journalctl，降级 dmesg / /var/log/syslog。
func collectSysLog(run func(string) string) *model.SysLogReport {
	rep := &model.SysLogReport{ErrCount: "0", WarnCount: "0", Source: "journalctl"}

	errOut := run(`journalctl -p 3 -S "-24h" --no-pager -q 2>/dev/null | tail -40`)
	if strings.TrimSpace(errOut) != "" {
		lines := nonEmptyLines(errOut)
		rep.ErrCount = fmt.Sprintf("%d", countLines(errOut))
		rep.TopErrors = capLines(lines, 40)
	}
	warnOut := run(`journalctl -p 4 -S "-24h" --no-pager -q 2>/dev/null | tail -20`)
	if strings.TrimSpace(warnOut) != "" {
		lines := nonEmptyLines(warnOut)
		rep.WarnCount = fmt.Sprintf("%d", countLines(warnOut))
		rep.TopWarns = capLines(lines, 20)
	}
	dmesg := run(`dmesg --level=err,warn 2>/dev/null | tail -15`)
	if strings.TrimSpace(dmesg) != "" {
		rep.DmesgErrs = capLines(nonEmptyLines(dmesg), 15)
	}
	if rep.ErrCount == "0" && strings.TrimSpace(run("command -v journalctl 2>/dev/null")) == "" {
		rep.Source = "/var/log/syslog"
		syslog := run(`grep -iE "error|fail|crit" /var/log/syslog 2>/dev/null | tail -20`)
		if strings.TrimSpace(syslog) != "" {
			lines := nonEmptyLines(syslog)
			rep.ErrCount = fmt.Sprintf("%d", len(lines))
			rep.TopErrors = capLines(lines, 20)
		}
	}
	return rep
}

func nonEmptyLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func capLines(lines []string, max int) []string {
	if len(lines) > max {
		return lines[:max]
	}
	return lines
}
