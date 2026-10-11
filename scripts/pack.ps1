#Requires -Version 5.1
<#
.SYNOPSIS
    Packt Schmutzfink als Offline-ZIP fuer eine oder mehrere Zielumgebungen.

.DESCRIPTION
    Baut Web-UI und Go-Binary, legt CLIP-Modell, ONNX Runtime und Tokenizer bei
    (kein Internet auf dem Ziel noetig) und erzeugt je Setup ein ZIP unter dist/.

    Ziele:
      win-native-pg     Windows Server + natives PostgreSQL
      win-pg-container  Windows Server + Postgres-Container
      linux-compose     Linux, Docker Compose (App + Postgres)
      linux-vm          Linux-VM, systemd + natives PostgreSQL

.PARAMETER Target
    Ein oder mehrere Ziele, oder "all".

.PARAMETER OutDir
    Ausgabeverzeichnis (Standard: dist im Repositories).

.PARAMETER SkipDockerImages
    Image-Tars nicht erzeugen (kleineres ZIP, Ziel braucht dann Docker-Hub oder lokalen Build).

.EXAMPLE
    .\scripts\pack.ps1
    .\scripts\pack.ps1 -Target linux-vm,win-native-pg
    .\scripts\pack.ps1 -Target linux-compose -SkipDockerImages
#>
[CmdletBinding()]
param(
    [ValidateSet('all', 'win-native-pg', 'win-pg-container', 'linux-compose', 'linux-vm')]
    [string[]]$Target = @('all'),
    [string]$OutDir = '',
    [switch]$SkipDockerImages
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$OrtVersion = '1.24.1'
$OpenclipRepoSlug = 'amikos--openclip-vit-b-32-laion2b-s34b-b79k-onnx'
$OpenclipRevision = '248a2ed76a7189fc080e654e36930171331ef085'
$OpenclipBaseUrl = "https://huggingface.co/amikos/openclip-vit-b-32-laion2b-s34b-b79k-onnx/resolve/$OpenclipRevision"
$TokenizersTag = 'rust-v0.1.5'
$TokenizersBase = "https://releases.amikos.tech/pure-tokenizers/$TokenizersTag"
$OrtBase = "https://github.com/microsoft/onnxruntime/releases/download/v$OrtVersion"

$OpenclipFiles = @{
    'text_model.onnx'            = @{ Sha = '252b86e0ef1fc95b22cfd52fbf647142727fdbecc152556ffe0fba0b10a80370'; Size = 253973818 }
    'vision_model.onnx'          = @{ Sha = '7e14f76233d0c840c0621b1ef68f5877efe9357850782b1bbaf0c01693f73b43'; Size = 351618799 }
    'tokenizer.json'             = @{ Sha = 'b556ac8c99757ffb677208af34bc8c6721572114111a6e0aaf5fa69ff0b8d842'; Size = 2224041 }
    'preprocessor_config.json'   = @{ Sha = '910e70b3956ac9879ebc90b22fb3bc8a75b6a0677814500101a4c072bd7857bd'; Size = 316 }
}

function Join-Paths {
    $acc = [string]$args[0]
    for ($i = 1; $i -lt $args.Count; $i++) {
        $acc = Join-Path $acc ([string]$args[$i])
    }
    return $acc
}

function Write-Info([string]$Message) {
    Write-Host "==> $Message"
}

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][scriptblock]$Script,
        [Parameter(Mandatory = $true)][string]$Fail
    )
    & $Script
    if ($LASTEXITCODE -ne 0) { throw $Fail }
}

function Convert-ToLf([string]$Path) {
    $text = [System.IO.File]::ReadAllText($Path) -replace "`r`n", "`n" -replace "`r", "`n"
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($Path, $text, $utf8)
}

function Copy-UnixFile([string]$Src, [string]$Dst) {
    $dir = Split-Path $Dst -Parent
    if ($dir -and -not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
    }
    Copy-Item -LiteralPath $Src -Destination $Dst -Force
    Convert-ToLf $Dst
}

