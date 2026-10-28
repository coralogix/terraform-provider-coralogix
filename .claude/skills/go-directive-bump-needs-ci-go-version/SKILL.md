---
name: go-directive-bump-needs-ci-go-version
description: "Use when raising the go directive in go.mod, or CI fails with 'go.mod requires go >= X (running go Y; GOTOOLCHAIN=local)'. Make CI install the go.mod version."
---

# Go directive bump needs CI to install that Go

**Trigger:** you raise `go` in `go.mod`, or CI fails with `go.mod requires go >= 1.26.10 (running go 1.26.9; GOTOOLCHAIN=local)`.

**Fix:** let the workflow read the version from `go.mod`:

```yaml
- uses: actions/setup-go@v6
  with:
    go-version-file: go.mod
```

**Why:** `actions/setup-go@v6` sets `GOTOOLCHAIN=local`, so Go cannot download the newer toolchain itself. A range such as `go-version: 1.26.x` without `check-latest` resolves to the runner's cached patch, which can be older than `go.mod`. Older `setup-go` versions keep `GOTOOLCHAIN=auto`, so the same bump passes in one workflow and fails in another.
