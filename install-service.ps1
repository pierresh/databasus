# install-service.ps1 — Register databasus.exe as a Windows service
#
# Installs databasus.exe as a Windows service that:
#   - Starts automatically when Windows boots
#   - Restarts automatically if the process crashes
#   - Writes server output to databasus-data\databasus.log
#
# Run this script ONCE after deploying databasus.exe on a new server.
# Safe to re-run — it removes and recreates the service if it already exists.
#
# REQUIREMENTS:
#   - Run as Administrator (right-click PowerShell → "Run as administrator")
#   - databasus.exe must be in the same folder as this script
#
# USAGE:
#   .\install-service.ps1
#
# To use a custom service name (e.g. when running two instances):
#   .\install-service.ps1 -ServiceName Databasus2

param (
    [string]$ServiceName = "Databasus"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$DisplayName = "Databasus Backup Service"
$Description = "Databasus database backup service — runs scheduled backups automatically."

# ── Must run as Administrator ──────────────────────────────────────────────────
$currentPrincipal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host ""
    Write-Host "  [ERROR] This script must be run as Administrator." -ForegroundColor Red
    Write-Host "          Right-click PowerShell and choose 'Run as administrator'," -ForegroundColor Red
    Write-Host "          then run this script again." -ForegroundColor Red
    Write-Host ""
    exit 1
}

# ── Resolve paths ──────────────────────────────────────────────────────────────
$InstallDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Binary     = Join-Path $InstallDir "databasus.exe"
$LogDir     = Join-Path $InstallDir "databasus-data"

# ── Banner ─────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  Databasus Service Installer" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Service name : $ServiceName"
Write-Host "  Binary       : $Binary"
Write-Host "  Log folder   : $LogDir"
Write-Host ""

# ── Step 1 — Pre-flight checks ─────────────────────────────────────────────────
Write-Host "[ 1 / 4 ]  Checking files..." -ForegroundColor White

if (-not (Test-Path $Binary)) {
    Write-Host ""
    Write-Host "  [ERROR] databasus.exe not found at:" -ForegroundColor Red
    Write-Host "          $Binary" -ForegroundColor Red
    Write-Host "          Make sure install-service.ps1 is in the same folder as databasus.exe." -ForegroundColor Red
    Write-Host ""
    exit 1
}
Write-Host "  [OK]  databasus.exe found" -ForegroundColor Green

# ── Step 2 — Remove existing service (idempotent) ─────────────────────────────
Write-Host ""
Write-Host "[ 2 / 4 ]  Checking for existing service..." -ForegroundColor White

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "  A previous '$ServiceName' service was found — removing it first..." -ForegroundColor Yellow
    & $Binary --uninstall-service 2>&1 | Out-Null
    Start-Sleep -Seconds 2
    Write-Host "  [OK]  Previous service removed." -ForegroundColor Green
} else {
    Write-Host "  [OK]  No existing service found." -ForegroundColor Green
}

# ── Step 3 — Install and configure service ─────────────────────────────────────
Write-Host ""
Write-Host "[ 3 / 4 ]  Registering service..." -ForegroundColor White

& $Binary --install-service
if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "  [ERROR] Failed to install the service (exit code $LASTEXITCODE)." -ForegroundColor Red
    exit 1
}

# Restart automatically on failure: after 5 s on the first two crashes, after 30 s
# on any subsequent crash. Reset the failure count after 24 hours of clean running.
sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/5000/restart/30000 | Out-Null

Write-Host "  [OK]  Service '$ServiceName' registered successfully." -ForegroundColor Green

# ── Step 4 — Start service ─────────────────────────────────────────────────────
Write-Host ""
Write-Host "[ 4 / 4 ]  Starting service..." -ForegroundColor White

Start-Service -Name $ServiceName
# Give it a few seconds to initialise and extract tools on first run
Start-Sleep -Seconds 8

$svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq "Running") {
    Write-Host "  [OK]  Service is running." -ForegroundColor Green
} else {
    $status = if ($svc) { $svc.Status } else { "unknown" }
    Write-Host "  [!!]  Service did not start (status: $status)." -ForegroundColor Yellow
    Write-Host "        Check the log for details:" -ForegroundColor Yellow
    Write-Host "          $LogDir\databasus.log" -ForegroundColor Yellow
}

# ── Summary ────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  Installation complete" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Service     : $ServiceName  ($DisplayName)"
Write-Host "  Auto-start  : Yes — starts automatically at Windows boot"
Write-Host "  On crash    : Restarts automatically (after 5 s)"
Write-Host ""
Write-Host "  USEFUL COMMANDS  (run in PowerShell as Administrator)"
Write-Host "  ----------------------------------------------------"
Write-Host "  Start   :  Start-Service $ServiceName"
Write-Host "  Stop    :  Stop-Service  $ServiceName"
Write-Host "  Status  :  Get-Service   $ServiceName"
Write-Host ""
Write-Host "  NOTE: In PowerShell, 'sc' is an alias for Set-Content."
Write-Host "        Use 'sc.exe' if you prefer the classic syntax:"
Write-Host "  Start   :  sc.exe start  $ServiceName"
Write-Host "  Stop    :  sc.exe stop   $ServiceName"
Write-Host "  Status  :  sc.exe query  $ServiceName"
Write-Host ""
Write-Host "  LOG FILE"
Write-Host "  --------"
Write-Host "  $LogDir\databasus.log"
Write-Host ""
