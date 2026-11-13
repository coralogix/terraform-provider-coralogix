---
name: missing-enum-stays-null
description: "Use when a missing enum reads as its first value, or a field with x-coralogix-presence flattens to a business value. Keep that reading on the field default."
---

# A missing enum stays null

**Trigger:** Flatten writes the first enum value for a nil pointer, or a field with `x-coralogix-presence` reads back as a business value.

**Fix:** Leave the generator reading a missing enum as null. Put the resource's reading on the field line as a string `default`. Do not apply that reading when the field has presence.

**Why:** Presence means the value was not set. A first-value fallback applies to every business-first enum, including those.

**Exception:** A released resource that read a missing enum as `unspecified`, for an enum the API fills in (`computed` with `useNonNullStateForUnknown`), needs the same reading, or a null state plans `(known after apply)` on every update. Put `readNullAs: unspecified` on that field line.
