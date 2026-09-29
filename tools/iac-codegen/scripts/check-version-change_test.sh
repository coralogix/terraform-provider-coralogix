#!/usr/bin/env bash
set -euo pipefail

checker=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-version-change.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

new_repo() {
  local repo=$1
  mkdir -p "$repo/tools/iac-codegen/internal/version" "$repo/tools/iac-codegen/internal/generator"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name Test
  git -C "$repo" config commit.gpgsign false
}

write_version() {
  local repo=$1
  local value=$2
  printf 'package version\n\nconst Current = "%s"\n' "$value" >"$repo/tools/iac-codegen/internal/version/version.go"
}

commit_all() {
  local repo=$1
  local message=$2
  git -C "$repo" add .
  git -C "$repo" commit -qm "$message"
}

repo="$work/existing"
new_repo "$repo"
write_version "$repo" v0.1.0-beta.1
printf 'package generator\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
commit_all "$repo" base
base=$(git -C "$repo" rev-parse HEAD)

printf 'package generator\n\n// behavior changed\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
printf '// comment only\n' >>"$repo/tools/iac-codegen/internal/version/version.go"
commit_all "$repo" unchanged-version
if (cd "$repo" && "$checker" "$base") >/dev/null 2>&1; then
  printf '%s\n' 'comment-only version edit passed unexpectedly' >&2
  exit 1
fi

write_version "$repo" v0.1.0-beta.2
commit_all "$repo" changed-version
(cd "$repo" && "$checker" "$base")

repo="$work/new-generator"
new_repo "$repo"
printf 'base\n' >"$repo/README.md"
commit_all "$repo" base
base=$(git -C "$repo" rev-parse HEAD)
write_version "$repo" v0.1.0-beta.1
printf 'package generator\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
commit_all "$repo" add-generator
(cd "$repo" && "$checker" "$base")
