---
name: proto-size-impl-budget
description: "Use when reviewing or adding a Terraform resource. Estimate size from pinned SDK fields vs all non-test Go; flag 3x overruns. Do NOT use for small schema tweaks or dashboard/alert widget edits."
---

# SDK size vs implementation budget

**Trigger:** A new or large resource for one management API.

**Fix:** Count exported JSON-tagged fields on generated model structs in the **pinned** `coralogix-management-sdk` from `go.mod` (`go/openapi/gen/<service>`). Skip duplicated OpenAPI filter/error types. Do not use live proto HEAD. If the SDK has no types for that API, skip the ratio.

Count **all non-test, non-example `.go` lines** for that API: resource, data source, helpers, and any generated client copied into this repo. Skip tests, examples, and docs. Compare:

- Expected: `17 × fields`
- Typical band: about `8×` to `23×`

Flag only extremes: about **3× the midpoint** or more. A copied OpenAPI/SDK client is included in the line count, so it shows up here. In-band is not a pass; still read the code. Below-band is often a JSON blob (valid if that was the intent). Dashboard and alert are a deep-expand class (~25 lines/field); do not score other APIs against them.

**Why:** The pinned SDK is the surface this provider can implement. Typical APIs are not linear enough for a linter. Counting all Go, not only expand/flatten, is what catches a dumped generated client.
