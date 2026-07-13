// Tests for src/download.js

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { isAllowedHost, releaseAssetUrl } from "../src/download.js";

describe("download", () => {
  describe("releaseAssetUrl", () => {
    it("returns correct GitHub release URL", () => {
      const url = releaseAssetUrl("v0.1.2", "glyphdeck-windows-amd64.exe");
      assert.ok(url.startsWith("https://github.com/"));
      assert.ok(url.includes("v0.1.2"));
      assert.ok(url.includes("glyphdeck-windows-amd64.exe"));
    });

    it("encodes special characters", () => {
      const url = releaseAssetUrl("v0.1.2", "test file.exe");
      assert.ok(url.includes("test%20file.exe"));
    });
  });

  describe("isAllowedHost", () => {
    it("allows github.com", () => {
      assert.strictEqual(isAllowedHost("github.com"), true);
    });

    it("allows objects.githubusercontent.com", () => {
      assert.strictEqual(isAllowedHost("objects.githubusercontent.com"), true);
    });

    it("allows api.github.com", () => {
      assert.strictEqual(isAllowedHost("api.github.com"), true);
    });

    it("rejects evil.com", () => {
      assert.strictEqual(isAllowedHost("evil.com"), false);
    });

    it("rejects github.com.evil.com", () => {
      // This does NOT end with .github.com — it ends with .evil.com
      assert.strictEqual(isAllowedHost("github.com.evil.com"), false);
    });
  });
});
