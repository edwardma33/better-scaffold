import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

function go(args) {
  const result = spawnSync("go", args, { encoding: "utf8" });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Collecting dependency notices failed: ${result.stderr}`);
  return result.stdout;
}

function documents(directory, prefix = "") {
  const found = [];
  for (const entry of readdirSync(path.join(directory, prefix), { withFileTypes: true })) {
    const relative = path.join(prefix, entry.name);
    if (entry.isDirectory()) {
      if (!["testdata", "test", "tests", ".git", "node_modules"].includes(entry.name)) found.push(...documents(directory, relative));
    } else if (entry.isFile() && !/\.(?:go|c|h|js|py)$/i.test(entry.name) && /^(?:licen[sc]e|copying|notice|patents|authors|copyright)(?:$|[._-])/i.test(entry.name)) {
      found.push(relative);
    }
  }
  return found.sort();
}

function escaped(value) {
  return value.replace(/[A-Z]/g, (letter) => `!${letter.toLowerCase()}`);
}

export function collectNotices(binaries, output, runGo = go) {
  const environment = JSON.parse(runGo(["env", "-json", "GOMODCACHE", "GOROOT", "GOVERSION"]));
  const modules = new Map();
  for (const binary of binaries) {
    const info = JSON.parse(runGo(["version", "-m", "-json", binary]));
    if (info.GoVersion !== environment.GOVERSION) throw new Error(`Compiler version differs for ${binary}; rebuild before collecting notices`);
    for (const original of info.Deps ?? []) {
      const dependency = original.Replace ?? original;
      if (!dependency.Version || dependency.Version === "(devel)") throw new Error(`Cannot collect notices for unversioned dependency ${dependency.Path}`);
      modules.set(`${dependency.Path}@${dependency.Version}`, dependency);
    }
  }
  const index = { formatVersion: 1, components: [] };
  function copy(name, version, source, paths, declaration) {
    if (!declaration && !paths.some((p) => /^(?:licen[sc]e|copying)(?:$|[._-])/i.test(path.basename(p)))) throw new Error(`No license document found for ${name}@${version}`);
    const destination = path.join(output, "third-party", `${name}@${version}`);
    for (const relative of paths) {
      if (!readFileSync(path.join(source, relative), "utf8").trim()) throw new Error(`Empty dependency notice for ${name}@${version}: ${relative}`);
      mkdirSync(path.dirname(path.join(destination, relative)), { recursive: true });
      cpSync(path.join(source, relative), path.join(destination, relative));
    }
    index.components.push({ name, version, ...(declaration ? { declaration } : {}), documents: paths.map((relative) => path.relative(output, path.join(destination, relative)).split(path.sep).join("/")) });
  }
  for (const [, dependency] of [...modules].sort(([a], [b]) => a.localeCompare(b))) {
    const source = path.join(environment.GOMODCACHE, `${escaped(dependency.Path)}@${escaped(dependency.Version)}`);
    if (!existsSync(source)) throw new Error(`Missing dependency source ${dependency.Path}@${dependency.Version}; download modules before packaging`);
    const files = documents(source);
    if (`${dependency.Path}@${dependency.Version}` === "github.com/mattn/go-localereader@v0.0.1") {
      if (!/## License\s+MIT\s+## Author\s+Yasuhiro Matsumoto/.test(readFileSync(path.join(source, "README.md"), "utf8"))) throw new Error("Upstream go-localereader license declaration changed");
      files.push("README.md");
      copy(dependency.Path, dependency.Version, source, files, "MIT, declared in the upstream README; this version contains no separate license file");
    } else {
      copy(dependency.Path, dependency.Version, source, files);
    }
  }
  const goDocuments = ["LICENSE", "PATENTS"].filter((name) => existsSync(path.join(environment.GOROOT, name)));
  goDocuments.push(...documents(path.join(environment.GOROOT, "src")).map((name) => path.join("src", name)));
  copy("go", environment.GOVERSION, environment.GOROOT, goDocuments);
  mkdirSync(path.join(output, "third-party"), { recursive: true });
  writeFileSync(path.join(output, "third-party", "index.json"), JSON.stringify(index, null, 2) + "\n");
  return index;
}
