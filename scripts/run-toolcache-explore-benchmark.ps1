param(
  [string]$Only = "small",
  [string]$OutDir = "desktop/benchmarks"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

if (-not $env:APPDATA) { $env:APPDATA = "$env:USERPROFILE\AppData\Roaming" }
$env:BENCHMARK_AGENT = "1"

$configPath = Join-Path (Get-Location) "arcdesk.toml"
if (-not (Test-Path $configPath)) {
  throw "missing arcdesk.toml at repo root"
}

function Set-ToolCacheConfig([bool]$Enabled) {
  $lines = Get-Content $configPath
  $out = New-Object System.Collections.Generic.List[string]
  $inSection = $false
  $done = $false
  foreach ($line in $lines) {
    if ($line -match '^\[tool_cache\]') {
      $inSection = $true
      $out.Add($line)
      continue
    }
    if ($inSection -and $line -match '^\[') {
      $inSection = $false
    }
    if ($inSection -and $line -match '^\s*enabled\s*=') {
      $out.Add("enabled = " + ($(if ($Enabled) { "true" } else { "false" })))
      $done = $true
      continue
    }
    if (-not $inSection) { $out.Add($line) }
  }
  if (-not $done) {
    $out.Add("")
    $out.Add("[tool_cache]")
    $out.Add("enabled = " + ($(if ($Enabled) { "true" } else { "false" })))
  }
  Set-Content -Path $configPath -Value ($out -join "`n") -Encoding utf8
}

Write-Host "Building explorebench..."
go build -o ".\bin\explorebench.exe" .\cmd\explorebench
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "=== cache OFF (tool_cache.enabled=false) ==="
Set-ToolCacheConfig $false
& ".\bin\explorebench.exe" -variant cache-off -only $Only -out $OutDir
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "=== cache ON (tool_cache.enabled=true) ==="
Set-ToolCacheConfig $true
& ".\bin\explorebench.exe" -variant cache-on -only $Only -out $OutDir
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Summarizing..."
go run ./cmd/summarizebench -dir $OutDir -out "$OutDir/toolcache-explore-summary.md" 2>$null
go run ./cmd/toolcachebench -fixture benchmarks/fixtures/toolcache -out $OutDir 2>$null | Out-Null

$off = Get-ChildItem "$OutDir/benchmark-$Only-cache-off-*.json" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
$on  = Get-ChildItem "$OutDir/benchmark-$Only-cache-on-*.json" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if ($off -and $on) {
  $a = Get-Content $off.FullName -Encoding UTF8 -Raw | ConvertFrom-Json
  $b = Get-Content $on.FullName -Encoding UTF8 -Raw | ConvertFrom-Json
  Write-Host ""
  Write-Host "=== explorebench local tool cache ($Only) ==="
  Write-Host ("  tool calls          : off={0} on={1}" -f $a.toolUsage.totalToolCalls, $b.toolUsage.totalToolCalls)
  Write-Host ("  duplicate calls     : off={0} on={1}" -f $a.localToolCache.duplicateCalls, $b.localToolCache.duplicateCalls)
  Write-Host ("  cache hits          : off={0} on={1}" -f $a.localToolCache.hits, $b.localToolCache.hits)
  Write-Host ("  cache misses        : off={0} on={1}" -f $a.localToolCache.misses, $b.localToolCache.misses)
  Write-Host ("  execute reduction   : off={0}% on={1}%" -f $a.localToolCache.executeReductionPct, $b.localToolCache.executeReductionPct)
  Write-Host ("  cached results      : off={0} on={1}" -f $a.localToolCache.cachedResults, $b.localToolCache.cachedResults)
  Write-Host ("  task time (ms)      : off={0} on={1}" -f $a.timings.taskCompletedMs, $b.timings.taskCompletedMs)
  Write-Host ("  prompt tokens       : off={0} on={1}" -f $a.api.totalPromptTokens, $b.api.totalPromptTokens)
  Write-Host ""
  Write-Host "Reports:"
  Write-Host "  $($off.FullName)"
  Write-Host "  $($on.FullName)"
}

Write-Host "Done. tool_cache left enabled=true in arcdesk.toml"
