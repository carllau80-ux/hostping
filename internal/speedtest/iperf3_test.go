package speedtest

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// mockIperfServer 按 iperf3 官方协议回放的最小服务端（单测用）。
// 返回服务端统计到的字节数与收到的本端结果 JSON。
type mockServer struct {
	ln           net.Listener
	received     int64      // 上行时服务端收到的数据字节
	ownResults   map[string]any // 客户端发来的结果 JSON
	durationSecs int
	reverse      bool
	parallel     int
}

func startMockServer(t *testing.T, duration, parallel int, reverse bool, downMbps float64) (*mockServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	m := &mockServer{ln: ln, durationSecs: duration, reverse: reverse, parallel: parallel}
	go m.serve(downMbps)
	time.Sleep(100 * time.Millisecond) // 等监听就绪
	return m, nil
}

func (m *mockServer) addr() net.Addr { return m.ln.Addr() }

func (m *mockServer) readJSON(c net.Conn) map[string]any {
	var size [4]byte
	if _, err := readFull(c, size[:]); err != nil {
		return nil
	}
	n := binary.BigEndian.Uint32(size[:])
	data := make([]byte, n)
	if _, err := readFull(c, data); err != nil {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}

func (m *mockServer) writeJSON(c net.Conn, obj map[string]any) {
	data, _ := json.Marshal(obj)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(data)))
	_, _ = c.Write(size)
	_, _ = c.Write(data)
}

func (m *mockServer) serve(downMbps float64) {
	ctrl, err := m.ln.Accept()
	if err != nil {
		return
	}
	defer ctrl.Close()
	// 先读控制连接 cookie（37 字节）
	cookie := make([]byte, cookieSize)
	if _, err := readFull(ctrl, cookie); err != nil {
		return
	}
	// PARAM_EXCHANGE
	_, _ = ctrl.Write([]byte{byte(stateParamExchange)})
	params := m.readJSON(ctrl)
	if params == nil {
		return
	}
	if v, ok := params["parallel"].(float64); ok {
		m.parallel = int(v)
	}
	// CREATE_STREAMS
	_, _ = ctrl.Write([]byte{byte(stateCreateStreams)})
	dataConns := make([]net.Conn, 0, m.parallel)
	for i := 0; i < m.parallel; i++ {
		dc, err := m.ln.Accept()
		if err != nil {
			return
		}
		cookie := make([]byte, cookieSize)
		if _, err := readFull(dc, cookie); err != nil {
			return
		}
		dataConns = append(dataConns, dc)
	}
	// TEST_START / TEST_RUNNING
	_, _ = ctrl.Write([]byte{byte(stateTestStart)})
	_, _ = ctrl.Write([]byte{byte(stateTestRunning)})

	deadline := time.Now().Add(time.Duration(m.durationSecs) * time.Second)

	if m.reverse {
		// 下行：服务端发送数据
		block := make([]byte, tcpBlockSize)
		for i := range block {
			block[i] = 0xAB
		}
		// 按目标速率发送：downMbps Mbps → bytes/s = downMbps*1e6/8
		perSec := int64(downMbps * 1e6 / 8)
		perTick := perSec / 10 // 100ms 一批
		var wg sync.WaitGroup
		var sent int64
		for _, dc := range dataConns {
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer c.Close()
				for time.Now().Before(deadline) {
					n, err := c.Write(block[:min(int(perTick), len(block))])
					if err != nil {
						return
					}
					atomicAdd(&sent, int64(n))
					time.Sleep(100 * time.Millisecond)
				}
				c.Close()
			}(dc)
		}
		wg.Wait()
		m.received = sent
		// 服务端发送完毕后读取客户端的 TEST_END（与真实协议一致）
		var b [1]byte
		for {
			if _, err := ctrl.Read(b[:]); err != nil {
				return
			}
			if b[0] == byte(stateTestEnd) {
				break
			}
		}
	} else {
		// 上行：服务端持续读取直到控制通道收到 TEST_END
		buf := make([]byte, tcpBlockSize)
		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				for _, dc := range dataConns {
					n, err := dc.Read(buf)
					if n > 0 {
						atomicAdd(&m.received, int64(n))
					}
					if err != nil {
						return
					}
				}
			}
		}()
		// 等控制通道的 TEST_END（1 字节）
		var b [1]byte
		for {
			if _, err := ctrl.Read(b[:]); err != nil {
				close(done)
				return
			}
			if b[0] == byte(stateTestEnd) {
				break
			}
		}
		close(done)
		time.Sleep(200 * time.Millisecond) // 等数据收尾
	}

	// EXCHANGE_RESULTS
	_, _ = ctrl.Write([]byte{byte(stateExchangeResults)})
	m.ownResults = m.readJSON(ctrl)
	// 服务端结果
	var streams []map[string]any
	for i := 0; i < m.parallel; i++ {
		streams = append(streams, map[string]any{
			"id":         i + 1,
			"bytes":      m.received / int64(m.parallel),
			"retransmits": 3,
			"jitter":     0,
			"errors":     0,
			"packets":    100,
			"start_time": 0,
			"end_time":   m.durationSecs,
		})
	}
	m.writeJSON(ctrl, map[string]any{
		"cpu_util_total":         5.0,
		"cpu_util_user":          2.0,
		"cpu_util_system":        3.0,
		"sender_has_retransmits": 1,
		"streams":                streams,
	})
	// DISPLAY_RESULTS → IPERF_DONE
	_, _ = ctrl.Write([]byte{byte(stateDisplayResults)})
	_, _ = ctrl.Write([]byte{byte(stateIperfDone)})
	time.Sleep(100 * time.Millisecond)
}

