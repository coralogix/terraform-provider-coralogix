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
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	dashboardSchemaToProtoSpanRelationType = map[string]dashboardservice.SpanRelationType{
		utils.UNSPECIFIED: dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_NONE_UNSPECIFIED,
		"other":           dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_OTHER,
		"parent":          dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_PARENT,
		"root":            dashboardservice.SPANRELATIONTYPE_SPAN_RELATION_TYPE_ROOT,
	}
	dashboardProtoToSchemaSpanRelationType = utils.ReverseMap(dashboardSchemaToProtoSpanRelationType)
	dashboardValidSpanRelationTypes        = utils.GetKeys(dashboardSchemaToProtoSpanRelationType)

	dashboardSchemaToProtoLegendBy = map[string]dashboardservice.LegendBy{
		utils.UNSPECIFIED: dashboardservice.LEGENDBY_LEGEND_BY_UNSPECIFIED,
		"thresholds":      dashboardservice.LEGENDBY_LEGEND_BY_THRESHOLDS,
		"groups":          dashboardservice.LEGENDBY_LEGEND_BY_GROUPS,
	}
	dashboardProtoToSchemaLegendBy = utils.ReverseMap(dashboardSchemaToProtoLegendBy)
	dashboardValidLegendBy         = utils.GetKeys(dashboardSchemaToProtoLegendBy)

	dashboardSchemaToProtoThresholdBy = map[string]dashboardservice.CommonThresholdBy{
		utils.UNSPECIFIED: dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_UNSPECIFIED,
		"value":           dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_VALUE,
		"background":      dashboardservice.COMMONTHRESHOLDBY_THRESHOLD_BY_BACKGROUND,
	}
	dashboardProtoToSchemaThresholdBy = utils.ReverseMap(dashboardSchemaToProtoThresholdBy)
	dashboardValidThresholdBy         = utils.GetKeys(dashboardSchemaToProtoThresholdBy)

	dashboardSchemaToProtoColorApplyTarget = map[string]dashboardservice.ColorApplyTarget{
		utils.UNSPECIFIED: dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_UNSPECIFIED,
		"value":           dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_VALUE,
		"background":      dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_BACKGROUND,
		"row":             dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_ROW,
	}
	dashboardProtoToSchemaColorApplyTarget = utils.ReverseMap(dashboardSchemaToProtoColorApplyTarget)
	dashboardValidColorApplyTarget         = utils.GetKeys(dashboardSchemaToProtoColorApplyTarget)

	dashboardSchemaToProtoColorSolidType = map[string]dashboardservice.ColorSolidType{
		utils.UNSPECIFIED: dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_UNSPECIFIED,
		"blue":            dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_BLUE,
		"green":           dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_GREEN,
		"red":             dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_RED,
		"purple":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_PURPLE,
		"cyan":            dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_CYAN,
		"magenta":         dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_MAGENTA,
		"orange":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_ORANGE,
		"yellow":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_YELLOW,
	}
	dashboardProtoToSchemaColorSolidType = utils.ReverseMap(dashboardSchemaToProtoColorSolidType)
	dashboardValidColorSolidType         = utils.GetKeys(dashboardSchemaToProtoColorSolidType)

	dashboardSchemaToProtoTextAlignment = map[string]dashboardservice.TextAlignment{
		utils.UNSPECIFIED: dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_UNSPECIFIED,
		"left":            dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_LEFT,
		"center":          dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_CENTER,
		"right":           dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_RIGHT,
	}
	dashboardProtoToSchemaTextAlignment = utils.ReverseMap(dashboardSchemaToProtoTextAlignment)
	dashboardValidTextAlignment         = utils.GetKeys(dashboardSchemaToProtoTextAlignment)

	dashboardSchemaToProtoValuesMappingType = map[string]dashboardservice.ValuesMappingType{
		utils.UNSPECIFIED: dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_UNSPECIFIED,
		"value":           dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_VALUE,
		"regex":           dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_REGEX,
	}
	dashboardProtoToSchemaValuesMappingType = utils.ReverseMap(dashboardSchemaToProtoValuesMappingType)
	dashboardValidValuesMappingType         = utils.GetKeys(dashboardSchemaToProtoValuesMappingType)

	dashboardSchemaToProtoFieldDataType = map[string]dashboardservice.FieldDataType{
		utils.UNSPECIFIED: dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_UNSPECIFIED,
		"number":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_NUMBER,
		"string":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_STRING,
		"boolean":         dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_BOOLEAN,
		"timestamp":       dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_TIMESTAMP,
		"object":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_OBJECT,
		"array":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_ARRAY,
		"regex":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_REGEX,
		"union":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_UNION,
		"enum":            dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_ENUM,
	}
	dashboardProtoToSchemaFieldDataType = utils.ReverseMap(dashboardSchemaToProtoFieldDataType)
	dashboardValidFieldDataType         = utils.GetKeys(dashboardSchemaToProtoFieldDataType)
)

func DynamicSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"query_definitions": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
						"query": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"logs":       dynamicLogsQuerySchema(),
								"spans":      dynamicSpansQuerySchema(),
								"metrics":    dynamicMetricsQuerySchema(),
								"data_prime": dynamicDataPrimeQuerySchema(),
							},
							Validators: []validator.Object{
								ExactlyOneOfChildren("logs", "spans", "metrics", "data_prime"),
							},
						},
					},
				},
			},
			"time_frame": TimeFrameSchema(),
			"visualization": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"stat":      dynamicStatSchema(),
					"stat_card": dynamicStatCardSchema(),
					"table":     dynamicTableSchema(),
				},
				Validators: []validator.Object{
					ExactlyOneOfChildren("stat", "stat_card", "table"),
				},
			},
		},
		Validators: []validator.Object{
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
	}
}

func dynamicLogsQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"lucene_query": schema.StringAttribute{
				Optional: true,
			},
			"group_by": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"aggregations":   dynamicAggregationsSchema(),
			"filters":        LogsFiltersSchema(),
			"data_mode_type": dynamicDataModeTypeSchema(),
		},
	}
}

func dynamicSpansQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"lucene_query": schema.StringAttribute{
				Optional: true,
			},
			"group_by": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: spanObservationFieldSchema(),
				},
			},
			"aggregations":   dynamicAggregationsSchema(),
			"filters":        SpansFilterSchema(),
			"data_mode_type": dynamicDataModeTypeSchema(),
		},
	}
}

func dynamicMetricsQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"promql_query": schema.StringAttribute{
				Required: true,
			},
			"promql_query_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidPromQLQueryType...),
				},
				MarkdownDescription: fmt.Sprintf("The PromQL query type. Valid values are: %s.", strings.Join(DashboardValidPromQLQueryType, ", ")),
			},
		},
	}
}

func dynamicDataPrimeQuerySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				Optional: true,
			},
			"data_mode_type": dynamicDataModeTypeSchema(),
		},
	}
}

func dynamicAggregationsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: LogsAggregationAttributes(),
			Validators: []validator.Object{
				logsAggregationValidator{},
			},
		},
	}
}

func dynamicDataModeTypeSchema() schema.Attribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString(utils.UNSPECIFIED),
		Validators: []validator.String{
			stringvalidator.OneOf(DashboardValidDataModeTypes...),
		},
		MarkdownDescription: fmt.Sprintf("The data mode type. Valid values are: %s.", strings.Join(DashboardValidDataModeTypes, ", ")),
	}
}

func spanObservationFieldSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"keypath": schema.ListAttribute{
			ElementType:         types.StringType,
			Required:            true,
			MarkdownDescription: "Ordered path segments identifying the span field.",
		},
		"scope": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(DashboardValidObservationFieldScope...),
			},
			MarkdownDescription: fmt.Sprintf("Where the field lives. Valid values are: %s.", strings.Join(DashboardValidObservationFieldScope, ", ")),
		},
		"relation_type": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(utils.UNSPECIFIED),
			Validators: []validator.String{
				stringvalidator.OneOf(dashboardValidSpanRelationTypes...),
			},
			MarkdownDescription: fmt.Sprintf("The span relation type. Valid values are: %s.", strings.Join(dashboardValidSpanRelationTypes, ", ")),
		},
	}
}

func dynamicStatSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, math.MaxInt32),
				},
			},
			"display_series_name": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"legend_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidLegendBy...),
				},
				MarkdownDescription: fmt.Sprintf("How the legend is grouped. Valid values are: %s.", strings.Join(dashboardValidLegendBy, ", ")),
			},
			"max": schema.Float64Attribute{
				Optional: true,
			},
			"min": schema.Float64Attribute{
				Optional: true,
			},
			"threshold_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidThresholdBy...),
				},
				MarkdownDescription: fmt.Sprintf("Which part of the widget the threshold colors. Valid values are: %s.", strings.Join(dashboardValidThresholdBy, ", ")),
			},
			"threshold_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidThresholdTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
			},
			"thresholds": dynamicThresholdsSchema(),
			"unit": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidUnits...),
				},
				MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(DashboardValidUnits, ", ")),
			},
			"value_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
		},
	}
}

func dynamicThresholdsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"from": schema.Float64Attribute{
					Optional: true,
				},
				"color": schema.StringAttribute{
					Optional: true,
				},
				"label": schema.StringAttribute{
					Optional: true,
				},
			},
		},
	}
}

func dynamicStatCardSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_label_mapping": dynamicColorLabelMappingSchema(),
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, math.MaxInt32),
				},
			},
			"label":  dynamicStatVisualElementSchema(),
			"legend": LegendSchema(),
			"legend_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidLegendBy...),
				},
				MarkdownDescription: fmt.Sprintf("How the legend is grouped. Valid values are: %s.", strings.Join(dashboardValidLegendBy, ", ")),
			},
			"primary_value": dynamicStatVisualElementSchema(),
			"title":         dynamicStatVisualElementSchema(),
			"unit": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidUnits...),
				},
				MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(DashboardValidUnits, ", ")),
			},
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
		},
	}
}

func dynamicStatVisualElementSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"mapped_values": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Mapped values encoded as a JSON object string.",
				PlanModifiers: []planmodifier.String{
					utils.PreserveStateForEquivalentJSON{},
				},
			},
			"observation_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"template_text": schema.StringAttribute{
				Optional: true,
			},
			"template_variables": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mapped_values": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Mapped values encoded as a JSON object string.",
							PlanModifiers: []planmodifier.String{
								utils.PreserveStateForEquivalentJSON{},
							},
						},
						"observation_field": schema.SingleNestedAttribute{
							Attributes: ObservationFieldSchema(),
							Optional:   true,
						},
					},
				},
			},
		},
	}
}

func dynamicColorLabelMappingSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"color_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidColorApplyTarget...),
				},
				MarkdownDescription: fmt.Sprintf("Which part of the widget the color applies to. Valid values are: %s.", strings.Join(dashboardValidColorApplyTarget, ", ")),
			},
			"range": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"min_max": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"auto": schema.BoolAttribute{
								Optional: true,
							},
							"custom": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"max": schema.Float64Attribute{
										Optional: true,
									},
									"min": schema.Float64Attribute{
										Optional: true,
									},
								},
							},
						},
						Validators: []validator.Object{
							ExactlyOneOfChildren("auto", "custom"),
						},
					},
					"threshold_type": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString(utils.UNSPECIFIED),
						Validators: []validator.String{
							stringvalidator.OneOf(DashboardValidThresholdTypes...),
						},
						MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
					},
					"thresholds": dynamicThresholdsSchema(),
				},
			},
			"regex": dynamicMappingSectionsSchema(),
			"value": dynamicMappingSectionsSchema(),
		},
	}
}

func dynamicMappingSectionsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"sections": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"color": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(utils.UNSPECIFIED),
							Validators: []validator.String{
								stringvalidator.OneOf(dashboardValidColorSolidType...),
							},
							MarkdownDescription: fmt.Sprintf("The section color. Valid values are: %s.", strings.Join(dashboardValidColorSolidType, ", ")),
						},
						"map_to": schema.StringAttribute{
							Optional: true,
						},
						"value": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func dynamicTableSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"columns": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.SingleNestedAttribute{
							Attributes: ObservationFieldSchema(),
							Optional:   true,
						},
					},
				},
			},
			"rules": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{
							Optional: true,
						},
						"id": schema.StringAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
						"properties": schema.ListNestedAttribute{
							Optional: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Optional: true,
										Computed: true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseNonNullStateForUnknown(),
										},
									},
									"definition": dynamicTablePropertyDefinitionSchema(),
								},
							},
						},
						"rule_scope": dynamicTableRuleScopeSchema(),
					},
				},
			},
			"settings": dynamicTableSettingsSchema(),
		},
	}
}

func dynamicTablePropertyDefinitionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"alignment": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidTextAlignment...),
				},
				MarkdownDescription: fmt.Sprintf("The text alignment. Valid values are: %s.", strings.Join(dashboardValidTextAlignment, ", ")),
			},
			"column_display_name": schema.StringAttribute{
				Optional: true,
			},
			"link": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"actions": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseNonNullStateForUnknown(),
									},
								},
								"name": schema.StringAttribute{
									Optional: true,
								},
								"should_open_in_new_window": schema.BoolAttribute{
									Optional: true,
								},
								"url": schema.StringAttribute{
									Optional: true,
								},
							},
						},
					},
				},
			},
			"regex_extract": schema.StringAttribute{
				Optional: true,
			},
			"thresholds": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"max": schema.Float64Attribute{
						Optional: true,
					},
					"min": schema.Float64Attribute{
						Optional: true,
					},
					"type": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString(utils.UNSPECIFIED),
						Validators: []validator.String{
							stringvalidator.OneOf(DashboardValidThresholdTypes...),
						},
						MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
					},
					"values": dynamicThresholdsSchema(),
				},
			},
			"units": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"allow_abbreviation": schema.BoolAttribute{
						Optional: true,
					},
					"custom_unit": schema.StringAttribute{
						Optional: true,
					},
					"decimal_precision": schema.Int64Attribute{
						Optional: true,
						Validators: []validator.Int64{
							int64validator.Between(0, math.MaxInt32),
						},
					},
					"max": schema.Float64Attribute{
						Optional: true,
					},
					"min": schema.Float64Attribute{
						Optional: true,
					},
					"unit": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString(utils.UNSPECIFIED),
						Validators: []validator.String{
							stringvalidator.OneOf(DashboardValidUnits...),
						},
						MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(DashboardValidUnits, ", ")),
					},
				},
			},
			"values_alias": schema.StringAttribute{
				Optional: true,
			},
			"values_mapping": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"mappings": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"input_value": schema.StringAttribute{
									Optional: true,
								},
								"replace_value": schema.StringAttribute{
									Optional: true,
								},
								"type": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString(utils.UNSPECIFIED),
									Validators: []validator.String{
										stringvalidator.OneOf(dashboardValidValuesMappingType...),
									},
									MarkdownDescription: fmt.Sprintf("The values mapping type. Valid values are: %s.", strings.Join(dashboardValidValuesMappingType, ", ")),
								},
							},
						},
					},
				},
			},
		},
	}
}

func dynamicTableRuleScopeSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"field_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidFieldDataType...),
				},
				MarkdownDescription: fmt.Sprintf("The field data type. Valid values are: %s.", strings.Join(dashboardValidFieldDataType, ", ")),
			},
			"regex": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

func dynamicTableSettingsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"column_widths": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"column_name": schema.StringAttribute{
							Optional: true,
						},
						"width": schema.Int64Attribute{
							Optional: true,
							Validators: []validator.Int64{
								int64validator.Between(math.MinInt32, math.MaxInt32),
							},
						},
					},
				},
			},
			"row_style": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidRowStyles...),
				},
				MarkdownDescription: fmt.Sprintf("The row style. Valid values are: %s.", strings.Join(DashboardValidRowStyles, ", ")),
			},
		},
	}
}

func DynamicType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: dynamicModelAttr(),
	}
}

func dynamicModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query_definitions": types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: dynamicQueryDefinitionModelAttr(),
			},
		},
		"time_frame": types.ObjectType{
			AttrTypes: TimeFrameModelAttr(),
		},
		"visualization": types.ObjectType{
			AttrTypes: dynamicVisualizationModelAttr(),
		},
	}
}

func dynamicQueryDefinitionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":   types.StringType,
		"name": types.StringType,
		"query": types.ObjectType{
			AttrTypes: dynamicQueryModelAttr(),
		},
	}
}

func dynamicQueryModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs":       types.ObjectType{AttrTypes: dynamicLogsQueryAttr()},
		"spans":      types.ObjectType{AttrTypes: dynamicSpansQueryAttr()},
		"metrics":    types.ObjectType{AttrTypes: dynamicMetricsQueryAttr()},
		"data_prime": types.ObjectType{AttrTypes: dynamicDataPrimeQueryAttr()},
	}
}

func dynamicLogsQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"group_by": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"aggregations": types.ListType{
			ElemType: types.ObjectType{AttrTypes: AggregationModelAttr()},
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{AttrTypes: LogsFilterModelAttr()},
		},
		"data_mode_type": types.StringType,
	}
}

func dynamicSpansQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"lucene_query": types.StringType,
		"group_by": types.ListType{
			ElemType: types.ObjectType{AttrTypes: spanObservationFieldAttr()},
		},
		"aggregations": types.ListType{
			ElemType: types.ObjectType{AttrTypes: AggregationModelAttr()},
		},
		"filters": types.ListType{
			ElemType: types.ObjectType{AttrTypes: SpansFilterModelAttr()},
		},
		"data_mode_type": types.StringType,
	}
}

func dynamicMetricsQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"promql_query":      types.StringType,
		"promql_query_type": types.StringType,
	}
}

func dynamicDataPrimeQueryAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query":          types.StringType,
		"data_mode_type": types.StringType,
	}
}

func spanObservationFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"keypath": types.ListType{
			ElemType: types.StringType,
		},
		"scope":         types.StringType,
		"relation_type": types.StringType,
	}
}

func dynamicVisualizationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"stat":      types.ObjectType{AttrTypes: dynamicStatModelAttr()},
		"stat_card": types.ObjectType{AttrTypes: dynamicStatCardModelAttr()},
		"table":     types.ObjectType{AttrTypes: dynamicTableModelAttr()},
	}
}

func dynamicTableModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"columns": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()},
		},
		"rules": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()},
		},
		"settings": types.ObjectType{AttrTypes: dynamicTableSettingsModelAttr()},
	}
}

func dynamicTableColumnModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": ObservationFieldsObject(),
	}
}

func dynamicTableRuleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"description": types.StringType,
		"id":          types.StringType,
		"name":        types.StringType,
		"properties": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()},
		},
		"rule_scope": types.ObjectType{AttrTypes: dynamicTableRuleScopeModelAttr()},
	}
}

func dynamicTablePropertyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":         types.StringType,
		"definition": types.ObjectType{AttrTypes: dynamicTablePropertyDefinitionModelAttr()},
	}
}

func dynamicTablePropertyDefinitionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"alignment":           types.StringType,
		"column_display_name": types.StringType,
		"link":                types.ObjectType{AttrTypes: dynamicTablePropertyLinkModelAttr()},
		"regex_extract":       types.StringType,
		"thresholds":          types.ObjectType{AttrTypes: dynamicTablePropertyThresholdsModelAttr()},
		"units":               types.ObjectType{AttrTypes: dynamicTablePropertyUnitsModelAttr()},
		"values_alias":        types.StringType,
		"values_mapping":      types.ObjectType{AttrTypes: dynamicTablePropertyValuesMappingModelAttr()},
	}
}

func dynamicTablePropertyLinkModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"actions": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()},
		},
	}
}

func dynamicTableLinkActionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                        types.StringType,
		"name":                      types.StringType,
		"should_open_in_new_window": types.BoolType,
		"url":                       types.StringType,
	}
}

func dynamicTablePropertyThresholdsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max":  types.Float64Type,
		"min":  types.Float64Type,
		"type": types.StringType,
		"values": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
	}
}

func dynamicTablePropertyUnitsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"custom_unit":        types.StringType,
		"decimal_precision":  types.Int64Type,
		"max":                types.Float64Type,
		"min":                types.Float64Type,
		"unit":               types.StringType,
	}
}

func dynamicTablePropertyValuesMappingModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mappings": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()},
		},
	}
}

func dynamicTableValueMappingModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"input_value":   types.StringType,
		"replace_value": types.StringType,
		"type":          types.StringType,
	}
}

func dynamicTableRuleScopeModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field":      ObservationFieldsObject(),
		"field_type": types.StringType,
		"regex":      types.StringType,
	}
}

func dynamicTableSettingsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"column_widths": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()},
		},
		"row_style": types.StringType,
	}
}

func dynamicTableColumnWidthModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"column_name": types.StringType,
		"width":       types.Int64Type,
	}
}

func dynamicStatModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"display_series_name": types.BoolType,
		"legend": types.ObjectType{
			AttrTypes: LegendAttr(),
		},
		"legend_by":      types.StringType,
		"max":            types.Float64Type,
		"min":            types.Float64Type,
		"threshold_by":   types.StringType,
		"threshold_type": types.StringType,
		"thresholds": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
		"unit":        types.StringType,
		"value_field": ObservationFieldsObject(),
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
	}
}

func dynamicThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"from":  types.Float64Type,
		"color": types.StringType,
		"label": types.StringType,
	}
}

func dynamicStatCardModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_label_mapping": types.ObjectType{AttrTypes: dynamicColorLabelMappingAttr()},
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"label":               types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"legend_by":           types.StringType,
		"primary_value":       types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"title":               types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"unit":                types.StringType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
	}
}

func dynamicStatVisualElementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mapped_values":     types.StringType,
		"observation_field": ObservationFieldsObject(),
		"template_text":     types.StringType,
		"template_variables": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()},
		},
	}
}

func dynamicTemplateVariableAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mapped_values":     types.StringType,
		"observation_field": ObservationFieldsObject(),
	}
}

func dynamicColorLabelMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"color_by": types.StringType,
		"range":    types.ObjectType{AttrTypes: dynamicRangeMappingAttr()},
		"regex":    types.ObjectType{AttrTypes: dynamicSectionsMappingAttr()},
		"value":    types.ObjectType{AttrTypes: dynamicSectionsMappingAttr()},
	}
}

func dynamicRangeMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"min_max":        types.ObjectType{AttrTypes: dynamicMinMaxAttr()},
		"threshold_type": types.StringType,
		"thresholds": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
	}
}

func dynamicMinMaxAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"auto":   types.BoolType,
		"custom": types.ObjectType{AttrTypes: dynamicMinMaxCustomAttr()},
	}
}

func dynamicMinMaxCustomAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max": types.Float64Type,
		"min": types.Float64Type,
	}
}

func dynamicSectionsMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"sections": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicMappingSectionAttr()},
		},
	}
}

func dynamicMappingSectionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"color":  types.StringType,
		"map_to": types.StringType,
		"value":  types.StringType,
	}
}

