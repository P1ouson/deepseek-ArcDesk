param(
  [string]$Variant = "after",
  [string]$Only = "",
  [string]$OutDir = "desktop/benchmarks"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$env:BENCHMARK_AGENT = "1"
# Desktop stores credentials under Roaming\arcdesk
if (-not $env:APPDATA) { $env:APPDATA = "$env:USERPROFILE\AppData\Roaming" }

Write-Host "Building explorebench ($Variant)..."
go build -o ".\bin\explorebench.exe" .\cmd\explorebench
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$args = @("-variant", $Variant, "-out", $OutDir)
if ($Only) { $args += @("-only", $Only) }

Write-Host "Running explorebench $Variant ..."
& ".\bin\explorebench.exe" @args
exit $LASTEXITCODE
