#!/usr/bin/env bash
# Fails when a change to the generator edits, renames, or deletes an existing test.
#
# A generator change must keep every existing test as it is. A test that has to change shows that
# the change altered released behavior. A new test is allowed, in a new file or added to the end of
# an existing test file: no line of an existing test may be removed or changed.
#
# The check reads these paths:
#   - every *_test.go file (generated acceptance tests included): only added lines are allowed;
#   - the generator test inputs and goldens: tools/iac-codegen/internal/**/testdata/**: no change;
#   - the acceptance inputs of generated resources: internal/provider/generated/*/acceptance.yaml:
#     no change.
#
# A change that does not touch the generator (tools/iac-codegen, apart from its scripts) is not
# checked: a normal resource change may fix its own test.
#
# A new golden or a new version forces a version bump, and a bump edits the version line of the
# existing goldens. So do not add a golden for new behavior: add a Go test that holds its input.
#
# usage: check-tests-unchanged.sh <base-commit>
set -euo pipefail

base=${1:?usage: check-tests-unchanged.sh <base-commit>}
root=$(git rev-parse --show-toplevel)
cd "$root"

changed=$(git diff --name-only "$base"...HEAD -- tools/iac-codegen | grep -Ev '^tools/iac-codegen/scripts/' || true)
if [[ -z "$changed" ]]; then
  exit 0
fi

# --no-renames shows a rename as a delete and an add, so a moved file is reported.
bad=""

# A test file: a deleted file, or a file with a removed or changed line.
while IFS=$'\t' read -r added removed path; do
  [[ -z "$path" ]] && continue
  if [[ "$removed" != "0" ]]; then
    bad+="  edited   $path (${removed} line(s) removed or changed)"$'\n'
  fi
done < <(git diff --numstat --no-renames "$base"...HEAD -- ':(glob)**/*_test.go')

# Any file of a golden, test input, or acceptance input: no change, no delete.
while read -r status path; do
  [[ -z "$path" ]] && continue
  bad+="  $([[ $status == D ]] && echo deleted || echo changed) $path"$'\n'
done < <(git diff --name-status --no-renames "$base"...HEAD -- \
  ':(glob)tools/iac-codegen/internal/**/testdata/**' \
  ':(glob)internal/provider/generated/*/acceptance.yaml' | grep -E '^[MD]' || true)

if [[ -n "$bad" ]]; then
  printf '%s\n' 'A generator change must not edit or delete an existing test, golden, or test input.' \
    'Keep the output of every existing input the same, and add new tests as new code:' >&2
  printf '%s' "$bad" >&2
  exit 1
fi
