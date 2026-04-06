# Changelog

## v1.3 - 2026-04-06
- added optional `--timeout <seconds>` support so a single file download can be limited without affecting regular API requests
- expanded source URL parsing to accept Yandex Music domains beyond `.ru` and added support for playlist links by UUID
- made model ID decoding tolerant to both numeric and string values, improving compatibility with newer playlist and track payloads
- kept track status columns aligned for long playlists and added tests covering URL parsing, timeout handling, flexible IDs, and list rendering

## v1.2 - 2026-04-06
- introduced structured download logging written to `dl_logs.txt`, including session/track/request metadata with sanitized URLs
- made log cancellation-aware by cancelling downloads, atomically writing temp files, and guarding quit buttons while a session is stopping
- centralized logger access, skip reasons, and cleanup helpers plus added tests for shutdown flow and temp-file cleanup

## v1.1 - 2026-04-06
- added a “Back to URL” button and navigation guard so downloads can return to the source screen even while a download is running
- reset the download UI state (progress, focus, filters, etc.) when jumping back so the screen always starts fresh
- added source-screen reset helpers plus tests that cover focus cycling and reset behavior
