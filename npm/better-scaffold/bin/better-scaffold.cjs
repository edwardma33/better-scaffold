#!/usr/bin/env node
"use strict";

const path = require("node:path");
const { spawnSync } = require("node:child_process");

const platforms = new Set([
  "darwin-arm64", "darwin-x64", "linux-arm64", "linux-x64", "win32-arm64", "win32-x64",
]);

function binaryPath(platform, arch, root = path.resolve(__dirname, "..")) {
  const target = `${platform}-${arch}`;
  if (!platforms.has(target)) {
    throw new Error(`Better Scaffold does not support ${target}. Supported platforms are macOS, Linux, and Windows on x64 and ARM64.`);
  }
  return path.join(root, "native", target, platform === "win32" ? "better-scaffold.exe" : "better-scaffold");
}

function launch(args, { platform = process.platform, arch = process.arch, root, spawn = spawnSync } = {}) {
  const executable = binaryPath(platform, arch, root);
  const result = spawn(executable, args, { stdio: "inherit" });
  if (result.error) {
    if (result.error.code === "ENOENT") {
      throw new Error(`The Better Scaffold binary for ${platform}-${arch} is missing. Reinstall the package for your platform. Host-only development tarballs work only on the machine type they were built for.`);
    }
    throw new Error(`Unable to start Better Scaffold: ${result.error.message}`);
  }
  return result;
}

if (require.main === module) {
  try {
    const result = launch(process.argv.slice(2));
    if (result.signal) {
      process.kill(process.pid, result.signal);
    } else {
      process.exitCode = result.status ?? 1;
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

module.exports = { binaryPath, launch };
