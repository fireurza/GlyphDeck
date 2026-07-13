// Cache directory management for downloaded release binaries and checksums.

import { homedir } from "node:os";
import { join } from "node:path";

/**
 * Returns the default cache directory for the current platform.
 * Override with GLYPHDECK_LAUNCHER_CACHE_DIR for testing.
 */
export function cacheDir() {
  if (process.env.GLYPHDECK_LAUNCHER_CACHE_DIR) {
    return process.env.GLYPHDECK_LAUNCHER_CACHE_DIR;
  }

  switch (process.platform) {
    case "win32":
      return join(process.env.LOCALAPPDATA || join(homedir(), "AppData", "Local"), "GlyphDeck", "launcher");
    case "darwin":
      return join(homedir(), "Library", "Caches", "GlyphDeck");
    default:
      // Linux and others: XDG_CACHE_HOME or ~/.cache
      return join(process.env.XDG_CACHE_HOME || join(homedir(), ".cache"), "glyphdeck");
  }
}

/**
 * Returns the binary cache path for a given version and asset name.
 */
export function binaryPath(cacheRoot, version, assetName) {
  return join(cacheRoot, version, assetName);
}

/**
 * Returns the checksums cache path for a given version.
 */
export function checksumsPath(cacheRoot, version) {
  return join(cacheRoot, version, "checksums.txt");
}

/**
 * Returns the lock file path for a given cache entry.
 */
export function lockPath(filePath) {
  return filePath + ".lock";
}
