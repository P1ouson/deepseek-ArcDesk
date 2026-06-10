#!/usr/bin/env node
/**
 * Release readiness gate for CI / local validation.
 * Browser preview (vite) uses the mock bridge — Wails binary is required for full QA.
 */
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const desktop = path.resolve(root, "..");

function run(cmd, args, cwd) {
  const res = spawnSync(cmd, args, { cwd, stdio: "inherit", shell: process.platform === "win32" });
  if (res.status !== 0) process.exit(res.status ?? 1);
}

console.log("==> Go toolchain");
run("node", [path.resolve(root, "..", "..", "scripts", "check-go-toolchain.mjs")], root);

console.log("==> TypeScript");
run("pnpm", ["typecheck"], root);

console.log("==> Bridge drift");
run("pnpm", ["check:bridge"], root);

console.log("==> Wire drift");
run("pnpm", ["check:wire"], root);
console.log("NOTE: mock behavior is stubbed in bridge.ts; binding parity is enforced above — use Wails binary for kernel QA.");

console.log("==> Unit tests");
run("pnpm", ["test:unit"], root);

console.log("==> Frontend production build (Wails embed source)");
run("pnpm", ["build"], root);

const distIndex = path.join(root, "dist", "index.html");
const distAssets = path.join(root, "dist", "assets");
if (!existsSync(distIndex) || !existsSync(distAssets)) {
  console.error("P0: frontend/dist missing after build — Wails would embed stale or empty UI.");
  process.exit(1);
}

console.log("==> Go build");
run("go", ["build", "-o", process.platform === "win32" ? "NUL" : "/dev/null", "."], desktop);

console.log("==> Go tests");
run("go", ["test", "./..."], desktop);

console.log("==> Playwright smoke (mock preview)");
run("pnpm", ["test:e2e"], root);

console.log("==> Wails binary compile (optional, RELEASE_WAILS=1)");
const wailsProbe = spawnSync("wails", ["version"], {
  cwd: desktop,
  shell: process.platform === "win32",
  encoding: "utf8",
});
if (process.env.RELEASE_WAILS === "1") {
  if (wailsProbe.status !== 0) {
    console.error("RELEASE_WAILS=1 but wails CLI is not available.");
    process.exit(1);
  }
  const version = (wailsProbe.stdout || wailsProbe.stderr || "").trim().split("\n")[0];
  console.log(`Wails CLI (${version}). Building desktop binary (-s skips frontend rebuild).`);
  console.log("After build, run the manual checklist: desktop/frontend/docs/wails-smoke-checklist.md");
  run("wails", ["build", "-s"], desktop);
} else if (wailsProbe.status === 0) {
  console.log("NOTE: wails CLI found. Set RELEASE_WAILS=1 to include `wails build -s` in this gate.");
} else {
  console.log("NOTE: wails CLI not available — go build above validates embed; run `wails build` before release.");
}

console.log("\nRelease check passed (embed + compile + mock smoke).");
