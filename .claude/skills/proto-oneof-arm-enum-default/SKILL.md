---
name: proto-oneof-arm-enum-default
description: "Use when an apply fails with 'at most one of [a, b, c] may be set', or when giving a schema Default to an enum whose proto field is a oneof arm."
---

# A oneof arm must never carry a schema Default

**Trigger:** Create/update fails with `json: error calling MarshalJSON for type *dashboard_service.X: at most one of [a, b, c] may be set`, or a union branch silently activates when the user set nothing.

**Fix:** Drop `Computed` + `Default: stringdefault.StaticString(utils.UNSPECIFIED)` from the attribute and make it plain `Optional`. Then guard the union: `ExactlyOneOfChildren(...)` on the parent object when the schema's attributes are exactly the oneof arms, or `stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("sibling"))` when the arms are mixed in with unrelated attributes (e.g. `heatmap.preset` / `heatmap.color_range`).

**Why:** `OptionalEnumPointer` maps `"unspecified"` to a non-nil pointer at the zero enum value, so a defaulted attribute is *always* populated. The generated SDK enforces oneof cardinality inside `MarshalJSON`, so the field never reaches the API — every sibling arm becomes unusable and the error names generated JSON fields, not the HCL path.

**Finding these:** the arm names come from the protos, not the Go types — `grep -A6 'oneof ' ../cx-management-apis/proto/com/coralogixapis/dashboards/v1/ast/widgets/*.proto` and cross-check each arm against the provider schema. Round-trip unit tests cannot catch this; only a real apply can, so every new union needs one acceptance step with a minimal config that omits the optional enums.
