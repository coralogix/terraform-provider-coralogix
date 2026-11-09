---
name: generated-state-upgrade-empty-collections
description: "Use when a generated resource upgrades old state by refresh (schema.upgrade strategy refresh) and the first plan after the upgrade shows labels or lists changing from [] or {} to null. Covers readEmptyAs and a flatten hook."
---

# A refresh upgrade has no prior value, so empty collections read as `[]`

**Trigger:** the upgraded state holds `[]` or `{}` where the plan (configuration without the attribute) holds null, and the first plan after the upgrade is not empty. A read with a prior value does not show it, because `keepPriorEmpty` keeps the null of the prior state.

**Fix:** state the null on the field: `readEmptyAs: "null"` for a list, a set, or a list of objects. A plain string map has no such key, so set it null in `AfterFlatten` when the attribute rejects an empty value (`sizeAtLeast: 1`), so the empty map cannot be configured. Test it: upgrade a raw state of every prior version through the protocol harness (`UpgradeFrom`), compare it with a read of the same alert, and plan the configuration on it (`internal/provider/alert_state_upgrade_test.go`).

**Why:** the upgrade passes no prior state to the flatten step, and the API answers an empty collection for a field that the user never set.
