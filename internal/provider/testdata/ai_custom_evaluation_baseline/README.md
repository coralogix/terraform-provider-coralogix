# coralogix_ai_custom_evaluation protocol baseline

These goldens record the `coralogix_ai_custom_evaluation` resource through the plugin protocol, against
the fake in `ai_custom_evaluation_baseline_fake_test.go`. The resource has no data source.

The resource is generated now (`internal/provider/generated/aicustomevaluation`, with the overlay in
`internal/provider/ai`). The goldens show the generated resource. The tables below describe the
handwritten behavior that the goldens recorded first, and the section "What the migration changed"
says what differs. The fixes are in `CHANGELOG.md`.

```sh
UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestAICustomEvaluationBaseline'
```

## What the migration changed

Of the 96 scenarios of the handwritten resource, two were replaced (the failure of the list route and the failure of the call that cleared the examples, which no longer exist), and 81 of the other 94 are the same in every plan, state, diagnostic, and second plan. They differ only in the requests: a read is `GET /ai/custom-evaluations/v3/{id}` and no
longer a list, and an update is one `PATCH` with a mask. The other differences are these:

| Difference | Scenarios |
|---|---|
| The wording of an API error or of a missing resource: `Unable to create ai_custom_evaluation`, `Resource disappeared during read`, `Resource no longer exists`. | `create-api-error`, `name-duplicate`, `rename-to-taken-name`, `update-api-error`, `update-not-found`, `delete-api-error`, `read-api-error`, `read-not-found`, `import-not-found`, `import-api-error` |
| A 404 of the route is an error and keeps the state, on a read and on a destroy. Before, a 404 of the list route removed the resource, and a destroy treated every 404 as done. | `read-route-not-found` (was `read-list-not-found`), `delete-route-not-found` (new) |
| An update sends one `PATCH` whose mask names the changed fields. The examples are cleared in the same call, so the second call and its failure case are gone. A change of only `applications` sends no `PATCH`. | `criteria-examples-cleared-in-one-call` (was `criteria-examples-clear-fails`), `minimal-lifecycle`, `applications-added-on-update` |
| An id that is no UUID fails to import with an error. Before, the list had no such evaluation and the import warned. | `import-not-a-uuid` |
| The order of the elements of a set in the golden (`applications`, `application_ids`). The sets are equal. | `applications-multiple`, `import-with-applications` |

The fake answers the new read, which the recording does not have (the API of the recording does not
have the call yet): it follows the contract of the call. The update mask takes the names of the JSON
body in lower camel case (`policyType`), and the gateway refuses a proto name (`policy_type`); the
recording has cases for both.

## The fake follows the real API

`fidelity/cases.json` lists real API requests, and `fidelity/recorded.json` holds the answers of
EU2. `TestAICustomEvaluationFakeMatchesRecordedBackend` sends the same requests to the fake and
compares the answers. To record again, with a key that can manage AI custom evaluations and read AI
applications, on a team that has at least two AI applications:

```sh
RECORD_AI_CUSTOM_EVALUATION_FIDELITY=1 CORALOGIX_API_KEY=... CORALOGIX_ENV=EU2 \
  go test ./internal/provider -run TestRecordAICustomEvaluationFidelity
```

Each case has a run name of its own, and every evaluation that it creates is named after it. The
recorder deletes the evaluations that its requests created, and every evaluation whose name starts
with a run name. It never touches another evaluation of the team. The recording masks the run name,
the ids that the cases capture, the company id, the times, and the names of the applications, which
belong to the tenant. A list keeps only the evaluations of the run. `TestAICustomEvaluationRecorderDryRun`
runs the recorder against the fake, so the cleanup keeps working between recordings.

The real list of applications has no stable order: two calls with the page size 1 and different
page offsets can answer with the same application. The recording compares those two answers by
shape only. The fake pages in a fixed order, so the paging scenarios do not show what a real tenant
with more than 200 applications does.

## What the real API does

