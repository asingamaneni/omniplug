#!/usr/bin/env node
// Thin launcher: exec the platform binary fetched by install.js, forwarding args.
"use strict";

const path = require("path");
const fs = require("fs");
const { spawnSync } = require("child_process");

const exe = process.platform === "win32" ? "omniplug.exe" : "omniplug-bin";
const bin = path.join(__dirname, exe);

if (!fs.existsSync(bin)) {
  console.error(
    "omniplug: binary not found. Reinstall the package, or build from source:\n" +
    "  go install github.com/asingamaneni/omniplug/cmd/omniplug@latest"
  );
  process.exit(1);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`omniplug: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
