package bmc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dbping/internal/model"

	ipmi "github.com/bougou/go-ipmi/pkg/client"
	ipmitypes "github.com/bougou/go-ipmi/pkg/types"
)

// collectFRU 采集 FRU 设备清单（机箱/板卡/产品信息区）。
func collectFRU(cli *ipmi.Client, r *model.BMCReport) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	frus, err := cli.GetFRUs(ctx)
	if err != nil {
		r.FRUNote = "FRU 读取失败：" + shortErr(err)
		return
	}
	if len(frus) == 0 {
		r.FRUNote = "未发现 FRU 设备"
		return
	}
	skip := 0
	for _, fru := range frus {
		if fru == nil {
			continue
		}
		if fru.DeviceNotPresent {
			skip++
			continue
		}
		name := fru.DeviceName
		if name == "" {
			name = fmt.Sprintf("FRU #%d", fru.DeviceID)
		}
		rows := fruRows(fru)
		if len(rows) == 0 {
			skip++
			continue
		}
		r.FRU = append(r.FRU, model.KV{Key: "__fru", Value: name})
		r.FRU = append(r.FRU, rows...)
	}
	if len(r.FRU) == 0 {
		r.FRUNote = "FRU 设备存在但无信息区"
		return
	}
	if skip > 0 {
		r.FRUNote = fmt.Sprintf("另有 %d 个 FRU 设备不存在/为空", skip)
	}
}

// fruRows 展开单个 FRU 的信息区为 KV 行（"__fru" 开头的 Key 表示设备名分隔行）。
func fruRows(fru *ipmitypes.FRU) []model.KV {
	var rows []model.KV
	if a := fru.ChassisInfoArea; a != nil {
		rows = appendKV(rows, "部件号", a.PartNumber)
		rows = appendKV(rows, "序列号", a.SerialNumber)
	}
	if a := fru.BoardInfoArea; a != nil {
		rows = appendKV(rows, "板卡厂商", a.Manufacturer)
		rows = appendKV(rows, "板卡型号", a.ProductName)
		rows = appendKV(rows, "板卡序列号", a.SerialNumber)
		rows = appendKV(rows, "板卡部件号", a.PartNumber)
		if !a.MfgDateTime.IsZero() && a.MfgDateTime.Year() > 1996 {
			rows = append(rows, model.KV{Key: "板卡生产日期", Value: a.MfgDateTime.Format("2006-01-02")})
		}
	}
	if a := fru.ProductInfoArea; a != nil {
		rows = appendKV(rows, "产品厂商", a.Manufacturer)
		rows = appendKV(rows, "产品名称", a.Name)
		rows = appendKV(rows, "产品型号", a.PartModel)
		rows = appendKV(rows, "产品版本", a.Version)
		rows = appendKV(rows, "产品序列号", a.SerialNumber)
		rows = appendKV(rows, "资产标签", a.AssetTag)
	}
	return rows
}

func appendKV(rows []model.KV, key string, val []byte) []model.KV {
	v := strings.TrimRight(string(val), "\x00 \t")
	if strings.TrimSpace(v) == "" {
		return rows
	}
	return append(rows, model.KV{Key: key, Value: v})
}
