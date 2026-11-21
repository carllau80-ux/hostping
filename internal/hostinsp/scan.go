package hostinsp

// scan.go 端口相关：监听端口采集（巡检节）+ 端口规格解析（nmap 式扫描用）。

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"dbping/internal/model"
)

// wellKnown 常见端口服务映射。
var wellKnown = map[int]string{
	20: "FTP-数据", 21: "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP", 53: "DNS",
	67: "DHCP", 80: "HTTP", 110: "POP3", 111: "RPCBind", 123: "NTP", 135: "RPC",
	139: "NetBIOS", 143: "IMAP", 161: "SNMP", 389: "LDAP", 443: "HTTPS",
	445: "SMB", 465: "SMTPS", 514: "Syslog", 587: "SMTP-TLS", 623: "IPMI",
	636: "LDAPS", 993: "IMAPS", 995: "POP3S", 1080: "SOCKS", 1433: "MSSQL",
	1521: "Oracle", 2049: "NFS", 2181: "ZooKeeper", 3000: "Grafana/Node",
	3306: "MySQL", 3389: "RDP", 5000: "Flask/UPnP", 5432: "PostgreSQL",
	5555: "ADB", 5900: "VNC", 5984: "CouchDB", 6379: "Redis", 6443: "K8s API",
	8000: "HTTP-Alt", 8008: "HTTP-Alt", 8080: "HTTP-Proxy", 8081: "HTTP-Alt",
	8443: "HTTPS-Alt", 8500: "Consul", 9000: "Portainer/PHP-FPM",
	9090: "Prometheus", 9200: "Elasticsearch", 9300: "ES-Cluster", 11211: "Memcached",
	15672: "RabbitMQ管理", 27017: "MongoDB", 50000: "DB2", 50001: "DB2-SSL",
}

// ServiceName 返回常见端口的服务名（未知端口返回空）。
func ServiceName(port int) string { return wellKnown[port] }

// ParsePortSpec 解析端口规格 "22,80,443,8000-8100" → 有序去重端口列表。
func ParsePortSpec(spec string) ([]int, error) {
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bits := strings.SplitN(part, "-", 2)
			lo, err1 := parseIntTrim(bits[0])
			hi, err2 := parseIntTrim(bits[1])
			if err1 != nil || err2 != nil || lo <= 0 || hi > 65535 || lo > hi {
				return nil, fmt.Errorf("端口范围无效: %s", part)
			}
			if hi-lo > 10000 {
				return nil, fmt.Errorf("端口范围过大（>10000）: %s", part)
			}
			for p := lo; p <= hi; p++ {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		} else {
			p, err := parseIntTrim(part)
			if err != nil || p <= 0 || p > 65535 {
				return nil, fmt.Errorf("端口无效: %s", part)
			}
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
		if len(out) > 2000 {
			return nil, fmt.Errorf("端口总数超过 2000，请缩小范围")
		}
	}
	return out, nil
}

func parseIntTrim(s string) (int, error) {
	n := 0
	for _, ch := range strings.TrimSpace(s) {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid port char")
		}
		n = n*10 + int(ch-'0')
	}
	if n == 0 {
		return 0, fmt.Errorf("port 0")
	}
	return n, nil
}

// ---------- 监听端口采集（巡检节） ----------

// portStat 单端口的聚合状态。
type portStat struct {
	process string
	states  map[string]int
	proto   string // tcp / udp
}

// collectListenPorts 采集主机全部监听端口（TCP/UDP）及相关进程、连接数、状态。
// 数据来自 ss -tanp / ss -uanp（含全部连接状态，一次采集多用途）。
func collectListenPorts(r *model.HostReport, run func(string) string) {
	stats := map[string]*portStat{}
	get := func(port string) *portStat {
		if stats[port] == nil {
			stats[port] = &portStat{states: map[string]int{}}
		}
		return stats[port]
	}
	parseSS := func(rows, proto string) {
		for _, line := range strings.Split(rows, "\n") {
			line = strings.TrimRight(line, "\r")
			fs := strings.Fields(line)
			if len(fs) < 5 || fs[0] == "State" {
				continue
			}
			// State Recv-Q Send-Q Local:Port Peer:Port [Process]
			local := fs[3]
			i := strings.LastIndexByte(local, ':')
			if i < 0 {
				continue
			}
			port := local[i+1:]
			if !allDigits(port) {
				continue
			}
			st := get(port)
			st.proto = proto
			st.states[fs[0]]++
			if st.process == "" {
				st.process = extractProc(line)
			}
		}
	}
	parseSS(run("ss -tanp 2>/dev/null"), "tcp")
	parseSS(run("ss -uanp 2>/dev/null"), "udp")

	// 只保留监听中的端口（UDP: UNCONN 即绑定监听）
	var ports []int
	for ps, st := range stats {
		if st.states["LISTEN"] > 0 || st.states["UNCONN"] > 0 {
			n, _ := strconv.Atoi(ps)
			ports = append(ports, n)
		}
	}
	sort.Ints(ports)

	for _, n := range ports {
		key := strconv.Itoa(n)
		st := stats[key]
		// 连接数摘要：除监听行之外的全部状态计数
		var parts []string
		total := 0
		for _, s := range []string{"ESTAB", "TIME-WAIT", "CLOSE-WAIT", "SYN-SENT", "SYN-RECV", "FIN-WAIT-1", "FIN-WAIT-2", "LAST-ACK", "CLOSING"} {
			if c := st.states[s]; c > 0 {
				parts = append(parts, s+" "+strconv.Itoa(c))
				total += c
			}
		}
		conns := "-"
		if len(parts) > 0 {
			conns = strings.Join(parts, " · ")
		}
		state := "LISTEN"
		if st.proto == "udp" {
			state = "UDP"
		}
		pi := model.PortInfo{
			Port:    key,
			State:   state,
			Service: orDash(wellKnown[n]),
			Process: orDash(st.process),
			Conns:   conns,
		}
		r.ListenPorts = append(r.ListenPorts, pi)
	}
	if len(r.ListenPorts) == 0 {
		warn(r, "未采集到监听端口（ss 命令不可用或权限不足）")
	}
}

// extractProc 从 ss 行尾提取 ((\"name\",pid=..,fd=..)) 中的进程名。
func extractProc(line string) string {
	si := strings.Index(line, "((\"")
	if si < 0 {
		return ""
	}
	raw := line[si+3:]
	if qi := strings.IndexByte(raw, '"'); qi >= 0 {
		return raw[:qi]
	}
	return ""
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
