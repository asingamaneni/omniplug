#!/usr/bin/env sh
# Install Hugo (extended) for the host. Prefers Homebrew when available (the only
# practical route on macOS, where Hugo ships a .pkg rather than a tarball); on
# Linux without brew, downloads the pinned extended tarball into GOPATH/bin.
# No-op if Hugo is already on PATH. Override with HUGO_VERSION=...
set -eu

HUGO_VERSION="${HUGO_VERSION:-0.163.3}"

if command -v hugo >/dev/null 2>&1; then
  echo "hugo already installed: $(hugo version | head -n1)"
  exit 0
fi

# Homebrew installs Hugo extended and works on macOS and Linux.
if command -v brew >/dev/null 2>&1; then
  echo "installing Hugo via Homebrew..."
  brew install hugo
  exit 0
fi

os="$(uname -s)"
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) goarch=amd64 ;;
  arm64 | aarch64) goarch=arm64 ;;
  *) echo "unsupported arch: $arch — install Hugo manually: https://gohugo.io/installation/"; exit 1 ;;
esac

case "$os" in
  Linux)
    dest="$(go env GOPATH)/bin"
    mkdir -p "$dest"
    url="https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-${goarch}.tar.gz"
    echo "downloading Hugo ${HUGO_VERSION} (extended) for linux/${goarch}..."
    curl -fsSL "$url" | tar -xz -C "$dest" hugo
    echo "installed hugo to ${dest}/hugo"
    ;;
  Darwin)
    echo "Homebrew not found. Install it from https://brew.sh then run 'brew install hugo',"
    echo "or install the macOS package:"
    echo "  https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_darwin-universal.pkg"
    exit 1
    ;;
  *)
    echo "unsupported OS: $os — install Hugo manually: https://gohugo.io/installation/"
    exit 1
    ;;
esac
