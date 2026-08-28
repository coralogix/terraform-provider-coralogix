# Unreleased

#### resource/coralogix_dashboard
- FIX: An annotation keeps its `color`. The Coralogix UI offers a swatch per colour and the API stores the choice, but the provider had no attribute, so a copy of a red annotation was created as unspecified and the colour was silently lost. Valid values are `default`, `green`, `cyan`, `blue`, `purple`, `magenta`, `red`, `orange`, `yellow` and `unspecified`.
- FEAT: An annotation takes a `description`. The API stores one, 0 to 4096 characters, and the Coralogix UI offers it next to the annotation name, so a dashboard built in the UI could not be expressed in Terraform.
- FIX: A metrics annotation `strategy` no longer requires `start_time`. The API models the strategy as a union with a single member, so a stored strategy can select nothing, which is what the UI writes when no start time metric is chosen. That annotation could not be written, and the provider sent the member regardless of the configuration. An omitted `start_time` now sends no member, and a stored strategy that selects nothing reads back with the block unset, so an imported annotation plans clean.
- FIX: A `variables_v2` value no longer requires `label`. The API leaves the label out for a value the user typed, a `regex` or a `lucene` value for example, so a variable built in the Coralogix UI could not be written in Terraform. `label` is now optional on `single_string`, `single_numeric`, `regex`, `lucene`, `interval` and on the entries of a `multi_string` list.
- FIX: A `variables_v2` query `value_display_options` that sets neither regex now reads as no value at all. The API returns an empty object there, which the provider stored as an object of nulls; the attribute requires at least one of `value_regex` and `label_regex`, so an imported dashboard produced a configuration that could not be planned.
- FEAT: A dashboard `filters` entry takes a `display_name` and an `id`. The Coralogix UI names the filter chip and the API stores the name, so a dashboard built in the UI could not be expressed in Terraform. The `id` identifies the filter inside the dashboard, the same way a section, a row and a widget already do: the API stores what it was created with and assigns none, so a filter used to lose its id on every apply. It is generated when omitted.
- FEAT: A dashboard `filters` entry takes a `scope`, either `all_widgets` or `specific_widgets` with `widget_ids`, the same shape an annotation already has. The Coralogix UI can restrict a filter to chosen widgets and the API stores that, so such a dashboard could not be expressed in Terraform.
- FIX: A dashboard-level `filters` entry with a `spans` source no longer requires `field`, and accepts an `observation_field`. The Coralogix UI writes a spans filter that names its target with an observation field alone. The widget-level filter source is unchanged: its shape is part of the frozen prior schemas, so the dashboard-level source is now its own schema.
- FIX: A widget's `spans` query `filters` no longer require `field`, and accept an `observation_field`, on `line_chart`, `data_table`, `gauge`, `bar_chart`, `horizontal_bar_chart`, `pie_chart` and `hexagon`. The Coralogix UI writes a spans filter that names its target with an observation field alone, so those widgets could not be written. The prior schema versions keep the old shape, because their types are frozen.
- FEAT: A `line_chart` query definition takes an `interval_resolution`, the same block a `bar_chart` x-axis already has. The Coralogix UI writes one on every line chart query.
- FEAT: A `hexagon` `metrics` query takes an `editor_mode`. The API stores which query editor the query was written with, and the Coralogix UI writes `builder`, so a copy of a hexagon built in the UI came out as a text query.
- FEAT: A `hexagon` takes a `decimal_precision`, and its `spans` query takes `group_bys`. The API stores both, and the UI writes them, so a hexagon built in the UI could not be expressed. `decimal_precision` is a wrapper value in the API, so `false` is a value and not an absence.
- FIX: A `gauge` keeps its `arc_display`, and a `gauge` `min` and `max` and a `data_table` column `width` no longer declare a static default. All three are wrapper values in the proto, so an omitted one is absent and not zero: the defaults sent a value the dashboard never had, which showed up as a change on the first plan after an import. Each of the three keeps a non-null value already in state, so a configuration written against the old default keeps its value and still plans clean.
- FEAT: A dashboard `auto_refresh` accepts `one_minute` and `fifteen_minutes`. The API models five intervals, each as its own field, and the Coralogix UI offers all five, but the provider had only `off`, `two_minutes` and `five_minutes`. A dashboard set to one of the other two could not be written or read. The expand now clears all five fields before it sets one, so a change of interval cannot leave two set.
- FEAT: A widget takes a `highlighted` flag. The Coralogix UI marks a widget as highlighted for every user of the dashboard and the API stores it, so the flag was lost on every apply. The API rejects it on a widget that only holds a `reference`.
- FIX: A `horizontal_bar_chart` logs query now sends `group_names_fields` and `stacked_group_name_field`. The schema had both and the read filled them in, but the request never carried them, so a configuration that set either failed the apply with "provider produced inconsistent result after apply" and the value was silently never stored. The other five chart queries already sent them.

# Release 3.11.0

#### data-source/coralogix_user
- FEAT: A user can now be looked up by `user_name` — the email address — instead of only by `id`. Exactly one of the two must be set, and the match is case-insensitive, so a config does not have to track the letter case SSO login stored.
#### resource/coralogix_group
- FIX: Stop clearing group membership when `members` is not managed in configuration. Omitting the argument no longer empties the group on the next update: `members` is now `Optional` and `Computed`, so Terraform reads and stores the group's current members and leaves them alone. Whether membership is managed is decided by the configuration alone, never by what an earlier refresh happened to store, so the same configuration always means the same thing.
- BREAKING: Removing the `members` line from a `coralogix_group` no longer empties the group. Membership becomes unmanaged instead: Terraform reads and stores the group's current members and leaves them alone. Set `members = []` to remove everyone. A configuration that lists `members` keeps managing exactly that list and needs no change on upgrade. This also fixes the reverse case, which is why the change was made — a group whose membership is maintained in the Coralogix UI or by `coralogix_group_attachment` was being emptied on every apply, and `lifecycle { ignore_changes = [members] }` is no longer needed to prevent that.
#### resource/coralogix_rules_group
- FIX: An ambiguous `order` inside a rule-group is now rejected at plan time instead of silently producing duplicate sort indices. Within a rule-subgroup, `order` must be set on every rule or on none of them, and the values must be unique; the same rule applies to `rule_subgroups[*].order` within a rule-group. Previously a list where some rules set `order` and others left it out was sent to the API with two rules sharing one index, because the rules without an `order` were numbered by position over indices a sibling had already claimed. The resulting order was then whatever the API returned, so every subsequent plan could report changes nobody made. An `order` that is still unknown at plan time — derived from a variable or another resource — is checked again against the resolved configuration before the request is built, so it cannot bypass the rule. Note this rejects a configuration that used to plan: an existing rule-group whose `order` values are partially set or duplicated now fails until the configuration is corrected, which is also the point at which the ordering it actually gets becomes predictable.
- DOC: The rule-group level `order` documents that it must be unique across every rule-group in the account, and that Terraform cannot enforce it because rule-groups are separate resources planned in isolation.

#### resource/coralogix_parsing_rules
- DOC: Same rule-group level `order` uniqueness note as `coralogix_rules_group`. The subgroup and rule level `order` are read-only here, so they cannot collide.

#### resource/coralogix_alert
- FEAT: Add `EQUALS` and `NOT_EQUALS` condition types for logs and metric threshold alerts.

#### resource/coralogix_connector
- FEAT: Add support for the `eventbridge` connector type.
- FEAT: Add support for the `incident_io` connector type (preview).

#### resource/coralogix_preset
- FEAT: Add support for `eventbridge` as a `connector_type` value.
- FEAT: Add support for `incident_io` as a `connector_type` value (preview).