function Get-Sha256([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-Download([string]$Url, [string]$Dest) {
    $dir = Split-Path $Dest -Parent
    if ($dir -and -not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
    }
    Write-Info "Download $Url"
    $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
    if ($curl) {
        & $curl.Source -fsSL --retry 3 -o $Dest $Url
        if ($LASTEXITCODE -ne 0) { throw "Download fehlgeschlagen: $Url" }
        return
    }
    Invoke-WebRequest -Uri $Url -OutFile $Dest -UseBasicParsing
}

function Expand-Tar([string]$Archive, [string]$Dest) {
    if (-not (Test-Path $Dest)) {
        New-Item -ItemType Directory -Path $Dest | Out-Null
    }
    $tar = Get-Command tar.exe -ErrorAction SilentlyContinue
    if (-not $tar) { throw "tar.exe fehlt, Archiv kann nicht entpackt werden: $Archive" }
    & $tar.Source -xf $Archive -C $Dest
    if ($LASTEXITCODE -ne 0) { throw "Entpacken fehlgeschlagen: $Archive" }
}

function Copy-Robo([string]$Src, [string]$Dst) {
    if (-not (Test-Path $Dst)) {
        New-Item -ItemType Directory -Path $Dst | Out-Null
    }
    & robocopy.exe $Src $Dst /E /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
    if ($LASTEXITCODE -ge 8) { throw "Kopieren fehlgeschlagen: $Src -> $Dst (robocopy $LASTEXITCODE)" }
}

function New-ZipFromDir([string]$SourceDir, [string]$ZipPath) {
    if (Test-Path $ZipPath) { Remove-Item -LiteralPath $ZipPath -Force }
    $zipDir = Split-Path $ZipPath -Parent
    if (-not (Test-Path $zipDir)) {
        New-Item -ItemType Directory -Path $zipDir | Out-Null
    }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory(
        (Resolve-Path $SourceDir).Path,
        $ZipPath,
        [System.IO.Compression.CompressionLevel]::Optimal,
        $false
    )
}

function New-DeployPassword {
    $chars = [char[]]((48..57) + (65..90) + (97..122))
    -join (1..20 | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })
}

function Write-EnvFile {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$DatabaseUrl,
        [Parameter(Mandatory = $true)][ValidateSet('windows', 'linux')][string]$Os,
        [string]$AdminPassword = 'CHANGE_ME'
    )
    if ($Os -eq 'windows') {
        $ort = './vendor/onnxruntime/onnxruntime.dll'
        $tok = './vendor/tokenizers/tokenizers.dll'
    } else {
        $ort = './vendor/onnxruntime/libonnxruntime.so'
        $tok = './vendor/tokenizers/libtokenizers.so'
    }
    $ld = ''
    if ($Os -eq 'linux') {
        $ld = "LD_LIBRARY_PATH=./vendor/onnxruntime`n"
    }
    $body = @"
# Hinter nginx nur lokal binden. Im Container :8787 setzen.
HTTP_ADDR=127.0.0.1:8787
DATABASE_URL=$DatabaseUrl
STORAGE_BACKEND=local
STORAGE_DIR=./data/objects
# Erstes Konto nur bei leerer Datenbank, Rolle Admin. "admin"/"CHANGE_ME" sperrt den Bestand bis zur Passwortänderung.
ADMIN_USER=admin
ADMIN_PASSWORD=$AdminPassword
WEB_DIST=./web/dist
EMBEDDINGS=1
# auto = Secure bei HTTPS bzw. X-Forwarded-Proto (nur bei vertrauenswürdigem Proxy)
COOKIE_SECURE=auto
TRUST_PROXY=auto
ONNXRUNTIME_OPENCLIP_CACHE_DIR=./vendor/openclip
ONNXRUNTIME_LIB_PATH=$ort
TOKENIZERS_LIB_PATH=$tok
$ld
"@
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($Path, ($body -replace "`r`n", "`n"), $utf8)
}

