# Dev build: frontend + Wails desktop exe (no NSIS installer).
# Run from anywhere:  powershell -File desktop/build-dev.ps1
$ErrorActionPreference = "Stop"
$DesktopRoot = $PSScriptRoot

$env:GOTOOLCHAIN = "auto"

Write-Host "==> Frontend" -ForegroundColor Cyan
Push-Location (Join-Path $DesktopRoot "frontend")
npm run build
Pop-Location

Write-Host "==> Wails desktop exe" -ForegroundColor Cyan
Push-Location $DesktopRoot
wails build
Pop-Location

$exe = Join-Path $DesktopRoot "build\bin\arcdesk-desktop.exe"
if (-not (Test-Path $exe)) {
    throw "Build failed: $exe not found"
}

Write-Host ""
Write-Host "Built:" -ForegroundColor Green
Write-Host "  $exe" -ForegroundColor Green
