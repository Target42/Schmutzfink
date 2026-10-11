#Requires -Version 5.1
# Als Administrator ausfuehren. Legt eine geplante Aufgabe an, die Schmutzfink nach dem Start des Servers ausfuehrt.
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$exe = Join-Path $here 'schmutzfink.exe'
if (-not (Test-Path $exe)) { throw "schmutzfink.exe fehlt." }

$taskName = 'Schmutzfink'
$action = New-ScheduledTaskAction -Execute $exe -WorkingDirectory $here
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero)
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
Start-ScheduledTask -TaskName $taskName
Write-Host "Aufgabe '$taskName' ist eingerichtet und gestartet. UI: http://127.0.0.1:8787"
