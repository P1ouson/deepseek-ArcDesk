param(
  [string]$OutDir = "desktop/benchmarks",
  [string]$Fixture = ""
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$env:GOTOOLCHAIN = "auto"
$env:BENCHMARK_AGENT = "1"

Write-Host "=== metrics acceptance (go test) ==="
go test ./internal/agent/... -run "TestToolCacheMetricsAcceptance|TestToolCacheP2NormalizationLift|TestToolCacheFixtureMetricsAcceptance" -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./internal/reporeuse/... ./internal/workspacerefresh/... -run "TestP4|TestRefreshIfStaleMetaBump" -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./internal/plancache/... ./internal/agent/... -run "TestP5|TestPlanCache" -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./internal/failuremem/... -run "TestRankedSearchSmart|TestTokenSimilarity" -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./internal/knowledge/... -run "TestKnowledgeProvenanceInjectionLift|TestP6SemanticFallbackInjectionLift" -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go test ./internal/prefixruntime/... ./internal/verifyselect/... ./internal/agent/... -run "TestP7|TestForProviderRequest|TestMinimumChecks|TestHealthMonitor|TestRumination" -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "=== fixture benchmark (toolcachebench -check) ==="
go build -o ".\bin\toolcachebench.exe" .\cmd\toolcachebench
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$args = @("-out", $OutDir, "-check")
if ($Fixture) { $args += @("-fixture", $Fixture) }

& ".\bin\toolcachebench.exe" @args
exit $LASTEXITCODE
