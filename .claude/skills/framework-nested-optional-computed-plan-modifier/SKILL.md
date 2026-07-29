---
name: framework-nested-optional-computed-plan-modifier
description: "Use when nested Optional+Computed list/set objects cause 'inconsistent result after apply' on add. Prefer UseNonNullStateForUnknown over UseStateForUnknown. Do NOT use for top-level IDs."
---

# Nested Optional+Computed Plan Modifier

**Trigger:** Adding a nested object (e.g. a dashboard widget) fails after apply with null→known: plan had null for an omitted Optional+Computed field, then flatten wrote a server value.

**Fix:** On nested Optional+Computed attributes that the API may assign (ids, defaults), use `UseNonNullStateForUnknown()` instead of `UseStateForUnknown()`. Keep unknown when prior state is null (new element); copy state only when non-null (existing element).

**Why:** On update, the framework plans omitted nested Optional+Computed as unknown. `UseStateForUnknown` copies null prior state for a new list/set element into the plan, so Terraform rejects the later known value from apply.
