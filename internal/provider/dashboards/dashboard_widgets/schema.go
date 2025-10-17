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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ObservationFieldSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"keypath": schema.ListAttribute{
			ElementType: types.StringType,
			Required:    true,
		},
		"scope": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidObservationFieldScope...),
			},
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
				Optional: true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("relative")),
				},
				MarkdownDescription: "Absolute time frame specifying a fixed start and end time.",
			},
			"relative": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"duration": schema.StringAttribute{
						Required: true,
					},
				},
				Optional: true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("absolute")),
				},
				MarkdownDescription: "Relative time frame specifying a duration from the current time.",
			},
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
			Attributes: ObservationFieldSchema(),
			Optional:   true,
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
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLegendPlacements...),
				},
				MarkdownDescription: fmt.Sprintf("The placement of the legend. Valid values are: %s.", strings.Join(DashboardValidLegendPlacements, ", ")),
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
					Required: true,
				},
				"operator": FilterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchema(),
					Optional:   true,
				},
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
		Default:  stringdefault.StaticString("unspecified"),
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
					Required:            true,
					MarkdownDescription: "Field in the logs to apply the filter on.",
				},
				"operator": FilterOperatorSchema(),
				"observation_field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchema(),
					Optional:   true,
				},
			},
			Optional: true,
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("metrics"),
					path.MatchRelative().AtParent().AtName("spans"),
				),
			},
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
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("metrics"),
					path.MatchRelative().AtParent().AtName("logs"),
				),
			},
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
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("spans"),
					path.MatchRelative().AtParent().AtName("logs"),
				),
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
			"selected_values": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "the values to filter by. When the type is `equals`, this field is optional, the filter will match only the selected values, and all the values if not set. When the type is `not_equals`, this field is required, and the filter will match spans without the selected values.",
			},
		},
		Validators: []validator.Object{
			filterOperatorValidator{},
		},
		Required:            true,
		MarkdownDescription: "Operator to use for filtering.",
	}
}
