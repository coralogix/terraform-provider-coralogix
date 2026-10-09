---
name: switch-resource-to-generated
description: "Use when a handwritten resource that users already have becomes generated code (tools/iac-codegen existing-resource mode). Record a baseline first, then compare. Do NOT use for a new resource."
---

# Switch a handwritten resource to generated code

**Trigger:** A released resource moves to `tools/iac-codegen`, and its users must see no change.

**Recipe:**
1. **Baseline first.** Before any change, drive the provider through the plugin protocol against an in-memory backend, and keep the results as golden files: a schema dump (with descriptions, validators, defaults, plan modifiers) and scenarios (create, refresh, update, unset, import, 404, state upgrade). Copy `internal/provider/global_router_baseline_*_test.go`. The harness needs only the resource name.
2. **Check the fake against the real API.** Record real answers with a recorder that stays outside the repo, and compare them offline (`TestGlobalRouterFakeMatchesRecordedBackend`). A fake that is built from notes hides real behavior: the first comparison found 7 differences.
3. **Write `behavior-overrides.yaml`** in the output directory: one line per released field behavior. Run `tfgen check --overrides`. Add `PINNED-SDK` lines for facts the pinned SDK lacks. When the API holds a value in a one-property object and the released schema shows the plain value (`{"query": {"value": "x"}}` as `query = "x"`), list the component in `unwrap`. Read the handwritten expand and flatten for each one-property object first: many stay objects, and one component can collapse in one place only (an `unwrap: true` or `unwrap: false` field line).
4. **Generate, register, delete the handwritten code.** The golden diff must show only wording of messages.
5. **Fix confirmed bugs in a separate step.** The golden diff must show only the fixes. Do not keep a bug with an override line.
6. **Later API changes.** A new optional scalar field generates without a line only if the contract has `x-coralogix-presence: true`; otherwise the generator stops, so ask for the annotation or probe a read and write a line. A new enum value stops the generator until it is in `values` or `rejected`.
7. **Generated acceptance test.** Add `acceptance.yaml` next to the overrides (prerequisite HCL, real values, `skip`, `minimal`, `upgradeFrom`) and run `tfgen generate`. Keep the hand test until the generated one has passed on a tenant.

**Why:** Terraform plan modifiers, defaults, and null/empty reads decide whether an upgrade is silent. A schema dump and a second plan after apply catch what a compile cannot.
