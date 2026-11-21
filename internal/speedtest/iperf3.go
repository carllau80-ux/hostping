// Package speedtest 纯 Go 实现的 iperf3 客户端（TCP 上行/下行带宽测试）。
// 协议依据 iperf3 官方源码（src/iperf_api.c / iperf_client_api.c / iperf_tcp.c）还原：
//   - 控制通道：TCP 连接，消息为 [4 字节大端长度][JSON]（参数与结果交换）
//   - 状态驱动：服务端在控制通道发送 1 字节状态码（PARAM_EXCHANGE=9、CREATE_STREAMS=10、
//     TEST_START=1、TEST_RUNNING=2、EXCHANGE_RESULTS=13、DISPLAY_RESULTS=14、IPERF_DONE=16）
//   - 数据连接：每条先发送 37 字节 cookie（36 字符 UUID + '\0'），此后为原始字节流（128KB 块）
//   - 结束：客户端发送 1 字节 TEST_END=4；随后 EXCHANGE_RESULTS 阶段客户端先发后收结果 JSON
package speedtest

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// 状态码（iperf_api.h）
const (
	stateTestStart       = 1
	stateTestRunning     = 2
	stateTestEnd         = 4
	stateParamExchange   = 9
	stateCreateStreams   = 10
	stateServerTerminate = 11
	stateExchangeResults = 13
	stateDisplayResults  = 14
	stateIperfDone       = 16
	stateAccessDenied    = -1
	stateServerError     = -2
)

const (
	cookieSize   = 37            // 36 字符 UUID + '\0'
	tcpBlockSize = 128 * 1024    // 默认 TCP 块大小（DEFAULT_TCP_BLKSIZE）
)

// Debug 协议调试日志（stderr）。
var Debug = false

func dbg(format string, args ...any) {
	if Debug {
		fmt.Fprintf(os.Stderr, "[iperf3] "+format+"\n", args...)
	}
}

// Interval 每秒区间速率。
type Interval struct {
	Sec     int     `json:"sec"`     // 区间序号（1 起）
	MBps    float64 `json:"mbps"`    // 该秒速率（Mbps）
	Retrans int64   `json:"retrans"` // 重传（尽力采集）
	Omitted bool    `json:"omitted"` // 预热（omit）阶段，不计入汇总
}

// Result 测速汇总。
type Result struct {
	Direction  string     `json:"direction"`  // up / down
	Seconds    float64    `json:"seconds"`    // 实测时长（秒，不含预热）
	Mbps       float64    `json:"mbps"`       // 本端测得速率（上行=发送 / 下行=接收）
	ServerMbps float64    `json:"serverMbps"` // 对端报告的速率
	Retrans    int64      `json:"retrans"`    // 重传（下行来自服务端报告；上行本端不可得为 -1）
	Warning    string     `json:"warning,omitempty"` // 结果交换失败时的提示（实测数据仍有效）
	Intervals  []Interval `json:"intervals"`
}

// progressFn 每秒进度回调。
type progressFn func(iv Interval)

