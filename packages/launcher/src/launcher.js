// Core launcher logic — resolves version, downloads, verifies, and runs GlyphDeck.

import { readFile, unlink, stat } from "node:fs/promises";
import { open, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { currentAssetName, isSupported, supportedPlatforms } from "./platform.js";
import { cacheDir, binaryPath, checksumsPath, lockPath } from "./cache.js";
import { downloadBinary, downloadChecksums } from "./download.js";
import { parseChecksums, expectedChecksum, verifyChecksum } from "./checksums.js";
import { execute } from "./execute.js";

// Fully anchored regex: exactly X.Y.Z or vX.Y.Z, nothing else.
const STRICT_VERSION_RE = /^v?\d+\.\d+\.\d+$/;

const STALE_LOCK_MS = 5 * 60 * 1000; // 5 minutes

/**
 * Maps a semver version string to a GitHub release tag.
 * Accepts only strict X.Y.Z or vX.Y.Z forms.
 * Development version "0.0.0-development" uses the env override or fails.
 */
export function versionToTag(version) {
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

  if (!STRICT_VERSION_RE.test(version)) {
    throw new Error(
      `Invalid version format: ${version}. Expected X.Y.Z or vX.Y.Z.`
    );
  }

  // Strip leading v if present, then re-add.
  return `v${version.replace(/^v/, "")}`;
}

/**
 * Validates a release tag. Accepts only exact vX.Y.Z form.
 */
export function validateTag(tag) {
  if (!/^v\d+\.\d+\.\d+$/.test(tag)) {
    throw new Error(
      `Invalid release tag format: ${tag}. Expected vX.Y.Z.`
    );
  }
  return tag;
}

/**
 * Acquires an exclusive lock file for a cache entry.
 * Stores PID and creation timestamp. Handles stale locks (5 minutes).
 * Returns a release function.
 */
async function acquireLock(lockFilePath) {
  await mkdir(join(lockFilePath, ".."), { recursive: true });

  for (let attempt = 0; attempt < 60; attempt++) {
    try {
      const fd = await open(lockFilePath, "wx");
      const meta = JSON.stringify({
        pid: process.pid,
        created: Date.now(),
      });
      await fd.writeFile(meta);
      await fd.close();
      return async () => {
        try {
          // Read the lock to verify we own it before releasing.
          const content = await readFile(lockFilePath, "utf-8");
          const data = JSON.parse(content);
          if (data.pid === process.pid) {
            await unlink(lockFilePath);
          }
        } catch {
          try { await unlink(lockFilePath); } catch { /* already gone */ }
        }
      };
    } catch (err) {
      if (err.code === "EEXIST") {
        const isStale = await checkStaleLock(lockFilePath);
        if (isStale) {
          try { await unlink(lockFilePath); } catch { /* race — ok */ }
          continue;
        }
        // Active lock — wait and retry.
        await new Promise((resolve) => setTimeout(resolve, 1000));
      } else {
        throw err;
      }
    }
  }

  throw new Error(`Could not acquire lock after 60 attempts: ${lockFilePath}`);
}

/**
 * Checks if a lock file is stale (malformed, missing PID, or older than 5 minutes).
 */
async function checkStaleLock(lockFilePath) {
  try {
    const content = await readFile(lockFilePath, "utf-8");
    const data = JSON.parse(content);

    const lockPid = parseInt(data.pid, 10);
    if (isNaN(lockPid)) return true; // Malformed — stale.

    // Stale if created more than STALE_LOCK_MS ago.
    const created = parseInt(data.created, 10);
    if (!isNaN(created) && Date.now() - created > STALE_LOCK_MS) {
      return true;
    }

    // Check if the locking process is still alive.
    if (!isProcessAlive(lockPid)) return true;

    return false; // Active, valid lock.
  } catch {
    // Can't read or parse — treat as stale.
    return true;
  }
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
    // Use stat() directly rather than access-then-read (avoids TOCTOU).
    let binaryStat;
    try {
      binaryStat = await stat(binPath);
    } catch {
      binaryStat = null;
    }

    if (binaryStat && binaryStat.isFile()) {
      try {
        const checksumsContent = await readFile(chkPath, "utf-8");
        const checksums = parseChecksums(checksumsContent);
        const expected = expectedChecksum(checksums, assetName);
        const valid = await verifyChecksum(binPath, expected);
        if (valid) {
          return binPath;
        }
        await unlink(binPath);
      } catch {
        // Checksums file missing or corrupt — re-download.
      }
    }

    await downloadChecksums(tag, chkPath);
    const checksumsContent = await readFile(chkPath, "utf-8");
    const checksums = parseChecksums(checksumsContent);

    await downloadBinary(tag, assetName, binPath);

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
