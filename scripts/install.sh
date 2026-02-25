#!/bin/sh
set -e

REPO="dwellir-public/cli"
BINARY="dwellir"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
  echo "Failed to determine latest version." && exit 1
fi

URL="https://github.com/$REPO/releases/download/v${LATEST}/${BINARY}_${OS}_${ARCH}.tar.gz"
echo "Downloading dwellir v${LATEST} for ${OS}/${ARCH}..."

TMP=$(mktemp -d)
curl -fsSL "$URL" | tar -xz -C "$TMP"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi
chmod +x "$INSTALL_DIR/$BINARY"
rm -rf "$TMP"

echo "dwellir v${LATEST} installed to $INSTALL_DIR/$BINARY"
dwellir version
