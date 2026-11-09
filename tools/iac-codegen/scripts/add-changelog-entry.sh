#!/usr/bin/env bash
# Add one bullet under a section heading of the "# Unreleased" part of a changelog.
# Usage: add-changelog-entry.sh <changelog> <section heading> <bullet>
# The bullet goes after the last line of the section. The section is created right after
# "# Unreleased" when that part does not have it yet.
set -euo pipefail

if [[ $# -ne 3 ]]; then
  printf '%s\n' 'usage: add-changelog-entry.sh <changelog> <section heading> <bullet>' >&2
  exit 2
fi
changelog=$1 heading=$2 bullet=$3

if ! grep -qx '# Unreleased' "$changelog"; then
  printf '%s has no "# Unreleased" heading.\n' "$changelog" >&2
  exit 1
fi

out=$(mktemp)
trap 'rm -f "$out"' EXIT
# The first pass finds whether the Unreleased part has the section, the second writes the file.
# The heading and the bullet come from the environment: awk -v would turn "\\" into "\".
# Inside the section, blank lines wait until the next line: the bullet goes after the last
# non-blank line of the section, before the blank lines that end it.
HEADING=$heading BULLET=$bullet awk '
  BEGIN { heading = ENVIRON["HEADING"]; bullet = ENVIRON["BULLET"] }
  FNR == NR {
    if ($0 == "# Unreleased") { unreleased = 1; next }
    if (unreleased && /^# /) unreleased = 0
    if (unreleased && $0 == heading) found = 1
    next
  }
  $0 == "# Unreleased" {
    print
    unreleased = 1
    if (!found) { print ""; print heading; print bullet; done = 1 }
    next
  }
  unreleased && /^# / { unreleased = 0 }
  unreleased && !done && $0 == heading { print; insection = 1; next }
  insection && $0 == "" { blanks++; next }
  insection && /^#/ { print bullet; done = 1; insection = 0 }
  { for (; blanks > 0; blanks--) print "" }
  { print }
  END {
    if (insection) print bullet
    for (; blanks > 0; blanks--) print ""
  }
' "$changelog" "$changelog" >"$out"
cat "$out" >"$changelog"
