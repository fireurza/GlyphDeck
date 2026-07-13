// Downloads release assets from GitHub with checksum verification.

import { createHash } from "node:crypto";
import { createWriteStream } from "node:fs";
import { mkdir, rename, unlink } from "node:fs/promises";
import { dirname } from "node:path";
import { pipeline } from "node:stream/promises";
import { get } from "node:https";

const GITHUB_RELEASE_HOST = "github.com";
const RELEASE_REPO = "fireurza/GlyphDeck";

const TIMEOUT_MS = 60_000; // 60 seconds overall timeout
const MAX_SIZE_BYTES = 100 * 1024 * 1024; // 100 MiB max binary size
const MAX_CHECKSUMS_SIZE = 64 * 1024; // 64 KiB for checksums file
const MAX_REDIRECTS = 5;

/**
 * Returns the GitHub release download URL for a given tag and asset name.
 */
export function releaseAssetUrl(tag, assetName) {
  return `https://${GITHUB_RELEASE_HOST}/${RELEASE_REPO}/releases/download/${encodeURIComponent(tag)}/${encodeURIComponent(assetName)}`;
}

/**
 * Downloads a file from a URL to a local path with size limit, timeout,
 * redirect following, and SHA-256 hashing.
 *
 * The hash is finalized exactly once after the complete download finishes.
 * Redirects are followed (up to MAX_REDIRECTS). Only HTTPS is accepted.
 * A single temporary file is used; it is atomically renamed only after
 * the final response completes.
 */
export async function downloadToFile(url, destPath, maxSize = MAX_SIZE_BYTES) {
  await mkdir(dirname(destPath), { recursive: true });

  const tmpPath = destPath + ".tmp";

  try {
    // Follow redirects and perform the actual download into tmpPath.
    const hexHash = await followAndDownload(url, tmpPath, maxSize, 0);

    // Safely replace destination on Windows: remove existing, then rename.
    try { await unlink(destPath); } catch { /* doesn't exist */ }
    await rename(tmpPath, destPath);
    return hexHash;
  } catch (err) {
    // Clean up partial download.
    try { await unlink(tmpPath); } catch { /* ignore */ }
    throw err;
  }
}

/**
 * Follows redirects and downloads the final response body to tmpPath.
 * Returns the SHA-256 hex digest of the downloaded content.
 * The rename to the final path is handled by the caller (downloadToFile).
 */
async function followAndDownload(url, tmpPath, maxSize, redirectCount) {
  if (redirectCount > MAX_REDIRECTS) {
    throw new Error(`Too many redirects (max ${MAX_REDIRECTS})`);
  }

  const urlObj = new URL(url);
  if (urlObj.protocol !== "https:") {
    throw new Error(`Only HTTPS is supported, got: ${urlObj.protocol}`);
  }

  return new Promise((resolve, reject) => {
    const controller = new AbortController();
    const timeout = setTimeout(() => {
      controller.abort();
      reject(new Error(`Download timed out after ${TIMEOUT_MS / 1000}s: ${url}`));
    }, TIMEOUT_MS);

    const req = get(url, { signal: controller.signal }, (res) => {
      const { statusCode, headers } = res;

      if (statusCode >= 300 && statusCode < 400 && headers.location) {
        clearTimeout(timeout);

        const redirectUrl = new URL(headers.location, url);

        if (redirectUrl.protocol !== "https:") {
          reject(new Error(`Redirect must use HTTPS, got: ${redirectUrl.protocol}`));
          return;
        }

        if (!isAllowedHost(redirectUrl.hostname)) {
          reject(new Error(`Redirect to untrusted host: ${redirectUrl.hostname}`));
          return;
        }

        // Consume the redirect response body.
        res.resume();

        // Follow the redirect.
        followAndDownload(redirectUrl.href, tmpPath, maxSize, redirectCount + 1)
          .then(resolve)
          .catch(reject);
        return;
      }

      if (statusCode !== 200) {
        clearTimeout(timeout);
        res.resume();
        reject(new Error(`Download failed with status ${statusCode}: ${url}`));
        return;
      }

      const contentLength = parseInt(headers["content-length"], 10);
      if (!isNaN(contentLength) && contentLength > maxSize) {
        clearTimeout(timeout);
        res.destroy();
        reject(new Error(`Content too large: ${contentLength} bytes (max ${maxSize})`));
        return;
      }

      let downloaded = 0;
      const hash = createHash("sha256");
      const fileStream = createWriteStream(tmpPath);

      res.on("data", (chunk) => {
        downloaded += chunk.length;
        if (downloaded > maxSize) {
          fileStream.destroy();
          res.destroy();
          clearTimeout(timeout);
          reject(new Error(`Download exceeded maximum size of ${maxSize} bytes`));
          return;
        }
        hash.update(chunk);
      });

      pipeline(res, fileStream)
        .then(() => {
          clearTimeout(timeout);
          // Finalize hash exactly once.
          resolve(hash.digest("hex"));
        })
        .catch((err) => {
          clearTimeout(timeout);
          reject(err);
        });
    });

    req.on("error", (err) => {
      clearTimeout(timeout);
      reject(new Error(`Download request failed: ${err.message}`));
    });
  });
}

/**
 * Downloads the checksums.txt file for a release tag.
 */
export async function downloadChecksums(tag, destPath) {
  const url = releaseAssetUrl(tag, "checksums.txt");
  await downloadToFile(url, destPath, MAX_CHECKSUMS_SIZE);
}

/**
 * Downloads a release binary for a tag and asset name.
 */
export async function downloadBinary(tag, assetName, destPath) {
  const url = releaseAssetUrl(tag, assetName);
  await downloadToFile(url, destPath, MAX_SIZE_BYTES);
}

/**
 * Returns whether a hostname is allowed for GitHub release downloads.
 */
export function isAllowedHost(hostname) {
  return (
    hostname === GITHUB_RELEASE_HOST ||
    hostname.endsWith(".github.com") ||
    hostname.endsWith(".githubusercontent.com")
  );
}
