# Unreleased

#### resource/coralogix_alerts_scheduler
- CHORE: The resource is generated from the API contract. Schema version stays 0, every attribute keeps its type, and plans and state keep their values: `start_time` and `time_zone` keep their configured form (with or without `Z`, `UTC` or `UTC+0`), and `UTC` is sent as `UTC+0`.
- FIX: `schedule.recurring.dynamic.frequency.daily = {}` no longer fails with "inconsistent result after apply". The state keeps `daily = {}`.
- FIX: `filter.alerts_unique_ids = []` no longer fails with "inconsistent result after apply". The state keeps `[]`; a filter without a selector still sends an empty `alertUniqueIds` and reads back as null.
- FIX: Update of a rule deleted outside Terraform (with `-refresh=false`) returns an error that says to apply with a refresh, which removes the rule from state. Before, it returned the raw API 404.
- FIX: `id` stays known in the plan of an update. Before, every update planned it as unknown, so resources that reference it planned a change too. The API keeps the id on update.
- FEAT: Validation requires `weekly.days` and `monthly.days`, and exactly one of `daily`, `weekly`, or `monthly` in `frequency`. Before, the API rejected such a rule at apply.
- CHORE: Update reads the rule from the replace response instead of a second read; the two differ only in a field that the resource does not manage. Attributes without a description show the API description.

#### data-source/coralogix_alerts_scheduler
- CHORE: Uses the schema and the conversion of the generated resource.

#### resource/coralogix_alerts_scheduler
- CHORE: Add a protocol baseline of the resource (schema and lifecycle scenarios against an in-memory API), checked against a recording of the real API. Test-only; no behavior change.
#### provider
- CHORE: Bump `coralogix-management-sdk` to the version with the alert suppression rule contract. The SDK holds required fields as values; `coralogix_alerts_scheduler`, `coralogix_events2metric`, and `coralogix_slo_v2` adapt, with no change to plans.

#### resource/coralogix_view
- FEAT: Add `coralogix_view` for a saved Explore view. `name` and `time_selection` are required, and `time_selection` needs exactly one of `quick_selection` or `custom_selection`. `search_query`, `filters`, `folder_id`, `view_type`, and `is_compact_mode` can be set, changed, and removed. An omitted `view_type`, `is_compact_mode`, or `search_query.syntax_type` reads back as the server default (`VIEW_TYPE_LOGS`, `false`, `SYNTAX_TYPE_LUCENE`), and removing one restores that default. A configured `is_compact_mode` is applied with a replace right after create, because create does not take it. `time_selection.quick_selection.caption` is computed. The API stores `time_selection.quick_selection.seconds` as the nearest preset, so set one of the listed presets.
- CHORE: Bump `coralogix-management-sdk` to the version with the views API contract.
- FEAT: Validation rejects a `time_selection.quick_selection.seconds` that is not a preset, and a `filters.filters[*].selected_values` entry set to `false`: the API stores the nearest preset and drops false values, so the read would disagree with the plan.
- FIX: An update of a resource that no longer exists fails with "Resource no longer exists", which says that a refresh (`terraform apply` without `-refresh=false`) removes it from state. Before, it warned that Terraform removed it from state, and the apply then failed with "Missing Resource State After Update".

#### resource/coralogix_view_folder
- CHORE: The resource is generated from the API contract. Schema version stays 1. `id` stays computed and keeps its value when the folder is renamed. `name` stays required and must be between 1 and 100 characters.
- FIX: An update of a resource that no longer exists fails with "Resource no longer exists", which says that a refresh (`terraform apply` without `-refresh=false`) removes it from state. Before, it warned that Terraform removed it from state, and the apply then failed with "Missing Resource State After Update".

#### resource/coralogix_slo_v2
- CHORE: The resource is generated from the API contract. Create, Read, Update, and Delete use the generated client. The schema, the plan, and the state are unchanged; only the wording of some messages and descriptions differs.
- FEAT: `window.slo_time_frame` accepts `1_day` and `30_days`. Before, they were rejected, and an SLO with such a window read back as `""`.
- FIX: Removing `description` clears it. Before, the API kept the stored description, because a replace without it leaves the stored value; the provider now sends `""`.
- CHORE: Add the generated acceptance test, and upgrade tests that create request-based, window-based, and APM SLOs with provider 3.21.0 and plan with this build. Add acceptance tests for the `1_day` and `30_days` time frames.
- CHORE: Bump `coralogix-management-sdk` to the SLO contract that marks server-set fields read-only and requires the APM latency `time_window`.
- FIX: An update of a resource that no longer exists fails with "Resource no longer exists", which says that a refresh (`terraform apply` without `-refresh=false`) removes it from state. Before, it warned that Terraform removed it from state, and the apply then failed with "Missing Resource State After Update".
- FIX: An imported SLO with a threshold, a percentile, or a target that a float32 cannot hold exactly, such as `0.2`, plans no change. Before, the import stored `0.20000000298023224`, and the next plan sent an update.

#### data-source/coralogix_slo_v2
- CHORE: Uses the schema and the conversion of the generated resource.

#### resource/coralogix_configuration_group
- FIX: An update of a resource that no longer exists fails with "Resource no longer exists", which says that a refresh (`terraform apply` without `-refresh=false`) removes it from state. Before, it warned that Terraform removed it from state, and the apply then failed with "Missing Resource State After Update".

#### resource/coralogix_connector
- FIX: An update of a resource that no longer exists fails with "Resource no longer exists", which says that a refresh (`terraform apply` without `-refresh=false`) removes it from state. Before, it warned that Terraform removed it from state, and the apply then failed with "Missing Resource State After Update".

#### resource/coralogix_global_router
- FIX: An update of a resource that no longer exists fails with "Resource no longer exists", which says that a refresh (`terraform apply` without `-refresh=false`) removes it from state. Before, it warned that Terraform removed it from state, and the apply then failed with "Missing Resource State After Update".
- FIX: Upgrading the state of a router that no longer exists fails with "Resource no longer exists", which says to remove it with `terraform state rm`. Before, it warned that Terraform removed it from state, and the plan then failed with "Missing Upgraded Resource State".

#### resource/coralogix_preset
- FIX: An update of a resource that no longer exists fails with "Resource no longer exists", which says that a refresh (`terraform apply` without `-refresh=false`) removes it from state. Before, it warned that Terraform removed it from state, and the apply then failed with "Missing Resource State After Update".

