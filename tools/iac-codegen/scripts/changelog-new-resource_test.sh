#!/usr/bin/env bash
set -euo pipefail

script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/changelog-new-resource.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

repo=$work/repo
mkdir -p "$repo/docs/resources" "$repo/docs/data-sources" "$repo/internal/provider/generated/old" "$repo/internal/provider/generated/fresh" \
  "$repo/internal/provider/generated/nods"
git init -q -b master "$repo"
g() {
  git -C "$repo" -c user.name=t -c user.email=t@example.com -c commit.gpgSign=false "$@"
}
printf '# Unreleased\n\n# Release 1.0.0\n\n- First.\n' >"$repo/CHANGELOG.md"
printf 'package old\n\nconst TypeName = "old"\n' >"$repo/internal/provider/generated/old/resource.go"
printf 'package fresh\n\nconst TypeName = "fresh"\n' >"$repo/internal/provider/generated/fresh/resource.go"
printf 'package nods\n\nconst TypeName = "nods"\n' >"$repo/internal/provider/generated/nods/resource.go"
echo page >"$repo/docs/resources/old.md"
echo page >"$repo/docs/data-sources/old.md"
g add -A
g commit -q -m base
# The docs that the generator writes after HEAD: a new resource with a data source, a new
# resource with none, and a change to a released one.
echo page >"$repo/docs/resources/fresh.md"
echo page >"$repo/docs/data-sources/fresh.md"
echo page >"$repo/docs/resources/nods.md"
echo changed >"$repo/docs/resources/old.md"

run() { (cd "$repo" && "$script" "$1"); }
run fresh
run fresh # a repeated run adds nothing
run nods
run old

got=$(cat "$repo/CHANGELOG.md")
for line in '#### resource/coralogix_fresh' '- FEAT: Add `coralogix_fresh`, generated from the API contract.' \
  '#### data-source/coralogix_fresh' '- FEAT: Add `data.coralogix_fresh`, generated from the API contract.' \
  '#### resource/coralogix_nods' '- FEAT: Add `coralogix_nods`, generated from the API contract.'; do
  if [[ $(grep -cxF -- "$line" "$repo/CHANGELOG.md") -ne 1 ]]; then
    printf 'FAIL: want exactly one line %q in:\n%s\n' "$line" "$got" >&2
    exit 1
  fi
done
for absent in 'coralogix_old' 'data.coralogix_nods'; do
  if grep -qF -- "$absent" "$repo/CHANGELOG.md"; then
    printf 'FAIL: %s must have no entry in:\n%s\n' "$absent" "$got" >&2
    exit 1
  fi
done
printf 'changelog-new-resource.sh: ok\n'