// RunTest 连接 iperf3 服务端执行一次 TCP 带宽测试。
// server: 服务端地址；port: 服务端口（默认 5201）；duration: 测试秒数；
// omit: 预热秒数（不计入汇总）；parallel: 并行流数；reverse: true=下行（服务端发送）。
func RunTest(server string, port, duration, omit, parallel int, reverse bool, progress progressFn) (*Result, error) {
	if parallel < 1 {
		parallel = 1
	}
	if parallel > 32 {
		parallel = 32
	}
	if duration < 1 {
		duration = 1
	}
	dir := "up"
	if reverse {
		dir = "down"
	}
	overall := time.Duration(duration+omit+30) * time.Second

	ctrl, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", server, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接测速服务端失败: %w", err)
	}
	defer ctrl.Close()
	if tc, ok := ctrl.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	_ = ctrl.SetDeadline(time.Now().Add(overall))

	cookie := makeCookie()
	cookieWire := append([]byte(cookie), 0) // 36 字符 + '\0' = 37 字节
	// 控制连接建立后立即发送 cookie（服务端 iperf_accept 先读 cookie 再发 PARAM_EXCHANGE）
	if _, err := ctrl.Write(cookieWire); err != nil {
		return nil, fmt.Errorf("发送 cookie 失败: %w", err)
	}
	dbg("已发送 cookie（%d 字节）", len(cookieWire))

	// 状态序列（顺序严格；控制状态消息为 1 字节）
	expect := func(want byte) error {
		var b [1]byte
		if _, err := readFull(ctrl, b[:]); err != nil {
			return fmt.Errorf("读取状态失败: %w", err)
		}
		st := int8(b[0])
		dbg("收到状态 %d（期望 %d）", st, want)
		switch st {
		case int8(want):
			return nil
		case stateServerTerminate:
			return fmt.Errorf("服务端终止了测试（服务端可能不支持当前参数或已达上限）")
		case stateAccessDenied:
			return fmt.Errorf("服务端拒绝访问（cookie 校验失败）")
		case stateServerError:
			var e [8]byte
			_, _ = readFull(ctrl, e[:])
			return fmt.Errorf("服务端错误（errno=%d）", int32(binary.BigEndian.Uint32(e[4:8])))
		default:
			return fmt.Errorf("意外的状态: %d（期望 %d）", st, want)
		}
	}

	// 1) PARAM_EXCHANGE：发送本端参数（4B 大端长度 + JSON）
	if err := expect(stateParamExchange); err != nil {
		return nil, err
	}
	params := map[string]any{
		"tcp":            true,
		"omit":           omit,
		"time":           duration,
		"num":            0,
		"blockcount":     0,
		"parallel":       parallel,
		"len":            tcpBlockSize,
		"port":           port,
		"client_version": "1.2.0",
	}
	if reverse {
		params["reverse"] = true
	}
	if err := writeJSON(ctrl, params); err != nil {
		return nil, fmt.Errorf("发送参数失败: %w", err)
	}
	// 服务端不回参数 JSON，直接进入状态字节（CREATE_STREAMS）

	// 2) CREATE_STREAMS：打开数据连接，每条先发 37 字节 cookie
	if err := expect(stateCreateStreams); err != nil {
		return nil, err
	}
	conns := make([]net.Conn, 0, parallel)
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	for i := 0; i < parallel; i++ {
		dc, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", server, port), 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("打开数据连接 %d/%d 失败: %w", i+1, parallel, err)
		}
		conns = append(conns, dc)
		if _, err := dc.Write(cookieWire); err != nil {
			return nil, fmt.Errorf("发送 cookie 失败: %w", err)
		}
	}

	// 3) TEST_START
	if err := expect(stateTestStart); err != nil {
		return nil, err
	}
	// 4) TEST_RUNNING：开始传输
	if err := expect(stateTestRunning); err != nil {
		return nil, err
	}

	total := omit + duration
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 每流字节数 + 全局累计
	perStream := make([]int64, parallel)
	var bytesTotal int64
	var blockCount int64
	block := make([]byte, tcpBlockSize)
	for i := range block {
		block[i] = byte(i%256) + 1 // 避免全零（防中间设备压缩）
	}
	buf := make([]byte, tcpBlockSize)

	// 每秒采样：累计字节快照
	mu := sync.Mutex{}
	var snap []int64
	sampleStop := make(chan struct{})
	sampleDone := make(chan struct{})
	go func() {
		defer close(sampleDone)
		tick := time.NewTicker(1 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-sampleStop:
				return
			case <-tick.C:
				cur := atomic.LoadInt64(&bytesTotal)
				mu.Lock()
				snap = append(snap, cur)
				mu.Unlock()
			}
		}
	}()

	if !reverse {
		// 上行：每条流持续发送 128KB 块
		for i, dc := range conns {
			wg.Add(1)
			go func(idx int, c net.Conn) {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					_ = c.SetWriteDeadline(time.Now().Add(3 * time.Second))
					n, err := c.Write(block)
					if n > 0 {
						atomic.AddInt64(&bytesTotal, int64(n))
						atomic.AddInt64(&blockCount, 1)
						atomic.AddInt64(&perStream[idx], int64(n))
					}
					if err != nil {
						if isStop(err) {
							return
						}
						return
					}
				}
			}(i, dc)
		}
	} else {
		// 下行：每条流持续读取（服务端发送，结束时关闭连接 → EOF）
		for i, dc := range conns {
			wg.Add(1)
			go func(idx int, c net.Conn) {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
					n, err := c.Read(buf)
					if n > 0 {
						atomic.AddInt64(&bytesTotal, int64(n))
						atomic.AddInt64(&perStream[idx], int64(n))
					}
					if err != nil {
						if isStop(err) {
							return
						}
						return // EOF 或其它错误：该流结束
					}
				}
			}(i, dc)
		}
	}

	// 传输阶段运行 omit + duration 秒
	time.Sleep(time.Duration(total) * time.Second)
	close(stop)
	wg.Wait()
	close(sampleStop)
	<-sampleDone

	// 计算区间（相邻累计快照差值）
	mu.Lock()
	intervals := make([]Interval, 0, len(snap))
	var dataBytes int64
	var dataSecs int
	for i, v := range snap {
		prev := int64(0)
		if i > 0 {
			prev = snap[i-1]
		}
		delta := v - prev
		if delta < 0 {
			delta = 0
		}
		omitted := i < omit
		iv := Interval{Sec: i + 1, MBps: float64(delta) * 8 / 1e6, Omitted: omitted}
		if !omitted {
			dataBytes += delta
			dataSecs++
		}
		intervals = append(intervals, iv)
		if progress != nil {
			progress(iv)
		}
	}
	mu.Unlock()
	if dataSecs == 0 && len(snap) > 0 {
		dataBytes = snap[len(snap)-1]
		dataSecs = len(snap) - omit
		if dataSecs < 1 {
			dataSecs = 1
		}
	}
	res := &Result{Direction: dir, Intervals: intervals}
	if dataSecs > 0 {
		res.Seconds = float64(dataSecs)
		res.Mbps = float64(dataBytes) * 8 / 1e6 / float64(dataSecs)
	}

	// 5) 结束：控制通道发送 1 字节 TEST_END
	if err := ctrl.SetDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return nil, fmt.Errorf("设置控制超时失败: %w", err)
	}
	if _, err := ctrl.Write([]byte{byte(stateTestEnd)}); err != nil {
		return nil, fmt.Errorf("发送 TEST_END 失败: %w", err)
	}
	dbg("已发送 TEST_END，等待 EXCHANGE_RESULTS…")

	// 6) EXCHANGE_RESULTS：先发本端结果，再读对端结果。
	//    交换失败（如非标准 iperf3 服务端、stream id 不兼容）不视为测速失败：
	//    数据已经传输完成，本端实测结果依然有效。
	exchangeOK := true
	var srvJSON map[string]any
	if err := expect(stateExchangeResults); err != nil {
		dbg("结果交换阶段异常: %v", err)
		res.Warning = "服务端结果交换失败（可能为非标准 iperf3 服务端），已使用本端实测结果"
		exchangeOK = false
	}
	if exchangeOK {
		streams := []map[string]any{}
		for i := range conns {
			// 流 ID 必须复刻 iperf3 iperf_add_stream 的怪异规则：
			// 首个流 id=1，后续 id = 已有流数量 + 2（即 1, 3, 4, 5, 6...）
			id := 1
			if i > 0 {
				id = i + 2
			}
			streams = append(streams, map[string]any{
				"id":              id,
				"bytes":           atomic.LoadInt64(&perStream[i]),
				"retransmits":     -1,
				"jitter":          0,
				"errors":          0,
				"omitted_errors":  0,
				"packets":         atomic.LoadInt64(&perStream[i]) / tcpBlockSize,
				"omitted_packets": 0,
				"start_time":      0,
				"end_time":        res.Seconds,
			})
		}
		own := map[string]any{
			"cpu_util_total":         0,
			"cpu_util_user":          0,
			"cpu_util_system":        0,
			"sender_has_retransmits": 0,
			"streams":                streams,
		}
		if err := writeJSON(ctrl, own); err != nil {
			dbg("发送本端结果失败: %v", err)
			res.Warning = "服务端结果交换失败（连接中断），已使用本端实测结果"
			exchangeOK = false
		} else {
			srvJSON, err = readJSON(ctrl)
			if err != nil {
				dbg("读取服务端结果失败: %v", err)
				res.Warning = "服务端结果交换失败（连接中断），已使用本端实测结果"
				exchangeOK = false
			}
		}
	}

	// 7) DISPLAY_RESULTS：客户端自行进入完成状态（服务端不发 IPERF_DONE）
	if exchangeOK {
		if err := expect(stateDisplayResults); err != nil {
			dbg("DISPLAY_RESULTS 异常: %v", err)
		}
	}

	parseServerResults(srvJSON, reverse, res)
	return res, nil
}

