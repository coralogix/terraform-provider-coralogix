#!/usr/bin/env bash
# Print the generated resources of internal/provider/codegen.go, one per line, as "<RESOURCE> <OUT>".
# With --names, add the Terraform name: "<RESOURCE> <OUT> coralogix_<TypeName>". A line of
# codegen.go that names GET= and UPDATE= adds the generator flags after the last field:
# "--get-operation <id> --update-operation <id>". Runs in the repository of the current directory.
set -euo pipefail

names=0
if [[ ${1:-} == --names ]]; then
  names=1
elif [[ $# -ne 0 ]]; then
  printf '%s\n' 'usage: codegen-list.sh [--names]' >&2
  exit 2
fi

root=$(git rev-parse --show-toplevel)
sed -nE 's#^//go:generate make -C \.\./\.\. codegen RESOURCE=([^ ]+) OUT=([^ ]+)( GET=([^ ]+) UPDATE=([^ ]+))?$#\1 \2 \4 \5#p' \
  "$root/internal/provider/codegen.go" |
  while read -r resource out get update; do
    flags=
    [[ -z $get ]] || flags+=" --get-operation $get"
    [[ -z $update ]] || flags+=" --update-operation $update"
    if [[ $names -eq 0 ]]; then
      printf '%s %s%s\n' "$resource" "$out" "$flags"
      continue
    fi
    type_name=$(sed -nE 's/^const TypeName = "([^"]+)"$/\1/p' "$root/internal/provider/generated/$out/resource.go" 2>/dev/null || true)
    if [[ -z "$type_name" ]]; then
      printf 'internal/provider/generated/%s/resource.go has no TypeName constant.\n' "$out" >&2
      exit 1
    fi
    printf '%s %s coralogix_%s%s\n' "$resource" "$out" "$type_name" "$flags"
  done
