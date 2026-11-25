# DrawGraph

**DrawGraph** is a native GPU-rendered diagramming desktop application and **Agent-first CLI** inspired by [draw.io](https://www.drawio.com/), written in pure Go 1.27 (`CGO_ENABLED=0`) using [MyGo](https://github.com/egoist/mygo) native UI (`ui.Painter`) and [Lobe MyGo](https://github.com/lobehub/lobe-mygo).

- **Only two release targets**: **Windows x64 (`windows/amd64`) MSI** and **macOS Apple Silicon (`darwin/arm64`) DMG**.
- **Virtualized High-Performance Infinite Canvas & HD Media**: 512×512 spatial hash frustum culling (`4µs` query over 10,000+ nodes), multi-tier GPU LOD + adaptive line grid, parallel-edge fanning, smart red alignment snap guides, Command Palette finder (`Ctrl/Cmd+K`), click/drag Minimap, tiled high-DPI PNG export (`8192×8192`), and drag/drop or clipboard paste of **HD images** and **videos** (viewport-virtualized decoder pooling).
- **Multi-Language UI & Habits (`auto`, `en`, `zh-CN`, `zh-TW`, `ja`, `ko`)**: Automatic OS locale detection with live one-click / menu switching, persisted globally alongside user diagramming preferences (`habits.json`).
- **One-Click Formatting (6 Built-in Algorithms)**: `layered` (Sugiyama DAG `LR`/`RL`/`TB`/`BT`), `tree` (org/decision tree), `grid` (media/service matrix), `radial` (topology hub), `swimlane` (tiered lanes), and `compact` (non-destructive overlap cleanup) with 6 curated presets (`lobe-clean`, `drawio-classic`, `blueprint`, `slate-mono`, `vibrant-tech`, `warm-paper`).
- **Native MCP Server + Agent CLI + Live IPC**: Run `drawgraph mcp` for Model Context Protocol JSON-RPC 2.0 (including Mermaid import, DSL/batch mutation, inline base64 PNG vision responses, icon search/install, and live GUI camera/format control via `--live`).
- **Zero WebView / Zero Electron / Zero CGO**: Instant launch, GPU-accelerated canvas rendering (Direct3D 11 on Windows, Metal on macOS, plus hardware-accelerated `ui/video` playback).
- **Bidirectional draw.io & Mermaid Support**: Imports and exports `.drawio` XML (`mxGraphModel`), imports Mermaid flowcharts/subgraphs, and persists native `.drawgraph` JSON.
- **Git-Extensible SVG Icon Plugins**: Extend DrawGraph's icon library from any Git repository URL or local SVG directory (`drawgraph icon install <git-url>`).

## Installation & Updates

### One-Line Install

- **Windows x64 (`windows/amd64` MSI)**:
  ```powershell
  irm https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.ps1 | iex
  ```
- **macOS Apple Silicon (`darwin/arm64` DMG) & Windows Git Bash**:
  ```sh
  curl -fsSL https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.sh | sh
  ```

### Manual & Automatic Updates

- **GUI**: Open **Help → Check for Updates…** (or click the version badge on the status bar) to check, download, SHA-256 verify, and install updates, or enable automatic startup checks & background downloads.
- **CLI**:
  ```sh
  drawgraph update check --json
  drawgraph update apply --silent
  drawgraph update config --auto-check true --auto-download true --channel stable
  ```

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

# Import Mermaid directly into a formatted DrawGraph diagram
go run ./cmd/drawgraph mermaid arch.drawgraph --strategy tree --stdin < flow.mmd

# Start native MCP JSON-RPC 2.0 server (or print mcpServers config for Cursor/Claude/Antigravity)
drawgraph mcp config
drawgraph mcp
```

### Multi-Language UI & Persistent Cross-Project Habits

```sh
# Switch UI language (`auto`, `en`, `zh-CN`, `zh-TW`, `ja`, `ko`) and default layout engine
drawgraph habits set --language zh-CN --strategy layered --wheel-mode zoom --preset blueprint
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

## Packaging & Releases

See [RELEASE.md](RELEASE.md) and [AGENTS.md](AGENTS.md) for the full build, local always-latest installation, and GitHub Release pipeline:

- **Windows x64 MSI (Build + Install Locally)**:
  ```powershell
  powershell -ExecutionPolicy Bypass -File scripts\install-local.ps1 -Version 0.1.0
  ```
- **macOS arm64 DMG** (run on macOS Apple Silicon):
  ```sh
  ./scripts/build-dmg.sh 0.1.0
  ```
- **Full Release (`vX.Y.Z` tag + CI双端发布)**:
  ```powershell
  powershell -ExecutionPolicy Bypass -File scripts\release.ps1 -Version 0.1.0
  ```

## License

MIT
