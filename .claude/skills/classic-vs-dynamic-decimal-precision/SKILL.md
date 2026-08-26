---
name: classic-vs-dynamic-decimal-precision
description: "Use when wiring decimalPrecision on a coralogix_dashboard widget. It is a bool on classic widgets and an int32 on dynamic ones, so copying either side's code silently produces the wrong type."
---

# `decimalPrecision` is a bool on classic widgets and an int32 on dynamic ones

**Trigger:** adding or reviewing `decimal_precision` on a `coralogix_dashboard` widget, or copying
the block from one widget family to the other.

**Fix:** read the SDK field's Go type first.

```go
DecimalPrecision *bool  // classic: a switch turning value abbreviation off -> BoolAttribute
DecimalPrecision *int32 // dynamic: a count of decimal places, 0-15      -> Int64Attribute
```

Classic widgets pair it with a separate `decimal` field; dynamic ones have no `decimal`. Say which
meaning applies in the `MarkdownDescription`, since the attribute name is identical.

**Why:** one JSON name carries two meanings. Copying a whole schema block across families gives a
schema that compiles and type-checks, then rejects or misreads every value.
