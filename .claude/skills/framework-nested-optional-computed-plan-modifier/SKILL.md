---
name: framework-nested-optional-computed-plan-modifier
description: "Use when nested Optional+Computed list/set add fails after apply with null→known. Prefer UseNonNullStateForUnknown for immutable/server-only ids only; verify set→remove before configurable defaults."
---

# Nested Optional+Computed Plan Modifier

**Trigger:** Adding a nested object fails after apply with null→known: plan had null for an omitted Optional+Computed field, then flatten wrote a server value.

**Fix:** For immutable/server-only nested values the API assigns (e.g. widget/`query_definitions`/aggregation `id`), use `UseNonNullStateForUnknown()` instead of `UseStateForUnknown()`. Keep unknown when prior state is null (new element); copy state only when non-null. Check every nested list level.

**Do not** apply this to configurable Optional+Computed defaults without a set → remove round-trip test. After non-null state exists, removing the attr from HCL plans unknown; `UseNonNullStateForUnknown` copies the old value back, so expand may resend it instead of clearing or restoring the backend default.

**Why:** On update, omitted nested Optional+Computed plans as unknown. `UseStateForUnknown` copies null prior state for a new list element, so Terraform rejects the later known apply value.
