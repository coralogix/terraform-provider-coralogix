---
name: new-enum-attribute-must-not-default-to-unspecified
description: "Use when adding an Optional+Computed enum to a released resource, or a Provider migration job fails with 'expected NoOp, got action(s): [update]'. A static unspecified default breaks upgrades."
---

# A new enum attribute must not default to `unspecified`

**Trigger:** Adding an enum to a resource that already has released versions, or a
`Provider migration (...)` job failing `expected NoOp, got action(s): [update]` while every
create-and-apply test passes.

**Fix:** all three parts, or the attribute drifts.

```go
Optional: true, Computed: true,                                             // 1. no Default
PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()}, // 2.
```

```go
// 3. an absent field arrives as the enum's zero value "", which is in no map
func FlattenEnum[T ~string](v T, m map[T]string) types.String {
    if name, ok := m[v]; ok { return types.StringValue(name) }
    return types.StringValue(utils.UNSPECIFIED)
}
```

A default claims what the API stores for an omitted field, and it usually stores something else,
so the first plan after upgrading shows a change. Part 3 stops flatten writing `""`, which the
schema disallows. Part 2 stops the framework planning an omitted nested Optional+Computed
attribute as unknown; use the non-null variant so a list element added on update keeps its
unknown plan. Removing the attribute now keeps the previous value — say so in the description,
and point at `unspecified` as the reset.

**Why it hides:** sending a value means the API echoes it, so the map never misses and the plan
value is never unknown. Only migration and `PlanOnly` steps omit the field.
