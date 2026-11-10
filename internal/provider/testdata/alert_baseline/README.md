# coralogix_alert protocol baseline

These goldens record the handwritten `coralogix_alert` resource through the plugin protocol, against
the fake in `alert_baseline_fake_test.go`. When the resource switches to generated code, the golden
diff must show only message wording or proven bug fixes.

```sh
UPDATE_GOLDEN=1 go test ./internal/provider -run TestAlertBaseline
```

Always run a single scenario while you write it (`-run 'TestAlertBaselineScenarios/<name>$'`).
The data source is not part of this baseline: it moves later.

The unit tests job leaves these tests out and the `alert-baseline` job of `build.yml` runs them
(about 80 seconds): the pattern is `ALERT_BASELINE_TESTS` in that file.

## What is covered

229 scenarios, one golden each (`scenario_<name>.json`), plus the schema (`schema_current.txt`).

| Prefix | Alert type or topic |
|---|---|
| `common-` | attributes of every type: priority, incidents settings, schedule, notification group, validation, import, deletion outside Terraform, type switch; `alert_baseline_shared_test.go` holds `common-case-settings`, `common-access-policy`, and `common-data-sources` (with their validation scenarios) |
| `logs-immediate-`, `logs-threshold-`, `logs-ratio-` | `alert_baseline_logs_a_test.go` |
| `logs-anomaly-`, `logs-new-value-`, `logs-unique-count-`, `logs-time-relative-` | `alert_baseline_logs_b_test.go` |
| `metric-threshold-`, `metric-anomaly-`, `metric-type-`, `slo-threshold-` | `alert_baseline_metric_slo_test.go` (configuration, SLO ids that do not exist), `alert_baseline_slo_lifecycle_test.go` (the lifecycle against an SLO that exists) |
| `tracing-immediate-`, `tracing-threshold-`, `flow-`, `analytics-immediate-`, `analytics-threshold-` | `alert_baseline_trace_flow_test.go` |

Per alert type: create, refresh, update of every managed field, removal of every optional field on
Update, every enum value, collections (omitted, empty, non-empty, cleared), import, destroy, and
config validation in both mismatch directions with null members.

## The fake follows the real API

`fidelity/<scenario>.json` holds the requests that the provider sent to the real API during the
recording, and the answers of EU2. `TestAlertFakeMatchesRecordedBackend` replays each request into
the fake and compares the answers, so a scenario's golden is the provider against a fake that was
checked with the real service. Ids are `<id:N>`, times are `<time>`, and `alertVersionId` is
`<version>`: no request sends it and Terraform never sees it. The fake reads the shapes from the
`openapi.yaml` of the pinned SDK, writes the zero values that the REST gateway writes, and adds
the rules of each alert type from the `init` function of its scenario file.

Two EU2 teams recorded this baseline: one without a limit of entity labels (`common-labels` and
`common-top-level-attributes`), and one that is over the limit (every other scenario). The answers
do not differ between them. To record again, use a team key (an organization key gets 403 on
alerts):

```sh
RECORD_ALERT_FIDELITY=1 CORALOGIX_API_KEY=... CORALOGIX_ENV=EU2 \
  RECORD_ALERT_ONLY=<scenario>,<scenario> \
  go test ./internal/provider -run TestRecordAlertFidelity
```

The recorder deletes only the alerts that its run created. A recording never holds a key, a
company id, or a real id. Run the scan in the PR before you commit a recording.

## Scenarios that need more than alerts

An `slo_threshold` alert needs an SLO that exists. The lifecycle scenarios of
`alert_baseline_slo_lifecycle_test.go` make a temporary request-based SLO through
`alertBackend.seedSLO` (an APM SLO is no option, because the recording tenant has no service in its
catalog) and cover create, refresh, update, removal of optionals, every enum, the rules, import,
destroy, both arms and the switch between them, and an SLO or an alert deleted outside Terraform.
The recorder deletes the alerts first and the SLOs after. Its recordings hold the SLO exchanges
too, with the creator of the SLO masked by key (`<user>`), the times masked, and the company id
of the "not found" message replaced. The scenarios of `alert_baseline_metric_slo_test.go` cover the
configuration and the ids that no SLO has (404 for a UUID, 500 for another text).

## What the real API does

