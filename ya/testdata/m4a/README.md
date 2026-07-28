# M4A Test Fixtures

These binary fixtures are checked in so CI never needs `ffmpeg` to exercise M4A behavior.

## `taggable-stco.m4a`

- Source command:

```sh
/opt/homebrew/bin/ffmpeg -y -f lavfi -i sine=frequency=440:duration=1 -c:a aac -b:a 128k -map_metadata -1 -vn ya/testdata/m4a/taggable-stco.m4a
```

- Intended atom: `stco`
- SHA-256: `492a285f0539daa3814f76db16e85a5dc9957248442dd0ac56337c85d3eaeddc`
- Audio: yes, one AAC-LC mono audio stream
- Property under test: writable baseline M4A fixture with `stco` chunk offsets and no pre-existing title, artist, cover, or Yandex source-url tag

## `taggable-co64.m4a`

- Source command:

```sh
go run ./ya/testdata/m4a/generate co64 ya/testdata/m4a/taggable-stco.m4a ya/testdata/m4a/taggable-co64.m4a
```

- Intended atom: `co64`
- SHA-256: `a1117201d759c660fcd3124616bd66d24706408f02bb04f3ab7a8074fed3a47b`
- Audio: yes, one AAC-LC mono audio stream
- Property under test: readable/taggable M4A fixture with 64-bit chunk offsets preserved as `co64`

## `malformed-truncated-track.m4a`

- Source command:

```sh
/opt/homebrew/bin/ffmpeg -y -f lavfi -i sine=frequency=440:duration=1 -c:a aac -b:a 128k -map_metadata -1 -metadata track=1/9 -vn /tmp/source-with-track.m4a
go run ./ya/testdata/m4a/generate truncate-trkn /tmp/source-with-track.m4a ya/testdata/m4a/malformed-truncated-track.m4a
```

- Intended atom: truncated `trkn`
- SHA-256: `66c48eafd91c768fa511b2a551ad0e8a32307f1719da010c746d1436cd15f975`
- Audio: no, the committed fixture is intentionally malformed and truncated from a valid one-audio-stream source file
- Property under test: parser failure path must reject a realistic truncated MP4 metadata atom without rewriting or mutating the file
