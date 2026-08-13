---
name: rest-client-not-found-detection
description: "Use when a REST-backed resource (SCIM user, Grafana) fails to soft-delete on 404, or when cxsdk.Code returns Unknown for a status.Error from rest/client.go."
---

# REST clients return bare status errors — use status.Code

**Trigger:** A read/delete path for SCIM users or Grafana uses `cxsdk.Code(err) == codes.NotFound`, but the branch never runs for a real 404.

**Fix:** Keep `rest/client.go` returning a bare status error, and check with `status.Code`:

```go
// internal/clientset/rest/client.go
return "", status.Error(codes.NotFound, "Not found")

// call site
if status.Code(err) == codes.NotFound { ... RemoveResource }
```

**Why:** `cxsdk.Code` only unwraps `*cxsdk.SdkAPIError` (SDK gRPC clients). The hand-rolled REST client returns a plain `status.Error`. Matching pair is `status.Code`. Wrapping REST errors in `SdkAPIError` only to keep `cxsdk.Code` is unnecessary.

**Trap:** Do not mix them. After a wrap, `status.Code` returns `Unknown`. After no wrap, `cxsdk.Code` returns `Unknown`. OpenAPI facade clients are separate — use `cxsdkOpenapi.IsNotFound`.
