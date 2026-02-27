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

if [ -n "${INSTALL_DIR:-}" ]; then
  TARGET_DIR="$INSTALL_DIR"
elif [ -w "/usr/local/bin" ]; then
  TARGET_DIR="/usr/local/bin"
else
  TARGET_DIR="$HOME/.local/bin"
fi

mkdir -p "$TARGET_DIR"
if [ "$TARGET_DIR" = "/usr/local/bin" ] && [ ! -w "$TARGET_DIR" ]; then
  echo "Installing to $TARGET_DIR (requires sudo)..."
  sudo mv "$TMP/$BINARY" "$TARGET_DIR/$BINARY"
else
  mv "$TMP/$BINARY" "$TARGET_DIR/$BINARY"
fi
chmod +x "$TARGET_DIR/$BINARY"
rm -rf "$TMP"

echo "dwellir v${LATEST} installed to $TARGET_DIR/$BINARY"
if [ "$TARGET_DIR" = "$HOME/.local/bin" ]; then
  echo "Ensure $HOME/.local/bin is in your PATH."
fi
dwellir version
