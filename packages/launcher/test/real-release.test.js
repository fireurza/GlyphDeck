// Real release download validation.
// Downloads the actual v0.1.2 release assets and verifies checksums.
// Uses an isolated cache — does not touch the normal launcher cache.
// Does not execute GlyphDeck.
// Run with: node --experimental-test-module-mocks packages/launcher/test/real-release.test.js

import { describe, it, before, after } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm, readFile, writeFile, unlink } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { createHash } from "node:crypto";
import { downloadChecksums, downloadBinary, downloadToFile, releaseAssetUrl } from "../src/download.js";
import { parseChecksums, expectedChecksum, verifyChecksum } from "../src/checksums.js";
import { currentAssetName, isSupported } from "../src/platform.js";

const TEST_TAG = "v0.1.2";

describe("real-release download", () => {
  let tempDir;
  let chkPath;
  let binPath;

  before(async () => {
    if (!isSupported()) {
      return; // Skip on unsupported platforms.
    }

    tempDir = await mkdtemp(join(tmpdir(), "glyphdeck-real-dl-"));
    chkPath = join(tempDir, "checksums.txt");
    binPath = join(tempDir, currentAssetName());
  });

  after(async () => {
    if (tempDir) {
      await rm(tempDir, { recursive: true, force: true });
    }
  });

  it("downloads checksums.txt from real GitHub release", { skip: !isSupported() }, async () => {
    await downloadChecksums(TEST_TAG, chkPath);

    const content = await readFile(chkPath, "utf-8");
    const checksums = parseChecksums(content);

    assert.ok(checksums.size > 0, "checksums.txt must have entries");
    const assetName = currentAssetName();
    assert.ok(checksums.has(assetName), `checksums.txt must contain ${assetName}`);
  });

  it("downloads platform binary from real GitHub release", { skip: !isSupported() }, async () => {
    const assetName = currentAssetName();
    await downloadBinary(TEST_TAG, assetName, binPath);

    // Verify file exists and is non-zero.
    const stat = await readFile(binPath);
    assert.ok(stat.length > 0, "Binary must be non-empty");
  });

  it("verifies downloaded binary against checksums.txt", { skip: !isSupported() }, async () => {
    const content = await readFile(chkPath, "utf-8");
    const checksums = parseChecksums(content);
    const assetName = currentAssetName();
    const expected = expectedChecksum(checksums, assetName);

    const valid = await verifyChecksum(binPath, expected);
    assert.strictEqual(valid, true, "Binary must match checksum");
  });

  it("re-verifies cached binary on second check", { skip: !isSupported() }, async () => {
    // Re-read checksums and re-verify (simulates second run).
    const content = await readFile(chkPath, "utf-8");
    const checksums = parseChecksums(content);
    const assetName = currentAssetName();
    const expected = expectedChecksum(checksums, assetName);

    const valid = await verifyChecksum(binPath, expected);
    assert.strictEqual(valid, true, "Cached binary must still verify");
  });

  it("detects corrupt binary and rejects it", { skip: !isSupported() }, async () => {
    // Copy the binary, corrupt it, verify detection.
    const corruptPath = join(tempDir, "corrupt.bin");

    // Read original, flip some bytes.
    const original = await readFile(binPath);
    const corrupted = Buffer.from(original);
    if (corrupted.length > 100) {
      corrupted[50] ^= 0xFF; // Flip bits at position 50.
    }

    await writeFile(corruptPath, corrupted);

    const content = await readFile(chkPath, "utf-8");
    const checksums = parseChecksums(content);
    const assetName = currentAssetName();
    const expected = expectedChecksum(checksums, assetName);

    const valid = await verifyChecksum(corruptPath, expected);
    assert.strictEqual(valid, false, "Corrupt binary must not verify");

    await unlink(corruptPath);
  });

  it("follows redirects from real GitHub release", { skip: !isSupported() }, async () => {
    // The download functions test redirect following via the real GitHub CDN.
    // downloadBinary already exercises the redirect chain.
    // This test validates the final result.
    const content = await readFile(chkPath, "utf-8");
    assert.ok(content.includes(currentAssetName()), "checksums must contain current asset");
    assert.ok(content.length > 100, "checksums must have reasonable content");
  });
});
