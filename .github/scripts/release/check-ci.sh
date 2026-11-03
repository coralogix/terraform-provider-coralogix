#!/usr/bin/env bash
# Exits 0 only when every check on a release PR's commit has passed, except approve-release.yml's
# own ("Release PR: ..."). approve-release.yml runs it before approving and again right before
# merging: master requires only "Acceptance tests result", so a check that is re-run after the
# approval would otherwise not hold the merge.
#
# Usage: check-ci.sh <sha>
# GH_TOKEN and GITHUB_REPOSITORY read the checks.
set -euo pipefail

sha="$1"

# The latest run of every check on the commit: the API keeps the newest check of each name, so a
# re-run replaces the run it repeats.
runs=$(gh api --paginate "repos/$GITHUB_REPOSITORY/commits/$sha/check-runs?per_page=100" \
  --jq '.check_runs[] | select(.name | startswith("Release PR:") | not) | [.name, .status, (.conclusion // "")] | @tsv')
# The checks that sum up the others; until they report, CI has not finished.
# Not piped into grep -q: under pipefail, grep stopping early can fail cut.
names=$(cut -f1 <<<"$runs")
for required in "version-bump" "Acceptance tests result"; do
  if ! grep -qxF "$required" <<<"$names"; then
    echo "::notice::Waiting: '$required' has not reported yet"
    exit 1
  fi
done
pending=$(awk -F'\t' '$2 != "completed" {print $1}' <<<"$runs")
if [ -n "$pending" ]; then
  echo "::notice::Waiting for: $(paste -sd, - <<<"$pending")"
  exit 1
fi
failed=$(awk -F'\t' '$3 != "success" && $3 != "skipped" && $3 != "neutral" {print $1 " (" $3 ")"}' <<<"$runs")
if [ -n "$failed" ]; then
  echo "::warning::Not approving: failed checks: $(paste -sd, - <<<"$failed"). Re-run them; approval follows when they pass."
  exit 1
fi
echo "Every check on $sha has passed."
