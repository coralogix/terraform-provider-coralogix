#!/bin/bash
# Plugs the generated analytics alert types (D20, option C) into a copy of
# the Terraform provider, and runs the provider's alert tests there.
#   1. Copy the provider at HEAD (git archive) to a temporary directory.
#   2. Generate AnalyticsThresholdType and AnalyticsImmediateType into
#      internal/provider/alerts/generated.
#   3. Apply provider.patch: the handwritten plug-in (schema arms, model,
#      attribute types, expand and flatten, and a round-trip test).
#   4. go vet and go test the alert packages.
# The provider checkout is not changed. provider.patch was made against
# provider commit 752482ec.
#
# Run from the repository root: integration/alertsanalytics/run.sh <provider checkout>
set -euo pipefail

PROVIDER="${1:?usage: integration/alertsanalytics/run.sh <provider checkout>}"
HERE="$(pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

git -C "$PROVIDER" archive HEAD | tar -x -C "$WORK"
go run ./cmd/tfgen --spec spec/openapi.patched.yaml \
  --types AnalyticsThresholdType,AnalyticsImmediateType --tag "Alert definitions service" \
  --out "$WORK/internal/provider/alerts/generated"
cd "$WORK"
patch -s -p1 < "$HERE/integration/alertsanalytics/provider.patch"
go vet ./internal/provider/alerts/...
go test ./internal/provider/alerts/...
