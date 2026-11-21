package hostinsp

import "testing"

func TestGuessOS(t *testing.T) {
	cases := []struct {
		name      string
		openPorts []int
		banners   map[int]string
		ttl       int
		ttlOK     bool
		want      string
	}{
		{"Linux: 22开放+TTL64", []int{22, 80}, nil, 63, true, "Linux/Unix"},
		{"Windows: 3389开放+TTL128", []int{3389, 445}, nil, 127, true, "Windows"},
		{"Windows banner", []int{22}, map[int]string{22: "SSH-2.0-OpenSSH_for_Windows_8.1"}, 127, true, "Windows"},
		{"Ubuntu banner", []int{22}, map[int]string{22: "SSH-2.0-OpenSSH_8.4p1 Ubuntu-5"}, 64, true, "Linux (Ubuntu)"},
		{"仅TTL128", []int{80}, nil, 128, true, "Windows"},
		{"仅TTL255", []int{23}, nil, 255, true, "网络设备"},
		{"无任何依据", []int{8080}, nil, 0, false, "未知"},
		{"ICMP过滤但22开放", []int{22}, nil, 0, false, "Linux/Unix"},
	}
	for _, c := range cases {
		got, _ := GuessOS(c.openPorts, c.banners, c.ttl, c.ttlOK)
		if got != c.want {
			t.Errorf("GuessOS(%s) = %q, 期望 %q", c.name, got, c.want)
		}
	}
}
