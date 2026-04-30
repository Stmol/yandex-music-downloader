# Changelog

## v1.7 - Unreleased
- added a lossless FLAC download mode alongside the existing MP3 flow; MP3 remains the default format
- added fallback-friendly download behavior: when FLAC is unavailable, invalid, fails to download, or cannot be tagged, the app automatically retries the same track as the best available MP3
- added FLAC metadata writing with Vorbis comments and embedded cover artwork, including title, artists, album, album artist, genre, date, track/disc numbers, Yandex track ID, and a Yandex Music track URL comment
- added a download format selector in the TUI action group so users can choose MP3 or FLAC for the whole current queue before starting downloads
- redesigned the download screen controls into a focused action group with consistent keyboard behavior: `Tab` switches between the track list and controls, arrow keys move inside the controls, and `Enter`/`Space` activates the selected control
- made the download screen layout responsive to terminal height by resizing the track list from `tea.WindowSizeMsg`, keeping the header, progress bar, controls, and hotkey help visible
- updated downloaded-track status rendering to show the actual saved format: `✅ FLAC` for FLAC downloads and `✅ MP3` for MP3 downloads or MP3 fallbacks

## v1.6 - 2026-04-24
- added support for Yandex Music album page URLs: paste an `/album/<id>` link to fetch the album and download all tracks in order

## v1.5 - 2026-04-13
- added support for new Yandex Music playlist links with prefixed UUIDs such as `lk.` and `ps.`

## v1.4 - 2026-04-09
- added ID3 metadata writing for downloaded MP3 files, including title, artists, album, year, genre, track number, and Yandex track ID where available
- added cover downloading and embedding as MP3 front-cover artwork, with non-fatal cover failures and best-effort temporary cover cleanup
- added `--skip-cover=true` so users can avoid cover traffic while still writing text ID3 tags
- changed downloaded filenames to the canonical `Artist - Track Name.mp3` pattern

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
