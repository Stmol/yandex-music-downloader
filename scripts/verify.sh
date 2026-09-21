#!/usr/bin/env bash

set -euo pipefail

repo_root=${YAMDL_VERIFY_ROOT:-$(git rev-parse --show-toplevel)}
cd "$repo_root"

step() {
  local name=$1
  shift
  printf '\n==> %s\n' "$name"
  "$@"
}

check_format() {
  local root=$1
  local files
  local unformatted

  files=$(git -C "$root" ls-files '*.go')
  if [[ -z "$files" ]]; then
    return 0
  fi

  unformatted=$(cd "$root" && printf '%s\n' "$files" | xargs gofmt -l)
  if [[ -n "$unformatted" ]]; then
    printf 'Go files need formatting:\n%s\n' "$unformatted" >&2
    return 1
  fi
}

check_assets() {
  local required=(
    AGENTS.md
    CONTRIBUTING.md
    LICENSE
    SECURITY.md
    .gitleaks.toml
    .editorconfig
    scripts/build.sh
    scripts/verify.sh
    scripts/verify_test.sh
    scripts/release-validate.sh
    scripts/release-validate-test.sh
    docs/architecture.md
    docs/testing.md
    docs/releasing.md
  )
  local path

  for path in "${required[@]}"; do
    if [[ ! -e "$path" && ! -L "$path" ]]; then
      printf 'Missing required project asset: %s\n' "$path" >&2
      return 1
    fi
  done

  local skill
  local skills=(
    .agents/skills/yamdl/SKILL.md
    .agents/skills/bugs/SKILL.md
    .agents/skills/release-flow/SKILL.md
  )

  for skill in "${skills[@]}"; do
    if [[ ! -f "$skill" || -L "$skill" ]]; then
      printf 'Skill must be a regular file: %s\n' "$skill" >&2
      return 1
    fi
  done
}

check_shell_syntax() {
  local script
  while IFS= read -r -d '' script; do
    bash -n "$script"
  done < <(find scripts -maxdepth 1 -type f -name '*.sh' -print0)
}

check_cli_build() {
  local binary_path="${TMPDIR:-/tmp}/yamdl-verify-bin.$$"
  go build -trimpath -o "$binary_path" ./cmd/yamdl
  rm -f "$binary_path"
}

main() {
  step 'Go formatting' check_format "$repo_root"
  step 'Go module integrity' go mod verify
  step 'Go module tidy diff' go mod tidy -diff
  step 'Go vet' go vet ./...
  step 'Go tests' go test ./...
  step 'Go race tests' go test -race ./...
  step 'Staticcheck' go tool staticcheck ./...
  step 'CLI build' check_cli_build
  step 'Shell syntax' check_shell_syntax
  step 'Project assets' check_assets
  step 'Whitespace' git diff --check
  printf '\nVerification passed.\n'
}

if [[ "${YAMDL_VERIFY_LIB_ONLY:-}" != '1' ]]; then
  main "$@"
fi
