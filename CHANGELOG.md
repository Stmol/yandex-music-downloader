# Changelog

## v1.13 - 2026-08-19
- added a non-interactive `yamdl download` command for downloading a track, album, playlist, or chart without opening the terminal UI
- added required `--token` and `--link` options plus `--format`, `--output`, `--timeout`, and `--skip-cover` controls for batch downloads; MP3 and `./downloads` remain the defaults
- report each batch track's position and outcome, skip unavailable or duplicate entries, and print a downloaded/skipped/failed summary with the output directory
- validate batch arguments, authentication, source resolution, and output paths before starting downloads; Ctrl+C and SIGTERM cancel the client, print the partial summary, and return exit code 130

## v1.12 - 2026-08-13
- refactored the download queue around explicit per-session events, so progress counters stay scoped to the current run, unknown track IDs cannot corrupt state, and restarting `Download all` requeues eligible tracks consistently
- unified MP3, FLAC, and M4A publication through a temporary artifact pipeline that streams MP3 responses into staging files, writes metadata before the final rename, publishes complete files atomically, and removes incomplete artifacts after failures
- kept format-specific metadata rules in one publication path: MP3 and FLAC tags remain required before delivery, while M4A tagging remains best-effort without blocking a verified audio file
- improved download diagnostics with streamed byte counts, sanitized response previews for text errors, injectable transports for HTTP tests, and clearer file-write failure stages
- redesigned the terminal download screen with a unified action bar, clearer focus styling, responsive list sizing, width-safe status/title rendering, and expanded inline command help
- added direct download-screen shortcuts for MP3/FLAC selection (`1`/`2`), `Download all` (`D`), back (`b`), quit or cancel (`q`), and expanded help (`?`), while preventing list `Enter` from activating an action
- expanded regression coverage for session lifecycle, artifact publication and cleanup, streamed HTTP downloads, layout constraints, keyboard navigation, and concurrent state updates

## v1.11 - 2026-07-28
- added M4A metadata writing for lossless `flac-mp4` downloads: title, artists, album, album artist, genre, year, track/disc numbers, Yandex Music source URL, and embedded JPEG/PNG cover art when available
- create the required M4A metadata atom tree when a valid MP4/M4A has no initial `ilst`, so lossless Yandex Music files receive tags and cover art
- made M4A tagging non-fatal: if metadata writing fails, the verified audio file is still delivered and a warning is logged instead of deleting or blocking the download
- covered both `stco` and `co64` sample-offset atoms in the M4A tagging path without requiring ffmpeg or AtomicParsley at runtime
- preserved unrelated MP3 comment frames while writing an application-owned Yandex Music source URL comment

## v1.10 - 2026-07-28
- migrated the terminal UI to Charm v2 (`Bubble Tea`, `Bubbles`, and `Lip Gloss`) and updated the application to the new declarative view and keyboard-event APIs
- made token input, URL input, the track list, progress bar, and action controls adapt to the current terminal width and height; wide Unicode track titles now keep the status column aligned
- improved download-screen navigation: arrow keys and `H`/`J`/`K`/`L` move between the format and action controls, while `Esc` returns focus to the track list
- added a startup preflight check for `TERM=xterm`, which explains how to switch to `xterm-256color` before starting the app so colors, focus, and selected rows render correctly

## v1.9 - 2026-05-24
- added support for Yandex Music chart URLs, including the default regional chart at `/chart` and region-specific charts such as `/chart/world`

## v1.8 - 2026-05-09
- improved compatibility with newer Yandex Music lossless responses so lossless downloads work again with the current API and can be saved in the returned container format
- improved download cancellation in the TUI: stopping an active queue now resets interrupted tracks back to a ready state instead of leaving them as failed downloads

## v1.7 - 2026-05-01
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