#### tools/iac-codegen
- CHORE: Bump the generator to `v0.1.0-beta.33`.
- CHORE: Bump the generator to `v0.1.0-beta.32`.
- FEAT: An optional request scalar needs `x-coralogix-presence` only when it declares a default that is not its zero value and the zero value is valid, for example `false` next to a default of `true`. An enum never needs it. Without presence or a default, a scalar whose zero value is valid is handled as if it declared that zero value as its server default: it is `Optional + Computed`, and removing it resets it to zero. A scalar whose zero value is not valid, such as an integer with `minimum: 1`, reads zero as null.
- CHORE: Bump the generator to `v0.1.0-beta.31`.
- FEAT: `readEmptyAs: "null"` on a `keyedBy` map reads an empty API list as null.
- FIX: A skipped empty arm of a top-level oneOf also leaves the resource validator. Before, the validator named an attribute that the schema did not have.
- FIX: `requestValueBy` is refused on a resource that Update patches with an update mask: the mask cannot hold a derived value, so a switch of the object that sets it could be ignored.
- FIX: `keyedBy` with `object: true` needs an arm for every accepted value when the items' oneOf requires an arm. Before, a value without one produced items without their required arm.
- FIX: An `atLeast` bound of an integer attribute must be a whole number in the attribute's range, as for `between`. Before, a bound past the range or past 2^53 rendered a different number.
- FIX: `readEmptyAs: "null"` also applies to a list or set that an `enums` line maps. Before, an empty list read as an empty set where there was no prior value, as in an import, a data source, or a refresh upgrade.
- FIX: `optional: true` on a map that the contract requires sends an empty map when Terraform leaves it out, as it does for a list. Before, the map was sent as null.
- CHORE: Bump the generator to `v0.1.0-beta.29`.
- CHORE: Bump the generator to `v0.1.0-beta.28`.
- FEAT: A new resource gets its docs example, `import.sh`, acceptance test, and registration in `internal/provider/generated/registry.go`. `acceptance.yaml` is optional for it. `make codegen RESOURCE=X OUT=x` runs the generator and the docs. `--out` must be a directory in `internal/provider/generated`.
- FEAT: The generated acceptance test supports immutable fields below the top level.
- FIX: `make generate` works in a directory that is not named `terraform-provider-coralogix`.
- CHORE: Bump the generator to `v0.1.0-beta.27`.
- FEAT: `groups` in `behavior-overrides.yaml` makes required Terraform objects that hold top-level fields of the resource, for a released resource that nests fields the API holds at the top level. The arms of a top-level oneOf in a group check each other like the arms of a nested oneOf; `requireOne: true` makes them ExactlyOneOf, and needs a group that holds the arms of one oneOf and no other field. Flatten fails with "Invalid response from server" unless the response has exactly one of them.
- FEAT: Generation fails when a `groups` name is also the name of a top-level attribute.
- FEAT: `typeName` names the released resource type when it is not the OpenAPI component (`slo_v2` for `Slo`).
- FEAT: `enums.<Enum>.trimPrefix` removes a prefix from each value before the Terraform value is made (`SLO_TIME_FRAME_7_DAYS` is `7_days`).
- FEAT: Field lines take `useNonNullStateForUnknown`, and the validators `between`, `exactlyOneOf`, and `atLeastOneOf`. A `between` bound of an integer attribute must be a whole number that the validator states exactly.
- FEAT: `readEmptyAs: "null"` works on a list or set of strings.
- FEAT: `sendNullAsEmpty: true` sends `""` for a null string, for an API that keeps a stored value that the request omits. It needs `readEmptyAs: "null"`.
- FEAT: `Hooks.BeforeFlatten` changes the API value before flatten, for a value that only an older record has.
- FIX: A component that a request shares with the response keeps its read-only fields in flatten. Before, the read-only field was missing from the shared conversion object, and the apply failed with a value conversion error.
- FEAT: `createAs` on a field line folds top-level Create fields into an object field that Update and Get hold, for example `quota: {createAs: {limit: limitValue}}`. Create sends each property of the object as its flat field.
- FEAT: `optional: true` on a field line makes a field optional that a request of the contract requires.
- FEAT: `collection: set` on a field line makes a list a set, for a released resource whose attribute is a set while the contract has a list.
- FEAT: An `enums` line applies to a list or set of the enum too: each element is mapped, the zero value included, and `enum: true` validates each element. `names` gives an accepted value its released Terraform spelling, for example `SEVERITY_DEBUG: Debug`.
- FEAT: `keyedBy: {key, value}` on a field line shows a list of objects as a map keyed by a string property, for example the events2metric `metric_fields` keyed by `target_base_metric_name`. With `value`, the map holds that one string property, as `metric_labels` does. The API order of the items does not show as a change.
- FEAT: `keyedBy` with `object: true` shows a list of objects keyed by an enum as an object with an attribute per enum value, such as the events2metric `aggregations` (`min`, `max`, `samples`, ...). `arms` keeps in each attribute only the oneOf arm of its value, and `keyAs` sends the attribute name as a property of the item.
- FEAT: An object of a keyedBy object, and a nested attribute of a `computed: true` line, are optional and computed.
- FEAT: `requestValueBy` on a field line sends an API value that the resource derives from the object field that is set, for example the events2metric `type` from `logs_query` or `spans_query`. The field is not a Terraform attribute.
- FEAT: An `atLeast` validator on a field line keeps a released `AtLeast` of a number.
- FEAT: A `regexMatches: {pattern, message}` validator on a field line keeps a released `stringvalidator.RegexMatches` with its message.
- FEAT: `name` on a field line sets the Terraform attribute name of the field, for a released resource whose attribute name differs from the JSON name, for example `applicationnameFilters: {name: applications}`.
- FEAT: A `readOnly: true` line works on a field of a nested component. A `computed: true` line on a field that Get requires allows Create to leave it optional while Update requires it.
- FEAT: `skip` can name an empty object arm of a `oneOf` group that allows no arm, such as an `Empty` marker.
- FIX: The SDK constant of an enum value has "_" between a digit and the letter after it, as openapi-generator writes it (`E2M_TYPE_LOGS2METRICS` is `E2MTYPE_E2_M_TYPE_LOGS2_METRICS`). Before, the SDK check reported the constant missing.
- FIX: An object that a `computed: true` line makes optional and computed is a `types.Object` in the model, as a response-only object is. Before, it was a pointer to a struct, and Create failed with "Received unknown value" when the configuration left the object to the server. Create and Update send it when it is known, and `createAs` and `requestValueBy` read it.
- FEAT: `requireOne` on an arm of a top-level oneOf makes the resource validator ExactlyOneOf. On a field that holds no oneOf, the line is an error; before, it changed nothing.
- FEAT: The attributes of a `keyedBy` object take the `useStateForUnknown` or `useNonNullStateForUnknown` of its line.
- FEAT: The generated acceptance test supports a oneOf group among the top-level fields: it sets the first arm that the file does not skip.
- FIX: A list that the contract requires and an `optional` line relaxes is sent as `[]`, not `null`, when the configuration leaves it out.
- FIX: A `keyedBy` object inside a list or map item generates. Before, the converter looked for an SDK type of the object, which it does not have.
- FIX: The relaxation for a field that Create leaves optional and Update requires needs `computed: true` and a plan modifier that keeps the state value (`useStateForUnknown` or `useNonNullStateForUnknown`). Before, a change of another attribute planned the field unknown, and Update left out a field that it requires.
- FIX: `createAs` makes the object required when Create requires a flat field that it holds, and it refuses two lines that send one flat field. A `createAs` line outside the resource component is refused.
- FIX: `optional` needs a list or a map, and its staleness is checked against the Create and Update requests of the resource. `keyedBy` needs a required `value`, and its object form needs an `enums` line for the key that accepts each value of `arms`. Two `name` lines cannot choose one name.
- FIX: A skip line can name a required property that the request marks read-only, and the empty marker arm of a type that only has the arms of one group.
- FIX: Flatten reads a `format: float` number, alone or in a list, set, or map, as the shortest decimal of the float32, so `0.1` stays `0.1` also on a read without a prior value, such as an import. Before, an import stored `0.10000000149011612`, and the next plan showed an update. The attribute stays a Float32: its semantic equality already kept the configured value after an apply.
- FIX: The contract checks read the resource of a response property written as `allOf: [$ref]`. Before, they read the `allOf` schema, found no Get field, and refused every field of the resource.
- FIX: A skipped top-level field has no lifecycle, so a value that Replace echoes in its body but Create does not have, such as an internal flag, can be skipped. A request that requires a skipped field is an error (`SKIP_REQUIRED_FIELD`).
- FEAT: In existing-resource mode, a response can hold other values next to the resource, such as the ids of objects that the call also changed. The generated code reads only the resource.
- FEAT: `api.serverSetID: true` accepts a server-set id that the Get response does not require, because the request shares the schema. The generated flatten fails when the id is missing. The id must be read-only, or absent, in the Create and Update bodies, also when a body is `allOf: [$ref]`.
- FIX: An Update with the id in its body sends a server-set id, in the resource that the body wraps or in the body itself. Before, no Update field sent it, so the update request had no id.
- FEAT: In existing-resource mode, Create and Update can take one request body type, and a response can hold the resource in a required (value) field.
- FEAT: In existing-resource mode, a read-only field inside a component that the request shares with the response is computed, and the request does not send it. Before, it was an attribute that the user could set, or generation failed.
- FIX: A request body, or a `requestWrapper` property, written as `allOf: [$ref]` reads as the referenced schema in validation and in the model. Before, `clientSetID` and `updateIDInBody` reported the id missing, and the model sent none of the fields.
- FIX: A field line on a component that the user sets in one place gives a read-only copy of that component only its text. Before, `required: true` and `computed: false` failed generation with "required: true needs a field that the user sets", and a `default` or `useStateForUnknown` planned a value the server does not set.
- FEAT: An existing resource whose Update sends the id only in the body (`updateIDInBody`) can have a Create body without the id. The id is computed, and Update sends the value from state.
- FIX: An inline request body gets the `1` suffix only when a component of the same name is in the same per-tag SDK package, as openapi-generator names it, including components that only a parameter of the tag uses.
- FIX: Generated Read, Update, and Delete treat only a resource-not-found 404 (`cxsdk.IsNotFound`) as a missing resource. A 404 of a route that does not exist, for example in a region without the service, is now an error and no longer removes the resource from state. This applies to every generated resource: `coralogix_configuration_group`, `coralogix_connector`, `coralogix_global_router`, `coralogix_preset`, `coralogix_view`, and `coralogix_view_folder`.
- FEAT: A server default can sit next to an enum `$ref`, and Get can state it with the same default instead of `required`. An omitted value then reads as null.
- FEAT: A field that only Update and Get have, with a declared Update default, is managed. The generated Create follows with an Update when the configuration sets another value. An omitted field that Create stored differently is reset to the default, and an unknown configured value stops Create before the API call.
- FEAT: A new resource supports a scalar server default on an optional nested field. The attribute is Optional and Computed, a request does not send its unknown value, and an object whose attributes are empty, `""`, or their default reads as the omitted object.
- FEAT: `validators.inferred: true` keeps the contract limits as validators in existing mode, and a `required: true` line states the presence of a required number in a nested component.
- FEAT: A nested server default also applies to an existing-mode field without a line.
- FEAT: `validators.rejectUnknown: true` gives an existing-mode resource without a released version the unknown-value check of a new resource. `coralogix_view` sets it.
- FIX: A `$ref` with a sibling other than `default`, a descriptive keyword, `x-coralogix-presence`, or a foreign `x-` extension is an error. Before, the type came from the target and the sibling, such as `pattern`, `prefixItems`, or `x-coralogix-collection`, was lost. A default next to a `$ref` is checked against the type and limits of the target.
- FIX: Only an object that holds an attribute with a server default reads `""` or that default as empty. Before, every object did once the resource had a nested default.
- FIX: A server default that Get does not require keeps a configured default when a read omits the value, at the top level and in a nested object. Before, the read stored null and the apply was inconsistent.
- FIX: The Update that follows Create for an Update-only field sends the state id in the body when only the server sets the id, as the normal Update does.
- FIX: A configured value that is still unknown stops Create and Update, also for a nested field with a server default. Before, the request dropped it as if it were omitted.
- FIX: A nested server default inside a set is an error. Flatten cannot match a set element that a read returns without the default to the configured element, so it could not keep the configured default.
- FIX: The unknown-value check and the change check drop server-set values inside a set of objects, as they do in a list or map. Before, an unknown server-set child of a set element blocked Create and Update.
- FIX: A generated Update that gets a resource-not-found 404 returns an error that says the resource no longer exists and that a refresh removes it from state. A state upgrade that reads the resource returns an error that names `terraform state rm`, because a refresh runs the upgrade first. Neither calls `RemoveResource` any more, which the Framework does not allow there. Read still warns and removes the resource, and Delete still treats the 404 as deleted.
- FEAT: `api.idProperty` names the property that holds the id when it is not named like the path parameter, for example `uniqueIdentifier` for `{alert_scheduler_rule_id}`.
- FIX: The SDK check expects a snake_case path parameter in lower camel case, as the Go SDK writes it. Before, it rejected the SDK of any resource with such a parameter.
- FEAT: A field line `valueNames` keeps released names for the numbers of an integer list or set, for example `Sunday` for day `1`.
- FEAT: A field line `asBool` keeps a released bool for an object without fields, for example `always_active = true` for the oneOf arm `alwaysActive: {}`.
- FIX: With `api.idProperty`, an id property that Create does not send is classified as the server-set id, as a path-parameter id is. Before, it was an unsupported field lifecycle. An `idProperty` that names the path parameter is reported as unneeded.
- FIX: A `name` line can take the name of a field without an attribute (skipped, or a `requestValue`), and such fields take part in no name collision.
- FEAT: `types.<Type>.groups` makes required Terraform objects that hold fields of a nested component, as `groups` does for top-level fields, for example `frequency` around the daily, weekly, and monthly arms of a recurring schedule.
- FIX: A renamed top-level field with a server default or an Update-only value keeps its released name in the omitted-default and follow-up Update paths, and in the merged request and response conversion.
- FEAT: `readEmptyAs: "null"` also works on a list or set of strings inside wrapper objects (`unwrap`), for example `alertUniqueIds: {value: []}`.
- FIX: `enums.<Enum>.names` and `zero` cannot give a value the spelling of a rejected value that `readRejected` reads back. `defaultArm` needs a full-replace Update, and expand does not send the default arm while another arm is unknown.
- FIX: A nested group whose model struct name is also the model of a component, the resource, or another group is an error. Before, the group used the other model, and the generated code did not compile.
- FIX: `types.<Type>.groups` of a type that is no component, or no object of the resource, is an error. Before, a misspelled type name dropped the group without a word. `requireOne` on a nested group needs the arms of one oneOf, as on a top-level group; before, it was accepted and the next read failed.
- FIX: A `names` entry that gives the spelling the enum has anyway (lower case, `trimPrefix`, or `verbatim`) is an error, so a rename that a contract change makes stale does not stay unnoticed.
- FIX: A `name` line on a `requestValue` field is an error: the field is no Terraform attribute, so the name changes nothing.
- FEAT: A field line `terraformType: Int64` keeps the released Int64 type of an int32 field, or of a list or set of int32. Expand converts to int32 and reports a value outside its range.
- FEAT: A field line `defaultArm` sends an empty arm when the configuration sets no arm of a oneOf that the API requires, for example `alertUniqueIds: {}` for a released filter without a selector. The arm must be a list of strings in wrapper objects with `readEmptyAs: "null"`, and is sent as the empty outer wrapper (`{}`), not as a wrapper of an empty value (`{"value": []}`). An object arm is refused: it would read back as an object of nulls where the plan has null.

