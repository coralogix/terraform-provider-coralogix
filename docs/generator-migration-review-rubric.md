# Generator migration review rubric

Use this for generator migration PRs only. Review migrated and new generated resources. Follow
other repo rules. Check the newest commit and comments before you decide.

## 1. Keep released behavior

Compare old and new schema, data source, create, read, update, delete, import, state version, and
errors. Check set/change/remove, defaults, null/empty values, IDs, partial failure, retry, and
cleanup. Change behavior only to fix a proven bug; give an example. Old and migration tests can
show old behavior. No old code copy is needed. List each fix in `CHANGELOG.md` as a `FIX:` line.

## 2. Keep future fields automatic

After the SDK bump, a normal field must flow from protobuf and OpenAPI through `tfgen` into schema,
model, write, and read. Check for manual field lists in overrides, hooks, wrappers, and templates.
Try a test field. Unsupported fields must fail with a clear error.

## 3. Limit generated changes

Run standard `tfgen generate`. Normal cases need no extra operation flags; overrides load on their
own. Check the diff. Other generated resources change only for tested shared bug fixes; name each
fix. Put special rules for one resource in a small hook or validator. Do not add that resource's
name to shared generator code or copy its full field list into handwritten code.

Explain each shared generator change in 2–3 simple sentences with an example: the input, the old
output, and the new output. For each new hook, check whether a `behavior-overrides.yaml` key can
express the same rule. If one can, use the key. For each new key, check whether an existing key can
be extended instead. Each new key or generator branch needs a golden fixture. An overrides line must
not keep a confirmed bug. Delete a `PINNED-SDK` line that matches nothing after the SDK bump.

## 4. Run all three test groups

Keep the old resource tests. Run them against the generated resource. Add new tests for the
generated resource. Add migration tests: create with a released provider, switch to the new code,
and check that Terraform plans no change. Then test update and import where they apply. All three
test groups must pass in CI on the newest commit. Record the protocol baseline goldens on master
with the handwritten resource first. Then the switch commit's golden diff shows only real behavior
changes.

## 5. Check service behavior and prior feedback

If tests use a fake API, compare its replies with real API calls or a saved recording. Check 404s.
If the API says the resource is gone, remove it from state. If the API route is missing, return an
error. If create, update, or read now calls another service or RPC, list each call and the API-key
permission it needs. A least-privilege key that could do the old write must still work; test with
one. Read comments on earlier generator migration PRs and apply the ones that fit this PR. Report
each remaining blocker with an example and code location.
