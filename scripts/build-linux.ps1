#Requires -Version 5.1
<#
.SYNOPSIS
    Baut das Linux-Binary mit eingebetteter Web-UI. Ohne CLIP/ONNX-ZIP.
#>
$ErrorActionPreference = 'Stop'
$Repo = Split-Path -Parent $PSScriptRoot
$OutDir = Join-Path $Repo 'dist\cache\bin'
$Out = Join-Path $OutDir 'schmutzfink'
$WebSrc = Join-Path $Repo 'web\dist'
$EmbedDst = Join-Path $Repo 'internal\webui\dist'

Write-Host "==> Web-UI"
Push-Location (Join-Path $Repo 'web')
try {
    if (-not (Test-Path 'node_modules')) {
        npm install
        if ($LASTEXITCODE -ne 0) { throw 'npm install fehlgeschlagen' }
    }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'npm run build fehlgeschlagen' }
} finally {
    Pop-Location
}

Write-Host "==> Web-UI einbetten"
if (Test-Path $EmbedDst) {
    Get-ChildItem -LiteralPath $EmbedDst -Force | Remove-Item -Recurse -Force
} else {
    New-Item -ItemType Directory -Path $EmbedDst | Out-Null
}
& robocopy.exe $WebSrc $EmbedDst /E /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
if ($LASTEXITCODE -ge 8) { throw "Kopieren nach $EmbedDst fehlgeschlagen" }

Write-Host "==> Linux-Binary -> $Out"
if (-not (Test-Path $OutDir)) {
    New-Item -ItemType Directory -Path $OutDir | Out-Null
}
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
Push-Location $Repo
try {
    go build -trimpath -ldflags "-s -w" -o $Out ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw 'go build fehlgeschlagen' }
} finally {
    Pop-Location
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
}

Get-ChildItem -LiteralPath $EmbedDst -Force | Remove-Item -Recurse -Force
$stub = @'
<!doctype html>
<html lang="de">
  <head>
    <meta charset="utf-8" />
    <title>Schmutzfink</title>
  </head>
  <body>
    <p>Web-UI nicht eingebettet. Lokal: <code>npm run build</code> im Ordner web, oder <code>WEB_DIST=./web/dist</code>.</p>
  </body>
</html>
'@
$utf8 = New-Object System.Text.UTF8Encoding $false
[System.IO.File]::WriteAllText((Join-Path $EmbedDst 'index.html'), ($stub -replace "`r`n", "`n"), $utf8)

$size = [math]::Round((Get-Item $Out).Length / 1MB, 1)
Write-Host "==> Fertig: $Out ($size MB)"
Write-Host "Auf den Server: scp dist/cache/bin/schmutzfink root@HOST:/tmp/schmutzfink"
