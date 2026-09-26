#!/bin/bash
# Generates the fake SDK from spec/fake/*.yaml with the same steps,
# templates, and openapi-generator version as the pinned real SDK:
#   1. go/scripts/sanitize_openapi_for_go_codegen.py
#   2. openapi-generator (Docker), with the SDK templates and options
#   3. normalize_regex_validator_tags
# Only fakesdk/go/openapi/cxsdk is handwritten.
#
# Run from the repository root: fakesdk/generate.sh
set -euo pipefail

SDK_DIR="$(go list -m -f '{{.Dir}}' github.com/coralogix/coralogix-management-sdk)"
VERSION="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["generator-cli"]["version"])' "$SDK_DIR/openapitools.json")"
# shellcheck source=/dev/null
source "$SDK_DIR/go/scripts/openapi_generator_common.sh"

# generate_package makes one SDK package from one fake spec, like the real SDK
# makes one package per service.
generate_package() {
  local spec="$1" pkg="$2"
  local out="fakesdk/go/openapi/gen/$pkg"
  local work
  work="$(mktemp -d)"
  mkdir -p "$work/out"

  # The sanitizer reads JSON, like the split specs of the real SDK.
  python3 -c 'import json,sys,yaml; json.dump(yaml.safe_load(open(sys.argv[1])), open(sys.argv[2], "w"), indent=2)' \
    "$spec" "$work/source.json"
  python3 "$SDK_DIR/go/scripts/sanitize_openapi_for_go_codegen.py" "$work/source.json" "$work/openapi.json"

  docker run --rm \
    -v "$work:/work" \
    -v "$SDK_DIR/go/openapi/templates:/templates:ro" \
    "openapitools/openapi-generator-cli:v$VERSION" generate \
    -i /work/openapi.json \
    -g go \
    -o /work/out \
    --template-dir=/templates \
    --additional-properties=withGoMod=false,packageName="$pkg",enumClassPrefix=true,disallowAdditionalPropertiesIfNotPresent=false \
    --global-property=apiTests=false,modelTests=false,apiDocs=false,modelDocs=false

  rm -rf "$out"
  mkdir -p "$out"
  cp "$work"/out/*.go "$out/"
  rm -rf "$work"
  normalize_regex_validator_tags "$out"
}

generate_package spec/fake/openapi.yaml fake_boards_service
generate_package spec/fake/settings.yaml fake_settings_service
generate_package spec/fake/rules.yaml fake_rules_service
generate_package spec/fake/views.yaml fake_views_service
