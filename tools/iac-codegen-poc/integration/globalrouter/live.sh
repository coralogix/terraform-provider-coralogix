#!/bin/bash
# Live check of the GlobalRouter switch (D21, E15 step O5). It needs a
# Coralogix API key and creates real objects, so a person runs it:
#   1. Build two provider binaries: the provider at HEAD ("old"), and the
#      switched copy of run.sh ("new").
#   2. old: terraform apply (a generic HTTPS connector and a router).
#   3. new: terraform plan must show no changes.
#   4. new: remove the router from the state, import it, plan: no changes.
#   5. new: change the description, apply, plan: no changes.
#   6. Destroy, also when a step fails.
#
# Run from the repository root:
#   CORALOGIX_ENV=EU2 integration/globalrouter/live.sh <provider checkout>
# The key is read with a hidden prompt when CORALOGIX_API_KEY is not set.
# STEP=added checks the switch of step O6 (run.sh) in the same way.
set -euo pipefail

PROVIDER="${1:?usage: integration/globalrouter/live.sh <provider checkout>}"
export CORALOGIX_ENV="${CORALOGIX_ENV:-EU2}"
if [ -z "${CORALOGIX_API_KEY:-}" ]; then
  printf 'Coralogix API key: '
  read -rs CORALOGIX_API_KEY
  echo
  export CORALOGIX_API_KEY
fi

HERE="$(pwd)"
TMP="$(mktemp -d)"
BIN="$TMP/bin"
TF="$TMP/tf"
mkdir -p "$BIN/old" "$BIN/new" "$TF"

echo "== build the old and the new provider"
git -C "$PROVIDER" archive HEAD | (mkdir -p "$TMP/old" && tar -x -C "$TMP/old")
(cd "$TMP/old" && go build -o "$BIN/old/terraform-provider-coralogix" .)
WORK_DIR="$TMP/new" "$HERE/integration/globalrouter/run.sh" "$PROVIDER" > "$TMP/run.log" ||
  { cat "$TMP/run.log"; exit 1; }
(cd "$TMP/new" && go build -o "$BIN/new/terraform-provider-coralogix" .)

# use makes terraform run the old or the new binary.
use() {
  cat > "$TMP/cli.tfrc" <<EOT
provider_installation {
  dev_overrides {
    "coralogix/coralogix" = "$BIN/$1"
  }
  direct {}
}
EOT
  export TF_CLI_CONFIG_FILE="$TMP/cli.tfrc"
}

# no_changes fails when terraform plans a change.
no_changes() {
  local code=0
  terraform -chdir="$TF" plan -detailed-exitcode -input=false -no-color > "$TMP/plan.txt" || code=$?
  if [ "$code" != 0 ]; then
    cat "$TMP/plan.txt"
    echo "FAIL: $1: the plan is not empty (exit $code)"
    exit 1
  fi
  echo "ok: $1: no changes"
}

cleanup() {
  echo "== destroy"
  use old
  terraform -chdir="$TF" destroy -auto-approve -input=false -no-color > "$TMP/destroy.txt" 2>&1 ||
    { cat "$TMP/destroy.txt"; echo "WARNING: destroy failed; delete the objects named $SUFFIX by hand"; }
  rm -rf "$TMP"
}

SUFFIX="iacpoc-$(date +%s)"
write_config() {
  cat > "$TF/main.tf" <<EOT
terraform {
  required_providers {
    coralogix = { source = "coralogix/coralogix" }
  }
}

resource "coralogix_connector" "http" {
  id          = "$SUFFIX"
  name        = "$SUFFIX"
  type        = "generic_https"
  description = "IaC codegen POC live check"
  connector_config = {
    fields = [
      { field_name = "url", value = "https://example.com/iac-codegen-poc" },
      { field_name = "method", value = "post" },
    ]
  }
}

resource "coralogix_global_router" "example" {
  name        = "$SUFFIX"
  description = "$1"
  routing_labels = {
    environment = "$SUFFIX"
  }
  entity_labels = { owner = "iac-codegen-poc" }
  rules = [
    {
      entity_type = "alerts"
      name        = "p1"
      condition   = "alertDef.priority == \"P1\""
      targets     = [{ connector_id = coralogix_connector.http.id }]
    },
  ]
  fallback_targets = [
    {
      entity_type = "alerts"
      target      = { connector_id = coralogix_connector.http.id }
    },
  ]
}
EOT
}

trap cleanup EXIT
write_config "created by the old provider"

echo "== old: apply"
use old
terraform -chdir="$TF" apply -auto-approve -input=false -no-color > "$TMP/apply.txt" || { cat "$TMP/apply.txt"; exit 1; }
no_changes "old provider after apply"

use new
no_changes "new provider on the old state"

echo "== new: import"
ID="$(terraform -chdir="$TF" state show -no-color coralogix_global_router.example | awk '$1 == "id" { gsub(/"/, "", $3); print $3 }')"
terraform -chdir="$TF" state rm coralogix_global_router.example > /dev/null
terraform -chdir="$TF" import -input=false -no-color coralogix_global_router.example "$ID" > /dev/null
no_changes "new provider after import"

echo "== new: update"
write_config "updated by the new provider"
terraform -chdir="$TF" apply -auto-approve -input=false -no-color > "$TMP/apply.txt" || { cat "$TMP/apply.txt"; exit 1; }
no_changes "new provider after update"
echo "PASS"
