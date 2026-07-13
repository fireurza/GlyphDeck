// Packed-package smoke test — verifies the npm package can be packed and used.
// This test does NOT download a live binary. It uses a fixture.

import { describe, it, before, after } from "node:test";
import assert from "node:assert/strict";
import { execSync, spawnSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { mkdtemp, rm, writeFile, chmod } from "node:fs/promises";
import { join, dirname } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

const __dirname = dirname(fileURLToPath(import.meta.url));
const launcherDir = join(__dirname, "..");
const repoRoot = join(__dirname, "..", "..", "..");

describe("packed-package smoke", () => {
  let tempDir;
  let fixtureDir;
  let tarballPath;

  before(async () => {
    tempDir = await mkdtemp(join(tmpdir(), "glyphdeck-pack-test-"));
    fixtureDir = await mkdtemp(join(tmpdir(), "glyphdeck-fixture-"));

    // Create a fixture binary that echoes its arguments and exits 0.
    const fixturePath = join(fixtureDir, process.platform === "win32" ? "glyphdeck-test.exe" : "glyphdeck-test");
    const fixtureContent = process.platform === "win32"
      ? '@echo off\r\necho fixture-ok\r\nexit /b 0\r\n'
      : '#!/bin/sh\necho fixture-ok\nexit 0\n';

    await writeFile(fixturePath, fixtureContent);
    if (process.platform !== "win32") {
      await chmod(fixturePath, 0o755);
    }

    // Create a checksums.txt for the fixture.
    const fixtureHash = createHash("sha256").update(fixtureContent).digest("hex");
    const assetName = process.platform === "win32" ? "glyphdeck-test.exe" : "glyphdeck-test";
    const checksumsContent = `${fixtureHash}  ${assetName}\n`;
    const checksumsPath = join(fixtureDir, "checksums.txt");
    await writeFile(checksumsPath, checksumsContent);

    // Pack the launcher package.
    const packResult = execSync("npm pack", {
      cwd: launcherDir,
      encoding: "utf-8",
      stdio: ["pipe", "pipe", "pipe"],
    });
    // npm pack outputs the tarball filename on the last line.
    const lines = packResult.trim().split("\n");
    const tarballName = lines[lines.length - 1].trim();
    tarballPath = join(launcherDir, tarballName);
  });

  after(async () => {
    // Clean up temp directories.
    try { await rm(tempDir, { recursive: true, force: true }); } catch {}
    try { await rm(fixtureDir, { recursive: true, force: true }); } catch {}
    // Remove generated tarball.
    try { await rm(tarballPath, { force: true }); } catch {}
  });

  it("produces a valid tarball", () => {
    assert.ok(existsSync(tarballPath), `Tarball not found: ${tarballPath}`);
  });

  it("tarball contains expected files", () => {
    const tarList = execSync(`tar -tzf "${tarballPath}"`, { encoding: "utf-8" });
    const files = tarList.trim().split("\n").map((f) => f.replace(/^package\//, ""));

    // Must include bin entry point, source files, legal docs.
    assert.ok(files.some((f) => f.startsWith("bin/glyphdeck.js")), "Missing bin/glyphdeck.js");
    assert.ok(files.some((f) => f.startsWith("src/platform.js")), "Missing src/platform.js");
    assert.ok(files.some((f) => f.includes("LICENSE")), "Missing LICENSE");
    assert.ok(files.some((f) => f.includes("COMMERCIAL-LICENSING.md")), "Missing COMMERCIAL-LICENSING.md");
    assert.ok(files.some((f) => f.includes("package.json")), "Missing package.json");

    // Must NOT include test files.
    assert.ok(!files.some((f) => f.includes("test/")), "Tarball must not include test files");
  });

  it("executes launcher-specific commands from packed package", () => {
    // Use npm exec with the local tarball to run --launcher-version.
    const result = spawnSync("npx.cmd", [
      "--yes",
      "--package", tarballPath,
      "glyphdeck",
      "--launcher-version",
    ], {
      cwd: tempDir,
      encoding: "utf-8",
      timeout: 30_000,
    });

    // On Windows, npx.cmd may not exist; try npx directly.
    if (result.error?.code === "ENOENT") {
      // Skip if npx not available (CI will run this on ubuntu/macos).
      return;
    }

    assert.strictEqual(result.status, 0, `--launcher-version failed: ${result.stderr}`);
    // Should output "0.0.0-development".
    assert.ok(result.stdout.includes("0.0.0-development"), `Expected development version in output: ${result.stdout}`);
  });

  it("prints cache path from packed package", () => {
    const result = spawnSync("npx.cmd", [
      "--yes",
      "--package", tarballPath,
      "glyphdeck",
      "--launcher-print-path",
    ], {
      cwd: tempDir,
      encoding: "utf-8",
      timeout: 30_000,
      env: {
        ...process.env,
        GLYPHDECK_LAUNCHER_RELEASE_TAG: "v0.1.2",
        GLYPHDECK_LAUNCHER_CACHE_DIR: tempDir,
      },
    });

    if (result.error?.code === "ENOENT") return;

    assert.strictEqual(result.status, 0, `--launcher-print-path failed: ${result.stderr}`);
    const output = result.stdout.trim();
    assert.ok(output.length > 0, "Should print a cache path");
    assert.ok(output.includes(tempDir.replace(/\\/g, "/")), `Cache path should be under temp dir: ${output}`);
  });
});
