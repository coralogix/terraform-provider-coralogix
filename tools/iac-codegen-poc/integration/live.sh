#!/bin/bash
# Live check of a switch (D21). It needs a Coralogix API key and creates real
# objects, so a person runs it:
#   1. Build two provider binaries: the provider at HEAD ("old"), and the
#      switched copy of switch.sh ("new").
#   2. old: terraform apply <name>/live.tf.
#   3. new: terraform plan must show no changes.
#   4. Import LIVE_ADDRESS (<name>/switch.env) with the old and with the new
#      provider: the same state. Then both plan on it: the same plan. (An
#      import can leave a change to apply, for example a write-only secret
#      that the import wrote to state; the new provider must plan the same.)
#   5. new: change the description, apply, plan: no changes.
#   6. Destroy, also when a step fails.
#
# Run from the repository root:
#   CORALOGIX_ENV=EU2 integration/live.sh <name> <provider checkout>
# The key is read with a hidden prompt when CORALOGIX_API_KEY is not set.
# STEP=added checks the switch of STEP=added (switch.sh) in the same way.
set -euo pipefail

NAME="${1:?usage: integration/live.sh <name> <provider checkout>}"
PROVIDER="${2:?usage: integration/live.sh <name> <provider checkout>}"
HERE="$(pwd)"
# shellcheck source=/dev/null
source "$HERE/integration/$NAME/switch.env"
export CORALOGIX_ENV="${CORALOGIX_ENV:-EU2}"
if [ -z "${CORALOGIX_API_KEY:-}" ]; then
  printf 'Coralogix API key: '
  read -rs CORALOGIX_API_KEY
  echo
  export CORALOGIX_API_KEY
fi

TMP="$(mktemp -d)"
BIN="$TMP/bin"
TF="$TMP/tf"
mkdir -p "$BIN/old" "$BIN/new" "$TF"

echo "== build the old and the new provider"
git -C "$PROVIDER" archive HEAD | (mkdir -p "$TMP/old" && tar -x -C "$TMP/old")
(cd "$TMP/old" && go build -o "$BIN/old/terraform-provider-coralogix" .)
WORK_DIR="$TMP/new" "$HERE/integration/switch.sh" "$NAME" "$PROVIDER" > "$TMP/switch.log" 2>&1 ||
  { cat "$TMP/switch.log"; exit 1; }
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

# no_changes fails when terraform plans a change. Then it also writes the
# JSON paths that differ, which the plan text can hide.
no_changes() {
  local code=0
  terraform -chdir="$TF" plan -detailed-exitcode -input=false -no-color -out="$TMP/nc.plan" > "$TMP/plan.txt" || code=$?
  if [ "$code" != 0 ]; then
    cat "$TMP/plan.txt"
    explain_plan "$TMP/nc.plan"
    echo "FAIL: $1: the plan is not empty (exit $code)"
    exit 1
  fi
  echo "ok: $1: no changes"
}

# explain_plan writes, for each resource of the saved plan $1, the JSON paths
# that the refresh changed (drift) and that the plan changes.
explain_plan() {
  terraform -chdir="$TF" show -json "$1" | python3 -c '
import json, sys
plan = json.load(sys.stdin)
def walk(p, a, b, out):
    if isinstance(a, dict) and isinstance(b, dict):
        for k in sorted(set(a) | set(b)):
            walk(p + "." + k, a.get(k), b.get(k), out)
    elif isinstance(a, list) and isinstance(b, list) and len(a) == len(b):
        for i, (x, y) in enumerate(zip(a, b)):
            walk("%s[%d]" % (p, i), x, y, out)
    elif a != b:
        out.append("%s: %s -> %s" % (p, json.dumps(a), json.dumps(b)))
for kind in ("resource_drift", "resource_changes"):
    for c in plan.get(kind, []):
        ch = c["change"]
        out = []
        walk("", ch.get("before"), ch.get("after"), out)
        unknown = []
        walk("", {}, ch.get("after_unknown") or {}, unknown)
        if out or unknown:
            print("%s %s:" % (kind, c["address"]))
            for line in out:
                print("  " + line)
            for line in unknown:
                print("  unknown" + line.split(":", 1)[0])
'
}

