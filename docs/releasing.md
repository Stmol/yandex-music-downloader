# Release Process

Releases are tag-driven and must pass the release workflow before publication.

## Preflight

1. Update `CHANGELOG.md` with a section matching the exact `vX.Y.Z` tag.
2. Run `bash scripts/verify.sh`.
3. Validate the tag, changelog, version string, cross-platform builds, ZIP
   contents, and `SHA256SUMS` with the release validator.
4. Smoke-test the runnable Linux binary with `--help`, `--version`,
   `download --help`, and `export --help` when export is present.

## Version contract

The release tag is the source of the release version. Build metadata injects it
into the binary, and every archive name and checksum entry must contain the
same version. Development builds may use a Git-derived fallback, but a release
must fail closed when the tag and injected version disagree.

## Publication

Only the final publish job receives `contents: write`. Verification, security,
cross-build, archive, and checksum jobs must complete successfully first.
The workflow publishes ZIP archives and `SHA256SUMS`; live API access and
package-manager distribution are separate concerns.
