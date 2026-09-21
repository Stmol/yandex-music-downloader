# Architecture

## Runtime flow

`cmd/yamdl` delegates process arguments and standard streams to
`internal/cli`. The CLI selects the terminal UI or a non-interactive command.
The UI and batch command share source resolution and download infrastructure,
while `ya` owns Yandex Music API access, audio publication, metadata, and
models.

```text
cmd/yamdl
    -> internal/cli
       -> ui                 terminal interaction
       -> internal/batch     concurrent download sessions
          -> source          URL and source resolution
          -> ya              API, audio, metadata, models
          -> utils            HTTP, filesystem, token, logging helpers
```

## Boundaries

- `internal/cli` owns user-facing flags, streams, and exit codes.
- `source` converts supported URLs into source requests; it does not download
  bytes or write files.
- `internal/batch` owns scheduling, interruption stages, and session events.
- `ya` owns provider payload compatibility and format-specific artifact rules.
- `utils` contains reusable low-level helpers and must not become a second CLI
  layer.

Playlist export, when present, is a separate CLI command and must not be folded
into the download pipeline. Provider-neutral refactoring is outside the scope
of routine maintenance.

## Artifact safety

Audio downloads are streamed to temporary paths. MP3 and FLAC are published
only after required metadata succeeds. M4A publication keeps a verified audio
file even when optional metadata writing fails. Cancellation and errors remove
incomplete temporary artifacts.
