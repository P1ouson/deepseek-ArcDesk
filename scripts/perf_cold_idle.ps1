# Cold start + 2min idle sampling for arcdesk-desktop.exe
param(
  [string]$Exe = "d:\reasonix_code\DeepSeek-Reasonix-desktop-v1.2.1\desktop\build\bin\arcdesk-desktop.exe",
  [int]$IdleSeconds = 120,
  [int]$SampleEverySec = 5
)

$ErrorActionPreference = "Stop"
Get-Process -Name "arcdesk-desktop" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep 1

$sw = [System.Diagnostics.Stopwatch]::StartNew()
$proc = Start-Process -FilePath $Exe -PassThru
$startMs = $sw.ElapsedMilliseconds

# Wait for main window (best-effort)
$windowMs = $null
for ($i = 0; $i -lt 60; $i++) {
  $proc.Refresh()
  if ($proc.MainWindowHandle -ne 0) {
    $windowMs = $sw.ElapsedMilliseconds
    break
  }
  Start-Sleep -Milliseconds 500
}

$samples = @()
$end = (Get-Date).AddSeconds($IdleSeconds)
while ((Get-Date) -lt $end) {
  try {
    $p = Get-Process -Id $proc.Id -ErrorAction Stop
    $cpu = $p.CPU
    $ws = $p.WorkingSet64
    $pm = $p.PrivateMemorySize64
    $samples += [PSCustomObject]@{
      t_sec = [math]::Round($sw.Elapsed.TotalSeconds, 1)
      working_set_mb = [math]::Round($ws / 1MB, 2)
      private_mb = [math]::Round($pm / 1MB, 2)
      cpu_total_sec = [math]::Round($cpu, 3)
    }
  } catch {
    break
  }
  Start-Sleep -Seconds $SampleEverySec
}

Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue

$cpuDeltas = @()
for ($i = 1; $i -lt $samples.Count; $i++) {
  $dt = $samples[$i].t_sec - $samples[$i-1].t_sec
  if ($dt -gt 0) {
    $cpuDeltas += ($samples[$i].cpu_total_sec - $samples[$i-1].cpu_total_sec) / $dt
  }
}
$avgCpuPct = if ($cpuDeltas.Count -gt 0) { [math]::Round(($cpuDeltas | Measure-Object -Average).Average * 100 / [Environment]::ProcessorCount, 2) } else { 0 }

$first = $samples | Select-Object -First 1
$last = $samples | Select-Object -Last 1
$ramGrowth = if ($first -and $last) { [math]::Round($last.working_set_mb - $first.working_set_mb, 2) } else { 0 }

$result = [PSCustomObject]@{
  process_start_ms = $startMs
  main_window_ms = $windowMs
  idle_seconds = $IdleSeconds
  sample_count = $samples.Count
  idle_avg_cpu_pct_per_core = $avgCpuPct
  idle_ram_start_mb = $first.working_set_mb
  idle_ram_end_mb = $last.working_set_mb
  idle_ram_growth_mb = $ramGrowth
  samples = $samples
}

$out = Join-Path $PSScriptRoot "..\benchmarks\perf\cold_idle.json"
New-Item -ItemType Directory -Force -Path (Split-Path $out) | Out-Null
$result | ConvertTo-Json -Depth 5 | Set-Content $out -Encoding UTF8
Write-Output $result | ConvertTo-Json -Depth 3
