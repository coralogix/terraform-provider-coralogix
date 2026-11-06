#!/usr/bin/env bash
# Require every resource of internal/provider/codegen.go in tools/iac-codegen/auto-regenerate.txt.
# Prints "<RESOURCE> <OUT> <Terraform name>" for each listed resource that codegen.go has, and
# reports each listed resource that it does not have yet on stderr.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")" && git rev-parse --show-toplevel)
cd "$root"
list=tools/iac-codegen/auto-regenerate.txt

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
sed -E 's/#.*//; s/[[:space:]]+//g; /^$/d' "$list" | sort -u >"$work/listed"

: >"$work/generated"
missing=0
while read -r resource out; do
  type_name=$(sed -nE 's/^const TypeName = "([^"]+)"$/\1/p' "internal/provider/generated/$out/resource.go" 2>/dev/null || true)
  if [[ -z "$type_name" ]]; then
    printf 'internal/provider/generated/%s/resource.go has no TypeName constant.\n' "$out" >&2
    exit 1
  fi
  name=coralogix_$type_name
  printf '%s\n' "$name" >>"$work/generated"
  if ! grep -qx "$name" "$work/listed"; then
    printf '%s is generated (internal/provider/codegen.go) but missing from %s.\n' "$name" "$list" >&2
    missing=1
    continue
  fi
  printf '%s %s %s\n' "$resource" "$out" "$name"
done < <(sed -nE 's#^//go:generate make -C \.\./\.\. codegen RESOURCE=([^ ]+) OUT=([^ ]+)$#\1 \2#p' \
  internal/provider/codegen.go)

if [[ $missing -ne 0 ]]; then
  printf 'The regeneration pull request must regenerate every generated resource. Add the missing names.\n' >&2
  exit 1
fi

sort -u "$work/generated" -o "$work/generated"
comm -23 "$work/listed" "$work/generated" | while read -r name; do
  printf '%s is listed but not generated yet. Skipped.\n' "$name" >&2
done
