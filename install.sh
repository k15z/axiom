#!/bin/sh
# Axiom installer — detects OS/arch and downloads the right binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/k15z/axiom/main/install.sh | sh

set -e

REPO="k15z/axiom"
INSTALL_DIR="/usr/local/bin"

# Allow overriding install directory
if [ -n "$AXIOM_INSTALL_DIR" ]; then
    INSTALL_DIR="$AXIOM_INSTALL_DIR"
fi

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *)      echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)             echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest version from GitHub API
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
    else
        echo "Error: curl or wget is required" >&2
        exit 1
    fi
}

# Allow pinning a version
VERSION="${AXIOM_VERSION:-$(get_latest_version)}"

if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest version. Set AXIOM_VERSION to install a specific version." >&2
    exit 1
fi

FILENAME="axiom_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

echo "Installing axiom v${VERSION} (${OS}/${ARCH})..."

# Create temp directory
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Download
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$URL" -o "${TMPDIR}/${FILENAME}"
elif command -v wget >/dev/null 2>&1; then
    wget -q "$URL" -O "${TMPDIR}/${FILENAME}"
fi

# Extract
tar -xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "${TMPDIR}/axiom" "${INSTALL_DIR}/axiom"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "${TMPDIR}/axiom" "${INSTALL_DIR}/axiom"
fi

chmod +x "${INSTALL_DIR}/axiom"

echo "axiom v${VERSION} installed to ${INSTALL_DIR}/axiom"
echo ""
echo "Get started:"
echo "  export ANTHROPIC_API_KEY=sk-ant-..."
echo "  axiom add \"your first test\" --run"
