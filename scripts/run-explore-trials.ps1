param(
  [int]$Trials = 3,
  [string]$OutDir = "desktop/benchmarks",
  [switch]$SummarizeOnly
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

function Median([double[]]$vals) {
  $s = $vals | Sort-Object
  $n = $s.Count
  if ($n -eq 0) { return 0 }
  if ($n % 2 -eq 1) { return $s[[int]($n / 2)] }
  return ($s[$n / 2 - 1] + $s[$n / 2]) / 2.0
}

function Summarize($paths, $label) {
  if ($paths.Count -eq 0) { throw "no benchmark JSON files for $label" }
  $rows = @()
  foreach ($p in $paths) {
    if (-not (Test-Path -LiteralPath $p)) { throw "missing report: $p" }
    $jsonText = [System.IO.File]::ReadAllText($p)
    $j = $jsonText | ConvertFrom-Json
    $rows += [PSCustomObject]@{
      Path = $p
      TaskMs = [double]$j.timings.taskCompletedMs
      ToolCalls = [int]$j.toolUsage.totalToolCalls
      Turns = [int]$j.api.totalAgentTurns
      PromptTokens = [int]$j.api.totalPromptTokens
      Cost = [double]$j.api.estimatedCost
      LocalCacheHits = [int]$j.localToolCache.hits
      KvHitRate = [double]$j.cache.avgHitRate * 100
      FanoutMax = [int]$j.fanout.maxConcurrency
    }
  }
  [PSCustomObject]@{
    variant = $label
    trials = $rows.Count
    medianTaskMs = [math]::Round((Median ($rows | ForEach-Object { $_.TaskMs })), 0)
    medianToolCalls = [math]::Round((Median ($rows | ForEach-Object { [double]$_.ToolCalls })), 0)
    medianTurns = [math]::Round((Median ($rows | ForEach-Object { [double]$_.Turns })), 0)
    medianPromptTokens = [math]::Round((Median ($rows | ForEach-Object { [double]$_.PromptTokens })), 0)
    medianCost = [math]::Round((Median ($rows | ForEach-Object { $_.Cost })), 4)
    medianLocalCacheHits = [math]::Round((Median ($rows | ForEach-Object { [double]$_.LocalCacheHits })), 0)
    medianKvHitRate = [math]::Round((Median ($rows | ForEach-Object { $_.KvHitRate })), 1)
    fanoutMax = ($rows | Select-Object -First 1).FanoutMax
    runs = $rows
  }
}

function Latest-TrialReports([string]$variant, [int]$count) {
  Get-ChildItem (Join-Path $OutDir "benchmark-small-$variant-*.json") |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First $count |
    Sort-Object LastWriteTime |
    ForEach-Object { $_.FullName }
}

function Write-Summary($afterPaths, $beforePaths) {
  $afterSum = Summarize $afterPaths "after"
  $beforeSum = Summarize $beforePaths "before"
  $summary = [PSCustomObject]@{
    generatedAt = (Get-Date).ToString("o")
    trialsPerVariant = $Trials
    after = $afterSum
    before = $beforeSum
    delta = [PSCustomObject]@{
      taskMsPct = if ($beforeSum.medianTaskMs -gt 0) { [math]::Round((1 - $afterSum.medianTaskMs / $beforeSum.medianTaskMs) * 100, 1) } else { $null }
      toolCalls = $afterSum.medianToolCalls - $beforeSum.medianToolCalls
      promptTokensPct = if ($beforeSum.medianPromptTokens -gt 0) { [math]::Round((1 - $afterSum.medianPromptTokens / $beforeSum.medianPromptTokens) * 100, 1) } else { $null }
      costPct = if ($beforeSum.medianCost -gt 0) { [math]::Round((1 - $afterSum.medianCost / $beforeSum.medianCost) * 100, 1) } else { $null }
      kvHitRatePp = [math]::Round($afterSum.medianKvHitRate - $beforeSum.medianKvHitRate, 1)
    }
  }
  $outPath = Join-Path $OutDir "explore-trials-summary.json"
  $summary | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 $outPath
  Write-Host ""
  Write-Host "========== MEDIAN SUMMARY =========="
  $summary | ConvertTo-Json -Depth 5
  Write-Host ""
  Write-Host "wrote $outPath"
}

if ($SummarizeOnly) {
  $afterPaths = @(Latest-TrialReports "after" $Trials)
  $beforePaths = @(Latest-TrialReports "before" $Trials)
  if ($afterPaths.Count -lt $Trials -or $beforePaths.Count -lt $Trials) {
    throw "need $Trials after and $Trials before reports under $OutDir (have after=$($afterPaths.Count) before=$($beforePaths.Count))"
  }
  Write-Summary $afterPaths $beforePaths
  exit 0
}

$env:GOTOOLCHAIN = "auto"
$env:BENCHMARK_AGENT = "1"
if (-not $env:APPDATA) { $env:APPDATA = "$env:USERPROFILE\AppData\Roaming" }

go build -o ".\bin\explorebench.exe" .\cmd\explorebench
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$readfile = "internal/tool/builtin/readfile.go"
$agent = "internal/agent/agent.go"
$backup = ".benchmark-backup-live"
New-Item -ItemType Directory -Force -Path $backup | Out-Null

function Restore-Opt {
  Copy-Item "$backup/readfile.go" $readfile -Force
  Copy-Item "$backup/agent.go" $agent -Force
}

function Apply-Before {
  Copy-Item $readfile "$backup/readfile.go" -Force
  Copy-Item $agent "$backup/agent.go" -Force
  git show HEAD:internal/tool/builtin/readfile.go | Set-Content -Encoding UTF8 $readfile
  $a = (Get-Content $agent | Out-String)
  $a = $a -replace '(?s)func \(a \*Agent\) exploreParallelism\(\) int \{.*?\n\}', "func (a *Agent) exploreParallelism() int {`r`n`treturn 8`r`n}"
  Set-Content -Encoding UTF8 $agent $a
  go build -o ".\bin\explorebench.exe" .\cmd\explorebench
  if ($LASTEXITCODE -ne 0) { throw "build failed after Apply-Before" }
}

function Run-Trials($variant) {
  $paths = @()
  for ($i = 1; $i -le $Trials; $i++) {
    Write-Host "=== $variant trial $i/$Trials ==="
    & ".\bin\explorebench.exe" -variant $variant -only small -out $OutDir 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "explorebench $variant trial $i failed" }
    $latest = Get-ChildItem "$OutDir/benchmark-small-$variant-*.json" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if ($null -eq $latest) { throw "no report for $variant trial $i" }
    $paths += $latest.FullName
    Write-Host "report $($latest.Name)"
  }
  return ,$paths
}

Write-Host "========== AFTER (optimized) x$Trials =========="
$afterPaths = @(Run-Trials "after")

Write-Host "========== BEFORE (baseline) x$Trials =========="
try {
  Apply-Before
  $beforePaths = @(Run-Trials "before")
}
finally {
  Restore-Opt
  go build -o ".\bin\explorebench.exe" .\cmd\explorebench | Out-Null
}

Write-Summary $afterPaths $beforePaths