#### resource/coralogix_dashboard
- FIX: A bar chart whose x-axis groups by time is readable again. The Coralogix UI writes the interval resolution as `timeBuckets` and rewrites a stored `time` block to it on any save, even one that changes something unrelated such as a colour, after which the read failed with "unknown bar chart x axis type" and `plan`, `refresh`, `apply` and `import` all stopped working against that dashboard. An x-axis stored with no kind selected hit the same failure and is also readable now.
- FEAT: A bar chart x-axis takes `time_buckets`, the interval resolution the API and the Coralogix UI use: `auto` with a maximum number of data points and a minimum interval, or `manual` with a fixed interval, plus `use_advanced_limit`. Set at most one mode; omit both to leave the choice to the backend. Durations are written the way the API stores them, as a number of seconds with an `s` such as `900s`. A `maximum_data_points` beyond the API's 32-bit field is rejected at plan time rather than wrapping to a negative number. The older `xaxis.time` keeps working.
- DEPRECATION: `bar_chart.xaxis.time` warns at plan time when it is set. The Coralogix UI no longer writes it and rewrites it to `time_buckets` when the dashboard is saved, so a configuration using it can stop matching the dashboard it manages. It keeps working; `time_buckets.manual.interval` is the direct replacement for `time.interval`.
- FEAT: Add the `dynamic` widget, one widget type carrying all 15 visualizations — stat and stat card, table, time series, vertical and horizontal bars, gauge and pie, hexagon bins, heatmap and geomap. Exactly one visualization per widget. `query_definitions[*].id` is settable rather than read-only, which is what makes `time_series_lines_multi.query_display_settings[*].query_id` usable at all — it needs the id of the query it styles. The deprecated `stat` and `time_series_lines` are included deliberately: opening either in the Coralogix UI rewrites it to `stat_card` or `time_series_lines_multi`, and a release supporting only the replacement could not read the result.
- FIX: A `dynamic` `stat` widget opened in the Coralogix UI is rewritten to a `stat_card`. Releases 3.10.0 and 3.10.1 shipped `stat` without `stat_card`, so `plan`, `apply` and `destroy` all failed against such a dashboard. Upgrading fixes it with no configuration change.
- FEAT: Add the remaining `line_chart`, `bar_chart`, `horizontal_bar_chart`, `pie_chart`, `gauge` and `data_table` display and query fields, so a dashboard built in the Coralogix UI can be expressed in Terraform. Display formatting — custom units, decimal places, axis bounds and legends — plus query grouping and the PromQL editor settings. See the resource documentation for the full set.
- FEAT: Add `hash_colors` to `line_chart` query definitions, `bar_chart`, `horizontal_bar_chart` and `pie_chart`. Each series takes a colour from a hash of its name and `color_scheme` is ignored. The Coralogix UI calls this `Legend Color Hashing`.
- FEAT: A `dynamic` widget spans query filter takes an `observation_field`, and its `field` is no longer required. The Coralogix UI writes a filter that sets only the observation field, so such a dashboard could not be expressed: the plan failed with `attribute "field" is required`.
- FIX: A logs filter now accepts `field` together with `observation_field`. The Coralogix UI writes both, where `field` is the dot-joined form of the observation field, so a dashboard built in the UI could not be applied and an imported one could not be planned. Covers the logs `filters` list on every widget query and the dashboard-level filter source.
- FIX: `unit` accepts `percent` and `datetime_iso`, which the API supports but the shared unit map omitted; the gauge unit map was also missing `datetime_iso`. A widget saved with either unit read back as null and the next apply overwrote it with `unspecified`. Affects every widget type exposing a `unit`, not only `dynamic`.
- FIX: An enum the API sets itself no longer shows a spurious change on the first plan after upgrading, and an enum the API leaves unset now reads back as `unspecified` rather than an empty string. Set `unspecified` explicitly to hand the choice back to the API. `legend.placement` had the same problem and is fixed with them.
- FIX: A `decimal` or `decimal_precision` now rejects only a value the API's 32-bit integer field cannot hold. A fractional value such as `1.5` was truncated to `1` and a large value wrapped, so the request carried a number the user never wrote and the apply then failed with an inconsistent result. The documented 0 to 15 range is not enforced, because the API accepts values outside it and stores them unchanged.
- FIX: A widget can now set a `title` together with a `markdown` definition. The Coralogix UI always gives a markdown widget a title, so a dashboard exported from the UI failed to validate.
- FIX: Editing `content_json` updates the dashboard in place instead of recreating it, so it keeps its ID and existing links to it keep working.
- FIX: `data.coralogix_dashboard` no longer fails on a dashboard whose widgets use an attribute with a custom value type. Deriving the data-source schema from the resource's dropped `CustomType` for numeric attributes. Affects any resource whose data source derives its schema this way.
- DOC: A `dynamic` widget time-series visualization needs `promql_query_type = "range"` on its metrics query. An instant query returns a single point, so the chart renders empty while the configuration looks correct. Note this does not apply to the classic `line_chart`: the API has no such field on its metrics query, so only `unspecified` applies cleanly there and any other value fails the apply with an inconsistent result. That is long-standing rather than new, and is tracked separately.


# Release 3.10.1

#### resource/coralogix_dashboard
- FIX: Preserve the difference between filter selections that select all values and list selections that contain zero values.

# Release 3.10.0

#### resource/coralogix_connector
- FEAT: Add support for the `microsoft_teams` connector type.

#### resource/coralogix_preset
- FEAT: Add support for `microsoft_teams` as a `connector_type` value.

#### resource/coralogix_tco_policies_rum
- FEAT: Add `coralogix_tco_policies_rum` resource and data source for RUM TCO policies (severities/DPXL matching, quota-based priority override, application/subsystem rules, archive retention). RUM policies have no dataset `targets`.

#### provider
- CHORE: Bump `coralogix-management-sdk` to include OpenAPI required fields for dashboard `variables_v2`.

#### resource/coralogix_dashboard
- FIX: An `access_policy` the provider computes (import, data source, create without the attribute) is now stored in the shape `jsonencode` produces. A configuration that adopts the same policy with `jsonencode` then matches it, so plans stop reporting `auto_refresh = (known after apply)`. Configured text and existing state are unchanged.
- FIX: A dashboard managed with `content_json` no longer plans `auto_refresh = (known after apply)` on every run. A dashboard written as HCL can avoid the same diff by setting `auto_refresh` in the configuration.
- FIX: Adapt `variables_v2` expand/flatten to SDK required value types (`id`, `name`, `display_name`, `display_type`, `source`, `value`, static/query `all_option` / `values_order_direction`, static `values[].value`/`label`, logs `observation_field`, metrics `label_name`).

#### provider
- FIX: API error diagnostics now classify failures by HTTP status and include the backend's response body, instead of showing only the bare error text.
- FIX: gRPC error diagnostics now surface the attached `google.rpc` details (for example, per-field validation messages).
- FIX: Error diagnostics no longer echo the request payload, so credentials carried in a request body cannot leak into CLI or CI logs.
- FIX: Error diagnostics label the failing call `operation` instead of `url`, matching the value callers pass (`Read`, `Create`, `List`, …).
#### resource/coralogix_dashboard
- FEAT: Add typed HCL support for the `dynamic` widget definition: `query_definitions` with the logs/spans/metrics/data_prime query union, a top-level `time_frame` and `interpretation`, and the `stat` visualization at full fidelity. The remaining visualizations follow in later releases; a dynamic widget using one of them, or using the deprecated top-level `query` instead of `query_definitions`, fails the read with a clear diagnostic instead of writing state that silently drops it. Previously every `dynamic` widget failed import and data-source reads.
- DEPRECATION: Mark `dynamic.visualization.stat.value_field` as deprecated, matching the API. Use `value_fields`; the singular form is still read and written so existing dashboards import without losing it.
- FEAT: Add `variables_v2` with static, textbox, and query-backed dashboard variables.
- DEPRECATION: Mark `variables` as deprecated. Use `variables_v2` for new dashboard variables.
- FIX: Schema v2/v3→v4 state upgrade no longer fails with `Missing Upgraded Resource State` when the dashboard was deleted outside Terraform. The upgrader no longer removes the resource from state (illegal inside a state upgrader); it returns a valid v4 state carrying the prior `id`, `name`, `description` and `content_json`, so the following refresh detects the missing dashboard and plans a recreate.

#### resource/coralogix_team
- CHORE: Migrate from the deprecated gRPC Teams client to the OpenAPI Teams REST client. No functional change. Also affects `data.coralogix_team`.
- FIX: The "no longer exists in Coralogix backend" warning rendered the team ID with `%q`, which formats an integer as a character literal (team `12345` printed as `'〹'`). It now prints the numeric ID. Also affects `data.coralogix_team`.

#### resource/coralogix_enrichment
- FIX: Read no longer silently reports a wrong `custom_enrichment_id` when the resource or data-source ID is not a valid `uint32`. Malformed, negative and out-of-range IDs now return an error instead of being written to state as `0` or a mismatched value.

#### provider
- CHORE: Bump `golang.org/x/crypto` to v0.52.0, `golang.org/x/net` to v0.55.0, `google.golang.org/grpc` to v1.82.1 and `github.com/cloudflare/circl` to v1.6.3 to clear open security advisories. The minimum Go version for building from source is now **1.26**, aligning with `coralogix-management-sdk` and `coralogix-operator`.

