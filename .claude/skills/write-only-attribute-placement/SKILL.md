---
name: write-only-attribute-placement
description: "Use when a secret sits in a set item or in a Computed nested attribute and needs a write-only (`_wo`) form. The Framework rejects a WriteOnly child there, so the value goes in a sibling map."
---

# Write-only attribute placement

**Trigger:** A secret is a child of a `SetNestedAttribute`, or of a nested attribute that is Computed, and you want a `_wo` attribute for it.

**Rule (terraform-plugin-framework):** A `SetNestedAttribute` cannot contain a WriteOnly descendant. A Computed nested attribute cannot contain one either. A List item may hold one only when no parent is Computed. A WriteOnly nested attribute needs all children WriteOnly.

**Fix:** Do not nest the new attribute. Put two attributes next to the nested one, outside the set and outside any Computed parent:

- `<thing>_wo`: a write-only map of string, keyed by the identity of the item (`<entity_type>.<field_name>` when one name is not unique).
- `<thing>_wo_versions`: an ordinary map of number with the same keys. It is kept in state and marks which keys are write-only.

Add them as `extraAttributes` of the resource (`elementType: string` and `elementType: int64`, `inUpdate: true` on the versions). In `BeforeWrite`, read the value from the config and merge it into the request. In `AfterRead`, drop the keys of the prior versions from the flattened value, and copy the versions from the prior value. Reject a key in both places in `ValidateConfig`.

**Why:** The API returns the secret on every read. Only the versions in the plan or state tell a read which values to keep out of state, or the next plan shows a permanent diff.
