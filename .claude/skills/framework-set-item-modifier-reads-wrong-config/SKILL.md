---
name: framework-set-item-modifier-reads-wrong-config
description: "Use when a plan modifier on an attribute inside a SetNestedAttribute item reads req.ConfigValue or req.StateValue, or a set item's value turns (known after apply) with no config change."
---

# A plan modifier inside a set item gets another item's config and state

**Trigger:** a modifier on `groups[*].interval`-style attributes compares config, state and plan, and real Terraform shows a sibling item's value as `(known after apply)` on every plan.

**Fix:** don't trust `req.ConfigValue` or `req.StateValue` there. The framework (v1.17, `fwserver/attribute_plan_modification.go`) takes both from the item at the same *index*. Config, state and plan sort their set items separately, so that item can be a different one. Instead:
- use `req.PlanValue` as the prior value, since Terraform proposed it from the prior item it paired;
- find the configured item by matching the planned item: the `AtSetValue` step of `req.Path` holds it.

For the generated version, see `setItemConfig` in `tools/iac-codegen/internal/generator/templates/schema.go.tmpl`.

**Why:** offline modifier unit tests pass the "right" `ConfigValue` by hand, so the bug only shows up with real Terraform. Cover it with `resource.UnitTest` and two items: one that sets the field and one that leaves it out.