func ExpandDynamic(ctx context.Context, dynamic *DynamicModel) (*dashboardservice.WidgetDefinition, diag.Diagnostics) {
	if dynamic == nil {
		return nil, nil
	}

	queryDefinitions, diags := expandDynamicQueryDefinitions(ctx, dynamic.QueryDefinitions)
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := ExpandTimeFrameSelect(ctx, dynamic.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	visualization, diags := expandDynamicVisualization(ctx, dynamic.Visualization)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.WidgetDefinition{
		Dynamic: &dashboardservice.WidgetsDynamic{
			QueryDefinitions: queryDefinitions,
			TimeFrame:        timeFrame,
			Visualization:    visualization,
		},
	}, nil
}

func expandDynamicQueryDefinitions(ctx context.Context, queryDefinitions types.List) ([]dashboardservice.DynamicQueryDefinition, diag.Diagnostics) {
	var queryDefinitionsObjects []types.Object
	var expandedQueryDefinitions []dashboardservice.DynamicQueryDefinition
	diags := queryDefinitions.ElementsAs(ctx, &queryDefinitionsObjects, true)
	if diags.HasError() {
		return nil, diags
	}
	for _, qdo := range queryDefinitionsObjects {
		var queryDefinition DynamicQueryDefinitionModel
		if dg := qdo.As(ctx, &queryDefinition, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedQueryDefinition, expandDiags := expandDynamicQueryDefinition(ctx, &queryDefinition)
		if expandDiags.HasError() {
			diags.Append(expandDiags...)
			continue
		}
		expandedQueryDefinitions = append(expandedQueryDefinitions, *expandedQueryDefinition)
	}

	return expandedQueryDefinitions, diags
}

func expandDynamicQueryDefinition(ctx context.Context, queryDefinition *DynamicQueryDefinitionModel) (*dashboardservice.DynamicQueryDefinition, diag.Diagnostics) {
	if queryDefinition == nil {
		return nil, nil
	}

	query, diags := expandDynamicQuery(ctx, queryDefinition.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.DynamicQueryDefinition{
		Id:    *ExpandDashboardIDs(queryDefinition.ID),
		Name:  utils.TypeStringToStringPointer(queryDefinition.Name),
		Query: query,
	}, nil
}

func expandDynamicQuery(ctx context.Context, query *DynamicQueryModel) (dashboardservice.DynamicQuery, diag.Diagnostics) {
	if query == nil {
		return dashboardservice.DynamicQuery{}, nil
	}

	switch {
	case query.Logs != nil:
		logs, diags := expandDynamicLogsQuery(ctx, query.Logs)
		if diags.HasError() {
			return dashboardservice.DynamicQuery{}, diags
		}
		return dashboardservice.DynamicQuery{Logs: logs}, nil
	case query.Spans != nil:
		spans, diags := expandDynamicSpansQuery(ctx, query.Spans)
		if diags.HasError() {
			return dashboardservice.DynamicQuery{}, diags
		}
		return dashboardservice.DynamicQuery{Spans: spans}, nil
	case query.Metrics != nil:
		metrics := expandDynamicMetricsQuery(query.Metrics)
		return dashboardservice.DynamicQuery{Metrics: metrics}, nil
	case query.DataPrime != nil:
		dataPrime := expandDynamicDataPrimeQuery(query.DataPrime)
		return dashboardservice.DynamicQuery{Dataprime: dataPrime}, nil
	default:
		return dashboardservice.DynamicQuery{}, diag.Diagnostics{diag.NewErrorDiagnostic("Error Expand Dynamic Query", "unknown dynamic query type")}
	}
}

func expandDynamicLogsQuery(ctx context.Context, logs *DynamicQueryLogsModel) (*dashboardservice.Logs, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := ExpandObservationFields(ctx, logs.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := ExpandLogsAggregations(ctx, logs.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := ExpandLogsFilters(ctx, logs.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Logs{
		LuceneQuery:  ExpandLuceneQuery(logs.LuceneQuery),
		GroupBy:      groupBy,
		Aggregation:  aggregations,
		Filters:      filters,
		DataModeType: OptionalEnumPointer(logs.DataModeType, DashboardSchemaToProtoDataModeType),
	}, nil
}

func expandDynamicSpansQuery(ctx context.Context, spans *DynamicQuerySpansModel) (*dashboardservice.Spans, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := expandSpanObservationFields(ctx, spans.GroupBy)
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := ExpandLogsAggregations(ctx, spans.Aggregations)
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := ExpandSpansFilters(ctx, spans.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Spans{
		LuceneQuery:  ExpandLuceneQuery(spans.LuceneQuery),
		GroupBy:      groupBy,
		Aggregation:  aggregations,
		Filters:      filters,
		DataModeType: OptionalEnumPointer(spans.DataModeType, DashboardSchemaToProtoDataModeType),
	}, nil
}

func expandDynamicMetricsQuery(metrics *DynamicQueryMetricsModel) *dashboardservice.Metrics {
	if metrics == nil {
		return nil
	}

	return &dashboardservice.Metrics{
		PromqlQuery:     ExpandPromqlQuery(metrics.PromqlQuery),
		PromqlQueryType: OptionalEnumPointer(metrics.PromqlQueryType, DashboardSchemaToProtoPromQLQueryType),
	}
}

func expandDynamicDataPrimeQuery(dataPrime *DynamicQueryDataPrimeModel) *dashboardservice.Dataprime {
	if dataPrime == nil {
		return nil
	}

	return &dashboardservice.Dataprime{
		DataprimeQuery: &dashboardservice.CommonDataprimeQuery{
			Text: dataPrime.Query.ValueStringPointer(),
		},
		DataModeType: OptionalEnumPointer(dataPrime.DataModeType, DashboardSchemaToProtoDataModeType),
	}
}

func expandSpanObservationFields(ctx context.Context, groupBy types.List) ([]dashboardservice.SpanObservationField, diag.Diagnostics) {
	var objects []types.Object
	diags := groupBy.ElementsAs(ctx, &objects, true)
	if diags.HasError() {
		return nil, diags
	}

	var fields []dashboardservice.SpanObservationField
	for _, obj := range objects {
		var model SpanObservationFieldModel
		if dg := obj.As(ctx, &model, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		keypath, dg := typeStringListToStringSlice(ctx, model.Keypath)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		fields = append(fields, dashboardservice.SpanObservationField{
			Keypath:      keypath,
			Scope:        OptionalEnumPointer(model.Scope, DashboardSchemaToProtoObservationFieldScope),
			RelationType: OptionalEnumPointer(model.RelationType, dashboardSchemaToProtoSpanRelationType),
		})
	}

	return fields, diags
}

func expandDynamicVisualization(ctx context.Context, visualization *DynamicVisualizationModel) (*dashboardservice.Visualization, diag.Diagnostics) {
	if visualization == nil {
		return nil, nil
	}

	switch {
	case visualization.Stat != nil:
		stat, diags := expandDynamicStat(ctx, visualization.Stat)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{Stat: stat}, nil
	case visualization.StatCard != nil:
		statCard, diags := expandDynamicStatCard(ctx, visualization.StatCard)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{StatCard: statCard}, nil
	case visualization.Table != nil:
		table, diags := expandDynamicTable(ctx, visualization.Table)
		if diags.HasError() {
			return nil, diags
		}
		return &dashboardservice.Visualization{Table: table}, nil
	default:
		return nil, nil
	}
}

func expandDynamicTable(ctx context.Context, table *DynamicTableModel) (*dashboardservice.Table, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	columns, diags := expandDynamicTableColumns(ctx, table.Columns)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := expandDynamicTableRules(ctx, table.Rules)
	if diags.HasError() {
		return nil, diags
	}

	settings, diags := expandDynamicTableSettings(ctx, table.Settings)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Table{
		Columns:  columns,
		Rules:    rules,
		Settings: settings,
	}, nil
}

func expandDynamicTableColumns(ctx context.Context, columns types.List) ([]dashboardservice.TableColumn, diag.Diagnostics) {
	var models []DynamicTableColumnModel
	diags := columns.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.TableColumn, 0, len(models))
	for i := range models {
		field, dg := ExpandObservationFieldObject(ctx, models[i].Field)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.TableColumn{Field: field})
	}

	return expanded, diags
}

func expandDynamicTableRules(ctx context.Context, rules types.List) ([]dashboardservice.TableRule, diag.Diagnostics) {
	var models []DynamicTableRuleModel
	diags := rules.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.TableRule, 0, len(models))
	for i := range models {
		properties, dg := expandDynamicTableProperties(ctx, models[i].Properties)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		ruleScope, dg := expandDynamicTableRuleScope(ctx, models[i].RuleScope)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.TableRule{
			Description: models[i].Description.ValueStringPointer(),
			Id:          ExpandDashboardUUID(models[i].ID),
			Name:        models[i].Name.ValueStringPointer(),
			Properties:  properties,
			RuleScope:   ruleScope,
		})
	}

	return expanded, diags
}

func expandDynamicTableProperties(ctx context.Context, properties types.List) ([]dashboardservice.Property, diag.Diagnostics) {
	var models []DynamicTablePropertyModel
	diags := properties.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.Property, 0, len(models))
	for i := range models {
		definition, dg := expandDynamicTablePropertyDefinition(ctx, models[i].Definition)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.Property{
			Id:         ExpandDashboardUUID(models[i].ID),
			Definition: definition,
		})
	}

	return expanded, diags
}

func expandDynamicTablePropertyDefinition(ctx context.Context, definition *DynamicTablePropertyDefinitionModel) (*dashboardservice.PropertyDefinition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	link, diags := expandDynamicTablePropertyLink(ctx, definition.Link)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandDynamicTablePropertyThresholds(ctx, definition.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	valuesMapping, diags := expandDynamicTablePropertyValuesMapping(ctx, definition.ValuesMapping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PropertyDefinition{
		Alignment:         OptionalEnumPointer(definition.Alignment, dashboardSchemaToProtoTextAlignment),
		ColumnDisplayName: definition.ColumnDisplayName.ValueStringPointer(),
		Link:              link,
		RegexExtract:      definition.RegexExtract.ValueStringPointer(),
		Thresholds:        thresholds,
		Units:             expandDynamicTablePropertyUnits(definition.Units),
		ValuesAlias:       definition.ValuesAlias.ValueStringPointer(),
		ValuesMapping:     valuesMapping,
	}, nil
}

func expandDynamicTablePropertyLink(ctx context.Context, link *DynamicTablePropertyLinkModel) (*dashboardservice.PropertyLinks, diag.Diagnostics) {
	if link == nil {
		return nil, nil
	}

	var models []DynamicTableLinkActionModel
	diags := link.Actions.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	actions := make([]dashboardservice.LinkAction, 0, len(models))
	for i := range models {
		actions = append(actions, dashboardservice.LinkAction{
			Id:                    ExpandDashboardUUID(models[i].ID),
			Name:                  models[i].Name.ValueStringPointer(),
			ShouldOpenInNewWindow: models[i].ShouldOpenInNewWindow.ValueBoolPointer(),
			Url:                   models[i].Url.ValueStringPointer(),
		})
	}

	return &dashboardservice.PropertyLinks{Actions: actions}, diags
}

func expandDynamicTablePropertyThresholds(ctx context.Context, thresholds *DynamicTablePropertyThresholdsModel) (*dashboardservice.PropertyThresholds, diag.Diagnostics) {
	if thresholds == nil {
		return nil, nil
	}

	values, diags := expandDynamicThresholds(ctx, thresholds.Values)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PropertyThresholds{
		Max:    thresholds.Max.ValueFloat64Pointer(),
		Min:    thresholds.Min.ValueFloat64Pointer(),
		Type:   OptionalEnumPointer(thresholds.Type, DashboardSchemaToProtoThresholdType),
		Values: values,
	}, nil
}

func expandDynamicTablePropertyUnits(units *DynamicTablePropertyUnitsModel) *dashboardservice.PropertyUnits {
	if units == nil {
		return nil
	}

	return &dashboardservice.PropertyUnits{
		AllowAbbreviation: units.AllowAbbreviation.ValueBoolPointer(),
		CustomUnit:        units.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(units.DecimalPrecision),
		Max:               units.Max.ValueFloat64Pointer(),
		Min:               units.Min.ValueFloat64Pointer(),
		Unit:              OptionalEnumPointer(units.Unit, DashboardSchemaToProtoUnit),
	}
}

func expandDynamicTablePropertyValuesMapping(ctx context.Context, valuesMapping *DynamicTablePropertyValuesMappingModel) (*dashboardservice.PropertyValuesMapping, diag.Diagnostics) {
	if valuesMapping == nil {
		return nil, nil
	}

	var models []DynamicTableValueMappingModel
	diags := valuesMapping.Mappings.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	mappings := make([]dashboardservice.PropertyValuesMappingValueMapping, 0, len(models))
	for i := range models {
		mappings = append(mappings, dashboardservice.PropertyValuesMappingValueMapping{
			InputValue:   models[i].InputValue.ValueStringPointer(),
			ReplaceValue: models[i].ReplaceValue.ValueStringPointer(),
			Type:         OptionalEnumPointer(models[i].Type, dashboardSchemaToProtoValuesMappingType),
		})
	}

	return &dashboardservice.PropertyValuesMapping{Mappings: mappings}, diags
}

func expandDynamicTableRuleScope(ctx context.Context, ruleScope *DynamicTableRuleScopeModel) (*dashboardservice.RuleScope, diag.Diagnostics) {
	if ruleScope == nil {
		return nil, nil
	}

	field, diags := ExpandObservationFieldObject(ctx, ruleScope.Field)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.RuleScope{
		Field:     field,
		FieldType: OptionalEnumPointer(ruleScope.FieldType, dashboardSchemaToProtoFieldDataType),
		Regex:     ruleScope.Regex.ValueStringPointer(),
	}, nil
}

func expandDynamicTableSettings(ctx context.Context, settings *DynamicTableSettingsModel) (*dashboardservice.TableSettings, diag.Diagnostics) {
	if settings == nil {
		return nil, nil
	}

	var models []DynamicTableColumnWidthModel
	diags := settings.ColumnWidths.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	columnWidths := make([]dashboardservice.ColumnWidthEntry, 0, len(models))
	for i := range models {
		columnWidths = append(columnWidths, dashboardservice.ColumnWidthEntry{
			ColumnName: models[i].ColumnName.ValueStringPointer(),
			Width:      expandInt32Pointer(models[i].Width),
		})
	}

	return &dashboardservice.TableSettings{
		ColumnWidths: columnWidths,
		RowStyle:     OptionalEnumPointer(settings.RowStyle, DashboardRowStyleSchemaToProto),
	}, diags
}

func expandDynamicStat(ctx context.Context, stat *DynamicStatModel) (*dashboardservice.Stat, diag.Diagnostics) {
	if stat == nil {
		return nil, nil
	}

	valueField, diags := ExpandObservationFieldObject(ctx, stat.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	categoryFields, diags := ExpandObservationFields(ctx, stat.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, stat.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandDynamicThresholds(ctx, stat.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, stat.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Stat{
		AllowAbbreviation: stat.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:    categoryFields,
		CustomUnit:        stat.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(stat.DecimalPrecision),
		DisplaySeriesName: stat.DisplaySeriesName.ValueBoolPointer(),
		Legend:            legend,
		LegendBy:          OptionalEnumPointer(stat.LegendBy, dashboardSchemaToProtoLegendBy),
		Max:               stat.Max.ValueFloat64Pointer(),
		Min:               stat.Min.ValueFloat64Pointer(),
		ThresholdBy:       OptionalEnumPointer(stat.ThresholdBy, dashboardSchemaToProtoThresholdBy),
		ThresholdType:     OptionalEnumPointer(stat.ThresholdType, DashboardSchemaToProtoThresholdType),
		Thresholds:        thresholds,
		Unit:              OptionalEnumPointer(stat.Unit, DashboardSchemaToProtoUnit),
		ValueField:        valueField,
		ValueFields:       valueFields,
	}, nil
}

func expandDynamicStatCard(ctx context.Context, statCard *DynamicStatCardModel) (*dashboardservice.StatCard, diag.Diagnostics) {
	if statCard == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, statCard.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, statCard.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	colorLabelMapping, diags := expandDynamicColorLabelMapping(ctx, statCard.ColorLabelMapping)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := expandDynamicStatVisualElement(ctx, statCard.Label)
	if diags.HasError() {
		return nil, diags
	}

	primaryValue, diags := expandDynamicStatVisualElement(ctx, statCard.PrimaryValue)
	if diags.HasError() {
		return nil, diags
	}

	title, diags := expandDynamicStatVisualElement(ctx, statCard.Title)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, statCard.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.StatCard{
		AllowAbbreviation: statCard.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:    categoryFields,
		ColorLabelMapping: colorLabelMapping,
		CustomUnit:        statCard.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(statCard.DecimalPrecision),
		Label:             label,
		Legend:            legend,
		LegendBy:          OptionalEnumPointer(statCard.LegendBy, dashboardSchemaToProtoLegendBy),
		PrimaryValue:      primaryValue,
		Title:             title,
		Unit:              OptionalEnumPointer(statCard.Unit, DashboardSchemaToProtoUnit),
		ValueFields:       valueFields,
	}, nil
}

func expandDynamicStatVisualElement(ctx context.Context, element *DynamicStatVisualElementModel) (*dashboardservice.StatVisualElement, diag.Diagnostics) {
	if element == nil {
		return nil, nil
	}

	observationField, diags := ExpandObservationFieldObject(ctx, element.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	mappedValues, diags := expandJSONStringToMap(element.MappedValues)
	if diags.HasError() {
		return nil, diags
	}

	templateVariables, diags := expandDynamicTemplateVariables(ctx, element.TemplateVariables)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.StatVisualElement{
		MappedValues:      mappedValues,
		ObservationField:  observationField,
		TemplateText:      element.TemplateText.ValueStringPointer(),
		TemplateVariables: templateVariables,
	}, nil
}

func expandDynamicTemplateVariables(ctx context.Context, variables types.List) ([]dashboardservice.DisplayNameTemplateVariable, diag.Diagnostics) {
	var models []DynamicTemplateVariableModel
	diags := variables.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.DisplayNameTemplateVariable, 0, len(models))
	for i := range models {
		observationField, dg := ExpandObservationFieldObject(ctx, models[i].ObservationField)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		mappedValues, dg := expandJSONStringToMap(models[i].MappedValues)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.DisplayNameTemplateVariable{
			MappedValues:     mappedValues,
			ObservationField: observationField,
		})
	}

	return expanded, diags
}

func expandDynamicColorLabelMapping(ctx context.Context, mapping *DynamicColorLabelMappingModel) (*dashboardservice.ColorLabelMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}

	rangeMapping, diags := expandDynamicRangeMapping(ctx, mapping.Range)
	if diags.HasError() {
		return nil, diags
	}

	regex, diags := expandDynamicSectionsRegex(ctx, mapping.Regex)
	if diags.HasError() {
		return nil, diags
	}

	value, diags := expandDynamicSectionsValue(ctx, mapping.Value)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.ColorLabelMapping{
		ColorBy: OptionalEnumPointer(mapping.ColorBy, dashboardSchemaToProtoColorApplyTarget),
		Range:   rangeMapping,
		Regex:   regex,
		Value:   value,
	}, nil
}

func expandDynamicRangeMapping(ctx context.Context, rangeMapping *DynamicRangeMappingModel) (*dashboardservice.RangeMapping, diag.Diagnostics) {
	if rangeMapping == nil {
		return nil, nil
	}

	thresholds, diags := expandDynamicThresholds(ctx, rangeMapping.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	var minMax *dashboardservice.MinMax
	if rangeMapping.MinMax != nil {
		minMax = &dashboardservice.MinMax{}
		if rangeMapping.MinMax.Auto.ValueBool() {
			minMax.Auto = map[string]interface{}{}
		}
		if rangeMapping.MinMax.Custom != nil {
			minMax.Custom = &dashboardservice.MinMaxCustom{
				Max: rangeMapping.MinMax.Custom.Max.ValueFloat64Pointer(),
				Min: rangeMapping.MinMax.Custom.Min.ValueFloat64Pointer(),
			}
		}
	}

	return &dashboardservice.RangeMapping{
		MinMax:        minMax,
		ThresholdType: OptionalEnumPointer(rangeMapping.ThresholdType, DashboardSchemaToProtoThresholdType),
		Thresholds:    thresholds,
	}, nil
}

func expandDynamicSectionsRegex(ctx context.Context, mapping *DynamicSectionsMappingModel) (*dashboardservice.RegexMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}
	sections, diags := expandDynamicMappingSections(ctx, mapping.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.RegexMapping{Sections: sections}, nil
}

func expandDynamicSectionsValue(ctx context.Context, mapping *DynamicSectionsMappingModel) (*dashboardservice.ColorLabelMappingValueMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}
	sections, diags := expandDynamicMappingSections(ctx, mapping.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.ColorLabelMappingValueMapping{Sections: sections}, nil
}

func expandDynamicMappingSections(ctx context.Context, sections types.List) ([]dashboardservice.MappingSection, diag.Diagnostics) {
	var models []DynamicMappingSectionModel
	diags := sections.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.MappingSection, 0, len(models))
	for i := range models {
		expanded = append(expanded, dashboardservice.MappingSection{
			Color: OptionalEnumPointer(models[i].Color, dashboardSchemaToProtoColorSolidType),
			MapTo: models[i].MapTo.ValueStringPointer(),
			Value: models[i].Value.ValueStringPointer(),
		})
	}

	return expanded, diags
}

func expandInt32Pointer(value types.Int64) *int32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := int32(value.ValueInt64())
	return &converted
}

func expandJSONStringToMap(value types.String) (map[string]interface{}, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(value.ValueString()), &parsed); err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid mapped_values JSON", err.Error())}
	}
	return parsed, nil
}

func expandDynamicThresholds(ctx context.Context, thresholds types.List) ([]dashboardservice.CommonThreshold, diag.Diagnostics) {
	var models []DynamicThresholdModel
	diags := thresholds.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.CommonThreshold, 0, len(models))
	for _, model := range models {
		expanded = append(expanded, dashboardservice.CommonThreshold{
			From:  model.From.ValueFloat64Pointer(),
			Color: model.Color.ValueStringPointer(),
			Label: model.Label.ValueStringPointer(),
		})
	}

	return expanded, diags
}

func FlattenDynamic(ctx context.Context, dynamic *dashboardservice.WidgetsDynamic) (*WidgetDefinitionModel, diag.Diagnostics) {
	if dynamic == nil {
		return nil, nil
	}

	queryDefinitions, diags := flattenDynamicQueryDefinitions(ctx, dynamic.GetQueryDefinitions())
	if diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := FlattenTimeFrameSelect(ctx, dynamic.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	visualization, diags := flattenDynamicVisualization(ctx, dynamic.Visualization)
	if diags.HasError() {
		return nil, diags
	}

	return &WidgetDefinitionModel{
		Dynamic: &DynamicModel{
			QueryDefinitions: queryDefinitions,
			TimeFrame:        timeFrame,
			Visualization:    visualization,
		},
	}, nil
}

func flattenDynamicQueryDefinitions(ctx context.Context, definitions []dashboardservice.DynamicQueryDefinition) (types.List, diag.Diagnostics) {
	if len(definitions) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	definitionsElements := make([]attr.Value, 0, len(definitions))
	for i := range definitions {
		flattenedDefinition, diags := flattenDynamicQueryDefinition(ctx, &definitions[i])
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		definitionElement, diags := types.ObjectValueFrom(ctx, dynamicQueryDefinitionModelAttr(), flattenedDefinition)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		definitionsElements = append(definitionsElements, definitionElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicQueryDefinitionModelAttr()}, definitionsElements)
}

func flattenDynamicQueryDefinition(ctx context.Context, definition *dashboardservice.DynamicQueryDefinition) (*DynamicQueryDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	query, diags := flattenDynamicQuery(ctx, &definition.Query)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicQueryDefinitionModel{
		ID:    types.StringValue(definition.GetId()),
		Name:  utils.StringPointerToTypeString(definition.Name),
		Query: query,
	}, nil
}

func flattenDynamicQuery(ctx context.Context, query *dashboardservice.DynamicQuery) (*DynamicQueryModel, diag.Diagnostics) {
	if query == nil {
		return nil, nil
	}

	switch {
	case query.Logs != nil:
		return flattenDynamicLogsQuery(ctx, query.Logs)
	case query.Spans != nil:
		return flattenDynamicSpansQuery(ctx, query.Spans)
	case query.Metrics != nil:
		return flattenDynamicMetricsQuery(query.Metrics), nil
	case query.Dataprime != nil:
		return flattenDynamicDataPrimeQuery(query.Dataprime), nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error Flatten Dynamic Query", "unknown dynamic query type")}
	}
}

func flattenDynamicLogsQuery(ctx context.Context, logs *dashboardservice.Logs) (*DynamicQueryModel, diag.Diagnostics) {
	if logs == nil {
		return nil, nil
	}

	groupBy, diags := FlattenObservationFields(ctx, logs.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := flattenAggregations(ctx, logs.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := FlattenLogsFilters(ctx, logs.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicQueryModel{
		Logs: &DynamicQueryLogsModel{
			LuceneQuery:  flattenLuceneQuery(logs.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
			DataModeType: flattenOptionalEnum(logs.DataModeType, DashboardProtoToSchemaDataModeType),
		},
	}, nil
}

func flattenDynamicSpansQuery(ctx context.Context, spans *dashboardservice.Spans) (*DynamicQueryModel, diag.Diagnostics) {
	if spans == nil {
		return nil, nil
	}

	groupBy, diags := flattenSpanObservationFields(ctx, spans.GetGroupBy())
	if diags.HasError() {
		return nil, diags
	}

	aggregations, diags := flattenAggregations(ctx, spans.GetAggregation())
	if diags.HasError() {
		return nil, diags
	}

	filters, diags := FlattenSpansFilters(ctx, spans.GetFilters())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicQueryModel{
		Spans: &DynamicQuerySpansModel{
			LuceneQuery:  flattenLuceneQuery(spans.LuceneQuery),
			GroupBy:      groupBy,
			Aggregations: aggregations,
			Filters:      filters,
			DataModeType: flattenOptionalEnum(spans.DataModeType, DashboardProtoToSchemaDataModeType),
		},
	}, nil
}

func flattenDynamicMetricsQuery(metrics *dashboardservice.Metrics) *DynamicQueryModel {
	if metrics == nil {
		return nil
	}

	return &DynamicQueryModel{
		Metrics: &DynamicQueryMetricsModel{
			PromqlQuery:     flattenPromqlQuery(metrics.PromqlQuery),
			PromqlQueryType: flattenOptionalEnum(metrics.PromqlQueryType, DashboardProtoToSchemaPromQLQueryType),
		},
	}
}

func flattenDynamicDataPrimeQuery(dataPrime *dashboardservice.Dataprime) *DynamicQueryModel {
	if dataPrime == nil {
		return nil
	}

	query := types.StringNull()
	if dataPrime.DataprimeQuery != nil && dataPrime.DataprimeQuery.Text != nil {
		query = types.StringPointerValue(dataPrime.DataprimeQuery.Text)
	}

	return &DynamicQueryModel{
		DataPrime: &DynamicQueryDataPrimeModel{
			Query:        query,
			DataModeType: flattenOptionalEnum(dataPrime.DataModeType, DashboardProtoToSchemaDataModeType),
		},
	}
}

func flattenSpanObservationFields(ctx context.Context, fields []dashboardservice.SpanObservationField) (types.List, diag.Diagnostics) {
	if len(fields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: spanObservationFieldAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	fieldElements := make([]attr.Value, 0, len(fields))
	for i := range fields {
		model := &SpanObservationFieldModel{
			Keypath:      utils.StringSliceToTypeStringList(fields[i].GetKeypath()),
			Scope:        flattenOptionalEnum(fields[i].Scope, DashboardProtoToSchemaObservationFieldScope),
			RelationType: flattenOptionalEnum(fields[i].RelationType, dashboardProtoToSchemaSpanRelationType),
		}
		fieldElement, diags := types.ObjectValueFrom(ctx, spanObservationFieldAttr(), model)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		fieldElements = append(fieldElements, fieldElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: spanObservationFieldAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: spanObservationFieldAttr()}, fieldElements)
}

func flattenDynamicVisualization(ctx context.Context, visualization *dashboardservice.Visualization) (*DynamicVisualizationModel, diag.Diagnostics) {
	if visualization == nil {
		return nil, nil
	}

	switch {
	case visualization.Stat != nil:
		stat, diags := flattenDynamicStat(ctx, visualization.Stat)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{Stat: stat}, nil
	case visualization.StatCard != nil:
		statCard, diags := flattenDynamicStatCard(ctx, visualization.StatCard)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{StatCard: statCard}, nil
	case visualization.Table != nil:
		table, diags := flattenDynamicTable(ctx, visualization.Table)
		if diags.HasError() {
			return nil, diags
		}
		return &DynamicVisualizationModel{Table: table}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Unsupported Dashboard Widget Definition",
			"The dynamic widget uses a visualization variant this provider version does not support as typed HCL yet. Only `stat`, `stat_card`, and `table` are currently supported.",
		)}
	}
}

func flattenDynamicTable(ctx context.Context, table *dashboardservice.Table) (*DynamicTableModel, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	columns, diags := flattenDynamicTableColumns(ctx, table.GetColumns())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := flattenDynamicTableRules(ctx, table.GetRules())
	if diags.HasError() {
		return nil, diags
	}

	settings, diags := flattenDynamicTableSettings(ctx, table.Settings)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTableModel{
		Columns:  columns,
		Rules:    rules,
		Settings: settings,
	}, nil
}

func flattenDynamicTableColumns(ctx context.Context, columns []dashboardservice.TableColumn) (types.List, diag.Diagnostics) {
	if len(columns) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(columns))
	for i := range columns {
		field, dg := FlattenObservationField(ctx, columns[i].Field)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableColumnModelAttr(), &DynamicTableColumnModel{Field: field})
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}, elements)
}

func flattenDynamicTableRules(ctx context.Context, rules []dashboardservice.TableRule) (types.List, diag.Diagnostics) {
	if len(rules) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(rules))
	for i := range rules {
		properties, dg := flattenDynamicTableProperties(ctx, rules[i].GetProperties())
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		ruleScope, dg := flattenDynamicTableRuleScope(ctx, rules[i].RuleScope)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTableRuleModel{
			Description: types.StringPointerValue(rules[i].Description),
			ID:          flattenDashboardUUID(rules[i].Id),
			Name:        types.StringPointerValue(rules[i].Name),
			Properties:  properties,
			RuleScope:   ruleScope,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableRuleModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}, elements)
}

func flattenDynamicTableProperties(ctx context.Context, properties []dashboardservice.Property) (types.List, diag.Diagnostics) {
	if len(properties) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(properties))
	for i := range properties {
		definition, dg := flattenDynamicTablePropertyDefinition(ctx, properties[i].Definition)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTablePropertyModel{
			ID:         flattenDashboardUUID(properties[i].Id),
			Definition: definition,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTablePropertyModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}, elements)
}

