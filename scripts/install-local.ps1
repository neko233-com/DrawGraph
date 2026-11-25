# Builds (unless -SkipBuild) and installs DrawGraph Windows x64 MSI locally so the machine
# always runs the latest local build. Requests UAC elevation automatically when non-elevated.
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\install-local.ps1 -Version 0.1.0
param(
    [string]$Version = "",
    [string]$MsiPath = "dist\DrawGraph-windows-amd64.msi",
    [switch]$SkipBuild,
    [switch]$ValidateOnly
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if (-not $SkipBuild) {
    if ([string]::IsNullOrWhiteSpace($Version)) {
        & (Join-Path $PSScriptRoot "build-msi.ps1")
    } else {
        & (Join-Path $PSScriptRoot "build-msi.ps1") -Version $Version
    }
    if ($LASTEXITCODE -ne 0) { throw "build-msi.ps1 failed" }
}

$resolved = (Resolve-Path $MsiPath).Path

if ($ValidateOnly) {
    $stage = Join-Path $env:TEMP "drawgraph-msi-check"
    Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
    $p = Start-Process msiexec.exe -ArgumentList @("/a", "`"$resolved`"", "/qn", "TARGETDIR=`"$stage`"") -Wait -PassThru
    if ($p.ExitCode -ne 0) { throw "msiexec /a failed ($($p.ExitCode))" }
    Get-ChildItem -Recurse $stage | Select-Object FullName, Length
    & (Join-Path $stage "PFiles64\DrawGraph\drawgraph.exe") version
    Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
    return
}

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).
    IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

Write-Host "Installing $resolved..."
if ($isAdmin) {
    $proc = Start-Process msiexec.exe -ArgumentList @("/i", "`"$resolved`"", "/passive", "/norestart") -Wait -PassThru
} else {
    $proc = Start-Process msiexec.exe -ArgumentList @("/i", "`"$resolved`"", "/passive", "/norestart") -Verb RunAs -Wait -PassThru
}
if ($proc.ExitCode -ne 0 -and $proc.ExitCode -ne 3010) {
    throw "msiexec install failed with exit code $($proc.ExitCode)"
}

$installDir = "C:\Program Files\DrawGraph"
if (-not (Test-Path (Join-Path $installDir "drawgraph.exe"))) {
    throw "drawgraph.exe missing after install under $installDir"
}
if (-not (Test-Path (Join-Path $installDir "drawgraph-gui.exe"))) {
    throw "drawgraph-gui.exe missing after install under $installDir"
}

Write-Host ""
Write-Host "Installed verification:"
& (Join-Path $installDir "drawgraph.exe") version
Get-StartApps | Where-Object { $_.Name -eq "DrawGraph" } | Format-Table Name, AppID
