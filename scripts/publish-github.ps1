#Requires -Version 5.1
<#
.SYNOPSIS
    Veroeffentlicht bewusst eine Version: Tag auf Nyx, Snapshot ohne History auf GitHub.

.DESCRIPTION
    1. Legt bei Bedarf einen annotierten Tag auf dem aktuellen HEAD an
    2. Pusht Branch und Tag nach origin (Nyx)
    3. Erzeugt einen frischen Root-Commit nur mit dem Tree dieses Tags
    4. Force-pusht diesen Snapshot als main + denselben Tag nach GitHub

    Die lokale / Nyx-History bleibt unveraendert. Auf GitHub liegt nur der
    veroeffentlichte Stand (ein Commit je Release, History wird ersetzt).

.PARAMETER Tag
    Versions-Tag, z.B. v1.0.0

.PARAMETER Message
    Tag-/Release-Nachricht (Standard: "Release <Tag>")

.PARAMETER GithubRemote
    Name des GitHub-Remotes (Standard: github). Fehlt er, wird -GithubUrl gebraucht.

.PARAMETER GithubUrl
    URL des oeffentlichen Repos. Setzt oder aktualisiert das Remote GithubRemote.

.PARAMETER RemoteName
    Internes Remote fuer Tags/Branch (Standard: origin = Nyx).

.PARAMETER AllowDirty
    Auch bei uncommitteten Aenderungen fortfahren (nicht empfohlen).

.PARAMETER SkipNyx
    Tag/Branch nicht nach Nyx pushen (nur GitHub-Snapshot).

.PARAMETER DryRun
    Zeigt die Schritte, pusht nichts.

.EXAMPLE
    .\scripts\publish-github.ps1 -Tag v1.0.0 -GithubUrl https://github.com/USER/Schmutzfink.git

.EXAMPLE
    git remote add github https://github.com/USER/Schmutzfink.git
    .\scripts\publish-github.ps1 -Tag v1.0.0
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v?\d+\.\d+\.\d+([.-].+)?$')]
    [string]$Tag,

    [string]$Message = '',

    [string]$GithubRemote = 'github',

    [string]$GithubUrl = '',

    [string]$RemoteName = 'origin',

    [switch]$AllowDirty,

    [switch]$SkipNyx,

    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

function Write-Info([string]$Message) {
    Write-Host "==> $Message"
}

function Invoke-Git {
    param(
        [Parameter(Mandatory = $true)][string[]]$Args,
        [string]$Fail = 'git fehlgeschlagen'
    )
    & git @Args
    if ($LASTEXITCODE -ne 0) { throw "$Fail (git $($Args -join ' '))" }
}

function Get-GitOutput {
    param([Parameter(Mandatory = $true)][string[]]$Args)
    $out = & git @Args 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "git $($Args -join ' ') fehlgeschlagen: $out"
    }
    return ($out | Out-String).Trim()
}

$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $RepoRoot

if (-not $Message) {
    $Message = "Release $Tag"
}

$branch = Get-GitOutput @('rev-parse', '--abbrev-ref', 'HEAD')
if ($branch -eq 'HEAD') {
    throw 'Detached HEAD: bitte auf einem Branch auschecken (z.B. main).'
}

$status = Get-GitOutput @('status', '--porcelain')
if ($status -and -not $AllowDirty) {
    throw "Working tree ist nicht clean. Committe zuerst oder nutze -AllowDirty.`n$status"
}

$head = Get-GitOutput @('rev-parse', 'HEAD')
Write-Info "Repo: $RepoRoot"
Write-Info "Branch: $branch @ $head"
Write-Info "Tag: $Tag"

$existingTag = & git rev-parse --verify --quiet "refs/tags/$Tag" 2>$null
if ($LASTEXITCODE -eq 0 -and $existingTag) {
    $tagTarget = Get-GitOutput @('rev-list', '-n', '1', $Tag)
    if ($tagTarget -ne $head) {
        throw "Tag $Tag zeigt bereits auf $tagTarget, HEAD ist $head. Anderen Tag waehlen oder Tag lokal loeschen."
    }
    Write-Info "Tag $Tag existiert bereits und zeigt auf HEAD"
} else {
    Write-Info "Erzeuge annotierten Tag $Tag"
    if (-not $DryRun) {
        Invoke-Git @('tag', '-a', $Tag, '-m', $Message) -Fail "Tag $Tag anlegen fehlgeschlagen"
    }
}

