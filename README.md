# DrawGraph

**DrawGraph** is a native GPU-rendered diagramming desktop application and **Agent-first CLI** inspired by [draw.io](https://www.drawio.com/), written in pure Go 1.27 (`CGO_ENABLED=0`) using [MyGo](https://github.com/egoist/mygo) native UI (`ui.Painter`) and [Lobe MyGo](https://github.com/lobehub/lobe-mygo).

- **Only two release targets**: **Windows x64 (`windows/amd64`) MSI** and **macOS Apple Silicon (`darwin/arm64`) DMG**.
- **Infinite Canvas & High-Resolution Media**: Unbounded pan/zoom (`5%`–`800%`), interactive click/drag Minimap, tiled high-DPI PNG export (up to `8192×8192`), and seamless drag-and-drop or clipboard paste of **HD images** (`PNG`/`JPG`/`WebP`/`GIF`/`BMP`/`SVG`) and **videos** (`MP4`/`MOV`/`WebM`/`MKV`/`AVI`) directly as canvas nodes — preserving full native source resolution while defaulting to an aspect-fitted canvas footprint (with one-click `1:1 Native` toggle).
- **Zero WebView / Zero Electron / Zero CGO**: Instant launch, GPU-accelerated canvas rendering (Direct3D 11 on Windows, Metal on macOS, plus hardware-accelerated `ui/video` playback).
- **Bidirectional draw.io Compatibility**: Imports and exports `.drawio` (`mxGraphModel` / compressed & uncompressed XML) alongside native `.drawgraph` JSON.
- **Native Agent CLI + Live IPC**: AI agents can draw entire architecture/flow diagrams via concise DSL or transactional JSON batching, export PNG/SVG/draw.io headlessly, or push updates live (`--live`) into an open GUI window.
- **Git-Extensible SVG Icon Plugins**: Extend DrawGraph's icon library from any Git repository URL or local SVG directory (`drawgraph icon install <git-url>`), with optional `drawgraph-icons.json` manifest or automatic SVG discovery.
- **One-Click Elegant Formatting & Persistent Habits**: Layered Sugiyama-style DAG layout (`LR`/`TB`), crossing minimization, smart orthogonal/curved edge routing, container wrapping, palette harmonization, and cross-project user habit memory (`habits.json`) so preferences never reset between projects.

## Quick Start

### Run Desktop GUI

```sh
CGO_ENABLED=0 go run ./cmd/drawgraph-gui
```

### Run Agent CLI

```sh
# Create an architecture diagram from DSL + One-Click Format in one step
go run ./cmd/drawgraph dsl arch.drawgraph --format --json <<'EOF'
container vpc "Production VPC"
node client "Web Client" icon:builtin/browser
node gw "API Gateway" icon:builtin/gateway parent:vpc
node svc "Order Service" icon:builtin/server parent:vpc
node db "Orders DB" kind:cylinder icon:builtin/database parent:vpc
client -> gw : "HTTPS"
gw -> svc : "gRPC"
svc -> db : "SQL"
EOF

# Export crisp SVG, PNG (rendered via MyGo GPU renderer), or draw.io XML
go run ./cmd/drawgraph export arch.drawgraph --to arch.svg
go run ./cmd/drawgraph export arch.drawgraph --to arch.png
go run ./cmd/drawgraph export arch.drawgraph --to arch.drawio

# Import high-resolution images or videos directly as aspect-fitted nodes
go run ./cmd/drawgraph add-image arch.drawgraph --src design-4k.png
go run ./cmd/drawgraph add-video arch.drawgraph --src demo.mp4
```

### Extend Icons from Git

```sh
# Install any Git repo containing SVG icons (auto-discovers SVGs or reads drawgraph-icons.json)
drawgraph icon install https://github.com/lucide-icons/lucide.git --subdir icons --id lucide

# Search across built-in + installed Git packs
drawgraph icon search server --json

# Pull latest icons from remote Git repositories
drawgraph icon sync --all
```

### Persistent Cross-Project Habits

```sh
# Set global preferences remembered across all future projects
drawgraph habits set --direction TB --preset blueprint --routing orthogonal --auto-remember true

# View learned & saved habits
drawgraph habits show --json
```

## Automated Tests

```sh
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go test -count=1 ./...
```

The suite covers geometry & orthogonal routing (`internal/geo`), graph model & undo history (`internal/graph`), `.drawio` XML import/export including compressed diagrams (`internal/drawio`), Git icon plugin install/sync (`internal/icons`), persistent cross-project habit learning (`internal/habits`), layered layout & one-click formatting (`internal/layout`), headless GPU PNG & SVG export (`internal/export`), Agent CLI + live IPC (`internal/agentcli`, `internal/ipc`), and headless GPU UI interaction testing via `ui.NewTester` (`internal/app`).

## Packaging

- **Windows x64 MSI** (requires Go 1.27+ and WiX 5 `wix.exe`):
  ```powershell
  powershell -ExecutionPolicy Bypass -File scripts\build-msi.ps1 -Version 0.1.0
  ```
- **macOS arm64 DMG** (run on macOS Apple Silicon):
  ```sh
  ./scripts/build-dmg.sh 0.1.0
  ```

## License

MIT
