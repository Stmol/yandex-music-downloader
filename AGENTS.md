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
