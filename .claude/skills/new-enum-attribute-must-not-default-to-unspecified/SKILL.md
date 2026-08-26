---
name: new-enum-attribute-must-not-default-to-unspecified
description: "Use when adding an Optional+Computed enum to a released resource, or a Provider migration job fails with 'expected NoOp, got action(s): [update]'. A static unspecified default breaks upgrades."
---

# A new enum attribute must not default to `unspecified`

**Trigger:** You are adding an enum attribute to a resource that already has released
versions, and reaching for the house pattern:

```go
Optional: true, Computed: true,
Default:  stringdefault.StaticString(utils.UNSPECIFIED),   // asserts what the API stores
```

Or a `Provider migration (...)` job fails with `expected NoOp, got action(s): [update]` while
every create-and-apply acceptance test passes.

**Fix:** three parts, and all three are needed.

```go
Optional: true, Computed: true,                       // 1. no Default
PlanModifiers: []planmodifier.String{
    stringplanmodifier.UseNonNullStateForUnknown(),   // 2. keep what the API chose
},
Validators: []validator.String{stringvalidator.OneOf(valid...)},
```

```go
// 3. an absent field arrives as the enum's zero value, "", which is in no map
func FlattenEnum[T ~string](value T, mapping map[T]string) types.String {
    if name, ok := mapping[value]; ok {
        return types.StringValue(name)
    }
    return types.StringValue(utils.UNSPECIFIED)
}
```

Miss part 3 and flatten writes `""`, a value the schema does not allow. Miss part 2 and the
attribute plans unknown on every update, because the framework plans an omitted nested
Optional+Computed attribute as unknown whatever the prior state holds — `UseNonNullStateForUnknown`
rather than `UseStateForUnknown`, so an element added to a list on update keeps its unknown plan
instead of copying null. Say in the description that setting `unspecified` explicitly hands the
choice back, because removing the attribute now keeps the previous value.

**Why:** a dashboard created by an earlier release has no value for the new field, so the API
returns whatever *it* defaults to — often not `UNSPECIFIED`. A static default claims otherwise,
so the first plan after upgrading wants to change a field the user never wrote. Create-path
tests cannot see this: they send `UNSPECIFIED` themselves and the API echoes it back.

A default is only safe once you have confirmed the API stores `UNSPECIFIED` for an absent field.
Sending a value hides parts 2 and 3 completely: the API echoes what you sent, so the map never
misses and the plan value is never unknown. Only omitting the field exposes them, and only the
migration and `PlanOnly` acceptance steps run that path:

```bash
CORALOGIX_DASHBOARD_MIGRATION_ACC=1 TF_ACC=1 go test ./internal/provider \
  -run '^TestAccCoralogixResourceDashboardMigration' -v -count=1
```
