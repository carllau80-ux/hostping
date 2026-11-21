package hostinsp

import "testing"

func TestParsePortSpec(t *testing.T) {
	cases := []struct {
		in      string
		want    []int
		wantErr bool
	}{
		{"22", []int{22}, false},
		{"22,80,443", []int{22, 80, 443}, false},
		{"1-3", []int{1, 2, 3}, false},
		{"80,1-3,80", []int{80, 1, 2, 3}, false}, // 去重
		{" 22 , 80 ", []int{22, 80}, false},
		{"", nil, false},
		{"0", nil, true},
		{"65536", nil, true},
		{"5-2", nil, true},
		{"1-99999999", nil, true},
		{"abc", nil, true},
	}
	for _, c := range cases {
		got, err := ParsePortSpec(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParsePortSpec(%q) 期望报错，实际 got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePortSpec(%q) 意外报错: %v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("ParsePortSpec(%q) = %v, 期望 %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParsePortSpec(%q)[%d] = %d, 期望 %d", c.in, i, got[i], c.want[i])
			}
		}
	}
}
