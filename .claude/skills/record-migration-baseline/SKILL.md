---
name: record-migration-baseline
description: "Use when recording or reviewing a handwritten Terraform resource's protocol golden baseline before generator migration. Check schema, writes, reads, and omitted fields."
---

# Record a migration baseline

**Trigger:** A resource needs protocol goldens before it switches to generated code.

**Fix:** Check the full schema and each operation before recording goldens:

Record current behavior from the handwritten resource on the target base commit. Keep the baseline change limited to tests, fixtures, and notes. Link each observed behavior to a golden. Do not silently fix provider behavior in the baseline PR.

1. Read the resource schema, model, expand and flatten paths, CRUD methods, import, and the relevant protobuf and OpenAPI definitions. List every resource variant and every field. Include defaults, validators, plan modifiers, sensitive fields, and write-only version fields.
2. Build a coverage matrix before recording. For each writable variant, cover create, refresh, update, delete, and import. For every managed field, cover a value change. For each optional managed field, cover omission on both Create and Update. If omitted Create fails, first create a valid resource, then remove the field on Update and record the next refresh and plan. Give each computed field a non-null service reply. Include nested optional fields, null, empty, and default behavior where they differ. Trace every schema variant through create and update. Record read-only or deprecated variants through import, and record what create and update do with them.
3. Trace each omitted value through Terraform, expansion, SDK serialization, and service validation. Do not infer an enum's wire value from its name: check its zero value and the actual request. A fake service must return the observed service result for a case that depends on validation. If service behavior is unknown, identify the limit in the golden notes instead of inventing a reply.
4. Record protocol plans, request bodies and status, diagnostics, state, and the next plan. Include API errors, missing resources, partial writes, and duplicate or unordered values when they affect state. For write-only fields, record rotation, removal or a switch to the ordinary field, then refresh and plan. Compare the coverage matrix with the golden file list. Resolve every missing cell or state why it does not apply.
5. Use only synthetic values. Do not read credentials. Do not put secrets, tenant IDs, private links, or internal ticket details in a public repository. Run the baseline tests and review the final diff and fixture content before opening the PR.

**Why:** A missing case can make a migration appear safe when it changes released behavior.

The [migration review rubric](../../../docs/generator-migration-review-rubric.md) applies when the generated resource replaces this baseline.
