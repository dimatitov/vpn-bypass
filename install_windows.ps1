# Запускать PowerShell от имени администратора
$ErrorActionPreference = "Stop"
$SourceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$TargetDir = Join-Path $env:ProgramData "vpn-bypass"
$TaskName = "VPN Bypass"

$Python = (Get-Command python.exe -ErrorAction SilentlyContinue).Source
if (-not $Python) { throw "Не найден Python 3 в PATH." }

New-Item -ItemType Directory -Path $TargetDir -Force | Out-Null
Copy-Item (Join-Path $SourceDir "vpn_bypass.py") $TargetDir -Force
Copy-Item (Join-Path $SourceDir "domains.txt") $TargetDir -Force
Copy-Item (Join-Path $SourceDir "cidrs.txt") $TargetDir -Force

$Script = Join-Path $TargetDir "vpn_bypass.py"
$Action = New-ScheduledTaskAction -Execute $Python -Argument "`"$Script`" watch --interval 60"
$Trigger = New-ScheduledTaskTrigger -AtStartup
$Principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings | Out-Null
Start-ScheduledTask -TaskName $TaskName

Write-Host "Готово."
Write-Host "Домены: $TargetDir\domains.txt"
Write-Host "Лог: $TargetDir\vpn-bypass.log"