func flattenDynamicTablePropertyDefinition(ctx context.Context, definition *dashboardservice.PropertyDefinition) (*DynamicTablePropertyDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	link, diags := flattenDynamicTablePropertyLink(ctx, definition.Link)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenDynamicTablePropertyThresholds(ctx, definition.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	valuesMapping, diags := flattenDynamicTablePropertyValuesMapping(ctx, definition.ValuesMapping)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTablePropertyDefinitionModel{
		Alignment:         flattenOptionalEnum(definition.Alignment, dashboardProtoToSchemaTextAlignment),
		ColumnDisplayName: types.StringPointerValue(definition.ColumnDisplayName),
		Link:              link,
		RegexExtract:      types.StringPointerValue(definition.RegexExtract),
		Thresholds:        thresholds,
		Units:             flattenDynamicTablePropertyUnits(definition.Units),
		ValuesAlias:       types.StringPointerValue(definition.ValuesAlias),
		ValuesMapping:     valuesMapping,
	}, nil
}

func flattenDynamicTablePropertyLink(ctx context.Context, link *dashboardservice.PropertyLinks) (*DynamicTablePropertyLinkModel, diag.Diagnostics) {
	if link == nil {
		return nil, nil
	}

	actions := link.GetActions()
	if len(actions) == 0 {
		return &DynamicTablePropertyLinkModel{
			Actions: types.ListNull(types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}),
		}, nil
	}

	elements := make([]attr.Value, 0, len(actions))
	var diagnostics diag.Diagnostics
	for i := range actions {
		model := &DynamicTableLinkActionModel{
			ID:                    flattenDashboardUUID(actions[i].Id),
			Name:                  types.StringPointerValue(actions[i].Name),
			ShouldOpenInNewWindow: types.BoolPointerValue(actions[i].ShouldOpenInNewWindow),
			Url:                   types.StringPointerValue(actions[i].Url),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableLinkActionModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}, elements)
	if dg.HasError() {
		return nil, dg
	}
	return &DynamicTablePropertyLinkModel{Actions: list}, nil
}