function Sync-EmbeddedWeb([string]$Repo) {
    $src = Join-Paths $Repo 'web' 'dist'
    $dst = Join-Paths $Repo 'internal' 'webui' 'dist'
    if (-not (Test-Path $src)) { throw "web/dist fehlt - zuerst npm run build" }
    Write-Info "Web-UI in Binary einbetten"
    if (Test-Path $dst) {
        Get-ChildItem -LiteralPath $dst -Force | Remove-Item -Recurse -Force
    }
    Copy-Robo $src $dst
}

function Restore-EmbeddedWebStub([string]$Repo) {
    $dst = Join-Paths $Repo 'internal' 'webui' 'dist'
    if (Test-Path $dst) {
        Get-ChildItem -LiteralPath $dst -Force | Remove-Item -Recurse -Force
    } else {
        New-Item -ItemType Directory -Path $dst | Out-Null
    }
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
    [System.IO.File]::WriteAllText((Join-Path $dst 'index.html'), ($stub -replace "`r`n", "`n"), $utf8)
}

function Ensure-WebBuild {
    param([string]$Repo)
    Write-Info "Web-UI bauen"
    Push-Location (Join-Path $Repo 'web')
    try {
        if (-not (Test-Path 'node_modules')) {
            Invoke-Checked { npm install } "npm install fehlgeschlagen"
        }
        Invoke-Checked { npm run build } "npm run build fehlgeschlagen"
    } finally {
        Pop-Location
    }
}

function Ensure-GoBuild {
    param(
        [string]$Repo,
        [string]$Goos,
        [string]$OutFile
    )
    Write-Info "Go-Binary ($Goos) -> $OutFile"
    $outDir = Split-Path $OutFile -Parent
    if (-not (Test-Path $outDir)) {
        New-Item -ItemType Directory -Path $outDir | Out-Null
    }
    $env:CGO_ENABLED = '0'
    $env:GOOS = $Goos
    $env:GOARCH = 'amd64'
    Push-Location $Repo
    try {
        Invoke-Checked { go build -trimpath -ldflags "-s -w" -o $OutFile ./cmd/server } "go build ($Goos) fehlgeschlagen"
    } finally {
        Pop-Location
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    }
}

function Ensure-OpenClip {
    param([string]$CacheRoot)
    $dest = Join-Paths $CacheRoot 'openclip' $OpenclipRepoSlug $OpenclipRevision
    if (-not (Test-Path $dest)) {
        New-Item -ItemType Directory -Path $dest | Out-Null
    }

    $local = Join-Paths $env:LOCALAPPDATA 'onnx-purego' 'openclip' $OpenclipRepoSlug $OpenclipRevision
    foreach ($name in $OpenclipFiles.Keys) {
        $target = Join-Path $dest $name
        $meta = $OpenclipFiles[$name]
        $ok = $false
        if (Test-Path $target) {
            $len = (Get-Item -LiteralPath $target).Length
            if ($len -eq $meta.Size -and (Get-Sha256 $target) -eq $meta.Sha) { $ok = $true }
        }
        if ($ok) { continue }

        $fromLocal = Join-Path $local $name
        if (Test-Path $fromLocal) {
            Write-Info "CLIP: $name aus lokalem Cache"
            Copy-Item -LiteralPath $fromLocal -Destination $target -Force
        } else {
            Get-Download "$OpenclipBaseUrl/$name" $target
        }
        $len = (Get-Item -LiteralPath $target).Length
        $sha = Get-Sha256 $target
        if ($len -ne $meta.Size -or $sha -ne $meta.Sha) {
            throw "CLIP-Datei $name hat unerwartete Groesse/Checksumme"
        }
    }
    return (Join-Path $CacheRoot 'openclip')
}

