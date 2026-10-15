#!/usr/bin/env bash

set -euo pipefail

version=${1:-}
changelog=${2:-CHANGELOG.md}

if [[ -z "$version" || ! -f "$changelog" ]]; then
  printf 'Usage: %s VERSION [CHANGELOG]\n' "${0##*/}" >&2
  exit 2
fi

awk -v heading="## $version" '
  $0 == heading || (index($0, heading) == 1 && substr($0, length(heading) + 1, 1) ~ /[[:space:]]/) {
    capture=1
    found=1
    next
  }
  capture && /^## / { exit }
  capture { print; has_content=1 }
  END { if (!found || !has_content) exit 1 }
' "$changelog"
