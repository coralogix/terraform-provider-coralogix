---
name: switch-resource-to-generated
description: "Use when a handwritten resource that users already have becomes generated code (tools/iac-codegen existing-resource mode). Record a baseline first, then compare. Do NOT use for a new resource."
---

# Switch a handwritten resource to generated code

**Trigger:** A released resource moves to `tools/iac-codegen`, and its users must see no change.

**Recipe:**
1. **Baseline first.** Before any change, drive the provider through the plugin protocol against an in-memory backend, and keep the results as golden files: a schema dump (with descriptions, validators, defaults, plan modifiers) and scenarios (create, refresh, update, unset, import, 404, state upgrade). Copy `internal/provider/global_router_baseline_*_test.go`. The harness needs only the resource name.
2. **Check the fake against the real API.** Record real answers with a recorder that stays outside the repo, and compare them offline (`TestGlobalRouterFakeMatchesRecordedBackend`). A fake that is built from notes hides real behavior: the first comparison found 7 differences.
3. **Write `behavior-overrides.yaml`** in the output directory: one line per released field behavior. Run `tfgen check --overrides`. Add `PINNED-SDK` lines for facts the pinned SDK lacks.
4. **Generate, register, delete the handwritten code.** The golden diff must show only wording of messages.
5. **Fix confirmed bugs in a separate step.** The golden diff must show only the fixes. Do not keep a bug with an override line.

**Why:** Terraform plan modifiers, defaults, and null/empty reads decide whether an upgrade is silent. A schema dump and a second plan after apply catch what a compile cannot.
