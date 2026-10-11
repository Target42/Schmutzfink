# Publishes the signed release APK for portal download (MOBILE_APK_PATH).
# Usage: .\mobile\scripts\publish-apk.ps1

$ErrorActionPreference = "Stop"

$mobileDir = Split-Path $PSScriptRoot -Parent
$repoRoot = Split-Path $mobileDir -Parent

$apkSrc = Join-Path $mobileDir "build\app\outputs\flutter-apk\app-release.apk"
if (-not (Test-Path $apkSrc)) {
  Write-Error "APK fehlt: $apkSrc - zuerst: cd mobile; flutter build apk --release"
}

$pubspec = Get-Content (Join-Path $mobileDir "pubspec.yaml") -Raw
if ($pubspec -notmatch 'version:\s*([0-9.]+)\+(\d+)') {
  Write-Error "version in pubspec.yaml nicht gefunden (erwartet z.B. 1.0.0+1)"
}
$version = $Matches[1]
$versionCode = [int]$Matches[2]

$destDir = Join-Path $repoRoot "data\mobile"
New-Item -ItemType Directory -Force -Path $destDir | Out-Null
$apkDest = Join-Path $destDir "schmutzfink-android.apk"
$metaDest = Join-Path $destDir "schmutzfink-android.json"

Copy-Item -Force $apkSrc $apkDest
$meta = @{
  version      = $version
  version_code = $versionCode
  released_at  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  notes        = ""
} | ConvertTo-Json
[System.IO.File]::WriteAllText($metaDest, $meta)

Write-Host "Veroeffentlicht: $apkDest"
Write-Host "Version $version ($versionCode)"
Write-Host "Metadaten: $metaDest"
