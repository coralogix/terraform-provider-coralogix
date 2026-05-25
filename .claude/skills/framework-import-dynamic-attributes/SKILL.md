---
name: framework-import-dynamic-attributes
description: "Use when a Terraform Plugin Framework resource import reads required DynamicAttribute values. Handle imported partial state before converting dynamic values."
---

# Framework Import Dynamic Attributes

**Trigger:** A framework resource uses `ImportStatePassthroughID` and a required `schema.DynamicAttribute`.

**Fix:** In `Read`, treat null, unknown, or zero-value dynamic attributes as absent imported state, fetch backend values, and only overwrite fetched state with prior dynamic state when the prior value is known.

**Why:** Terraform import initially supplies only the imported ID, so converting a missing dynamic value as if it came from config can fail before the backend read populates state.
