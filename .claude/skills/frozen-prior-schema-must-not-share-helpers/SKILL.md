---
name: frozen-prior-schema-must-not-share-helpers
description: "Use when widening a shared coralogix_dashboard widget schema helper like LineChartSchema(). Checks whether dashboard_schema/v1..v3 also use it; prior schemas must stay frozen."
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

`TestPriorSchemasStayFrozen` in
`internal/provider/dashboards/dashboard_schema/frozen_prior_schema_test.go` fingerprints V1 to V3
and fails on any change, including one made through a shared helper. Run it after touching any
helper; a failure means you widened a prior schema, not that the fingerprint is stale.

**Why:** Prior schema versions exist only to decode state written by older provider releases.
Widening one changes how that old state is interpreted. Most prior versions in this repo declare
widgets inline for exactly this reason — a shared helper is the exception, and it is the one that
bites. Duplication is correct here: a frozen snapshot is meant to drift from the current schema.
