// Copyright 2025 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboard_widgets

import (
	"fmt"
	"math"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ObservationFieldSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"keypath": schema.ListAttribute{
			ElementType: types.StringType,
			Required:    true,
			Validators: []validator.List{
				listvalidator.SizeAtLeast(1),
			},
			MarkdownDescription: "Ordered path segments. Single element for literal-dot identifiers (`[\"log.level\"]`); multiple elements for nested paths (`[\"meta\",\"responseTime\"]`).",
		},
		"scope": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidObservationFieldScope...),
			},
			MarkdownDescription: "Where the field lives. Disambiguates fields with the same name across scopes (e.g. `timestamp` in metadata vs user data).",
		},
	}
}

func SpansFilterSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.SingleNestedAttribute{
					Attributes: SpansFieldAttributes(),
					Required:   true,
				},
				"operator": FilterOperatorSchema(),
			},
		},
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

func SpansFieldSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Attributes: SpansFieldAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansFieldValidator{},
		},
	}
}

func SpansFieldsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: SpansFieldAttributes(),
			Validators: []validator.Object{
				spansFieldValidator{},
			},
		},
		Optional: true,
	}
}

func SpansFieldAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidSpanFieldTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the field. Can be one of %q", DashboardValidSpanFieldTypes),
		},
		"value": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The value of the field. When the field type is `metadata`, can be one of %q", DashboardValidSpanFieldMetadataFields),
		},
	}
}

func SpansAggregationsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: SpansAggregationAttributes(),
			Validators: []validator.Object{
				spansAggregationValidator{},
			},
		},
		Optional: true,
	}
}

func SpansAggregationSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Attributes: SpansAggregationAttributes(),
		Optional:   true,
		Validators: []validator.Object{
			spansAggregationValidator{},
		},
	}
}
func SpansAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidSpanAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("Can be one of %q", DashboardValidSpanAggregationTypes),
		},
		"aggregation_type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. When the aggregation type is `metrics`, can be one of %q. When the aggregation type is `dimension`, can be one of %q.", DashboardValidSpansAggregationMetricAggregationTypes, DashboardValidSpansAggregationDimensionAggregationTypes),
		},
		"field": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: fmt.Sprintf("The field to aggregate on. When the aggregation type is `metrics`, can be one of %q. When the aggregation type is `dimension`, can be one of %q.", DashboardValidSpansAggregationMetricFields, DashboardValidSpansAggregationDimensionFields),
		},
	}
}

func MetricFiltersSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"metric": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Metric name to apply the filter on.",
				},
				"label": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Label associated with the metric.",
				},
				"operator": FilterOperatorSchema(),
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		Optional: true,
	}
}

func TimeFrameSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"absolute": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"start": schema.StringAttribute{
						Required: true,
					},
					"end": schema.StringAttribute{
						Required: true,
					},
				},
				Optional:            true,
				MarkdownDescription: "Absolute time frame specifying a fixed start and end time.",
			},
			"relative": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"duration": schema.StringAttribute{
						Required: true,
					},
				},
				Optional:            true,
				MarkdownDescription: "Relative time frame specifying a duration from the current time.",
			},
		},
		Validators: []validator.Object{
			ExactlyOneOfChildren("absolute", "relative"),
		},
		MarkdownDescription: "Specifies the time frame. Can be either absolute or relative.",
	}
}

func LogsAggregationSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required:   true,
		Attributes: LogsAggregationAttributes(),
		Validators: []validator.Object{
			logsAggregationValidator{},
		},
	}
}

func LogsAggregationsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Required: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: LogsAggregationAttributes(),
			Validators: []validator.Object{
				logsAggregationValidator{},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

func LogsAggregationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidLogsAggregationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The type of the aggregation. Can be one of %q", DashboardValidLogsAggregationTypes),
		},
		"field": schema.StringAttribute{
			Optional: true,
		},
		"percent": schema.Float64Attribute{
			Optional: true,
			Validators: []validator.Float64{
				float64validator.Between(0, 100),
			},
			MarkdownDescription: "The percentage of the aggregation to return. required when type is `percentile`.",
		},
		"observation_field": schema.SingleNestedAttribute{
			Attributes:          ObservationFieldSchema(),
			Optional:            true,
			MarkdownDescription: "Explicit field reference with scope. Use when the field name contains a literal dot (e.g. `log.level`) or exists in multiple scopes — the bare `field` is resolved by the backend via dot-split, which silently fails to match flat fields whose identifier contains dots.",
		},
	}
}

func LegendSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"is_visible": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether to display the legend. True by default.",
			},
			"columns": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf(DashboardValidLegendColumns...)),
					listvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: fmt.Sprintf("The columns to display in the legend. Valid values are: %s.", strings.Join(DashboardValidLegendColumns, ", ")),
			},
			"group_by_query": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"placement": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					// The API owns the value when the attribute is omitted, so keep what
					// it chose instead of planning unknown on every run.
					stringplanmodifier.UseNonNullStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLegendPlacements...),
				},
				MarkdownDescription: fmt.Sprintf("The placement of the legend. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(DashboardValidLegendPlacements, ", ")),
			},
		},
		Optional: true,
	}
}

func LogsFiltersSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Optional: true,
				},
				"operator": FilterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes:          ObservationFieldSchema(),
					Optional:            true,
					MarkdownDescription: "Explicit field reference with scope. Use when the field name contains a literal dot (e.g. `log.level`) or exists in multiple scopes — the bare `field` is resolved by the backend via dot-split, which silently fails to match flat fields whose identifier contains dots.",
				},
			},
			Validators: []validator.Object{
				ExactlyOneOfChildren("field", "observation_field"),
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

func UnitSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidUnits...),
		},
		MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(DashboardValidUnits, ", ")),
	}
}

func FiltersSourceSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"logs": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"field": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Field in the logs to apply the filter on.",
				},
				"operator": FilterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes:          ObservationFieldSchema(),
					Optional:            true,
					MarkdownDescription: "Explicit field reference with scope. Use when the field name contains a literal dot (e.g. `log.level`) or exists in multiple scopes — the bare `field` is resolved by the backend via dot-split, which silently fails to match flat fields whose identifier contains dots.",
				},
			},
			Validators: []validator.Object{
				ExactlyOneOfChildren("field", "observation_field"),
			},
			Optional: true,
		},
		"spans": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"field": schema.SingleNestedAttribute{
					Attributes: SpansFieldAttributes(),
					Required:   true,
					Validators: []validator.Object{
						spansFieldValidator{},
					},
				},
				"operator": FilterOperatorSchema(),
			},
			Optional: true,
		},
		"metrics": schema.SingleNestedAttribute{
			Attributes: map[string]schema.Attribute{
				"metric_name": schema.StringAttribute{
					Optional: true,
				},
				"label": schema.StringAttribute{
					Optional: true,
				},
				"operator": FilterOperatorSchema(),
			},
			Optional: true,
		},
	}
}

func FilterOperatorSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("equals", "not_equals"),
				},
				MarkdownDescription: "The type of the operator. Can be one of `equals` or `not_equals`.",
			},
			"selection_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(filterSelectionTypeAll, filterSelectionTypeList),
				},
				PlanModifiers: []planmodifier.String{
					filterSelectionTypeFromSelectedValues{},
				},
				MarkdownDescription: "How the operator selects values. Use `all` to select every value. Use `list` to select only `selected_values`. If omitted, an empty legacy `selected_values` list means `all`.",
			},
			"selected_values": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
				MarkdownDescription: "Values to filter by. For `equals`, set `selection_type` to `list` to represent an empty selection. If `selection_type` is omitted, an empty list selects all values for backward compatibility. For `not_equals`, this list must contain at least one value.",
			},
		},
		Validators: []validator.Object{
			filterOperatorValidator{},
		},
		Required:            true,
		MarkdownDescription: "Operator to use for filtering.",
	}
}

func ColorsBySchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidColorsBy...),
		},
		MarkdownDescription: fmt.Sprintf("What colors are derived from. Valid values are: %s.", strings.Join(DashboardValidColorsBy, ", ")),
	}
}

func HashColorsSchema() schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:            true,
		MarkdownDescription: "When true, each series takes a color from a hash of its name, and `color_scheme` is ignored. The Coralogix UI calls this `Legend Color Hashing`.",
	}
}

func CustomUnitSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		MarkdownDescription: "A custom unit label. Takes effect only when `unit` is `custom`.",
	}
}

// DecimalSchema is the number of decimal places shown for numeric values. The
// API documents the range as 0-15 but the generated type is a plain int32, so
// that bound is left to the API instead of a validator that could reject values
// the API accepts. What the int32 itself cannot hold is rejected, because the
// conversion would otherwise truncate or wrap the value silently.
func DecimalSchema() schema.NumberAttribute {
	return schema.NumberAttribute{
		Optional: true,
		Validators: []validator.Number{
			int32NumberValidator{},
		},
		MarkdownDescription: "The number of decimal places shown for numeric values. Must be a whole number; the API accepts 0 to 15.",
	}
}

// NonEmptySpansFieldsSchema is SpansFieldsSchema with an explicit empty list
// rejected. A zero-length list flattens back as null, so accepting one leaves a
// permanent diff. SpansFieldsSchema itself is shared with the frozen prior
// schemas and cannot gain the validator.
func NonEmptySpansFieldsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: SpansFieldAttributes(),
			Validators: []validator.Object{
				spansFieldValidator{},
			},
		},
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
	}
}

// DecimalPrecisionSchema keeps the API field name. It is a boolean on the
// classic widgets: it turns value abbreviation off. The dynamic widgets use the
// same JSON name for an integer precision count, so do not read one as the other.
func DecimalPrecisionSchema() schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:            true,
		MarkdownDescription: "When true, numeric values are rendered in full instead of abbreviated (`1200` instead of `1.2K`).",
	}
}

