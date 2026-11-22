package hostinsp

import "testing"

// zfs list -H -o name,used,avail,mountpoint 基础行（制表符分隔，parseStorcliLines 解析）。
const zfsBaseOut = "tank\t5.2T\t5.7T\t/tank\ntank/data\t1.1T\t3.9T\t/tank/data\n"

// zfs get -H -o name,property,value compress,compressratio,dedup,dedupratio 属性行。
const zfsPropsOut = "tank\tcompress\toff\n" +
	"tank\tcompressratio\t1.00x\n" +
	"tank\tdedup\toff\n" +
	"tank\tdedupratio\t1.00x\n" +
	"tank/data\tcompress\tlz4\n" +
	"tank/data\tcompressratio\t2.10x\n" +
	"tank/data\tdedup\ton\n" +
	"tank/data\tdedupratio\t1.25x\n"

func TestMergeZFSDatasets(t *testing.T) {
	base := parseStorcliLines(zfsBaseOut, 4)
	props := parseStorcliLines(zfsPropsOut, 3)
	rows := mergeZFSDatasets(base, props)
	if len(rows) != 2 {
		t.Fatalf("行数 = %d, 期望 2", len(rows))
	}
	// tank: 压缩 off、重删 off
	r0 := rows[0]
	if r0[0] != "tank" || r0[4] != "off" || r0[6] != "off" {
		t.Errorf("tank 行 = %v", r0)
	}
	// tank/data: 压缩算法 lz4（开启）、重删 on、压缩比/重删比正确
	r1 := rows[1]
	if r1[0] != "tank/data" {
		t.Errorf("名称 = %q", r1[0])
	}
	if r1[4] != "lz4" {
		t.Errorf("压缩算法 = %q, 期望 lz4", r1[4])
	}
	if r1[5] != "2.10x" {
		t.Errorf("压缩比 = %q", r1[5])
	}
	if r1[6] != "on" {
		t.Errorf("重删 = %q", r1[6])
	}
	if r1[7] != "1.25x" {
		t.Errorf("重删比 = %q", r1[7])
	}
	// 挂载点列对齐（第 4 列为路径）
	if r1[3] != "/tank/data" {
		t.Errorf("挂载点 = %q", r1[3])
	}
}

func TestMergeZFSDatasetsEmpty(t *testing.T) {
	if rows := mergeZFSDatasets(nil, nil); len(rows) != 0 {
		t.Errorf("空输入应返回空")
	}
	base := parseStorcliLines(zfsBaseOut, 4)
	rows := mergeZFSDatasets(base, nil)
	if len(rows) != 2 || rows[0][4] != "" {
		t.Errorf("无属性行时压缩列应为空: %v", rows[0])
	}
}
