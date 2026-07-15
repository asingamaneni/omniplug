---
title: Installation
weight: 10
---

# Installation

omniplug ships as a single static binary — no runtime required. Pick whichever channel fits your setup.

## Homebrew (macOS / Linux)

```bash
brew install asingamaneni/tap/omniplug
```

This taps `asingamaneni/homebrew-tap` and installs the latest release. Upgrade with `brew upgrade omniplug`.

## npm / npx

```bash
npm install -g omniplug
# or, run once without installing:
npx omniplug --help
```

The npm package is a thin wrapper that downloads the matching prebuilt binary from GitHub Releases on install.

## Go install

If you have Go 1.23+:

```bash
go install github.com/asingamaneni/omniplug/cmd/omniplug@latest
```

## Direct download

Grab a prebuilt archive for your OS/arch from the [Releases page](https://github.com/asingamaneni/omniplug/releases), extract it, and put the `omniplug` binary on your `PATH`:

```bash
tar -xzf omniplug_darwin_arm64.tar.gz
sudo mv omniplug /usr/local/bin/
```

## Verify

```bash
omniplug --version
omniplug list-targets
```

## Build from source

```bash
git clone https://github.com/asingamaneni/omniplug
cd omniplug
make build      # -> ./bin/omniplug
make install    # -> $GOPATH/bin/omniplug
```
