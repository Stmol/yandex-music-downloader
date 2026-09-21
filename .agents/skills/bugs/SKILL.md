---
name: bugs
description: Use to diagnose or fix YAMDL bugs involving cancellation, races, HTTP context, temporary files, metadata, or provider payload compatibility.
---

# YAMDL Bug Skill

Use this skill when a YAMDL bug report involves broken behavior, a regression,
an intermittent test, cancellation, concurrent state, HTTP requests, file
publication, metadata, or an incompatible provider payload.

## Not for

Do not use this skill for generic debugging workflows, broad refactors, or
release publication. Do not infer live API behavior from a local build.

## Diagnosis checklist

1. Reproduce at the narrowest existing seam before changing production code.
2. For cancellation, follow `context.Context` from the signal through the
   client, scheduler, and response body; assert that blocked work is released.
3. For races, identify the owner of each queue/session field and run the
   focused test with `-race`.
4. For HTTP failures, use an injectable transport or `httptest` and verify
   status, body sanitization, and context propagation.
5. For artifacts, assert temporary-file cleanup, atomic rename behavior, and
   the format-specific metadata contract.
6. For provider payloads, preserve compatibility in `ya/model` and add the
   smallest fixture that demonstrates the actual shape.

## Verification

Add only meaningful regression coverage, then run the focused test, its race
variant when relevant, and `bash scripts/verify.sh` before handoff.
