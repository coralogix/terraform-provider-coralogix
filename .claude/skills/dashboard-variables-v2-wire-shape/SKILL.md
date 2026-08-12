---
name: dashboard-variables-v2-wire-shape
description: "Use when expanding/flattening coralogix_dashboard variables_v2, or editing static values[].label defaults. Build typed SDK VariableV2 structs from flat TF models; wrap value/label, promql, dataprime, scope, and spans shapes."
---

# Dashboard `variables_v2` wire shape

**Trigger:** Editing `variables_v2` expand/flatten (`variables_v2.go`) or static `values[].label` schema/plan modifier (`dashboard_schema/variables_v2.go`).

**Fix:** Keep Terraform UX flat, then build typed SDK models (not `map[string]any` JSON):
- value arms → `SingleStringValue{Value: &StringValueLabel{...}}` (same for regex/lucene/interval/numeric)
- `id` string ↔ `UUID{Value: ...}` via `ExpandDashboardUUID`
- promql `query` string ↔ `PromQlQuery{Value: ...}` via `ExpandPromqlQuery`
- dataprime `query` string ↔ `CommonDataprimeQuery{Text: ...}`
- observation `scope` short name ↔ `DATASET_SCOPE_*` via existing enum maps
- spans `value = { type, value }` ↔ `SpanField` via `ExpandSpansField` / `FlattenSpansField`
- metrics operator `{type, selected_values}` ↔ `{equals|notEquals: {selection: {list: ...}}}`
- empty metrics `label_filters` → omit/nil (avoid null↔[] drift)
- empty `value_display_options`: schema `AtLeastOneOfChildren` rejects `{}` (Framework cannot plan null when config keeps the block; omit the block or set a regex)
- empty `multi_string.list.values` → flatten known empty list, not null (values is Required)
- enums (`display_type`, order, refresh, data_mode) → `OptionalEnumPointer` (never pointer-to-empty)
- static `values[].label`: Optional+Computed + `StaticValueLabelFromValue` (omit → plan=`value`). Flatten always stores API label (incl. label==value). Never `UseStateForUnknown` — needed for omit→set→omit and omit→value-change. Expand still copies `value` if label null/unknown.
On flatten, fill missing oneof siblings as null from schema AttrTypes (API omits unset arms).

**Why:** OpenAPI types nest wrappers and proto enums; flat TF fields must be wrapped into typed SDK structs or the API returns 400 / ObjectValueFrom fails. Label defaults via plan (not flatten-to-null) so omit, custom, explicit `label=value`, and value changes all round-trip without drift.
