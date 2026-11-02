#!/usr/bin/env bash
# Fails unless a release PR comes from the team that releases the provider: the PR's author and
# the signer of each of its commits must be members of coralogix/team-eco-system. The repository
# is public, so without this anyone with write access could release.
#
# Usage: check-releaser.sh <pr-number>
# Env: GH_TOKEN and GH_REPO read the PR. RELEASE_TEAM_TOKEN reads the team; the default Actions
# token can't, so it is a secret with a token that has organization Members: read.
set -euo pipefail

org=coralogix
team=team-eco-system
pr="$1"

if [[ -z "${RELEASE_TEAM_TOKEN:-}" ]]; then
  echo "::error::RELEASE_TEAM_TOKEN is not set. Add a repository secret with a token that can read the members of $org/$team."
  exit 1
fi
if ! GH_TOKEN="$RELEASE_TEAM_TOKEN" gh api "orgs/$org/teams/$team" --silent; then
  echo "::error::RELEASE_TEAM_TOKEN cannot read $org/$team; it needs organization Members: read."
  exit 1
fi
member() {
  [[ "$(GH_TOKEN="$RELEASE_TEAM_TOKEN" gh api "orgs/$org/teams/$team/memberships/$1" --jq .state 2> /dev/null)" == active ]]
}

author=$(gh pr view "$pr" --json author --jq .author.login)
if ! member "$author"; then
  echo "::error::PR #$pr is by $author, who is not in $org/$team. Only that team releases."
  exit 1
fi
# A committer counts only with a verified signature, which ties the commit to that account.
# A commit made on github.com (committer web-flow), such as "Update branch", stops the release:
# close the PR and open a new one with `make bump`.
for oid in $(gh pr view "$pr" --json commits --jq '.commits[].oid'); do
  read -r verified committer < <(gh api "repos/$GH_REPO/commits/$oid" \
    --jq '"\(.commit.verification.verified) \(.committer.login // "unknown")"')
  if [[ "$verified" != true ]] || ! member "$committer"; then
    echo "::error::Commit $oid on PR #$pr is committed by $committer (signature verified: $verified). Every commit of a release PR must be signed by a member of $org/$team; close the PR and run \`make bump\` again."
    exit 1
  fi
done
echo "PR #$pr is from $author, and every commit is signed by a member of $org/$team."
