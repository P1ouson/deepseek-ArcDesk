param(
  [string]$OutDir = "desktop/benchmarks"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$scenarios = @("small", "medium", "large", "coding")
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

foreach ($sc in $scenarios) {
  Write-Host "`n===== $sc BEFORE ====="
  & "$PSScriptRoot/run-explore-variant.ps1" -Mode before -OutDir $OutDir -Only $sc
  if ($LASTEXITCODE -ne 0) { Write-Warning "$sc before failed" }

  Write-Host "`n===== $sc AFTER ====="
  & "$PSScriptRoot/run-explore-variant.ps1" -Mode after -OutDir $OutDir -Only $sc
  if ($LASTEXITCODE -ne 0) { Write-Warning "$sc after failed" }
}

Write-Host "`nAll scenarios attempted. JSON reports in $OutDir"
