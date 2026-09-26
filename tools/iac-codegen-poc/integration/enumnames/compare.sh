#!/bin/bash
# Generates the enum names (F54) of every SDK package whose enums the
# provider's handwritten maps use, and compares them with the handwritten
# names (compare.py). The provider checkout is not changed.
#
# Run from the repository root: integration/enumnames/compare.sh <provider checkout>
set -euo pipefail

PROVIDER="${1:?usage: integration/enumnames/compare.sh <provider checkout>}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
SDK_DIR="$(go list -m -f '{{.Dir}}' github.com/coralogix/coralogix-management-sdk)"

# One line per SDK package: package|tag|enum components. The packages are the
# ones that the provider imports.
python3 - "$PROVIDER" "$SDK_DIR" > "$WORK/lists" <<'PY'
import glob, re, sys, yaml
provider, sdk = sys.argv[1], sys.argv[2]
used = set()
for f in glob.glob(provider + '/internal/provider/**/*.go', recursive=True):
    used |= set(re.findall(r'coralogix-management-sdk/go/openapi/gen/(\w+)"', open(f).read()))
doc = yaml.safe_load(open('spec/openapi.patched.yaml'))
tags = {t for item in doc['paths'].values() for op in item.values() if isinstance(op, dict) for t in op.get('tags', [])}
camel = lambda s: ''.join(p[:1].upper() + p[1:] for p in re.split(r'[^A-Za-z0-9]+', s) if p)
for pkg in sorted(used):
    tag = [t for t in tags if t.lower().replace(' ', '_') == pkg]
    types = set()
    for f in glob.glob(f'{sdk}/go/openapi/gen/{pkg}/model_*.go'):
        types |= set(re.findall(r'^type (\w+) string$', open(f).read(), re.M))
    enums = sorted(n for n, s in doc['components']['schemas'].items() if s.get('enum') and camel(n) in types)
    if tag and enums:
        print(pkg + '|' + tag[0] + '|' + ','.join(enums))
PY

while IFS='|' read -r pkg tag enums; do
  go run ./cmd/tfgen --spec spec/openapi.patched.yaml --enums "$enums" --tag "$tag" --out "$WORK/$pkg/e"
done < "$WORK/lists"
python3 integration/enumnames/compare.py "$PROVIDER" "$WORK"/*/e/enums.go
