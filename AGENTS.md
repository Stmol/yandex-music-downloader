# YAMDL Agent and Contributor Contract

YAMDL is a Go command-line and terminal UI application for downloading media
from Yandex Music. Keep changes small, preserve existing user-visible behavior,
and do not require a live Yandex Music account for local or CI verification.

## Package map and ownership

- `cmd/yamdl`: the executable entry point. Keep process setup here.
- `internal/cli`: command dispatch, flag parsing, exit codes, and non-interactive
  command behavior. CLI output contracts belong here.
- `internal/batch`: concurrent download orchestration and cancellation state.
- `source`: URL parsing and source resolution for tracks, albums, playlists, and
  charts.
- `ui`: Bubble Tea terminal screens and user interaction.
- `ya`: Yandex Music client, audio artifacts, metadata, and API models.
- `utils`: shared filesystem, HTTP, logging, and token helpers.

Do not move behavior between these packages as part of a quality or release
change. A provider-neutral abstraction or a Spotify implementation requires a
separate design decision.

## CLI contracts

- `yamdl` starts the TUI after validating root flags.
- `yamdl download` is non-interactive and writes progress and summaries to
  stdout; argument errors and diagnostics go to stderr.
- `yamdl --version` writes only the build version to stdout and does not start
  the TUI or contact Yandex Music.
- Successful help/version commands return exit code `0`; invalid arguments
  return `2`; a user interruption returns `130`.
- Never print an OAuth token, authorization header, or full credential-bearing
  URL. Use placeholders such as `TOKEN` in documentation and fixtures.
- A future or parallel `yamdl export` command must remain separate from
  `download`; it must not change download dispatch or output semantics.

## Safety and correctness rules

- Thread `context.Context` through HTTP and download operations. Cancellation
  must stop scheduling new work and must not leave partial audio artifacts.
- Preserve race-safe ownership of queue/session state. Run race tests when
  touching concurrent code.
- Write downloaded files through temporary artifacts and publish only complete
  files. Clean temporary files on success, cancellation, and failure.
- Treat metadata rules as format-specific: MP3/FLAC tagging is required before
  publication, while M4A metadata is best effort after a verified audio file.
- Use `httptest` or injectable transports for network behavior. Do not add
  tests that call Yandex Music or require a personal token.
- TUI changes require focused model/rendering tests and a manual terminal
  review when output layout or keyboard behavior changes.

## Repository agent skills

Read and follow the applicable repository skill before starting its work.
These tracked instructions are available to every agent; if the agent cannot
invoke skills by name, open the linked `SKILL.md` and follow it directly.

- [yamdl](.agents/skills/yamdl/SKILL.md): use before changing YAMDL code,
  tests, CLI/TUI behavior, provider payload handling, or audio artifacts, and
  for project-specific verification.
- [bugs](.agents/skills/bugs/SKILL.md): use before diagnosing or fixing YAMDL
  regressions, intermittent tests, cancellation, races, HTTP behavior,
  temporary files, metadata, or provider payload compatibility. For code
  fixes, also follow `yamdl`.
- [release-flow](.agents/skills/release-flow/SKILL.md): use when preparing or
  reviewing release artifacts or the release workflow, and complete its
  preflight before creating or pushing a release tag. This skill does not
  authorize publication; creating or pushing tags requires explicit release
  authorization.

Before handing off code changes or opening a PR, review the diff against this
contract and the applicable skills, then run `bash scripts/verify.sh`. Before
merging a PR, confirm the required CI checks passed for the current PR revision.
Report which skills were applied, the verification commands and results, and
any skipped checks or blockers. Reading a skill alone is not verification.
CI runs the scripts and checks file presence; it does not execute agent skills
or perform an agent review.

## Verification levels

Local, deterministic checks are the source of truth for repository changes:

```bash
bash scripts/verify.sh
```

CI repeats the verifier, security scanning, and cross-platform compilation.
Release jobs additionally validate the version, changelog, archives, checksums,
and a runnable Linux CLI smoke test. A live API smoke test is separate evidence;
green local or CI checks never imply that live credentials or provider access
were tested.

## Files and secrets

Do not commit `token.txt`, downloads, build output, logs, local planning files,
or machine-specific paths. Keep public documentation in tracked `docs/*.md`
files and local/generated material in ignored subdirectories. Preserve
unrelated staged, unstaged, and untracked user work when operating on the repo.
