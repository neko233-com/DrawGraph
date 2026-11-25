# AGENTS.md

Engineering rules and workflow for **DrawGraph**.

## Stack & Constraints

- **Go 1.27.3+**, **100% pure Go (`CGO_ENABLED=0`)**. Never introduce `import "C"` or C/C++ build dependencies.
- **Native GPU UI**: Built with `github.com/egoist/mygo` (`ui.Painter`, `ui.View`, `ui.Tester`) and `github.com/lobehub/lobe-mygo`. No WebView, no HTML/CSS/JS UI.
- **Release Targets Only**:
  - `windows/amd64` -> `dist/DrawGraph-windows-amd64.msi` (WiX v5 via `scripts/build-msi.ps1`)
  - `darwin/arm64` -> `dist/DrawGraph-darwin-arm64.dmg` (`mygo build` + `hdiutil` via `scripts/build-dmg.sh`)
- **Cross-Project Persistence**:
  - Global user habits live in `<ConfigDir>/DrawGraph/habits.json` (`DRAWGRAPH_HOME` overrides base dir for tests).
  - Installed Git/local icon packs live in `<ConfigDir>/DrawGraph/icons/registry.json` and `<ConfigDir>/DrawGraph/icons/packs/<id>/`.

## Verification Checklist

```powershell
$env:CGO_ENABLED = "0"
gofmt -w .
go vet ./...
go test -count=1 ./...
powershell -ExecutionPolicy Bypass -File scripts\build-msi.ps1 -Version 0.1.0
```
