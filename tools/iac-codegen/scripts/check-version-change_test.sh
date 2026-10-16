#!/usr/bin/env bash
set -euo pipefail

checker=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-version-change.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# fake_tfgen writes a fake generator. Its output depends on what it is given, and on $1, so two
# fakes with different text produce different output.
fake_tfgen() {
  local path=$1 text=$2
  cat >"$path" <<EOF
#!/usr/bin/env bash
case "\$1" in
  generate) mkdir -p "\$5" && printf '%s %s\n' "$text" "\$3" >"\$5/resource.go" ;;
  check) printf 'check %s %s\n' "$text" "\$3" ;;
esac
EOF
  chmod +x "$path"
}
fake_tfgen "$work/tfgen-a" a
fake_tfgen "$work/tfgen-b" b
printf 'openapi: 3.1.0\ncomponents:\n  schemas:\n    Thing:\n      type: object\n    Other:\n      type: object\n' >"$work/spec.yaml"
export SPEC=$work/spec.yaml

new_repo() {
  local repo=$1
  mkdir -p "$repo/tools/iac-codegen/internal/version" "$repo/tools/iac-codegen/internal/generator/testdata/golden" \
    "$repo/internal/provider/generated/thing"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name Test
  git -C "$repo" config commit.gpgsign false
  printf '//go:generate make -C ../.. codegen RESOURCE=Thing OUT=thing\n' >"$repo/internal/provider/codegen.go"
  printf 'resource: Thing\n' >"$repo/internal/provider/generated/thing/behavior-overrides.yaml"
  printf 'golden\n' >"$repo/tools/iac-codegen/internal/generator/testdata/golden/thing.go"
  printf 'package generator\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
  write_version "$repo" v0.1.0-beta.1
  commit_all "$repo" base
}

write_version() {
  printf 'package version\n\nconst Current = "%s"\n' "$2" >"$1/tools/iac-codegen/internal/version/version.go"
}

commit_all() {
  git -C "$1" add .
  git -C "$1" commit -qm "$2"
}

passes() {
  local name=$1 repo=$2 base=$3 old=$4 new=$5
  if ! (cd "$repo" && TFGEN_OLD=$old TFGEN_NEW=$new "$checker" "$base") >"$work/out.txt" 2>&1; then
    printf '%s failed unexpectedly:\n' "$name" >&2
    cat "$work/out.txt" >&2
    exit 1
  fi
}

fails() {
  local name=$1 repo=$2 base=$3 old=$4 new=$5 reason=$6
  if (cd "$repo" && TFGEN_OLD=$old TFGEN_NEW=$new "$checker" "$base") >"$work/out.txt" 2>&1; then
    printf '%s passed unexpectedly\n' "$name" >&2
    exit 1
  fi
  if ! grep -qF "$reason" "$work/out.txt"; then
    printf '%s failed without "%s":\n' "$name" "$reason" >&2
    cat "$work/out.txt" >&2
    exit 1
  fi
}

# A change of code with the same output needs no new version.
repo="$work/same-output"
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package generator\n\n// refactor\n' >"$repo/tools/iac-codegen/internal/generator/generator.go"
commit_all "$repo" refactor
passes "same output" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-a"

# A change of output needs a new version.
fails "changed output" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-b" "produce different output"
write_version "$repo" v0.1.0-beta.2
commit_all "$repo" new-version
passes "changed output with a new version" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-b"

# A comment next to the version is not a new version.
repo="$work/comment-only"
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf '// comment only\n' >>"$repo/tools/iac-codegen/internal/version/version.go"
commit_all "$repo" comment
fails "comment-only version edit" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-b" "is still v0.1.0-beta.1"

# The version must stay readable.
repo="$work/missing-version"
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'package version\n' >"$repo/tools/iac-codegen/internal/version/version.go"
commit_all "$repo" no-version
fails "missing version" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-a" "missing or invalid"

# A golden file of the generator tests is output of the generator.
repo="$work/golden"
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'new golden\n' >"$repo/tools/iac-codegen/internal/generator/testdata/golden/thing.go"
commit_all "$repo" golden
fails "changed golden file" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-a" "golden files changed"

# A change outside the generator, or only in its scripts, runs nothing.
repo="$work/outside"
new_repo "$repo"
base=$(git -C "$repo" rev-parse HEAD)
printf 'readme\n' >"$repo/README.md"
mkdir -p "$repo/tools/iac-codegen/scripts"
printf '# script\n' >"$repo/tools/iac-codegen/scripts/x.sh"
commit_all "$repo" outside
passes "change outside the generator" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-b"

# A new generator has no base to compare.
repo="$work/new-generator"
mkdir -p "$repo"
git -C "$repo" init -q
git -C "$repo" config user.email test@example.com
git -C "$repo" config user.name Test
git -C "$repo" config commit.gpgsign false
printf 'base\n' >"$repo/README.md"
commit_all "$repo" base
base=$(git -C "$repo" rev-parse HEAD)
mkdir -p "$repo/tools/iac-codegen/internal/version"
write_version "$repo" v0.1.0-beta.1
commit_all "$repo" add-generator
passes "new generator" "$repo" "$base" "$work/tfgen-a" "$work/tfgen-b"
