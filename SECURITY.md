# Security Policy

## Supported versions

Security fixes are applied to the latest release and the default development
branch. Older releases may not receive fixes.

## Reporting a vulnerability

Please do not open a public issue for a suspected vulnerability. Contact the
repository maintainers privately through the security contact configured for
the hosting service, or use a private security advisory when available.

Include the affected version or commit, a minimal reproduction, impact, and
any proposed mitigation. Do not include real Yandex Music tokens or private
media in the report.

## Local secret handling

OAuth tokens belong in local `token.txt` or an equivalent secret store and must
not be committed. The repository runs Gitleaks in CI; confirmed deterministic
fixtures or public protocol constants that resemble credentials require a
narrow documented allowlist entry.