function Ensure-OnnxRuntime {
    param(
        [string]$CacheRoot,
        [ValidateSet('windows', 'linux')][string]$Os
    )
    $dest = Join-Path $CacheRoot "onnxruntime-$Os"
    $marker = Join-Path $dest '.ok'
    if (Test-Path $marker) { return $dest }
    if (-not (Test-Path $dest)) {
        New-Item -ItemType Directory -Path $dest | Out-Null
    }

    if ($Os -eq 'windows') {
        $localLib = Join-Paths $env:LOCALAPPDATA 'onnx-purego' 'onnxruntime' "onnxruntime-win-x64-$OrtVersion" 'lib'
        $dll = Join-Path $localLib 'onnxruntime.dll'
        if (Test-Path $dll) {
            Write-Info "ONNX Runtime Windows aus lokalem Cache"
            Get-ChildItem -LiteralPath $localLib -File -Filter '*.dll' | Copy-Item -Destination $dest -Force
        } else {
            $zip = Join-Path $CacheRoot "onnxruntime-win-x64-$OrtVersion.zip"
            if (-not (Test-Path $zip)) {
                Get-Download "$OrtBase/onnxruntime-win-x64-$OrtVersion.zip" $zip
            }
            $extract = Join-Path $CacheRoot "onnxruntime-win-extract"
            if (Test-Path $extract) { Remove-Item $extract -Recurse -Force }
            Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force
            $libDir = Get-ChildItem -Path $extract -Recurse -Directory -Filter 'lib' | Select-Object -First 1
            if (-not $libDir) { throw "ONNX Runtime Windows: lib/ nicht gefunden" }
            Get-ChildItem -LiteralPath $libDir.FullName -File -Filter '*.dll' | Copy-Item -Destination $dest -Force
        }
        if (-not (Test-Path (Join-Path $dest 'onnxruntime.dll'))) {
            throw "onnxruntime.dll fehlt nach dem Packen"
        }
    } else {
        $tgz = Join-Path $CacheRoot "onnxruntime-linux-x64-$OrtVersion.tgz"
        if (-not (Test-Path $tgz)) {
            Get-Download "$OrtBase/onnxruntime-linux-x64-$OrtVersion.tgz" $tgz
        }
        $extract = Join-Path $CacheRoot 'onnxruntime-linux-extract'
        if (Test-Path $extract) { Remove-Item $extract -Recurse -Force }
        Expand-Tar $tgz $extract
        $libDir = Get-ChildItem -Path $extract -Recurse -Directory -Filter 'lib' | Select-Object -First 1
        if (-not $libDir) { throw "ONNX Runtime Linux: lib/ nicht gefunden" }
        $provider = Get-ChildItem -LiteralPath $libDir.FullName -File | Where-Object {
            $_.Name -like 'libonnxruntime_providers_shared.so*'
        } | Select-Object -First 1
        $main = Get-ChildItem -LiteralPath $libDir.FullName -File | Where-Object {
            $_.Name -like 'libonnxruntime.so*' -and $_.Name -notlike '*providers*'
        } | Sort-Object Length -Descending | Select-Object -First 1
        if (-not $main) { throw "libonnxruntime.so fehlt nach dem Packen" }
        Copy-Item -LiteralPath $main.FullName -Destination (Join-Path $dest 'libonnxruntime.so') -Force
        if ($provider) {
            Copy-Item -LiteralPath $provider.FullName -Destination (Join-Path $dest 'libonnxruntime_providers_shared.so') -Force
        }
    }
    Set-Content -LiteralPath $marker -Value $OrtVersion
    return $dest
}

