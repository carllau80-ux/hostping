package hostinsp

// osdetect.go 操作系统推测：ICMP TTL + 服务 Banner + 开放端口特征（类 nmap 的轻量 OS 探测）。

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

// ICMPTTL 向目标发送 ICMP Echo 并从回包 IP 头读取 TTL。
// Windows/Linux 的 ip4:icmp 原始套接字读取结果均含 IP 头；普通用户权限即可（无需管理员）。
func ICMPTTL(target string, timeout time.Duration) (int, bool) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	conn, err := net.DialTimeout("ip4:icmp", target, timeout)
	if err != nil {
		return 0, false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	// 构造 ICMP Echo Request（type 8, code 0）
	pkt := make([]byte, 8)
	pkt[0] = 8
	pkt[1] = 0
	binary.BigEndian.PutUint16(pkt[4:6], 0x4870)              // identifier
	binary.BigEndian.PutUint16(pkt[6:8], uint16(time.Now().UnixNano()&0xffff)) // seq
	binary.BigEndian.PutUint16(pkt[2:4], icmpChecksum(pkt))

	if _, err := conn.Write(pkt); err != nil {
		return 0, false
	}
	buf := make([]byte, 1500)
	for i := 0; i < 3; i++ { // 最多读几个包，跳过非 echo-reply（如 TTL 超时通知）
		n, err := conn.Read(buf)
		if err != nil {
			return 0, false
		}
		if n < 21 {
			continue
		}
		ihl := int(buf[0]&0x0f) * 4
		if ihl < 20 || ihl >= n {
			continue
		}
		if buf[ihl] != 0 { // 0 = Echo Reply
			continue
		}
		return int(buf[8]), true // IP 头第 9 字节 = TTL
	}
	return 0, false
}

func icmpChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i : i+2]))
	}
	for sum>>16 != 0 {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return ^uint16(sum)
}

// GrabBanner 尝试读取 TCP 服务的首行 Banner（仅对主动推 Banner 的协议有效）。
func GrabBanner(target string, port int, timeout time.Duration) string {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", target, port), timeout)
	if err != nil {
		return ""
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil && n == 0 {
		return ""
	}
	s := strings.TrimSpace(string(buf[:n]))
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// bannerPorts 主动推 Banner 的常见端口（按优先级）。
var bannerPorts = []int{22, 21, 25, 465, 587, 110, 143, 993, 995, 23}

// BannerProbePorts 返回开放端口中值得抓 Banner 的子集（最多 5 个，按 bannerPorts 优先级）。
func BannerProbePorts(openPorts []int) []int {
	open := map[int]bool{}
	for _, p := range openPorts {
		open[p] = true
	}
	var out []int
	for _, p := range bannerPorts {
		if open[p] {
			out = append(out, p)
			if len(out) >= 5 {
				break
			}
		}
	}
	return out
}

// GuessOS 综合开放端口 / Banner / TTL 推测操作系统。
// openPorts: 扫描出的开放端口；banners: 端口 → 抓取的 Banner；ttl/ttlOK: ICMP 结果。
func GuessOS(openPorts []int, banners map[int]string, ttl int, ttlOK bool) (guess, basis string) {
	open := map[int]bool{}
	for _, p := range openPorts {
		open[p] = true
	}
	var ev []string // 依据

	// 1) Banner 指纹（最可信）
	unixHint, winHint := false, false
	distro := ""
	for _, p := range bannerPorts {
		b := banners[p]
		if b == "" {
			continue
		}
		short := b
		if len(short) > 48 {
			short = short[:48] + "…"
		}
		lb := strings.ToLower(b)
		switch {
		case strings.Contains(lb, "windows") || strings.Contains(lb, "microsoft"):
			winHint = true
			ev = append(ev, "端口 "+fmt.Sprint(p)+" Banner: "+short)
		case strings.Contains(lb, "ubuntu"):
			unixHint = true
			distro = "Ubuntu"
			ev = append(ev, "端口 "+fmt.Sprint(p)+" Banner: "+short)
		case strings.Contains(lb, "debian"):
			unixHint = true
			distro = "Debian"
			ev = append(ev, "端口 "+fmt.Sprint(p)+" Banner: "+short)
		case strings.Contains(lb, "centos"):
			unixHint = true
			distro = "CentOS"
			ev = append(ev, "端口 "+fmt.Sprint(p)+" Banner: "+short)
		case strings.Contains(lb, "redhat") || strings.Contains(lb, "rhel"):
			unixHint = true
			distro = "RHEL"
			ev = append(ev, "端口 "+fmt.Sprint(p)+" Banner: "+short)
		case strings.Contains(lb, "openssh") || strings.Contains(lb, "dropbear") || strings.Contains(lb, "proftpd") || strings.Contains(lb, "postfix"):
			unixHint = true
			ev = append(ev, "端口 "+fmt.Sprint(p)+" Banner: "+short)
		}
	}

	// 2) 端口特征
	if open[3389] {
		winHint = true
		ev = append(ev, "RDP(3389) 开放")
	}
	if open[445] && !open[22] {
		winHint = true
		ev = append(ev, "SMB(445) 开放且无 SSH")
	}
	if open[22] {
		unixHint = true
	}

	// 3) TTL 启发式：64 → Linux/Unix，128 → Windows，255 → 网络设备
	ttlGuess := ""
	if ttlOK {
		ev = append(ev, fmt.Sprintf("TTL %d", ttl))
		switch {
		case ttl <= 32:
			ttlGuess = "老旧网络设备/Solaris"
		case ttl <= 64:
			ttlGuess = "Linux/Unix"
		case ttl <= 128:
			ttlGuess = "Windows"
		default:
			ttlGuess = "网络设备"
		}
	}

	// 4) 综合：Banner 明确指认 > TTL 与端口特征一致 > 单一特征
	switch {
	case strings.Contains(strings.ToLower(strings.Join(ev, " ")), "windows") && (winBannerSeen(ev) || (ttlOK && ttl <= 128 && ttl > 64)):
		guess = "Windows"
	case winHint && ttlOK && ttl > 64 && ttl <= 128:
		guess = "Windows"
	case unixHint && (ttlOK && ttl <= 64):
		if distro != "" {
			guess = "Linux (" + distro + ")"
		} else {
			guess = "Linux/Unix"
		}
	case ttlGuess == "Windows" && winHint:
		guess = "Windows"
	case ttlGuess == "Linux/Unix":
		guess = "Linux/Unix"
	case unixHint:
		guess = "Linux/Unix"
	case winHint:
		guess = "Windows"
	case ttlGuess != "":
		guess = ttlGuess
	default:
		guess = "未知"
	}
	if len(ev) == 0 {
		return guess, "无特征依据（ICMP 被过滤且无 Banner）"
	}
	return guess, strings.Join(ev, " · ")
}

func winBannerSeen(ev []string) bool {
	for _, e := range ev {
		l := strings.ToLower(e)
		if strings.Contains(l, "windows") || strings.Contains(l, "microsoft") {
			return true
		}
	}
	return false
}
