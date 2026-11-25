# AGENTS.md

Engineering rules and workflow for **DrawGraph**.

## Stack & Constraints

- **Go 1.27.3+**, **100% pure Go (`CGO_ENABLED=0`)**. Never introduce `import "C"` or C/C++ build dependencies.
- **Native GPU UI**: Built with `github.com/egoist/mygo` (`ui.Painter`, `ui.View`, `ui.Tester`, `ui/video`) and `github.com/lobehub/lobe-mygo`. No WebView, no HTML/CSS/JS UI.
- **Release Targets Only**:
  - `windows/amd64` -> `dist/DrawGraph-windows-amd64.msi` (WiX v5 via `scripts/build-msi.ps1`)
  - `darwin/arm64` -> `dist/DrawGraph-darwin-arm64.dmg` (`mygo build` + `hdiutil` via `scripts/build-dmg.sh`)
- **Cross-Project Persistence**:
  - Global user habits live in `<ConfigDir>/DrawGraph/habits.json` (`DRAWGRAPH_HOME` overrides base dir for tests).
  - Installed Git/local icon packs live in `<ConfigDir>/DrawGraph/icons/registry.json` and `<ConfigDir>/DrawGraph/icons/packs/<id>/`.

## Extensibility & Upgrade Architecture (Reserved Extension Points)

All core subsystems are designed with additive forward-compatible contracts—preserve these extension points on every change:

- **Schema Migrations**: `internal/graph/migrate.go` (`RegisterMigration`, `MigrateUp`). Keep JSON schemas additive and preserve `Meta` maps and unknown `FormatVersion > FormatVersion` files cleanly.
- **Shape Plugins**: `internal/render/render.go` (`RegisterShape`, `RegisteredShapes`). New diagram node types can plug in GPU painters without modifying core render dispatch.
- **Layout Strategies**: `internal/layout/layout.go` (`RegisterStrategy`, `RegisteredStrategies`). Built-in strategies (`"layered"`, `"tree"`, `"grid"`, `"radial"`, `"swimlane"`, `"compact"`) plus custom plugins register cleanly.
- **Multi-Language i18n**: `internal/i18n/i18n.go` (`RegisterLocale`, `SupportedLocales`). Ships built-in `auto` OS detection, `en`, `zh-CN`, `zh-TW`, `ja`, and `ko` dictionaries persisted via `habits.Profile.Language`.
- **Spatial Index & Virtual Canvas**: `internal/graph/spatial.go` & `internal/render/render.go` (`SpatialIndex`, `PaintSceneWithStats`, GPU LOD, smart snap guides). Keep `Page.MarkDirty()` updated after bulk geometry mutations.
- **Native MCP & Agent Ops**: `internal/ops` + `internal/mcp` (`drawgraph mcp` stdio JSON-RPC 2.0 & HTTP) + `internal/mermaid` share one unified execution path with `internal/agentcli`.
- **Export Drivers**: `internal/export/export.go` (`RegisterDriver`, `SupportedExtensions`). New output formats register cleanly for both CLI and GUI export dialogs.
- **Git Icon Plugins**: `internal/icons/registry.go` (`PackManifest` with `SchemaVersion` & `MinAppVersion`) supports Git URLs, local directories, and single SVGs.
- **Self-Update Engine**: `internal/update` supports `stable` & `prerelease` GitHub Release channels, SHA-256 verification, manual CLI (`drawgraph update check|download|apply|config`) and GUI manual/auto updates (Windows MSI + macOS DMG).

## On Windows: Build and Always Install the Latest MSI Locally

When running on Windows, after **any** change to shipped code, run the full verification, MSI packaging, and local installation cycle below so the user's machine **always runs the latest build**:

```powershell
$env:CGO_ENABLED = "0"
gofmt -w .
go vet ./...
$env:GOOS = "darwin"; $env:GOARCH = "arm64"; go vet ./...; Remove-Item Env:GOOS, Env:GOARCH
go test -count=1 ./...
powershell -ExecutionPolicy Bypass -File scripts\install-local.ps1
```

If elevation (UAC) is unavailable in an unattended session, validate the MSI payload cleanly without claiming installation succeeded, and report clearly to the user:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\install-local.ps1 -ValidateOnly
```

After a real installation, verify:

```powershell
& "C:\Program Files\DrawGraph\drawgraph.exe" version
Test-Path "C:\Program Files\DrawGraph\drawgraph-gui.exe"
Get-StartApps | Where-Object { $_.Name -eq "DrawGraph" }
```

The Start Menu shortcut must remain searchable (not marked `NotContentIndexed`; `ClearShortcutNotContentIndexed` in `installer/DrawGraph.wxs` clears it during install).

## Full Release Pipeline

See `RELEASE.md` for details. To publish a new release (`vX.Y.Z`) with both `DrawGraph-windows-amd64.msi` and `DrawGraph-darwin-arm64.dmg` plus SHA-256 checksums:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\release.ps1 -Version X.Y.Z
```
