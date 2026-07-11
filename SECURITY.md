# Security Policy

## Supported Versions

| Version | Supported |
|---------|----------|
| v0.1.x  | ✅       |

## Reporting a Vulnerability

Please report security vulnerabilities privately to the repository maintainers
via GitHub's [Security Advisories](https://github.com/fireurza/GlyphDeck/security/advisories/new).

Do not open a public issue for security vulnerabilities.

## Security Model

GlyphDeck v0.1.1 is designed for **local/private-network use only**.

- Binds to `127.0.0.1` (loopback) by default.
- Mutating API requests require same-origin `Origin` and loopback host.
- Admin authentication is required for all API access.
- Sessions use HttpOnly cookies with SameSite=Lax.
- No public network exposure is intended or recommended.

**GlyphDeck should never be exposed to the public internet without additional**
**authentication and transport security layers.**
