param(
  [ValidateSet("before","after")]
  [string]$Mode = "after",
  [string]$OutDir = "desktop/benchmarks",
  [string]$Only = ""
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$env:GOTOOLCHAIN = "auto"
$env:BENCHMARK_AGENT = "1"

$backup = ".benchmark-backup"
$readfile = "internal/tool/builtin/readfile.go"
$agent = "internal/agent/agent.go"

if (-not (Test-Path $backup)) {
  New-Item -ItemType Directory -Force -Path $backup | Out-Null
  Copy-Item $readfile "$backup/readfile.go" -Force
  Copy-Item $agent "$backup/agent.go" -Force
}

function Use-Optimized {
  Copy-Item "$backup/readfile.go" $readfile -Force
  Copy-Item "$backup/agent.go" $agent -Force
}

function Use-Baseline {
  git checkout HEAD -- $readfile $agent
  & go run ./scripts/benchagenthook -agent $agent
  if ($LASTEXITCODE -ne 0) { throw "benchagenthook failed" }
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

if ($Mode -eq "before") { Use-Baseline } else { Use-Optimized }

go build -o bin/explorebench.exe ./cmd/explorebench
if ($LASTEXITCODE -ne 0) { throw "build failed" }

$args = @("-variant", $Mode, "-out", $OutDir)
if ($Only) { $args += @("-only", $Only) }
& .\bin\explorebench.exe @args
$code = $LASTEXITCODE

Use-Optimized
exit $code
