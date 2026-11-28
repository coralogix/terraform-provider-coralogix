#!/usr/bin/env bash
# Fails when a generated resource cannot take a typical API change without a hand edit.
#
# `tfgen probe` changes the contract of each generated resource the way an API team does: a new
# optional field (string, bool, integer, or a new message) on every message that the resource uses,
# a new arm on every oneOf that it uses, and a new value on every enum that it uses. It keeps the overrides, the hooks, and the acceptance file as they are and
# runs the check of `tfgen check`. A change that the check rejects is a gap: the generator or the
# contract needs a fix, and not an edit of an override.
#
# The gaps that exist are in tools/iac-codegen/probe-baseline.txt, one line for each:
#   <generated package> <TAB> <change> <TAB> <message that breaks the resource>
# A line with the change "untested" names an attribute that no config of the acceptance test sets. A
# new field of the API must not join them: the generated test has to set it with no line in the
# acceptance file. A new line fails the check. A line that no longer fails also fails the check: delete it, so the
# baseline only shrinks.
#
# usage: check-future-changes.sh [--update-baseline]
# TFGEN names a prebuilt binary, SPEC names the OpenAPI file instead of the pinned SDK's, JOBS sets
# the number of resources that run at once (default 3), and PARALLEL the number of checks that run at
# once inside one resource (default 1). A changed dashboard needs up to 4 GB for one check: more at
# once ran a CI runner out of memory.
set -euo pipefail

scripts=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(git rev-parse --show-toplevel)
cd "$root"

baseline=tools/iac-codegen/probe-baseline.txt
update=false
[[ ${1:-} == --update-baseline ]] && update=true

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

binary=${TFGEN:-}
if [[ -z "$binary" ]]; then
  binary=$work/tfgen
  (cd tools/iac-codegen && go build -o "$binary" ./cmd/tfgen)
fi

spec=${SPEC:-}
if [[ -z "$spec" ]]; then
  go mod download
  spec=$(go list -m -f '{{.Dir}}' github.com/coralogix/coralogix-management-sdk)/openapi.yaml
fi

mkdir -p "$work/out"
probe_one() {
  local resource=$1 out=$2 dir=internal/provider/generated/$2
  local args=(--resource "$resource" --openapi "$spec" --parallel "${PARALLEL:-1}")
  [[ -f $dir/behavior-overrides.yaml ]] && args+=(--overrides "$dir/behavior-overrides.yaml")
  [[ -f $dir/acceptance.yaml ]] && args+=(--acceptance "$dir/acceptance.yaml")
  local result
  result=$("$binary" probe "${args[@]}" 2>"$work/out/$out.err" || true)
  if [[ -z $result ]]; then
    printf '%s\tERROR\t%s\n' "$out" "$(head -c 200 "$work/out/$out.err" | tr '\n\t' '  ')" >"$work/out/$out.txt"
    return
  fi
  awk -F'\t' -v out="$out" '$2 == "FAIL" { print out "\t" $1 "\t" $3 }' <<<"$result" >"$work/out/$out.txt"
  # The first check of the probe warns about an attribute that no config of a released resource sets.
  awk -v out="$out" '/no config sets [0-9]+ attributes/ { on = 1; next }
    on && /^  / { sub(/^  /, ""); print out "\tuntested\t" $0; next } { on = 0 }' "$work/out/$out.err" >>"$work/out/$out.txt"
}
export -f probe_one
export binary spec work

"$scripts/codegen-list.sh" | xargs -P "${JOBS:-3}" -L1 bash -c 'probe_one "$0" "$1"'
cat "$work"/out/*.txt | sort -u >"$work/current"

if $update; then
  { printf '%s\n' '# Gaps that tools/iac-codegen/scripts/check-future-changes.sh knows. One line for each:' \
      '# <generated package> <TAB> <change> <TAB> <message that breaks the resource>' \
      '# Only fix a gap and delete its line. Never add a line for a new gap: fix the generator or the contract.'
    cat "$work/current"
  } >"$baseline"
  printf 'Wrote %s gaps to %s\n' "$(wc -l <"$work/current" | tr -d ' ')" "$baseline"
  exit 0
fi

{ grep -v '^#' "$baseline" || true; } | sort -u >"$work/known"
new=$(comm -13 "$work/known" "$work/current" || true)
fixed=$(comm -23 "$work/known" "$work/current" || true)
if [[ -n "$new" ]]; then
  printf '%s\n' 'A generated resource cannot take a typical API change without a hand edit.' \
    'Fix the generator or the contract. Do not add the line to the baseline:' >&2
  printf '  %s\n' "$new" >&2
fi
if [[ -n "$fixed" ]]; then
  printf '%s\n' 'These gaps no longer fail. Delete their lines from tools/iac-codegen/probe-baseline.txt:' >&2
  printf '  %s\n' "$fixed" >&2
fi
if [[ -n "$new" || -n "$fixed" ]]; then
  exit 1
fi
printf 'Future-change probe OK (%s known gaps).\n' "$(wc -l <"$work/known" | tr -d ' ')"
