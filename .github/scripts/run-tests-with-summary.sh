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

# The workflow uploads this file, and the test coverage job checks that
# every test ran in at least one job and passed or failed in at most one.
if [[ -n ${RAN_TESTS_FILE:-} ]]; then
  mkdir -p "$(dirname "$RAN_TESTS_FILE")"
  bash "$(dirname "$0")/go-test-names.sh" < "$test_output" > "$RAN_TESTS_FILE"
fi

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

# The workflow uploads this file, and the acceptance result job combines the
# files of all jobs into one summary. A passing job writes an empty file, so a
# re-run that passes replaces the failures of the earlier attempt.
if [[ -n ${FAILED_TESTS_FILE:-} ]]; then
  mkdir -p "$(dirname "$FAILED_TESTS_FILE")"
  if [[ $test_status -ne 0 ]]; then
    printf '%s\n' "${failed_tests:-The test command failed, but its output did not include failed test names.}" > "$FAILED_TESTS_FILE"
  else
    : > "$FAILED_TESTS_FILE"
  fi
fi

exit "$test_status"
