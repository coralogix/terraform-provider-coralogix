# coralogix_api_key protocol baseline

These goldens record the handwritten `coralogix_api_key` resource and its data source through the
plugin protocol, against the fake in `api_key_baseline_fake_test.go`. When the resource switches to
generated code, the golden diff must show only message wording or proven bug fixes.

```sh
UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestAPIKeyBaseline|TestDataSourceBaseline/api_key|TestDataSourceBaselineSchema/api_key'
```

## The fake follows the real API

`fidelity/cases.json` lists real API requests, and `fidelity/recorded.json` holds the answers of
EU2. `TestAPIKeyFakeMatchesRecordedBackend` sends the same requests to the fake and compares the
answers. To record again, with the key of a team:

```sh
RECORD_API_KEY_FIDELITY=1 CORALOGIX_API_KEY=... CORALOGIX_ENV=EU2 \
  go test ./internal/provider -run TestRecordAPIKeyFidelity
```

The recorder names every key it creates after its run and deletes the keys it created. The
recording holds no key id, team id, or secret: the recorder replaces them. It shows a policy as the
JSON it holds and sorts the permissions of a key, because the API returns both in no stable order.

## What the real API does

| Fact | Golden |
|---|---|
| Create returns `{keyId, name, value}`. Get wraps the key in `keyInfo` and shows the value of every key that is not hashed, so an import reads the value. Update and Delete return `{}`. | `scenario_full-lifecycle.json`, `scenario_import.json` |
| Names are not unique and have no length limit. An empty name is 400 on Create and Update. | `scenario_name-*.json`, `scenario_rename-to-empty-name.json` |
| A key needs a preset or a permission (400 for both empty). An Update that sends a list replaces it, and an empty list clears it, also both. A repeated preset or permission is 500. | `scenario_presets-and-permissions-empty.json`, `scenario_update-*.json` |
| A preset is matched in any case and returned by its name, so `apm` in the config reads back as `APM` (inconsistent result). A permission of a preset can also be a direct permission. | `scenario_preset-in-lower-case.json`, `scenario_permission-of-a-preset.json` |
| Create sends no `isActive`, so a key is active, and `active = false` in the config reads back as `true` (inconsistent result). | `scenario_create-inactive.json` |
| A team key can only be owned by the team of the caller: another team and a user that is no member are 403, an organisation is 400. | `scenario_owner-*.json` |
| A policy of a team key may hold only `team-custom-api-keys:*` permissions (400 for the policy of the docs example). The API stores compact JSON in no stable key order and adds the team permissions that the policy leaves out, as `deny`. A complete policy reads back equal, an incomplete one does not (inconsistent result). Update with `""` clears the policy. | `scenario_policy-*.json` |
| A key that never existed and a deleted key are both 404 on Get, Update, and Delete. An id that is no uuid is 400. | `scenario_*-not-found.json`, `scenario_import-invalid-id.json` |

## Limits of the recording

* The recording key belongs to a team, so a key for a user or an organisation was not created. The
  fake answers 403 and 400 as the API did to the recording key, and no scenario shows the read of
  such a key. The handwritten flatten reads the owner of a Get answer by the field that it has.
* A policy of a key that another kind of owner has, such as the `data-ingest-api-keys:*` policy
  of the docs example, could not be recorded.
* The fake sorts nothing: it returns the lists in the order of the request, and the replay compares
  permissions as sets.

## Bugs that the baseline records

These are the behaviors the migration may fix. Each fix is listed in the changelog of the
migration, and the golden diff of the switch shows it.

* `scenario_update-not-found.json`: Update answers 404, removes the resource, and the framework
  fails with "Missing Resource State After Update".
* `scenario_update-read-fails.json`: a failed Get after Update returns no error and no state.
* `scenario_create-read-fails.json`: a failed Get after Create keeps no state, so the key and its
  secret are lost.
* `scenario_create-inactive.json`, `scenario_preset-in-lower-case.json`,
  `scenario_policy-incomplete.json`: inconsistent result after apply.