function Ensure-Tokenizers {
    param(
        [string]$CacheRoot,
        [ValidateSet('windows', 'linux')][string]$Os
    )
    $dest = Join-Path $CacheRoot "tokenizers-$Os"
    $marker = Join-Path $dest '.ok'
    if (Test-Path $marker) { return $dest }
    if (-not (Test-Path $dest)) {
        New-Item -ItemType Directory -Path $dest | Out-Null
    }

    if ($Os -eq 'windows') {
        $asset = 'libtokenizers-x86_64-pc-windows-msvc.tar.gz'
        $libName = 'tokenizers.dll'
        $local = Join-Paths $env:APPDATA 'tokenizers' 'lib' 'tokenizers.dll'
        if (Test-Path $local) {
            Copy-Item -LiteralPath $local -Destination (Join-Path $dest $libName) -Force
            Set-Content -LiteralPath $marker -Value $TokenizersTag
            return $dest
        }
    } else {
        $asset = 'libtokenizers-x86_64-unknown-linux-gnu.tar.gz'
        $libName = 'libtokenizers.so'
    }

    $archive = Join-Path $CacheRoot $asset
    if (-not (Test-Path $archive)) {
        Get-Download "$TokenizersBase/$asset" $archive
    }
    $extract = Join-Path $CacheRoot "tokenizers-$Os-extract"
    if (Test-Path $extract) { Remove-Item $extract -Recurse -Force }
    Expand-Tar $archive $extract
    $found = Get-ChildItem -Path $extract -Recurse -File | Where-Object { $_.Name -eq $libName } | Select-Object -First 1
    if (-not $found) {
        $found = Get-ChildItem -Path $extract -Recurse -File | Where-Object {
            $_.Extension -in @('.dll', '.so') -or $_.Name -like '*.so*'
        } | Select-Object -First 1
    }
    if (-not $found) { throw "Tokenizer-Bibliothek nicht in $asset gefunden" }
    Copy-Item -LiteralPath $found.FullName -Destination (Join-Path $dest $libName) -Force
    Set-Content -LiteralPath $marker -Value $TokenizersTag
    return $dest
}

function Copy-Vendor {
    param(
        [string]$Stage,
        [string]$OpenclipDir,
        [string]$OrtDir,
        [string]$TokDir
    )
    $vendor = Join-Path $Stage 'vendor'
    Copy-Robo $OpenclipDir (Join-Path $vendor 'openclip')
    Copy-Robo $OrtDir (Join-Path $vendor 'onnxruntime')
    Copy-Robo $TokDir (Join-Path $vendor 'tokenizers')
    Get-ChildItem -Path $vendor -Recurse -File -Filter '.ok' | Remove-Item -Force
}

function Copy-WebDist([string]$Repo, [string]$Stage) {
    $src = Join-Paths $Repo 'web' 'dist'
    $dst = Join-Paths $Stage 'web' 'dist'
    if (-not (Test-Path $src)) { throw "web/dist fehlt - Build fehlgeschlagen?" }
    Copy-Robo $src $dst
}

function Save-DockerImageTar {
    param(
        [string]$Image,
        [string]$TarPath
    )
    $docker = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $docker) { throw "Docker fehlt, Image $Image kann nicht gespeichert werden." }
    $inspect = & docker image inspect $Image 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Info "docker pull $Image"
        Invoke-Checked { docker pull $Image } "docker pull $Image fehlgeschlagen"
    }
    $dir = Split-Path $TarPath -Parent
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
    }
    Write-Info "docker save $Image"
    Invoke-Checked { docker save -o $TarPath $Image } "docker save $Image fehlgeschlagen"
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not $OutDir) { $OutDir = Join-Path $RepoRoot 'dist' }
$OutDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
$Cache = Join-Path $OutDir 'cache'
$StagingRoot = Join-Path $OutDir 'staging'
New-Item -ItemType Directory -Path $Cache -Force | Out-Null
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null

$wanted = @()
foreach ($t in $Target) {
    if ($t -eq 'all') {
        $wanted = @('win-native-pg', 'win-pg-container', 'linux-compose', 'linux-vm')
        break
    }
    $wanted += $t
}
$wanted = $wanted | Select-Object -Unique
Write-Info ("Ziele: " + ($wanted -join ', '))

$needWin = $wanted | Where-Object { $_ -like 'win-*' }
$needLinux = $wanted | Where-Object { $_ -like 'linux-*' }

Ensure-WebBuild $RepoRoot
Sync-EmbeddedWeb $RepoRoot
$openclipDir = Ensure-OpenClip $Cache