func flattenDynamicTablePropertyThresholds(ctx context.Context, thresholds *dashboardservice.PropertyThresholds) (*DynamicTablePropertyThresholdsModel, diag.Diagnostics) {
	if thresholds == nil {
		return nil, nil
	}

	values, diags := flattenDynamicThresholds(ctx, thresholds.GetValues())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTablePropertyThresholdsModel{
		Max:    types.Float64PointerValue(thresholds.Max),
		Min:    types.Float64PointerValue(thresholds.Min),
		Type:   flattenOptionalEnum(thresholds.Type, DashboardProtoToSchemaThresholdType),
		Values: values,
	}, nil
}

func flattenDynamicTablePropertyUnits(units *dashboardservice.PropertyUnits) *DynamicTablePropertyUnitsModel {
	if units == nil {
		return nil
	}

	return &DynamicTablePropertyUnitsModel{
		AllowAbbreviation: types.BoolPointerValue(units.AllowAbbreviation),
		CustomUnit:        types.StringPointerValue(units.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(units.DecimalPrecision),
		Max:               types.Float64PointerValue(units.Max),
		Min:               types.Float64PointerValue(units.Min),
		Unit:              flattenOptionalEnum(units.Unit, DashboardProtoToSchemaUnit),
	}
}

func flattenDynamicTablePropertyValuesMapping(ctx context.Context, valuesMapping *dashboardservice.PropertyValuesMapping) (*DynamicTablePropertyValuesMappingModel, diag.Diagnostics) {
	if valuesMapping == nil {
		return nil, nil
	}

	mappings := valuesMapping.GetMappings()
	if len(mappings) == 0 {
		return &DynamicTablePropertyValuesMappingModel{
			Mappings: types.ListNull(types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}),
		}, nil
	}

	elements := make([]attr.Value, 0, len(mappings))
	var diagnostics diag.Diagnostics
	for i := range mappings {
		model := &DynamicTableValueMappingModel{
			InputValue:   types.StringPointerValue(mappings[i].InputValue),
			ReplaceValue: types.StringPointerValue(mappings[i].ReplaceValue),
			Type:         flattenOptionalEnum(mappings[i].Type, dashboardProtoToSchemaValuesMappingType),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableValueMappingModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}, elements)
	if dg.HasError() {
		return nil, dg
	}
	return &DynamicTablePropertyValuesMappingModel{Mappings: list}, nil
}

