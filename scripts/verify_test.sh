#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
verify_script="$script_dir/verify.sh"
temp_root=$(mktemp -d "${TMPDIR:-/tmp}/yamdl-verify-test.XXXXXX")
trap 'rm -rf "$temp_root"' EXIT

git -C "$temp_root" init -q
mkdir -p "$temp_root/bin"
printf 'package sample\n\nfunc main() {}\n' > "$temp_root/main.go"
git -C "$temp_root" add main.go

cat > "$temp_root/bin/gofmt" <<'EOF'
#!/usr/bin/env bash
if [[ "${FAKE_GOFMT_MODE:-pass}" == fail ]]; then
  printf 'main.go\n'
fi
EOF
chmod +x "$temp_root/bin/gofmt"

if ! PATH="$temp_root/bin:$PATH" YAMDL_VERIFY_ROOT="$temp_root" YAMDL_VERIFY_LIB_ONLY=1 bash -c \
  'source "$1"; check_format "$YAMDL_VERIFY_ROOT"' _ "$verify_script"; then
  printf 'clean formatting check unexpectedly failed\n' >&2
  exit 1
fi

if PATH="$temp_root/bin:$PATH" FAKE_GOFMT_MODE=fail YAMDL_VERIFY_ROOT="$temp_root" YAMDL_VERIFY_LIB_ONLY=1 bash -c \
  'source "$1"; check_format "$YAMDL_VERIFY_ROOT"' _ "$verify_script"; then
  printf 'formatting check accepted a controlled failure\n' >&2
  exit 1
fi

printf 'verify.sh controlled pass/fail scenarios passed\n'
