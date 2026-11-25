#!/usr/bin/env bash
set -euo pipefail

# Builds macOS arm64 (Apple Silicon) app bundle + CLI + compressed DMG into dist/DrawGraph-darwin-arm64.dmg.
# Must be run on macOS (uses hdiutil / mygo build).

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  VERSION="$(git describe --tags --abbrev=0 2>/dev/null || echo "0.1.0")"
fi
VERSION="${VERSION#v}"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
mkdir -p "$OUT_DIR"

COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "dev")"
DATE="$(date -u +%Y-%m-%d)"
LDFLAGS="-s -w -buildid= -X github.com/neko233-com/DrawGraph/internal/version.Version=$VERSION -X github.com/neko233-com/DrawGraph/internal/version.Commit=$COMMIT -X github.com/neko233-com/DrawGraph/internal/version.Date=$DATE"

export CGO_ENABLED=0

echo "Building DrawGraph.app with MyGo (darwin/arm64)..."
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

go tool mygo build -platform darwin/arm64 -skip-notarize -skip-dmg -o "$STAGE" .

APP_BUNDLE="$STAGE/darwin-arm64/DrawGraph.app"
if [[ ! -d "$APP_BUNDLE" ]]; then
  APP_BUNDLE="$(find "$STAGE" -maxdepth 3 -name 'DrawGraph.app' | head -n 1)"
fi
if [[ -z "$APP_BUNDLE" || ! -d "$APP_BUNDLE" ]]; then
  echo "error: DrawGraph.app not found under $STAGE" >&2
  exit 1
fi

echo "Embedding drawgraph Agent CLI (darwin/arm64) inside app bundle..."
mkdir -p "$APP_BUNDLE/Contents/Resources/bin"
GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" \
  -o "$APP_BUNDLE/Contents/Resources/bin/drawgraph" ./cmd/drawgraph

# Re-sign bundle ad-hoc after adding drawgraph CLI binary so Gatekeeper local checks remain consistent.
if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "$APP_BUNDLE" || true
fi

DMG_SRC="$STAGE/dmg-src"
mkdir -p "$DMG_SRC"
ditto "$APP_BUNDLE" "$DMG_SRC/DrawGraph.app"
ln -s /Applications "$DMG_SRC/Applications"

DMG_PATH="$OUT_DIR/DrawGraph-darwin-arm64.dmg"
rm -f "$DMG_PATH"
hdiutil create -volname "DrawGraph" -srcfolder "$DMG_SRC" -ov -format UDZO "$DMG_PATH"

shasum -a 256 "$DMG_PATH" | awk -v name="$(basename "$DMG_PATH")" '{print $1 "  " name}' > "${DMG_PATH}.sha256"
echo "DMG:    $DMG_PATH"
cat "${DMG_PATH}.sha256"