# Release 3.9.0

#### resource/coralogix_api_key
- FIX: Schema v0→v1 state upgrade no longer fails with `Value Conversion Error ... Struct defines fields not found in object: organisation_id`. The upgrader now decodes prior state into a v0-shaped owner type, sets `owner.organisation_id` and `access_policy` to null, and tolerates an absent `owner`.

#### resource/coralogix_alert
- DOC: Document preferred `no_data_policy.state` values (`OK`, `ALERTING`, `KEEP_LAST`, `NO_DATA`) and that `auto_retire_seconds` is only honored for `ALERTING` / `KEEP_LAST` / `NO_DATA`.
- DEPRECATION: `no_data_policy.state = "UNSPECIFIED"` now emits a plan warning (still accepted and round-tripped in state); omit the block for equivalent legacy behavior.

#### resource/coralogix_tco_policies_logs
- FEAT: Add `targets` to route matched logs to specific named datasets. Each target requires its own `priority`. Policy-level `priority` is now optional; omit it when `targets` is set.
- FIX: `targets[*].priority_override.usage_tiers[*].daily_quota_percentage` is validated to `0`–`100`, matching the policy-level field.

#### resource/coralogix_dashboard
- FEAT: Add `reference` on widgets to reuse a widget from another dashboard by `dashboard_id` and `widget_id`. Exactly one of `definition` or `reference` must be set.
- CHORE: Use the shared SDK `dashboardjson.Unmarshal` helper for `content_json` (snake_case aliases, unknown-key discard) instead of a local copy. Create/replace no longer re-strip unknowns; that happens in Unmarshal, as in the operator.
- FIX: Adding a section, row, widget or annotation without an `id` no longer fails with "Provider produced inconsistent result after apply". Generated `id` fields (section, row, widget, line-chart `query_definitions[].id`, data-table aggregation, annotation) and widget `width` now use `UseNonNullStateForUnknown` so new nested elements stay `(known after apply)` instead of planning as null.
- FEAT: Add support for `dataprime` and `event_recurrence` annotation source types. Dashboards using DataPrime queries or recurring calendar events as annotation sources can now be authored and round-tripped through Terraform.
- FEAT: Add `scope` support to `annotations` — restrict an annotation to `all_widgets` or `specific_widgets`.
- FEAT: Add `query` and `category` to `bar_chart.colors_by` and `horizontal_bar_chart.colors_by`. Dashboards using either value can now be authored and read back through Terraform.
- FIX: `colors_by` now validates against the accepted values at plan time. Previously an unrecognized value was silently dropped from the request and then failed at apply with "Provider produced inconsistent result after apply", which never named the offending attribute.

#### resource/coralogix_events2metric
- CHORE: Migrate events2metric operations from the legacy gRPC client to the REST client.
- FIX: Create fails when `permutations` is omitted because the plan value is unknown.
- FIX: Empty `metric_fields.*.aggregations.histogram.buckets` no longer becomes null after apply.
- FIX: Schema v0→v1 state upgrade converts `permutations.limit` from string to int64 so pre-v1 state loads.
- FIX: Omitted `metric_fields.*.aggregations` children no longer fail after apply when the API returns the full aggregation set. Those attributes now use `UseNonNullStateForUnknown`.

#### data-source/coralogix_group
- CHORE: Lookup by `display_name` no longer uses the gRPC API.
- FIX: A group that is not on the first page of results can now be found by `display_name`.

#### resource/coralogix_group
- FIX: A group deleted outside Terraform is now removed from state on read instead of failing with an error.

#### resource/coralogix_user
- FIX: A user deleted outside Terraform is now removed from state on read instead of failing with an error.

#### resource/coralogix_grafana_folder
- FIX: A folder that is already gone is treated as deleted instead of failing with an error.

#### provider
- CHORE: Bump `terraform-plugin-framework` to v1.17.0.
- CHORE: SCIM users and groups now use `api.<domain>` instead of `ng-api-http.<domain>`.

# Release 3.8.0

#### resource/coralogix_parsing_rules
- FIX: A `json_extract` rule's `destination_field` now accepts values case-insensitively (e.g. the capitalized `Severity` shown in the Coralogix UI), maps them to the correct backend field instead of silently falling back to `category`, and preserves the user's casing on read so no perpetual plan diff arises.

#### resource/coralogix_alert
- FEAT: Add root-level `data_sources` block for associating an alert with existing data spaces/datasets.
- FEAT: Add `undetected_values_management` support to `logs_ratio_threshold`.
- FEAT: Add `retriggering_period_minutes` to `notification_group.destinations`.

#### resource/coralogix_dashboard

- CHORE: Migrate dashboard operations from the legacy gRPC client to the REST client.
- FEAT: Add support for DataPrime queries in `horizontal_bar_chart` widgets.
- PERF: Replace per-child oneof validators (e.g. `instant`/`range`, widget type selection) with a single cheaper validator attached to the parent object, reducing plan/apply time for dashboards with many widgets.

#### resource/coralogix_events2metric
- FEAT: Add support for `data_source` to target a specific `<namespace>/<dataset_name>` instead of the default logs/spans stream.

# Release 3.7.0

#### resource/coralogix_integration

- FIX: Surface the backend's human-readable validation message when creating or updating an integration is rejected, instead of printing the raw `*V1Failure` struct (e.g. `&{{0xc0004…} map[]}`). The diagnostic detail now shows `Failure.GetErrorMessage()`.
- FIX: Changing `version` now plans as an in-place update instead of forcing a destroy-and-recreate. The `RequiresReplace()` plan modifier was removed from `version`; version bumps flow through the existing `Update` method, preserving the integration's identity (and, for managed integrations, its provisioned service account) instead of deleting and re-provisioning it.

#### resource/coralogix_preset
- FEAT: Add `attachment_config` (`AUTO` / `ENABLED` / `DISABLED`, default `AUTO`) to control whether notification payloads include attachments.

#### resource/coralogix_connector
- FEAT: Add support for the `pagerduty_incidents` connector type.

#### resource/coralogix_global_router
- FEAT: Add `disabled` flag to disable a router without deleting it, and `fallback_targets` for per-entity-type fallback. The flat `fallback` attribute is now deprecated in favor of `fallback_targets`.

#### resource/coralogix_dashboard

- DOCS: Clarify when to use `observation_field` over the bare `field` string in logs filters, logs aggregations, and dashboard variables. `observation_field` is required for flat field identifiers whose name contains literal dots (e.g. `log.level`) and for disambiguating fields that share a name across multiple scopes; the bare `field` value is resolved by the backend via dot-split, which silently fails to match literal-dot identifiers.
- FEAT: Add support for the `manual` annotation source, allowing static `instant`/`range` threshold annotations with configurable `orientation`, `unit`, and `message_template`.

#### resource/coralogix_alert
- FIX: Stop perpetual `(known after apply)` drift on `no_data_policy.auto_retire_seconds` when the field is omitted from configuration.

#### provider
- CHORE: Bump `coralogix-management-sdk` and adapt OpenAPI-backed resources to the regenerated oneOf SDK models.

#### resource/coralogix_ai_custom_evaluation
- FIX: Correct example score mapping and clearing of empty `criteria.*.examples` lists.

#### resource/coralogix_dashboard

- FIX: Deprecate the dashboard variable `constant_value` attribute and fail fast on it. It maps to the API's deprecated `Constant` variant, which the backend rejects with an opaque `invalid variable definition: Constant(...)` error. A `DeprecationMessage` surfaces this at plan time, and the provider now returns a clear error (instead of letting the opaque API rejection through) directing users to a `multi_select` variable with a `constant_list` source and a single `selected_values` entry — the supported replacement.
- FIX: `folder.id` and `folder.path` no longer perpetually show `(known after apply)` on plans after a successful apply. Dropped `Computed: true` from both inner attributes (they remain `Optional` with the existing `ExactlyOneOf` mutual-exclusion validator) and updated `flattenDashboardFolder` to mirror whichever field the user set in config, so state matches config cleanly on every refresh. Users whose state was previously double-populated with both `folder.id` and `folder.path` by the buggy flatten will see a one-time diff on the first plan after upgrade as the unused field returns to null; the subsequent apply self-heals.


# Release 3.6.0

#### resource/coralogix_alert

