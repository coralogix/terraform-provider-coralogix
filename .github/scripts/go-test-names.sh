#!/usr/bin/env bash
# Reads `go test -v` output (or `go test -list` output with --list) on stdin.
# Writes one line per test: package, test name, and result, separated by tabs.
#
#   github.com/org/repo/internal/provider	TestAccFoo	PASS
#   github.com/org/repo/internal/provider	TestAccFoo/sub_case	SKIP
#
# Go prints the package only on the `ok` or `FAIL` line after its tests, so the
# names wait in a buffer until that line arrives. With --list, the result is LISTED.

set -euo pipefail

mode=ran
if [[ ${1:-} == --list ]]; then
  mode=list
fi

awk -v mode="$mode" '
  function flush(pkg,    i) {
    for (i = 1; i <= count; i++) {
      result = (names[i] in results) ? results[names[i]] : "NONE"
      printf "%s\t%s\t%s\n", pkg, names[i], result
    }
    count = 0
    delete results
  }
  ($1 == "ok" || $1 == "FAIL") && NF >= 2 && $2 ~ /\// { flush($2); next }
  mode == "list" && /^(Test|Example|Fuzz)[^ \t]*$/ {
    names[++count] = $0
    results[$0] = "LISTED"
    next
  }
  mode == "ran" && $1 == "===" && $2 == "RUN" { names[++count] = $3; next }
  mode == "ran" && $1 == "---" && $2 ~ /^(PASS|FAIL|SKIP):$/ {
    results[$3] = substr($2, 1, length($2) - 1)
    next
  }
  END { if (count > 0) flush("UNKNOWN_PACKAGE") }
'
