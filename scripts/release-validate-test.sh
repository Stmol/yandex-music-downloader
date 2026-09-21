#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
validator="$script_dir/release-validate.sh"
temp_root=$(mktemp -d "${TMPDIR:-/tmp}/yamdl-release-test.XXXXXX")
trap 'rm -rf "$temp_root"' EXIT

version='v9.9.9'
output_dir="$temp_root/build"
changelog="$temp_root/CHANGELOG.md"
mkdir -p "$output_dir"
printf '# Changelog\n\n## %s - 2026-09-19\n\nTest release.\n' "$version" > "$changelog"

targets=(
  'linux_amd64:yamdl'
  'linux_arm64:yamdl'
  'windows_amd64:yamdl.exe'
  'windows_arm64:yamdl.exe'
  'darwin_amd64:yamdl'
  'darwin_arm64:yamdl'
)

for target in "${targets[@]}"; do
  IFS=: read -r platform binary <<< "$target"
  printf 'test binary for %s\n' "$platform" > "$temp_root/$binary"
  zip -j -q "$output_dir/yamdl_${version}_${platform}.zip" "$temp_root/$binary"
  rm "$temp_root/$binary"
done

(
  cd "$output_dir"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.zip > SHA256SUMS
  else
    shasum -a 256 ./*.zip > SHA256SUMS
  fi
)

bash "$validator" "$version" "$output_dir" "$changelog"

printf 'tampered archive\n' >> "$output_dir/yamdl_${version}_linux_amd64.zip"
if bash "$validator" "$version" "$output_dir" "$changelog"; then
  printf 'validator accepted a tampered archive\n' >&2
  exit 1
fi

printf 'release validator pass/fail scenarios passed\n'
