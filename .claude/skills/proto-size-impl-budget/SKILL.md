---
name: proto-size-impl-budget
description: "Use when reviewing or adding a Terraform resource. Estimate size from pinned SDK fields; flag 3x overruns or vendored clients. Do NOT use for small schema tweaks or dashboard/alert widget edits."
---

# SDK size vs implementation budget

**Trigger:** A new or large `resource_*.go` for one management API.

**Fix:** Count exported JSON-tagged fields on generated model structs in the **pinned** `coralogix-management-sdk` from `go.mod` (`go/openapi/gen/<service>`). Skip duplicated OpenAPI filter/error types. Do not use live proto HEAD. If the SDK has no types for that API, skip the ratio. Compare implementation lines (schema + expand + flatten + CRUD; skip tests):

- Expected: `7.8 × fields + 210`
- Typical band: about `4×fields+180` to `12×fields+250`

Flag only extremes: about **3× the midpoint** or more. Also flag if the PR vendors a generated OpenAPI or SDK client into this repo. In-band is not a pass; still read the code. Below-band is often a JSON blob (valid if that was the intent). Dashboard and alert are a deep-expand class (~22 lines/field); do not score other APIs against them.

**Why:** The pinned SDK is the surface this provider can implement. Typical APIs are not linear enough for a linter. A large ratio still catches workarounds such as copied clients or expanding a small API like a dashboard.
