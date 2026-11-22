package bmc

import (
	"strings"
	"testing"

	sensorpkg "github.com/bougou/go-ipmi/pkg/command/sensor"
)

// sensorStatusFromFlags 状态矩阵：LNR/LCR/UNR/UCR → critical；LNC/UNC → warning；其余 nominal。
func TestSensorStatusFromFlags(t *testing.T) {
	cases := []struct {
		name     string
		reading  sensorpkg.GetSensorReadingResponse
		fallback string
		want     string
	}{
		{"无标志位", sensorpkg.GetSensorReadingResponse{}, "nominal", "nominal"},
		{"超过上限严重", sensorpkg.GetSensorReadingResponse{Above_UCR: true}, "nominal", "critical"},
		{"超过上限不可恢复", sensorpkg.GetSensorReadingResponse{Above_UNR: true}, "nominal", "critical"},
		{"低于下限严重", sensorpkg.GetSensorReadingResponse{Below_LCR: true}, "nominal", "critical"},
		{"低于下限不可恢复", sensorpkg.GetSensorReadingResponse{Below_LNR: true}, "nominal", "critical"},
		{"超过上限警告", sensorpkg.GetSensorReadingResponse{Above_UNC: true}, "nominal", "warning"},
		{"低于下限警告", sensorpkg.GetSensorReadingResponse{Below_LNC: true}, "nominal", "warning"},
		{"警告不覆盖已有 critical", sensorpkg.GetSensorReadingResponse{Above_UNC: true}, "critical", "critical"},
	}
	for _, c := range cases {
		if got := sensorStatusFromFlags(&c.reading, c.fallback); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSensorTypeName(t *testing.T) {
	if sensorTypeName(0x01) != "温度" || sensorTypeName(0x04) != "风扇" || sensorTypeName(0x08) != "电源模块" {
		t.Errorf("常见类型映射错误")
	}
	if !strings.Contains(sensorTypeName(0xEE), "0x") {
		t.Errorf("未知类型应回退码值显示")
	}
}

func TestThresholdHelpers(t *testing.T) {
	if appendSep("", "") != "" {
		t.Errorf("appendSep 双空应返回空")
	}
	if appendSep("a", "b") != "a / b" {
		t.Errorf("appendSep 拼接错误")
	}
	if appendSep("a", "") != "a" {
		t.Errorf("appendSep 单侧拼接错误")
	}
}
