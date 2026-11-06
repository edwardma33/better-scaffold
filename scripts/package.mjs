import { spawnSync } from "node:child_process";
import { chmodSync, cpSync, existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { collectNotices } from "./notices.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = path.join(root, "npm", "better-scaffold");
const dist = path.join(root, "dist");
const prepared = path.join(dist, "npm", "better-scaffold");

const targets = [
  { platform: "darwin", arch: "arm64", goos: "darwin", goarch: "arm64" },
  { platform: "darwin", arch: "x64", goos: "darwin", goarch: "amd64" },
  { platform: "linux", arch: "arm64", goos: "linux", goarch: "arm64" },
  { platform: "linux", arch: "x64", goos: "linux", goarch: "amd64" },
  { platform: "win32", arch: "arm64", goos: "windows", goarch: "arm64" },
  { platform: "win32", arch: "x64", goos: "windows", goarch: "amd64" },
];

function run(executable, args, options = {}) {
  const result = spawnSync(executable, args, { cwd: root, stdio: "inherit", ...options });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${executable} failed with ${result.signal ?? result.status}`);
  return result.stdout;
}

function ensureDirectory(directory) {
  try {
    if (lstatSync(directory).isSymbolicLink()) throw new Error(`Refusing symlink output directory ${directory}`);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  mkdirSync(directory, { recursive: true });
}

function npm(args, options) {
  if (process.platform !== "win32") return run("npm", args, options);
  const cli = process.env.npm_execpath ?? path.join(path.dirname(process.execPath), "node_modules", "npm", "bin", "npm-cli.js");
  if (!existsSync(cli)) throw new Error("Cannot find npm-cli.js. Use an official Node.js installation with npm to build the package on Windows.");
  return run(process.execPath, [cli, ...args], options);
}

function build() {
  const args = process.argv.slice(2);
  if (args.length > 1 || (args.length === 1 && args[0] !== "--all")) {
    throw new Error("Usage: node scripts/package.mjs [--all]");
  }
  const all = args.includes("--all");
  const manifest = JSON.parse(readFileSync(path.join(source, "package.json"), "utf8"));
  if (manifest.name !== "better-scaffold" || !/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(manifest.version)) {
    throw new Error("Expected better-scaffold and a semantic version in npm/better-scaffold/package.json");
  }
  const selected = all ? targets : targets.filter(({ platform, arch }) => platform === process.platform && arch === process.arch);
  if (selected.length === 0) throw new Error(`Unsupported build host ${process.platform}-${process.arch}`);
  ensureDirectory(dist);
  ensureDirectory(path.join(dist, "npm"));
  ensureDirectory(prepared);
  rmSync(prepared, { recursive: true, force: true });
  mkdirSync(prepared, { recursive: true });
  cpSync(path.join(source, "bin"), path.join(prepared, "bin"), { recursive: true });
  chmodSync(path.join(prepared, "bin", "better-scaffold.cjs"), 0o755);
  cpSync(path.join(source, "README.md"), path.join(prepared, "README.md"));
  cpSync(path.join(root, "LICENSE"), path.join(prepared, "LICENSE"));
  manifest.private = !all;
  writeFileSync(path.join(prepared, "package.json"), JSON.stringify(manifest, null, 2) + "\n");
  const binaries = [];
  for (const target of selected) {
    const key = `${target.platform}-${target.arch}`;
    const directory = path.join(prepared, "native", key);
    mkdirSync(directory, { recursive: true });
    const output = path.join(directory, target.platform === "win32" ? "better-scaffold.exe" : "better-scaffold");
    console.log(`Building ${key}`);
    run("go", ["build", "-trimpath", "-ldflags", `-s -w -X main.version=${manifest.version}`, "-o", output, "./cmd/better-scaffold"], {
      env: { ...process.env, CGO_ENABLED: "0", GOOS: target.goos, GOARCH: target.goarch },
    });
    binaries.push(output);
  }
  const notices = collectNotices(binaries, prepared);
  console.log(`Collected notices for ${notices.components.length} dependency components`);
  const cache = mkdtempSync(path.join(tmpdir(), "better-scaffold-pack-cache-"));
  try {
    npm(["pack", "--ignore-scripts", "--offline", "--cache", cache, "--pack-destination", dist], { cwd: prepared });
  } finally {
    rmSync(cache, { recursive: true, force: true });
  }
  console.log(`Package ready: ${path.join(dist, `${manifest.name}-${manifest.version}.tgz`)}`);
  if (!all) console.log("This private development package contains only the host binary. Use --all for release packaging.");
}

try {
  build();
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
