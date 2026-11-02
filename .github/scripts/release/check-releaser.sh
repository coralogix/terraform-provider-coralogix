#!/usr/bin/env bash
# Fails unless a release PR comes from the team that releases: the PR's author and the signer of
# each of its commits must be listed in the allowlist (.github/RELEASERS, the members of
# coralogix/team-eco-system). The repository is public, so without this anyone with write access
# could release.
#
# Usage: check-releaser.sh <pr-number> <allowlist>
# Pass the allowlist as it is on master, never the PR's copy. GH_TOKEN and GH_REPO read the PR.
set -euo pipefail

pr="$1"
allowlist="$2"

# GitHub logins are case-insensitive.
releasers=$(sed -e 's/#.*//' -e 's/[[:space:]]//g' "$allowlist" | grep -v '^$' | tr '[:upper:]' '[:lower:]')
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
for oid in $(gh pr view "$pr" --json commits --jq '.commits[].oid'); do
  read -r verified committer < <(gh api "repos/$GH_REPO/commits/$oid" \
    --jq '"\(.commit.verification.verified) \(.committer.login // "unknown")"')
  if [[ "$verified" != true ]] || ! member "$committer"; then
    echo "::error::Commit $oid on PR #$pr is committed by $committer (signature verified: $verified). Every commit of a release PR must be signed by someone in .github/RELEASERS; close the PR and run \`make bump\` again."
    exit 1
  fi
done
echo "PR #$pr is from $author, and every commit is signed by someone in .github/RELEASERS."
