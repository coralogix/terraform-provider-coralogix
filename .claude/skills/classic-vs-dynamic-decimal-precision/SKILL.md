---
name: classic-vs-dynamic-decimal-precision
description: "Use when wiring decimalPrecision on a coralogix_dashboard widget. It is a bool on classic widgets and an int32 on dynamic ones, so copying either side's code silently produces the wrong type."
---

# `decimalPrecision` is a bool on classic widgets and an int32 on dynamic ones

**Trigger:** You are adding or reviewing `decimal_precision` on a `coralogix_dashboard` widget, or
copying an existing `decimal_precision` block from one widget family to another.

**Fix:** Check the SDK field's Go type before choosing the Terraform type.

```go
// classic widgets: model_bar_chart.go, model_widgets_gauge.go,
// model_widgets_pie_chart.go, model_line_chart_query_definition.go
DecimalPrecision *bool   `json:"decimalPrecision,omitempty"`   // -> schema.BoolAttribute

// dynamic widgets: model_vertical_bars.go, model_stat_card.go, ...
DecimalPrecision *int32  `json:"decimalPrecision,omitempty"`   // -> schema.Int64Attribute, 0-15
```

On the classic widgets the field is a switch that turns value abbreviation off (`1200` instead of
`1.2K`). On the dynamic ones it is a count of decimal places, and the separate `decimal` field does
not exist. Say which meaning applies in the `MarkdownDescription`, because the attribute name is the
same on both.

```bash
grep -rn "DecimalPrecision \*" ~/go/pkg/mod/github.com/coralogix/coralogix-management-sdk@<version>/go/openapi/gen/dashboard_service/
```

**Why:** One JSON name carries two meanings across the dashboard API. A wrong choice compiles: a
bool assigned where an int32 is expected fails the build, but copying a whole schema block from the
other family gives a schema that type-checks and then rejects or silently misreads every value.
