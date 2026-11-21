# zoom.ps1 裁剪并放大截图区域
# 用法: powershell -File zoom.ps1 <in.png> <out.png> <x> <y> <w> <h> [scale]
param([string]$in, [string]$out, [int]$x, [int]$y, [int]$w, [int]$h, [int]$scale = 4)
Add-Type -AssemblyName System.Drawing
$src = [System.Drawing.Image]::FromFile($in)
$crop = New-Object System.Drawing.Bitmap ([int]($w * $scale)), ([int]($h * $scale))
$g = [System.Drawing.Graphics]::FromImage($crop)
$g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::NearestNeighbor
$dstRect = New-Object System.Drawing.Rectangle 0, 0, ([int]($w * $scale)), ([int]($h * $scale))
$srcRect = New-Object System.Drawing.Rectangle $x, $y, $w, $h
$g.DrawImage($src, $dstRect, $srcRect, [System.Drawing.GraphicsUnit]::Pixel)
$g.Dispose()
$src.Dispose()
$crop.Save($out, [System.Drawing.Imaging.ImageFormat]::Png)
Write-Output "saved $out"
