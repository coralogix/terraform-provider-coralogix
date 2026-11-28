#!/usr/bin/env bash
set -euo pipefail

checker=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-tests-unchanged.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

new_repo() {
  local repo=$1
  mkdir -p "$repo/tools/iac-codegen/internal/generator/testdata/golden" "$repo/internal/provider/generated/thing" \
    "$repo/internal/provider/other"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name Test
  git -C "$repo" config commit.gpgsign false
  printf 'package generator\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
  printf 'package generator\n' >"$repo/tools/iac-codegen/internal/generator/thing_test.go"
  printf 'golden\n' >"$repo/tools/iac-codegen/internal/generator/testdata/golden/thing.go"
  printf 'values: {}\n' >"$repo/internal/provider/generated/thing/acceptance.yaml"
  printf 'package other\n' >"$repo/internal/provider/other/other_test.go"
  commit_all "$repo" base
}

commit_all() {
  git -C "$1" add .
  git -C "$1" commit -qm "$2"
}

expect_ok() {
  if ! (cd "$1" && "$checker" "$2") >"$work/out" 2>&1; then
    printf 'expected success for %s\n' "$3" >&2
    cat "$work/out" >&2
    exit 1
  fi
}

expect_fail() {
  if (cd "$1" && "$checker" "$2") >"$work/out" 2>&1; then
    printf 'expected failure for %s\n' "$3" >&2
    exit 1
  fi
  grep -q -F "$4" "$work/out" || { printf 'missing %s in the report for %s\n' "$4" "$3" >&2; cat "$work/out" >&2; exit 1; }
}

# A generator change that adds a test and a golden passes.
repo=$work/adds
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package generator\n// code\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
printf 'package generator\n' >"$repo/tools/iac-codegen/internal/generator/new_test.go"
printf 'golden\n' >"$repo/tools/iac-codegen/internal/generator/testdata/golden/new.go"
commit_all "$repo" change
expect_ok "$repo" "$base" "added tests"

# A test added to the end of an existing test file passes.
repo=$work/appends
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package generator\n// code\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
printf 'func TestNew() {}\n' >>"$repo/tools/iac-codegen/internal/generator/thing_test.go"
commit_all "$repo" change
expect_ok "$repo" "$base" "an appended test"

# A change to an existing test fails.
repo=$work/edits-test
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package generator\n// code\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
printf 'package edited\n' >"$repo/tools/iac-codegen/internal/generator/thing_test.go"
commit_all "$repo" change
expect_fail "$repo" "$base" "an edited test" "thing_test.go"

# A change to an existing golden fails.
repo=$work/edits-golden
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package generator\n// code\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
printf 'changed\n' >"$repo/tools/iac-codegen/internal/generator/testdata/golden/thing.go"
commit_all "$repo" change
expect_fail "$repo" "$base" "an edited golden" "golden/thing.go"

# A deleted acceptance input fails, and so does a moved test.
repo=$work/deletes
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package generator\n// code\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
git -C "$repo" rm -q internal/provider/generated/thing/acceptance.yaml
git -C "$repo" mv tools/iac-codegen/internal/generator/thing_test.go tools/iac-codegen/internal/generator/moved_test.go
commit_all "$repo" change
expect_fail "$repo" "$base" "a deleted acceptance input" "acceptance.yaml"
grep -q -F "thing_test.go" "$work/out" || { echo "a moved test must be reported" >&2; exit 1; }

# A change that does not touch the generator may edit its own test.
repo=$work/no-generator
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package other\n// edited\n' >"$repo/internal/provider/other/other_test.go"
commit_all "$repo" change
expect_ok "$repo" "$base" "a change outside the generator"

# A change to a generator script alone is not a generator change, so a test elsewhere may change.
repo=$work/scripts-only
new_repo "$repo"
mkdir -p "$repo/tools/iac-codegen/scripts"
printf '#!/bin/sh\n' >"$repo/tools/iac-codegen/scripts/x.sh"
commit_all "$repo" scripts
base=$(git -C "$repo" rev-parse HEAD)
printf '#!/bin/sh\n# edited\n' >"$repo/tools/iac-codegen/scripts/x.sh"
printf 'package other\n// edited\n' >"$repo/internal/provider/other/other_test.go"
commit_all "$repo" change
expect_ok "$repo" "$base" "a script-only change"

# A removed test line passes only when test-edits-allowed.txt names it.
mkdir -p "$work/gate"
cp "$checker" "$work/gate/check-tests-unchanged.sh"
printf 'tools/iac-codegen/internal/generator/thing_test.go\told := 1\n' >"$work/gate/test-edits-allowed.txt"
repo=$work/allowed
new_repo "$repo"
printf 'package generator\n\told := 1\n\tkeep := 2\n' >"$repo/tools/iac-codegen/internal/generator/thing_test.go"
commit_all "$repo" more
base=$(git -C "$repo" rev-parse HEAD)
printf 'package generator\n// code\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
printf 'package generator\n\tnew := 1\n\tkeep := 2\n' >"$repo/tools/iac-codegen/internal/generator/thing_test.go"
commit_all "$repo" change
if ! (cd "$repo" && "$work/gate/check-tests-unchanged.sh" "$base") >"$work/out" 2>&1; then
  printf 'a named line may be removed\n' >&2; cat "$work/out" >&2; exit 1
fi
printf 'package generator\n\tnew := 1\n\tother := 3\n' >"$repo/tools/iac-codegen/internal/generator/thing_test.go"
commit_all "$repo" change2
if (cd "$repo" && "$work/gate/check-tests-unchanged.sh" "$base") >"$work/out" 2>&1; then
  printf 'a line that no entry names must not be removed\n' >&2; exit 1
fi
grep -q -F 'edited' "$work/out"
