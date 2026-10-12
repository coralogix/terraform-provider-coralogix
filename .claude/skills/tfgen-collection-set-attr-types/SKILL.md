---
name: tfgen-collection-set-attr-types
description: "Use when tfgen collection:set or x-coralogix-collection: set makes schema SetNested but apply fails Value Conversion Error List vs Set. Keep convert.go attr type maps on the same collection kind."
---

# Collection set must update attr.Type maps

**Trigger:** Generated schema is `SetNestedAttribute`, flatten uses `flattenSet`, but apply fails with `Value Conversion Error` listing `types.SetType[...]` expected vs `types.ListType[...]` received (often on a nested `fields` path).

**Fix:** `attrType` for `convObjects` must use `f.Collection`, not a hard-coded `ListType`:

```go
expr = "types." + f.Collection + "Type{ElemType: " + expr + "}"
```

Regenerate the resource after the generator change. Do not patch only `convert.go` of one resource.

**Why:** Schema, model (`types.Set`), flatten helper, and the hand-built `*AttrTypes()` map are four descriptions of the same object. The framework compares the map to the schema on `State.Set`.