- FIX: The `priority` deprecation warning is now type-aware — emitted only for the alert types that embed an `override` block, and suppressed for the types where top-level `priority` is the only mechanism and is therefore not deprecated.
- FIX: Preserve omitted `custom_evaluation_delay` as unset instead of defaulting it to `0`; existing omitted configurations previously stored as `0` will plan an update to unset the field on the next apply.

#### resource/coralogix_ai_evaluation

- FEAT: Add support for managing AI evaluations.

#### resource/coralogix_ai_custom_evaluation

- FEAT: Add support for managing AI custom evaluations.

#### resource/coralogix_dashboard

- DOCS: Document `folder.path` server-side auto-create side effect: when `path` references a folder hierarchy that does not yet exist, the dashboards backend implicitly creates the missing folders, but those folders are not tracked in Terraform state and are not removed on `terraform destroy`. The `folder.id` form (referencing a `coralogix_dashboards_folder` resource) is now called out as the recommended lifecycle-symmetric pattern in the schema descriptions and the example.
- FEAT: Add Optional `selection_type` to `variables[*].definition.multi_select` (`multi`, `single`); omit to use the API default (multi-select with the implicit "All" option).
- FEAT: Wire `threshold_type` (`absolute` / `relative` / `unspecified`) onto the gauge widget so the proto field `Gauge.threshold_type` (field 12) is no longer silently dropped on apply and reset to the proto default on refresh. Mirrors the existing hexagon plumbing; defaults to `unspecified` so pre-existing state round-trips clean.
- FIX: Every `*.query.logs.filters[*]` block (across all widget types — `data_table`, `line_chart`, `bar_chart`, `pie_chart`, `gauge`, `hexagon`, `horizontal_bar_chart`) and the top-level `filters[*].source.logs` block now accept `observation_field` as the sole filter target. `field` is `Optional` (was `Required`), and a `stringvalidator.ExactlyOneOf` on `field` keeps the field-vs-observation_field misconfiguration explicit. Configs copied from a `data "coralogix_dashboard"` whose backend filter used `observation_field` no longer fail validation with `Missing Configuration for Required Attribute`.
- FIX: `layout.sections[*].options.color` no longer flattens the proto zero-value (`SECTION_PREDEFINED_COLOR_UNSPECIFIED`) to the literal string `"unspecified"`. Sections with no color set now round-trip as `null`, which matches the resource schema's allowed `OneOf` values (cyan/green/blue/purple/magenta/pink/orange). Existing state containing the leaked `"unspecified"` value will refresh to `null` on the next plan; no state migration is required because the broken value was already non-applyable through the resource schema's validator.

# Release 3.5.0

#### resource/coralogix_dashboard