| Fact | Golden |
|---|---|
| Create, Get, and Replace answer `{"alertDef": ...}`. Delete answers `{}`. A deleted, unknown, or malformed id is 404 `Not Found: The requested Alert was not found`, on every operation. | `scenario_common-deleted-outside-terraform.json`, `scenario_common-import.json` |
| Zero values are written by name: a missing enum reads back as `..._UNSPECIFIED` or its first value (`ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED`, `ALERT_DEF_TYPE_LOGS_IMMEDIATE_OR_UNSPECIFIED`), a missing list as `[]`, a missing map as `{}`. | every scenario |
| Replace keeps an omitted `description`. Only `""` clears it. It is cleared when the same Replace also changes the content of the type. | `scenario_common-top-level-attributes.json`, `scenario_tracing-immediate-description-removal.json` |
| `alertVersionId` changes only when the type or the content of the type changes. A new name, priority, enabled flag, incident settings, or router keeps it. | fidelity recordings |
| Labels (`entityLabels`) are written as sent and cleared when the request omits them. A team over its limit of entity labels refuses every alert with labels: 400 `Out of Range: Meta Labels Limit Reached`. | `scenario_common-labels.json`, `scenario_common-top-level-attributes.json` |
| Case settings: `enrichment_queries` takes at most one query (400). A destination whose connector does not exist is 400 `Invalid destinations`. Replace without `caseSettings` clears them. | `scenario_common-case-settings.json` |
| Data sources: a data space and data set that do not exist are 400 (`default`/`logs` exists on the recording tenant). Replace without `dataSources` clears them. | `scenario_common-data-sources.json` |
| Access policy: Create takes the policy as a string, Replace as an action (`{"upsert": text}`, `{"delete": {}}`), and a Replace without the action keeps the policy. The API stores the policy with all 14 `alerts:*` permissions: the ones the request omits are `deny`. Get returns it only with `include_access_permissions=true`, with the keys in a different order each time (the recordings sort them). Text that is no JSON is 400. | `scenario_common-access-policy.json`, `scenario_common-access-policy-create.json` |
| Notification: a webhook needs `notify_on` and `retriggering_period` together (400). A router `notify_on` must equal `incidents_settings.notify_on` (400). A connector that does not exist is 400. | `scenario_common-notification-group-*.json` |
| Alerts with several rules refuse mixed condition families (more/less), the same priority on two rules, and rules in different time-window groups. `trigger_undetected_values` and an auto-retire timeframe other than `NEVER` need a less-than rule; an all-default `undetected_values_management` is dropped when there is none. | `scenario_logs-threshold-*`, `scenario_metric-threshold-*`, `scenario_logs-time-relative-*` |
| Anomaly, new value, unique count, tracing threshold, and metric anomaly take exactly one rule. | `scenario_*-rules*.json` |
| Numbers: `auto_retire_seconds` must be a multiple of 60; `logs_anomaly` stores `minimum_threshold` as an integer (7.5 reads back 7); values above int32 are clamped; some values above 2^53 are HTTP 500. | `scenario_logs-*-threshold-values.json`, `scenario_logs-unique-count-max-unique-count.json`, `scenario_tracing-*-latency.json` |
| Lucene: an unbalanced parenthesis is 400; an empty query is stored as absent; a missing filter comes back with empty label filters. | `scenario_logs-immediate-api-errors.json` |

| SLO alerts | Golden |
|---|---|
| Deleting an SLO deletes the alerts that refer to it: they answer 404 afterwards, and the Delete answer `{"effectedSloAlertIds": [...]}` lists them. Refresh then drops the alert with a warning, Update ends in `Missing Resource State After Update`, Destroy succeeds. | `scenario_slo-threshold-slo-deleted-outside-terraform.json` |
| `groupByKeys` always reads back as the by-clause labels of the SLO query (empty for an SLO without one), whatever the request sends. | `scenario_slo-threshold-group-by.json` |
| An `error_budget` threshold above 101 is 400 (the message says 100, 101 is accepted). A `burn_rate` threshold of 0 or above `1 / (1 - target)` (200 for a target of 99.5) is 400 `Failed getting SLO metric alert`. A negative threshold is 400 on both arms. | `scenario_slo-threshold-threshold-values.json` |
| A burn rate duration outside 1..48 is 400. The unit `UNSPECIFIED` is stored as `HOURS`. | `scenario_slo-threshold-burn-rate-duration-values.json` |
| A Replace that names an SLO that does not exist answers 404 `Failed Getting SLO definition`. | `scenario_slo-threshold-change-slo.json` |

## Released behaviour that looks like a bug

The goldens record these as they are. A change in a golden is a decision, never a refresh.

- Update after the alert was deleted outside Terraform ends in `Missing Resource State After Update`
  (`common-deleted-outside-terraform`, `flow-deleted-outside-terraform`).
- An explicit empty collection (`severities`, label filters, `notification_payload_filter`,
  `group_by`, flow `stages`, groups, and `alert_defs`) reads back as null: inconsistent result
  after apply, and the next plan is not empty.
- Removing `description` on Update gives an inconsistent result, because the API keeps the old
  value. Removing `no_data_policy` keeps the old state and sends no request.
- A webhook with only email `recipients` plans `notify_on` as unknown again after every refresh
  (`common-notification-group-recipients`).
- Numbers that the API rounds or clamps (`minimum_threshold`, `percentage_of_deviation`,
  `latency_threshold_ms`, `span_amount`, `max_unique_count`) give an inconsistent result.
- A span field without `filter_type` fails with a value conversion error.
- `Read` panics with a nil pointer when it gets a null prior state (`resource_coralogix_alert.go`,
  first lines of `Read`). Terraform never calls it that way, so no scenario records it.

- `slo_threshold` with the unit `UNSPECIFIED` gives an inconsistent result (the API stores `HOURS`).
- `slo_threshold` with a `group_by` that is not the label list of the SLO gives an inconsistent
  result and a non-empty replan. Changing `slo_id` to an SLO with other labels keeps the old
  `group_by` in the plan, which gives an inconsistent result (`slo-threshold-group-by`,
  `slo-threshold-change-slo`).
- Update to an SLO that does not exist answers 404, and the provider reads it as a deleted alert:
  `Missing Resource State After Update` (`slo-threshold-change-slo`).

- `access_policy` that omits permissions shows drift after every refresh and a replan that is not empty:
  the API stores the omitted permissions as `deny`, and the provider keeps the stored text. A policy
  that names all 14 permissions refreshes to an empty plan (`common-access-policy`,
  `common-access-policy-create`).
- `access_policy = ""` reads back as null, so the plan after the refresh is not empty, on Create and on
  Update (`common-access-policy`, `common-access-policy-create`). Removing the attribute sends no
  request and keeps the policy in state.

Not covered: a destination of `case_settings` that exists. It needs a Notification Center connector, which
this baseline cannot create, so the scenarios record the answer for a connector that does not exist.
The handwritten acceptance test `TestAccCoralogixResourceAlert_caseSettings` covers real connectors.

A failed Update followed by a refresh leaves a non-empty replan in some scenarios. That is the
harness: it keeps the failed config as the applied one. It is no provider behaviour.
