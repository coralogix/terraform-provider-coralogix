---
name: frozen-prior-schema-must-not-share-helpers
description: "Use when adding an attribute to a coralogix_dashboard widget schema, to check no prior schema version (dashboard_schema/v1..v3) shares the helper you are editing. Prior schemas decode old state and must stay frozen."
---

# A frozen prior schema must not share a schema helper

**Trigger:** You are about to widen a schema helper such as `LineChartSchema()`. Before editing,
check who else calls it:

```bash
rg "LineChartSchema\(\)" internal/provider/dashboards/dashboard_schema
```

If a `v1.go`..`v3.go` calls it, your new attribute lands in that prior version too.

**Fix:** Give the prior version its own frozen copy and point it there. Name it for the version
it serves, keep it in the same package, and say in a comment that it must not be redirected back:

```go
// v3.go
"line_chart": dashboardwidgets.LineChartSchemaV3(),   // frozen snapshot, not the shared helper
```

Guard it with a test that counts the attribute in each schema version, asserting 0 for the frozen
one and the real count for the current one. Prove the freeze by diffing
`dashboardschema.V3().Type().TerraformType(ctx).String()` against `master`; it must be identical.

**Why:** Prior schema versions exist only to decode state written by older provider releases.
Widening one changes how that old state is interpreted. Most prior versions in this repo declare
widgets inline for exactly this reason — a shared helper is the exception, and it is the one that
bites. Duplication is correct here: a frozen snapshot is meant to drift from the current schema.
