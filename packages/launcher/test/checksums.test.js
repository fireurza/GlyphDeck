// Tests for src/checksums.js

import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { writeFile, unlink, mkdtemp, rm } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { createHash, randomBytes } from "node:crypto";
import { parseChecksums, expectedChecksum, verifyChecksum } from "../src/checksums.js";

describe("checksums", () => {
  describe("parseChecksums", () => {
    it("parses a valid checksums.txt", () => {
      const content = `
abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890  glyphdeck-windows-amd64.exe
0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  glyphdeck-linux-amd64
`;
      const map = parseChecksums(content);
      assert.strictEqual(map.size, 2);
      assert.strictEqual(
        map.get("glyphdeck-windows-amd64.exe"),
        "abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890"
      );
      assert.strictEqual(
        map.get("glyphdeck-linux-amd64"),
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
      );
    });

    it("returns empty map for empty content", () => {
      const map = parseChecksums("");
      assert.strictEqual(map.size, 0);
    });

    it("handles trailing newlines and spaces", () => {
      const content =
        "abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890  asset.exe  \n\n";
      const map = parseChecksums(content);
      assert.strictEqual(map.get("asset.exe"), "abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890");
    });

    it("ignores malformed lines", () => {
      const content = "not-a-valid-line\nabc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890  asset.exe";
      const map = parseChecksums(content);
      assert.strictEqual(map.size, 1);
    });
  });

  describe("expectedChecksum", () => {
    it("returns hash for known asset", () => {
      const map = new Map();
      map.set("asset.exe", "abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890");
      const hash = expectedChecksum(map, "asset.exe");
      assert.strictEqual(hash, "abc123def4567890abcdef1234567890abcdef1234567890abcdef1234567890");
    });

    it("throws for unknown asset", () => {
      const map = new Map();
      assert.throws(() => expectedChecksum(map, "nonexistent"), /not found/);
    });
  });

  describe("verifyChecksum", () => {
    let tempDir;

    beforeEach(async () => {
      tempDir = await mkdtemp(join(tmpdir(), "glyphdeck-test-"));
    });

    afterEach(async () => {
      await rm(tempDir, { recursive: true, force: true });
    });

    async function writeTempFile(name, data) {
      const filePath = join(tempDir, name);
      await writeFile(filePath, data);
      return filePath;
    }

    it("returns true for matching hash", async () => {
      const data = randomBytes(1024);
      const filePath = await writeTempFile("test.bin", data);

      const hash = createHash("sha256").update(data).digest("hex");
      const result = await verifyChecksum(filePath, hash);
      assert.strictEqual(result, true);
    });

    it("returns false for mismatched hash", async () => {
      const data = randomBytes(1024);
      const filePath = await writeTempFile("test.bin", data);

      const result = await verifyChecksum(filePath, "0".repeat(64));
      assert.strictEqual(result, false);
    });

    it("handles case-insensitive comparison", async () => {
      const data = Buffer.from("hello");
      const filePath = await writeTempFile("test.bin", data);

      const hash = createHash("sha256").update(data).digest("hex");
      const result = await verifyChecksum(filePath, hash.toUpperCase());
      assert.strictEqual(result, true);
    });
  });
});
