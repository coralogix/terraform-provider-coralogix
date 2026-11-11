---
name: plan-modifier-cannot-null-a-configured-empty-list
description: "Use when asked for a plan modifier that turns a configured empty list or set into null (NormalizeEmptyListToNull, a planEmptyAs key). Terraform core rejects it: Provider produced invalid plan."
---

# A plan cannot turn a configured `[]` into null

**Trigger:** An Optional attribute that is not computed has `x = []` in the configuration, and a plan modifier sets the plan to null.

**Proof:** `terraform plan` fails with `Provider produced invalid plan ... planned value cty.NullVal(...) does not match config value cty.ListValEmpty(...)`. For a nested list the text is `planned for absence but config wants existence`. It holds at the top level, in a list item, and for a `ListNestedAttribute`.

**Why:** Core accepts a plan that differs from a non-null configured value only when the plan equals a non-null prior value. A null plan for `[]` never qualifies, so the modifier cannot take effect.

**Fix:** Keep `[]` in the plan, and make the read give the plan back. `readEmptyAs: "null"` makes a response without values null, and generated state keeps the form (`[]` or null) that the plan or the prior state has. Do not add a plan-side key.

**Check:** A throwaway `resource.UnitTest` with a one-attribute provider shows it in seconds. Delete it afterwards.
