//go:build !windows

package winicon

// SetMainWindowIcon 非 Windows 平台占位（macOS/Linux 由打包工具处理应用图标）。
func SetMainWindowIcon() error { return nil }