func flattenDynamicTableRuleScope(ctx context.Context, ruleScope *dashboardservice.RuleScope) (*DynamicTableRuleScopeModel, diag.Diagnostics) {
	if ruleScope == nil {
		return nil, nil
	}

	field, diags := FlattenObservationField(ctx, ruleScope.Field)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTableRuleScopeModel{
		Field:     field,
		FieldType: flattenOptionalEnum(ruleScope.FieldType, dashboardProtoToSchemaFieldDataType),
		Regex:     types.StringPointerValue(ruleScope.Regex),
	}, nil
}

func flattenDynamicTableSettings(ctx context.Context, settings *dashboardservice.TableSettings) (*DynamicTableSettingsModel, diag.Diagnostics) {
	if settings == nil {
		return nil, nil
	}

	columnWidths := settings.GetColumnWidths()
	widths := types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()})
	if len(columnWidths) > 0 {
		elements := make([]attr.Value, 0, len(columnWidths))
		var diagnostics diag.Diagnostics
		for i := range columnWidths {
			model := &DynamicTableColumnWidthModel{
				ColumnName: types.StringPointerValue(columnWidths[i].ColumnName),
				Width:      flattenInt32Pointer(columnWidths[i].Width),
			}
			element, dg := types.ObjectValueFrom(ctx, dynamicTableColumnWidthModelAttr(), model)
			if dg.HasError() {
				diagnostics.Append(dg...)
				continue
			}
			elements = append(elements, element)
		}
		if diagnostics.HasError() {
			return nil, diagnostics
		}
		list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()}, elements)
		if dg.HasError() {
			return nil, dg
		}
		widths = list
	}

	return &DynamicTableSettingsModel{
		ColumnWidths: widths,
		RowStyle:     flattenOptionalEnum(settings.RowStyle, DashboardRowStyleProtoToSchema),
	}, nil
}

