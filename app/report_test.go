package app

import (
	"os"
	"testing"
	"time"

	"dbping/internal/model"
)

// TestSummaryReport 生成汇总报告样张（含概览），便于人工检查 HTML 输出。
func TestSummaryReport(t *testing.T) {
	pg := model.Conn{ID: "t1", Name: "测试PG", Type: "pg", Host: "192.168.31.58", Port: 5432}
	fake := model.Conn{ID: "t2", Name: "故障Oracle", Type: "oracle", Host: "10.0.0.9", Port: 1521}

	rows := []*sumRow{
		{
			conn: pg,
			checked: true,
			res: model.PingResult{ConnID: "t1", OK: true, Stage: "ok", TCPMs: 5, SQLMs: 8, CheckedAt: time.Now().UnixMilli()},
			ov: &model.Overview{
				Version: "15.18", VersionFull: "PostgreSQL 15.18 (Debian) on x86_64-pc-linux-gnu",
				Instance: "15.18", Hostname: "pg01", Port: 5432, OS: "Linux x86_64",
				Role: "主库（可写）", ReadOnly: "否", ArchiveMode: "归档未开启（仅 WAL: replica）",
				Charset: "UTF8", Timezone: "Asia/Shanghai", DBSize: "8583 kB", TableCount: "6",
				ConnCur: "6", ConnMax: "100", Cluster: "",
			},
		},
		{
			conn: fake,
			checked: true,
			res: model.PingResult{ConnID: "t2", OK: false, Stage: "tcp", ErrorKind: "network", Error: "[网络不通] dial tcp 10.0.0.9:1521: actively refused"},
		},
		{
			conn: model.Conn{ID: "t3", Name: "未检测MySQL", Type: "mysql", Host: "10.0.0.8", Port: 3306},
		},
	}

	html := renderSummaryReport(rows, time.Now())
	out := `C:\Users\excel\ZCodeProject\dbping\build\summary_test.html`
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Log("written:", out)
}