| Fact | Golden |
|---|---|
| Create answers `{"item": evaluation}` with every config field set (`""`, `[]`, or `false` when the request left it out). Update answers the same shape with `applicationIds: []`, whatever the links are. The API of the recording has no read of one evaluation (a new call was added after it): the handwritten resource lists them all and filters by id, and the generated one reads by id. | `scenario_full-lifecycle.json` |
| Update is a PATCH. The update mask is a property of the body (`"updateMask": "examples"`), not a query parameter. A mask that names a field clears it when the body has none. Without a mask, an empty `examples` list is ignored, so the resource clears the examples with a second PATCH. | `scenario_criteria-removed-on-update.json` |
| A mask that names `name`, or a path that the service does not know, answers 400. An id that is no UUID answers 400 on Update and 404 on the other routes. | fidelity `update`, `delete`, `links` |
| Applications are linked with `POST` and unlinked with `DELETE` on `/custom-evaluations/v3/{id}/applications/{application_id}`. Linking twice is 409, and unlinking a link that does not exist is 404. Create takes the application ids in its body. | `scenario_applications-added-on-update.json`, `scenario_update-link-already-exists.json` |
| Names are unique in the team, also on a rename (409). The API accepts an empty name on Update, any policy type, and any example score. The resource refuses them with validators or when it reads. | `scenario_name-duplicate.json`, `scenario_rename-to-taken-name.json`, `scenario_criteria-score-unexpected.json` |

## Released behavior that a migration must keep or fix on purpose

| Behavior | Golden |
|---|---|
| `criteria` is not an API object. The resource sends `acceptable.flags` as `safe`, `prohibited.flags` as `violates`, and the examples of both lists as one list with the score `"1"` for acceptable and `"0"` for prohibited. It reads them back in the same way, and an example with another score fails the read. At most 100 examples across both lists, checked when the apply starts, not at validation. | `scenario_criteria-*.json` |
| Every Update with no examples in the plan sends a second PATCH with the mask `examples`, also for an evaluation that never had examples. If the second call fails, the first change is already made and the state keeps the old values until the next refresh. | `scenario_criteria-examples-cleared-in-one-call.json`, `scenario_applications-added-on-update.json` |
| `applications` names applications by `application` and `subsystem`. The resource lists the applications of the team in pages of 200 (`page_offset` counts pages) to find their ids, only when it needs them: Create with applications, Update when the selectors changed, and a refresh when the linked ids changed outside Terraform. A refresh keeps the configured selectors when the ids are the same. | `scenario_applications-omitted.json`, `scenario_applications-second-page.json`, `scenario_applications-linked-outside.json` |
| Update changes the fields first and then links the new applications, then unlinks the removed ones, each sorted by id. A failed link or unlink fails the apply after the fields changed; the state keeps the old values until the next refresh. | `scenario_update-link-fails.json`, `scenario_update-unlink-fails.json` |
| An application that matches no application, or more than one, fails the apply before any call that writes. | `scenario_applications-not-found.json`, `scenario_applications-ambiguous.json` |
| Read treats a 404 of the list route as a resource that is gone and removes it from state with a warning. A missing route and a missing evaluation look the same. | `scenario_read-route-not-found.json`, `scenario_read-not-found.json` |
| Import takes the id as it is. An id that no evaluation has, or that is no UUID, imports to a warning and no resource. | `scenario_import-not-found.json`, `scenario_import-not-a-uuid.json` |
| Delete treats 404 as success. | `scenario_delete-not-found.json` |
| `application_ids` is computed and has no plan modifier, so it is `(known after apply)` in every plan that changes the resource. | `scenario_description-changed-outside.json` |
| An applications set with the same selector twice is refused at validation. | `scenario_applications-duplicate-selector.json` |

## Not covered

- The key needs the permissions of the released resource: manage AI evaluations, and read AI
  applications. A key without the second one cannot use `applications`. The generated resource reads
  one custom evaluation with its own call, the same service as the list that it replaced. No key that
  can list but cannot read one custom evaluation has been tried.
- The data source does not exist, so there is no data source baseline.
- Only `policy_type` values `quality` and `security` are valid in Terraform. The API accepts others.
