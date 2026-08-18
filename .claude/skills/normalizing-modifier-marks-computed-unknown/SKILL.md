---
name: normalizing-modifier-marks-computed-unknown
description: "Use when a plan is never empty and an attribute the user never configured shows (known after apply) forever, next to a JSON/normalized attribute that shows as unchanged. Explains the framework rule and the two fixes."
---

# A Normalizing Plan Modifier Turns Other Computed Attributes Unknown

**Trigger:** `terraform plan` never comes back empty. It reports `~ some_attr = (known after apply)` for an attribute absent from HCL, while the attribute that actually differs renders as unchanged. Applying does not converge. In this repo it was `coralogix_dashboard.auto_refresh` next to `access_policy`.

**Cause:** `MarkComputedNilsAsUnknown` (`fwserver/server_planresourcechange.go`) runs when `PlannedState.Raw != PriorState.Raw` **anywhere**, and it replaces the value of every attribute that is `Computed`, null in config, and has no `Default` — **it never checks the planned value**, so a known value is overwritten too. It runs *before* attribute plan modifiers, so a modifier that hides the difference later (`PreserveStateForEquivalentJSON` pinning the state's JSON text) cannot undo the marking. State text and config text never converge, so the raw difference is permanent.

**Fix, pick one:**
- Root: let state adopt the config text, so proposed == prior after one apply. Costs a one-time cosmetic diff when only formatting or key order changed.
- Local: plan a known value for the collateral attribute — but only where the provider *guarantees* the post-apply value. `content_json` guarantees null via the flatten path, so `NullWhenContentJSONManaged` plans null. Where the backend decides, unknown is the only correct plan; a known value risks "Provider produced inconsistent result after apply".

**Do not** use plain `UseStateForUnknown()` as the local fix. It copies the prior value, so removing the attribute from HCL stops resetting it.

**Check first:** attributes with a `Default` and attributes with a restoring modifier (`id`, via `UseStateForUnknown`) are immune — that is why only one attribute appears in the diff.
