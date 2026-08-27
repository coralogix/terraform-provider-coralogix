---
name: new-enum-attribute-must-not-default-to-unspecified
description: "Use when adding an Optional+Computed enum to a released resource, or a Provider migration job fails with 'expected NoOp, got action(s): [update]'. A static unspecified default breaks upgrades."
---

# A new enum attribute must not default to `unspecified`

**Trigger:** adding an enum to a resource that already has released versions, or a
`Provider migration (...)` job failing `expected NoOp, got action(s): [update]` while every
create-and-apply test passes.

**Fix:** three parts, all needed.

```go
Optional: true, Computed: true,                                                        // 1. no Default
PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},  // 2.
// 3. flatten an unmapped value (the enum's zero value "") to utils.UNSPECIFIED, not ""
```

A default claims what the API stores for an omitted field, and it usually stores something else.
Part 3 stops flatten writing `""`, which the schema disallows. Part 2 stops the framework planning
an omitted nested Optional+Computed attribute as unknown; the non-null variant keeps a list element
added on update unknown. Removing the attribute now keeps the previous value, so name `unspecified`
as the reset in the description.

**Why it hides:** sending a value means the API echoes it, so the map never misses and the plan is
never unknown. Only migration and `PlanOnly` steps omit the field.
