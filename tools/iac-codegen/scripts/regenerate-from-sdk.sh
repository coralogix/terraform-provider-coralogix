#!/usr/bin/env bash
# Pin the Management SDK at a ref, regenerate the resources of tools/iac-codegen/auto-regenerate.txt
# and the docs, and add a changelog entry for each resource whose generated code changed.
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

# attribute_names reads a docs diff and prints the attribute names of its lines that start with
# the given sign. A description change removes and adds the same name, so callers compare both.
attribute_names() {
  sed -nE "s/^[$1]- \`([^\`]+)\`.*/\\1/p" | sort -u
}

changed=()
while read -r out; do
  type_name=$(sed -nE 's/^const TypeName = "([^"]+)"$/\1/p' "internal/provider/generated/$out/resource.go")
  if [[ -z "$type_name" ]]; then
    printf 'internal/provider/generated/%s/resource.go has no TypeName constant.\n' "$out" >&2
    exit 1
  fi
  name=coralogix_$type_name
  changed+=("$name")

  git diff -U0 -- "docs/resources/$type_name.md" >"$work/docs.diff"
  attribute_names + <"$work/docs.diff" >"$work/plus"
  attribute_names - <"$work/docs.diff" >"$work/minus"
  added=$(comm -23 "$work/plus" "$work/minus" | sed 's/.*/`&`/' | paste -sd, - | sed 's/,/, /g')
  removed=$(comm -13 "$work/plus" "$work/minus" | sed 's/.*/`&`/' | paste -sd, - | sed 's/,/, /g')

  heading="#### resource/$name"
  if [[ -n "$added" ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" \
      "- FEAT: Add $added from the API contract of Management SDK \`$version\`."
  fi
  if [[ -n "$removed" ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" \
      "- BREAKING: Remove $removed, which the API contract of Management SDK \`$version\` no longer has."
  fi
  if [[ -z "$added" && -z "$removed" ]]; then
    "$scripts/add-changelog-entry.sh" CHANGELOG.md "$heading" \
      "- CHORE: Regenerate from the API contract of Management SDK \`$version\`."
  fi
done < <(git status --porcelain -- internal/provider/generated |
  sed -nE 's#^.{3}internal/provider/generated/([^/]+)/.*#\1#p' | sort -u)

if [[ ${#changed[@]} -gt 0 ]]; then
  printf '%s\n' "${changed[@]}"
fi
