---
name: proto-size-impl-budget
description: "Use when reviewing or adding a Terraform resource. Estimate size from proto fields; flag 3x overruns or vendored generated clients. Do NOT use for small schema tweaks or dashboard/alert widget edits."
---

# Proto size vs implementation budget

**Trigger:** A new or large `resource_*.go` for one management API.

**Fix:** Count numbered proto fields for that API in the public `coralogix/cx-management-apis` repo. If protos are missing, count fields on the generated SDK structs. If neither exists, skip the ratio. Compare implementation lines (schema + expand + flatten + CRUD; skip tests):

- Expected: `7.5 × fields + 220`
- Typical band: about `4×fields+150` to `12×fields+290`

Flag only extremes: about **3× the midpoint** or more. Also flag if the PR vendors a generated OpenAPI or SDK client into this repo. In-band is not a pass; still read the code. Below-band is often a JSON blob (valid if that was the intent). Dashboard and alert are a deep-expand class (~17–21 lines/field); do not score other APIs against them. Use field count, not proto lines.

**Why:** Typical APIs are not linear enough for a linter. A large ratio still catches workarounds such as copied clients or expanding a small proto like a dashboard.
