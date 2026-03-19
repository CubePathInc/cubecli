#!/bin/sh
set -e

# CubeCLI installer
# Usage: curl -fsSL https://raw.githubusercontent.com/CubePathInc/cubecli/main/install.sh | sh

REPO="CubePathInc/cubecli"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest version
echo "Fetching latest version..."
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "Error: Could not determine latest version"
    exit 1
fi

echo "Installing CubeCLI v${VERSION} (${OS}/${ARCH})..."

# Download
EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"

FILENAME="cubecli_${VERSION}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$URL" -o "${TMPDIR}/${FILENAME}"

# Extract
cd "$TMPDIR"
if [ "$EXT" = "tar.gz" ]; then
    tar xzf "$FILENAME"
else
    unzip -q "$FILENAME"
fi

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv cubecli "$INSTALL_DIR/"
else
    sudo mv cubecli "$INSTALL_DIR/"
fi

echo "CubeCLI v${VERSION} installed to ${INSTALL_DIR}/cubecli"
cubecli version