- FEAT: Add support for dashboard `access_policy`.
- FIX: Stop perpetual drift on the `line_chart.stacked_line` attribute when it is omitted from config. The attribute now defaults to `"unspecified"` (matching the sibling `scale_type` / `data_mode_type` pattern), so plans no longer mark it `(known after apply)` on every cycle.
- FIX: Eliminate "Provider produced inconsistent result after apply" on `layout.sections[*].id`, `layout.sections[*].rows[*].id`, and `layout.sections[*].rows[*].widgets[*].id` by marking the `id` attributes `Optional+Computed` so the provider-generated UUID can round-trip on first Create (#505).
- FIX: `layout.sections[*].options.collapsed` now reflects the API value instead of being forced to `null` whenever `description` is unset (typo in flatten nil-guard) (#505).
- FIX: Drop the `Default(0)` and add `UseStateForUnknown()` on the widget `width` attribute so an unset width no longer produces a perpetual `width = 0` drift after every apply. The field is deprecated and ignored by the API; a `DeprecationMessage` now surfaces this to users.

#### resource/coralogix_events2metric

- FIX: `Create` now returns immediately after `resp.Diagnostics.AddError` so a backend rejection no longer poisons state with an empty-ID record (which subsequently resolved server-side to an arbitrary unrelated E2M).

#### resource/coralogix_alert

- FIX: `schedule.active_on` now accepts overnight windows (e.g. `start_time = "22:00"`, `end_time = "08:00"`). The provider was rejecting them with "End time is before start time" because both values get parsed against Go's zero date, making any earlier-clock-time end_time appear "before" start_time. The Coralogix API has no such ordering constraint.
- FIX: `tracing_filter.latency_threshold_ms` no longer drifts to a rounded value after apply. The flatten path was using `big.ParseFloat` with a 10-bit precision argument, which silently rounded values whose mantissa exceeded 10 bits (e.g. `30000` → `30016`, `50000` → `49984`), causing "Provider produced inconsistent result after apply" on v2→v3 migrations. Switched to `strconv.ParseInt` + `big.Float.SetInt64`, matching the pattern already used in this file for `MaxUniqueCountPerGroupByKey` and `TimeframeMs`.
- FIX: Stop injecting `router.id = "router_default"` on the `notification_group.router` API request when the user omits an id from config. Empty `router = {}` now sends an empty-router block (no `id`), so the API performs label-based Global Router matching as documented. Previously the hard-coded default bypassed label-based routing entirely.
- FIX: `notification_group.{group_by_keys, destinations}` now actually clear on update. Removing either attribute from HCL (or the whole `notification_group` block) used to silently re-send the prior state value to the API because both attributes were `Optional+Computed` with `UseStateForUnknown()`. Dropped `Computed` and the plan modifier on both sub-attributes; an explicit empty literal (`group_by_keys = []` or `destinations = []`) is now rejected at plan time with `listvalidator.SizeAtLeast(1)` — omit the attribute or use the router-only form to clear. Re-applies #553, which was accidentally reverted by the recovery force-push on #549.

#### resource/coralogix_quota_allocation_rule_set, data_source/coralogix_quota_allocation_rule_set

- FEAT: Add support for managing and reading account-level quota allocation rule sets.
- FEAT: Support `allocation_type` on quota allocation rules and expose read-only `cx_managed` from the quota allocation data source.
- FIX: Treat delete as successful when the backend clears the singleton quota allocation rule set but returns an error response.

#### resource/coralogix_dashboard

- FIX: Avoid "provider produced inconsistent result after apply" when a `variables[*].definition.multi_select.source.query.query.metrics.label_value` block is configured without `label_filters` (or without `operator.selected_values`) — flatten now returns null for empty backend slices instead of an empty list.

# Release 3.4.2

#### provider

- FIX: When `domain` is an AWS PrivateLink management host (`api.private.<region>.coralogix.com`), dial gRPC on that host instead of `ng-api-grpc.<domain>` so dashboards and other gRPC resources work over PrivateLink.
- FIX: Route SCIM users and groups REST clients to the PrivateLink management host (`https://api.private.<region>.coralogix.com/scim/...`) instead of `ng-api-http.api.private...`.
- CHORE: Bump `coralogix-management-sdk` to latest master.

#### resource/coralogix_integration

- FIX: Support importing integrations when Terraform has not populated dynamic `parameters` state yet.

#### resource/coralogix_slo_v2

- FIX: Adapt SLO create to the renamed SDK request type (`SlosServiceReplaceSloRequest`).

# Release 3.4.1

#### resource/coralogix_api_key

- FIX: Preserve the `value` attribute across Create and refresh so it is available to downstream consumers (e.g. `aws_secretsmanager_secret_version`) on the same apply and does not drift on subsequent plans.

#### resource/coralogix_data_enrichments

- FIX: Multi-type updates no longer cause drift.

#### resource/coralogix_alerts_scheduler

- FIX: `monthly.days` int64 decode.
- DOCS: Update examples and documentation for all-alert suppression.

#### provider

- DOCS: Add Coralogix documentation links to resource and data source descriptions.

# Release 3.4.0

#### resource/coralogix_tco_policies_logs

- FEAT: Add `dpxl_expression` as an alternative to `severities` for matching logs, and `quota_based_priority_override` for dynamic priority reassignment based on daily quota tiers.

#### resource/coralogix_alert

- FEAT: Support permanent Always Active suppression rules.
- FIX: Stop persisting `webhooks_settings` after they are removed from configuration.

#### resource/coralogix_user

- FIX: Treat `user_name` as case-insensitive — backend case normalization (e.g. on SSO login) no longer causes spurious drift or apply errors.

#### resource/coralogix_dashboard

- FIX: Accept `observation_field` as an alternative to `field` in logs aggregation widget configurations.

#### resource/coralogix_parsing_rules

- FIX: Stop `Update` from creating duplicate rule groups.
- FIX: Normalize empty `description` on block rules so it no longer causes spurious drift on re-apply.

#### resource/coralogix_archive_retentions

- FIX: Prevent panic when retentions are sourced from a Terraform variable.

# Release 3.3.0

#### provider

- FEAT: Add support for US3 region

#### resource/coralogix_hosted_dashboard

- FIX: Resolve panic during terraform import

# Release 3.2.0

#### resource/coralogix_recording_rules

- FIX: Name attribute now accepts updates

## data_source/coralogix_webhook

- FIX: Nil pointer dereference panic when empty strings are passed for `id` or `name` attributes

#### resource/coralogix_alert_scheduler

- DOCS: Clarify what_expression field usage

#### resource/coralogix_dashboard

- FEAT: Support dataprime queries for dashboards

#### resource/coralogix_alert

- FIX: Repair 2 provider errors
- FIX: Allow disabling Advanced Notification for webhooks by making notify_on and retriggering_period optional

#### resource/coralogix_connector

- FEAT: Support email and serviceNow connector types

# Release 3.1.1

Fix:
* Don't send `maxUniqueCountPerGroupByKey` when optional field `max_unique_count_per_group_by_key` is not set in `logs_unique_count` alerts. Previously, the provider sent `"0"` causing API `400 Bad Request` errors.


# Release 3.1.0

**Internal:**

* Some more restructuring. Most of the clients are now based on REST APIs. 
* Improved request/response logging using `TF_LOG`

## resource/coralogix_parsing_rules, data_source/coralogix_parsing_rules

- New addition! These will replace `coralogix_rules_group`

## resource/coralogix_rules_group, data_source/coralogix_rules_group

- Deprecated in favor of the new `coralogix_parsing_rules` (Phase out date TBD)

## resource/coralogix_data_enrichments, data_source/coralogix_data_enrichments

- New addition! These will replace `coralogix_enrichment` **and** `coralogix_data_set`

## resource/coralogix_dashboard

- Fix: providing a dashboard folder id will now take precedence over name

## resource/coralogix_api_key

- Feat: Added support for PBAC.

## resource/coralogix_alert

- Feat: Added support for `no_data_policy`.
- Feat: Added support for `ignore_infinity`.
- Feat: Added support for `percentage_of_deviation`.
- Fix: Nil pointer dereference in alert import.

## resource/coralogix_custom_role

- Fix: Make permissions in custom roles case-insensitive

## resource/coralogix_events2metric

- Fix: E2M lucene null results in errors

# Release 3.0.1

## resource/coralogix_global_router

Fix:
- State upgrade issue was resolved

# Release 3.0.0

**Note** From now on, the provider will follow actual semver. 

**Internal:**
* Restructuring
* Some resources use a new backend for sending API requests
* Test race conditions fixed
* A range of smaller fixes across a variety of resources
  
## resource/coralogix_ip_access 
## data_source/coralogix_ip_access

Feat:
* Added!

## resource/coralogix_dashboard

Fix:
* Line Chart now actually supports data prime queries
* A state upgrade bug for v2 schemas that prevented upgrades.

## resource/coralogix_global_router

### Breaking:
* If no ID is provided, the router is now a custom router

Feat:
* Custom routers are now supported. 

## resource/coralogix_team
## data_source/coralogix_team

- Welcome back!

**Note:** Terraform destroy & apply with the same team resource may lead to an error. Contact support if that happens. 

## resource/coralogix_slo_v2

Fix:
- A bug prevented changes to `groups.labels` from the server to be correctly recognized

### Breaking:
- groups.labels is now read-only

## resource/coralogix_tco_policies_logs, resource/coralogix_tco_policies_traces

Fix:
- Enabling/disabling a policy wasn't correctly recognized.

# Release 2.2.3

Re-release

# Release 2.2.2

## resource/coralogix_hosted_dashboard
Fix: 
 * Recreate hosted dashboard when not found in backend 

# Release 2.2.1

## resource/coralogix_dashboard
Fix: 
 * Data Prime query was not recognized by model

# Release 2.2.0

## resource/coralogix_dashboard

Feat:
* Support for Gauge Threshold labels added
  
## resource/coralogix_alert
Change: Field `webhooks_settings.notify_on` changed from optional to mandatory.

## resource/coralogix_team
## data_source/coralogix_team

- Fully removed

## resource/coralogix_slo_v2

Feature: 
- activated the resource/data source

# Release 2.1.2

## resource/coralogix_dashboard

Fix: 
* Incorrect mapping for gauge widget units in dashboards, actually
* Incorrect mapping for layout color options in dashboards.

# Release 2.1.1


## provider

Fix:
* Fixed environment alias mapping to correctly handle both shorthand and longhand environment names (e.g., AP1/APAC1, EU1/EUROPE1, US1/USA1)

## resource/coralogix_slo_v2

Docs:
* Enhanced field documentation for some attributes

## resource/coralogix_dashboard

Fix: 
* Incorrect mapping for gauge widget units in dashboards 

* Incorrect mapping for gauge widget units in dashboards 

# Release 2.1.0


## data_source/coralogix_slo

- Deprecation notice

## resource/coralogix_slo

- Deprecation notice

## data_source/coralogix_slo_v2

Feature: 
- added new SLO type independent of APM

## resource/coralogix_slo_v2

Feature: 
- added new SLO type independent of APM


## resource/data_set

- Deprecation notice

## data_source/data_set

- Deprecation notice

## resource/coralogix_dashboard

Breaking: time_frame property of the Hexagon widget moved into the query for consistency with others

Feature: 
* time_frame is now supported by all widgets
* dataprime query type has been added to line charts
* gauge now has the "decimal" and "display series name" properties
* Stacked line is now available in line charts

Fix:
* JSON import won't fail on unknown keys
* resolve "Value Conversion Error" during variable generation with `selected_values`
* resolve "Inconsistent Result Error" in `promql_query_type`

## data_source/coralogix_dashboard_folder

Feature:
* Import by `name` is now available

## resource/coralogix_alert

Breaking: `output_schema_id` was renamed to `payload_id` for users of notification center alerts

Fix:
* fixing type conversion for `alert_type` using `foreach`
* setting the rule's priority to the alert's priority if not set.

## resource/coralogix_team

- Deprecation notice

# Release 2.0.20
## resource/coralogix_alert
Feature: adding support for dynamic duration format for metric alerts time-window (`of_the_last`).

# Release 2.0.19

## resource/coralogix_preset
Bug Fix:
changing `config_overrides.*.payload_type` to Computed (in addition to Optional) - Will be computed if not set.


# Release 2.0.18

## New resources and data-sources ()
* [coralogix_connector](docs/resources/connector.md) 
* [coralogix_global_router](docs/resources/global_router.md)
* [coralogix_preset](docs/resources/preset.md).

## resource/coralogix_alert
Feature:  adding support for `notification_group.destinations` 

## data_source/coralogix_custom_role 
Feature: adding support for import by name.

# Release 2.0.17

## resource/coralogix_dashboard

Fix: Allow for dashboard JSON to set folder

Docs: Update to reflect JSON incompatibility

# Release 2.0.16

## resource/coralogix_grafana_folder

Fix:
* Fixed 412 error for updating coralogix_grafana_folder

## resource/coralogix_dashboard

Feature: 
* allow to specify folder when creating a dashboard from json


## resource/coralogix_alert

Update:
* coralogix_alert `priority` is now optional

## General

Making CORALOGIX_ENV case-insensitive

# Release 2.0.15

## data-source/coralogix_group

Feature:
* Added support for searching by group `display_name`

## resource/coralogix_group_attachment

Feature:
* New resource for attaching users to groups

## resource/coralogix_alert

Fix:
* Alert overrides were not updated when top level property changed

# Release 2.0.14

Re-Release of 2.0.13 for the TF registry

# Release 2.0.13

## resource/coralogix_alert

Fix:
* Time zone math
* Default alert overrides are not automatically P5

## resource/coralogix_rules_groups

Feature: 
* Custom name for when loading a rule group set from yaml file

Internal:
* Fixed environment variable reading for old providers
* Docs updates

# Release 2.0.12

Internal:
* Updated SDK version
* Docs updates

# Release 2.0.11

Internal:
* Version constant update

# Release 2.0.10

## resource/coralogix_rules_groups

Fix: 
* Severities lookup

# Release 2.0.9

## resource/coralogix_recording_rules

Fix: 
* Recording rules attributes
* Remove validation for RuleGroupSet length

## resource/coralogix_archive_logs

Fix: 
* Invalid empty region

## resource/**

Fix:
* Replace when resource isn't found

# Release 2.0.8

## resource/coralogix_dashboard_folder

Fix:

* Do not fail on dashboards folder creation if the remote state differs

## resource/coralgoix_dashboard

Feature: 
* Hexagon Dashboard widget

Fix: 
* Added aggregation for spans in line charts

## resource/coralogix_slo

Fix: 
* SLO threshold operator issue

## resource/coralogix_alert

Feat:
* Custom_evaluation_delay

# Release 2.0.7

### resource/coralogix_alert

Fix:

* Add PhantomMode field


### resource/coralogix_integration

Fix:

* Add support for lists

# Release 2.0.6

### resource/coralogix_dashboard

Fix:

* Add promqlQueryType field to dashboard

# Release 2.0.5

Fix:

* Bumped SDK to 1.1.1

# Release 2.0.4

Fix:

* Fixed env parsing

# Release 2.0.3
### resource/coralogix_scope
* Fixed scope update

# Release 2.0.2
### resource/coralogix_scope
* Update scopes in place instead of creating new ones on update

# Release 2.0.1
### resource/coralogix_slo
* Various SLO fixes


# Release 2.0.0

The provider is now based on the Coralogix Management SDK with the latest APIs. This fixes a variety of issues and should be mostly transparent to the user. 

Breaking Changes:

#### resource/coralogix_alert

Revamped the structure of alerts in general. Please consult the guide v1-v2-migration-guide on how to migrate.

# Release 1.18.16

Feature:

* Add analytics header to requests

# Release 1.18.15

Fix:

* SLO issue when using variables

# Release 1.18.14

Fix:

* Fixed geo_ip enrichments

# Release 1.18.13

New features:

* added low severity alerts

# Release 1.18.12

Fix: 

* coralogix_integration with sensitive data didn't work
* coralogix_integration with additional default parameters didn't work
* documentation examples are now automatically generated

New Features

New Features:

* new endpoint: `[AP3, APAC3]`.

### resource/coralogix_dashboard 

* Support for auto generated IDs added

DEVELOPERS:

* go version was update to 1.23.x

# Release 1.18.7

New Features:

### data-source/coralogix_webhook
* Added support for searching by webhook name.

# Release 1.18.6

New Features:

### resource/coralogix_rules_group
* added support for `text` option for `json_extract` rule type.

# Release 1.18.5

Fix: 

### resource/coralogix_webhook
* Replaced depracated MS Teams webhook with MS Teams Workflow Webhook.

# Release 1.18.4

Fix: 

#### resource/coralogix_integration
* improved error messages for invalid parameters before creating 

### resource/coralogix_webhook
* Replaced depracated MS Teams webhook with MS Teams Workflow Webhook.

### resource/coralogix_alert
* removed regex validation from search query

New Features:

* endpoints can now specified in an abbreviated fashion: `[AP1, AP2, EU1, EU2, US1, US2]`.

# Release 1.18.3

**defunct**

# Release 1.18.2

Fix: Duplicate GRPC extension crash (actually)

# Release 1.18.1

Fix: Duplicate GRPC extension crash

# Release 1.18.0

New Features:
#### resource/coralogix_integration
* added integration support

#### resource/coralogix_sli
* removed, use `coralogix_slo` instead

#### resource/coralogix_traces_policy
* removed, use `coralogix_traces_policies` instead

#### resource/coralogix_logs_policy
* removed, use `coralogix_logs_policies` instead

# Release 1.17

New Features:
#### resource/coralogix_scope
* added Scope support

#### resource/coralogix_group
* added support for associated scopes


# Release 1.16.4
Bug fixing:
#### resource/coralogix_dashboard
* changing `pie_chart` and `horizontal_bar_chart` `query.logs.group_names` to Optional.

# Release 1.16.3
New Features:
#### resource/coralogix_dashboard
* added support for more than one `section`.
* added support for `query` option in `multi_select` variables.

# Release 1.16.2

New Features:
#### resource/coralogix_api_key
* added support for `Organisation_Id` owners.

#### resource/coralogix_dashboard
* added support for section options 

Bug fixing:
#### resource/coralogix_api_key
* HTTP 403 responses will now be displaying the actual error message

Various documentation fixes

Deprecation: `coralogix_sli` deprecated in favor of `coralogix_slo`

# Release 1.16.1
New Features:
#### resource/coralogix_alert
* adding `more_than_or_equal_usual` and `less_than_or_equal_usual` conditions to `metric.promql` alert.

# Release 1.16.0
Breaking Changes:
#### resource/coralogix_api_key
* Roles are replaced by "Presets" and "Permission" keys. Read more [here](https://coralogix.com/docs/api-keys/).

Various documentation upgrades

# Release 1.15.1
New Features:
#### resource/coralogix_alert
* adding `5Min` to `time_window` options for `unique_count` condition.

# Release 1.15.0
Breaking Changes:
#### resource/coralogix_alert
* `group_by` needs to be set instead of `group_by_keys` in case of `more_than_usual` condition.
* `time_window` was added for `more_than_usual` condition.

# Release 1.14.1
New Features:
#### resource/coralogix_dashboard
* adding units for `line_chart` query_definitions.

# Release 1.14.0
Breaking Changes:
#### coralogix_tco_policy_logs and coralogix_tco_policy_traces 
* Resources and Data Sources were deprecated. Use [coralogix_tco_policies_logs](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/tco_policies_logs.md) and [coralogix_tco_policies_traces](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/tco_policies_traces.md) instead.

Bug fixing:
#### resource/coralogix_dashboard
* fixing [issue](https://github.com/coralogix/terraform-provider-coralogix/issues/224).

# Release 1.13.6
Bug fixing:
#### resource/coralogix_dashboard
* replace in a case of not_found error in `coralogix_dashboard` resource.
* adding validations.

# Release 1.13.5
Bug fixing:
#### resource/coralogix_events2metric
* fixing conversion of `buckets` from `float32` to `float64`.

# Release 1.13.4
Breaking Changes:
* `org_key` and `CORALOGIX_ORG_KEY` were removed from the provider configuration. use `api_key` and `CORALOGIX_API_KEY` instead.

# Release 1.13.3
New Features:
#### resource/coralogix_webhook
* adding `attachments` to `slack` webhook type [attachments](docs/resources/webhook.md#nested-schema-for-slackattachments). fixing [issue](https://github.com/coralogix/terraform-provider-coralogix/issues/219).

# Release 1.13.2
Bug fixing:
#### resource/coralogix_alert
* fixing [runtime error: invalid memory address or nil pointer dereference](https://github.com/coralogix/terraform-provider-coralogix/issues/212).

# Release 1.13.1
Bug fixing:
#### resource/coralogix_dashboard
* adding schema upgrade v1 to v2 (for `annotations.source.metrics` field).

# Release 1.13.0
Breaking Changes:
#### resource/coralogix_dashboard
* `annotations.source.metric` was changed to `annotations.source.metrics`.

Bug fixing:
#### resource/coralogix_dashboard
* fixing [inconsistent result for color_scheme](https://github.com/coralogix/terraform-provider-coralogix/issues/217).

New Features:
#### resource/coralogix_dashboard
* adding `data_mode` for data_table widget.
* adding `logs` and `spans` options for `annotations.source`.
* adding `auto_refresh` for dashboard.

# Release 1.12.1
Bug fixing:
#### resource/coralogix_slo
* fixing `threshold_symbol_type` bug in ac ase of `greater_or_equal` and add `less_or_equal` option.

# Release 1.12.0
Breaking Changes:
#### resource/coralogix_sli
* `filters` was changed from `TypeList` to `TypeSet`.

# Release 1.11.13
Breaking Changes:
#### resource/coralogix_team and resource/coralogix_moving_quota
* `coralogix_moving_quota` was removed, and the `coralogix_team` resource was changed to support setting of daily-quota.

# Release 1.11.12
New Features:
#### resource/coralogix_dashboards_folder
* Adding support for `parent_id`.

# Release 1.11.11
Breaking Changes:
#### resource/coralogix_user, resource/coralogix_group and resource/coralogix_custom_role
* `team_id` was removed. managed by (team's) api-key with the right permissions.

Bug fixing:
#### resource/coralogix_events2metric
* fixing `buckets` type-conversion bug (from float32 to float64).
#### resource/coralogix_dashboard
* fixing `time_frame.relative.duration` flattening bug when set to `seconds:0`.

# Release 1.11.10
New Features:
#### resource/coralogix_custom_role
* Adding `coralogix_custom_role` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/custom_role.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/custom_role.md).

# Release 1.11.9
New Features:
#### resource/coralogix_user
* Adding `coralogix_user` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/user.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/user.md).
#### resource/coralogix_group
* Adding `coralogix_user_group` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/group.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/group.md).

# Release 1.11.8
New Features:
#### resource/coralogix_api_key
* Adding `coralogix_api_key` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/api_key.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/api_key.md).

# Release 1.11.7
Bug fixing:
#### resource/coralogix_dashboard
* fixing flatten of `json_content` field bug.

# Release 1.11.6
Bug fixing:
#### resource/coralogix_dashboard
* fixing DataTableSpansAggregationModel parsing bug.

# Release 1.11.5
Bug fixing:
#### resource/coralogix_slo
* fixing log messages and flattening update-response into schema.

# Release 1.11.4
New Features:
#### resource/coralogix_dashboards_folder
* Adding support for `coralogix_dashboards_folder` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/dashboards_folder.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/dashboards_folder.md).
#### resource/coralogix_dashboard
* Adding support for `folder`, `annotations` fields.
* Adding support for `data_prime` for `bar_chart`, `data_table` and `pie_chart` widgets.
* adding validation for `env` field.

# Release 1.11.3
Bug fixing:
* adding validation for `env` field.

# Release 1.11.1
New Features:
#### resource/coralogix_webhook
* Adding support for `event_bridge` webhook type.

Bug fixing:
#### resource/coralogix_team
* fixing log message when for permission denied error.

# Release 1.11.0
Breaking Changes:
#### resource/coralogix_alert
* `show_in_insights` was removed. use `incident_settings` or notification's `notify_on` and `retriggering_period_minutes` instead.
* exactly one of `incident_settings` or all of  `notifications_group.*.notification.*.` `notify_on` and `retriggering_period_minutes` must be set.

New Features:
#### resource/coralogix_alert
* Adding support for `metric.0.promql.0.condition.0.less_than_usual`.

Bug fixing:
* avoiding calling moving quota endpoint when moving quota is not needed.
* fixing `coralogix_alerts_scheduler` terraform lose track over the resource when `coralogix_alerts_scheduler` is change externally.

# Release 1.10.11
New Features:
#### resource/coralogix_slo 
* Adding support for `coralogix_slo` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/slo.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/slo.md).

# Release 1.10.10
New Features:
#### resource/coralogix_team
* Adding support for `coralogix_team` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/team.md)
### resource/coralogix_moving_quota
* Adding support for `coralogix_moving_quota` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/moving_quota.md)

