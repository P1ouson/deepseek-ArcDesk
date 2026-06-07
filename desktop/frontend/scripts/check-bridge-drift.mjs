#!/usr/bin/env node
/**
 * Ensures hand-written AppBindings in bridge.ts covers every exported App method
 * in the desktop Go module (the Wails binding surface).
 */
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const desktopDir = path.resolve(root, "..");
const bridgePath = path.join(root, "src", "lib", "bridge.ts");

function collectGoAppMethods() {
  const methods = new Set();
  const pattern = /func \(a \*App\) ([A-Z][A-Za-z0-9_]*)\(/g;
  for (const name of readdirSync(desktopDir)) {
    if (!name.endsWith(".go")) continue;
    const text = readFileSync(path.join(desktopDir, name), "utf8");
    for (const match of text.matchAll(pattern)) {
      methods.add(match[1]);
    }
  }
  return methods;
}

function collectAppBindingMethods() {
  const text = readFileSync(bridgePath, "utf8");
  const start = text.indexOf("export interface AppBindings");
  if (start < 0) throw new Error("AppBindings interface not found in bridge.ts");
  const end = text.indexOf("\n}", start);
  const block = text.slice(start, end);
  const methods = new Set();
  const pattern = /^\s+([A-Z][A-Za-z0-9_]*)\(/gm;
  for (const match of block.matchAll(pattern)) {
    methods.add(match[1]);
  }
  return methods;
}

const goMethods = collectGoAppMethods();
const tsMethods = collectAppBindingMethods();

const missingInTS = [...goMethods].filter((name) => !tsMethods.has(name)).sort();
const staleInTS = [...tsMethods].filter((name) => !goMethods.has(name)).sort();

if (missingInTS.length || staleInTS.length) {
  if (missingInTS.length) {
    console.error("Go App methods missing from AppBindings:");
    for (const name of missingInTS) console.error(`  + ${name}`);
  }
  if (staleInTS.length) {
    console.error("AppBindings methods with no Go counterpart:");
    for (const name of staleInTS) console.error(`  - ${name}`);
  }
  process.exit(1);
}

console.log(`Bridge drift check OK (${goMethods.size} methods aligned).`);
