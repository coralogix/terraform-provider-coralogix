# Generator migration review rubric

Use this for generator migration PRs only. Review migrated and new generated resources. Follow
other repo rules. Check the newest commit and comments before you decide.

## 1. Keep released behavior

Compare old and new schema, data source, create, read, update, delete, import, state version, and
errors. Check set/change/remove, defaults, null/empty values, IDs, partial failure, retry, and
cleanup. Change behavior only to fix a proven bug; give an example. Old and migration tests can
show old behavior. No old code copy is needed.

## 2. Keep future fields automatic

After the SDK bump, a normal field must flow from protobuf and OpenAPI through `tfgen` into schema,
model, write, and read. Check for manual field lists in overrides, hooks, wrappers, and templates.
Try a test field. Unsupported fields must fail with a clear error.

## 3. Limit generated changes

Run standard `tfgen generate`. Normal cases need no extra operation flags; overrides load on their
own. Check the diff. Other generated resources change only for tested shared bug fixes; name each
fix. Put special rules for one resource in a small hook or validator. Do not add that resource's
name to shared generator code or copy its full field list into handwritten code.

## 4. Run all three test groups

Keep the old resource tests. Run them against the generated resource. Add new tests for the
generated resource. Add migration tests: create with a released provider, switch to the new code,
and check that Terraform plans no change. Then test update and import where they apply. All three
test groups must pass in CI on the newest commit.

## 5. Check service behavior and prior feedback

If tests use a fake API, compare its replies with real API calls or a saved recording. Check 404s.
If the API says the resource is gone, remove it from state. If the API route is missing, return an
error. Read comments on earlier generator migration PRs and apply the ones that fit this PR. Report
each remaining blocker with an example and code location.
