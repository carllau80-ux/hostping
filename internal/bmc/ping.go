package bmc

import (
	"net"
	"strconv"
	"time"
)

// rmcpPingPayload RMCP Presence Ping（ASF 规范）：版本 0x06 / 序列 0xFF / 类 0x06，
// IANA 企业号 4542 + 消息类型 0x80（presence ping）。BMC 存在即回 pong（0x40）。
var rmcpPingPayload = []byte{0x06, 0x00, 0xFF, 0x06, 0x00, 0x00, 0x11, 0xBE, 0x80, 0x00, 0x00, 0x00}

// rmcpPing 发送 RMCP Presence Ping 探测 BMC 是否可达（任何 UDP 回包即视为存活）。
// 用于区分"完全无响应"（IPMI over LAN 未开启/IP 错误/防火墙）与"有响应但会话失败"。
func rmcpPing(host string, port int, timeout time.Duration) bool {
	raddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(rmcpPingPayload); err != nil {
		return false
	}
	buf := make([]byte, 512)
	// 最多读 3 个包：任意 RMCP 回包（pong/RAKP 错误）都说明 BMC 在监听 623
	for i := 0; i < 3; i++ {
		n, err := conn.Read(buf)
		if err != nil {
			return false
		}
		if n >= 4 && buf[0] == 0x06 {
			return true
		}
	}
	return false
}
