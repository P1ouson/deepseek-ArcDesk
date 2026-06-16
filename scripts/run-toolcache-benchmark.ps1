param(
  [string]$OutDir = "desktop/benchmarks",
  [string]$Fixture = ""
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$env:BENCHMARK_AGENT = "1"

Write-Host "Building toolcachebench..."
go build -o ".\bin\toolcachebench.exe" .\cmd\toolcachebench
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$args = @("-out", $OutDir, "-check")
if ($Fixture) { $args += @("-fixture", $Fixture) }

Write-Host "Running toolcachebench (cache-off vs cache-on)..."
& ".\bin\toolcachebench.exe" @args
exit $LASTEXITCODE
