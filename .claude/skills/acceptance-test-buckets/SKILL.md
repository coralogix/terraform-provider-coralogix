---
name: acceptance-test-buckets
description: "Use when a dashboard test starts to use a helper from another file, when a new domain directory is added under internal/provider/, or when a pull request skips acceptance jobs it should run. Keeps .github/scripts/detect-test-scope.sh correct."
---

# Keep the acceptance test buckets correct

**Trigger:** You edit `.github/scripts/detect-test-scope.sh`, add a domain directory, or a dashboard test begins to use code of a non-dashboard file.

**Fix:** The script splits files into two buckets. Check each rule:

| Change | Edit in the script |
|---|---|
| New domain directory under `internal/provider/` | Add its name to `domains`. Until then, a change in it runs every job. That is safe. |
| A dashboard test uses a helper from a non-dashboard file | Add that file to `shared`. A change to it then runs both buckets. |
| A non-dashboard area starts to use dashboard code (as `alerts` does with `dashboard_schema`) | Add its jobs to the `deps` gates in `acceptance-tests.yml`. |

```bash
printf '%s\n' internal/provider/webhook/x.go | bash .github/scripts/detect-test-scope.sh
# code=true dashboards=false rest=true deps=false
```

**Why:** A pull request that changes only one bucket skips the jobs of the other bucket. A use of code across buckets that the script does not know about would skip a job that should run. The nightly run and every push to master run both buckets, so a miss shows up there.
