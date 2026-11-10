---
name: released-unknown-sent-as-zero
description: "Use when migrating a handwritten resource whose expand calls ValueBoolPointer/ValueInt64Pointer/ValueStringPointer on an Optional+Computed attribute. Unknown was sent as zero; keep it with sendUnknownAsZero."
---

# A released resource may send an unknown plan value as zero

**Trigger:** A handwritten expand reads an `Optional+Computed` attribute with `ValueXPointer()`. For an unknown value that returns a pointer to the zero value (`false`, `0`, `""`), not nil, so the request sends it on every Create and Update where the config omits the attribute.

**Check:** In the protocol baseline, look at the request bodies of a create or update that omits the attribute. `coralogix_parsing_rules` sent `enabled: false` (a group without `active` is created inactive) and `order: 0` (append last), and the acceptance tests pinned `active = false`.

**Fix:** Add `sendUnknownAsZero: true` (with `computed: true`) to the field line. Generated code otherwise leaves an unknown value out of the request, and the API applies its own default (`enabled: true`, `order: 1`), which changes existing resources on their next update.

**Why:** The framework's zero-valued unknown is invisible in schema and state dumps; only the request bodies show it.
