#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
Get-Process -Name schmutzfink -ErrorAction SilentlyContinue | Stop-Process -Force
Write-Host "Schmutzfink beendet."
