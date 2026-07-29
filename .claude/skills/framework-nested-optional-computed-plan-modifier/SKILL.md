---
name: framework-nested-optional-computed-plan-modifier
description: "Use when adding a nested Optional+Computed list/set element fails after apply with null→known. Use UseNonNullStateForUnknown for server-generated ids at every level, not for configurable defaults."
---

# Nested Optional+Computed Plan Modifier

**Trigger:** Adding a nested object fails after apply with null→known: plan had null for an omitted Optional+Computed field, then flatten wrote a server value.

**Fix:** For immutable/server-only nested values the API assigns (e.g. widget/`query_definitions`/aggregation `id`), use `UseNonNullStateForUnknown()` instead of `UseStateForUnknown()`. Keep unknown when prior state is null (new element); copy state only when non-null. Check every nested list level.

**Do not** apply this to configurable Optional+Computed defaults without a set → remove round-trip test. After non-null state exists, removing the attr from HCL plans unknown; `UseNonNullStateForUnknown` copies the old value back, so expand may resend it instead of clearing or restoring the backend default.

**Fix every level, not just the leaf.** A new element can appear at any depth (new section, new row inside an existing section, new widget inside an existing row). Any ancestor level whose generated `id` still uses `UseStateForUnknown` fails the same way. Grep the schema for `UseStateForUnknown` on nested `Optional+Computed` ids and fix them together.

**Unit-testing this:** a fabricated `planmodifier.StringRequest` has a zero-value `State`, whose `Raw.IsNull()` is true, so `UseStateForUnknown` returns early and a "stays unknown" assertion passes with either modifier. Set `State` to a `tfsdk.State` with a non-null `Raw` (an empty object value is enough) to reproduce update behavior.

**Why:** On update, omitted nested Optional+Computed plans as unknown. `UseStateForUnknown` only skips when the whole prior state is null (create), so on update it copies the null prior state of a new list element into the plan, and Terraform rejects the later known apply value.
