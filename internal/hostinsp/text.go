package hostinsp

import "strings"

// pickField 从 "KEY=VALUE" 风格文本中取指定键的值。
func pickField(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key) {
			return strings.TrimSpace(strings.TrimPrefix(line, key))
		}
	}
	return ""
}

// firstField 取文本第一个非空字段。
func firstField(text string) string {
	fs := strings.Fields(text)
	if len(fs) > 0 {
		return fs[0]
	}
	return ""
}

// firstMatch 找到以 prefix 开头的行，返回前缀之后的内容。
func firstMatch(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// countLines 统计非空行数。
func countLines(text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func shortErr(err error) string {
	s := err.Error()
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}
