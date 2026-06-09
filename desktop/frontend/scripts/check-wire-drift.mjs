#!/usr/bin/env node
/**
 * Ensures desktop/wire.go and internal/serve/wire.go share the same JSON field tags
 * on wire types, and that WireEvent in types.ts covers the desktop wireEvent fields.
 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repo = path.resolve(root, "../..");

function collectGoJsonFields(filePath, structName) {
  const text = readFileSync(filePath, "utf8");
  const start = text.indexOf(`type ${structName} struct`);
  if (start < 0) throw new Error(`${structName} not found in ${filePath}`);
  const end = text.indexOf("\n}", start);
  const block = text.slice(start, end);
  const fields = new Map();
  for (const match of block.matchAll(/^\s+(\w+)\s+\S+\s+`json:"([^"]+)"`/gm)) {
    const tag = match[2].split(",")[0];
    if (tag && tag !== "-") fields.set(match[1], tag);
  }
  return fields;
}

function collectTsInterfaceFields(filePath, ifaceName) {
  const text = readFileSync(filePath, "utf8");
  const start = text.indexOf(`export interface ${ifaceName}`);
  if (start < 0) throw new Error(`${ifaceName} not found in ${filePath}`);
  const end = text.indexOf("\n}", start);
  const block = text.slice(start, end);
  const fields = new Set();
  for (const match of block.matchAll(/^\s+(\w+)\??:/gm)) {
    fields.add(match[1]);
  }
  return fields;
}

const desktopWire = path.join(repo, "desktop", "wire.go");
const serveWire = path.join(repo, "internal", "serve", "wire.go");
const typesTs = path.join(root, "src", "lib", "types.ts");

const desktopEvent = collectGoJsonFields(desktopWire, "wireEvent");
const serveEvent = collectGoJsonFields(serveWire, "wireEvent");
const tsEvent = collectTsInterfaceFields(typesTs, "WireEvent");

const goMismatch = [];
for (const field of desktopEvent.keys()) {
  if (!serveEvent.has(field)) continue;
  if (serveEvent.get(field) !== desktopEvent.get(field)) {
    goMismatch.push(`${field}: desktop=${desktopEvent.get(field)} serve=${serveEvent.get(field)}`);
  }
}
const desktopOnly = [...desktopEvent.keys()].filter((field) => !serveEvent.has(field));
const serveOnly = [...serveEvent.keys()].filter((field) => !desktopEvent.has(field));
if (desktopOnly.length) {
  console.log(`NOTE: desktop-only wireEvent fields: ${desktopOnly.join(", ")}`);
}
if (serveOnly.length) {
  console.log(`NOTE: serve-only wireEvent fields: ${serveOnly.join(", ")}`);
}

const tsMissing = [];
for (const [, tag] of desktopEvent) {
  if (tag === "kind") continue;
  const camel = tag.replace(/_([a-z])/g, (_, c) => c.toUpperCase());
  const candidates = [tag, camel];
  if (tag === "err") candidates.push("err");
  if (!candidates.some((name) => tsEvent.has(name))) {
    tsMissing.push(tag);
  }
}

if (goMismatch.length || tsMissing.length) {
  if (goMismatch.length) {
    console.error("wire.go drift between desktop and serve:");
    for (const line of goMismatch) console.error(`  ${line}`);
  }
  if (tsMissing.length) {
    console.error("WireEvent missing desktop wire fields:");
    for (const line of tsMissing) console.error(`  ${line}`);
  }
  process.exit(1);
}

console.log(`Wire drift check OK (${desktopEvent.size} wireEvent fields aligned).`);
