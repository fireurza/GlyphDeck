// Maps Node.js process.platform and process.arch to GlyphDeck release asset names.

const ASSET_MAP = {
  "win32-x64": "glyphdeck-windows-amd64.exe",
  "linux-x64": "glyphdeck-linux-amd64",
  "darwin-x64": "glyphdeck-darwin-amd64",
  "darwin-arm64": "glyphdeck-darwin-arm64",
};

/**
 * Returns the release asset name for the current platform, or null if unsupported.
 */
export function currentAssetName() {
  const key = `${process.platform}-${process.arch}`;
  return ASSET_MAP[key] || null;
}

/**
 * Returns whether the current platform is supported.
 */
export function isSupported() {
  return currentAssetName() !== null;
}

/**
 * Returns the list of all supported platform keys.
 */
export function supportedPlatforms() {
  return Object.keys(ASSET_MAP);
}
