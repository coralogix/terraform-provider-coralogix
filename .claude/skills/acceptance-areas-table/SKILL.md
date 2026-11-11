---
name: acceptance-areas-table
description: "Use when `acceptance-areas lint` fails, a new resource, test file, or example is added, or a test helper starts to be used by another resource. Keeps .github/acceptance-areas.yaml correct. Do NOT use for shard patterns."
---

# Keep the acceptance area table correct

**Trigger:** The `areas-lint` job fails, or you add a file under `internal/provider/` or `examples/`.

**Fix:** Read the error and pick one of these:

| Error | Action |
|---|---|
| `files are in no area and not in the shared list` | Add the resource to an area under `resources:` (or `generated:`), or add the file to `shared`. |
| `area A uses code of area B` | Add `depends_on: [B]` to area A. Or move the helper to a shared file and remove the use. |
| `... matches no file` | Remove the stale entry. |
| `tests are started by no job` | A test name matches no shard pattern in `acceptance-tests.yml`. Fix the pattern. |

```yaml
areas:
  webhook:
    resources: [webhook]      # test files, test data, and examples of coralogix_webhook
    generated: [webhook]      # internal/provider/generated/webhook/
    depends_on: [notifications]
```

Run `go run ./tools/acceptance-areas uses` to see which files many areas use. A file that two or more other areas use belongs under `shared`.

**Why:** A pull request that changes only one area skips the jobs of other areas. A use of code across areas that the table does not know about would skip a job that should run. Everything outside an area runs every job, so a missing entry costs time and never hides a test.
