# @fireglyph/glyphdeck — Launcher

Downloads and runs the verified GlyphDeck release binary.

**This package is not yet published to npm.**

## Architecture

The launcher is a thin Node.js wrapper. It does not contain or rebuild GlyphDeck.
On first run, it downloads the matching signed release binary from GitHub, verifies
its SHA-256 checksum, caches it locally, and executes it.

Subsequent runs use the cached binary (re-verified on each execution).

## Supported platforms

| Platform    | Architecture | Asset                          |
| ----------- | ------------ | ------------------------------ |
| Windows     | x64          | glyphdeck-windows-amd64.exe    |
| Linux       | x64          | glyphdeck-linux-amd64          |
| macOS       | x64          | glyphdeck-darwin-amd64         |
| macOS       | arm64        | glyphdeck-darwin-arm64         |

## Usage

```bash
# Run GlyphDeck (downloads on first use):
npx @fireglyph/glyphdeck

# Pass arguments through to GlyphDeck:
npx @fireglyph/glyphdeck --help

# Show launcher version:
npx @fireglyph/glyphdeck --launcher-version

# Show cached binary path:
npx @fireglyph/glyphdeck --launcher-print-path

# Clear cache:
npx @fireglyph/glyphdeck --launcher-clear-cache
```

## Version mapping

The launcher version maps to a GitHub release tag:

- `0.1.2` → downloads from `v0.1.2`
- `0.0.0-development` → requires `GLYPHDECK_LAUNCHER_RELEASE_TAG`

## Security

- Every binary is verified against `checksums.txt` before execution.
- Downloads use HTTPS with redirect validation.
- Partially downloaded or mismatched files are deleted.
- No telemetry. No automatic privilege elevation.

## Cache

| Platform | Default location                          |
| -------- | ----------------------------------------- |
| Windows  | `%LOCALAPPDATA%\GlyphDeck\launcher`       |
| macOS    | `~/Library/Caches/GlyphDeck`              |
| Linux    | `$XDG_CACHE_HOME/glyphdeck` or `~/.cache/glyphdeck` |

Override with `GLYPHDECK_LAUNCHER_CACHE_DIR`.

## License

SEE LICENSE IN LICENSE

Commercial use requires a separate written commercial license from
FireGlyph Studios. See COMMERCIAL-LICENSING.md for details.
