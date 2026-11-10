# coralogix_team protocol baseline

These goldens record the handwritten `coralogix_team` resource through the plugin protocol, against
the fake in `team_baseline_fake_test.go`. When the resource switches to generated code, the golden
diff must show only message wording or proven bug fixes.

```sh
UPDATE_GOLDEN=1 go test ./internal/provider -run TestTeamBaseline
```

The released provider registered no `coralogix_team` data source. The generated one is new, and its
`datasource_*.json` goldens record it.

## The fake follows the real API

`fidelity/cases.json` lists real API requests, and `fidelity/recorded.json` holds the answers of
EU2. `TestTeamFakeMatchesRecordedBackend` sends the same requests to the fake and compares the
answers. To record again, with a key whose owner is an organization (a team key gets 403):

```sh
RECORD_TEAM_FIDELITY=1 CORALOGIX_API_KEY=... CORALOGIX_ENV=EU2 \
  go test ./internal/provider -run TestRecordTeamFidelity
```

The recorder names every team it creates after its run, and deletes only those. Each team moves
quota from the default team of the organization and gives it back on delete.

## What the real API does

| Fact | Golden |
|---|---|
| Create returns only `{"teamId":{"id":N}}`; the resource reads the team with Get. Update and Delete return `{}`. | `scenario_full-lifecycle.json` |
| The quota is stored as a 32-bit float: 0.3 reads back as `0.30000001192092896`. The handwritten flatten rounds to three decimals. | `scenario_quota-more-than-three-decimals*.json` (inconsistent result) |
| Create ignores an omitted quota, and a quota of 0.01 or less, and stores the minimum quota, which reads back as float32(0.02). | `scenario_quota-omitted.json`, `scenario_quota-at-most-minimum.json` (inconsistent result) |
| An increase of the quota reaches the target. A decrease leaves the target plus 0.01 (0.5 to 0.4 reads back as 0.41), and a decrease of 0.01 or less is rejected (400: 0.5 to 0.495). An Update that sends the stored quota again, as the resource does on every rename, answers 200 and changes nothing: the API compares float32 values, so 0.6, stored as `0.6000000238418579`, is no change. | `scenario_quota-increase.json`, `scenario_full-lifecycle.json` (0.25 reads back as 0.26: inconsistent result), fidelity case `quota-unchanged-and-small-decrease` |
| Delete first lowers the quota to 0.01, so a team with the minimum quota cannot be deleted (400). | `scenario_quota-omitted.json` |
| Update rejects a quota of 0 or less (400). | `scenario_quota-zero-on-update.json` |
| A name is unique: Create answers 400 for a taken name, and Update answers 500. An empty name is 400 on both. | `scenario_name-*.json`, `scenario_rename-*.json` |
| An id that never existed is 404 on Get, Update, and Delete. A deleted team is not: Get answers 400, Update 412, Delete 400. | `scenario_*-not-found.json`, `scenario_*-after-delete-outside.json` |

Quotas between 0.01 and 0.02 read back inconsistently on Create (0.015 reads back as 0.01), so no
case and no scenario uses one.
