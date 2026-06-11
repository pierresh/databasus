@echo off
:: install-service.bat - Install databasus.exe as a Windows service
::
:: Double-click this file as Administrator to install Databasus as a service.
:: It launches install-service.ps1 with -ExecutionPolicy Bypass so you do
:: not need to change the system PowerShell execution policy.
::
:: REQUIREMENT: Right-click this file and choose "Run as administrator"

:: Check for admin rights
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo.
    echo  [ERROR] This script must be run as Administrator.
    echo          Right-click this file and choose "Run as administrator",
    echo          then try again.
    echo.
    pause
    exit /b 1
)

:: Launch the PowerShell installer.
:: -ExecutionPolicy Bypass applies only to this PowerShell process, not system-wide.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-service.ps1"

pause
