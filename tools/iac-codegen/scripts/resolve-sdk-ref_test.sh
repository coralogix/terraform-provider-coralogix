#!/usr/bin/env bash
set -euo pipefail

resolver=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/resolve-sdk-ref.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

repo=$work/repo
git init -q -b master "$repo"
# A developer's git config can sign commits and tags; the test repository needs neither.
g() {
  git -C "$repo" -c user.name=t -c user.email=t@example.com -c commit.gpgSign=false \
    -c tag.gpgSign=false -c tag.forceSignAnnotated=false "$@"
}
commit() {
  g commit -q --allow-empty -m "$1"
  g rev-parse HEAD
}
first=$(commit first)
g tag light
g tag -a -m annotated annotated
g branch both
g tag both
second=$(commit second)
# Sorts before refs/heads/master, and ls-remote matches it for the pattern "master".
g branch feature/master "$first"
g branch split
g tag split "$first"

expect() {
  local ref=$1 want=$2 got
  got=$("$resolver" "$repo" "$ref")
  if [[ "$got" != "$want" ]]; then
    printf 'FAIL: %s resolved to %s, want %s\n' "$ref" "$got" "$want" >&2
    exit 1
  fi
}
expect_failure() {
  local ref=$1 message=$2
  if "$resolver" "$repo" "$ref" >/dev/null 2>"$work/stderr"; then
    printf 'FAIL: %s must not resolve\n' "$ref" >&2
    exit 1
  fi
  if ! grep -q "$message" "$work/stderr"; then
    printf 'FAIL: %s: want "%s", got:\n' "$ref" "$message" >&2
    cat "$work/stderr" >&2
    exit 1
  fi
}

expect master "$second"
expect light "$first"
expect annotated "$first"
expect both "$first"
expect "$first" "$first"
expect_failure missing 'neither a branch nor a tag'
expect_failure split 'a branch and a tag on different commits'
expect_failure 'bad;ref' 'Invalid SDK ref'

printf '%s\n' 'resolve-sdk-ref tests passed'
