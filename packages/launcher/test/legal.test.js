// Verifies that the launcher package's legal files match the repository root copies.

import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(__dirname, "..", "..", "..");
const pkgRoot = join(__dirname, "..");

describe("package legal files", () => {
  it("LICENSE matches repository root copy", async () => {
    const rootLicense = await readFile(join(repoRoot, "LICENSE"), "utf-8");
    const pkgLicense = await readFile(join(pkgRoot, "LICENSE"), "utf-8");
    assert.strictEqual(pkgLicense, rootLicense, "LICENSE must match repository root");
  });

  it("COMMERCIAL-LICENSING.md matches repository root copy", async () => {
    const rootCommercial = await readFile(join(repoRoot, "COMMERCIAL-LICENSING.md"), "utf-8");
    const pkgCommercial = await readFile(join(pkgRoot, "COMMERCIAL-LICENSING.md"), "utf-8");
    assert.strictEqual(pkgCommercial, rootCommercial, "COMMERCIAL-LICENSING.md must match repository root");
  });
});
