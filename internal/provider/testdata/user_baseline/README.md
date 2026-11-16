# coralogix_user protocol baseline

These goldens record the handwritten `coralogix_user` resource and its data source through the
plugin protocol, against the fake in `user_baseline_fake_test.go`. When the resource switches to
generated code, the golden diff must show only message wording or proven bug fixes.

```sh
UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestUserBaseline|TestDataSourceBaseline(Schema|Scenarios)/user'
```

## The fake follows the real API

`fidelity/cases.json` lists real API requests, and `fidelity/recorded.json` holds the answers of
EU2. `TestUserFakeMatchesRecordedBackend` sends the same requests to the fake and compares the
answers; it skips while `recorded.json` does not exist. To record again, with the key of a team that
has team-members permissions:

```sh
RECORD_USER_FIDELITY=1 USER_FIDELITY_ALLOW_MUTATION=I_UNDERSTAND_THIS_LEAVES_INACTIVE_USERS \
  USER_FIDELITY_MEMBER_CAP=<cap> CORALOGIX_API_KEY=... CORALOGIX_ENV=EU2 \
  go test ./internal/provider -run TestRecordUserFidelity -count=1 -v
```

Run it by hand, one run at a time. The recorder searches, creates, and updates only users whose
username holds its random run id, creates every user with `ONBOARDING_MODE_NO_INVITE`, and sends no
delete. It redacts every answer before it writes it, and refuses to write a recording that still holds
member data. `TestUserRecorderDryRun` runs the whole recorder against the fake, so the recorder and its
cleanup keep working without a key.

A capture in `cases.json` whose name ends in `_TOKEN` takes the `nextPageToken` of the answer and sends
it back as it came. The recording keeps a page token that is a small offset, and writes any other text
token as `<page-token>`, because an opaque token can encode ids of the team.

## Limits of the recording

* The Users API has no delete. Each run creates 6 users named
  `tf-acc-user-<run>-<n>-<letter>@coralogix.com` and leaves them inactive. They stay on the team.
* The team has a member cap. Before its first write, the recorder counts the members of the team and
  the users that earlier runs left, and refuses to run unless its users and 2 seats of headroom fit
  under `USER_FIDELITY_MEMBER_CAP`. It reads only `totalCount` of those searches and keeps nothing
  else. Whether an inactive user counts against the cap is not confirmed yet; until it is, assume
  that it does.
* Every create uses `ONBOARDING_MODE_NO_INVITE`, so invited and already-invited results were not
  recorded. The fake models them from the contract.
* The recording key belongs to a team, so organization-level reads and other teams' members were not
  recorded. No search without a run-scoped `username` is recorded.
* Group membership (`groupIds`) and a domain outside the allowlist were not recorded.
  `scenario_groups-non-empty.json`, `datasource_read-with-groups.json` and
  `scenario_create-domain-not-allowed.json` record the code, not the API.
* Failed, unknown, and unspecified create statuses, 500s, and a page token that never advances are
  injected by the fake.

## Bugs that the baseline records

These are the behaviors the migration may fix. Each fix is listed in the changelog of the
migration, and the golden diff of the switch shows it.

* `scenario_update-user-gone.json`: Update removes the resource on a 404 and the framework fails with
  "Missing Resource State After Update".
* `scenario_*-route-not-found.json`: the 404 of a missing route is treated like a missing user.