func atomicAdd(p *int64, v int64) { *p += v }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestRunTestUpload(t *testing.T) {
	m, err := startMockServer(t, 2, 2, false, 0)
	if err != nil {
		t.Skip("模拟服务端启动失败:", err)
	}
	res, err := RunTest(addrHost(m.addr()), portOf(m.addr()), 2, 0, 2, false, nil)
	if err != nil {
		t.Fatalf("上行测速失败: %v", err)
	}
	if res.Direction != "up" {
		t.Errorf("Direction = %s", res.Direction)
	}
	if res.Mbps <= 0 {
		t.Errorf("上行速率 = %v", res.Mbps)
	}
	if len(res.Intervals) < 2 {
		t.Errorf("区间数 = %d", len(res.Intervals))
	}
	if m.received <= 0 {
		t.Errorf("服务端未收到数据: %d", m.received)
	}
	if m.ownResults == nil {
		t.Fatalf("未收到客户端结果 JSON")
	}
	if _, ok := m.ownResults["streams"]; !ok {
		t.Errorf("结果 JSON 缺少 streams")
	}
	if res.ServerMbps <= 0 {
		t.Errorf("对端速率 = %v", res.ServerMbps)
	}
	if res.Retrans != -1 {
		t.Errorf("上行重传应为 -1, got %v", res.Retrans)
	}
}

func TestRunTestDownload(t *testing.T) {
	m, err := startMockServer(t, 2, 1, true, 200)
	if err != nil {
		t.Skip("模拟服务端启动失败:", err)
	}
	res, err := RunTest(addrHost(m.addr()), portOf(m.addr()), 2, 0, 1, true, nil)
	if err != nil {
		t.Fatalf("下行测速失败: %v", err)
	}
	if res.Direction != "down" {
		t.Errorf("Direction = %s", res.Direction)
	}
	if res.Mbps <= 0 {
		t.Errorf("下行速率 = %v", res.Mbps)
	}
	if res.Retrans != 3 {
		t.Errorf("下行重传应来自服务端 = 3, got %v", res.Retrans)
	}
	if res.ServerMbps <= 0 {
		t.Errorf("对端速率 = %v", res.ServerMbps)
	}
}

func TestMakeCookie(t *testing.T) {
	c := makeCookie()
	if len(c) != 36 {
		t.Errorf("cookie 长度 = %d, 期望 36", len(c))
	}
	if c[14] != '4' {
		t.Errorf("UUID version 位错误: %s", c)
	}
	if c[19] != '8' && c[19] != '9' && c[19] != 'a' && c[19] != 'b' {
		t.Errorf("UUID variant 位错误: %s", c)
	}
}

func addrHost(a net.Addr) string { return "127.0.0.1" }
func portOf(a net.Addr) int {
	// addr 形如 127.0.0.1:port
	s := a.String()
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			p := 0
			for j := i + 1; j < len(s); j++ {
				p = p*10 + int(s[j]-'0')
			}
			return p
		}
	}
	return 0
}

var _ = fmt.Sprintf
