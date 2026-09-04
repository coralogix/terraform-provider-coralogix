#!/usr/bin/env python3
"""Manual workaround for the code generator's lack of `oneOf` support.

`tfplugingen-openapi` cannot map any schema that uses `oneOf`/`anyOf` composition,
and when a resource's root request/response schema uses it, the WHOLE resource is
dropped. Both alert_def_properties (16 oneOf subschemas) and ruleMatchers (4) do.

This script strips the root-level `oneOf`/`anyOf` wherever a sibling `properties`
map already exists, so the generator falls back to the flat object. This LOSES the
discriminated-union / mutual-exclusion semantics — it is only here to show the
best-case output achievable with spec surgery, not a correct schema.
"""
import json


def strip(obj):
    if isinstance(obj, dict):
        if ("oneOf" in obj or "anyOf" in obj) and "properties" in obj:
            obj.pop("oneOf", None)
            obj.pop("anyOf", None)
        for v in obj.values():
            strip(v)
    elif isinstance(obj, list):
        for v in obj:
            strip(v)


for src, dst in [
    ("alert_definitions_service.json", "alert_definitions_service.flat.json"),
    ("rule_groups_service.json", "rule_groups_service.flat.json"),
]:
    d = json.load(open(src))
    strip(d)
    json.dump(d, open(dst, "w"), indent=1)
    print("wrote", dst)
