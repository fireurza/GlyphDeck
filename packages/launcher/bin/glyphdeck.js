#!/usr/bin/env node
// GlyphDeck launcher — downloads and runs the verified release binary.

import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { readFile } from "node:fs/promises";
import { ensureBinary, clearCache, printPath } from "../src/launcher.js";
import { execute } from "../src/execute.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const packageDir = join(__dirname, "..");

const args = process.argv.slice(2);

// Handle launcher-specific commands.
if (args[0] === "--launcher-version") {
  const pkg = JSON.parse(await readFile(join(packageDir, "package.json"), "utf-8"));
  process.stdout.write(`${pkg.version}\n`);
  process.exit(0);
}

if (args[0] === "--launcher-print-path") {
  try {
    const pkg = JSON.parse(await readFile(join(packageDir, "package.json"), "utf-8"));
    const path = await printPath(pkg.version, packageDir);
    process.stdout.write(`${path}\n`);
  } catch (err) {
    process.stderr.write(`error: ${err.message}\n`);
    process.exit(1);
  }
  process.exit(0);
}

if (args[0] === "--launcher-clear-cache") {
  try {
    const pkg = JSON.parse(await readFile(join(packageDir, "package.json"), "utf-8"));
    await clearCache(pkg.version, packageDir);
    process.stdout.write("Cache cleared.\n");
  } catch (err) {
    process.stderr.write(`error: ${err.message}\n`);
    process.exit(1);
  }
  process.exit(0);
}

// Main flow: ensure binary is available and execute it.
try {
  const pkg = JSON.parse(await readFile(join(packageDir, "package.json"), "utf-8"));
  const binaryPath = await ensureBinary(pkg.version, packageDir);
  const exitCode = await execute(binaryPath, args);
  process.exit(exitCode);
} catch (err) {
  process.stderr.write(`glyphdeck: ${err.message}\n`);
  process.exit(1);
}
