#!/usr/bin/env bash
# Require current package versions; compare full output when a version or manifest changes.
set -euo pipefail

base=${1:?usage: check-generated-resources.sh <base-commit>}
root=$(git rev-parse --show-toplevel)
cd "$root"

version_path=tools/iac-codegen/internal/version/version.go
version_at() {
  git show "$1:$version_path" 2>/dev/null | awk -F'"' '/^[[:space:]]*const Current = "[^"]+"[[:space:]]*$/ { print $2 }'
}

base_version=$(version_at "$base" || true)
head_version=$(version_at HEAD || true)
if [[ -z "$head_version" ]]; then
  printf '%s\n' 'Generator version.Current is missing or invalid.' >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
sed -nE 's#^//go:generate make -C \.\./\.\. codegen RESOURCE=([^ ]+) OUT=([^ ]+)$#\1 \2#p' \
  internal/provider/codegen.go >"$work/resources"
if [[ ! -s "$work/resources" ]]; then
  printf '%s\n' 'No generated resources are listed in internal/provider/codegen.go.' >&2
  exit 1
fi

for resource_file in internal/provider/generated/*/resource.go; do
  out=$(basename "$(dirname "$resource_file")")
  if ! awk -v out="$out" '$2 == out { found = 1 } END { exit !found }' "$work/resources"; then
    printf 'Generated resource %s is missing from internal/provider/codegen.go.\n' "$out" >&2
    exit 1
  fi
done

for out in $(awk '{ print $2 }' "$work/resources"); do
  if [[ ! -f "internal/provider/generated/$out/resource.go" ]]; then
    printf 'Listed resource %s has no generated resource.go.\n' "$out" >&2
    exit 1
  fi
  if ! grep -Fqx "$out: $head_version" internal/provider/generated/versions.yaml; then
    printf 'Generated resource %s does not use generator %s in versions.yaml.\n' "$out" "$head_version" >&2
    exit 1
  fi
done

if [[ "$base_version" == "$head_version" ]] &&
  git diff --quiet "$base"...HEAD -- internal/provider/generated/versions.yaml; then
  exit 0
fi

mkdir -p "$work/pristine" "$work/regenerated"
git archive HEAD | tar -x -C "$work/pristine"
git archive HEAD | tar -x -C "$work/regenerated"

if [[ -n "${TFGEN:-}" ]]; then
  binary=$TFGEN
else
  binary=$work/tfgen
  (cd tools/iac-codegen && go build -o "$binary" ./cmd/tfgen)
fi

while read -r resource out; do
  (cd "$work/regenerated" && "$binary" generate --resource "$resource" --out "internal/provider/generated/$out")
done <"$work/resources"

for path in internal/provider/generated examples; do
  if ! diff -r --brief "$work/pristine/$path" "$work/regenerated/$path"; then
    printf 'Generated files are stale. Run go generate ./internal/provider/ and commit the output.\n' >&2
    exit 1
  fi
done
