import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmodSync, copyFileSync, mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = path.join(root, "npm/better-scaffold/bin/better-scaffold.cjs");
const require = createRequire(import.meta.url);
const { binaryPath, launch } = require(source);

test("launcher chooses all six platform binaries", () => {
  for (const platform of ["darwin", "linux", "win32"]) {
    for (const arch of ["x64", "arm64"]) {
      assert.equal(binaryPath(platform, arch, "/package"), path.join("/package", "native", `${platform}-${arch}`, platform === "win32" ? "better-scaffold.exe" : "better-scaffold"));
    }
  }
  assert.throws(() => binaryPath("freebsd", "x64"), /does not support freebsd-x64/);
});

test("launcher forwards arguments and reports native failure", () => {
  const args = ["init", "a directory with spaces", "--profile", "a profile.json"];
  const result = launch(args, { platform: "linux", arch: "arm64", root: "/package", spawn(executable, received, options) {
    assert.equal(executable, path.join("/package", "native/linux-arm64/better-scaffold"));
    assert.deepEqual(received, args);
    assert.equal(options.stdio, "inherit");
    assert.equal(options.cwd, undefined);
    assert.equal(options.env, undefined);
    return { status: 27, signal: null };
  } });
  assert.equal(result.status, 27);
  assert.throws(() => launch([], { spawn: () => ({ error: Object.assign(new Error("missing"), { code: "ENOENT" }) }) }), /binary.*missing/);
});

test("installed launcher preserves cwd, environment, arguments, and exit code", { skip: process.platform === "win32" }, () => {
  const temporary = mkdtempSync(path.join(tmpdir(), "better-scaffold-launcher-"));
  try {
    const packageRoot = path.join(temporary, "package with spaces");
    const binDirectory = path.join(packageRoot, "bin");
    mkdirSync(binDirectory, { recursive: true });
    const launcher = path.join(binDirectory, "better-scaffold.cjs");
    copyFileSync(source, launcher);
    const executable = binaryPath(process.platform, process.arch, packageRoot);
    mkdirSync(path.dirname(executable), { recursive: true });
    writeFileSync(executable, `#!/usr/bin/env node\nconsole.log(JSON.stringify({ args: process.argv.slice(2), cwd: process.cwd(), marker: process.env.SCAFFOLD_LAUNCHER_TEST }));\nprocess.exitCode = 27;\n`);
    chmodSync(executable, 0o755);
    const args = ["init", "app with spaces"];
    const result = spawnSync(process.execPath, [launcher, ...args], { cwd: temporary, env: { ...process.env, SCAFFOLD_LAUNCHER_TEST: "inherited" }, encoding: "utf8" });
    assert.equal(result.status, 27);
    const output = JSON.parse(result.stdout);
    assert.deepEqual(output.args, args);
    assert.equal(output.cwd, realpathSync(temporary));
    assert.equal(output.marker, "inherited");
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
});

test("native process signals remain signals", { skip: process.platform === "win32" }, () => {
  const temporary = mkdtempSync(path.join(tmpdir(), "better-scaffold-signal-"));
  try {
    mkdirSync(path.join(temporary, "bin"));
    const launcher = path.join(temporary, "bin", "better-scaffold.cjs");
    copyFileSync(source, launcher);
    const executable = binaryPath(process.platform, process.arch, temporary);
    mkdirSync(path.dirname(executable), { recursive: true });
    writeFileSync(executable, "#!/usr/bin/env node\nprocess.kill(process.pid, 'SIGTERM');\n");
    chmodSync(executable, 0o755);
    const result = spawnSync(process.execPath, [launcher]);
    assert.equal(result.signal, "SIGTERM");
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
});
