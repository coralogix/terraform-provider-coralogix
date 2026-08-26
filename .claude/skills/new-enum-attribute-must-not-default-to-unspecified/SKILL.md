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

**Fix:** drop the `Default`. Keep `Optional + Computed`.

```go
Optional: true, Computed: true,
Validators: []validator.String{stringvalidator.OneOf(valid...)},
MarkdownDescription: "... The API chooses a value when this is omitted, so set " +
    "`unspecified` explicitly to go back to that. ...",
```

Terraform then proposes the prior state value when the configuration omits the attribute, so
the API's own choice survives and the plan is empty. The request omits the field instead of
sending `UNSPECIFIED`, which is what lets the API choose. State the escape hatch in the
description: removing an explicitly set value keeps the old one, as for any Optional+Computed
attribute, and setting `unspecified` hands the choice back.

**Why:** a dashboard created by an earlier release has no value for the new field, so the API
returns whatever *it* defaults to — often not `UNSPECIFIED`. A static default claims otherwise,
so the first plan after upgrading wants to change a field the user never wrote. Create-path
tests cannot see this: they send `UNSPECIFIED` themselves and the API echoes it back.

A default is only safe once you have confirmed the API stores `UNSPECIFIED` for an absent field.
The migration jobs are what prove it, so run them before trusting a default:

```bash
CORALOGIX_DASHBOARD_MIGRATION_ACC=1 TF_ACC=1 go test ./internal/provider \
  -run '^TestAccCoralogixResourceDashboardMigration' -v -count=1
```