// parseServerResults 解析服务端结果 JSON，填充对端速率与重传。
func parseServerResults(j map[string]any, reverse bool, res *Result) {
	if j == nil {
		return
	}
	var totalBytes int64
	var totalTime float64
	var retrans int64
	if streams, ok := j["streams"].([]any); ok {
		for _, s := range streams {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			if b, ok := sm["bytes"].(float64); ok {
				totalBytes += int64(b)
			}
			if rt, ok := sm["retransmits"].(float64); ok && rt > 0 {
				retrans += int64(rt)
			}
			st, stok := sm["start_time"].(float64)
			et, etok := sm["end_time"].(float64)
			if stok && etok && et > st {
				if d := et - st; d > totalTime {
					totalTime = d
				}
			}
		}
	}
	if totalTime <= 0 {
		totalTime = res.Seconds
	}
	if totalTime > 0 {
		res.ServerMbps = float64(totalBytes) * 8 / 1e6 / totalTime
	}
	if reverse {
		res.Retrans = retrans // 下行：服务端为发送方，重传由服务端统计
	} else {
		res.Retrans = -1 // 上行：重传为本端 TCP 统计，Windows 下不可得
	}
}

// ---------- 底层收发 ----------

func readFull(c net.Conn, buf []byte) (int, error) {
	got := 0
	for got < len(buf) {
		n, err := c.Read(buf[got:])
		got += n
		if err != nil {
			return got, err
		}
	}
	return got, nil
}

// writeJSON 4 字节大端长度前缀 + JSON 文本（对应 iperf3 JSON_write）。
func writeJSON(c net.Conn, obj map[string]any) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	dbg("发送 JSON(%d 字节): %s", len(data), string(data))
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(data)))
	if _, err := c.Write(size); err != nil {
		return err
	}
	_, err = c.Write(data)
	return err
}

// readJSON 读取 4 字节大端长度前缀 + JSON（对应 iperf3 JSON_read）。
func readJSON(c net.Conn) (map[string]any, error) {
	var size [4]byte
	if _, err := readFull(c, size[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n == 0 || n > 8<<20 {
		return nil, fmt.Errorf("非法的 JSON 长度: %d", n)
	}
	data := make([]byte, n)
	if _, err := readFull(c, data); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return out, nil
}

// makeCookie 生成 36 字符 UUID 风格 cookie（等价 iperf3 make_cookie）。
func makeCookie() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

// isStop 判断错误是否可安全退出（停止信号、EOF、连接关闭等；超时除外——继续传输）。
func isStop(err error) bool {
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return false // 超时且未收到停止信号：继续传输
	}
	return true
}
