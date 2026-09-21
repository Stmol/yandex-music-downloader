# Contributing to YAMDL

## Before opening a change

1. Keep unrelated user work out of the change and work on a feature branch.
2. Read `AGENTS.md` and identify the package that owns the behavior.
3. Run `bash scripts/verify.sh` before opening a pull request.
4. Add focused regression coverage for behavior changes. Do not add live API
   tests or require a personal Yandex Music token.

## Pull requests

Describe the user-visible behavior, the verification commands that passed, and
any checks that were intentionally not run. Keep changes scoped: playlist
export, download behavior, provider integrations, and release-process changes
should be reviewable as separate concerns.

## Security

Never include tokens, authorization headers, downloaded private media, or
machine-specific paths in commits, issues, fixtures, or logs. See `SECURITY.md`
for vulnerability reporting.
