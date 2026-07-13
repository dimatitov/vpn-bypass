# Запускать PowerShell от имени администратора
$TargetDir = Join-Path $env:ProgramData "vpn-bypass"
$TaskName = "VPN Bypass"

Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

$Python = (Get-Command python.exe -ErrorAction SilentlyContinue).Source
$Script = Join-Path $TargetDir "vpn_bypass.py"
if ($Python -and (Test-Path $Script)) { & $Python $Script clear }

Remove-Item $TargetDir -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "Удалено."
