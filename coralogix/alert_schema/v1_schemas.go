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

package alertschema

import (
	"context"
	"fmt"
	alerttypes "terraform-provider-coralogix/coralogix/alert_types"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
)

func LogsFilterSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"simple_filter": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"lucene_query": schema.StringAttribute{
						Optional: true,
					},
					"label_filters": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Default: objectdefault.StaticValue(types.ObjectValueMust(LabelFiltersAttr(), map[string]attr.Value{
							"application_name": types.SetNull(types.ObjectType{AttrTypes: LabelFilterTypesAttr()}),
							"subsystem_name":   types.SetNull(types.ObjectType{AttrTypes: LabelFilterTypesAttr()}),
							"severities":       types.SetNull(types.StringType),
						})),
						Attributes: map[string]schema.Attribute{
							"application_name": LogsAttributeFilterSchema(),
							"subsystem_name":   LogsAttributeFilterSchema(),
							"severities": schema.SetAttribute{
								Optional:    true,
								ElementType: types.StringType,
								Validators: []validator.Set{
									setvalidator.ValueStringsAre(
										stringvalidator.OneOf(alerttypes.ValidLogSeverities...),
									),
								},
								MarkdownDescription: fmt.Sprintf("Severities. Valid values: %q.", alerttypes.ValidLogSeverities),
							},
						},
					},
				},
			},
		},
	}
}

func LogsAttributeFilterSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"value": schema.StringAttribute{
					Required: true,
				},
				"operation": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Default:  stringdefault.StaticString("IS"),
					Validators: []validator.String{
						stringvalidator.OneOf(alerttypes.ValidLogFilterOperationType...),
					},
					MarkdownDescription: fmt.Sprintf("Operation. Valid values: %q.'IS' by default.", alerttypes.ValidLogFilterOperationType),
				},
			},
		},
	}
}

func NotificationPayloadFilterSchema() schema.SetAttribute {
	return schema.SetAttribute{
		Optional:    true,
		ElementType: types.StringType,
	}
}

func UndetectedValuesManagementSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"trigger_undetected_values": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"auto_retire_timeframe": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(alerttypes.AutoRetireTimeframeProtoToSchemaMap[cxsdk.AutoRetireTimeframeNeverOrUnspecified]),
				Validators: []validator.String{
					stringvalidator.OneOf(alerttypes.ValidAutoRetireTimeframes...),
				},
				MarkdownDescription: fmt.Sprintf("Auto retire timeframe. Valid values: %q.", alerttypes.ValidAutoRetireTimeframes),
			},
		},
		Default: objectdefault.StaticValue(types.ObjectValueMust(UndetectedValuesManagementAttr(), map[string]attr.Value{
			"trigger_undetected_values": types.BoolValue(false),
			"auto_retire_timeframe":     types.StringValue(alerttypes.AutoRetireTimeframeProtoToSchemaMap[cxsdk.AutoRetireTimeframeNeverOrUnspecified]),
		})),
	}
}

func EvaluationDelaySchema() schema.Attribute {
	return schema.Int32Attribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Int32{
			func() planmodifier.Int32 {
				return &evaluationDelayPlanModifier{}
			}(),
		},
	}
}

type evaluationDelayPlanModifier struct{}

func (e *evaluationDelayPlanModifier) Description(ctx context.Context) string {
	return "Sets evaluation delay to null when unspecified"
}

func (e *evaluationDelayPlanModifier) MarkdownDescription(ctx context.Context) string {
	return "Sets evaluation delay to null when unspecified"
}

func (e *evaluationDelayPlanModifier) PlanModifyInt32(ctx context.Context, req planmodifier.Int32Request, resp *planmodifier.Int32Response) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	if req.PlanValue.ValueInt32() == 0 {
		resp.PlanValue = types.Int32Null()
	}
}

func LogsTimeWindowSchema(validValues []string) schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(validValues...),
		},
		MarkdownDescription: fmt.Sprintf("Time window to evaluate the threshold with. Valid values: %q.", validValues),
	}
}

func OverrideAlertSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"priority": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(alerttypes.ValidAlertPriorities...),
				},
				MarkdownDescription: fmt.Sprintf("Alert priority. Valid values: %q.", alerttypes.ValidAlertPriorities),
			},
		},
	}
}

func MetricFilterSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"promql": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

func MetricTimeWindowSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(alerttypes.ValidMetricTimeWindowValues...),
		},
		MarkdownDescription: fmt.Sprintf("Time window to evaluate the threshold with. Valid values: %q.", alerttypes.ValidMetricTimeWindowValues),
	}
}

func AnomalyMetricTimeWindowSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Required: true,
		Validators: []validator.String{
			stringvalidator.OneOf(alerttypes.ValidMetricTimeWindowValues...),
		},
		MarkdownDescription: fmt.Sprintf("Time window to evaluate the threshold with. Valid values: %q.", alerttypes.ValidMetricTimeWindowValues),
	}
}

func TracingQuerySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"latency_threshold_ms": schema.NumberAttribute{
				Optional: true,
			},
			"tracing_label_filters": TracingLabelFiltersSchema(),
		},
	}
}

func TracingLabelFiltersSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"application_name": TracingFiltersTypeSchema(),
			"subsystem_name":   TracingFiltersTypeSchema(),
			"service_name":     TracingFiltersTypeSchema(),
			"operation_name":   TracingFiltersTypeSchema(),
			"span_fields":      TracingSpanFieldsFilterSchema(),
		},
	}
}

func TracingFiltersTypeSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: TracingFiltersTypeSchemaAttributes(),
		},
	}
}

func TracingFiltersTypeSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"operation": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(alerttypes.ValidTracingFilterOperations...),
			},
			MarkdownDescription: fmt.Sprintf("Operation. Valid values: %q.", alerttypes.ValidTracingFilterOperations),
		},
		"values": schema.SetAttribute{
			Required:    true,
			ElementType: types.StringType,
		},
	}
}

func TracingSpanFieldsFilterSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"key": schema.StringAttribute{
					Required: true,
				},
				"filter_type": schema.SingleNestedAttribute{
					Required:   true,
					Attributes: TracingFiltersTypeSchemaAttributes(),
				},
			},
		},
	}
}

func LogsRatioGroupByForSchema() schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Default:  stringdefault.StaticString("Both"),
		Validators: []validator.String{
			stringvalidator.OneOf(alerttypes.ValidLogsRatioGroupByFor...),
		},
		MarkdownDescription: fmt.Sprintf("Group by for. Valid values: %q. Both by default.", alerttypes.ValidLogsRatioGroupByFor),
	}
}

func TimeDurationAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"duration": schema.Int64Attribute{
				Required: true,
			},
			"unit": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(alerttypes.ValidDurationUnits...),
				},
				MarkdownDescription: fmt.Sprintf("Duration unit. Valid values: %q.", alerttypes.ValidDurationUnits),
			},
		},
	}
}

func SloThresholdRulesAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Required: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"condition": schema.SingleNestedAttribute{
					Required: true,
					Attributes: map[string]schema.Attribute{
						"threshold": schema.Float64Attribute{
							Required: true,
						},
					},
				},
				"override": OverrideAlertSchema(),
			},
		},
	}
}

// Custom validators and plan modifiers that are referenced but undefined
type ComputedForSomeAlerts struct{}

func (c ComputedForSomeAlerts) Description(ctx context.Context) string {
	return "Computed for some alert types"
}

func (c ComputedForSomeAlerts) MarkdownDescription(ctx context.Context) string {
	return "Computed for some alert types"
}

func (c ComputedForSomeAlerts) PlanModifyList(ctx context.Context, request planmodifier.ListRequest, response *planmodifier.ListResponse) {
	// Implementation would go here
}

type GroupByValidator struct{}

func (g GroupByValidator) Description(ctx context.Context) string {
	return "Validates group by fields"
}

func (g GroupByValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates group by fields"
}

func (g GroupByValidator) ValidateList(ctx context.Context, request validator.ListRequest, response *validator.ListResponse) {
	// Implementation would go here
}
