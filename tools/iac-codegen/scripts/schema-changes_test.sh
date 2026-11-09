#!/usr/bin/env bash
set -euo pipefail

script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/schema-changes.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cat >"$work/old.md" <<'EOF'
# coralogix_thing (Resource)

- `outside` (String) A list item before the schema is not an attribute.

## Schema

### Required

- `name` (String) Name.
- `mode` (String) Mode.

### Optional

- `count` (Number) Count.
- `label` (String) Label.
- `size` (Number) Size.
- `nested` (Attributes) Nested. (see [below for nested schema](#nestedatt--nested))

### Read-Only

- `id` (String) ID.

<a id="nestedatt--nested"></a>
### Nested Schema for `nested`

Optional:

- `inner` (String) Inner.
- `gone` (Boolean) Gone.
EOF

cat >"$work/new.md" <<'EOF'
# coralogix_thing (Resource)

## Schema

### Required

- `name` (String) A new description.
- `label` (String) Label.

### Optional

- `count` (String) Count.
- `mode` (String) Mode.
- `size` (Number, Deprecated) Size.
- `nested` (Attributes) Nested. (see [below for nested schema](#nestedatt--nested))
- `added` (Boolean) Added.

### Read-Only

- `id` (String) ID.

<a id="nestedatt--nested"></a>
### Nested Schema for `nested`

Optional:

- `inner` (String) Inner.
- `extra` (List of String) Extra.
EOF

tab=$(printf '\t')
want="added${tab}added
breaking${tab}count${tab}Optional Number${tab}Optional String
breaking${tab}label${tab}Optional String${tab}Required String
relaxed${tab}mode${tab}Required String${tab}Optional String
added${tab}nested.extra
removed${tab}nested.gone"
got=$("$script" "$work/old.md" "$work/new.md")
if [[ "$got" != "$want" ]]; then
  printf 'FAIL: schema changes\nwant:\n%s\ngot:\n%s\n' "$want" "$got" >&2
  exit 1
fi

got=$("$script" "$work/missing.md" "$work/old.md" | awk -F '\t' '$1 != "added"')
if [[ -n "$got" ]]; then
  printf 'FAIL: a missing old doc must add every attribute, got:\n%s\n' "$got" >&2
  exit 1
fi

printf '%s\n' 'schema-changes tests passed'
