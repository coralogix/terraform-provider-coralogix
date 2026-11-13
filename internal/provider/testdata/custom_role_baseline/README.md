# coralogix_custom_role protocol baseline

These goldens record the handwritten `coralogix_custom_role` resource and its data source through
the plugin protocol, against the fake in `custom_role_baseline_fake_test.go`. When the resource
switches to generated code, the golden diff must show only message wording or proven bug fixes.

The resource is generated now. The tables below describe the handwritten behavior that these goldens
recorded first. The current goldens show the generated resource, and the fixes are in `CHANGELOG.md`.

```sh
UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestCustomRoleBaseline|TestDataSourceBaseline.*/custom_role'
```

## The fake follows the real API

`fidelity/cases.json` lists real API requests, and `fidelity/recorded.json` holds the answers of
EU2. `TestCustomRoleFakeMatchesRecordedBackend` sends the same requests to the fake and compares
the answers. To record again, with a key that can manage custom roles:

```sh
RECORD_CUSTOM_ROLE_FIDELITY=1 CORALOGIX_API_KEY=... CORALOGIX_ENV=EU2 \
  go test ./internal/provider -run TestRecordCustomRoleFidelity
```

Each case has a run name of its own, and every role that it creates is named after it. The
recorder deletes the roles that its requests created, and every role whose name starts with a run
name. It never touches another role of the team. The recording masks the role ids, the team id,
and the run name. Permission lists are sorted: Create and Update return them in another order, and
the resource shows a set. The list answer keeps only the roles of the case, and replaces the
permissions of a role with a marker (see below).

The recording covers the resource: the data source reads the same Get and list calls, which the
cases contain. The data source scenarios run against the fake only.

## What the real API does

| Fact | Golden |
|---|---|
| Create is a `PUT` of the collection and answers only `{"id":N}`. Update is a `POST` of the role and answers `{}`. Get wraps the role in `{"role":{...}}`. Delete answers `{}`. | `scenario_full-lifecycle.json` |
| The parent role is required, by name (case-sensitive) or by id, and not both. Update cannot change it: the resource sends no parent on Update, so a changed `parent_role` fails with an inconsistent result. | `scenario_parent-role-*.json` |
| Every permission must be one that the API knows, in any case (400 `Expression is not valid: …`). The API keeps the case that it was given, so the resource restores the case of the configuration on read. | `scenario_permissions-upper-case.json`, `scenario_permissions-case-*.json`, `scenario_permission-unknown*.json` |
| Permissions that differ only in case are one permission: the API keeps the first in ASCII order (`SPANS…` over `spans…`). A set with both has one element after the apply. | `scenario_permissions-differ-only-in-case*.json` (inconsistent result) |
| The API accepts a role without permissions, and the resource refuses it (`SizeAtLeast(1)`). An update to an empty `newPermissions` is accepted too. | `scenario_permissions-empty.json` |
| Names are unique in a team (409 on Create and on Update). The empty name is a name: Create and Update accept it, and a request without a name has it. A rename to the name that the role has is no conflict. | `scenario_name-*.json`, `scenario_rename-*.json` |
| An omitted description is the empty string. | `scenario_description-empty.json` |
| Get returns the permissions of the role. The list returns them together with the permissions of the parent role, so the data source shows other permissions for a lookup by name than for a lookup by id. | `datasource_read-by-name.json`, `datasource_read-without-permissions.json` |
| An id that no role has is 404 on Get, Update, and Delete. A malformed id is 400. | `scenario_*-not-found.json`, `fidelity` case `bad-ids` |

## Behavior that the migration may fix

The baseline records these as they are:

- Update removes the resource from the state on a 404 and then fails with `Missing Resource State After Update`.
- A failed Get after Create leaves no state, so the next apply creates a second role and fails with 409. A failed Get after Update keeps the prior state.
- An import of an id that is no number succeeds with an empty state (Read returns without a diagnostic).
- A changed `parent_role` is accepted by the plan and rejected after the apply.
- A permission in two cases, or an unknown permission, is a 400 or an inconsistent result at apply, not a plan error.
- The data source shows the permissions of the parent role for a lookup by name.

The fake has a role with the same name twice only for the data source scenario `duplicate-name`.
The real API refuses it.
