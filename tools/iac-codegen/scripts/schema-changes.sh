#!/usr/bin/env bash
# Compare the schema of two versions of a generated Terraform doc (docs/resources/*.md or
# docs/data-sources/*.md) and print one tab-separated line per attribute that changed:
#   added     <attribute>
#   removed   <attribute>
#   breaking  <attribute>  <before>  <after>
#   relaxed   <attribute>  <before>  <after>
# <attribute> is the dotted path of the attribute. <before> and <after> are its section and type,
# such as "Optional String". A change is breaking when the type changes, or when the attribute
# becomes Required or Read-Only: a configuration that was valid can then fail. A change from
# Required to Optional is relaxed. A description change prints nothing.
# Usage: schema-changes.sh <old doc> <new doc>. A missing file has no attributes.
set -euo pipefail
export LC_ALL=C

if [[ $# -ne 2 ]]; then
  printf '%s\n' 'usage: schema-changes.sh <old doc> <new doc>' >&2
  exit 2
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# attributes prints "<path>\t<section> <type>" for each attribute of a doc, sorted by path.
attributes() {
  if [[ ! -f $1 ]]; then
    return
  fi
  awk '
    /^## Schema$/ { schema = 1; next }
    !schema { next }
    /^### Nested Schema for `/ {
      prefix = $0
      sub(/^### Nested Schema for `/, "", prefix)
      sub(/`.*$/, "", prefix)
      prefix = prefix "."
      next
    }
    /^### (Required|Optional|Read-Only)$/ { section = $2; next }
    /^(Required|Optional|Read-Only):$/ { section = $0; sub(/:$/, "", section); next }
    /^- `[^`]+` \(/ {
      name = $0
      sub(/^- `/, "", name)
      sub(/`.*$/, "", name)
      type = $0
      sub(/^- `[^`]+` \(/, "", type)
      sub(/\).*$/, "", type)
      sub(/, Deprecated$/, "", type)
      printf "%s%s\t%s %s\n", prefix, name, section, type
    }
  ' "$1" | sort -t "$(printf '\t')" -k1,1
}

attributes "$1" >"$work/old"
attributes "$2" >"$work/new"

join -t "$(printf '\t')" -a 1 -a 2 -e - -o 0,1.2,2.2 "$work/old" "$work/new" |
  awk -F '\t' '
    $2 == "-" { print "added\t" $1; next }
    $3 == "-" { print "removed\t" $1; next }
    $2 == $3 { next }
    {
      split($2, before, " "); split($3, after, " ")
      old_type = substr($2, length(before[1]) + 2)
      new_type = substr($3, length(after[1]) + 2)
      kind = (old_type != new_type || after[1] == "Required" || after[1] == "Read-Only") ? "breaking" : "relaxed"
      print kind "\t" $1 "\t" $2 "\t" $3
    }
  '
