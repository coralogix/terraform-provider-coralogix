#!/bin/bash
# Generates the fake SDK from spec/fake/openapi.yaml with the same steps,
# templates, and openapi-generator version as the pinned real SDK:
#   1. go/scripts/sanitize_openapi_for_go_codegen.py
#   2. openapi-generator (Docker), with the SDK templates and options
#   3. normalize_regex_validator_tags
# Only fakesdk/go/openapi/cxsdk is handwritten.
#
# Run from the repository root: fakesdk/generate.sh
set -euo pipefail

SDK_DIR="$(go list -m -f '{{.Dir}}' github.com/coralogix/coralogix-management-sdk)"
PKG="fake_boards_service"
OUT="fakesdk/go/openapi/gen/$PKG"
VERSION="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["generator-cli"]["version"])' "$SDK_DIR/openapitools.json")"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/out"

# The sanitizer reads JSON, like the split specs of the real SDK.
python3 -c 'import json,sys,yaml; json.dump(yaml.safe_load(open(sys.argv[1])), open(sys.argv[2], "w"), indent=2)' \
  spec/fake/openapi.yaml "$WORK/source.json"
python3 "$SDK_DIR/go/scripts/sanitize_openapi_for_go_codegen.py" "$WORK/source.json" "$WORK/openapi.json"

docker run --rm \
  -v "$WORK:/work" \
  -v "$SDK_DIR/go/openapi/templates:/templates:ro" \
  "openapitools/openapi-generator-cli:v$VERSION" generate \
  -i /work/openapi.json \
  -g go \
  -o /work/out \
  --template-dir=/templates \
  --additional-properties=withGoMod=false,packageName="$PKG",enumClassPrefix=true,disallowAdditionalPropertiesIfNotPresent=false \
  --global-property=apiTests=false,modelTests=false,apiDocs=false,modelDocs=false

rm -rf "$OUT"
mkdir -p "$OUT"
cp "$WORK"/out/*.go "$OUT/"

# shellcheck source=/dev/null
source "$SDK_DIR/go/scripts/openapi_generator_common.sh"
normalize_regex_validator_tags "$OUT"
