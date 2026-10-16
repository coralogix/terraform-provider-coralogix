#!/usr/bin/env bash
# Decides whether the release-provider workflow may publish VERSION from SHA, the commit
# that set version.go. It looks at the merged PR of that commit.
#
#   check-release-pr.sh VERSION SHA
#
# Exit 0: publish. Exit 3: skip without an error (push of a PR that release-provider did
# not open). Exit 1: refuse.
#
# A release PR from release-provider needs an approval from someone other than the person
# who started the release, because the bot is its author and branch protection does not
# stop that person from approving it. A dispatch by a different maintainer also counts.
# A hand-made PR publishes only on dispatch; branch protection already kept its author
# from approving it.
#
# Reads GITHUB_REPOSITORY, GITHUB_EVENT_NAME and GITHUB_TRIGGERING_ACTOR; needs gh with
# pull-requests: read and actions: read.
set -euo pipefail

version=$1
sha=$2
workflow_path=.github/workflows/release-provider.yml
dispatch="it by dispatching release-provider with version $version"

pr=$(gh api "repos/$GITHUB_REPOSITORY/commits/$sha/pulls" \
  --jq '[.[] | select(.merged_at != null and .base.ref == "master")][0] // empty')
if [ -z "$pr" ]; then
  if [ "$GITHUB_EVENT_NAME" = push ]; then
    echo "::notice::$sha has no merged PR, so it does not publish. Publish $dispatch."
    exit 3
  fi
  echo "No merged PR for $sha; $GITHUB_TRIGGERING_ACTOR dispatched the publish."
  exit 0
fi
number=$(jq -r .number <<<"$pr")
head=$(jq -r .head.ref <<<"$pr")

starter=""
run_id=$(jq -r .body <<<"$pr" | sed -n 's/.*<!-- release-provider-run: \([0-9][0-9]*\) -->.*/\1/p' | head -1)
if [ -n "$run_id" ] && [ "$head" = "release/v$version" ]; then
  run=$(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$run_id" --jq '{path, actor: .triggering_actor.login}' 2>/dev/null || true)
  if [ -n "$run" ] && [ "$(jq -r .path <<<"$run")" = "$workflow_path" ]; then
    starter=$(jq -r .actor <<<"$run")
  fi
fi

if [ -z "$starter" ]; then
  if [ "$GITHUB_EVENT_NAME" = push ]; then
    echo "::notice::#$number was not opened by release-provider, so its merge does not publish. Publish $dispatch."
    exit 3
  fi
  echo "#$number is a hand-made release PR; $GITHUB_TRIGGERING_ACTOR dispatched the publish."
  exit 0
fi

# The latest approving or blocking review of each user counts; comments do not change it.
approvers=$(gh api --paginate "repos/$GITHUB_REPOSITORY/pulls/$number/reviews" \
    --jq '.[] | select(.user.type != "Bot") | [.user.login, .state] | @tsv' |
  awk -F'\t' '$2 != "COMMENTED" && $2 != "PENDING" { last[$1] = $2 }
    END { for (u in last) if (last[u] == "APPROVED") print u }' |
  grep -vxF "$starter" | sort | paste -sd, - || true)

if [ -n "$approvers" ]; then
  echo "#$number: $starter started the release, $approvers approved it."
  exit 0
fi
if [ "$GITHUB_EVENT_NAME" = workflow_dispatch ] && [ "$GITHUB_TRIGGERING_ACTOR" != "$starter" ]; then
  echo "#$number has no approval from anyone but $starter, who started the release; $GITHUB_TRIGGERING_ACTOR dispatched the publish."
  exit 0
fi
echo "::error::#$number has no approval from anyone other than $starter, who started the release. Another maintainer can publish $dispatch."
exit 1
