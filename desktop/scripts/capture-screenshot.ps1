# Capture ArcDesk main window to docs/screenshots/desktop-workbench.png
param(
  [string]$Exe = (Join-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) "desktop\build\bin\arcdesk-desktop.exe"),
  [string]$Out = (Join-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) "docs\screenshots\desktop-workbench.png"),
  [int]$WaitSec = 12
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path $Exe)) { throw "Build the desktop app first: desktop\scripts\build-windows-installer.ps1" }

Add-Type @"
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
public static class WinCap {
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }
  public static void Capture(IntPtr hWnd, string path) {
    ShowWindow(hWnd, 9);
    SetForegroundWindow(hWnd);
    System.Threading.Thread.Sleep(400);
    RECT r; GetWindowRect(hWnd, out r);
    int w = r.Right - r.Left; int h = r.Bottom - r.Top;
    if (w < 200 || h < 200) throw new Exception("window too small");
    using (var bmp = new Bitmap(w, h)) {
      using (var g = Graphics.FromImage(bmp)) {
        g.CopyFromScreen(r.Left, r.Top, 0, 0, new Size(w, h));
      }
      Directory.CreateDirectory(Path.GetDirectoryName(path)!);
      bmp.Save(path, ImageFormat.Png);
    }
  }
}
"@

Get-Process -Name "arcdesk-desktop" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep 1
$proc = Start-Process -FilePath $Exe -PassThru
$hwnd = [IntPtr]::Zero
for ($i = 0; $i -lt 40; $i++) {
  $proc.Refresh()
  if ($proc.MainWindowHandle -ne [IntPtr]::Zero) { $hwnd = $proc.MainWindowHandle; break }
  Start-Sleep -Milliseconds 500
}
if ($hwnd -eq [IntPtr]::Zero) { Stop-Process -Id $proc.Id -Force; throw "main window not found" }
Start-Sleep $WaitSec
[WinCap]::Capture($hwnd, (Resolve-Path (Split-Path $Out -Parent) | ForEach-Object { Join-Path $_.Path (Split-Path $Out -Leaf) }))
if (-not (Test-Path $Out)) { $Out = (Resolve-Path $Out -ErrorAction SilentlyContinue) }
Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
Write-Host "saved $Out"
