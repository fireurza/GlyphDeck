// Downloads release assets from GitHub with checksum verification.

import { createHash } from "node:crypto";
import { createWriteStream } from "node:fs";
import { mkdir, rename, stat, unlink } from "node:fs/promises";
import { dirname } from "node:path";
import { pipeline } from "node:stream/promises";
import { get } from "node:https";

const GITHUB_RELEASE_HOST = "github.com";
const GITHUB_API_HOST = "api.github.com";
const RELEASE_REPO = "fireurza/GlyphDeck";

const TIMEOUT_MS = 60_000; // 60 seconds overall timeout
const MAX_SIZE_BYTES = 100 * 1024 * 1024; // 100 MiB max binary size
const MAX_CHECKSUMS_SIZE = 64 * 1024; // 64 KiB for checksums file

/**
 * Returns the GitHub release download URL for a given tag and asset name.
 */
export function releaseAssetUrl(tag, assetName) {
  // Use direct download URL: https://github.com/<owner>/<repo>/releases/download/<tag>/<asset>
  return `https://${GITHUB_RELEASE_HOST}/${RELEASE_REPO}/releases/download/${encodeURIComponent(tag)}/${encodeURIComponent(assetName)}`;
}

/**
 * Downloads a file from a URL to a local path with size limit and timeout.
 * Returns the SHA-256 hash of the downloaded content.
 */
export async function downloadToFile(url, destPath, maxSize = MAX_SIZE_BYTES) {
  await mkdir(dirname(destPath), { recursive: true });

  const tmpPath = destPath + ".tmp";
  const hash = createHash("sha256");

  try {
    await new Promise((resolve, reject) => {
      const controller = new AbortController();
      const timeout = setTimeout(() => {
        controller.abort();
        reject(new Error(`Download timed out after ${TIMEOUT_MS / 1000}s: ${url}`));
      }, TIMEOUT_MS);

      const req = get(url, { signal: controller.signal }, (res) => {
        // Validate redirect target host.
        const { statusCode, headers } = res;

        if (statusCode >= 300 && statusCode < 400 && headers.location) {
          clearTimeout(timeout);
          const redirectUrl = new URL(headers.location, url);

          // Only follow redirects to github.com or objects.githubusercontent.com.
          if (!isAllowedHost(redirectUrl.hostname)) {
            reject(new Error(`Redirect to untrusted host: ${redirectUrl.hostname}`));
            return;
          }

          // Follow the redirect.
          downloadToFile(redirectUrl.href, destPath, maxSize)
            .then((h) => resolve(h))
            .catch(reject);
          return;
        }

        if (statusCode !== 200) {
          clearTimeout(timeout);
          reject(new Error(`Download failed with status ${statusCode}: ${url}`));
          return;
        }

        const contentLength = parseInt(headers["content-length"], 10);
        if (contentLength > maxSize) {
          clearTimeout(timeout);
          reject(new Error(`Content too large: ${contentLength} bytes (max ${maxSize})`));
          return;
        }

        let downloaded = 0;
        const fileStream = createWriteStream(tmpPath);

        res.on("data", (chunk) => {
          downloaded += chunk.length;
          if (downloaded > maxSize) {
            fileStream.destroy();
            clearTimeout(timeout);
            reject(new Error(`Download exceeded maximum size of ${maxSize} bytes`));
            return;
          }
          hash.update(chunk);
        });

        pipeline(res, fileStream)
          .then(() => {
            clearTimeout(timeout);
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

    // Atomic rename after successful download.
    await rename(tmpPath, destPath);
    return hash.digest("hex");
  } catch (err) {
    // Clean up partial download.
    try { await unlink(tmpPath); } catch { /* ignore */ }
    throw err;
  }
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
 * Returns the URL for a given release tag and asset.
 */
function isAllowedHost(hostname) {
  return hostname === GITHUB_RELEASE_HOST || hostname.endsWith(".github.com") || hostname.endsWith(".githubusercontent.com");
}

// Re-export for testing.
export { isAllowedHost };
