#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$tar = Join-Path $PSScriptRoot 'images\pgvector-pg16.tar'
if (Test-Path $tar) {
    Write-Host "Lade Postgres-Image aus $tar ..."
    docker load -i $tar
    if ($LASTEXITCODE -ne 0) { throw "docker load fehlgeschlagen." }
} else {
    Write-Host "Kein Image-Tar gefunden, versuche docker compose pull (braucht Internet) ..."
    docker compose pull
    if ($LASTEXITCODE -ne 0) { throw "docker compose pull fehlgeschlagen." }
}
Write-Host "Postgres-Image ist vorhanden."