#### resource/coralogix_global_router
- CHORE: The acceptance test compares imported `rules[*].targets` without their order, because the API returns them in no fixed order and an import has no prior order to keep.

#### resource/coralogix_events2metric
- CHORE: Add protocol baseline golden files against an in-memory events2metrics API, checked against answers recorded from a real tenant.
- CHORE: The resource is generated from the API contract. Schema version stays 1, and the HCL does not change. `permutations.limit` is an Int32 and `buckets` a list of Float32, the API types; both stay Terraform numbers, so state and configuration are unchanged.
- FIX: Leaving out `permutations` no longer sends a limit of 0. The API now stores its default of 30000, and an events2metric with `metric_labels` and no `permutations` can be created; before, the API refused it.
- FIX: Removing `description` clears it. Before, the API kept the stored description, and the apply failed with an inconsistent result. Create and Replace now send `""` for a description that is not set, which reads back as null.
- FIX: Switching from `logs_query` to `spans_query`, or back, applies. Before, `lucene` of the new query planned null and the API returned `""`, so the apply failed with an inconsistent result.
- FIX: A `permutations` block without `limit` (`permutations = {}`) no longer sends a limit of 0. Create leaves the limit out, so the API stores its default of 30000, and Replace sends the stored limit. Before, Create and Replace sent 0. Removing `limit` keeps the stored value, as before.
- FIX: State of schema version 0 is upgraded by reading the events2metric from the API. Before, the upgrade kept every metric field under the key `""`, with no source field. An events2metric that no longer exists makes the upgrade fail with "Resource no longer exists", which says to remove it with `terraform state rm`.
- FIX: An events2metric without metric fields or labels reads them as null in the data source and in the v0 state upgrade, as the released resource does. The API returns `[]` for none.
- CHORE: Migration tests also start from provider 3.21.0, and the generated acceptance and 3.21.0 upgrade tests are added.

#### data-source/coralogix_events2metric
- CHORE: Uses the schema and the conversion of the generated resource. `description` still shows the stored value, `""` included, as before.

# Release 3.21.0

