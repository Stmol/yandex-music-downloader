# Changelog

## v1.2 - 2026-04-06
- introduced structured download logging written to `dl_logs.txt`, including session/track/request metadata with sanitized URLs
- made log cancellation-aware by cancelling downloads, atomically writing temp files, and guarding quit buttons while a session is stopping
- centralized logger access, skip reasons, and cleanup helpers plus added tests for shutdown flow and temp-file cleanup

## v1.1 - 2026-04-06
- added a “Back to URL” button and navigation guard so downloads can return to the source screen even while a download is running
- reset the download UI state (progress, focus, filters, etc.) when jumping back so the screen always starts fresh
- added source-screen reset helpers plus tests that cover focus cycling and reset behavior
