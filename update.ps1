# update.ps1 — Update databasus.exe without reinstalling the service
#
# HOW TO UPDATE:
#   1. Extract the new databasus.exe from the zip and save it as
#      "databasus-new.exe" in the same folder as this script.
#   2. Run update.bat as Administrator (right-click → "Run as administrator").
#
# The script stops the service, swaps the binary, and restarts it.
# The service registration, recovery settings, and data folder are untouched.
#
# REQUIREMENTS:
#   - Run as Administrator
#   - databasus-new.exe must be in the same folder as this script
#   - The Databasus service must already be installed (run install-service.bat first)

param (
    [string]$ServiceName = "Databasus"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Must run as Administrator ──────────────────────────────────────────────────
$currentPrincipal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host ""
    Write-Host "  [ERROR] This script must be run as Administrator." -ForegroundColor Red
    Write-Host "          Right-click update.bat and choose 'Run as administrator'." -ForegroundColor Red
    Write-Host ""
    exit 1
}

# ── Resolve paths ──────────────────────────────────────────────────────────────
$InstallDir  = Split-Path -Parent $MyInvocation.MyCommand.Path
$OldBinary   = Join-Path $InstallDir "databasus.exe"
$NewBinary   = Join-Path $InstallDir "databasus-new.exe"
$LogDir      = Join-Path $InstallDir "databasus-data"

# ── Banner ─────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  Databasus Updater" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Service name : $ServiceName"
Write-Host "  Install dir  : $InstallDir"
Write-Host ""

# ── Step 1 — Pre-flight checks ─────────────────────────────────────────────────
Write-Host "[ 1 / 4 ]  Checking files..." -ForegroundColor White

if (-not (Test-Path $NewBinary)) {
    Write-Host ""
    Write-Host "  [ERROR] databasus-new.exe not found at:" -ForegroundColor Red
    Write-Host "          $NewBinary" -ForegroundColor Red
    Write-Host ""
    Write-Host "  To update:" -ForegroundColor White
    Write-Host "    1. Extract the new databasus.exe from the zip." -ForegroundColor White
    Write-Host "    2. Rename it to databasus-new.exe." -ForegroundColor White
    Write-Host "    3. Place it in the same folder as update.bat." -ForegroundColor White
    Write-Host "    4. Run update.bat again." -ForegroundColor White
    Write-Host ""
    exit 1
}
Write-Host "  [OK]  databasus-new.exe found" -ForegroundColor Green

$svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $svc) {
    Write-Host ""
    Write-Host "  [ERROR] Service '$ServiceName' is not installed." -ForegroundColor Red
    Write-Host "          Run install-service.bat first." -ForegroundColor Red
    Write-Host ""
    exit 1
}
Write-Host "  [OK]  Service '$ServiceName' is installed" -ForegroundColor Green

# ── Step 2 — Stop the service ──────────────────────────────────────────────────
Write-Host ""
Write-Host "[ 2 / 4 ]  Stopping service..." -ForegroundColor White

if ($svc.Status -eq "Running") {
    Stop-Service -Name $ServiceName
    # Wait up to 30 s for the service to stop
    $waited = 0
    do {
        Start-Sleep -Seconds 1
        $waited++
        $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    } while ($svc.Status -ne "Stopped" -and $waited -lt 30)

    if ($svc.Status -ne "Stopped") {
        Write-Host "  [ERROR] Service did not stop within 30 seconds." -ForegroundColor Red
        Write-Host "          Try stopping it manually: Stop-Service $ServiceName" -ForegroundColor Red
        exit 1
    }
    Write-Host "  [OK]  Service stopped." -ForegroundColor Green
} else {
    Write-Host "  [OK]  Service was already stopped." -ForegroundColor Green
}

# ── Step 3 — Swap the binary ───────────────────────────────────────────────────
Write-Host ""
Write-Host "[ 3 / 4 ]  Replacing binary..." -ForegroundColor White

if (Test-Path $OldBinary) {
    Remove-Item $OldBinary -Force
}
Move-Item $NewBinary $OldBinary
Write-Host "  [OK]  databasus.exe replaced." -ForegroundColor Green

# ── Step 4 — Start the service ─────────────────────────────────────────────────
Write-Host ""
Write-Host "[ 4 / 4 ]  Starting service..." -ForegroundColor White

Start-Service -Name $ServiceName
Start-Sleep -Seconds 6

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
Write-Host "  Update complete" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  The service registration and data folder were not changed."
Write-Host ""
Write-Host "  LOG FILE"
Write-Host "  --------"
Write-Host "  $LogDir\databasus.log"
Write-Host ""
