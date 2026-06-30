#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

shard="${1:-}"
if [[ "$shard" =~ ^([A-Z]+)$ ]]; then
  start="${BASH_REMATCH[1]}"
  end="$start"
elif [[ "$shard" =~ ^([A-Z]+)-([A-Z]+)$ ]]; then
  start="${BASH_REMATCH[1]}"
  end="${BASH_REMATCH[2]}"
else
  echo "usage: $0 START-END or PREFIX, for example A-C, D-DAS, or DAT-DZ" >&2
  exit 2
fi

end_upper="${end}{"

if [[ ! "$start" < "$end_upper" ]]; then
  echo "invalid shard range: $shard" >&2
  exit 2
fi

normalized_file_key() {
  local base name

  base="$(basename "$1")"
  name="${base%_test.go}"
  name="${name#resource_}"
  name="${name#data_source_}"
  name="${name#coralogix_}"

  printf '%s' "$name"
}

file_bucket_key() {
  local key

  key="$(normalized_file_key "$1" | tr '[:lower:]' '[:upper:]')"

  case "$key" in
    [A-Z]*)
      printf '%s' "$key"
      ;;
    *)
      printf 'Z%s' "$key"
      ;;
  esac
}

in_shard_range() {
  local key="$1"

  [[ ( "$key" == "$start" || "$key" > "$start" ) && "$key" < "$end_upper" ]]
}

selected_files=()
selected_packages=()
selected_tests=()

while IFS= read -r file; do
  bucket_key="$(file_bucket_key "$file")"
  if ! in_shard_range "$bucket_key"; then
    continue
  fi

  file_tests=()
  while IFS= read -r test_name; do
    file_tests+=("$test_name")
  done < <(sed -nE 's/^func (Test[[:alnum:]_]*)\(.*$/\1/p' "$file")

  if (( ${#file_tests[@]} == 0 )); then
    continue
  fi

  selected_files+=("$file")
  selected_packages+=("$(dirname "$file")")
  selected_tests+=("${file_tests[@]}")
done < <(find . -name '*_test.go' -not -path './vendor/*' -print | sort)

if (( ${#selected_tests[@]} == 0 )); then
  echo "No tests found for shard $shard" >&2
  exit 1
fi

packages=()
while IFS= read -r package; do
  packages+=("$package")
done < <(printf '%s\n' "${selected_packages[@]}" | sort -u)

test_regex="$(printf '%s\n' "${selected_tests[@]}" | sort -u | paste -sd'|' -)"
test_count="$(printf '%s\n' "${selected_tests[@]}" | sort -u | wc -l | tr -d ' ')"

echo "Acceptance test shard $shard"
echo "Selected $test_count tests from ${#selected_files[@]} files in ${#packages[@]} packages."
printf '  %s\n' "${selected_files[@]}"

if [[ "${SHARD_DRY_RUN:-}" == "1" ]]; then
  echo "SHARD_DRY_RUN=1; not running go test."
  exit 0
fi

make testacc TEST="${packages[*]}" TESTARGS="-run '^(${test_regex})\$\$'"
