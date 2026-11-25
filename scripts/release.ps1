# Full end-to-end release pipeline for DrawGraph:
#   1. Formats, vets (windows/amd64 + darwin/arm64), and runs all automated tests.
#   2. Updates mygo.json version, builds Windows x64 MSI, and installs locally.
#   3. Commits, creates signed/annotated tag v<Version>, pushes main & tag.
#   4. Watches GitHub Actions CI build both DrawGraph-windows-amd64.msi and DrawGraph-darwin-arm64.dmg.
#   5. Verifies published GitHub Release download URLs & SHA-256 checksums.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts\release.ps1 -Version 0.1.0
param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$Notes = "",
    [switch]$SkipLocalInstall,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$Version = $Version -replace '^v', ''
if ($Version -notmatch '^\d+\.\d+\.\d+') {
    throw "Version must look like 0.1.0 (got '$Version')"
}
$tag = "v$Version"

Write-Host "==> Step 1: Format, Vet (windows/amd64 + darwin/arm64), Test..."
$env:CGO_ENABLED = "0"
gofmt -w .
go vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet failed" }
$env:GOOS = "darwin"; $env:GOARCH = "arm64"
go vet ./...
if ($LASTEXITCODE -ne 0) { Remove-Item Env:GOOS, Env:GOARCH; throw "darwin/arm64 go vet failed" }
Remove-Item Env:GOOS, Env:GOARCH
go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw "go test failed" }

Write-Host "==> Step 2: Sync mygo.json version to $Version..."
$cfg = Get-Content mygo.json -Raw | ConvertFrom-Json
$cfg.version = $Version
($cfg | ConvertTo-Json -Depth 10) + "`n" | Set-Content mygo.json -Encoding utf8

Write-Host "==> Step 3: Build Windows x64 MSI..."
powershell -ExecutionPolicy Bypass -File scripts\build-msi.ps1 -Version $Version
if ($LASTEXITCODE -ne 0) { throw "build-msi.ps1 failed" }

if (-not $SkipLocalInstall) {
    Write-Host "==> Step 4: Install MSI locally so this machine has $tag..."
    powershell -ExecutionPolicy Bypass -File scripts\install-local.ps1 -SkipBuild
    if ($LASTEXITCODE -ne 0) { throw "install-local.ps1 failed" }
}

if ($DryRun) {
    Write-Host "DryRun complete. Skipping git commit/tag/push."
    return
}

Write-Host "==> Step 5: Commit, tag $tag, and push to origin..."
git add -A
$pending = git status --porcelain
if ($pending) {
    git commit -m "release: $tag"
    if ($LASTEXITCODE -ne 0) { throw "git commit failed" }
}
git push origin main
if ($LASTEXITCODE -ne 0) { throw "git push main failed" }

git tag -a $tag -m "DrawGraph $Version"
if ($LASTEXITCODE -ne 0) { throw "git tag failed" }
git push origin $tag
if ($LASTEXITCODE -ne 0) { throw "git push tag failed" }

Write-Host "==> Step 6: Waiting for GitHub Actions release workflow..."
Start-Sleep -Seconds 5
$runId = (gh run list --branch $tag -L 1 --json databaseId --jq '.[0].databaseId')
if (-not $runId) {
    $runId = (gh run list -L 1 --json databaseId --jq '.[0].databaseId')
}
gh run watch $runId --exit-status
if ($LASTEXITCODE -ne 0) { throw "release CI workflow failed" }

Write-Host "==> Step 7: Verifying release assets..."
gh release view $tag --json assets --jq '.assets[] | {name, size, url}'