# GitHub-Remote sicherstellen
$githubUrlResolved = $GithubUrl
if (-not $githubUrlResolved) {
    $remoteUrl = & git remote get-url $GithubRemote 2>$null
    if ($LASTEXITCODE -eq 0 -and $remoteUrl) {
        $githubUrlResolved = "$remoteUrl".Trim()
    }
}
if (-not $githubUrlResolved) {
    throw "Kein GitHub-Remote. Entweder: git remote add $GithubRemote <url>  oder  -GithubUrl https://github.com/.../....git"
}

$knownRemotes = @(Get-GitOutput @('remote')) -split "\r?\n" | Where-Object { $_ }
if ($knownRemotes -notcontains $GithubRemote) {
    Write-Info "Remote '$GithubRemote' anlegen -> $githubUrlResolved"
    if (-not $DryRun) {
        Invoke-Git @('remote', 'add', $GithubRemote, $githubUrlResolved) -Fail 'GitHub-Remote anlegen fehlgeschlagen'
    }
} elseif ($GithubUrl) {
    Write-Info "Remote '$GithubRemote' URL setzen -> $GithubUrl"
    if (-not $DryRun) {
        Invoke-Git @('remote', 'set-url', $GithubRemote, $GithubUrl) -Fail 'GitHub-Remote URL setzen fehlgeschlagen'
    }
}

if (-not $SkipNyx) {
    Write-Info "Push nach Nyx ($RemoteName): $branch + $Tag"
    if (-not $DryRun) {
        Invoke-Git @('push', $RemoteName, $branch) -Fail "Push $branch nach $RemoteName fehlgeschlagen"
        Invoke-Git @('push', $RemoteName, $Tag) -Fail "Push Tag $Tag nach $RemoteName fehlgeschlagen"
    }
} else {
    Write-Info 'SkipNyx: kein Push nach Nyx'
}

# Snapshot ohne History in Temp-Repo
$stage = Join-Path ([System.IO.Path]::GetTempPath()) ("schmutzfink-github-" + [guid]::NewGuid().ToString('N'))
Write-Info "Snapshot-Stage: $stage"

try {
    if ($DryRun) {
        Write-Info "DryRun: wuerde Tree von $Tag nach $GithubRemote (main + $Tag) force-pushen"
        Write-Info 'Fertig (DryRun).'
        return
    }

    New-Item -ItemType Directory -Path $stage | Out-Null
    # --no-hardlinks: Temp-Repo darf unter Windows sicher geloescht werden
    Invoke-Git @('clone', '--local', '--no-hardlinks', '--no-checkout', $RepoRoot, $stage) -Fail 'Temp-Clone fehlgeschlagen'

    Push-Location $stage
    try {
        # Nur den Tree des Tags, neuer Root-Commit
        Invoke-Git @('checkout', $Tag) -Fail "Checkout $Tag im Temp-Repo fehlgeschlagen"
        Invoke-Git @('checkout', '--orphan', 'github-release') -Fail 'Orphan-Branch fehlgeschlagen'
        Invoke-Git @('add', '-A') -Fail 'git add fehlgeschlagen'
        Invoke-Git @('commit', '-m', $Message) -Fail 'Snapshot-Commit fehlgeschlagen'
        $snap = Get-GitOutput @('rev-parse', 'HEAD')
        Write-Info "Snapshot-Commit: $snap"

        # Eigenen Tag im Temp-Repo auf den Snapshot (lokaler Tag bleibt auf voller History)
        & git tag -d $Tag 2>$null | Out-Null
        Invoke-Git @('tag', '-a', $Tag, '-m', $Message) -Fail "Snapshot-Tag $Tag fehlgeschlagen"

        Invoke-Git @('remote', 'remove', 'origin') -Fail 'origin im Temp-Repo entfernen fehlgeschlagen'
        Invoke-Git @('remote', 'add', 'github', $githubUrlResolved) -Fail 'GitHub-Remote im Temp-Repo fehlgeschlagen'

        Write-Info "Force-Push Snapshot -> ${GithubRemote}:main"
        Invoke-Git @('push', '-f', 'github', 'HEAD:main') -Fail 'Force-Push main nach GitHub fehlgeschlagen'

        Write-Info "Force-Push Tag $Tag -> GitHub"
        Invoke-Git @('push', '-f', 'github', "refs/tags/${Tag}:refs/tags/${Tag}") -Fail "Force-Push Tag $Tag nach GitHub fehlgeschlagen"
    } finally {
        Pop-Location
    }
} finally {
    if (Test-Path -LiteralPath $stage) {
        Write-Info 'Temp-Stage aufraeumen'
        Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Info "Fertig: Nyx hat $Tag auf der echten History, GitHub hat denselben Stand ohne History."
