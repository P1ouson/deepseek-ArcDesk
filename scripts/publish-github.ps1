# Publish ArcDesk to GitHub (P1ouson/ArcDesk).
# Prereqs: git, gh CLI, logged in (`gh auth login`) or GH_TOKEN set.
#
# Usage:
#   .\scripts\publish-github.ps1
#   .\scripts\publish-github.ps1 -SkipPush    # release only
#   .\scripts\publish-github.ps1 -Tag desktop-v0.1.0

param(
    [string]$Owner = "P1ouson",
    [string]$Repo = "ArcDesk",
    [string]$OldRepo = "better_ds_ui",
    [string]$Branch = "ui-redesign",
    [string]$TargetBranch = "main",
    [string]$Tag = "desktop-v0.1.0",
    [switch]$SkipPush,
    [switch]$SkipRename
)

$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent
Set-Location $Root

$gh = Get-Command gh -ErrorAction SilentlyContinue
if (-not $gh) { throw "Install GitHub CLI: winget install GitHub.cli" }

if (-not $env:GH_TOKEN) {
    gh auth status 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Run: gh auth login" }
}

$remote = "p1ouson"
$remoteUrl = "https://github.com/$Owner/$Repo.git"
$oldUrl = "https://github.com/$Owner/$OldRepo.git"

if (-not (git remote get-url $remote 2>$null)) {
    git remote add $remote $oldUrl
}

if (-not $SkipPush) {
    Write-Host "==> Push $Branch -> $Owner/$Repo ($TargetBranch)" -ForegroundColor Cyan
    git -c http.version=HTTP/1.1 push -u $remote "${Branch}:${TargetBranch}" --force-with-lease
}

if (-not $SkipRename) {
    $view = gh repo view "$Owner/$OldRepo" --json name -q .name 2>$null
    if ($LASTEXITCODE -eq 0 -and $view -eq $OldRepo) {
        Write-Host "==> Rename repo $OldRepo -> $Repo" -ForegroundColor Cyan
        gh repo rename $Repo --repo "$Owner/$OldRepo" --yes
        git remote set-url $remote $remoteUrl
    } elseif ($view -eq $Repo) {
        git remote set-url $remote $remoteUrl
    }
}

Write-Host "==> Repo metadata" -ForegroundColor Cyan
gh repo edit "$Owner/$Repo" `
    --description "ArcDesk — DeepSeek-native coding agent desktop app (Wails + Go). Windows installer included." `
    --add-topic deepseek --add-topic coding-agent --add-topic wails --add-topic desktop-app --add-topic go

$installer = Join-Path $Root "dist\arcdesk-desktop-amd64-installer.exe"
if (-not (Test-Path $installer)) {
    Write-Host "Building installer..." -ForegroundColor Yellow
    & (Join-Path $Root "desktop\scripts\build-windows-installer.ps1")
    Copy-Item $installer (Join-Path $Root "dist\arcdesk-desktop-amd64-installer.exe") -Force -ErrorAction SilentlyContinue
}

$notes = Join-Path $Root ".github\release-notes\desktop-v0.1.0.md"
if (-not (Test-Path $notes)) { throw "Missing release notes: $notes" }

Write-Host "==> Create release $Tag" -ForegroundColor Cyan
$existing = gh release view $Tag --repo "$Owner/$Repo" 2>$null
if ($LASTEXITCODE -eq 0) {
    gh release upload $Tag $installer --repo "$Owner/$Repo" --clobber
} else {
    gh release create $Tag `
        --repo "$Owner/$Repo" `
        --title "ArcDesk Desktop v0.1.0" `
        --notes-file $notes `
        $installer
}

Write-Host ""
Write-Host "Done:" -ForegroundColor Green
Write-Host "  https://github.com/$Owner/$Repo"
Write-Host "  https://github.com/$Owner/$Repo/releases/tag/$Tag"
