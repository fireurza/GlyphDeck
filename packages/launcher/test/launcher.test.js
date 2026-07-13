// Tests for src/launcher.js — version mapping and validation.

import { describe, it, beforeEach, afterEach } from "node:test";
import assert from "node:assert/strict";
import { versionToTag, validateTag } from "../src/launcher.js";

describe("launcher", () => {
  describe("validateTag", () => {
    it("accepts valid semver tags", () => {
      assert.strictEqual(validateTag("v0.1.2"), "v0.1.2");
      assert.strictEqual(validateTag("v1.0.0"), "v1.0.0");
      assert.strictEqual(validateTag("v10.20.30"), "v10.20.30");
    });

    it("rejects invalid tags", () => {
      assert.throws(() => validateTag("v1.2"), /Invalid release tag/);
      assert.throws(() => validateTag("1.2.3"), /Invalid release tag/);
      assert.throws(() => validateTag("latest"), /Invalid release tag/);
      assert.throws(() => validateTag(""), /Invalid release tag/);
    });
  });

  describe("versionToTag", () => {
    afterEach(() => {
      delete process.env.GLYPHDECK_LAUNCHER_RELEASE_TAG;
    });

    it("maps semver to v-prefixed tag", () => {
      assert.strictEqual(versionToTag("0.1.2"), "v0.1.2");
      assert.strictEqual(versionToTag("1.0.0"), "v1.0.0");
    });

    it("strips existing v prefix", () => {
      assert.strictEqual(versionToTag("v0.1.2"), "v0.1.2");
    });

    it("uses env override when set", () => {
      process.env.GLYPHDECK_LAUNCHER_RELEASE_TAG = "v9.9.9";
      assert.strictEqual(versionToTag("0.0.0-development"), "v9.9.9");
    });

    it("rejects invalid override tags", () => {
      process.env.GLYPHDECK_LAUNCHER_RELEASE_TAG = "invalid";
      assert.throws(() => versionToTag("0.0.0-development"), /Invalid release tag/);
    });

    it("throws for development version without override", () => {
      assert.throws(() => versionToTag("0.0.0-development"), /Development version/);
    });

    it("throws for non-semver versions", () => {
      assert.throws(() => versionToTag("not-semver"), /Cannot derive/);
    });
  });
});
