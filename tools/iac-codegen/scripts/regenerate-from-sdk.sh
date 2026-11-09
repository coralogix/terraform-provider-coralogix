#!/usr/bin/env bash
# Pin the Management SDK at a ref, regenerate the resources of tools/iac-codegen/auto-regenerate.txt
# and the docs, and add a changelog entry for each resource whose generated files changed: its
# package under internal/provider/generated or its examples, the paths that
# check-generated-resources.sh compares.
# Usage: regenerate-from-sdk.sh <SDK ref: commit SHA, tag, or branch>
# Prints the Terraform name of each changed resource, one per line, and nothing when the
# generated code is unchanged. TFGEN can name a prebuilt generator.
set -euo pipefail

if [[ $# -ne 1 || -z $1 ]]; then
  printf '%s\n' 'usage: regenerate-from-sdk.sh <SDK ref>' >&2
  exit 2
fi
ref=$1
sdk=github.com/coralogix/coralogix-management-sdk

root=$(git rev-parse --show-toplevel)
cd "$root"
scripts=$root/tools/iac-codegen/scripts

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# Progress goes to stderr: stdout is the list of changed resources.
go get "$sdk@$ref" >&2
go mod tidy >&2
# The generator reads openapi.yaml of the pinned SDK from the module cache with GOPROXY=off.
go mod download >&2
version=$(go list -m -f '{{.Version}}' "$sdk")

if [[ -n "${TFGEN:-}" ]]; then
  binary=$TFGEN
else
  binary=$work/tfgen
  (cd tools/iac-codegen && go build -o "$binary" ./cmd/tfgen)
fi

# Only resources of tools/iac-codegen/auto-regenerate.txt; the check fails when a generated one is missing.
"$scripts/check-auto-regenerate-list.sh" >"$work/resources"
while read -r resource out _; do
  printf 'Generating %s into %s\n' "$resource" "$out" >&2
  "$binary" generate --resource "$resource" --out "internal/provider/generated/$out" >&2
done <"$work/resources"
make generate >&2

# changelog_entries adds the entries of one doc kind (resources or data-sources) of a resource and
# counts them in entries. It is not called in a command substitution: there, set -e would not stop
# the script when an entry cannot be written, and bash 3.2 has no inherit_errexit.
changelog_entries() {
  local kind=$1 type_name=$2 version=$3 heading added removed breaking relaxed
  if [[ $kind == resources ]]; then
    heading="#### resource/coralogix_$type_name"
  else
    heading="#### data-source/coralogix_$type_name"
  fi
  git show "HEAD:docs/$kind/$type_name.md" >"$work/old.md" 2>/dev/null || : >"$work/old.md"
  "$scripts/schema-changes.sh" "$work/old.md" "docs/$kind/$type_name.md" >"$work/changes"

  # names prints the attributes of one kind of change as "`a`, `b`".
  names() { awk -F '\t' -v kind="$1" '$1 == kind { out = out sep "`" $2 "`"; sep = ", " } END { print out }' "$work/changes"; }
  # moves prints the changes of one kind as "`a` (Optional String to Required String), ...".
  moves() { awk -F '\t' -v kind="$1" '$1 == kind { out = out sep "`" $2 "` (" $3 " to " $4 ")"; sep = ", " } END { print out }' "$work/changes"; }
  # One assignment per line: a line of several takes the status of the last substitution only.
  added=$(names added)
  removed=$(names removed)
  breaking=$(moves breaking)
  relaxed=$(moves relaxed)

  if [[ -n $added ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" \
      "- FEAT: Add $added from the API contract of Management SDK \`$version\`."
    entries=$((entries + 1))
  fi
  if [[ -n $relaxed ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" \
      "- FEAT: Change $relaxed from the API contract of Management SDK \`$version\`."
    entries=$((entries + 1))
  fi
  if [[ -n $breaking ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" \
      "- BREAKING: Change $breaking from the API contract of Management SDK \`$version\`."
    entries=$((entries + 1))
  fi
  if [[ -n $removed ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" \
      "- BREAKING: Remove $removed, which the API contract of Management SDK \`$version\` no longer has."
    entries=$((entries + 1))
  fi
}

# A resource changed when its generated package or one of its examples did. The examples of a
# resource are under examples/resources/coralogix_<type> and examples/data-sources/coralogix_<type>.
awk '{ print $2, $3 }' "$work/resources" >"$work/names"
git status --porcelain --untracked-files=all -- internal/provider/generated examples |
  sed -nE 's#^.{3}(.*)$#\1#p' |
  while read -r path; do
    case $path in
      internal/provider/generated/*/*)
        out=${path#internal/provider/generated/}
        awk -v out="${out%%/*}" '$1 == out { print $2 }' "$work/names"
        ;;
      examples/resources/coralogix_*/* | examples/data-sources/coralogix_*/*)
        name=${path#examples/*/}
        name=${name%%/*}
        awk -v name="$name" '$2 == name { print $2 }' "$work/names"
        ;;
    esac
  done | sort -u >"$work/changed"

while read -r name; do
  type_name=${name#coralogix_}
  entries=0
  changelog_entries resources "$type_name" "$version"
  changelog_entries data-sources "$type_name" "$version"
  if [[ $entries -eq 0 ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "#### resource/$name" \
      "- CHORE: Regenerate from the API contract of Management SDK \`$version\`."
  fi
done <"$work/changed"

cat "$work/changed"
