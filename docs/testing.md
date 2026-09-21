# Testing and Verification

## Local gate

Run the single repository entrypoint:

```bash
bash scripts/verify.sh
```

It checks formatting, static analysis, tests, race behavior, module integrity,
the CLI build, shell syntax, project assets, and whitespace errors. The gate is
deterministic and must not call Yandex Music or require a token.

## Test layers

- Pure parsing, model, metadata, and filesystem behavior uses focused unit
  tests.
- HTTP behavior uses `httptest` or injectable transports and verifies context
  cancellation and sanitized diagnostics.
- Concurrent queue and UI session changes must be covered by race-enabled
  tests.
- Audio artifact tests verify temporary-file cleanup, atomic publication, and
  format-specific metadata rules.
- CLI tests assert stdout, stderr, and exit-code contracts without starting a
  real provider request.

## Evidence boundaries

Local tests and CI prove deterministic repository behavior. They do not prove
that a personal token works or that the live Yandex Music API accepts a given
request. Live API smoke tests are a separately named, manually authorized
check and are never hidden inside the default verifier.