func YAxisMaxSchema() schema.Float64Attribute {
	return schema.Float64Attribute{
		Optional:   true,
		CustomType: Float32Type{},
		Validators: []validator.Float64{
			float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
		},
		MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
	}
}

func YAxisMinSchema() schema.Float64Attribute {
	return schema.Float64Attribute{
		Optional:   true,
		CustomType: Float32Type{},
		Validators: []validator.Float64{
			float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
		},
		MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
	}
}

func XAxisTimeFormatSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			// The API owns the value when the attribute is omitted, so keep what it
			// chose. Non-null only: a list element added on update has null prior
			// state, and copying that null in would break the apply.
			stringplanmodifier.UseNonNullStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidXAxisTimeFormats...),
		},
		MarkdownDescription: fmt.Sprintf("The x-axis time format. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(DashboardValidXAxisTimeFormats, ", ")),
	}
}

func MetricsEditorModeSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			// The API owns the value when the attribute is omitted, so keep what it
			// chose. Non-null only: a list element added on update has null prior
			// state, and copying that null in would break the apply.
			stringplanmodifier.UseNonNullStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidMetricsEditorModes...),
		},
		MarkdownDescription: fmt.Sprintf("Which query editor the Coralogix UI opens for this query. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(DashboardValidMetricsEditorModes, ", ")),
	}
}

func MetricsSeriesLimitTypeSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			// The API owns the value when the attribute is omitted, so keep what it
			// chose. Non-null only: a list element added on update has null prior
			// state, and copying that null in would break the apply.
			stringplanmodifier.UseNonNullStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.OneOf(dashboardValidMetricsSeriesLimitTypes...),
		},
		MarkdownDescription: fmt.Sprintf("How the series limit is counted. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(dashboardValidMetricsSeriesLimitTypes, ", ")),
	}
}

func PromQLQueryTypeSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			// The API owns the value when the attribute is omitted, so keep what it
			// chose. Non-null only: a list element added on update has null prior
			// state, and copying that null in would break the apply.
			stringplanmodifier.UseNonNullStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidPromQLQueryType...),
		},
		MarkdownDescription: fmt.Sprintf("The PromQL query type. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(DashboardValidPromQLQueryType, ", ")),
	}
}

// SpanObservationFieldsSchema is the list-of-span-observation-fields shape used
// by `group_bys` and `group_names_fields` on a spans query.
func SpanObservationFieldsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: spanObservationFieldSchema(),
		},
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		MarkdownDescription: "Span observation fields to group the results by. Use these when a field needs an explicit scope or relation type.",
	}
}

// SpanObservationFieldSchema is the single-span-observation-field shape used by
// `stacked_group_name_field` on a spans query.
func SpanObservationFieldSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes:          spanObservationFieldSchema(),
		Optional:            true,
		MarkdownDescription: "Span observation field that divides each group into subgroups. Use this when the field needs an explicit scope or relation type.",
	}
}

// ObservationFieldsSchema is the list-of-observation-fields shape used by
// `group_by` and `group_bys` on a logs query.
func ObservationFieldsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		NestedObject: schema.NestedAttributeObject{
			Attributes: ObservationFieldSchema(),
		},
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		MarkdownDescription: "Observation fields to group the results by. Use these when a field name contains a literal dot, or exists in more than one scope.",
	}
}

func CommonAggregationSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			// The API owns the value when the attribute is omitted, so keep what it
			// chose. Non-null only: a list element added on update has null prior
			// state, and copying that null in would break the apply.
			stringplanmodifier.UseNonNullStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidCommonAggregations...),
		},
		MarkdownDescription: fmt.Sprintf("How the metric series is reduced to one value per group. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(DashboardValidCommonAggregations, ", ")),
	}
}

func BarValueDisplaySchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			// The API owns the value when the attribute is omitted, so keep what it
			// chose. Non-null only: a list element added on update has null prior
			// state, and copying that null in would break the apply.
			stringplanmodifier.UseNonNullStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidBarValueDisplays...),
		},
		MarkdownDescription: fmt.Sprintf("Where the bar value is displayed. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(DashboardValidBarValueDisplays, ", ")),
	}
}

func LegendBySchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.String{
			// The API owns the value when the attribute is omitted, so keep what it
			// chose. Non-null only: a list element added on update has null prior
			// state, and copying that null in would break the apply.
			stringplanmodifier.UseNonNullStateForUnknown(),
		},
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidLegendBys...),
		},
		MarkdownDescription: fmt.Sprintf("What the legend lists. The API chooses a value when this is omitted, so set `unspecified` explicitly to go back to that. Valid values are: %s.", strings.Join(DashboardValidLegendBys, ", ")),
	}
}
