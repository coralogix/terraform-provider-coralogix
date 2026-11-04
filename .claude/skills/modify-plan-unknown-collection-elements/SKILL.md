---
name: modify-plan-unknown-collection-elements
description: "Use when a resource ModifyPlan or plan modifier reads a list or set of nested objects with ElementsAs into Go structs. An unknown element fails the plan; walk types.Object values instead."
---

# ModifyPlan must tolerate unknown elements of a known collection

**Trigger:** `ModifyPlan` (or a plan modifier) calls `ElementsAs(ctx, &[]SomeModel{}, false)` on a configured, planned, or prior list or set of objects.

**Fix:** Read `Elements()` as `types.Object`, skip each element that is null or unknown, and read attributes with `obj.Attributes()["name"]`. To change a value, clone the attributes, set the new one, and rebuild with `types.ObjectValue` / `types.ListValue` / `types.SetValue`. Test it by calling `ModifyPlan` directly with `tftypes.NewValue(objType, tftypes.UnknownValue)` elements. The protocol baseline harness checks required attributes before it plans, so it never reaches the modifier with an unknown object.

**Why:** A whole-collection `IsUnknown()` guard misses `rules = [jsondecode(x.out)]`, where the list is known but an element is unknown. A Go struct cannot hold an unknown object, so the conversion adds a Value Conversion Error and the plan fails.
