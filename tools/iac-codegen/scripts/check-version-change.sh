#!/usr/bin/env bash
# Fails when the generator produces something new and version.Current stays the same.
#
# The generator version names what produced the generated files. The check builds tfgen at the
# base commit and at HEAD, runs both on the same input, and compares what they produce:
#   - the files of every resource that internal/provider/codegen.go lists;
#   - the `tfgen check` report of every component of the pinned spec, and of every generated
#     resource with its behavior-overrides file;
#   - the golden files of the generator tests, which hold the output for the synthetic specs.
# A change of code that changes none of these needs no new version.
#
# usage: check-version-change.sh <base-commit>
# TFGEN_OLD and TFGEN_NEW name prebuilt binaries instead of building them, and SPEC names the
# spec instead of the pinned one (for the tests).
set -euo pipefail

base=${1:?usage: check-version-change.sh <base-commit>}
root=$(git rev-parse --show-toplevel)
cd "$root"

changed=$(git diff --name-only "$base"...HEAD -- tools/iac-codegen | grep -Ev '^tools/iac-codegen/scripts/' || true)
if [[ -z "$changed" ]]; then
  exit 0
fi

version_path=tools/iac-codegen/internal/version/version.go
version_at() {
  git show "$1:$version_path" 2>/dev/null | awk -F'"' '/^[[:space:]]*const Current = "[^"]+"[[:space:]]*$/ { print $2 }'
}

base_version=$(version_at "$base" || true)
head_version=$(version_at HEAD || true)
if [[ -z "$head_version" ]]; then
  printf '%s\n' 'Generator code changed, but version.Current is missing or invalid.' >&2
  exit 1
fi
if [[ "$base_version" != "$head_version" ]]; then
  exit 0 # a new version is always allowed, and a new generator has no base to compare
fi

fail() {
  printf 'The generator output changed, but version.Current is still %s. Give the generator a new version.\n' "$head_version" >&2
  printf '%s\n' "$@" >&2
  exit 1
}

golden=$(git diff --name-only "$base"...HEAD -- tools/iac-codegen/internal/generator/testdata/golden)
if [[ -n "$golden" ]]; then
  fail "The golden files changed:" "$golden"
fi

work=$(mktemp -d)
cleanup() {
  git worktree remove --force "$work/base" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

old=${TFGEN_OLD:-}
new=${TFGEN_NEW:-}
if [[ -z "$old" ]]; then
  git worktree add --quiet --detach "$work/base" "$base"
  (cd "$work/base/tools/iac-codegen" && go build -o "$work/tfgen-old" ./cmd/tfgen)
  old=$work/tfgen-old
fi
if [[ -z "$new" ]]; then
  (cd tools/iac-codegen && go build -o "$work/tfgen-new" ./cmd/tfgen)
  new=$work/tfgen-new
fi

# Both binaries read the same input: a copy of HEAD each, so one cannot see the files of the other.
for side in old new; do
  mkdir -p "$work/$side-tree" "$work/$side-out/components" "$work/$side-out/resources"
  git archive HEAD | tar -x -C "$work/$side-tree"
done

spec=${SPEC:-}
if [[ -z "$spec" ]]; then
  go mod download
  spec=$(go list -m -f '{{.Dir}}' github.com/coralogix/coralogix-management-sdk)/openapi.yaml
fi

# Each binary runs from the provider root, so it finds the pinned SDK from the provider go.mod.
generate() {
  local side=$1 binary=$2 resource=$3 out=$4
  (cd "$work/$side-tree" && "$binary" generate --resource "$resource" --out "internal/provider/generated/$out") \
    >"$work/$side-out/resources/$out.generate.txt" 2>&1 || echo "exit $?" >>"$work/$side-out/resources/$out.generate.txt"
}
while read -r resource out; do
  generate old "$old" "$resource" "$out"
  generate new "$new" "$resource" "$out"
done < <(sed -nE 's#^//go:generate make -C \.\./\.\. codegen RESOURCE=([^ ]+) OUT=([^ ]+)$#\1 \2#p' internal/provider/codegen.go)

check() {
  local side=$1 binary=$2 file=$3
  shift 3
  "$binary" check "$@" >"$file" 2>&1 || echo "exit $?" >>"$file"
}
for overrides in internal/provider/generated/*/behavior-overrides.yaml; do
  dir=$(basename "$(dirname "$overrides")")
  resource=$(awk '/^resource:/ { print $2; exit }' "$overrides")
  for side in old new; do
    binary=$old
    [[ $side == new ]] && binary=$new
    check "$side" "$binary" "$work/$side-out/resources/$dir.check.txt" --resource "$resource" --openapi "$spec" --overrides "$overrides"
  done
done

components() {
  awk '
    /^components:[[:space:]]*$/ { in_components = 1; next }
    in_components && /^[^[:space:]]/ { exit }
    in_components && /^  schemas:[[:space:]]*$/ { in_schemas = 1; next }
    in_schemas && /^  [^[:space:]]/ { exit }
    in_schemas && /^    [^[:space:]#][^:]*:[[:space:]]*$/ { sub(/^    /, ""); sub(/:[[:space:]]*$/, ""); gsub(/["\047]/, ""); print }
  ' "$spec"
}
export -f check
export old new spec work
components | xargs -P 8 -I{} bash -c '
  name=$1 file=$(printf "%s" "$1" | tr "/" "_").txt
  check old "$old" "$work/old-out/components/$file" --resource "$name" --openapi "$spec"
  check new "$new" "$work/new-out/components/$file" --resource "$name" --openapi "$spec"
' _ {}

# tfgen writes into internal and examples. A diff that cannot compare also counts as a difference.
differences=""
for pair in "old-tree/internal new-tree/internal" "old-tree/examples new-tree/examples" "old-out new-out"; do
  read -r left right <<<"$pair"
  if [[ ! -e "$work/$left" && ! -e "$work/$right" ]]; then
    continue
  fi
  if ! found=$(diff -r --brief "$work/$left" "$work/$right" 2>&1); then
    differences+="$found"$'\n'
  fi
done
if [[ -n "$differences" ]]; then
  fail "The base and HEAD generators produce different output:" "${differences//$work\//}"
fi
