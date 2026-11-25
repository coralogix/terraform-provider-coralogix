#!/usr/bin/env bash
# Add the changelog entries of a new generated resource: one for the resource and one for its
# data source. A resource is new when its doc page is not in HEAD. A released resource gets no
# entry here: its entries come from the schema change (regenerate-from-sdk.sh). The script adds
# nothing twice, so a repeated `go generate` keeps one entry. Runs in the repository of the current
# directory, after the docs are generated.
# Usage: changelog-new-resource.sh <OUT: directory in internal/provider/generated>
set -euo pipefail

if [[ $# -ne 1 || -z $1 ]]; then
  printf '%s\n' 'usage: changelog-new-resource.sh <directory in internal/provider/generated>' >&2
  exit 2
fi
out=$1
scripts=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(git rev-parse --show-toplevel)
cd "$root"

type_name=$(sed -nE 's/^const TypeName = "([^"]+)"$/\1/p' "internal/provider/generated/$out/resource.go" 2>/dev/null || true)
if [[ -z "$type_name" ]]; then
  printf 'internal/provider/generated/%s/resource.go has no TypeName constant.\n' "$out" >&2
  exit 1
fi

add() { # add <doc kind> <heading> <bullet>
  local kind=$1 heading=$2 bullet=$3
  [[ -f "docs/$kind/$type_name.md" ]] || return 0
  if git cat-file -e "HEAD:docs/$kind/$type_name.md" 2>/dev/null; then
    return 0
  fi
  if grep -qxF -- "$bullet" CHANGELOG.md; then
    return 0
  fi
  "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" "$bullet"
}

add resources "#### resource/coralogix_$type_name" \
  "- FEAT: Add \`coralogix_$type_name\`, generated from the API contract."
add data-sources "#### data-source/coralogix_$type_name" \
  "- FEAT: Add \`data.coralogix_$type_name\`, generated from the API contract."
