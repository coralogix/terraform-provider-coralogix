# Scope protocol baseline

These goldens record the handwritten resource and data source on master at
`8dc5b65e`. Record them before replacing that implementation. Keep existing bugs
in this baseline; a later migration must show each fix in its golden diff.

## Coverage

| Contract | Protocol scenarios |
| --- | --- |
| Resource and data-source schema, validators, plan modifiers | `schema_current.txt`, `datasource_schema.txt` |
| Create, refresh, follow-up plan, update every writable field, delete | `minimal`, `full-lifecycle` |
| Description omitted on create, changed, removed on update | `minimal`, `full-lifecycle` |
| Explicitly empty description, including import and data source | `empty-description`, `import-empty-description` |
| Change nonempty description to an empty string, refresh, replan, data source | `clear-description` |
| Empty display name on create, update, refresh, import, and data-source read | `empty-display-name` |
| Nonempty and explicitly empty filters | `full-lifecycle`, `empty-filters` |
| Clear nonempty filters on update, refresh, replan, data source | `clear-filters` |
| Enum values `logs`, `spans`, and backend-rejected `unspecified` | `minimal`, `full-lifecycle`, `unspecified-entity-type` |
| Unversioned and empty filter expressions reach the API and fail | `unversioned-filter-expression`, `empty-filter-expression` |
| Loose released default-expression validator reaches an API error | `loose-default-expression-validator` |
| Released regex accepts bare `false`, a prefix before `false`, and repeated final `e` | `bare-false-default-expression`, `prefixed-false-default-expression`, `repeated-e-default-expression` |
| Required fields, invalid default-expression prefix, invalid enum | `config-validation` |
| Import, empty import ID, version-zero state | `import`, `import-empty-id`, `upgrade-version-zero` |
| Missing resource and missing route | `read-not-found`, `update-not-found`, `delete-not-found`, `route-missing`, `delete-route-missing` |
| Injected API failures | `create-api-error`, `read-api-error`, `update-api-error`, `delete-api-error` |
| Transport failures without an HTTP response, followed by recovery | `transport-create`, `transport-update`, `transport-delete`, `transport-read`, `transport-data-source` |
| Create and update plans with unknown fields, lists, items, and nested values | Seven `unknown-*` scenarios |
| Successful Create and Update replies with null scope, missing ID, or empty ID | `create-null-scope`, `create-missing-id`, `create-empty-id`, `update-null-scope`, `update-missing-id`, `update-empty-id` |
| Successful Read replies with empty/null lists or ID-less items, for both resource and data source | `read-empty-scopes`, `read-null-scopes`, `read-missing-id`, `read-empty-id` |
| Duplicate display names are allowed | `duplicate-display-names` |
| Data-source ID lookup, absent ID, empty ID, missing scope, API failure | `datasource_read-by-id`, `datasource_no-id`, `datasource_empty-id`, `datasource_not-found`, `datasource_api-error` |

### API outcome matrix

| Caller | Normal success | Missing resource | Other API error | No HTTP response | Unusable 200 reply |
| --- | --- | --- | --- | --- | --- |
| Create | `minimal` | No resource ID yet | `create-api-error` | `transport-create` | `create-null-scope`, `create-missing-id`, `create-empty-id` |
| Resource Read | `minimal` | `read-not-found` | `read-api-error`, `route-missing` | `transport-read` | `read-empty-scopes`, `read-null-scopes`, `read-missing-id`, `read-empty-id` |
| Update | `full-lifecycle` | `update-not-found` | `update-api-error` | `transport-update` | `update-null-scope`, `update-missing-id`, `update-empty-id` |
| Delete | `minimal` | `delete-not-found`, `delete-route-missing` | `delete-api-error` | `transport-delete` | The response body is not used |
| Data-source Read | `datasource_read-by-id` | `datasource_not-found` | `datasource_api-error` | `transport-data-source` | All four paired `read-*` response cases |

The injected 500 cases cover the non-not-found API error category for each caller.
Existing `TestScopeReadDoesNotPanicOnAPIFailure` unit tests also cover authorization,
rate-limit, and transport failures for both readers.

The `transport-*` cases return a nil HTTP response and an error before the fake
server receives a request. They record all five callers' diagnostics and state,
then retry or refresh after recovery and reach an empty plan. Failed Create leaves
null state; failed Update, Delete, and resource Read keep prior state. The failed
data-source read keeps only its configured ID in protocol state.

The `unknown-*` cases call the protocol planner before Create and before Update.
Unknown `display_name`, `description`, `default_expression`, the whole `filters`
list, and either filter field remain unknown in the plan without diagnostics.
A wholly unknown filter item currently produces required-child validation errors
for `entity_type` and `expression`; this released limitation is recorded separately.

The required `filters` attribute cannot be omitted or null in Terraform config.
The API fidelity cases separately record empty, null, and omitted filters on
replacement. Both computed fields, `id` and `team_id`, receive non-null service
values in resource and data-source goldens. There is no name lookup, union, or
write-only field in the released schema.

## Fake API evidence and limits

`fidelity/cases.json` and `fidelity/recorded.json` contain anonymized API requests
and replies. The additional expression, replacement, duplicate-name, empty-name,
description-clearing, and false-branch cases were recorded on EU2 on 2026-11-03.
All probe-created scopes were deleted and verified absent.
`TestScopeFakeMatchesRecordedBackend` replays these cases.

- Expression failures model only the recorded inputs: `true`, `false`,
  `prefixfalse`, `falsee`, an empty string, and `<v1>true-whatever`. The fake is
  not an expression compiler. Both alternatives in the released default-expression
  regex have protocol coverage; its permissive validation stays unchanged here.
- The fake rejects `ENTITY_TYPE_UNSPECIFIED`. It does not validate arbitrary
  entity-type strings; Terraform rejects values outside its released enum.
- The fake preserves input filter order. General backend reordering is not modeled.
- Clearing a nonempty description sends `description: ""`. The API returns the
  empty string, but the handwritten resource and data source flatten it to null.
  `clear-description` records the inconsistent update result and the nonempty
  follow-up plan. Omitting the description is a separate case in `full-lifecycle`.
- The 500 responses are injected test faults, not recorded live outages.
- Malformed successful write replies are injected faults, not observed live API
  behavior. The fake saves the write before returning a null scope or an ID-less
  scope. Create records the error and null state. Update records the error and
  unchanged prior state, then refreshes the stored change and replans.
- Successful but unusable GET replies are injected faults. Empty or null lists
  remove resource state with a warning; ID-less items produce an error and keep
  prior resource state. In all four cases, the data source reports an error and
  its protocol state keeps the configured ID with all other attributes null.
  Normal missing lookups still return the recorded 404 response.

## Verification

Run against the handwritten implementation, with live-test credentials unset:

```sh
go test ./internal/provider -run 'TestScope(Baseline|FakeMatchesRecordedBackend)' -count=2
go test ./internal/provider -run 'TestDataSourceBaseline/^(scope)$' -count=2
```

Set `UPDATE_GOLDEN=1` only when intentionally recording the handwritten behavior.
