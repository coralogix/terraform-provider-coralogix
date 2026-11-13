#!/usr/bin/env bash

set -uo pipefail

if [[ $# -eq 0 ]]; then
  echo "Usage: $0 command [args...]" >&2
  exit 2
fi

# The public experiment uses a private team. Keep its IDs and test output out
# of Actions logs, summaries, and artifacts. Each job reads its own IDs.
if [[ ${PRIVATE_ACC_OUTPUT:-} == true ]]; then
  set +x
  umask 077
  if [[ ${CI_PREP_LOOKUP:-} == true ]]; then
    fixture_dir=$(mktemp -d)
    trap 'rm -rf "$fixture_dir"' EXIT
    if ! PREPARE_CI_TEAM_OUTPUT_FILE="$fixture_dir/fixtures.json" \
      env -u GITHUB_OUTPUT -u GITHUB_STEP_SUMMARY -u GITHUB_ENV -u GITHUB_PATH -u GITHUB_STATE \
      go -C sdk run ./tools/prepare-ci-team lookup 2>&1 | \
      python3 -u "$(dirname "$0")/redact-private-acceptance-output.py"; then
      echo 'Integration lookup failed'
      exit 1
    fi
    if ! fields=$(python3 - "$fixture_dir/fixtures.json" 2>/dev/null <<'PY'
import json
import re
import sys

with open(sys.argv[1], encoding="utf-8") as file:
    fixtures = json.load(file)
team_id = fixtures.get("teamId")
names = ("slackIntegrationId", "pagerDutyIntegrationId", "pagerDutyConnectorKey")
values = [fixtures.get(name) for name in names]
uuid = re.compile(r"[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\Z")
if (not isinstance(team_id, str) or
        not re.fullmatch(r"(?:0|[1-9][0-9]*)", team_id) or
        int(team_id) > 4294967295 or
        any(not isinstance(value, str) or not uuid.fullmatch(value) for value in values)):
    raise SystemExit(1)
print("\t".join([team_id, *values]))
PY
    ); then
      echo 'Integration lookup output was invalid'
      exit 1
    fi
    IFS=$'\t' read -r TEST_TEAM_ID SLACK_INTEGRATION_ID PD_INTEGRATION_ID PD_CONNECTOR_KEY <<< "$fields"
    export TEST_TEAM_ID SLACK_INTEGRATION_ID PD_INTEGRATION_ID PD_CONNECTOR_KEY
  fi
  if env -u GITHUB_OUTPUT -u GITHUB_STEP_SUMMARY -u GITHUB_ENV -u GITHUB_PATH -u GITHUB_STATE "$@" 2>&1 | \
    PRIVATE_TEST_NAMES=true python3 -u "$(dirname "$0")/redact-private-acceptance-output.py"; then
    echo 'Acceptance test passed'
    exit 0
  fi
  if [[ -n ${FAILED_TESTS_FILE:-} && ! -s ${FAILED_TESTS_FILE} ]]; then
    mkdir -p "$(dirname "$FAILED_TESTS_FILE")"
    printf 'Test command failed before reporting a test name\n' > "$FAILED_TESTS_FILE"
  fi
  echo 'Acceptance test failed'
  exit 1
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
