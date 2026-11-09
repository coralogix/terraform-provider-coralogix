#!/usr/bin/env bash
# Print the commit SHA of a ref of a remote repository.
# Usage: resolve-sdk-ref.sh <repository URL or path> <40-character SHA, branch, or tag>
# A branch is refs/heads/<ref> and a tag is refs/tags/<ref>, matched exactly: git ls-remote also
# matches the end of a longer ref such as refs/heads/feature/<ref>. An annotated tag resolves to
# the commit it points to, not to the tag object. A ref that names no commit, or a branch and a
# tag on different commits, fails.
set -euo pipefail

if [[ $# -ne 2 || -z $2 ]]; then
  printf '%s\n' 'usage: resolve-sdk-ref.sh <repository> <SHA, branch, or tag>' >&2
  exit 2
fi
repo=$1 ref=$2

if [[ $ref =~ ^[0-9a-f]{40}$ ]]; then
  printf '%s\n' "$ref"
  exit 0
fi
if [[ ! $ref =~ ^[A-Za-z0-9._/-]+$ ]]; then
  printf 'Invalid SDK ref: %s\n' "$ref" >&2
  exit 1
fi

refs=$(git ls-remote "$repo" "refs/heads/$ref" "refs/tags/$ref" "refs/tags/$ref^{}")
branch=$(awk -v name="refs/heads/$ref" '$2 == name { print $1 }' <<<"$refs")
tag=$(awk -v name="refs/tags/$ref^{}" '$2 == name { print $1 }' <<<"$refs")
if [[ -z "$tag" ]]; then
  tag=$(awk -v name="refs/tags/$ref" '$2 == name { print $1 }' <<<"$refs")
fi

commits=$(printf '%s\n%s\n' "$branch" "$tag" | sed '/^$/d' | sort -u)
case $(grep -c . <<<"$commits" || true) in
  1) printf '%s\n' "$commits" ;;
  0)
    printf 'SDK ref %s is neither a branch nor a tag.\n' "$ref" >&2
    exit 1
    ;;
  *)
    printf 'SDK ref %s is a branch and a tag on different commits. Pass the commit SHA.\n' "$ref" >&2
    exit 1
    ;;
esac
