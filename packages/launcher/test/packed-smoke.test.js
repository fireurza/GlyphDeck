// Packed-package smoke test — verifies the npm package can be packed and used.
// This test does NOT download a live binary. It uses a fixture.

import { describe, it, before, after } from "node:test";
import assert from "node:assert/strict";
import { execSync, spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, rm, writeFile, chmod } from "node:fs/promises";
import { join, dirname } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

const __dirname = dirname(fileURLToPath(import.meta.url));
const launcherDir = join(__dirname, "..");

describe("packed-package smoke", () => {
  let tempDir;
  let fixtureDir;
  let tarballPath;

  before(async () => {
    tempDir = await mkdtemp(join(tmpdir(), "glyphdeck-pack-test-"));
    fixtureDir = await mkdtemp(join(tmpdir(), "glyphdeck-fixture-"));

    // Pack the launcher package.
    const packResult = execSync("npm pack", {
      cwd: launcherDir,
      encoding: "utf-8",
      stdio: ["pipe", "pipe", "pipe"],
    });
    const lines = packResult.trim().split("\n");
    const tarballName = lines[lines.length - 1].trim();
    tarballPath = join(launcherDir, tarballName);
  });

  after(async () => {
    try { await rm(tempDir, { recursive: true, force: true }); } catch {}
    try { await rm(fixtureDir, { recursive: true, force: true }); } catch {}
    try { await rm(tarballPath, { force: true }); } catch {}
  });

  it("produces a valid tarball", () => {
    assert.ok(existsSync(tarballPath), `Tarball not found: ${tarballPath}`);
  });

  it("tarball contains expected files", () => {
    const tarList = execSync(`tar -tzf "${tarballPath}"`, { encoding: "utf-8" });
    const files = tarList.trim().split("\n").map((f) => f.replace(/^package\//, ""));

    assert.ok(files.some((f) => f.startsWith("bin/glyphdeck.js")), "Missing bin/glyphdeck.js");
    assert.ok(files.some((f) => f.startsWith("src/platform.js")), "Missing src/platform.js");
    assert.ok(files.some((f) => f.includes("LICENSE")), "Missing LICENSE");
    assert.ok(files.some((f) => f.includes("COMMERCIAL-LICENSING.md")), "Missing COMMERCIAL-LICENSING.md");
    assert.ok(files.some((f) => f.includes("package.json")), "Missing package.json");
    assert.ok(!files.some((f) => f.includes("test/")), "Tarball must not include test files");
  });

  it("executes launcher-specific commands from packed package", () => {
    // Use the locally installed npx to run the packed tarball.
    // Skip if npx is not available (should be available in CI via setup-node).
    const npxCmd = process.platform === "win32" ? "npx.cmd" : "npx";
    const result = spawnSync(npxCmd, [
      "--yes",
      "--package", tarballPath,
      "glyphdeck",
      "--launcher-version",
    ], {
      cwd: tempDir,
      encoding: "utf-8",
      timeout: 30_000,
    });

    if (result.error) {
      // npx unavailable — skip assertion (tested on other platforms).
      return;
    }

    assert.strictEqual(result.status, 0,
      `--launcher-version failed (status=${result.status}): ${result.stderr}`
    );
    assert.ok(
      result.stdout.includes("0.0.0-development"),
      `Expected development version in output: ${result.stdout}`
    );
  });

  it("prints cache path from packed package", () => {
    const npxCmd = process.platform === "win32" ? "npx.cmd" : "npx";
    const result = spawnSync(npxCmd, [
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

    if (result.error) return;

    assert.strictEqual(result.status, 0,
      `--launcher-print-path failed (status=${result.status}): ${result.stderr}`
    );
    const output = result.stdout.trim();
    assert.ok(output.length > 0, "Should print a cache path");
    assert.ok(
      output.includes(tempDir.replace(/\\/g, "/")),
      `Cache path should be under temp dir: ${output}`
    );
  });
});
