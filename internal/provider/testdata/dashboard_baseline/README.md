# Dashboard protocol baseline

These files record what the handwritten `coralogix_dashboard` resource and data source do today.
They are made on `master`, before the resource becomes generated code.
After the switch, the golden diff must show only changes that someone wanted.

All values are synthetic. Random ids are masked as `<id-N>`, in order of first appearance.

## What is in this folder

| Files | Test | What they hold |
|---|---|---|
| `schema_current.txt`, `datasource_schema.txt` | `TestDashboardBaselineSchema`, `TestDataSourceBaselineSchema` | The schema as Terraform core receives it. |
| `scenario_*.json` (76) | `TestDashboardBaselineScenarios` | One hand-written scenario each: plan, requests, state, drift, replan. |
| `walk_*.json` (46) | `TestDashboardBaselineWalk` | The schema walk. One file for each base config. |
| `datasource_*.json` (57) | `TestDataSourceBaselineScenarios/dashboard` | The data source. One read for each walk base, and fixed scenarios. |
| `fidelity/cases.json`, `fidelity/recorded.json` | `TestDashboardFakeMatchesRecordedBackend` | Requests, and what EU2 answered. |

## How to record and check

```bash
# Scenarios, schema, data source (about 1 minute)
UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestDashboardBaseline(Schema|Scenarios)|TestDataSourceBaseline(Schema|Scenarios)/dashboard'

# The full walk (about 15 minutes for 8 bases; run groups of bases in parallel processes)
DASHBOARD_WALK=all UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestDashboardBaselineWalk/^(gauge_logs|gauge_metrics)$' -timeout 4h

# The dashboard acceptance tests against the fake, with no network and a placeholder key
DASHBOARD_FAKE_ACC=1 TF_ACC=1 CORALOGIX_API_KEY=placeholder CORALOGIX_DOMAIN=eu2.coralogix.com \
  go test ./internal/provider -run 'TestAcc.*Dashboard' -skip 'Folder|Grafana|Hosted' -timeout 90m
```

A plain `go test` walks only the `markdown` and `reference` bases and skips the other bases. Without `UPDATE_GOLDEN` the tests compare with the files.
Run an update twice. The second run must change nothing.

## The fake

The fake of the dashboards API is in `dashboard_baseline_fake_test.go`. Each of its rules has a source.

| Source | Rules |
|---|---|
| Recording on EU2 (`fidelity/recorded.json`) | A dashboard gets a default access policy. A policy that names some permissions has the others set to `deny`. A Replace without a policy keeps it. A blank policy reads back blank. Ids have 21 characters on every route. A folder id that no folder has is a 400. `folderPath` reads back as `folderId`. Two arms of a oneof are a 400. Error texts. An unset widget `highlighted` reads back `false`. `dataprimeQuery: {}` reads back with `text: ""`. |
| Acceptance tests (pass on the real API) | `LineChart.xAxisTimeFormat` is not filled. An empty time bucket `auto` keeps `minimumInterval` out. |
| Probe on EU2 | An omitted list reads back as `[]`. An omitted enum reads back as its first value, unless it is the arm of a oneof. Order is kept. Create does not check query syntax. |

The fake reads the SDK source for enum zero values and oneof groups. An SDK bump can change them.
The recording covers one widget in detail. Read-back rules for other widgets come from the probe and the acceptance tests only.

## Coverage

| Area | Where |
|---|---|
| Lifecycle, import, duplicate names | `minimal`, `full-lifecycle`, `import*`, `duplicate-names` |
| `content_json` | `content-json-*` (7 fixture files, invalid text, unknown, switch to and from HCL, import, folder, `auto_refresh`) |
| `access_policy` | `access-policy*` (set, reordered, changed, removed, blank, changed outside, import, with `content_json`, unknown) |
| Top-level fields | `folder-*`, `time-frame`, `auto-refresh`, `filters`, `annotations`, `variables-v2` |
| Deprecated fields | `variables-legacy*`, `widget-width`, and the walk bases `top_level` and `dynamic_*` (see below) |
| Plan-time Check | `check-*` |
| Missing resource | `read-not-found`, `update-not-found`, `delete-not-found`, `route-missing` |
| API and transport errors | `*-api-error`, `create-rejected`, `transport-*` |
| Bad 200 replies | `create-missing-id`, `create-empty-id`, `read-null-dashboard`, `read-empty-reply` |
| Create cleanup | `create-get-fails`, `create-get-fails-cleanup-fails`, `create-flatten-fails` |
| Unknown values | `unknown-*` |
| Config validation | `config-validation` |
| State upgrade | `upgrade-v1`, `upgrade-v2`, `upgrade-v3`, and `*-missing` |
| Every schema path | The walk. `TestDashboardWalkReachesEverySchemaPath` fails and names a path that no base reaches. |

