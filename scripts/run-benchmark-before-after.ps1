param(
  [string]$OutDir = "desktop/benchmarks"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$env:GOTOOLCHAIN = "auto"
$backup = ".benchmark-backup"
$readfile = "internal/tool/builtin/readfile.go"
$agent = "internal/agent/agent.go"

New-Item -ItemType Directory -Force -Path $backup | Out-Null
Copy-Item $readfile "$backup/readfile.go" -Force
Copy-Item $agent "$backup/agent.go" -Force

function Restore-Optimized {
  Copy-Item "$backup/readfile.go" $readfile -Force
  Copy-Item "$backup/agent.go" $agent -Force
}

function Apply-Before {
  git show HEAD:internal/tool/builtin/readfile.go | Set-Content -Encoding utf8NoBOM $readfile
  $a = Get-Content $agent -Raw
  $a = $a -replace '(?s)func \(a \*Agent\) exploreParallelism\(\) int \{.*?\n\}', @'
func (a *Agent) exploreParallelism() int {
	return 8
}
'@
  $a = $a -replace 'par := a\.exploreParallelism\(\)\s*\r?\n\s*benchagent\.RecordParallelBatch\(par, batch\.end-batch\.start\)\s*\r?\n\s*runParallel\(batch\.start, batch\.end, par, run\)', 'par := a.exploreParallelism()`r`n			benchagent.RecordParallelBatch(par, batch.end-batch.start)`r`n			runParallel(batch.start, batch.end, par, run)'
  Set-Content -Encoding utf8NoBOM $agent $a
}

try {
  Write-Host "=== BEFORE baseline (read_file=2000, fanout=8) ==="
  Apply-Before
  & "$PSScriptRoot/run-explore-benchmark.ps1" -Variant before -OutDir $OutDir
  if ($LASTEXITCODE -ne 0) { throw "before benchmark failed with exit $LASTEXITCODE" }

  Write-Host "=== AFTER optimized ==="
  Restore-Optimized
  & "$PSScriptRoot/run-explore-benchmark.ps1" -Variant after -OutDir $OutDir
  if ($LASTEXITCODE -ne 0) { throw "after benchmark failed with exit $LASTEXITCODE" }
}
finally {
  Restore-Optimized
}

Write-Host "Reports written under $OutDir"
