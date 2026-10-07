---
name: tfgen-set-of-objects-and-go-names
description: "Use when tfgen check fails with RENDERER_SHAPE_UNSUPPORTED set of object, or RENDERER_OUTPUT_INVALID on a dotted OpenAPI component like notification_center.ConnectorConfigField. Fix collectionConv and modelTypeName, not the generated files."
---

# tfgen sets of objects and namespaced components

**Trigger:** `tfgen check` reports `set of object is not supported`, or generated `model.go` has an invalid type such as `notification_center.FooModel`.

**Fix:**
- Sets of objects: `collectionConv` must accept `model.Set` of objects (empty objects still rejected). Expand/flatten use `expandElements` and `flattenSet`; nested diagnostics use the collection path, not `AtListIndex`. When `Object.NeedsPrior` is true, flatten must match prior set elements with `matchPriorSetItem` (identity via `same*`), not slice index: Terraform and the API may enumerate the set in different orders. `same*` must compare nested `keepPriorOrder` lists and sets with `sameUnordered`, not `sameList`; otherwise a shuffled nested list makes the parent look like a different set element.
- Namespaced components: `modelTypeName` must be `model.GoName(schema)+"Model"`, same as SDK `camelize`. Do not concatenate the raw schema name.

**Why:** OpenAPI `x-coralogix-collection: set` on object arrays is the contract for Terraform Sets. Dotted component names are valid OpenAPI keys but not Go identifiers.
