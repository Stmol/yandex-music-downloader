---
name: release-flow
description: Use for YAMDL release preflight, version/tag/changelog checks, cross-builds, archives, checksums, and release gates.
---

# YAMDL Release Flow Skill

Use this skill when preparing or reviewing a YAMDL release artifact or release
workflow. It covers preflight and validation; it does not publish a release by
itself.

## Not for

Do not use this skill for ordinary feature work, generic CI advice, live API
smoke tests, or provider architecture changes. Do not create or push tags
without explicit release authorization.

## Preflight

1. Confirm the working tree, branch, and intended `vX.Y.Z` tag.
2. Require a matching `CHANGELOG.md` section.
3. Run `bash scripts/verify.sh` without credentials.
4. Build all six targets with `bash scripts/build.sh "$VERSION"`.
5. Validate ZIP members, reject stale or extra artifacts, and verify
   `SHA256SUMS`.
6. Smoke-test only a runnable Linux binary with `--help`, `--version`,
   `download --help`, and `export --help` when that command exists.

## Version contract

The release tag, linker-injected binary version, archive names, changelog
heading, and checksum manifest must agree. A mismatch blocks publication.
