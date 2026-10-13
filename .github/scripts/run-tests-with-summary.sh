#!/usr/bin/env bash

set -uo pipefail

if [[ $# -eq 0 ]]; then
  echo "Usage: $0 command [args...]" >&2
  exit 2
fi

test_output=$(mktemp)
trap 'rm -f "$test_output"' EXIT

set +e
"$@" 2>&1 | tee "$test_output"
test_status=${PIPESTATUS[0]}
set -e

if [[ $test_status -ne 0 ]]; then
  failed_tests=$(sed -n 's/^--- FAIL: \([^ ]*\).*/\1/p' "$test_output" | sort -u)
  {
    echo '## Failed acceptance tests'
    if [[ -n $failed_tests ]]; then
      printf '\n```text\n%s\n```\n' "$failed_tests"
    else
      echo
      echo 'The test command failed, but its output did not include failed test names.'
    fi
  } >> "$GITHUB_STEP_SUMMARY"
fi

exit "$test_status"
