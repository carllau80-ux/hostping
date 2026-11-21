# rect.ps1 输出 HostPing 主窗口屏幕矩形并置前
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class W2 {
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hwnd, out RECT r);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
}
"@
$p = Get-Process -Name HostPing -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
if (-not $p) { Write-Output "no window"; exit 1 }
$r = New-Object W2+RECT
[W2]::GetWindowRect($p.MainWindowHandle, [ref]$r) | Out-Null
[W2]::SetForegroundWindow($p.MainWindowHandle) | Out-Null
Write-Output "$($r.L),$($r.T),$($r.R),$($r.B)"