#### resource/coralogix_configuration_group
- BREAKING (Beta): `coralogix_fleet_configuration_group` is renamed `coralogix_configuration_group`. Remove the old resource from state and import the group under the new name.
- BREAKING (Beta): The resource is generated from the API contract. A family is either `raw` (its own `remote_configurations`) or `preset` (generated from a configuration template); `remote_configuration` is renamed `remote_configurations`, `agent_selector` holds an `attributes` map, `priority_order` is an Int32, and `chart_name` values are `chart_name_otel_integration`, `chart_name_otel_ecs_ec2`, and `chart_name_otel_{linux,windows,macos}_standalone`.
- FEAT: Preset families: `chart_name`, `chart_version`, `metadata`, and `observability_features`; `integration_version` is resolved by the API when omitted.
- BREAKING (Beta): `name` is required. The family-level `collector_version` is removed: set `family.raw.collector_version`, and read `family.preset.collector_version`, which the API derives from `chart_version`. When `raw.collector_version` is set, every `agent_selector` must include a matching `service.version`.
- FEAT: Read-only `archived` attribute.
- FEAT: `raw_configuration` and `observability_features` compare as YAML and JSON documents, so a change of format plans no change.

#### data-source/coralogix_configuration_group
- BREAKING (Beta): `coralogix_fleet_configuration_group` is renamed `coralogix_configuration_group` and uses the schema of the generated resource.

#### resource/coralogix_preset
- CHORE: Adapt to the management SDK, where preset `name`, `connector_type`, and `entity_type` are required values.
- FIX: An omitted `description` reads back as absent. Before, it read back as `""`, because the API returns an empty string. A configured empty `description` stays `""`.
- CHORE: The resource is generated from the API contract. Create, Read, Update, and Delete use the generated client. Changing `id` still recreates the resource.

#### resource/coralogix_connector
- CHORE: Generated upgrade test covers a connector that already uses `field_values_wo`.
- CHORE: Bump `coralogix-management-sdk` so Connector required fields and unordered field sets come from the OpenAPI contract.
- CHORE: The resource is generated from the API contract. Create, Read, Update, and Delete use the generated client. Write-only `connector_config.field_values_wo` is a generated extra attribute (`WriteOnly: true`); handwritten code only merges secrets into the request and restores versions after read.
- CHORE: Add protocol baseline golden files against an in-memory connector API.
- FIX: Creating a connector without `description` no longer fails.
- FIX: Deleting a connector that the API has already removed succeeds.

#### resource/coralogix_alert
- DOCS: Describe `notification_group.group_by_keys` (combined vs. separate cases).

#### resource/coralogix_global_router
- FIX: A change of `id` replaces the router. Before, the plan showed an in-place update, and the apply failed with `Missing Resource State After Update`.

#### tools/iac-codegen
- CHORE: Bump the generator to `v0.1.0-beta.24`.
- FIX: A `requestValue` field is not in the Create or Update check lists. Those lists read Terraform attributes. A field that the request schema does not mark read-only stayed in the lists, so every Create and every Update failed.
- CHORE: Bump the generator to `v0.1.0-beta.23`.
- FIX: Presence, default, read-only, and schema checks walk a schema that is only `allOf` of one `$ref`. Required fields are the union of the required lists, so a required field inside the ref is not optional.
- FIX: A field that Create requires needs no `x-coralogix-presence` in Update. Terraform requires it, so Update always sends a value.
- FIX: The fields inside a skipped field need no `x-coralogix-presence`, because the field is never sent.
- CHORE: Bump the generator to `v0.1.0-beta.22`.
- FEAT: `enumExceptZero: true` accepts the enum values and leaves out the zero value. `enum: true` still accepts the zero value. The conversion maps still contain it.
- CHORE: Bump the generator to `v0.1.0-beta.21`.
- FIX: A computed-only attribute, and every attribute inside it, has no validators and no document plan modifier. A field line on a component that is also managed no longer copies those checks onto the computed-only place. A computed `id` still keeps `UseStateForUnknown`.
- FIX: `behavior-overrides.yaml` rejects a Delete operation named in both `api.delete.operation` and `api.operations.delete`. Before, `api.operations` chose the operation id while `api.delete` still turned on POST-action delete checks.
- FEAT: `readEmptyAs: null` on a string reads an omitted value as null. A configured `""` stays `""`.
- FIX: `requestValue` on a nested read-only field is sent on create and update.
- FEAT: `api.operations` stores the four operation ids, so `tfgen generate` needs no operation flags.
- FEAT: `requestValue` sends a fixed API value on create and update. The field is not a Terraform attribute.
- FEAT: `readNullAs: empty` reads a missing list or set of objects as an empty collection.
- FEAT: `requireOne` emits `ExactlyOneOf` for the arms of a oneOf.
- FEAT: `readRejected: true` reads a rejected enum value as the API spelling. Without it, that value fails the read.
- FIX: Remove `promote`. `unwrap` shows a one-property wrapper as the plain attribute. A missing enum stays null, including one marked with presence. A string `default` is what that resource reads a missing value as.
- FIX: A `date-time` field of a component has one type in every place of the component: `timetypes.RFC3339` when a request sends it in some place, otherwise a plain string. Before, a field inside a computed value always became `timetypes.RFC3339`, so a response-only time in a component that is also managed elsewhere failed generation with "has a computed object field in one place and a managed one in another".
- FIX: The error for a component that needs another model struct in each place names the field and its Go type in each place.
- CHORE: Bump the generator to `v0.1.0-beta.20`.
- CHORE: Bump the generator to `v0.1.0-beta.19`.
- CHORE: Bump the generator to `v0.1.0-beta.18`.
- FEAT: `unwrap` in `behavior-overrides.yaml` shows a value that the API holds in a one-property wrapper object as the plain value, for example `{"query": {"value": "error"}}` as `query = "error"`. A top-level list names the wrapper components, and an `unwrap: true` or `unwrap: false` field line decides one place. The value can be a scalar, an enum, a list, an object, or a oneOf, a wrapper inside a wrapper, or the items of a list.
- FEAT: Existing mode accepts `api.updateMaskInBody: true` when PATCH `updateMask` is an optional JSON body property. The generated update builds the same body and the same changed-field mask as a query-parameter mask, then sets that mask on the body. The schema has no mask attribute. A mask that names only top-level fields still cannot change a nested oneOf.
- FIX: `api.updateMaskInBody: true` on a full replace (PUT) is an error. Before, generation ignored the key and produced a replace that does not set a mask.
- CHORE: Bump the generator to `v0.1.0-beta.17`.
- FEAT: A `date-time` field that Create or Update sends, alone or in a list or set, is supported. It is a `timetypes.RFC3339` attribute that must be in UTC (`Z`). A time that only the response has stays a plain string.
- CHORE: Bump the generator to `v0.1.0-beta.16`.
- FEAT: A string `pattern` in Create, Update, and the response becomes a `stringvalidator.RegexMatches` validator, also on list items and map values. Only a pattern that Go cannot compile, such as a lookahead, or a pattern on an enum blocks generation. The acceptance test builds a matching value when its made-up string does not match, and generation stops when no value fits.
- FEAT: A Delete can return `google.protobuf.Empty`, which the OpenAPI fork writes as an inline object with no fields.
- CHORE: Bump the generator to `v0.1.0-beta.15`.
- FEAT: `upgradeCases` in `acceptance.yaml` adds extra upgrade subtests. An extraAttribute is included when `values` or a case supplies its HCL, so a write-only map can use a real field name instead of a made-up `key`.
- CHORE: Bump the generator to `v0.1.0-beta.13`.
- FEAT: Support nested objects whose Create, Update, and response schemas differ. A nested field that only the response has is computed. A nested field that Create has and Update does not is immutable. A type mismatch now names the exact nested field. A nested one-of can require an arm in Create and allow no arm in Update and the response.
- FIX: An immutable nested value that holds computed fields no longer replaces the resource on every change. The replace check now compares only the values that Create sends.
- FEAT: A missing `required` list means that no field is required. The OpenAPI fork cannot write an empty list, so an all-optional message, such as a PATCH body, was ineligible. A `required: []` line in `behavior-overrides.yaml` is now unused and reported.
- FEAT: A string `pattern` that only the resource response has, for example on a server-set `id`, no longer blocks generation.
- FIX: Reject an immutable value that holds an optional and computed field from `behavior-overrides.yaml`. An omitted one would replace the resource on every change.
- FIX: The generated acceptance test treats an immutable value with server-set fields as immutable. Before, it could expect an update where the plan replaces the resource.
- FEAT: A field with a server default no longer needs the default on its Update schema. The generated resource reads only the Create default. An Update default that differs from the Create default is still rejected.
- FEAT: `equality: yaml` or `equality: json` on a string field compares its value as a document, so a reformatted YAML or JSON value does not plan a change.
- FIX: A resource in existing mode keeps the configured form of an empty list, set, or map. An omitted list or map that the API returns empty no longer fails the apply with `was null, but now` an empty value.
- FEAT: `keepPriorOrder` works on a list whose request items use another component than its response items, such as a Create item without the server-set id and hash. The items pair on the fields that the request sends.
- CHORE: Bump the generator to `v0.1.0-beta.12`.
- FEAT: A `default` line in `behavior-overrides.yaml` accepts a number for an `Int64`, `Int32`, `Float64`, or `Float32` attribute, for example `{computed: true, default: 0}`. Before, it accepted only a string or a bool.
- FIX: In existing mode, a top-level field with a server default in the contract uses that default when `behavior-overrides.yaml` has no `default` or `computed: false` line for it. Before, the field became plain `Optional`, and an apply without it failed with `was null, but now` the default.
- CHORE: Bump the generator to `v0.1.0-beta.6`.
- CHORE: Bump the generator to `v0.1.0-beta.7`.
- CHORE: Bump the generator to `v0.1.0-beta.8`.
- CHORE: Bump the generator to `v0.1.0-beta.11`.
- CHORE: Bump the generator to `v0.1.0-beta.10`.
- FIX: The generated acceptance test leaves out `types.<Type>.extraAttributes`. A made-up map key such as `key` is not an API field.
- CHORE: Bump the generator to `v0.1.0-beta.9`.
- FEAT: `types.<Type>.extraAttributes` declares Terraform-only map attributes, including `writeOnly: true`. The archived-thing golden fixture covers the shape.
- FEAT: Existing-resource `NewResource` takes `Hooks{BeforeWrite, AfterRead}` so overlays pass write-only logic at construction instead of package `init`.
- FEAT: Existing-resource `lengthAtLeast` keeps a released non-empty string validator.
- FEAT: Existing-resource mode accepts a custom-method lifecycle: Get and Delete each carry the id, and Update keeps the id in the body.
- FEAT: `verbatim` on an enum keeps the API spelling as the Terraform value.
- FEAT: Generate unordered sets of objects. The Thing golden fixture includes a set of objects.
- FIX: Namespaced OpenAPI components such as `notification_center.ConnectorConfigField` render as valid Go model types.
- FIX: A nested object collection marked `x-coralogix-collection: set` now flattens as a set in the generated attr.Type map, matching the schema.
- FIX: A client-set `id` (`clientSetID: true`) has `RequiresReplace`. Before, a change of `id` planned an in-place update that the API answered with 404.
- FEAT: A `useStateForUnknown` key that the generator already applies, for example on a client-set `id`, is reported as `OVERRIDE_UNUSED`.
- CHORE: Bump the generator to `v0.1.0-beta.14`.

