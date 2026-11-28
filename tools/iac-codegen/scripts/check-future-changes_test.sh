#!/usr/bin/env bash
set -euo pipefail

checker=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-future-changes.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# A fake generator. `probe` prints what $FAKE_PROBE holds, so a test chooses the gaps.
cat >"$work/tfgen" <<'FAKE'
#!/usr/bin/env bash
[[ $1 == probe ]] || exit 2
printf '%b' "${FAKE_WARN:-}" >&2
printf '%b' "${FAKE_PROBE:-}"
FAKE
chmod +x "$work/tfgen"
printf 'openapi: 3.1.0\n' >"$work/spec.yaml"

repo=$work/repo
mkdir -p "$repo/internal/provider/generated/thing" "$repo/tools/iac-codegen/scripts"
git -C "$repo" init -q
printf '//go:generate make -C ../.. codegen RESOURCE=Thing OUT=thing\n' >"$repo/internal/provider/codegen.go"
printf 'resource: Thing\n' >"$repo/internal/provider/generated/thing/behavior-overrides.yaml"
cp "$(dirname "$checker")/codegen-list.sh" "$repo/tools/iac-codegen/scripts/"
cp "$checker" "$repo/tools/iac-codegen/scripts/"
script=$repo/tools/iac-codegen/scripts/check-future-changes.sh
export TFGEN=$work/tfgen SPEC=$work/spec.yaml

run() { (cd "$repo" && bash "$script" "$@") >"$work/out" 2>&1; }
gap='string-field\tFAIL\tOuter\tRENDERER_SHAPE_UNSUPPORTED x\nenum-value\tok\n'

# No gaps and an empty baseline pass.
printf '# header\n' >"$repo/tools/iac-codegen/probe-baseline.txt"
FAKE_PROBE='string-field\tok\nenum-value\tok\n' run || { cat "$work/out" >&2; echo "no gaps must pass" >&2; exit 1; }

# A gap that the baseline does not hold fails.
if FAKE_PROBE=$gap run; then echo "a new gap must fail" >&2; exit 1; fi
grep -q -F $'thing\tstring-field\tOuter' "$work/out"

# --update-baseline writes it, and then the same gap passes.
FAKE_PROBE=$gap run --update-baseline
grep -q -F $'thing\tstring-field\tOuter' "$repo/tools/iac-codegen/probe-baseline.txt"
FAKE_PROBE=$gap run || { cat "$work/out" >&2; echo "a known gap must pass" >&2; exit 1; }

# A gap that is fixed must leave the baseline.
if FAKE_PROBE='string-field\tok\nenum-value\tok\n' run; then echo "a fixed gap must fail until its line is deleted" >&2; exit 1; fi
grep -q -F 'Delete their lines' "$work/out"

# A resource that fails before any change is an ERROR line, and it fails the check.
printf '# header\n' >"$repo/tools/iac-codegen/probe-baseline.txt"
if FAKE_PROBE='' run; then echo "a probe with no output must fail" >&2; exit 1; fi
grep -q -F $'thing\tERROR' "$work/out"

# An attribute that no config sets is a line of the baseline: a new one fails, a known one passes.
warn='tfgen: warning: x: acceptance test: no config sets 2 attributes that a user can set, so they would ship untested:\n  a.b\n  c[].d\n'
printf '# header\n' >"$repo/tools/iac-codegen/probe-baseline.txt"
if FAKE_WARN=$warn FAKE_PROBE='string-field\tok\n' run; then echo "a new untested attribute must fail" >&2; exit 1; fi
grep -q -F $'thing\tuntested\tc[].d' "$work/out"
FAKE_WARN=$warn FAKE_PROBE='string-field\tok\n' run --update-baseline
grep -q -F $'thing\tuntested\ta.b' "$repo/tools/iac-codegen/probe-baseline.txt"
FAKE_WARN=$warn FAKE_PROBE='string-field\tok\n' run || { cat "$work/out" >&2; echo "a known untested attribute must pass" >&2; exit 1; }
if FAKE_PROBE='string-field\tok\n' run; then echo "a tested attribute must leave the baseline" >&2; exit 1; fi
