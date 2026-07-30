---
name: framework-optional-computed-nested-object
description: "Use when Create fails with 'Received unknown value' on an Optional+Computed SingleNestedAttribute Path targeting a Go struct pointer. Model that field as types.Object."
---

# Optional+Computed nested object unknown on create

**Trigger:** `req.Plan.Get` fails with `Received unknown value… Path: <attr> Target Type: *…Model Suggested Type: basetypes.ObjectValue` when the nested block is omitted from HCL.

**Fix:** Store the attribute as `types.Object` in the resource model (not `*SomeModel`). Decode with `.As()` only when `!IsNull() && !IsUnknown()`. Flatten with `types.ObjectValueFrom` / `ObjectNull`.

**Why:** Omitted Optional+Computed nested attributes are unknown on create. Go struct pointers cannot represent unknown; `types.Object` can.
