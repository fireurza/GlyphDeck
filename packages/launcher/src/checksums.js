// Parses and verifies checksums.txt against a local binary.

import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { pipeline } from "node:stream/promises";

/**
 * Parses a checksums.txt file content and returns a Map of filename -> sha256 hash.
 */
export function parseChecksums(content) {
  const map = new Map();
  for (const line of content.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;

    // Format: <sha256>  <filename>
    const match = trimmed.match(/^([a-f0-9]{64})\s+(\S+)$/i);
    if (!match) continue;

    map.set(match[2], match[1].toLowerCase());
  }
  return map;
}

/**
 * Looks up the expected checksum for a given asset name in a checksums map.
 * Throws if the asset is not found.
 */
export function expectedChecksum(checksums, assetName) {
  const hash = checksums.get(assetName);
  if (!hash) {
    throw new Error(`Asset '${assetName}' not found in checksums.txt`);
  }
  return hash;
}

/**
 * Verifies a local file against an expected SHA-256 checksum.
 * Returns true if the hash matches, false otherwise.
 */
export async function verifyChecksum(filePath, expected) {
  const hash = createHash("sha256");
  const stream = createReadStream(filePath);
  await pipeline(stream, hash);
  const actual = hash.digest("hex");
  return actual === expected.toLowerCase();
}
