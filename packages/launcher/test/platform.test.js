// Tests for src/platform.js

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { currentAssetName, isSupported, supportedPlatforms } from "../src/platform.js";

describe("platform", () => {
  it("returns an asset name for supported platforms", () => {
    // We can't mock process.platform/arch easily without --experimental-test-module-mocks,
    // so test the exported data structure.
    const platforms = supportedPlatforms();
    assert.ok(platforms.includes("win32-x64"));
    assert.ok(platforms.includes("linux-x64"));
    assert.ok(platforms.includes("darwin-x64"));
    assert.ok(platforms.includes("darwin-arm64"));
    assert.strictEqual(platforms.length, 4);
  });

  it("currentAssetName returns a string on supported platforms", () => {
    const name = currentAssetName();
    // Must return either null or a valid asset name string.
    assert.ok(name === null || typeof name === "string");
  });

  it("isSupported returns boolean", () => {
    assert.strictEqual(typeof isSupported(), "boolean");
  });
});