# same_json reports whether the JSON $2 (new) has the values of $1 (old). With
# STEP=added, the new provider has more attributes by design, so only the
# attributes that the old one has are compared, at every level.
same_json() {
  python3 - "$1" "$2" "${STEP:-switch}" <<'PY'
import json, sys
old, new, step = json.loads(sys.argv[1]), json.loads(sys.argv[2]), sys.argv[3]
def restrict(n, o):
    if isinstance(n, dict) and isinstance(o, dict):
        return {k: restrict(n[k], o[k]) for k in o if k in n}
    if isinstance(n, list) and isinstance(o, list) and len(n) == len(o):
        return [restrict(a, b) for a, b in zip(n, o)]
    return n
if step == "added":
    new = restrict(new, old)
sys.exit(0 if new == old else 1)
PY
}

# resource_json writes the state values of LIVE_ADDRESS as JSON.
resource_json() {
  terraform -chdir="$TF" show -json | python3 -c '
import json, sys
state = json.load(sys.stdin)
for r in state["values"]["root_module"]["resources"]:
    if r["address"] == sys.argv[1]:
        print(json.dumps(r["values"], sort_keys=True))
' "$LIVE_ADDRESS"
}

# plan_json writes the planned resource changes as JSON.
plan_json() {
  terraform -chdir="$TF" plan -out="$TMP/p.plan" -input=false -no-color > /dev/null
  terraform -chdir="$TF" show -json "$TMP/p.plan" | python3 -c '
import json, sys
plan = json.load(sys.stdin)
print(json.dumps([[c["address"], c["change"]] for c in plan.get("resource_changes", [])], sort_keys=True))
'
}

import_with() {
  use "$1"
  terraform -chdir="$TF" state rm "$LIVE_ADDRESS" > /dev/null
  terraform -chdir="$TF" import -input=false -no-color "$LIVE_ADDRESS" "$ID" > "$TMP/import.txt" 2>&1 ||
    { cat "$TMP/import.txt"; exit 1; }
}

apply() {
  terraform -chdir="$TF" apply -auto-approve -input=false -no-color > "$TMP/apply.txt" 2>&1 ||
    { cat "$TMP/apply.txt"; exit 1; }
}

SUFFIX="iacpoc-$(date +%s)"
cleanup() {
  echo "== destroy"
  use old
  terraform -chdir="$TF" destroy -auto-approve -input=false -no-color > "$TMP/destroy.txt" 2>&1 ||
    { cat "$TMP/destroy.txt"; echo "WARNING: destroy failed; delete the objects named $SUFFIX by hand"; }
  rm -rf "$TMP"
}

write_config() {
  sed -e "s/@SUFFIX@/$SUFFIX/g" -e "s/@DESCRIPTION@/$1/g" "$HERE/integration/$NAME/live.tf" > "$TF/main.tf"
}

trap cleanup EXIT
write_config "created by the old provider"

echo "== old: apply"
use old
apply
no_changes "old provider after apply"

use new
no_changes "new provider on the old state"

echo "== import"
ID="$(terraform -chdir="$TF" state show -no-color "$LIVE_ADDRESS" | awk '$1 == "id" { gsub(/"/, "", $3); print $3 }')"
import_with old
OLD_STATE="$(resource_json)"
import_with new
if ! same_json "$OLD_STATE" "$(resource_json)"; then
  echo "old import: $OLD_STATE"
  echo "new import: $(resource_json)"
  echo "FAIL: the imported state differs"
  exit 1
fi
echo "ok: the old and the new provider import the same state"
use old
OLD_PLAN="$(plan_json)"
use new
if ! same_json "$OLD_PLAN" "$(plan_json)"; then
  echo "old plan: $OLD_PLAN"
  echo "new plan: $(plan_json)"
  echo "FAIL: the plans after the import differ"
  exit 1
fi
echo "ok: the old and the new provider plan the same after the import"

echo "== new: update"
write_config "updated by the new provider"
apply
no_changes "new provider after update"
echo "PASS"
