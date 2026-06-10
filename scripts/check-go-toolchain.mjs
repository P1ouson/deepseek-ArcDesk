#!/usr/bin/env node
/**
 * Release gate: refuse builds on Go toolchains below the patched minimum in go.mod.
 */
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const modPath = path.join(root, "go.mod");
const mod = readFileSync(modPath, "utf8");
const m = mod.match(/^go\s+(\d+\.\d+(?:\.\d+)?)/m);
if (!m) {
  console.error("check-go-toolchain: could not parse go version from go.mod");
  process.exit(1);
}
const required = m[1].split(".").map((n) => parseInt(n, 10));
while (required.length < 3) required.push(0);

const verRes = spawnSync("go", ["env", "GOVERSION"], { encoding: "utf8" });
if (verRes.status !== 0) {
  console.error("check-go-toolchain: go env GOVERSION failed");
  process.exit(1);
}
const raw = (verRes.stdout || "").trim().replace(/^go/, "");
const got = raw.split(".").map((n) => parseInt(n, 10));
while (got.length < 3) got.push(0);

function cmp(a, b) {
  for (let i = 0; i < 3; i++) {
    if (a[i] !== b[i]) return a[i] - b[i];
  }
  return 0;
}

if (cmp(got, required) < 0) {
  console.error(
    `check-go-toolchain: GOVERSION go${raw} is below required go${required.join(".")} — upgrade Go before release builds`,
  );
  process.exit(1);
}

console.log(`check-go-toolchain: ok (go${raw} >= go${required.join(".")})`);
