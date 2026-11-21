//go:build windows

// Package winicon 运行时为主窗口设置图标（Wails 窗口类不读取 exe 资源图标，
// 通过 WM_SETICON 注入，任务栏与标题栏同步生效）。
package winicon

import (
	_ "embed"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

//go:embed icon.ico
var iconBytes []byte

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procLoadImageW               = user32.NewProc("LoadImageW")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
)

const (
	imageIcon     = 1
	lrLoadFromFile = 0x10
	lrDefaultSize  = 0x40
	wmSetIcon      = 0x80
	iconSmall      = 0
	iconBig        = 1
	gwlExStyle     = -20
	wsExToolWindow = 0x80
	smCxSmIcon     = 49
	smCySmIcon     = 50

	// gwlExStylePtr = -20 的 uintptr 补码表示（常量需可表示于 uintptr）
	gwlExStylePtr = ^uintptr(19)
)

// hwndOfCurrentProcess 找到当前进程的主顶层窗口（可见且非工具窗口）。
// opts: waited=false 只找一次；true 时轮询等待窗口出现（OnStartup 时窗口可能尚未显示）。
func hwndOfCurrentProcess(waited bool) uintptr {
	deadline := time.Now().Add(3 * time.Second)
	for {
		var found uintptr
		cb := syscall.NewCallback(func(hwnd, lparam uintptr) uintptr {
			var pid uint32
			procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid == uint32(os.Getpid()) {
				vis, _, _ := procIsWindowVisible.Call(hwnd)
				if vis != 0 {
					ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStylePtr)
					if ex&wsExToolWindow == 0 {
						found = hwnd
						return 0 // 停止枚举
					}
				}
			}
			return 1
		})
		procEnumWindows.Call(cb, 0)
		if found != 0 || !waited || time.Now().After(deadline) {
			return found
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// SetMainWindowIcon 为本进程主窗口设置大/小图标（来源：内嵌 icon.ico）。
// 窗口在 OnStartup 时刻可能尚未显示，带重试；失败返回错误由调用方记录。
func SetMainWindowIcon() error {
	if len(iconBytes) == 0 {
		return fmt.Errorf("内嵌图标为空")
	}
	hwnd := hwndOfCurrentProcess(true)
	if hwnd == 0 {
		return fmt.Errorf("未找到主窗口")
	}
	tmp, err := os.CreateTemp("", "dbping-*.ico")
	if err != nil {
		return err
	}
	path := tmp.Name()
	if _, err := tmp.Write(iconBytes); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	defer os.Remove(path)

	p16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	cxS, _, _ := procGetSystemMetrics.Call(smCxSmIcon)
	cyS, _, _ := procGetSystemMetrics.Call(smCySmIcon)

	hBig, _, callErr := procLoadImageW.Call(0, uintptr(unsafe.Pointer(p16)), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
	if hBig == 0 {
		return fmt.Errorf("LoadImage 失败: %v", callErr)
	}
	hSmall, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(p16)), imageIcon, cxS, cyS, lrLoadFromFile)
	procSendMessageW.Call(hwnd, wmSetIcon, iconBig, hBig)
	procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, hSmall)
	return nil
}
