#!/usr/bin/env bash

set -euo pipefail

version=${1:-}
output_dir=${2:-build}
changelog=${3:-CHANGELOG.md}

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  printf 'Invalid release version: %s\n' "$version" >&2
  exit 2
fi

if [[ ! -f "$changelog" ]]; then
  printf 'Missing changelog: %s\n' "$changelog" >&2
  exit 1
fi

if ! awk -v version="$version" \
  '$0 ~ "^## " version "([[:space:]]|$)" { found=1 } END { exit !found }' \
  "$changelog"; then
  printf 'CHANGELOG.md has no section for %s\n' "$version" >&2
  exit 1
fi

if [[ ! -d "$output_dir" ]]; then
  printf 'Missing release output directory: %s\n' "$output_dir" >&2
  exit 1
fi

platforms=(linux_amd64 linux_arm64 windows_amd64 windows_arm64 darwin_amd64 darwin_arm64)
binaries=(yamdl yamdl yamdl.exe yamdl.exe yamdl yamdl)
expected_archives=()

for platform in "${platforms[@]}"; do
  expected_archives+=("yamdl_${version}_${platform}.zip")
done

for index in "${!expected_archives[@]}"; do
  archive="${expected_archives[$index]}"
  binary="${binaries[$index]}"
  archive_path="$output_dir/$archive"
  if [[ ! -f "$archive_path" ]]; then
    printf 'Missing release archive: %s\n' "$archive_path" >&2
    exit 1
  fi

  entries=$(unzip -Z1 "$archive_path")
  entry_count=$(printf '%s\n' "$entries" | awk 'NF { count++ } END { print count + 0 }')
  if [[ "$entry_count" -ne 1 || "$entries" != "$binary" ]]; then
    printf 'Unexpected ZIP contents in %s\n' "$archive" >&2
    printf '%s\n' "$entries" >&2
    exit 1
  fi
done

shopt -s nullglob
archives=("$output_dir"/*.zip)
if [[ "${#archives[@]}" -ne "${#expected_archives[@]}" ]]; then
  printf 'Unexpected number of release archives: got %d, want %d\n' \
    "${#archives[@]}" "${#expected_archives[@]}" >&2
  exit 1
fi
for archive_path in "${archives[@]}"; do
  archive_name=$(basename "$archive_path")
  found=0
  for expected_archive in "${expected_archives[@]}"; do
    if [[ "$archive_name" == "$expected_archive" ]]; then
      found=1
      break
    fi
  done
  if [[ "$found" -ne 1 ]]; then
    printf 'Unexpected release archive: %s\n' "$archive_name" >&2
    exit 1
  fi
done

checksum_file="$output_dir/SHA256SUMS"
if [[ ! -f "$checksum_file" ]]; then
  printf 'Missing checksum manifest: %s\n' "$checksum_file" >&2
  exit 1
fi

seen_archives=()
seen_hashes=()
checksum_lines=0
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ -z "$line" ]] && continue
  read -r expected_hash archive_name extra <<< "$line"
  archive_name=${archive_name#./}
  if [[ -n "${extra:-}" || ! "$expected_hash" =~ ^[0-9a-fA-F]{64}$ ]]; then
    printf 'Invalid checksum entry for %s\n' "${archive_name:-<empty>}" >&2
    exit 1
  fi

  archive_index=-1
  for index in "${!expected_archives[@]}"; do
    if [[ "$archive_name" == "${expected_archives[$index]}" ]]; then
      archive_index=$index
      break
    fi
  done
  if [[ "$archive_index" -lt 0 ]]; then
    printf 'Checksum references unexpected archive: %s\n' "$archive_name" >&2
    exit 1
  fi
  if [[ -n "${seen_archives[$archive_index]:-}" ]]; then
    printf 'Duplicate checksum entry for %s\n' "$archive_name" >&2
    exit 1
  fi
  seen_archives[archive_index]=1
  seen_hashes[archive_index]="$expected_hash"
  checksum_lines=$((checksum_lines + 1))
done < "$checksum_file"

if [[ "$checksum_lines" -ne "${#expected_archives[@]}" ]]; then
  printf 'Unexpected checksum entry count: got %d, want %d\n' \
    "$checksum_lines" "${#expected_archives[@]}" >&2
  exit 1
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

for index in "${!expected_archives[@]}"; do
  archive="${expected_archives[$index]}"
  actual_hash=$(sha256_file "$output_dir/$archive")
  if [[ "${seen_hashes[$index]:-}" != "$actual_hash" ]]; then
    printf 'Checksum mismatch for %s\n' "$archive" >&2
    exit 1
  fi
done

printf 'Release artifacts validated for %s\n' "$version"
