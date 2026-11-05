#!/usr/bin/env bash
set -euo pipefail

source_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-auto-regenerate-list.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

repo=$work/repo
mkdir -p "$repo/tools/iac-codegen/scripts" "$repo/internal/provider/generated/thing" "$repo/internal/provider/generated/other"
git -C "$repo" init -q
cp "$source_script" "$repo/tools/iac-codegen/scripts/"
checker=$repo/tools/iac-codegen/scripts/check-auto-regenerate-list.sh
printf '//go:generate make -C ../.. codegen RESOURCE=Thing OUT=thing\n//go:generate make -C ../.. codegen RESOURCE=v2.Other OUT=other\n' \
  >"$repo/internal/provider/codegen.go"
printf 'package thing\n\nconst TypeName = "thing"\n' >"$repo/internal/provider/generated/thing/resource.go"
printf 'package other\n\nconst TypeName = "other_thing"\n' >"$repo/internal/provider/generated/other/resource.go"

printf '# comment\ncoralogix_thing\n\ncoralogix_other_thing  # trailing\ncoralogix_pending\n' >"$repo/tools/iac-codegen/auto-regenerate.txt"
out=$("$checker" 2>"$work/stderr")
want='Thing thing coralogix_thing
v2.Other other coralogix_other_thing'
if [[ "$out" != "$want" ]]; then
  printf 'FAIL: listed resources\nwant:\n%s\ngot:\n%s\n' "$want" "$out" >&2
  exit 1
fi
if ! grep -q '^coralogix_pending is listed but not generated yet' "$work/stderr"; then
  printf '%s\n' 'FAIL: a listed resource that is not generated must be reported' >&2
  exit 1
fi

printf 'coralogix_thing\n' >"$repo/tools/iac-codegen/auto-regenerate.txt"
if "$checker" >/dev/null 2>"$work/stderr"; then
  printf '%s\n' 'FAIL: a generated resource missing from the list must fail' >&2
  exit 1
fi
if ! grep -q '^coralogix_other_thing is generated' "$work/stderr"; then
  printf '%s\n' 'FAIL: the error must name the missing resource' >&2
  exit 1
fi

printf '%s\n' 'check-auto-regenerate-list tests passed'
