// Core launcher logic — resolves version, downloads, verifies, and runs GlyphDeck.

import { mkdir, readFile, unlink, access } from "node:fs/promises";
import { open } from "node:fs/promises";
import { join } from "node:path";
import { currentAssetName, isSupported, supportedPlatforms } from "./platform.js";
import { cacheDir, binaryPath, checksumsPath, lockPath } from "./cache.js";
import { downloadBinary, downloadChecksums, releaseAssetUrl } from "./download.js";
import { parseChecksums, expectedChecksum, verifyChecksum } from "./checksums.js";
import { execute } from "./execute.js";

/**
 * Reads the version from package.json relative to the given base path.
 */
function readPackageVersion(packageDir) {
  // packageDir is the directory containing package.json.
  // In development, this is packages/launcher/.
  // In production (npm install), this is the package root.
  const pkgPath = join(packageDir, "package.json");
  return readFile(pkgPath, "utf-8").then((data) => {
    const pkg = JSON.parse(data);
    return pkg.version;
  });
}

/**
 * Maps a semver version string to a GitHub release tag.
 * Stable versions: X.Y.Z -> vX.Y.Z
 * Development version "0.0.0-development" uses the env override or fails.
 */
export function versionToTag(version) {
  // Allow explicit override for development/testing.
  const override = process.env.GLYPHDECK_LAUNCHER_RELEASE_TAG;
  if (override) {
    return validateTag(override);
  }

  if (version === "0.0.0-development") {
    throw new Error(
      "Development version cannot map to a release. " +
        "Set GLYPHDECK_LAUNCHER_RELEASE_TAG to a valid tag (e.g., v0.1.2) for testing."
    );
  }

  // Strip leading v if present, then re-add.
  const cleaned = version.replace(/^v/, "");
  if (!/^\d+\.\d+\.\d+/.test(cleaned)) {
    throw new Error(`Cannot derive release tag from version: ${version}. Expected semver.`);
  }

  return `v${cleaned}`;
}

/**
 * Validates a release tag format.
 */
export function validateTag(tag) {
  if (!/^v\d+\.\d+\.\d+/.test(tag)) {
    throw new Error(`Invalid release tag format: ${tag}. Expected vX.Y.Z.`);
  }
  return tag;
}

/**
 * Acquires an exclusive lock file for a cache entry.
 * Uses a lock file with a PID marker. Handles stale locks.
 */
async function acquireLock(lockFilePath) {
  await mkdir(join(lockFilePath, ".."), { recursive: true });

  for (let attempt = 0; attempt < 30; attempt++) {
    try {
      const fd = await open(lockFilePath, "wx");
      await fd.writeFile(String(process.pid));
      await fd.close();
      return async () => {
        try {
          await unlink(lockFilePath);
        } catch {
          // Lock may already be released.
        }
      };
    } catch (err) {
      if (err.code === "EEXIST") {
        // Check if the lock is stale (older than 5 minutes).
        try {
          const stat = await readFile(lockFilePath, "utf-8");
          const lockPid = parseInt(stat, 10);
          if (isNaN(lockPid) || !isProcessAlive(lockPid)) {
            // Stale lock — remove and retry.
            await unlink(lockFilePath);
            continue;
          }
        } catch {
          // Can't read lock file — remove and retry.
          try { await unlink(lockFilePath); } catch {}
          continue;
        }

        // Wait and retry.
        await new Promise((resolve) => setTimeout(resolve, 1000));
      } else {
        throw err;
      }
    }
  }

  throw new Error(`Could not acquire lock after 30 attempts: ${lockFilePath}`);
}

function isProcessAlive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

/**
 * Ensures the binary is downloaded, verified, and ready to execute.
 * Returns the path to the verified binary.
 */
export async function ensureBinary(version, packageDir) {
  if (!isSupported()) {
    const platforms = supportedPlatforms().join(", ");
    throw new Error(
      `Unsupported platform: ${process.platform}/${process.arch}. Supported: ${platforms}`
    );
  }

  const assetName = currentAssetName();
  const tag = versionToTag(version);
  const root = cacheDir();
  const binPath = binaryPath(root, tag, assetName);
  const chkPath = checksumsPath(root, tag);
  const lockFilePath = lockPath(binPath);

  const releaseLock = await acquireLock(lockFilePath);

  try {
    // Check if cached binary already exists and is valid.
    const isCached = await fileExists(binPath);
    if (isCached) {
      try {
        const checksumsContent = await readFile(chkPath, "utf-8");
        const checksums = parseChecksums(checksumsContent);
        const expected = expectedChecksum(checksums, assetName);
        const valid = await verifyChecksum(binPath, expected);
        if (valid) {
          return binPath; // Cached binary is verified.
        }
        // Corrupt cache — remove and re-download.
        await unlink(binPath);
      } catch {
        // Checksums file missing or corrupt — re-download everything.
      }
    }

    // Download checksums first, then verify.
    await downloadChecksums(tag, chkPath);
    const checksumsContent = await readFile(chkPath, "utf-8");
    const checksums = parseChecksums(checksumsContent);

    // Download the binary.
    await downloadBinary(tag, assetName, binPath);

    // Verify the downloaded binary.
    const expected = expectedChecksum(checksums, assetName);
    const valid = await verifyChecksum(binPath, expected);
    if (!valid) {
      await unlink(binPath).catch(() => {});
      throw new Error(`Checksum verification failed for ${assetName}`);
    }

    return binPath;
  } finally {
    await releaseLock();
  }
}

async function fileExists(path) {
  try {
    await access(path);
    return true;
  } catch {
    return false;
  }
}

/**
 * Clears all cached binaries and checksums.
 */
export async function clearCache(version, packageDir) {
  const tag = versionToTag(version);
  const root = cacheDir();
  const versionDir = join(root, tag);

  try {
    const { rm } = await import("node:fs/promises");
    await rm(versionDir, { recursive: true, force: true });
  } catch {
    // Directory may not exist.
  }
}

/**
 * Returns the path where the binary would be cached (without downloading).
 */
export async function printPath(version, packageDir) {
  if (!isSupported()) {
    throw new Error(`Unsupported platform: ${process.platform}/${process.arch}`);
  }

  const assetName = currentAssetName();
  const tag = versionToTag(version);
  const root = cacheDir();
  return binaryPath(root, tag, assetName);
}
