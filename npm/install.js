#!/usr/bin/env node
// Postinstall: download the omniplug binary matching this platform from the
// GitHub release whose tag is `v<package.version>`, and unpack it into ./bin.
"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");
const { execFileSync } = require("child_process");

const REPO = "asingamaneni/omniplug";
const pkg = require("./package.json");
const version = pkg.version;

// Map Node's platform/arch to GoReleaser's os/arch tokens.
const OS = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCH = { x64: "amd64", arm64: "arm64" };

function fail(msg) {
  console.error(`omniplug: ${msg}`);
  process.exit(1);
}

const goos = OS[process.platform];
const goarch = ARCH[process.arch];
if (!goos || !goarch) {
  fail(`unsupported platform ${process.platform}/${process.arch}. ` +
    `Install from source: go install github.com/${REPO}/cmd/omniplug@latest`);
}

const ext = goos === "windows" ? "zip" : "tar.gz";
const asset = `omniplug_${goos}_${goarch}.${ext}`;
const url = `https://github.com/${REPO}/releases/download/v${version}/${asset}`;

const binDir = path.join(__dirname, "bin");
fs.mkdirSync(binDir, { recursive: true });
const archivePath = path.join(binDir, asset);

function download(u, dest, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > 10) return reject(new Error("too many redirects"));
    https.get(u, { headers: { "User-Agent": "omniplug-installer" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        return resolve(download(res.headers.location, dest, redirects + 1));
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`HTTP ${res.statusCode} for ${u}`));
      }
      const file = fs.createWriteStream(dest);
      res.pipe(file);
      file.on("finish", () => file.close(resolve));
      file.on("error", reject);
    }).on("error", reject);
  });
}

function unpack(archive, dir) {
  if (ext === "zip") {
    // Windows: use PowerShell's Expand-Archive.
    execFileSync("powershell", ["-NoProfile", "-Command",
      `Expand-Archive -Path "${archive}" -DestinationPath "${dir}" -Force`],
      { stdio: "inherit" });
  } else {
    execFileSync("tar", ["-xzf", archive, "-C", dir], { stdio: "inherit" });
  }
}

(async () => {
  try {
    await download(url, archivePath);
    unpack(archivePath, binDir);
    fs.rmSync(archivePath, { force: true });

    const exe = goos === "windows" ? "omniplug.exe" : "omniplug";
    const src = path.join(binDir, exe);
    const target = path.join(binDir, goos === "windows" ? "omniplug.exe" : "omniplug-bin");
    if (!fs.existsSync(src)) fail(`binary ${exe} not found in archive`);
    if (src !== target) fs.renameSync(src, target);
    if (goos !== "windows") fs.chmodSync(target, 0o755);
    console.log(`omniplug ${version} installed for ${goos}/${goarch}`);
  } catch (err) {
    fail(`failed to download ${url}: ${err.message}\n` +
      `Install from source instead: go install github.com/${REPO}/cmd/omniplug@latest`);
  }
})();
