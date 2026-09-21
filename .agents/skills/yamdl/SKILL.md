---
name: yamdl
description: Use for YAMDL feature work, CLI behavior, TUI behavior, provider payloads, audio artifacts, or project-specific verification.
---

# YAMDL Project Skill

Use this skill when a task changes YAMDL code, tests, CLI output, terminal UI
behavior, Yandex Music payload handling, or downloaded audio artifacts.

## Not for

Do not use this skill for generic code review, generic debugging, or release
publication without the release-specific skill. Do not use it to design a
provider-neutral or Spotify abstraction.

## Rules

- Read the root `AGENTS.md` before editing.
- Keep `download` and `export` as separate CLI commands.
- Preserve stdout, stderr, and exit-code contracts. Do not print tokens or
  authorization headers.
- Use injectable HTTP transports or `httptest`; never require live Yandex Music
  credentials in the default test suite.
- Trace context cancellation through preflight, queue scheduling, HTTP requests,
  and temporary-file cleanup. Run race tests for concurrent state changes.
- Verify format-specific artifact rules: MP3/FLAC metadata is required before
  publication; M4A metadata is best effort after audio verification.
- Prefer focused tests at the existing package seam over broad rewrites.

## Verification

Run `bash scripts/verify.sh`. A passing local verifier proves deterministic
repository behavior only; it does not prove live provider access.