# Release 1.10.9
New Features:
#### resource/coralogix_alerts_scheduler
* Adding support for `coralogix_alerts_scheduler` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/alerts_scheduler.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/alerts_scheduler.md).

# Release 1.10.7
New Features:
#### resource/coralogix_archive_logs
* Adding support for `coralogix_archive_logs` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/archive_logs.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/archive_logs.md).
* Adding support for `coralogix_archive_metrics` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/archive_metrics.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/archive_metrics.md).

# Release 1.10.6
New Features:
#### resource/coralogix_archive_retentions
* Adding support for `coralogix_archive_retentions` [resource](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/resources/archive_retentions.md) and [data-source](https://github.com/coralogix/terraform-provider-coralogix/tree/master/docs/data-sources/archive_retentions.md).

# Release 1.10.4
Breaking Changes:
#### resource/coralogix_tco_policy_overrides
* the resource was deprecated and removed.

# Release 1.10.0
Breaking Changes:
#### resource/coralogix_recording_rules_groups_set
* `group` was changed to `groups` and from `TypeSet` to `SetNestedAttribute`. e.g. - `group { }` => `groups = [{ }]`.
* `group.rule` was changed to `groups.rules` and from `TypeList` to `ListNestedAttribute`. e.g. - `rule { }` => `rules = [{ }]`.
* this version contains a [State Upgrader](https://developer.hashicorp.com/terraform/plugin/framework/migrating/resources/state-upgrade#framework). It will upgrade the state to the new schema. Please make sure to back up your state before upgrading.

# Release 1.9.0
Breaking Changes:
#### resource/coralogix_webhook
* All webhook types changed from `TypeList` to `SingleNestedAttribute` e.g. - `slack { }` => `slack = { }`.
* Linkage between webhook and alert was changed from webhook's `id` to webhook's `external_id`. e.g.- `integration_id = coralogix_webhook.slack_webhook.id` => `integration_id = coralogix_webhook.slack_webhook.external_id`

# Release 1.8.11
New Features:
#### resource/coralogix_dashboard
* Adding support for `markdown` and `horizonal_bar_chart` widgets.
* Adding support for `color_scheme` and `sort_by` for `bar_chart`.

# Release 1.8.10
New Features:
#### resource/coralogix_dashboard
* Adding limitation for `layout.sections` length (will support few sections in the future).
* is_visible is true by default (for all is_visible fields).
* Removing `gauge.query.logs/spans.aggregation` from schema.

# Release 1.8.6
New Features:
#### resource/coralogix_alert
* Adding support for `flow.group_by`.

# Release 1.8.0
Breaking Changes:
#### resource/coralogix_dashboard
* schemas where changed to support the new dashboard widgets and more convenient schema.

# Release 1.7.0
Breaking Changes:
#### resource/coralogix_tco_policy was changed to coralogix_tco_policy_logs and contains the next changes:
* `subsystem_name` was changed to `subsystems` and have different structure e.g. - 
`subsystem_name {
  is    = true
  rules = ["mobile", "web"]
  }` => `subsystems = {
  rule_type = "is"
  names = ["mobile", "web"]
  }`
* `application_name` was changed to `applications` and have different structure e.g. - `application_name {
  starts_with = true
  rule        = "prod"
  }` => `applications = {
  rule_type = "starts_with"
  names        = ["prod"]
  }`

**Please note** - this version contains a [State Upgrader](https://developer.hashicorp.com/terraform/plugin/framework/migrating/resources/state-upgrade#framework). It will upgrade the state to the new schema. Please make sure to back up your state before upgrading.
(for upgrading the schemas the resource names have to be change manually to coralogix_tco_policy_logs before upgrading)


FEATURES:

#### resource/coralogix_tco_policy_traces
* new resource - _coralogix_tco_policy_traces_

DEVELOPERS:
#### resource/coralogix_tco_policy
* using grpc endpoint instead of the REST endpoint.
* moved to `plugin-framework`.

# Release 1.6.5

INTERNAL CHANGES:
#### resource/coralogix_tco_policy
* `tco_policy` and `tco_policy_override` endpoints were changed.

# Release 1.6.4

Breaking Changes:

#### resource/coralogix_alert

* `ratio` and `time_relative`'s `condition`'s `ignore_infinity` conflicts with `condition`'s `less_than`.

BUG FIXING:

#### resource/coralogix_tco_policy

* Fixing - getting panic on creation errors.

# Release 1.6.3

DEVELOPERS:

#### resource/coralogix_actions

* Resource and Data Source were moved to plugin-framework.

# Release 1.6.2

FEATURES:

#### resource/coralogix_tco_policy

* Adding support for `archive_retention_id`.

# Release 1.6.1

FEATURES:

#### resource/coralogix_alert

* Adding support for `more_than_usual` condition for `metric.promql` alert.

# Release 1.6.0

BREAKING CHANGES:

#### resource/coralogix_events2metric

**Please note** - this version contains
a [State Upgrader](https://developer.hashicorp.com/terraform/plugin/framework/migrating/resources/state-upgrade#framework)
. It will upgrade the state to the new schema. Please make sure to back up your state before upgrading.

* `logs_query` type was changed from `Block List, Max: 1` to `Attributes`.
* `spans_query` type was changed from `Block List, Max: 1` to `Attributes`.
* `metric_fields` type was changed from `Block Set` to `Attributes Map`, and `metric_fields`'s `target_base_metric_name`
  was changed to the map's key. e.g. - `metric_fields {target_base_metric_name = "metric_name" ...}`
  => `metric_fields = {"metric_name" = {...}}`.
* `metric_fields`'s `aggregations` type was changed from `Block List, Max: 1` to `Attributes`.
* All `aggregations`'s fields (`avg`, `count`, `histogram`, `max`, `min`, `samples`, `sum`) types where changed
  from `Block List, Max: 1` `Attributes`.
* `metric_labels` type was changed from `Block Set` to `Attributes Map`, and `metric_labels`'s `target_label_name` was
  changed to the map's key. e.g. - `metric_labels {target_label_name = "label_name" ...}`
  => `metric_labels = {"label_name" = {...}}`.

BUG FIXING:

#### resource/coralogix_events2metric

* Fixing
  - [`aggregations` cannot be updated after creation](https://github.com/coralogix/terraform-provider-coralogix/issues/115)
  .

# Release 1.5.9

BUG FIXING:

#### resource/coralogix_alert

* Fixing - tracing alert with `tracing.tag_filter` and `tracing.applications`/`tracing.services`/`tracing.services`
  filters doesn't work for 'equal' operator.

FEATURES:

#### resource/coralogix_alert

* Adding support for 'notEquals' operator for `tracing.tag_filter` and `tracing.applications`/`tracing.services`
  /`tracing.services` filters.

# Release 1.5.8

BREAKING CHANGES:

#### resource/coralogix_dashboard

* for all the new line chart widgets - `query`, `series_name_template`, `series_count_limit`, `unit` and `scale_type`
  fields were deprecated. They will be part of `query_definition` now.
* all lists of objects names' 's' suffix was removed (e.g. - `widgets` => `widget`).

# Release 1.5.7

BUG FIXING:

#### resource/alert

* Fixing - received an error when updating promql alert condition from less_than to more_than

# Release 1.5.6

#### resource/tco_policy

*
Fixing [TF doesn't detect manually deleted resources](https://coralogix-dev.slack.com/archives/C04CV0JG36H/p1683820712917899)

# Release 1.5.5

BREAKING CHANGES:

#### resource/recording_rules_group

* Deprecated, and replaced with [recording_rules_groups_set](./docs/resources/recording_rules_groups_set.md) .

# Release 1.5.4

FEATURES:

* Adding [tco_policy_override](docs/resources/tco_policy_override.md) resource and data source.

# Release 1.5.3

BREAKING CHANGES:

#### resource/tco_policy

* `severities` is now required.
* `order` is now required.

BUG FIXING:

#### resource/tco_policy

*
Fixing [TF doesn't detect manually deleted resources](https://coralogix-dev.slack.com/archives/C04CV0JG36H/p1683820712917899)
*
Fixing [Order of policies can't be manged by TF](https://coralogix-dev.slack.com/archives/C04CV0JG36H/p1681995853325159)

FEATURES:

* Adding [Custom Domain option](docs/index.md#private-domains)

DEVELOPERS:

* go version was update to 1.20.x

# Release 1.5.2

FEATURES:

#### resource/events2metric

* Adding [aggregations](docs/resources/events2metric.md#nested-schema-for-metric_fieldsaggregations) option
  to `metric_fields`.

# Release 1.5.0

BREAKING CHANGES:

#### resource/events2metric (~~logs2metric~~)

* resource and data-source name _logs2metric_ was changed to _events2metric_ and contains `logs_query` and `span_query`
  option.

# Release 1.4.4

BREAKING CHANGES:

#### resource/alert

* `notifications_group.group_by_fields` was changed from _TypeSet_ (doesn't keep order of declaration) to _TypeList_ (
  keeps order of declaration). This change can cause to diffs in state.

# Release 1.4.0

BREAKING CHANGES:

#### resource/alert

* `meta_labels` was changed to key-value map. (e.g.
  - `meta_labels {key = "alert_type" value = "security"} meta_labels {key = "security_severity" value = "high"}`
  => `meta_labels = {alert_type = "security" security_severity = "high" }`).
  ([example-usage](docs/resources/alert.md#standard-alert)).
* `scheduling.time_frames` was changed to `time_frame`.
* `standard.occurrences_threshold` and `tracing.occurrences_threshold` were changed to `threshold`.
* `ratio.queries_ratio` was changed to `ratio_threshold`.
* `notification` was changed to list of `notifications_group` and have entire different
  schema ([nested-schema-for-notifications](docs/resources/alert.md#nested-schema-for-notifications_group)).
* `notification.ignore_infinity` was moved to `ratio.condition.ignore_infinity`
  and `time_relative.condition.ignore_infinity`.
* `notification.notify_every_min` was changed to `notifications_group.notification.retriggering_period_minutes`.
* `notification.on_trigger_and_resolved` (boolean) was changed to `notifications_group.notification.notify_on` (string).
* `notification.recipients.webhook_id` replaced with `notifications_group.notification.integration_id` and should
  contain the integration's (webhook's) id instead of the integration's name.
* flow-alert's (`flow`) schema was fixed. Any earlier version contained wrong schema of
  flow-alert. ([nested-schema-for-flow](docs/resources/alert.md#nested-schema-for-flow)).
* `tracing.field_filters` was removed, and `tracing.applications`, `tracing.applications` and `tracing.services` were
  added instead.
* `tracing.tag_filters` was changed to `tracing.tag_filter` and contains only `field` and `values`.
* `tracing.tag_filter.values`, `tracing.applications`, `tracing.applications` and `tracing.services` have the same
  format as the other alerts' filters. ([example-usage](docs/resources/alert.md#tracing-alert)).
* `tracing.latency_threshold_ms` was changed to `latency_threshold_milliseconds`.

# Release 1.3.31

BREAKING CHANGES:

#### resource/alert

* `categories` ,`classes`, `computers`, `ip_addresses`, `methods` and `search_query` are not supported
  filters for tracing alert, Therefore they were deleted from the tracing-alert scheme.
* `applications`,`severities` and `subsystems` filters have currently different format in
  tracing-alert (`field_filters`),
  Therefore they were deleted from the tracing-alert scheme.

# Release 1.3.29

BREAKING CHANGES:

#### resource/alert

* `alert_severity` was changed to `severity`.
* `manage_undetected_values.disable_triggering_on_undetected_values` was omitted.
  Instead, it's possible to set `manage_undetected_values.enable_triggering_on_undetected_values = false`
  (`manage_undetected_values.auto_retire_ratio` is not allowed in that case).

# Release 1.3.27

BREAKING CHANGES:

#### resource/alert

* `webhook_ids` was changed to `webhooks`.

# Release 1.3.0

BREAKING CHANGES:

#### provider

* `url` was deleted. Instead, added `env` which defines the Coralogix environment. Also, can be set by environment
  variable `CORALOGIX_ENV` instead. Can be one of the following - `[APAC1, APAC2, EUROPE1, EUROPE2, USA1]`.
* `timeout` was deleted. Will be defined by a different timeout for each resource (internally).

#### resource/rule

* The resource `rule` was deleted. Use `rule_group` with single inner `rule` instead.

#### resource/coralogix_rules_group

* `enabled` changed to `active`.
* `rule_matcher` was deleted and `severity`, `applicationName` and `subsystemName` were moved out to previous level as
  separated lists of `severities`, `applications` and `subsystems`.
* `rules` was deleted and replaced by `rule_subgroups` (every `rule-subgroup` is list of `rule`s with 'or' (||)
  operation between).
* `rules.*.group` was deleted and replaced by `rule_subgroups.*.rules`.
* `rules.*.group.*.type` was deleted. Instead, every `rule` inside `rules` (`rule_subgroups.*.rules.*`) can be one of
    - `[parse, block, json_extract, replace, extract_timestamp, remove_fields, json_stringify, extract]`.
* All the other parameters inside `rules.*.group.*` were moved to the specific rule type schemas
  inside `rule_subgroups.*.rules.*`. Any specific rule type schemas contain only its relevant fields.

#### resource/alert

* `severity` changed to `alert_severity` and can be one of the following - `[Info, Warning, Critical, Error]`.
* `type` was removed. Instead, every alert must contain exactly one of
    - `[standard, ratio, new_value, unique_count, time_relative, metric, tracing, flow]`.
* `schedule` changed to `scheduling`.
* `schedule.*.days` changed to `scheduling.*.days_enabled` and can be one of the following
    - `[Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday]`.
* `schedule.*.start` changed to `scheduling.*.start_time`.
* `schedule.*.end` changed to `scheduling.*.end_time`.
* All the other parameters inside `alert` were moved to the specific alert type schemas inside `alert`. Any specific
  alert type schemas contain only their relevant fields.

FEATURES:

* **New Resource:** `logs2metric`

IMPROVEMENTS:

#### provider

* `api_key` can be declared as environment variable `CORALOGIX_API_KEY` instead of to terraform configs.
* Add Acceptance Tests.
* Added retrying mechanism.

#### resource/coralogix_rules_group

* Add Acceptance Tests.
* Moved to Coralogix grpc endpoint.

#### resource/alert

* Add Acceptance Tests.
* Moved to Coralogix grpc endpoint.
