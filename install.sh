#!/bin/sh
set -e

REPO="aysdog/commitdog"
BINARY="commitdog"
INSTALL_DIR="/usr/local/bin"

# colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()    { printf "  ${GREEN}→${NC} %s\n" "$1"; }
warn()    { printf "  ${YELLOW}!${NC} %s\n" "$1"; }
error()   { printf "  ${RED}✗${NC} %s\n" "$1"; exit 1; }
success() { printf "  ${GREEN}✓${NC} %s\n" "$1"; }

echo ""
echo "  commitdog installer"
echo "  ─────────────────────────────"
echo ""

# detect OS
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)
    case "$ARCH" in
      x86_64)  ASSET="commitdog-linux-amd64" ;;
      aarch64) ASSET="commitdog-linux-arm64" ;;
      arm64)   ASSET="commitdog-linux-arm64" ;;
      *)       error "unsupported architecture: $ARCH" ;;
    esac
    ;;
  Darwin)
    case "$ARCH" in
      x86_64) ASSET="commitdog-darwin-amd64" ;;
      arm64)  ASSET="commitdog-darwin-arm64" ;;
      *)      error "unsupported architecture: $ARCH" ;;
    esac
    ;;
  *)
    error "unsupported OS: $OS. download manually from github.com/$REPO/releases"
    ;;
esac

info "detected $OS/$ARCH → $ASSET"

# get latest release URL
info "fetching latest release..."
DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/$ASSET"

# download
TMP="$(mktemp)"
info "downloading $ASSET..."

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP" || error "download failed. check your connection."
elif command -v wget >/dev/null 2>&1; then
  wget -q "$DOWNLOAD_URL" -O "$TMP" || error "download failed. check your connection."
else
  error "curl or wget required. install one and try again."
fi

# make executable
chmod +x "$TMP"

# install
info "installing to $INSTALL_DIR/$BINARY..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$TMP" "$INSTALL_DIR/$BINARY" || error "install failed. try running with sudo."
fi

echo ""
success "commitdog installed successfully!"
echo ""
echo "  get started:"
echo "    commitdog setup   ← do this once"
echo "    commitdog init    ← create a new repo"
echo "    commitdog         ← commit with a smart message"
echo ""
echo "  docs: github.com/$REPO"
echo ""