#!/usr/bin/env bash

set -euo pipefail

APP_NAME="yamdl"
OUTPUT_DIR="build"
VERSION="${1:-${YAMDL_VERSION:-$(git describe --tags --always)}}"

if [[ -z "$VERSION" ]]; then
  printf 'Release version must not be empty.\n' >&2
  exit 2
fi

rm -rf -- "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

platforms=(
  'linux amd64'
  'linux arm64'
  'windows amd64'
  'windows arm64'
  'darwin amd64'
  'darwin arm64'
)

for platform in "${platforms[@]}"; do
  read -r goos goarch <<< "$platform"
  binary_path="$OUTPUT_DIR/$APP_NAME"
  if [[ "$goos" == windows ]]; then
    binary_path+='.exe'
  fi

  printf 'Building %s/%s (%s)\n' "$goos" "$goarch" "$VERSION"
  GOOS="$goos" GOARCH="$goarch" go build \
    -trimpath \
    -ldflags "-s -w -X ya-music/internal/version.Version=$VERSION" \
    -o "$binary_path" \
    ./cmd/yamdl

  archive_path="$OUTPUT_DIR/${APP_NAME}_${VERSION}_${goos}_${goarch}.zip"
  zip -j -q "$archive_path" "$binary_path"
  rm "$binary_path"
done

(
  cd "$OUTPUT_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.zip > SHA256SUMS
  else
    shasum -a 256 ./*.zip > SHA256SUMS
  fi
)

printf 'Created %s archives and SHA256SUMS in %s\n' "${#platforms[@]}" "$OUTPUT_DIR"
