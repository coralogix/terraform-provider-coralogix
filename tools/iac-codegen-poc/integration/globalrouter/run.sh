#!/bin/bash
# Switches the handwritten coralogix_global_router resource to generated
# types (D21, E15) in a copy of the provider, and checks that the switch
# changes nothing for users:
#   1. Copy the provider at HEAD (git archive) to a temporary directory.
#   2. Generate the GlobalRouter types with overrides.yaml into
#      internal/provider/notifications/globalroutertypes.
#   3. Before the switch, with the handwritten code still there:
#      - dump the handwritten and the generated schema, and compare them
#        with cmd/schemacompare (no breaking difference);
#      - equivalence_test: the same API objects give the same state, and the
#        same state gives the same request, in the old and the new code.
#   4. Apply provider.patch: the resource, the data source, and the V1 schema
#      use the generated types; the handwritten models, expand, and flatten
#      are removed. Then go vet and the provider unit tests.
# The provider checkout is not changed. provider.patch was made against
# provider commit 752482ec.
#
# Run from the repository root: integration/globalrouter/run.sh <provider checkout>
# With WORK_DIR=<dir>, the switched copy is written there and kept.
# With STEP=added (step O6), the types come from overrides-added.yaml, which
# adds the API fields that Terraform did not have. Then step 3 allows these
# additions, and the equivalence test does not run: the state has more
# attributes.
set -euo pipefail

PROVIDER="${1:?usage: integration/globalrouter/run.sh <provider checkout>}"
HERE="$(pwd)"
IT="$HERE/integration/globalrouter"
if [ -n "${WORK_DIR:-}" ]; then
  WORK="$WORK_DIR"
  mkdir -p "$WORK"
else
  WORK="$(mktemp -d)"
  trap 'rm -rf "$WORK"' EXIT
fi
PKG="$WORK/internal/provider/notifications"
OVERRIDES="$IT/overrides.yaml"
TESTS='TestWriteSchemaDumps|TestEquivalence'
if [ "${STEP:-switch}" = added ]; then
  OVERRIDES="$IT/overrides-added.yaml"
  TESTS='TestWriteSchemaDumps'
fi

git -C "$PROVIDER" archive HEAD | tar -x -C "$WORK"
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --types GlobalRouter --tag "Global routers service" \
  --overrides "$OVERRIDES" --out "$PKG/globalroutertypes"

echo "== step 3: compare before the switch"
cp "$WORK/go.mod" "$WORK/go.mod.orig"
cp "$WORK/go.sum" "$WORK/go.sum.orig"
# The dump test imports package schemadump of this repository.
(cd "$WORK" && go mod edit -require=github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc@v0.0.0 -replace=github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc="$HERE")
cp "$IT/schema_dump_test.go.txt" "$PKG/schema_dump_test.go"
cp "$IT/equivalence_test.go.txt" "$PKG/equivalence_test.go"
DUMPS="$(mktemp -d)"
(cd "$WORK" && SCHEMA_DUMP_DIR="$DUMPS" GOFLAGS=-mod=mod go test ./internal/provider/notifications \
  -run "$TESTS" -count=1 -v | grep -E '^(--- |ok|FAIL)')
go run ./cmd/schemacompare "$DUMPS/handwritten.txt" "$DUMPS/generated.txt"
rm -rf "$DUMPS" "$PKG/schema_dump_test.go" "$PKG/equivalence_test.go"
mv "$WORK/go.mod.orig" "$WORK/go.mod"
mv "$WORK/go.sum.orig" "$WORK/go.sum"

echo "== step 4: switch"
cd "$WORK"
patch -s -p1 < "$IT/provider.patch"
go vet ./internal/provider/notifications/...
go test ./internal/...
