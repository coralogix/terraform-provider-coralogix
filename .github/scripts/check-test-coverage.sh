#!/usr/bin/env bash
# Checks that the test jobs together ran every test exactly once.
#
# Usage: check-test-coverage.sh LISTED_FILE RAN_FILE...
#
# LISTED_FILE comes from `go test -list` through go-test-names.sh --list.
# Each RAN_FILE comes from one test job through go-test-names.sh. The file name
# is the job name.
#
# The check fails when:
# - a listed test did not run in any job, or
# - a test without subtests passed or failed in more than one job.
# A test that started and then called t.Skip counts as run for the first rule.
# The check proves that the job patterns select every test, not that no test skips.
# A skip does not count as a run for the second rule. Some tests skip in the
# catch-all job unless an env var is set, and run in another job that sets it.

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 LISTED_FILE RAN_FILE..." >&2
  exit 2
fi

listed_file=$1
shift

report=$(awk -F '\t' '
  FNR == 1 { job = FILENAME; sub(/^.*\//, "", job); sub(/\.txt$/, "", job) }
  NR == FNR { listed[$1 "\t" $2] = 1; next }
  {
    key = $1 "\t" $2
    ran[key] = 1
    # A test with subtests is a parent. Only the subtests decide duplicates.
    parent = $2
    while (sub(/\/[^\/]*$/, "", parent)) { is_parent[$1 "\t" parent] = 1 }
    if ($3 != "SKIP") {
      runs[key]++
      jobs[key] = (key in jobs) ? jobs[key] ", " job : job
    }
  }
  END {
    for (key in listed) {
      if (!(key in ran)) { split(key, k, "\t"); print "missing\t" k[2] "\t" k[1] }
    }
    for (key in runs) {
      if (runs[key] > 1 && !(key in is_parent)) {
        split(key, k, "\t"); print "duplicate\t" k[2] "\t" jobs[key]
      }
    }
  }
' "$listed_file" "$@" | sort)

if [[ -z $report ]]; then
  echo "Every listed test ran in a job, and no test ran in two jobs."
  exit 0
fi

missing=$(awk -F '\t' '$1 == "missing" { print $2 "  (" $3 ")" }' <<< "$report")
duplicates=$(awk -F '\t' '$1 == "duplicate" { print $2 "  (" $3 ")" }' <<< "$report")

{
  echo '## Acceptance test coverage check failed'
  if [[ -n $missing ]]; then
    echo
    echo 'No job ran these tests. Add each one to a shard pattern, or remove it from the patterns that skip it.'
    printf '\n```text\n%s\n```\n' "$missing"
  fi
  if [[ -n $duplicates ]]; then
    echo
    echo 'More than one job ran these tests. Change the shard patterns so that only one matches each test.'
    printf '\n```text\n%s\n```\n' "$duplicates"
  fi
} | tee -a "${GITHUB_STEP_SUMMARY:-/dev/null}"

exit 1
