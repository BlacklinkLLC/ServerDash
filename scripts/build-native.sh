#!/usr/bin/env bash
# Builds ServerDash for a native (no Docker) install:
#   bin/serverdash   the Go API, which also serves the dashboard
#   web/dist         the built Svelte dashboard
#
# Uses the system's Go and Node when they're new enough; otherwise it
# downloads official toolchains into $TOOLCHAIN_DIR (kept out of the
# system PATH, used only for building) so distro versions never matter.
#
#   scripts/build-native.sh [INSTALL_DIR]

set -euo pipefail

DIR="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
TOOLCHAIN_DIR="${SERVERDASH_TOOLCHAIN_DIR:-/usr/local/lib/serverdash-toolchain}"
NODE_MAJOR="${SERVERDASH_NODE_MAJOR:-22}"

case "$(uname -m)" in
	x86_64|amd64)  GOARCH=amd64 NODEARCH=x64 ;;
	aarch64|arm64) GOARCH=arm64 NODEARCH=arm64 ;;
	armv7l)        GOARCH=armv6l NODEARCH=armv7l ;;
	*) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

need() { command -v "$1" >/dev/null 2>&1 || { echo "$1 is required" >&2; exit 1; }; }
need curl
need tar

# version_ge A B: true if dotted version A >= B
version_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }

ensure_go() {
	export PATH="$TOOLCHAIN_DIR/go/bin:$PATH"
	# go.mod pins a newer toolchain; any Go >= 1.21 fetches it itself.
	export GOTOOLCHAIN=auto
	local v
	v="$(go env GOVERSION 2>/dev/null | sed 's/^go//' || true)"
	if [ -n "$v" ] && version_ge "$v" 1.21; then
		echo "Using Go $v"
		return
	fi
	local latest
	latest="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
	echo "Downloading $latest"
	rm -rf "$TOOLCHAIN_DIR/go"
	mkdir -p "$TOOLCHAIN_DIR"
	curl -fsSL "https://go.dev/dl/${latest}.linux-${GOARCH}.tar.gz" | tar -xz -C "$TOOLCHAIN_DIR"
}

ensure_node() {
	export PATH="$TOOLCHAIN_DIR/node/bin:$PATH"
	local v
	v="$(node --version 2>/dev/null | sed 's/^v//' || true)"
	# Vite needs ^20.19 or >= 22.12.
	if [ -n "$v" ] && command -v npm >/dev/null 2>&1 && { version_ge "$v" 22.12.0 || { version_ge "$v" 20.19.0 && ! version_ge "$v" 21.0.0; }; }; then
		echo "Using Node $v"
		return
	fi
	local file
	file="$(curl -fsSL "https://nodejs.org/dist/latest-v${NODE_MAJOR}.x/SHASUMS256.txt" | awk '{print $2}' | grep -E "linux-${NODEARCH}\.tar\.xz$" | head -n1)"
	[ -n "$file" ] || { echo "couldn't find a Node ${NODE_MAJOR} build for linux-${NODEARCH}" >&2; exit 1; }
	echo "Downloading $file"
	rm -rf "$TOOLCHAIN_DIR/node" "$TOOLCHAIN_DIR/${file%.tar.xz}"
	mkdir -p "$TOOLCHAIN_DIR"
	curl -fsSL "https://nodejs.org/dist/latest-v${NODE_MAJOR}.x/$file" | tar -xJ -C "$TOOLCHAIN_DIR"
	mv "$TOOLCHAIN_DIR/${file%.tar.xz}" "$TOOLCHAIN_DIR/node"
}

ensure_go
ensure_node

echo "Building the dashboard"
cd "$DIR/web"
npm ci --no-audit --no-fund
npm run build

echo "Building the API"
cd "$DIR/server"
mkdir -p "$DIR/bin"
CGO_ENABLED=0 go build -trimpath -o "$DIR/bin/serverdash.new" ./cmd/serverdash
# Atomic swap, so a running service is never left with a half-written binary.
mv "$DIR/bin/serverdash.new" "$DIR/bin/serverdash"

echo "Built $DIR/bin/serverdash"
