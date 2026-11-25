# Builds the Windows x64 (windows/amd64) MSI into dist\DrawGraph-windows-amd64.msi
# Usage: powershell -ExecutionPolicy Bypass -File scripts\build-msi.ps1 -Version 0.1.0
param(
    [string]$Version = "",
    [string]$OutDir = "dist",
    [string]$Channel = "stable"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if ([string]::IsNullOrWhiteSpace($Version)) {
    try {
        $describe = (git describe --tags --abbrev=0 2>$null).Trim()
        if ($describe -match '^v?(\d+\.\d+\.\d+)') {
            $Version = $Matches[1]
        }
    } catch {}
}
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "0.1.0"
}
$Version = $Version -replace '^v', ''
if ($Version -notmatch '^\d+\.\d+\.\d+') {
    throw "Version must look like 0.1.0 (got '$Version')"
}

$wix = Get-Command wix.exe -ErrorAction SilentlyContinue
if (-not $wix) {
    $fallback = Join-Path $env:USERPROFILE ".dotnet\tools\wix.exe"
    if (Test-Path $fallback) {
        $wix = Get-Item $fallback
    } else {
        throw "WiX v5 (wix.exe) not found. Install via: dotnet tool install --global wix"
    }
}

$payload = Join-Path $root "build\payload-msi"
$obj = Join-Path $root "build\wixobj"
Remove-Item -Recurse -Force $payload, $obj -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $payload, $obj, $OutDir | Out-Null

$commit = "unknown"
try { $commit = ((git rev-parse --short HEAD) 2>$null).Trim() } catch {}
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-dd")
$ldflags = "-s -w -buildid= -X github.com/neko233-com/DrawGraph/internal/version.Version=$Version -X github.com/neko233-com/DrawGraph/internal/version.Commit=$commit -X github.com/neko233-com/DrawGraph/internal/version.Date=$date -X github.com/neko233-com/DrawGraph/internal/version.Channel=$Channel"
$guiLdflags = "$ldflags -H=windowsgui"

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

Write-Host "Embedding Windows resource (.syso) for $Version..."
& go run ./scripts/gen-winres -version $Version -ico assets/drawgraph.ico -out cmd/drawgraph-gui/rsrc_windows_amd64.syso
if ($LASTEXITCODE -ne 0) { throw "gen-winres failed" }

Write-Host "Building drawgraph.exe (Agent CLI)..."
& go build -trimpath -buildvcs=false -ldflags $ldflags -o (Join-Path $payload "drawgraph.exe") ./cmd/drawgraph
if ($LASTEXITCODE -ne 0) { throw "go build cmd/drawgraph failed" }

Write-Host "Building drawgraph-gui.exe (MyGo Native GPU UI)..."
& go build -trimpath -buildvcs=false -tags "mygo_nowebview mygo_noinspector" -ldflags $guiLdflags -o (Join-Path $payload "drawgraph-gui.exe") ./cmd/drawgraph-gui
if ($LASTEXITCODE -ne 0) { throw "go build cmd/drawgraph-gui failed" }

$icon = Join-Path $root "assets\drawgraph.ico"
$wxs = Join-Path $root "installer\DrawGraph.wxs"
$msi = Join-Path $OutDir "DrawGraph-windows-amd64.msi"
Remove-Item -Force $msi -ErrorAction SilentlyContinue

Write-Host "Compiling MSI with WiX..."
& $wix.Source build $wxs `
    -arch x64 `
    -pdbtype none `
    -intermediatefolder $obj `
    -d "Version=$Version" `
    -d "PayloadDir=$payload" `
    -d "IconFile=$icon" `
    -o $msi
if ($LASTEXITCODE -ne 0) { throw "wix build failed" }

$hash = (Get-FileHash -Algorithm SHA256 $msi).Hash.ToLowerInvariant()
"$hash  $(Split-Path -Leaf $msi)" | Set-Content -Encoding ascii (Join-Path $OutDir "DrawGraph-windows-amd64.msi.sha256")

Write-Host ""
Write-Host "MSI:    $msi"
Write-Host "SHA256: $hash"
Write-Host "Size:   $([math]::Round((Get-Item $msi).Length / 1MB, 2)) MB"
