import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { collectNotices } from "./notices.mjs";

function fixture(t) {
  const root = mkdtempSync(path.join(tmpdir(), "scaffold-notices-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const environment = { GOMODCACHE: path.join(root, "modules"), GOROOT: path.join(root, "go"), GOVERSION: "go1.26" };
  function file(name, value) { mkdirSync(path.dirname(path.join(root, name)), { recursive: true }); writeFileSync(path.join(root, name), value); }
  file("go/LICENSE", "Go license");
  file("go/src/vendor/example/LICENSE", "Vendor notice");
  file("modules/example.com/common@v1.0.0/LICENSE", "Common license");
  file("modules/example.com/common@v1.0.0/nested/NOTICE", "Nested attribution");
  file("modules/example.com/windows@v2.0.0/LICENSE", "Windows license");
  const runGo = (args) => JSON.stringify(args[0] === "env" ? environment : { GoVersion: environment.GOVERSION, Deps: [{ Path: "example.com/common", Version: "v1.0.0" }, ...(args.at(-1) === "windows" ? [{ Path: "example.com/windows", Version: "v2.0.0" }] : [])] });
  return { root, file, runGo, output: path.join(root, "package") };
}

test("notices include union of platform dependencies and original nested attributions", (t) => {
  const f = fixture(t);
  const index = collectNotices(["mac", "windows"], f.output, f.runGo);
  assert.deepEqual(index.components.map((component) => component.name), ["example.com/common", "example.com/windows", "go"]);
  const nested = index.components[0].documents.find((name) => name.endsWith("nested/NOTICE"));
  assert.equal(readFileSync(path.join(f.output, nested), "utf8"), "Nested attribution");
  assert.ok(index.components.at(-1).documents.some((name) => name.endsWith("vendor/example/LICENSE")));
});

test("missing dependency licenses prevent packaging", (t) => {
  const f = fixture(t);
  rmSync(path.join(f.root, "modules/example.com/common@v1.0.0/LICENSE"));
  assert.throws(() => collectNotices(["mac"], f.output, f.runGo), /No license document found/);
});
