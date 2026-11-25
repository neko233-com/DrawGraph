# DrawGraph one-click installer for Windows x64 (windows/amd64).
# Usage:
#   irm https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.ps1 | iex
#   $env:DRAWGRAPH_VERSION = "0.1.0"; irm https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

if ($env:OS -ne "Windows_NT") {
    throw "Use install.sh on macOS (darwin/arm64)."
}

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = "neko233-com/DrawGraph"
$version = $env:DRAWGRAPH_VERSION
$msiName = "DrawGraph-windows-amd64.msi"
if ([string]::IsNullOrWhiteSpace($version) -or $version -eq "latest") {
    $baseUrl = "https://github.com/$repo/releases/latest/download"
} else {
    $version = $version -replace '^v', ''
    $baseUrl = "https://github.com/$repo/releases/download/v$version"
}

$msiUrl = "$baseUrl/$msiName"
$shaUrl = "$baseUrl/$msiName.sha256"
$msi = Join-Path $env:TEMP "DrawGraph-$([guid]::NewGuid().ToString('N')).msi"

Write-Host "Downloading $msiUrl"
try {
    Invoke-WebRequest -Uri $msiUrl -OutFile $msi -UseBasicParsing
    try {
        $shaText = (Invoke-WebRequest -Uri $shaUrl -UseBasicParsing).Content.Trim()
        $want = ($shaText -split '\s+')[0].ToLowerInvariant()
        $got = (Get-FileHash -Algorithm SHA256 $msi).Hash.ToLowerInvariant()
        if ($want.Length -eq 64 -and $got -ne $want) {
            throw "SHA-256 checksum mismatch! got=$got want=$want"
        }
        Write-Host "Verified SHA-256: $got"
    } catch {
        Write-Warning "Checksum verification failed: $_"
        throw
    }

    Write-Host "Installing DrawGraph (administrator permission is required)..."
    $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).
        IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if ($isAdmin) {
        $process = Start-Process msiexec.exe -ArgumentList @("/i", "`"$msi`"", "/passive", "/norestart") -Wait -PassThru
    } else {
        $process = Start-Process msiexec.exe -ArgumentList @("/i", "`"$msi`"", "/passive", "/norestart") -Verb RunAs -Wait -PassThru
    }
    if ($process.ExitCode -ne 0 -and $process.ExitCode -ne 3010) {
        throw "msiexec failed with exit code $($process.ExitCode)"
    }
    Write-Host ""
    Write-Host "DrawGraph is installed."
    Write-Host "  App: C:\Program Files\DrawGraph\drawgraph-gui.exe (Start menu: DrawGraph)"
    Write-Host "  CLI: C:\Program Files\DrawGraph\drawgraph.exe (added to PATH)"
} finally {
    Remove-Item -Force $msi -ErrorAction SilentlyContinue
}
