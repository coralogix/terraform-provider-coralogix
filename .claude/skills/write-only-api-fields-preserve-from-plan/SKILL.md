---
name: write-only-api-fields-preserve-from-plan
description: "Use when apply fails with 'Provider produced inconsistent result after apply: was <value>, but now null' for a field the API accepted. The backend validates the field but omits it from responses; preserve the planned value in flatten. Do NOT use when the API echoes the field back — that's a normal flatten bug."
---

# Write-only API fields: preserve from plan

**Trigger:** apply errors with `inconsistent result after apply: ... was cty.<value>, but now null` on a field the API accepted without error. Confirm with a direct API call: create with the field set, then GET — if the response omits it (or returns `[]`), the field is write-only on that backend. Example: `coralogix_alert.data_sources` — create validates dataset existence (400 on unknown), but create/GET responses return `dataSources: []`.

**Fix:** thread the planned/state value into the flatten and keep it when the API returns nothing. `flattenAlert` already does this for `schedule` and now `data_sources`:

```go
func flattenAlert(ctx ..., currentDataSources *types.List) ... {
    dataSources, _ := flattenDataSources(ctx, props.DataSources)
    if len(props.DataSources) == 0 && currentDataSources != nil && !currentDataSources.IsUnknown() {
        dataSources = *currentDataSources
    }
```

Pass `&plan.X` from Create/Update, `&state.X` from Read, `nil` from import/upgrade paths (nothing to preserve). Prefer the API value whenever it is non-empty so newer backends that do echo the field win.

**Why:** Terraform requires post-apply state to match the plan for non-computed attributes; when the backend can't be read back, the plan is the only source of truth — and echo behavior can differ per backend version (staging returned `dataSources` verbatim, EU2 prod returns `[]`).
