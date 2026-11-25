# DrawGraph Release & End-to-End Packaging Guide

DrawGraph ships **exclusively** for:
- **Windows x64 (`windows/amd64`)** → `DrawGraph-windows-amd64.msi` + `.sha256`
- **macOS Apple Silicon (`darwin/arm64`)** → `DrawGraph-darwin-arm64.dmg` + `.sha256`

Both release binaries (`drawgraph-gui` and `drawgraph` Agent CLI) are 100% pure Go (`CGO_ENABLED=0`) and support manual and automatic self-updates directly from GitHub Releases.

## Prerequisites

- Go **1.27.3+**
- Windows: WiX Toolset **5.x** (`dotnet tool install --global wix --version 5.0.2`)
- macOS: Apple Silicon / macOS runner with `hdiutil`, `ditto`, `codesign`
- `gh` CLI authenticated for `neko233-com/DrawGraph`

## One-Command Full Release (Recommended)

```powershell
powershell -ExecutionPolicy Bypass -File scripts\release.ps1 -Version 0.1.0
```

This script:
1. Runs `gofmt -w .`, `go vet ./...` (`windows/amd64` + `darwin/arm64`), and `go test -count=1 ./...`.
2. Syncs `mygo.json` `"version"` to `0.1.0`.
3. Generates `cmd/drawgraph-gui/rsrc_windows_amd64.syso` and compiles `dist\DrawGraph-windows-amd64.msi` + `.sha256`.
4. Installs the new MSI locally on your Windows machine (`C:\Program Files\DrawGraph`) and verifies `drawgraph.exe` & Start Menu shortcut.
5. Commits, creates annotated tag `v0.1.0`, pushes `main` & `v0.1.0`.
6. Watches `.github/workflows/ci-release.yml` build both `DrawGraph-windows-amd64.msi` and `DrawGraph-darwin-arm64.dmg` (plus `.sha256`) and publish them to GitHub Releases.

## Local Build & Always-Latest Install (Windows)

```powershell
powershell -ExecutionPolicy Bypass -File scripts\install-local.ps1 -Version 0.1.0
```

To validate MSI contents without installing (e.g. if elevation is unavailable):

```powershell
powershell -ExecutionPolicy Bypass -File scripts\install-local.ps1 -ValidateOnly
```

## End-User One-Line Installers

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.ps1 | iex
```

**macOS Apple Silicon / Windows Git Bash:**
```sh
curl -fsSL https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.sh | sh
```

## Manual & Automatic Self-Updates

- **GUI**: Help → **Check for Updates…** (or click the version badge in the bottom-right status bar). Supports:
  - **Auto-check on startup** (`autoCheckUpdates`, default `true`, interval `24h`)
  - **Auto-download verified installer in background** (`autoDownloadUpdates`)
  - **Release channel switching** (`stable` / `prerelease`)
  - SHA-256 verification before running Windows `msiexec /i` or macOS `hdiutil` + `ditto /Applications/DrawGraph.app`
- **CLI**:
  ```sh
  drawgraph update check --json
  drawgraph update download --json
  drawgraph update apply --silent
  drawgraph update config --auto-check true --auto-download true --channel stable
  ```
