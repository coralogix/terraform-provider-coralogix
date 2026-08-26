---
name: schema-attribute-needs-matching-attr-type-map
description: "Use when a coralogix_dashboard apply or state upgrade fails with 'Value Conversion Error' at 'Path: layout' after adding a widget attribute. Widen the hand-written attr.Type map too."
---

# A new schema attribute needs a matching attr.Type map entry

**Trigger:** You added an attribute to a widget schema. The build passes, but tests or applies
fail with `Value Conversion Error` and `Path: layout`, listing two near-identical type dumps.

**Fix:** Widen every hand-written `map[string]attr.Type` that describes the same object, not just
the schema. For dashboards that is `widgetModelAttr()` in
`internal/provider/dashboards/resource_coralogix_dashboard.go`:

```go
"color_scheme": types.StringType,
"hash_colors":  types.BoolType,   // add next to the neighbours you copied in the schema
```

Grep both sides before you start; a widget may be described in more than one place:

```bash
rg '"<new_attribute>"' internal/provider/dashboards        # schema sites
rg '"<sibling_attribute>": types\.' internal/provider/dashboards   # attr.Type map sites
```

To find the mismatch fast, diff the two type dumps from the error rather than reading them.

Faster: `go test ./internal/provider/dashboards/ -run TestWidgetSchemaMatchesWidgetModelAttr`
walks the whole widget tree and names the attribute that is in one side and not the other.

**Why:** `layout` is a `types.List` of widget objects. The framework builds the element type from
the attr.Type map and the state type from the schema. It rejects any object whose two descriptions
differ, even by one attribute. Whether the Go model field is a struct pointer or a `types.Object`
does not matter — what matters is that some ancestor is a list or object described by a hand-written
map, which is true of every dashboard widget.