# Release 3.20.0

#### resource/coralogix_alert
- NOTE: The API now rejects an alert when `notification_group.router.notify_on` differs from `incidents_settings.notify_on`. Set both to the same value. If `router` is set without `notify_on`, the provider sends `"Triggered Only"`, so an alert with `incidents_settings.notify_on = "Triggered and Resolved"` fails too. Set `router.notify_on = "Triggered and Resolved"` for such alerts.

#### resource/coralogix_grafana_folder
- FIX: Handle out-of-band folder deletion on read — `terraform plan` now warns and recreates instead of erroring.
- FIX: Mark uid as ForceNew. The Grafana API cannot update a folder's uid, so a uid change now replaces the folder, which also deletes the dashboards inside it.

#### tools/iac-codegen
- FEAT: Add the fail-closed beta generator and local candidate eligibility checks for complete Terraform Plugin Framework resources.
- FEAT: Add existing-resource mode. A `behavior-overrides.yaml` file lists where a released resource differs from the API contract, and the generator keeps that behavior. A line that matches nothing in the contract is an error.
- FEAT: Generate an acceptance test (`acceptance_test.go`) from an `acceptance.yaml` file. It creates, imports, updates, and sets the minimal config of the resource, and can test the upgrade from a released provider.

#### resource/coralogix_global_router
- CHORE: The resource is generated from the API contract. The schema, the docs text, and the behavior are unchanged, except for the changes below.
- FIX: A rule without `entity_type` no longer fails. The provider sends no value, and the API uses `ALERTS`. Before, the provider sent `unspecified`, and the API refused it.
- FIX: A router without `description` no longer fails with "inconsistent result after apply". `description` is now `Optional` and `Computed`. To clear it, set `description = ""`.
- FIX: `fallback` and `fallback_targets` keep the order of the configuration when the API returns the same items in another order. Before, a reorder failed the apply.
- FIX: Removing `rules` from the configuration keeps the existing rules, as removing `entity_labels` does. Before, it removed all rules. To remove all rules, set `rules = []`.
- FEAT: Add the computed attributes `create_time` and `update_time`.
- CHORE: Error messages have new wording.

#### data-source/coralogix_global_router
- FEAT: Add the computed attributes `create_time` and `update_time`.

#### resource/coralogix_alert
- FEAT: Add `case_settings` (preview) so case settings (auto-resolve mode, enrichment query, case destinations) can be managed in Terraform and are kept across updates. Also flows through to `data-source/coralogix_alert`.

# Release 3.19.0

#### resource/coralogix_events2metric
- FIX: Histogram `buckets` no longer show a perpetual diff (and Replace) for decimals like `0.0003`. Flatten now maps each float32 bucket back to its shortest round-tripping decimal, so state converges to the configured value instead of drifting.

#### resource/coralogix_user
- CHORE: Migrate off SCIM to the Users OpenAPI (`/aaa/users/v2`). HCL is unchanged: same attributes, and `id` stays the user UUID, so existing state refreshes without a migration and import by id still works.
- CHORE: `emails` is derived from `user_name` as one primary `work` entry, which is what SCIM returned. `groups` comes from the user's group ids.
- CHORE: Create sends no login mode. Update and destroy send back the login modes and access type the user already has, so they are never wiped.
- CHORE: Destroy deactivates the user, as the SCIM delete did.
- CHORE: API-key permissions are unchanged. The Users API checks `team-members:ReadConfig` and `team-members:Manage`, the same permissions SCIM user provisioning needed.
- FEAT: Import by email as well as by user id: `terraform import coralogix_user.example someone@example.com`. Import by email takes one lookup.

#### data-source/coralogix_user
- CHORE: Read users through the Users OpenAPI instead of SCIM. Lookup by `id` or by `user_name` is unchanged, and `user_name` matching stays exact and case-insensitive.

#### resource/coralogix_data_set
- FIX: A file read error in `uploaded_file` returns a diagnostic instead of crashing the provider.
- FIX: Import rejects non-numeric IDs.

#### resource/coralogix_alert
- FEAT: Add support for the `type_definition.analytics_immediate` and `type_definition.analytics_threshold` alert types (preview), which trigger off a DataPrime query. Also flows through to `data-source/coralogix_alert`.

#### resource/coralogix_enrichment
- FIX: Roll back enrichments when the Add fails after the Delete during an update. The update deletes the existing enrichments and re-adds the new set in two separate calls; if the Add failed, the resource was left with all of its enrichments removed while state still showed them. The old set is now best-effort re-added on Add failure, the original Add error is still returned, and a rollback failure is reported as a separate diagnostic. The enrichments to delete are now taken from the pre-change configuration, so emptying or shrinking a fields set actually removes those enrichments from the backend.

# Release 3.18.0