func flattenDashboardUUID(id *dashboardservice.UUID) types.String {
	if id == nil {
		return types.StringNull()
	}
	return types.StringPointerValue(id.Value)
}

func flattenDynamicStat(ctx context.Context, stat *dashboardservice.Stat) (*DynamicStatModel, diag.Diagnostics) {
	if stat == nil {
		return nil, nil
	}

	valueField, diags := FlattenObservationField(ctx, stat.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	categoryFields, diags := FlattenObservationFields(ctx, stat.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, stat.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenDynamicThresholds(ctx, stat.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatModel{
		AllowAbbreviation: types.BoolPointerValue(stat.AllowAbbreviation),
		CategoryFields:    categoryFields,
		CustomUnit:        types.StringPointerValue(stat.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(stat.DecimalPrecision),
		DisplaySeriesName: types.BoolPointerValue(stat.DisplaySeriesName),
		Legend:            FlattenLegend(stat.Legend),
		LegendBy:          flattenOptionalEnum(stat.LegendBy, dashboardProtoToSchemaLegendBy),
		Max:               types.Float64PointerValue(stat.Max),
		Min:               types.Float64PointerValue(stat.Min),
		ThresholdBy:       flattenOptionalEnum(stat.ThresholdBy, dashboardProtoToSchemaThresholdBy),
		ThresholdType:     flattenOptionalEnum(stat.ThresholdType, DashboardProtoToSchemaThresholdType),
		Thresholds:        thresholds,
		Unit:              flattenOptionalEnum(stat.Unit, DashboardProtoToSchemaUnit),
		ValueField:        valueField,
		ValueFields:       valueFields,
	}, nil
}

func flattenDynamicStatCard(ctx context.Context, statCard *dashboardservice.StatCard) (*DynamicStatCardModel, diag.Diagnostics) {
	if statCard == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, statCard.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, statCard.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	colorLabelMapping, diags := flattenDynamicColorLabelMapping(ctx, statCard.ColorLabelMapping)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := flattenDynamicStatVisualElement(ctx, statCard.Label)
	if diags.HasError() {
		return nil, diags
	}

	primaryValue, diags := flattenDynamicStatVisualElement(ctx, statCard.PrimaryValue)
	if diags.HasError() {
		return nil, diags
	}

	title, diags := flattenDynamicStatVisualElement(ctx, statCard.Title)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatCardModel{
		AllowAbbreviation: types.BoolPointerValue(statCard.AllowAbbreviation),
		CategoryFields:    categoryFields,
		ColorLabelMapping: colorLabelMapping,
		CustomUnit:        types.StringPointerValue(statCard.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(statCard.DecimalPrecision),
		Label:             label,
		Legend:            FlattenLegend(statCard.Legend),
		LegendBy:          flattenOptionalEnum(statCard.LegendBy, dashboardProtoToSchemaLegendBy),
		PrimaryValue:      primaryValue,
		Title:             title,
		Unit:              flattenOptionalEnum(statCard.Unit, DashboardProtoToSchemaUnit),
		ValueFields:       valueFields,
	}, nil
}

func flattenDynamicStatVisualElement(ctx context.Context, element *dashboardservice.StatVisualElement) (*DynamicStatVisualElementModel, diag.Diagnostics) {
	if element == nil {
		return nil, nil
	}

	observationField, diags := FlattenObservationField(ctx, element.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	templateVariables, diags := flattenDynamicTemplateVariables(ctx, element.GetTemplateVariables())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatVisualElementModel{
		MappedValues:      flattenMapToJSONString(element.MappedValues),
		ObservationField:  observationField,
		TemplateText:      types.StringPointerValue(element.TemplateText),
		TemplateVariables: templateVariables,
	}, nil
}

func flattenDynamicTemplateVariables(ctx context.Context, variables []dashboardservice.DisplayNameTemplateVariable) (types.List, diag.Diagnostics) {
	if len(variables) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(variables))
	for i := range variables {
		observationField, dg := FlattenObservationField(ctx, variables[i].ObservationField)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTemplateVariableModel{
			MappedValues:     flattenMapToJSONString(variables[i].MappedValues),
			ObservationField: observationField,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTemplateVariableAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}, elements)
}

func flattenDynamicColorLabelMapping(ctx context.Context, mapping *dashboardservice.ColorLabelMapping) (*DynamicColorLabelMappingModel, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}

	rangeMapping, diags := flattenDynamicRangeMapping(ctx, mapping.Range)
	if diags.HasError() {
		return nil, diags
	}

	var regex *DynamicSectionsMappingModel
	if mapping.Regex != nil {
		sections, dg := flattenDynamicMappingSections(ctx, mapping.Regex.GetSections())
		if dg.HasError() {
			return nil, dg
		}
		regex = &DynamicSectionsMappingModel{Sections: sections}
	}

	var value *DynamicSectionsMappingModel
	if mapping.Value != nil {
		sections, dg := flattenDynamicMappingSections(ctx, mapping.Value.GetSections())
		if dg.HasError() {
			return nil, dg
		}
		value = &DynamicSectionsMappingModel{Sections: sections}
	}

	return &DynamicColorLabelMappingModel{
		ColorBy: flattenOptionalEnum(mapping.ColorBy, dashboardProtoToSchemaColorApplyTarget),
		Range:   rangeMapping,
		Regex:   regex,
		Value:   value,
	}, nil
}

func flattenDynamicRangeMapping(ctx context.Context, rangeMapping *dashboardservice.RangeMapping) (*DynamicRangeMappingModel, diag.Diagnostics) {
	if rangeMapping == nil {
		return nil, nil
	}

	thresholds, diags := flattenDynamicThresholds(ctx, rangeMapping.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	var minMax *DynamicMinMaxModel
	if rangeMapping.MinMax != nil {
		minMax = &DynamicMinMaxModel{Auto: types.BoolNull()}
		if rangeMapping.MinMax.Auto != nil {
			minMax.Auto = types.BoolValue(true)
		}
		if rangeMapping.MinMax.Custom != nil {
			minMax.Custom = &DynamicMinMaxCustomModel{
				Max: types.Float64PointerValue(rangeMapping.MinMax.Custom.Max),
				Min: types.Float64PointerValue(rangeMapping.MinMax.Custom.Min),
			}
		}
	}

	return &DynamicRangeMappingModel{
		MinMax:        minMax,
		ThresholdType: flattenOptionalEnum(rangeMapping.ThresholdType, DashboardProtoToSchemaThresholdType),
		Thresholds:    thresholds,
	}, nil
}

func flattenDynamicMappingSections(ctx context.Context, sections []dashboardservice.MappingSection) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(sections))
	for i := range sections {
		model := &DynamicMappingSectionModel{
			Color: flattenOptionalEnum(sections[i].Color, dashboardProtoToSchemaColorSolidType),
			MapTo: types.StringPointerValue(sections[i].MapTo),
			Value: types.StringPointerValue(sections[i].Value),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicMappingSectionAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}, elements)
}

func flattenInt32Pointer(value *int32) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func flattenMapToJSONString(value map[string]interface{}) types.String {
	if value == nil {
		return types.StringNull()
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return types.StringNull()
	}
	return types.StringValue(string(encoded))
}

func flattenDynamicThresholds(ctx context.Context, thresholds []dashboardservice.CommonThreshold) (types.List, diag.Diagnostics) {
	if len(thresholds) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicThresholdAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	thresholdElements := make([]attr.Value, 0, len(thresholds))
	for i := range thresholds {
		model := &DynamicThresholdModel{
			From:  types.Float64PointerValue(thresholds[i].From),
			Color: types.StringPointerValue(thresholds[i].Color),
			Label: types.StringPointerValue(thresholds[i].Label),
		}
		thresholdElement, diags := types.ObjectValueFrom(ctx, dynamicThresholdAttr(), model)
		if diags.HasError() {
			diagnostics.Append(diags...)
			continue
		}
		thresholdElements = append(thresholdElements, thresholdElement)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicThresholdAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicThresholdAttr()}, thresholdElements)
}

func flattenOptionalEnum[T ~string](value *T, mapping map[T]string) types.String {
	if value == nil {
		return types.StringNull()
	}
	if mapped, ok := mapping[*value]; ok {
		return types.StringValue(mapped)
	}
	return types.StringNull()
}
