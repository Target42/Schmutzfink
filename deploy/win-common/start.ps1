#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$exe = Join-Path $PSScriptRoot 'schmutzfink.exe'
if (-not (Test-Path $exe)) {
    throw "schmutzfink.exe fehlt in $PSScriptRoot"
}

$envFile = Join-Path $PSScriptRoot '.env'
$example = Join-Path $PSScriptRoot '.env.example'
if (-not (Test-Path $envFile) -and (Test-Path $example)) {
    Copy-Item $example $envFile
    Write-Host ".env aus .env.example angelegt. Bitte Passwoerter anpassen."
}

Write-Host "Starte Schmutzfink in $PSScriptRoot"
& $exe
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
