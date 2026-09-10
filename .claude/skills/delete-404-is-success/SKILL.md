---
name: delete-404-is-success
description: "Use when adding or reviewing a resource Delete, terraform destroy fails after an out-of-band delete, or a Delete error branch has no 404 case. Treat HTTP 404 as success. Do NOT use for Read/Update."
---

# Delete 404 is success

**Trigger:** `Delete` error path has no `StatusNotFound` / `codes.NotFound` branch.

**Fix:** On 404, return with no diagnostic. Nil-guard OpenAPI `httpResponse`. REST/SCIM: `status.Code(err) == codes.NotFound`.

```go
if err != nil {
    if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
        return
    }
    resp.Diagnostics.AddError(...)
    return
}
```

**Why:** Terraform's contract is that deleting an already-absent resource succeeds. A blocking diagnostic leaves the ID in state.
