#!/usr/bin/env bash
set -euo pipefail

script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/add-changelog-entry.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

expect() {
  local name=$1 want=$2
  if ! diff -u <(printf '%s' "$want") "$work/CHANGELOG.md"; then
    printf 'FAIL: %s\n' "$name" >&2
    exit 1
  fi
}

base='# Unreleased

#### resource/coralogix_alert
- FIX: one.

#### resource/coralogix_view
- FEAT: two.

# Release 1.0.0

#### resource/coralogix_slo_v2
- FIX: old.
'

printf '%s' "$base" >"$work/CHANGELOG.md"
"$script" "$work/CHANGELOG.md" '#### resource/coralogix_alert' '- CHORE: new.'
expect 'append to an existing section' '# Unreleased

#### resource/coralogix_alert
- FIX: one.
- CHORE: new.

#### resource/coralogix_view
- FEAT: two.

# Release 1.0.0

#### resource/coralogix_slo_v2
- FIX: old.
'

printf '%s' "$base" >"$work/CHANGELOG.md"
"$script" "$work/CHANGELOG.md" '#### resource/coralogix_slo_v2' '- CHORE: new.'
expect 'a released section gets a new Unreleased section' '# Unreleased

#### resource/coralogix_slo_v2
- CHORE: new.

#### resource/coralogix_alert
- FIX: one.

#### resource/coralogix_view
- FEAT: two.

# Release 1.0.0

#### resource/coralogix_slo_v2
- FIX: old.
'

printf '# Unreleased\n\n#### resource/coralogix_view\n- FEAT: two.\n' >"$work/CHANGELOG.md"
"$script" "$work/CHANGELOG.md" '#### resource/coralogix_view' '- CHORE: new.'
expect 'append to a section at the end of the file' '# Unreleased

#### resource/coralogix_view
- FEAT: two.
- CHORE: new.
'

printf '# Release 1.0.0\n' >"$work/CHANGELOG.md"
if "$script" "$work/CHANGELOG.md" '#### resource/coralogix_view' '- CHORE: new.' 2>/dev/null; then
  printf '%s\n' 'FAIL: a changelog without "# Unreleased" must fail' >&2
  exit 1
fi

printf '%s\n' 'add-changelog-entry tests passed'
