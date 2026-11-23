#!/usr/bin/env bash
set -euo pipefail

list=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/codegen-list.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

repo=$work/repo
mkdir -p "$repo/internal/provider/generated/thing" "$repo/internal/provider/generated/single"
git init -q -b master "$repo"
cat >"$repo/internal/provider/codegen.go" <<'GO'
package provider

//go:generate make -C ../.. codegen RESOURCE=Thing OUT=thing
//go:generate make -C ../.. codegen RESOURCE=SingleSettings OUT=single GET=Svc_GetSettings UPDATE=Svc_UpdateSettings
GO
printf 'package thing\n\nconst TypeName = "thing"\n' >"$repo/internal/provider/generated/thing/resource.go"
printf 'package single\n\nconst TypeName = "single_settings"\n' >"$repo/internal/provider/generated/single/resource.go"

expect() {
  local want=$1 got
  shift
  got=$(cd "$repo" && "$list" "$@")
  if [[ "$got" != "$want" ]]; then
    printf 'FAIL: codegen-list.sh %s printed:\n%s\nwant:\n%s\n' "$*" "$got" "$want" >&2
    exit 1
  fi
}

expect 'Thing thing
SingleSettings single --get-operation Svc_GetSettings --update-operation Svc_UpdateSettings'
expect 'Thing thing coralogix_thing
SingleSettings single coralogix_single_settings --get-operation Svc_GetSettings --update-operation Svc_UpdateSettings' --names
printf 'codegen-list.sh: ok\n'
