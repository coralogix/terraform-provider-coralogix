---
name: offline-plan-resource-change-harness
description: "Use when diagnosing why a Terraform plan is not empty, an attribute is unknown, or a plan modifier misbehaves, without Coralogix credentials. Drives PlanResourceChange directly in a unit test."
---

# Offline PlanResourceChange Harness

**Trigger:** You need to know what the provider plans for some attribute, and an acceptance test would need `CORALOGIX_API_KEY` and minutes per run. Plan-time behavior (plan modifiers, defaults, unknown marking) never calls the API, so it can be measured offline in milliseconds.

**Recipe:** in package `provider`, a `_test.go` file:

```go
server, _ := testAccProtoV6ProviderFactories["coralogix"]()
schemaResp, _ := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
object := schemaResp.ResourceSchemas["coralogix_<name>"].ValueType().(tftypes.Object)
// build prior/config/proposed with tftypes.NewValue; null for every attribute you do not set
resp, _ := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
    TypeName: "coralogix_<name>", PriorState: enc(prior), Config: enc(config), ProposedNewState: enc(proposed),
})
planned, _ := resp.PlannedState.Unmarshal(object)   // also read resp.RequiresReplace, resp.Diagnostics
```

**Emulating core:** you supply `ProposedNewState` yourself. Terraform's rule (`plans/objchange`): config value where set; for `Computed` attributes with a null config, the prior value — unless the prior object contains a non-computed attribute, which means it came from config.

**Why:** it isolates the provider from the backend, so a diff between two runs is caused by your change, not by API state. Compare variants by toggling the schema and re-running, and delete the file when done — it is a probe, not a test.
