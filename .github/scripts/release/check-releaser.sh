#!/usr/bin/env bash
# Fails unless a release PR comes from the team that releases: the PR's author and the signer of
# each of its commits must be listed in the allowlist (.github/RELEASERS, the members of
# coralogix/team-eco-system). The repository is public, so without this anyone with write access
# could release.
#
# Usage: check-releaser.sh <pr-number>
# GH_TOKEN and GH_REPO read the PR and the allowlist.
set -euo pipefail

pr="$1"

# The allowlist as it is at the PR's base commit, which no commit of the PR can change, also
# after a rebase merge. The base must be on the base branch, so it can't be another branch's list.
base=$(gh pr view "$pr" --json baseRefName,baseRefOid --jq '"\(.baseRefName) \(.baseRefOid)"')
read -r base_ref base_sha <<< "$base"
status=$(gh api "repos/$GH_REPO/compare/$base_sha...$base_ref" --jq .status)
if [[ "$status" != identical && "$status" != ahead ]]; then
  echo "::error::The base $base_sha of PR #$pr is not on $base_ref ($status)."
  exit 1
fi
if ! allowlist=$(gh api "repos/$GH_REPO/contents/.github/RELEASERS?ref=$base_sha" -H "Accept: application/vnd.github.raw"); then
  echo "::error::.github/RELEASERS is missing at $base_sha, the base of PR #$pr."
  exit 1
fi
# GitHub logins are case-insensitive.
releasers=$(sed -e 's/#.*//' -e 's/[[:space:]]//g' <<< "$allowlist" | grep -v '^$' | tr '[:upper:]' '[:lower:]')
member() {
  grep -qxF "$(tr '[:upper:]' '[:lower:]' <<< "$1")" <<< "$releasers"
}

author=$(gh pr view "$pr" --json author --jq .author.login)
if ! member "$author"; then
  echo "::error::PR #$pr is by $author, who is not in .github/RELEASERS. Only coralogix/team-eco-system releases."
  exit 1
fi
# A committer counts only with a verified signature, which ties the commit to that account.
# A commit made on github.com (committer web-flow), such as "Update branch", stops the release:
# close the PR and open a new one with `make bump`.
# A separate assignment, so a failed listing stops the script instead of checking no commits.
oids=$(gh pr view "$pr" --json commits --jq '.commits[].oid')
if [[ -z "$oids" ]]; then
  echo "::error::PR #$pr lists no commits."
  exit 1
fi
for oid in $oids; do
  commit=$(gh api "repos/$GH_REPO/commits/$oid" \
    --jq '"\(.commit.verification.verified) \(.committer.login // "unknown")"')
  read -r verified committer <<< "$commit"
  if [[ "$verified" != true ]] || ! member "$committer"; then
    echo "::error::Commit $oid on PR #$pr is committed by $committer (signature verified: $verified). Every commit of a release PR must be signed by someone in .github/RELEASERS; close the PR and run \`make bump\` again."
    exit 1
  fi
done
echo "PR #$pr is from $author, and every commit is signed by someone in .github/RELEASERS."
