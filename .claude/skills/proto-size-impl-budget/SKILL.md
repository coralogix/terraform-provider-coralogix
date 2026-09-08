---
name: proto-size-impl-budget
description: "Use when reviewing or adding a Terraform resource. Estimate size from pinned SDK fields vs all non-test Go; flag 3x overruns. Do NOT use for small schema tweaks or dashboard/alert widget edits."
---

# SDK size vs implementation budget

**Trigger:** A new or large resource for one management API.

**Fix:** Count exported JSON-tagged fields on generated model structs in the **pinned** `coralogix-management-sdk` from `go.mod` (`go/openapi/gen/<service>`). Count **this resource only**. If one gen package backs several resources (for example `policies_service` for three TCO resources), do not use the whole package. Use this resource’s types (name prefixes, or models reachable from its create/get/replace requests). Skip duplicated OpenAPI filter/error types. Do not use live proto HEAD. If the SDK has no types for that API, skip the ratio.

Count **all non-test, non-example `.go` lines** for **this resource**: its resource file, data source, and helpers. Skip tests, examples, and docs. If a generated client or helper is shared by several resources, do not add the whole shared pile to each one. Split those lines, or compare them to the combined SDK fields of every resource that uses them. A client copied in for this resource alone still counts here.

- Expected: `17 × fields`
- Typical band: about `10×` to `24×`

Flag only extremes: about **3× the midpoint** or more. A copied OpenAPI/SDK client is included in the line count, so it shows up here. In-band is not a pass; still read the code. Below-band is often a JSON blob (valid if that was the intent). Dashboard and alert sit in this band (~24–25×). Do not give them a separate formula.

**Why:** The pinned SDK is the surface this provider can implement. Typical APIs are not linear enough for a linter. Counting all Go, not only expand/flatten, is what catches a dumped generated client.