$winExe = Join-Paths $Cache 'bin' 'schmutzfink.exe'
$linuxBin = Join-Paths $Cache 'bin' 'schmutzfink'
$ortWin = $null
$ortLinux = $null
$tokWin = $null
$tokLinux = $null

if ($needWin) {
    Ensure-GoBuild $RepoRoot 'windows' $winExe
    $ortWin = Ensure-OnnxRuntime $Cache 'windows'
    $tokWin = Ensure-Tokenizers $Cache 'windows'
}
if ($needLinux) {
    Ensure-GoBuild $RepoRoot 'linux' $linuxBin
    $ortLinux = Ensure-OnnxRuntime $Cache 'linux'
    $tokLinux = Ensure-Tokenizers $Cache 'linux'
}

Restore-EmbeddedWebStub $RepoRoot

function Publish-WinNativePg {
    $name = 'win-native-pg'
    $stage = Join-Path $StagingRoot $name
    if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
    New-Item -ItemType Directory -Path $stage | Out-Null
    Copy-Item $winExe (Join-Path $stage 'schmutzfink.exe')
    Copy-WebDist $RepoRoot $stage
    Copy-Vendor $stage $openclipDir $ortWin $tokWin
    Copy-Item (Join-Path $RepoRoot 'deploy\win-native-pg\README.md') (Join-Path $stage 'README.md')
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\start.ps1') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\stop.ps1') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\install-task.ps1') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\uninstall-task.ps1') $stage
    $pg = Join-Path $stage 'postgres'
    New-Item -ItemType Directory -Path $pg | Out-Null
    Copy-Item (Join-Path $RepoRoot 'deploy\postgres\setup.ps1') $pg
    Copy-Item (Join-Path $RepoRoot 'deploy\postgres\setup.sql') $pg
    Write-EnvFile (Join-Path $stage '.env.example') 'postgres://schmutzfink:schmutzfink@127.0.0.1:5432/schmutzfink?sslmode=disable' 'windows'
    $zip = Join-Path $OutDir "schmutzfink-$name.zip"
    Write-Info "ZIP $zip"
    New-ZipFromDir $stage $zip
}

function Publish-WinPgContainer {
    $name = 'win-pg-container'
    $stage = Join-Path $StagingRoot $name
    if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
    New-Item -ItemType Directory -Path $stage | Out-Null
    Copy-Item $winExe (Join-Path $stage 'schmutzfink.exe')
    Copy-WebDist $RepoRoot $stage
    Copy-Vendor $stage $openclipDir $ortWin $tokWin
    Copy-Item (Join-Path $RepoRoot 'deploy\win-pg-container\README.md') (Join-Path $stage 'README.md')
    Copy-Item (Join-Path $RepoRoot 'deploy\win-pg-container\docker-compose.yml') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-pg-container\load-postgres.ps1') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\start.ps1') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\stop.ps1') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\install-task.ps1') $stage
    Copy-Item (Join-Path $RepoRoot 'deploy\win-common\uninstall-task.ps1') $stage
    Write-EnvFile (Join-Path $stage '.env.example') 'postgres://schmutzfink:schmutzfink@127.0.0.1:5432/schmutzfink?sslmode=disable' 'windows'
    if (-not $SkipDockerImages) {
        Save-DockerImageTar 'pgvector/pgvector:pg16' (Join-Path $stage 'images\pgvector-pg16.tar')
    }
    $zip = Join-Path $OutDir "schmutzfink-$name.zip"
    Write-Info "ZIP $zip"
    New-ZipFromDir $stage $zip
}

