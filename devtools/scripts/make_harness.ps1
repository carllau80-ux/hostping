$ErrorActionPreference = "Stop"
$dist = "C:\Users\excel\ZCodeProject\dbping\frontend\dist"
$html = [System.IO.File]::ReadAllText("$dist\index.html", [System.Text.Encoding]::UTF8)
$html = $html.Replace('<script src="app.js">', '<script src="_mock.js"></script><script src="app.js">')
[System.IO.File]::WriteAllText("$dist\_harness.html", $html, (New-Object System.Text.UTF8Encoding($false)))
Write-Output "harness written"
