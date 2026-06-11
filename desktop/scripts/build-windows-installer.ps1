# Build the small Windows setup wizard (NSIS) + full desktop binary.
# Requires: Go, Wails CLI, pnpm (frontend dist), NSIS (makensis on PATH).
#
# Output:
#   desktop/build/bin/arcdesk-desktop-amd64-installer.exe  (~10 MB launcher)
#   desktop/build/bin/arcdesk-desktop.exe                    (installed app)

$ErrorActionPreference = "Stop"
$DesktopRoot = Split-Path $PSScriptRoot -Parent
$RepoRoot = Split-Path $DesktopRoot -Parent

function Ensure-NSIS {
    if (Get-Command makensis -ErrorAction SilentlyContinue) { return }
    $candidates = @(
        "$env:ProgramFiles\NSIS\makensis.exe",
        "${env:ProgramFiles(x86)}\NSIS\makensis.exe"
    )
    foreach ($exe in $candidates) {
        if (Test-Path $exe) {
            $dir = Split-Path $exe -Parent
            $env:Path = "$dir;$env:Path"
            return
        }
    }
    Write-Host "NSIS not found. Install with: winget install NSIS.NSIS" -ForegroundColor Yellow
    throw "makensis is required for the setup wizard."
}

$env:GOTOOLCHAIN = "auto"
Ensure-NSIS

Write-Host "==> App icon (exe + installer)" -ForegroundColor Cyan
python (Join-Path $DesktopRoot "scripts\generate-appicon.py")

$distIndex = Join-Path $DesktopRoot "frontend\dist\index.html"
if (-not (Test-Path $distIndex)) {
    Write-Host "==> Frontend dist missing — building" -ForegroundColor Cyan
    Push-Location (Join-Path $DesktopRoot "frontend")
    pnpm install --no-frozen-lockfile
    pnpm build
    Pop-Location
}

Write-Host "==> Wails desktop + NSIS installer" -ForegroundColor Cyan
Push-Location $DesktopRoot
wails build -nsis -s
Pop-Location

# Installer only embeds arcdesk-desktop.exe (+ WebView2 bootstrapper). Sanity-check staging.
$bin = Join-Path $DesktopRoot "build\bin"
$allowed = @(
    "arcdesk-desktop.exe",
    "arcdesk-desktop-amd64-installer.exe"
)
Get-ChildItem -LiteralPath $bin -File -ErrorAction SilentlyContinue | ForEach-Object {
    if ($allowed -notcontains $_.Name) {
        Write-Warning "Unexpected file in build/bin (will NOT be in installer): $($_.Name)"
    }
}

$installer = Join-Path $DesktopRoot "build\bin\arcdesk-desktop-amd64-installer.exe"
$appExe = Join-Path $DesktopRoot "build\bin\arcdesk-desktop.exe"
if (-not (Test-Path $appExe)) {
    throw "Desktop app not produced: $appExe"
}

Write-Host ""
Write-Host "Desktop app built:" -ForegroundColor Green
Write-Host "  $appExe" -ForegroundColor Green

if (-not (Test-Path $installer)) {
    Write-Host ""
    Write-Host "NSIS installer was not produced (you can still run arcdesk-desktop.exe directly)." -ForegroundColor Yellow
    exit 0
}

$item = Get-Item $installer
Write-Host ""
Write-Host "Done. Distribute this file to users:" -ForegroundColor Green
Write-Host "  $($item.FullName)  ($([math]::Round($item.Length / 1MB, 2)) MB)" -ForegroundColor Green
Write-Host "Users run it to pick an install folder, then get Start Menu + Desktop shortcuts."
