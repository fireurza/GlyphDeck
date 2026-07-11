# Contributing

Thanks for considering contributing to GlyphDeck!

## Current Status

External code contributions are temporarily not accepted until contributor terms
that preserve FireGlyph Studios' commercial relicensing rights are established.

We continue to accept:
- Bug reports
- Security vulnerability reports (via [SECURITY.md](SECURITY.md))
- Feature requests
- Documentation feedback

## Getting Started

See [docs/development/LOCAL_DEVELOPMENT.md](docs/development/LOCAL_DEVELOPMENT.md) for
local development setup instructions.

## Development Workflow

1. Fork the repository.
2. Create a feature branch from `master`.
3. Make your changes.
4. Run the full validation suite:
   ```powershell
   go test ./... -count=1
   go vet ./cmd/... ./internal/... ./web/...
   cd web && npm test && npm run build
   .\scripts\build.ps1
   .\scripts\validation\run-mvp-smoke.ps1
   ```
5. Ensure no visible host windows open during validation.
6. Open a pull request.

## Pull Request Checklist

- [ ] All tests pass locally.
- [ ] Go vet is clean.
- [ ] MVP smoke passes (17/17).
- [ ] No visible host windows during validation.
- [ ] No secrets, DBs, logs, or build artifacts are staged.
- [ ] Documentation updated if behavior changed.

## Code Style

- Go: Follow standard Go conventions. Run `go fmt` before committing.
- TypeScript/React: The repository uses Prettier. Run `npx prettier --write .` in `web/`.

## License

GlyphDeck is source-available under the PolyForm Noncommercial License 1.0.0.
Commercial use requires a separate written license from FireGlyph Studios.
See [LICENSE](../LICENSE) and [COMMERCIAL-LICENSING.md](../COMMERCIAL-LICENSING.md).
