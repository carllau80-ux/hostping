# click.ps1 在屏幕指定坐标单击左键
# 用法: powershell -File click.ps1 <x> <y>
param([int]$x, [int]$y)
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class M {
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint dx, uint dy, uint d, UIntPtr e);
}
"@
[M]::SetCursorPos($x, $y) | Out-Null
Start-Sleep -Milliseconds 150
[M]::mouse_event(2, 0, 0, 0, [UIntPtr]::Zero)  # LEFTDOWN
[M]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)  # LEFTUP
Write-Output "clicked $x,$y"
