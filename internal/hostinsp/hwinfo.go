package hostinsp

// hwinfo.go lsscsi / lspci 采集。

import (
	"strings"

	"dbping/internal/model"
)

func collectLsscsi(c *SSHClient, r *model.HostReport, run func(string) string) {
	if strings.TrimSpace(run("command -v lsscsi 2>/dev/null")) == "" {
		r.LsscsiNote = "未安装 lsscsi"
		return
	}
	r.Lsscsi = strings.TrimSpace(run("lsscsi 2>/dev/null"))
}

func collectLspci(c *SSHClient, r *model.HostReport, run func(string) string) {
	if strings.TrimSpace(run("command -v lspci 2>/dev/null")) == "" {
		r.LspciNote = "未安装 lspci"
		return
	}
	r.Lspci = strings.TrimSpace(run("lspci 2>/dev/null"))
}