function Publish-LinuxCompose {
    $name = 'linux-compose'
    $stage = Join-Path $StagingRoot $name
    if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
    New-Item -ItemType Directory -Path $stage | Out-Null
    Copy-Item $linuxBin (Join-Path $stage 'schmutzfink')
    Copy-WebDist $RepoRoot $stage
    Copy-Vendor $stage $openclipDir $ortLinux $tokLinux
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-compose\Dockerfile') (Join-Path $stage 'Dockerfile')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-compose\docker-compose.yml') (Join-Path $stage 'docker-compose.yml')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-compose\.dockerignore') (Join-Path $stage '.dockerignore')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-compose\load-and-start.sh') (Join-Path $stage 'load-and-start.sh')
    Copy-Item (Join-Path $RepoRoot 'deploy\linux-compose\README.md') (Join-Path $stage 'README.md')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\nginx\schmutzfink.conf') (Join-Path $stage 'nginx\schmutzfink.conf')
    Write-EnvFile (Join-Path $stage '.env') 'postgres://schmutzfink:schmutzfink@postgres:5432/schmutzfink?sslmode=disable' 'linux' (New-DeployPassword)

    if (-not $SkipDockerImages) {
        $docker = Get-Command docker -ErrorAction SilentlyContinue
        if (-not $docker) { throw "Docker fehlt fuer linux-compose. Alternativ: -SkipDockerImages" }
        Write-Info "Docker-Image schmutzfink:offline bauen"
        Invoke-Checked { docker build -t schmutzfink:offline $stage } "docker build fehlgeschlagen"
        Save-DockerImageTar 'schmutzfink:offline' (Join-Path $stage 'images\schmutzfink.tar')
        Save-DockerImageTar 'pgvector/pgvector:pg16' (Join-Path $stage 'images\pgvector-pg16.tar')
    }

    $zip = Join-Path $OutDir "schmutzfink-$name.zip"
    Write-Info "ZIP $zip"
    New-ZipFromDir $stage $zip
}

function Publish-LinuxVm {
    $name = 'linux-vm'
    $stage = Join-Path $StagingRoot $name
    if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
    New-Item -ItemType Directory -Path $stage | Out-Null
    Copy-Item $linuxBin (Join-Path $stage 'schmutzfink')
    Copy-WebDist $RepoRoot $stage
    Copy-Vendor $stage $openclipDir $ortLinux $tokLinux
    Copy-Item (Join-Path $RepoRoot 'deploy\linux-vm\README.md') (Join-Path $stage 'README.md')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-vm\schmutzfink.service') (Join-Path $stage 'schmutzfink.service')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-vm\install.sh') (Join-Path $stage 'install.sh')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-vm\update.sh') (Join-Path $stage 'update.sh')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\linux-vm\start.sh') (Join-Path $stage 'start.sh')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\nginx\schmutzfink.conf') (Join-Path $stage 'nginx\schmutzfink.conf')
    $pg = Join-Path $stage 'postgres'
    New-Item -ItemType Directory -Path $pg | Out-Null
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\postgres\setup.sh') (Join-Path $pg 'setup.sh')
    Copy-UnixFile (Join-Path $RepoRoot 'deploy\postgres\setup.sql') (Join-Path $pg 'setup.sql')
    Write-EnvFile (Join-Path $stage '.env.example') 'postgres://schmutzfink:schmutzfink@127.0.0.1:5432/schmutzfink?sslmode=disable' 'linux'
    $zip = Join-Path $OutDir "schmutzfink-$name.zip"
    Write-Info "ZIP $zip"
    New-ZipFromDir $stage $zip
}

foreach ($item in $wanted) {
    Write-Info "Paket $item"
    switch ($item) {
        'win-native-pg' { Publish-WinNativePg }
        'win-pg-container' { Publish-WinPgContainer }
        'linux-compose' { Publish-LinuxCompose }
        'linux-vm' { Publish-LinuxVm }
        default { throw "Unbekanntes Ziel: $item" }
    }
}

Write-Host ""
Write-Info "Fertig. Archive unter $OutDir"
Get-ChildItem -LiteralPath $OutDir -Filter 'schmutzfink-*.zip' | ForEach-Object {
    Write-Host ("  {0}  {1:N1} MB" -f $_.Name, ($_.Length / 1MB))
}