#### provider
- FIX: Report an error instead of crashing when an API call returns no HTTP response, for example on a network error or an import with a malformed ID. Affects `coralogix_dashboards_folder`, `coralogix_scope`, `coralogix_ip_access`, `coralogix_action`, `coralogix_tco_policies_logs`, `coralogix_tco_policies_traces`, `coralogix_data_enrichments`, `coralogix_integration`, `coralogix_webhook`, `coralogix_archive_logs`, `coralogix_archive_metrics`, `coralogix_global_router`, `coralogix_preset`, `coralogix_parsing_rules`, `coralogix_recording_rules_groups_set`, and `coralogix_slo_v2` resources and data sources.

#### resource/coralogix_alert
- FIX: Add `notification_group.router.id`. Import and refresh now read the router ID, and a change to it shows in the plan. Before, every update silently set the ID to `""`, which moved the alert to label-based Global Router matching.
- NOTE: `router = {}` still means label-based matching (`id = ""`), as in 3.5.0. After the upgrade, an alert that still uses `router_default` plans `id: "router_default" -> ""`. To keep that router, set `id = "router_default"`. This affects alerts last applied with 2.0.18 to 3.4.2, and alerts whose router was set outside Terraform.
- NOTE: This fix does not restore router IDs that an earlier apply already set to `""`. Set `id` again where the alert must use a specific router.

#### data-source/coralogix_alert
- FEAT: Expose `notification_group.router.id`.

#### resource/coralogix_global_router
- FIX: Keep `rules[*].targets` in configuration order. The API can return targets in a different order, which failed applies with "Provider produced inconsistent result" and showed reorder-only diffs on the next plan. Targets with the same `connector_id` and `preset_id` are told apart by `custom_details`.

#### resource/coralogix_recording_rules_groups_set
- FEAT: Add support for `groups[*].rules[*].evaluation_delay_ms`, an optional delay (in milliseconds, `0`-`1800000`) that holds off a rule's evaluation so late-arriving data is ingested first. Removing it from the configuration clears the delay, and an explicit `0` is a real value distinct from unset. Also fixes a read-path gap: a delay set outside Terraform was previously dropped from state and silently stripped on the next apply.
- DOCS: Note in `yaml_content` that keys must be all-lowercase and unseparated (`evaluationdelayms`), since unrecognized keys are ignored without a diagnostic.

#### resource/coralogix_rules_group
- DOCS: Add a complete guide to migrate to `coralogix_parsing_rules` without replacing the remote rule group.

#### data-source/coralogix_rules_group
- DOCS: Document the new data-source type, output shape, field rename, and canonical enum values.

#### resource/coralogix_parsing_rules
- FIX: Preserve rule subgroup IDs in state.

#### data-source/coralogix_parsing_rules
- FIX: Preserve rule subgroup IDs in state.

#### resource/coralogix_custom_role
- FIX: Read no longer errors when the API returns a different set of permissions than the plan/state. Permissions are reconciled from the API response, so drift caused by UI edits or partial failures is surfaced in the plan and corrected on the next apply.

#### resource/coralogix_dashboard
- FEAT: Report backend validation issues as warnings during `terraform plan`. The create API only validates structure, so problems such as stale variable references or duplicate widget ids used to surface only when the dashboard was rendered. Issues never fail a plan. Set `CORALOGIX_DASHBOARD_VALIDATION=false` to turn validation off, or `CORALOGIX_DASHBOARD_VALIDATION_TIMEOUT` to change the 5s call timeout.

#### resource/coralogix_enrichment
- DOCS: Add a complete guide to migrate to `coralogix_data_enrichments` without replacing the remote enrichment.
- FIX: Send the public `aws.fields.resource` value when the legacy AWS enrichment resource creates or updates a field.

#### data-source/coralogix_enrichment
- DOCS: Document the new data-source type and output shape.

#### resource/coralogix_data_enrichments
- FIX: Keep imported enrichment type IDs unique and load only the requested custom field mappings after a numeric custom-enrichment import.
- FIX: Allow `enriched_field_name` to be absent so legacy enrichments with a null API value can be migrated without changing them.
- FIX: Preserve field mappings when an update changes only custom enrichment data or metadata.
- FIX: Keep parallel custom enrichment create and update responses isolated by numeric custom-enrichment ID.
- FIX: Recover the canonical resource ID after a failed update leaves a null ID in state.
- FIX: Keep each Geo IP field's `with_asn` value independent when multiple fields are configured.
- FIX: Recover enrichment types from state blocks when a failed update leaves a null resource ID.
- FIX: Deduplicate legacy enrichment type IDs during read so repeated mappings do not duplicate state fields.
- FIX: Require `custom_enrichment_data` when a `custom` block is configured, and guard against nil pointer dereference panics in `Create` and `Update` when it is omitted; validation defers when `custom` or `custom_enrichment_data` is not yet known at plan time.
- FIX: Restore the removed enrichments on a best-effort basis when `AddEnrichments` fails during an update, so a partial failure does not leave the enrichments deleted.

#### data-source/coralogix_data_enrichments
- FIX: Preserve the configured lookup ID, load only the requested custom field mappings for a numeric ID, and correct the example IDs.

#### resource/coralogix_data_set
- DOCS: Map every resource field to `coralogix_data_enrichments`, including both CSV input forms.

#### data-source/coralogix_data_set
- DOCS: Document the replacement data source and every output path, and point the deprecation message to `coralogix_data_enrichments`.

#### resource/coralogix_archive_retentions
- FIX: Guard `httpResponse` against nil in `Read` to prevent a nil pointer dereference panic when the API call returns a transport error.
- FIX: Prevent an index-out-of-range panic in `Create` when the backend returns fewer archive retentions than the configuration declares; the mismatch now surfaces an error diagnostic instead.

