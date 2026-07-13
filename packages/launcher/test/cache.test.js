// Tests for src/cache.js

import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { cacheDir, binaryPath, checksumsPath, lockPath } from "../src/cache.js";

describe("cache", () => {
  let tempDir;

  beforeEach(async () => {
    tempDir = await mkdtemp(join(tmpdir(), "glyphdeck-test-"));
  });

  afterEach(async () => {
    await rm(tempDir, { recursive: true, force: true });
  });

  it("cacheDir returns GLYPHDECK_LAUNCHER_CACHE_DIR when set", () => {
    process.env.GLYPHDECK_LAUNCHER_CACHE_DIR = tempDir;
    try {
      assert.strictEqual(cacheDir(), tempDir);
    } finally {
      delete process.env.GLYPHDECK_LAUNCHER_CACHE_DIR;
    }
  });

  it("cacheDir returns a platform-specific path when env is not set", () => {
    const dir = cacheDir();
    assert.ok(typeof dir === "string");
    assert.ok(dir.length > 0);
  });

  it("binaryPath returns correct path", () => {
    const result = binaryPath(tempDir, "v0.1.2", "glyphdeck-windows-amd64.exe");
    assert.ok(result.startsWith(tempDir));
    assert.ok(result.includes("v0.1.2"));
    assert.ok(result.includes("glyphdeck-windows-amd64.exe"));
  });

  it("checksumsPath returns correct path", () => {
    const result = checksumsPath(tempDir, "v0.1.2");
    assert.ok(result.startsWith(tempDir));
    assert.ok(result.endsWith("checksums.txt"));
  });

  it("lockPath appends .lock", () => {
    const binary = binaryPath(tempDir, "v0.1.2", "binary");
    assert.strictEqual(lockPath(binary), binary + ".lock");
  });
});
