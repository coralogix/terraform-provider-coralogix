#!/bin/bash
# Switches a handwritten resource to generated types (D21) in a copy of the
# provider, and checks that the switch changes nothing for users:
#   1. Copy the provider at HEAD (git archive) to a temporary directory.
#   2. Generate the types of <name>/switch.env with <name>/overrides.yaml.
#   3. Before the switch, with the handwritten code still there:
#      - dump the handwritten schema and the schema after the switch, and
#        compare them with cmd/schemacompare (no breaking difference);
#      - the equivalence test: the same API objects give the same state, and
#        the same state gives the same request, in the old and the new code;
#      - the plan test: an update plan with every computed attribute unknown
#        reads into the new model and expands (PlanWithUnknowns).
#   4. Apply <name>/provider.patch (the switch), then go vet and the provider
#      unit tests.
# New handwritten provider files of the switch are in <name>/add/*.go.txt
# (the suffix keeps them out of this module), and in its subdirectories for
# new packages. They are copied in before step 3, so the checks run the same
# code as the switched resource.
# The provider checkout is not changed. The patches were made against
# provider commit 752482ec.
#
# Run from the repository root: integration/switch.sh <name> <provider checkout>
# for example: integration/switch.sh globalrouter ../terraform-provider-coralogix
# With WORK_DIR=<dir>, the switched copy is written there and kept.
# With STEP=added, the types come from <name>/overrides-added.yaml, which adds
# the API fields that Terraform did not have. Then step 3 allows these
# additions, and only the plan test runs: the equivalence test cannot, the
# state has more attributes.
set -euo pipefail

NAME="${1:?usage: integration/switch.sh <name> <provider checkout>}"
PROVIDER="${2:?usage: integration/switch.sh <name> <provider checkout>}"
HERE="$(pwd)"
IT="$HERE/integration/$NAME"
# shellcheck source=/dev/null
source "$IT/switch.env"
if [ -n "${WORK_DIR:-}" ]; then
  WORK="$WORK_DIR"
  mkdir -p "$WORK"
else
  WORK="$(mktemp -d)"
  trap 'rm -rf "$WORK"' EXIT
fi
OVERRIDES="$IT/overrides.yaml"
TESTS='TestWriteSchemaDumps|Equivalence|PlanWithUnknowns'
if [ "${STEP:-switch}" = added ]; then
  OVERRIDES="$IT/overrides-added.yaml"
  TESTS='TestWriteSchemaDumps|PlanWithUnknowns'
fi

git -C "$PROVIDER" archive HEAD | tar -x -C "$WORK"
if [ -d "$IT/add" ]; then
  (cd "$IT/add" && find . -name '*.go.txt') | while read -r f; do
    mkdir -p "$WORK/$TEST_DIR/$(dirname "$f")"
    cp "$IT/add/$f" "$WORK/$TEST_DIR/${f%.txt}"
  done
fi
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --types "$ROOT" --tag "$TAG" \
  --overrides "$OVERRIDES" --out "$WORK/$PKG"

echo "== step 3: compare before the switch"
cp "$WORK/go.mod" "$WORK/go.mod.orig"
cp "$WORK/go.sum" "$WORK/go.sum.orig"
# The dump test imports package schemadump of this repository.
(cd "$WORK" && go mod edit -require=github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc@v0.0.0 -replace=github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc="$HERE")
cp "$IT/schema_dump_test.go.txt" "$WORK/$TEST_DIR/zz_schema_dump_test.go"
cp "$IT/equivalence_test.go.txt" "$WORK/$TEST_DIR/zz_equivalence_test.go"
DUMPS="$(mktemp -d)"
(cd "$WORK" && SCHEMA_DUMP_DIR="$DUMPS" GOFLAGS=-mod=mod go test "./$TEST_DIR" \
  -run "$TESTS" -count=1 -v | grep -E '^(--- |ok|FAIL)')
go run ./cmd/schemacompare "$DUMPS/handwritten.txt" "$DUMPS/generated.txt"
rm -rf "$DUMPS" "$WORK/$TEST_DIR/zz_schema_dump_test.go" "$WORK/$TEST_DIR/zz_equivalence_test.go"
mv "$WORK/go.mod.orig" "$WORK/go.mod"
mv "$WORK/go.sum.orig" "$WORK/go.sum"

echo "== step 4: switch"
cd "$WORK"
patch -s -p1 < "$IT/provider.patch"
go vet "./$TEST_DIR/..."
go test ./internal/...
