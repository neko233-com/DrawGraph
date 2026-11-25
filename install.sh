#!/bin/sh
# DrawGraph one-click installer for macOS (arm64 DMG) and Windows shells (Git Bash / MSYS / WSL MSI).
#
#   curl -fsSL https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.sh | sh
#   DRAWGRAPH_VERSION=0.1.0 curl -fsSL https://raw.githubusercontent.com/neko233-com/DrawGraph/main/install.sh | sh
set -eu

repo="neko233-com/DrawGraph"
version="${DRAWGRAPH_VERSION:-latest}"

if [ "$version" = "latest" ]; then
    base_url="https://github.com/$repo/releases/latest/download"
else
    version="${version#v}"
    base_url="https://github.com/$repo/releases/download/v$version"
fi

download() {
    url="$1"
    out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fL --retry 3 --connect-timeout 20 -o "$out" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "$out" "$url"
    else
        echo "error: curl or wget is required" >&2
        exit 1
    fi
}

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
    Darwin)
        if [ "$arch" != "arm64" ]; then
            echo "DrawGraph macOS releases target Apple Silicon (arm64); detected $arch." >&2
            exit 1
        fi
        dmg_name="DrawGraph-darwin-arm64.dmg"
        tmp_dir="$(mktemp -d)"
        trap 'rm -rf "$tmp_dir"' EXIT INT TERM
        dmg="$tmp_dir/$dmg_name"
        sha="$tmp_dir/$dmg_name.sha256"
        echo "Downloading $base_url/$dmg_name"
        download "$base_url/$dmg_name" "$dmg"
        download "$base_url/$dmg_name.sha256" "$sha"
        want="$(awk '{print $1}' "$sha" | tr '[:upper:]' '[:lower:]')"
        got="$(shasum -a 256 "$dmg" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')"
        if [ "$got" != "$want" ]; then
            echo "error: SHA-256 mismatch (got $got, want $want)" >&2
            exit 1
        fi
        mnt="$tmp_dir/mnt"
        mkdir -p "$mnt"
        hdiutil attach -nobrowse -readonly -mountpoint "$mnt" "$dmg" >/dev/null
        trap 'hdiutil detach -force "$mnt" >/dev/null 2>&1 || true; rm -rf "$tmp_dir"' EXIT INT TERM
        rm -rf /Applications/DrawGraph.app
        ditto "$mnt/DrawGraph.app" /Applications/DrawGraph.app
        hdiutil detach -force "$mnt" >/dev/null 2>&1 || true

        cli_src="/Applications/DrawGraph.app/Contents/Helpers/drawgraph"
        cli_dir="$HOME/.local/bin"
        if [ -w /usr/local/bin ]; then
            cli_dir="/usr/local/bin"
        fi
        mkdir -p "$cli_dir"
        ln -sf "$cli_src" "$cli_dir/drawgraph"
        echo ""
        echo "DrawGraph is installed."
        echo "  App: /Applications/DrawGraph.app"
        echo "  CLI: $cli_dir/drawgraph"
        ;;
    MINGW*|MSYS*|CYGWIN*|Linux)
        if [ "$os" = "Linux" ] && ! grep -qi microsoft /proc/version 2>/dev/null; then
            echo "DrawGraph supports Windows (amd64 MSI) and macOS (arm64 DMG)." >&2
            exit 1
        fi
        msi_name="DrawGraph-windows-amd64.msi"
        tmp_dir="$(mktemp -d)"
        trap 'rm -rf "$tmp_dir"' EXIT INT TERM
        msi="$tmp_dir/$msi_name"
        sha="$tmp_dir/$msi_name.sha256"
        echo "Downloading $base_url/$msi_name"
        download "$base_url/$msi_name" "$msi"
        download "$base_url/$msi_name.sha256" "$sha"
        want="$(awk '{print $1}' "$sha" | tr '[:upper:]' '[:lower:]')"
        if command -v sha256sum >/dev/null 2>&1; then
            got="$(sha256sum "$msi" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')"
        else
            got="$(shasum -a 256 "$msi" | awk '{print $1}' | tr '[:upper:]' '[:lower:]')"
        fi
        if [ "$got" != "$want" ]; then
            echo "error: SHA-256 mismatch (got $got, want $want)" >&2
            exit 1
        fi
        if command -v cygpath >/dev/null 2>&1; then
            winpath="$(cygpath -w "$msi")"
        elif command -v wslpath >/dev/null 2>&1; then
            winpath="$(wslpath -w "$msi")"
        else
            winpath="$msi"
        fi
        powershell.exe -NoProfile -Command \
            "\$p = Start-Process msiexec.exe -ArgumentList @('/i','$winpath','/passive','/norestart') -Verb RunAs -Wait -PassThru; exit \$p.ExitCode"
        echo ""
        echo "DrawGraph is installed."
        echo "  App: C:\\Program Files\\DrawGraph\\drawgraph-gui.exe (Start menu: DrawGraph)"
        echo "  CLI: C:\\Program Files\\DrawGraph\\drawgraph.exe"
        ;;
    *)
        echo "Unsupported OS: $os" >&2
        exit 1
        ;;
esac