#### resource/coralogix_connector
- FIX: Guard `httpResponse` against nil in `Read` and `Update` to prevent a nil pointer dereference panic when the API call returns a transport error. Mirrors the archive_retentions fix (#722).
- DOCS: Document the optional `additionalDetail` connector_config field for the `eventbridge` connector type.

#### resource/coralogix_custom_role
- FIX: Panic on transport errors during read and update.
- FIX: Panic when the create response omits the role ID.

#### resource/coralogix_view_folder
- FEAT: New resource for View Folders, the containers that group saved views in the Explore screen. Supports `name` (required, 1-100 characters), the computed `id`, full CRUD with in-place rename, and import by ID.

#### data-source/coralogix_view_folder
- FEAT: New data source resolving a view folder by either `id` or `name`.

#### resource/coralogix_action
- FEAT: Add support for `description`. Removing it from the configuration clears the value.
- FEAT: Add support for `dpxl_filter`, a DPXL expression that scopes when the action is offered. The expression must include a `<v1> ` version prefix (consistent with the other DPXL fields in the provider). Removing it from the configuration clears the value.
- FEAT: Add support for `url_fields`, an ordered list of `{name, required}` declarations for the `{{placeholder}}` slots in `url`.

#### data-source/coralogix_action
- FEAT: Expose `description`, `dpxl_filter` and `url_fields`. The data source echoes the backend values, so `dpxl_filter` reads back with its `<v1> ` prefix.

# Release 3.17.0

#### resource/coralogix_archive_metrics
- FIX: Prevent a nil pointer dereference panic in `Read`/`Create` when the backend returns an empty tenant configuration or one using an unsupported storage backend; both now surface an error diagnostic.
- FIX: Persist a partial state carrying the resource ID after a successful create mutation before the follow-up read, so a failing read no longer orphans a resource that already exists in the backend.

#### resource/coralogix_fleet_configuration_group
- FIX: Keep `family.remote_configuration` in configuration order when the API returns remotes in a different order.

#### resource/coralogix_slo_v2
- FEAT: Add support for `sli.apm_sli`, a third SLI branch that builds the SLO from an APM Service Catalog service instead of PromQL, with `services`, `error_config`, `latency_config` (`time_window`, `threshold`, `quantile.percentile`, `average`), `filters` and `grouping_keys`. APM SLOs can now also be read and imported; a read used to fail outright.
- FEAT: Add support for `product_type` (`unspecified`, `apm`). It is `Optional+Computed`, so removing it from the configuration keeps the last applied value - set `product_type = "unspecified"` to reset it.
- FEAT: Add support for `ownership_tags`, with independent `service`, `environment` and `team` dimensions carrying `static_values` or `label_keys` plus the computed `resolved_values`. Removing the block clears the tags.
- FEAT: Add support for `sli.window_based_metric_sli.missing_data_strategy` (`uncounted`, `good`, `bad`). It is `Optional+Computed`, so removing it from the configuration keeps the last applied value - set `missing_data_strategy = "uncounted"` to reset it.
- FEAT: Add the computed `apm_sli_metadata`, the backend's read-only copy of `sli.apm_sli`.
- FIX: Reject `unspecified` for `sli.window_based_metric_sli.window` and `sli.apm_sli.latency_config.time_window`. The backend has no implementation for that value and fails the create with an HTTP 500; the error now surfaces at plan time instead.
- FIX: Require exactly one of `sli.apm_sli.latency_config.quantile` or `sli.apm_sli.latency_config.average`. Setting neither was rejected by the API with `Latency query type must be specified`, and setting both failed to serialize; both now fail at plan time.
- FIX: Require exactly one of `sli.apm_sli.error_config` or `sli.apm_sli.latency_config`. Setting neither was rejected by the API with `Unsupported APM SLI type: undefined`, and setting both failed to serialize; both now fail at plan time.

# Release 3.16.0

#### resource/coralogix_archive_logs
- FIX: Guard `httpResponse` against nil in `Read` and `Update` to prevent a nil pointer dereference panic when the API call returns a transport error.


#### resource/coralogix_alert
- FIX: Omitting `labels` now clears them on update. The attribute was `Optional+Computed` with `UseStateForUnknown()`, so removing it from HCL copied the prior map into the plan and Replace kept the old labels. An explicit empty map (`labels = {}`) is rejected at plan; omit the attribute to clear.

#### resource/coralogix_alerts_scheduler
- FIX: `description` is now Optional+Computed with a default of `""`, so omitting or removing it produces a zero-diff plan instead of perpetual drift.

#### resource/coralogix_tco_policies_traces
- FEAT: Add support for `policies[*].dpxl_expression`, a DataPrime matcher that is mutually exclusive with `services`, `actions`, `tags`, `applications` and `subsystems`.
- FEAT: Add support for `policies[*].quota_based_priority_override`, an ordered list of `usage_tiers` that reassigns the policy's priority as its daily quota is consumed. `priority` is the fallback once all tiers are exhausted.

#### resource/coralogix_tco_policies_logs
- FIX: `policies[*].dpxl_expression` now conflicts with `applications` and `subsystems` at plan time, matching the API which rejects the combination.

#### resource/coralogix_tco_policies_rum
- FIX: `policies[*].dpxl_expression` now conflicts with `applications` and `subsystems` at plan time, matching the API which rejects the combination.

#### data-source/coralogix_api_key
- FIX: `access_policy` docs no longer describe the resource-only empty-string clear contract.

#### resource/coralogix_api_key
- FIX: Destroy converges when the backend answers `404 Not Found`.
- FIX: Clearing `access_policy` now requires setting it explicitly to an empty string (`""`); omitting the attribute preserves the existing policy. The empty string is sent to the backend to clear the policy, and this contract is documented in the attribute description.
- FIX: `access_policy` JSON that differs only in whitespace or key order no longer fails apply with "inconsistent result after apply". State keeps the configured text when it is JSON-equivalent to the backend value.
- FIX: `Read` now emits a warning before removing the resource from state when the backend returns `404 Not Found`, so a silently-recreated key is surfaced to the user.
- FIX: `coralogix_api_key` update now applies `access_policy` changes and clears a removed policy, while omitting the field (rather than clearing it) when the planned value is unknown.
- FIX: `coralogix_api_key` guard nil HTTP response on update
- FIX: `coralogix_api_key` keep state when read fails with a transient error, only recreate on 404

#### resource/coralogix_custom_role
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_scope
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_team
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_ip_access
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_dashboards_folder
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_slo_v2
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_integration
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_preset
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_global_router
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_recording_rules_groups_set
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_data_enrichments
- FIX: Destroy converges when the backend answers `404 Not Found`. The delete error message also correctly reads "Error deleting" instead of "Error reading".

#### resource/coralogix_user
- FIX: Destroy converges when the backend answers `404 Not Found`.

#### resource/coralogix_group
- FIX: Destroy converges when the backend answers `404 Not Found`.

# Release 3.15.0

#### resource/coralogix_alert
- FIX: Destroying an alert that was already deleted outside Terraform failed the apply. A delete that the backend answers with `404 Not Found` now converges instead of raising a blocking diagnostic. The delete error message also correctly reads "Error deleting alert" instead of "Error reading alert".

#### resource/coralogix_fleet_configuration_group
- FEAT: Add private preview (Beta) `coralogix_fleet_configuration_group` resource and data source for Fleet Manager configuration groups. Destroy deactivates the latest family and then archives the group. Omitting `family.collector_version` keeps the existing remote value.
- DOC: Example loads collector YAML from files and includes agent plus cluster-collector remotes.

#### resource/coralogix_action
- DEPRECATE: Warn that `is_hidden` is a per-user UI preference, not a property of the action, and will be removed in a future version.
- FIX: Nil pointer dereference when a network failure interrupts a read or an update. A DNS failure, dial timeout, or connection refused leaves the HTTP response nil, which the not-found check dereferenced; the provider now reports the transport error as a diagnostic and keeps the resource in state instead of crashing.
- FIX: Destroying an action that was already deleted outside Terraform failed the apply. A delete that the backend answers with `404 Not Found` now converges instead of raising a blocking diagnostic.

#### data-source/coralogix_action
- DEPRECATE: Document that `is_hidden` is a per-user UI preference, not a property of the action, and will be removed in a future version.

#### data-source/coralogix_rules_group
- DEPRECATE: Warn that this data source is deprecated and will be removed in a future version. Use `coralogix_parsing_rules` instead.

#### resource/coralogix_rules_group
- DOC: The deprecation warning now says the resource will be removed in a future version.

#### resource/coralogix_enrichment
- DOC: The deprecation warning now says the resource will be removed in a future version.

#### data-source/coralogix_enrichment
- DOC: The deprecation warning now says the data source will be removed in a future version.

#### resource/coralogix_data_set
- DOC: The deprecation warning now says Data Sets will be removed in a future version.

#### resource/coralogix_alert
- FIX: Nil pointer dereference when a network failure interrupts a read or an update. A DNS failure, dial timeout, or connection refused leaves the HTTP response nil, which the not-found check dereferenced; the provider now reports the transport error as a diagnostic and keeps the resource in state instead of crashing.

# Release 3.14.1

#### provider
- CHORE: Bump `coralogix-management-sdk`. The OpenAPI ClientSet now exposes Identity and Users clients, which the `coralogix_user` migration off SCIM needs. No user-facing behaviour changes.

# Release 3.14.0

#### resource/coralogix_connector
- FEAT: A connector field's value can be supplied write-only, so the secret is sent to the API and never written to state. Set it in `connector_config.field_values_wo`, keyed by field name, with a matching entry in `connector_config.field_values_wo_versions`; leave that field out of `fields`. Terraform holds no copy of a write-only value, so it cannot notice that a secret changed: increment the version to send a rotated one. Requires Terraform 1.11 or later. Existing configurations are unaffected. Note that importing a connector brings the secret into state, because an import has neither configuration nor prior state to say which field is managed this way; one apply afterwards removes it again. This reduces what is stored in state and does not replace securing the state file. Reading the same connector through `data.coralogix_connector` still returns the value, because a data source reads from the API and has no configuration telling it which value is managed write-only.
- FEAT: Warn when a field value that looks like a secret is set through `connector_config.fields` rather than `field_values_wo`, naming the field to move. Importing a connector warns too, since the import is the point at which a secret is written to state.
- FIX: Reject a null `connector_config.field_values_wo_versions` entry. It recorded nothing, so a later change to the secret alone was never sent. An empty `field_values_wo` no longer requires a versions map either, which a module passing a variable hits on its default. A null `field_values_wo` value is rejected as well, rather than aborting the apply with a value conversion error.
#### resource/coralogix_webhook
- FEAT: A webhook credential can be supplied write-only, so the secret is sent to Coralogix and never written to state. Use `jira.api_token_wo` with `jira.api_token_wo_version`, `pager_duty.service_key_wo` with `pager_duty.service_key_wo_version`, or `custom.headers_wo` with `custom.headers_wo_versions` for individual secret headers. Terraform holds no copy of a write-only value, so it cannot notice that a secret changed: increment the version to send a rotated one. Requires Terraform 1.11 or later. Existing configurations are unaffected. Note that importing a webhook brings the secret into state, because an import has neither configuration nor prior state to say which value is managed this way; one apply afterwards removes it again. This reduces what is stored in state and does not replace securing the state file. Reading the same webhook through `data.coralogix_webhook` still returns these values, because a data source reads from the API and has no configuration telling it which value is managed write-only.
- FEAT: Warn when a credential is set through an ordinary attribute rather than its write-only counterpart, naming the attribute to use instead. Importing a webhook warns too, since the import is the point at which a credential is written to state.
- FIX: Accept an empty `custom.headers_wo` without requiring `custom.headers_wo_versions`, which a module passing a variable hits on its default. Mismatched keys are still rejected. A plan no longer reports a missing version when only one of the two maps is still unknown; the comparison waits until both resolve. A null version entry is rejected: it records nothing, so a later change to the secret alone would never be sent. A null `custom.headers_wo` value is rejected as well, rather than being sent as an empty header.

#### resource/coralogix_quota_allocation_rule_set
- FEAT: Allow `rules = []` to clear all customer-managed quota allocation rules.
- FEAT: Document the full known entity-type list (including `browserLogs/v2`) and note that the list is additive.
- FEAT: Document `locked_units`, the percentage pool after locked units, `cx_managed` filtering, and delete semantics.
- FIX: Keep `allocation_type = "unspecified"` as a compatible alias of `percentage`, and normalize it during planning. API `UNSPECIFIED` still flattens to `percentage`.
- FIX: Stop sending Coralogix-managed rules on create/replace/delete. Customer requests now include only customer rules; the API preserves `cx_managed` rules.

#### data-source/coralogix_quota_allocation_rule_set
- FEAT: Document known entity types, `locked_units` vs percentage pool (including bundle-unit fit), and `cx_managed` semantics.

# Release 3.13.0

#### resource/coralogix_dashboard
- FEAT: A widget takes `custom_actions`, each opening a URL from that widget. The Coralogix UI offers these in the widget's settings panel and the API stores them, but the provider had no attribute, so a dashboard using them could not be written and an imported one lost them. The API keeps them on the dashboard, each tagged with the id of the widget it belongs to; they are nested under the widget here because that is where the UI offers them and because a widget id is usually generated, so it cannot be referenced from elsewhere in the same configuration. `data_source` is required: the API requires it, rejects its unspecified value, and does not check it against the widget, so set it to match the widget's own query. Omit the attribute rather than setting an empty list, which is rejected because the API stores no actions and an empty list would read back as null.
- FEAT: A layout section takes a `repetitive_var`, repeating the section once per selected value of a dashboard variable. The Coralogix UI offers this as "Add Repetition" and the API stores it, but the provider had no attribute, so a repeated section could not be written and an imported one lost the setting.
- FEAT: `content_json` now warns about fields the provider does not recognise, naming each one by its JSON path. Such fields were silently discarded, so a misspelled field looked applied but never reached the API. The check mirrors the decoder's own field matching, including its acceptance of both `camelCase` and `snake_case` spellings, so it does not warn about anything the provider actually keeps. It is a warning rather than an error, because a field can also be newer than the Coralogix SDK the provider is built against, which a configuration change cannot fix.
- FIX: A section whose options use the API's unnamed-default arm now round-trips. The Coralogix UI stores that arm for a dashboard whose sections were never named, and the provider read the whole options block back as absent, so the next apply cleared it. Express it as `options = { internal = {} }`. `name` is optional now so that arm is writable; a section that is not `internal` still requires a name, which the schema enforces because the API rejects an otherwise empty one.
- FIX: A metric annotation keeps its `orientation`. The Coralogix UI offers `vertical` and `horizontal` on a metrics-source annotation and the API stores the choice, but the provider had no attribute, so a horizontal annotation read back as vertical and the next apply reset it. Manual and Dataprime annotations already had the attribute. Omit it to accept what the API returns, which is `vertical`.
- CHORE: Correct two stale entries in the dashboard API branch-coverage manifest. The bar chart `xaxis.time_buckets` branch was still recorded as unsupported although it shipped earlier, and the section options unnamed-default branch was recorded as deliberately ignored. The manifest only failed on a missing entry, never on an untrue one, so both went unnoticed.

#### provider
- FIX: A malformed `domain` is now reported as a configuration error instead of becoming a malformed API host. A value such as `" "` used to produce the gRPC target `ng-api-grpc.:443` and fail later as an opaque dial error. The check is structural only — it rejects what cannot be a host at all, and accepts AWS PrivateLink hosts, customer-specific private hosts, and internal names that use underscores or non-ASCII labels — and it covers the `domain` argument and the `CORALOGIX_DOMAIN` environment variable on both halves of the muxed provider.
- FIX: The `domain` argument's conflict rule named `domain` instead of `env`, so it never emitted anything. Setting `env` and `domain` together was already rejected by `env`'s own rule; both attributes now report the conflict.

# Release 3.12.0

#### resource/coralogix_dashboard
- FIX: An annotation keeps its `color`. The Coralogix UI offers a swatch per colour and the API stores the choice, but the provider had no attribute, so a copy of a red annotation was created as unspecified and the colour was silently lost. Valid values are `default`, `green`, `cyan`, `blue`, `purple`, `magenta`, `red`, `orange` and `yellow`. Omit the attribute for an annotation with no colour chosen: that is what the API returns, and it reads back as no value.
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
- FIX: A `gauge` keeps its `arc_display`, and a `gauge` `min` and `max` and a `data_table` column `width` no longer declare a static default. All three are wrapper values in the proto, so an omitted one is absent and not zero: the defaults of 0, 100 and 0 sent a value the dashboard never had, which showed up as a change on the first plan after an import. All three keep the value the API returns instead, so a dashboard created outside Terraform plans clean. Note on upgrade: a configuration that left `min`, `max` or a column `width` out and relied on the old defaults keeps the value already in state. Set the value explicitly to change it.
- FEAT: A dashboard `auto_refresh` accepts `one_minute` and `fifteen_minutes`. The API models five intervals, each as its own field, and the Coralogix UI offers all five, but the provider had only `off`, `two_minutes` and `five_minutes`. A dashboard set to one of the other two could not be written or read. The expand now clears all five fields before it sets one, so a change of interval cannot leave two set.
- FEAT: A widget takes a `highlighted` flag. The Coralogix UI marks a widget as highlighted for every user of the dashboard and the API stores it, so the flag was lost on every apply. The API returns a value for every widget, so the attribute is computed and keeps its last value: set `highlighted = false` to stop highlighting a widget, rather than deleting the line. The API rejects the flag on a widget that only holds a `reference`, so `highlighted = true` next to a `reference` is now rejected at plan time instead of failing the apply.
- FIX: A `horizontal_bar_chart` logs query now sends `group_names_fields` and `stacked_group_name_field`. The schema had both and the read filled them in, but the request never carried them, so a configuration that set either failed the apply with "provider produced inconsistent result after apply" and the value was silently never stored. The other five chart queries already sent them.
- FIX: A dashboard whose state is still at schema version 1 can be upgraded again. Version 1 is what a provider older than v1.13.1 wrote. The upgrader handed the stored `layout`, `variables`, `filters` and `annotations` to the current schema unchanged, but those attributes have a different type today, so the upgrade failed with a value conversion error and no plan was possible. Version 1 now refreshes the dashboard from the API and rebuilds every attribute at the current type, the same way version 2 and version 3 already do. A dashboard that no longer exists keeps a minimal state with a warning and is recreated on the next apply.

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
