#!/usr/bin/env bash
# Require every resource of internal/provider/codegen.go in tools/iac-codegen/auto-regenerate.txt.
# Prints "<RESOURCE> <OUT> <Terraform name>" for each listed resource that codegen.go has, and
# reports each listed resource that it does not have yet on stderr.
set -euo pipefail

scripts=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$scripts" && git rev-parse --show-toplevel)
cd "$root"
list=tools/iac-codegen/auto-regenerate.txt

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
sed -E 's/#.*//; s/[[:space:]]+//g; /^$/d' "$list" | sort -u >"$work/listed"
"$scripts/codegen-list.sh" --names >"$work/generated"

missing=0
while read -r resource out name; do
  if ! grep -qx "$name" "$work/listed"; then
    printf '%s is generated (internal/provider/codegen.go) but missing from %s.\n' "$name" "$list" >&2
    missing=1
    continue
  fi
  printf '%s %s %s\n' "$resource" "$out" "$name"
done <"$work/generated"

if [[ $missing -ne 0 ]]; then
  printf 'The regeneration pull request must regenerate every generated resource. Add the missing names.\n' >&2
  exit 1
fi

awk '{ print $3 }' "$work/generated" | sort -u | comm -23 "$work/listed" - | while read -r name; do
  printf '%s is listed but not generated yet. Skipped.\n' "$name" >&2
done
