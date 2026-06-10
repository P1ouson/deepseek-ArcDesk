# Remove local build artifacts. Keeps source + committed icons/installer template.
# Windows release upload artifact (if present) is copied to dist/ before bin cleanup.
param(
    [switch]$SkipDistCopy
)

$ErrorActionPreference = "Stop"
$Root = Split-Path $PSScriptRoot -Parent

function Remove-TreeIfExists([string]$Path) {
    if (Test-Path $Path) {
        Remove-Item -LiteralPath $Path -Recurse -Force
        Write-Host "removed $Path"
    }
}

function Clear-DirKeepGitkeep([string]$Dir) {
    if (-not (Test-Path $Dir)) { return }
    Get-ChildItem -LiteralPath $Dir -Force | Where-Object { $_.Name -ne ".gitkeep" } | Remove-Item -Recurse -Force
    Write-Host "cleared $Dir (kept .gitkeep)"
}

$installer = Join-Path $Root "desktop\build\bin\arcdesk-desktop-amd64-installer.exe"
if (-not $SkipDistCopy -and (Test-Path $installer)) {
    $dist = Join-Path $Root "dist"
    New-Item -ItemType Directory -Force -Path $dist | Out-Null
    Copy-Item -LiteralPath $installer -Destination (Join-Path $dist "arcdesk-desktop-amd64-installer.exe") -Force
    Write-Host "saved upload artifact -> dist/arcdesk-desktop-amd64-installer.exe"
}

Remove-TreeIfExists (Join-Path $Root "bin")
Remove-TreeIfExists (Join-Path $Root "stage")
Remove-TreeIfExists (Join-Path $Root "npm\.stage")
Remove-TreeIfExists (Join-Path $Root "desktop\build\bin")
Remove-TreeIfExists (Join-Path $Root "desktop\frontend\test-results")
Remove-TreeIfExists (Join-Path $Root "desktop\frontend\playwright-report")
Remove-TreeIfExists (Join-Path $Root "desktop\frontend\wailsjs")
Remove-TreeIfExists (Join-Path $Root "desktop\desktop")

Clear-DirKeepGitkeep (Join-Path $Root "desktop\frontend\dist")

$generated = @(
    "desktop\build\windows\icon.ico",
    "desktop\build\windows\info.json",
    "desktop\build\windows\wails.exe.manifest",
    "desktop\build\windows\wails_tools.nsh",
    "desktop\frontend\package.json.md5"
)
foreach ($rel in $generated) {
    $path = Join-Path $Root $rel
    if (Test-Path $path) {
        Remove-Item -LiteralPath $path -Force
        Write-Host "removed $rel"
    }
}

Write-Host ""
Write-Host "Clean complete. Rebuild with:" -ForegroundColor Green
Write-Host "  desktop/scripts/build-windows-installer.ps1"
