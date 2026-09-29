#!/usr/bin/env bash
set -euo pipefail

base=${1:?usage: check-version-change.sh <base-commit>}
root=$(git rev-parse --show-toplevel)
cd "$root"

changed=$(git diff --name-only "$base"...HEAD -- tools/iac-codegen | grep -E '^tools/iac-codegen/(cmd|internal)/.*(\.go|\.tmpl)$' | grep -Ev '(_test\.go$|/testdata/|/internal/version/version\.go$)' || true)
if [[ -z "$changed" ]]; then
  exit 0
fi

version_path=tools/iac-codegen/internal/version/version.go
version_at() {
  git show "$1:$version_path" 2>/dev/null | awk -F'"' '/^[[:space:]]*const Current = "[^"]+"[[:space:]]*$/ { print $2 }'
}

base_version=$(version_at "$base" || true)
head_version=$(version_at HEAD || true)
if [[ -z "$head_version" ]]; then
  printf '%s\n' 'Generator code or templates changed, but version.Current is missing or invalid.' >&2
  printf '%s\n' "$changed" >&2
  exit 1
fi
if [[ "$base_version" == "$head_version" ]]; then
  printf '%s\n' 'Generator code or templates changed without a semantic generator version change.' >&2
  printf '%s\n' "$changed" >&2
  exit 1
fi
