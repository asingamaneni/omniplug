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
    const req = https.get(u, { headers: { "User-Agent": "omniplug-installer" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        return resolve(download(res.headers.location, dest, redirects + 1));
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`HTTP ${res.statusCode} for ${u}`));
      }
      const file = fs.createWriteStream(dest);
      // pipe() does NOT forward source errors, so a truncated/aborted body
      // would otherwise settle neither resolve nor reject (silent hang / a
      // partial archive). Watch the response too and fail loudly, cleaning up.
      const fail = (err) => {
        file.destroy();
        fs.rmSync(dest, { force: true });
        reject(err);
      };
      res.on("error", fail);
      res.on("aborted", () => fail(new Error("connection closed before the download completed")));
      file.on("error", fail);
      file.on("finish", () => file.close(resolve));
      res.pipe(file);
    });
    req.on("error", reject);
    // Guard against a stalled connection that never sends data or a FIN.
    req.setTimeout(60000, () => req.destroy(new Error("download timed out after 60s")));
  });
}

function unpack(archive, dir) {
  if (ext === "zip") {
    // Windows: PowerShell's Expand-Archive. Pass paths via env vars rather than
    // interpolating into the -Command string, so a directory name containing
    // $var, $(...), or backticks cannot be expanded or executed by PowerShell.
    execFileSync("powershell", ["-NoProfile", "-Command",
      "Expand-Archive -Path $env:OMNIPLUG_ARCHIVE -DestinationPath $env:OMNIPLUG_DEST -Force"],
      { stdio: "inherit", env: { ...process.env, OMNIPLUG_ARCHIVE: archive, OMNIPLUG_DEST: dir } });
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
