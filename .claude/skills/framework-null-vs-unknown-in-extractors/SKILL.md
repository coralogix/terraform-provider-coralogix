---
name: framework-null-vs-unknown-in-extractors
description: In `extract*` helpers under `internal/provider/**/resource_*.go` that feed `Replace*`/`Update*` SDK calls, never collapse `IsNull()` and `IsUnknown()` into one branch. Null = clear; unknown = omit.
---

# Null vs Unknown in extractor helpers

**Trigger:** an extractor turns a `types.Set/List/Map/Object` into an SDK request body, and you see `if x.IsNull() || x.IsUnknown()` returning the same value.

**Fix:**

```go
if x.IsUnknown() {
    return nil, nil               // omit key → server keeps existing value
}
if x.IsNull() {
    return []sdk.Foo{}, nil       // explicit empty → server clears on Replace
}
```

**Why:** the SDK's `ToMap()` uses `IsNil()` to decide whether to emit a JSON key. Collapsing both states either silently preserves removed values (both → nil) or silently destroys values still being resolved (both → empty slice).
