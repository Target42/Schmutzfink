#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
$taskName = 'Schmutzfink'
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
    Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
}
Get-Process -Name schmutzfink -ErrorAction SilentlyContinue | Stop-Process -Force
Write-Host "Aufgabe '$taskName' entfernt."