### The walk

A generator fills every configurable schema path with a valid value. It reads enum values, ranges and one-of groups from the validators.
There are 46 bases: 7 widget kinds with each query arm, each dynamic visualization, a widget reference, the markdown widget and the top level.
Each base is applied, then changed in batches: remove optional values, remove optional objects, empty lists, set enums.
A failing batch is split until one change is named.
A walk file holds the base request and state once. After that, a step holds only what changed.

Two paths no base reaches: `variables[].definition.constant_value` (the API rejects it; see `variables-legacy-constant-value`) and `stat_card.primary_value.mapped_values` (a config validator rejects it).

### Deprecated fields

| Field | Replacement | Golden |
|---|---|---|
| `variables` | `variables_v2` | `variables-legacy`, `datasource_read-deprecated-variables`, `walk_top_level` |
| `variables[].definition.constant_value` | `multi_select` with `constant_list` | `variables-legacy-constant-value` (apply fails with an error text) |
| `widgets[].width` | none (the API ignores it) | `widget-width` |
| `dynamic.interpretation` | `visualization` | `walk_dynamic_*` |
| `dynamic.*.value_field` | `value_fields` | `walk_dynamic_*` |
| `dynamic gauge show_inner_arc`, `show_outer_arc` | `arc_display` | `walk_dynamic_gauge` |
| `dynamic.time_series_lines` | `time_series_lines_multi` | `walk_dynamic_time_series_lines` |
| `bar_chart.xaxis.time` | `time_buckets` | `walk_bar_chart_*` |
| top-level `dynamic.query` | `query_definitions` | `content-json-dynamic-legacy-query`, `datasource_read-content-json-legacy-query`, `create-flatten-fails` |

## Released bugs that these files keep

Nothing in this PR fixes behavior. Each line below is a recorded fact. "May fix" means the migration PR can change it on purpose and list it in the changelog.

| # | Behavior | Golden | Migration |
|---|---|---|---|
| 1 | Import with an empty id fails with a value conversion error. | `import-empty-id` | may fix |
| 2 | A 404 on Get removes the resource from state, also when the route is missing. | `route-missing` | may fix |
| 3 | A state upgrade for a dashboard that is gone only warns and writes no state. | `upgrade-v1-missing`, `upgrade-v2-missing`, `upgrade-v3-missing` | may fix |
| 4 | `auto_refresh` next to `content_json` ends with a "provider produced inconsistent result" error. | `content-json-auto-refresh` | may fix |
| 5 | An `access_policy` that names fewer than four permissions never settles. The API adds `deny` for the others, so every plan shows a change. The acceptance tests use full policies. | `access-policy` (every step) | may fix |
| 6 | Removing `access_policy` from the config keeps the policy, with no update. The API fills a default policy, so state is never null. | `access-policy`, `access-policy-not-set` | keep |
| 7 | `constant_value` fails at apply. | `variables-legacy-constant-value` | keep |
| 8 | A dynamic widget with the deprecated top-level `query` cannot be read. `content_json` is the only way to manage it. | `content-json-dynamic-legacy-query` | keep |
| 9 | Setting an enum to `unspecified` ends with an inconsistent result for some fields: the bar chart `scale_type`, the horizontal bar chart `scale_type`, the pie chart `unit`. | `walk_bar_chart_*`, `walk_horizontal_bar_chart_*`, `walk_pie_chart_*` | may fix |
| 10 | The annotation `strategy` blocks (`duration`, `range`) end with an inconsistent result when optional values are removed. | `walk_top_level` | may fix |
| 11 | `auto_refresh` shows as unknown in many replans after an update. | most walk files | may fix |
| 12 | Removing the spans aggregation `field` or `aggregation_type` ends with an inconsistent result. | `walk_*_spans` | may fix |

## What is open

- The EU2 recording covers one widget in detail. Record the minimal body of each widget kind, and compare the read-back with the fake, to find more fill rules.
- `Create` with a sent dashboard id of 21 characters keeps that id (recorded). Other id rules for sections, rows and widgets are the probe's, not the recording's.
