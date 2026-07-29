---
name: framework-nested-optional-computed-plan-modifier
description: "Use when adding a nested Optional+Computed list/set element fails after apply with null→known. Use UseNonNullStateForUnknown for provider- or backend-generated ids, not configurable defaults."
---

# Nested Optional+Computed Plan Modifier

**Trigger:** Adding a nested object fails after apply with null→known: the plan had null for an omitted Optional+Computed field, then flatten wrote a known value. Where the value came from does not matter. Expand may mint it locally (a `uuid.NewString()` fallback when the id is null, as `ExpandDashboardUUID`/`ExpandDashboardIDs` do) or the backend may assign it. Both break the plan the same way.

**Fix:** For immutable ids the user did not configure, use `UseNonNullStateForUnknown()` instead of `UseStateForUnknown()`. Keep unknown when prior state is null (new element); copy state only when non-null. Apply it at every list level, not just the leaf: a new element can appear at any depth (new section, new row in an existing section, new widget in an existing row), and any level still on `UseStateForUnknown` fails the same way. Grep the schema for `UseStateForUnknown` on nested Optional+Computed ids and fix them together.

**Do not** apply this to configurable Optional+Computed defaults without a set → remove round-trip test. After non-null state exists, removing the attr from HCL plans unknown; `UseNonNullStateForUnknown` copies the old value back, so expand may resend it instead of clearing or restoring the backend default.

**Unit-testing this:** a fabricated `planmodifier.StringRequest` has a zero-value `State`, whose `Raw.IsNull()` is true, so `UseStateForUnknown` returns early and a "stays unknown" assertion passes with either modifier. Set `State` to a `tfsdk.State` with a non-null `Raw` (an empty object value is enough) to reproduce update behavior.

**Why:** On update, an omitted nested Optional+Computed attribute plans as unknown, and a list index beyond the prior state's length gets a null state value. `UseStateForUnknown` only skips when the whole prior state is null (create), so on update it copies that null into the plan and Terraform rejects the known value written after apply.
