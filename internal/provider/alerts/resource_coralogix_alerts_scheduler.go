// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	alertscheduler "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_scheduler_rule_service"
)

var (
	_                              resource.ResourceWithConfigure   = &AlertsSchedulerResource{}
	_                              resource.ResourceWithImportState = &AlertsSchedulerResource{}
	protoToSchemaDurationFrequency                                  = map[alertscheduler.DurationFrequency]string{
		alertscheduler.DURATIONFREQUENCY_DURATION_FREQUENCY_MINUTE: "minutes",
		alertscheduler.DURATIONFREQUENCY_DURATION_FREQUENCY_HOUR:   "hours",
		alertscheduler.DURATIONFREQUENCY_DURATION_FREQUENCY_DAY:    "days",
	}
	schemaToProtoDurationFrequency = utils.ReverseMap(protoToSchemaDurationFrequency)
	validDurationFrequencies       = utils.GetKeys(schemaToProtoDurationFrequency)
	daysToProtoValue               = map[string]int32{
		"Sunday":    1,
		"Monday":    2,
		"Tuesday":   3,
		"Wednesday": 4,
		"Thursday":  5,
		"Friday":    6,
		"Saturday":  7,
	}
	protoToDaysValue               = utils.ReverseMap(daysToProtoValue)
	validDays                      = utils.GetKeys(daysToProtoValue)
	protoToSchemaScheduleOperation = map[alertscheduler.ScheduleOperation]string{
		alertscheduler.SCHEDULEOPERATION_SCHEDULE_OPERATION_ACTIVATE:    "active",
		alertscheduler.SCHEDULEOPERATION_SCHEDULE_OPERATION_UNSPECIFIED: utils.UNSPECIFIED,
		alertscheduler.SCHEDULEOPERATION_SCHEDULE_OPERATION_MUTE:        "mute",
	}
	schemaToProtoScheduleOperation = utils.ReverseMap(protoToSchemaScheduleOperation)
	validScheduleOperations        = utils.GetKeys(schemaToProtoScheduleOperation)

	validTimeZones = []string{"UTC-11", "UTC-10", "UTC-9", "UTC-8", "UTC-7", "UTC-6", "UTC-5", "UTC-4", "UTC-3", "UTC-2", "UTC-1",
		"UTC+0", "UTC+1", "UTC+2", "UTC+3", "UTC+4", "UTC+5", "UTC+6", "UTC+7", "UTC+8", "UTC+9", "UTC+10", "UTC+11", "UTC+12", "UTC+13", "UTC+14"}
)

func NewAlertsSchedulerResource() resource.Resource {
	return &AlertsSchedulerResource{}
}

type AlertsSchedulerResource struct {
	client *alertscheduler.AlertSchedulerRuleServiceAPIService
}

type AlertsSchedulerResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	MetaLabels  types.Set    `tfsdk:"meta_labels"` //MetaLabelModel
	Filter      types.Object `tfsdk:"filter"`      //FilterModel
	Schedule    types.Object `tfsdk:"schedule"`    //ScheduleModel
	Enabled     types.Bool   `tfsdk:"enabled"`
}

type MetaLabelModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type FilterModel struct {
	WhatExpression  types.String `tfsdk:"what_expression"`
	MetaLabels      types.Set    `tfsdk:"meta_labels"`       //MetaLabelModel
	AlertsUniqueIDs types.Set    `tfsdk:"alerts_unique_ids"` //types.String
}

type ScheduleModel struct {
	Operation types.String `tfsdk:"operation"`
	OneTime   types.Object `tfsdk:"one_time"`  //OneTimeModel
	Recurring types.Object `tfsdk:"recurring"` //RecurringModel
}

type OneTimeModel struct {
	TimeFrame types.Object `tfsdk:"time_frame"` //TimeFrameModel
}

type DurationModel struct {
	ForOver   types.Int64  `tfsdk:"for_over"`
	Frequency types.String `tfsdk:"frequency"`
}

type RecurringModel struct {
	Dynamic types.Object `tfsdk:"dynamic"` //DynamicModel
}

type DynamicModel struct {
	RepeatEvery    types.Int64  `tfsdk:"repeat_every"` //DurationModel
	Frequency      types.Object `tfsdk:"frequency"`    //FrequencyModel
	TimeFrame      types.Object `tfsdk:"time_frame"`   //TimeFrameModel
	TerminationDay types.String `tfsdk:"termination_date"`
}

type FrequencyModel struct {
	Daily   types.Object `tfsdk:"daily"`   //DailyModel
	Weekly  types.Object `tfsdk:"weekly"`  //WeeklyModel
	Monthly types.Object `tfsdk:"monthly"` //MonthlyModel
}

type DailyModel struct{}

type WeeklyModel struct {
	Days types.Set `tfsdk:"days"` //types.String
}

type MonthlyModel struct {
	Days types.Set `tfsdk:"days"` //types.Int64
}

type TimeFrameModel struct {
	StartTime types.String `tfsdk:"start_time"`
	EndTime   types.String `tfsdk:"end_time"`
	Duration  types.Object `tfsdk:"duration"` //DurationModel
	TimeZone  types.String `tfsdk:"time_zone"`
}

func (r *AlertsSchedulerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerts_scheduler"
}

func (r *AlertsSchedulerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.AlertSchedulers()
}

func (r *AlertsSchedulerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Alert Scheduler ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Alert Scheduler name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Alert Scheduler description.",
			},
			"meta_labels": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: metaLabelsAttributes(),
				},
				Optional:            true,
				MarkdownDescription: "Alert Scheduler meta labels.",
			},
			"filter": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"what_expression": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "DataPrime query expression. - [DataPrime query language](https://coralogix.com/docs/dataprime-query-language/).",
					},
					"meta_labels": schema.SetNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: metaLabelsAttributes(),
						},
						Optional: true,
						Validators: []validator.Set{
							setvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("alerts_unique_ids")),
						},
					},
					"alerts_unique_ids": schema.SetAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.Set{
							setvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("meta_labels")),
						},
					},
				},
				Required:            true,
				MarkdownDescription: "Alert Scheduler filter. Only one of `meta_labels` or `alerts_unique_ids` can be set. If none of them set, all alerts will be affected.",
			},
			"schedule": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"operation": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf(validScheduleOperations...),
						},
						MarkdownDescription: "The operation to perform. Can be `mute` or `active`.",
					},
					"one_time": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"time_frame": schema.SingleNestedAttribute{
								Attributes: timeFrameAttributes(),
								Required:   true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("recurring")),
						},
					},
					"recurring": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"dynamic": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"repeat_every": schema.Int64Attribute{
										Required: true,
									},
									"frequency": schema.SingleNestedAttribute{
										Attributes: map[string]schema.Attribute{
											"daily": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{},
												Optional:   true,
											},
											"weekly": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{
													"days": schema.SetAttribute{
														ElementType: types.StringType,
														Optional:    true,
														Validators: []validator.Set{
															setvalidator.ValueStringsAre(
																stringvalidator.OneOf(validDays...),
															),
														},
													},
												},
												Optional: true,
											},
											"monthly": schema.SingleNestedAttribute{
												Attributes: map[string]schema.Attribute{
													"days": schema.SetAttribute{
														ElementType: types.Int64Type,
														Optional:    true,
													},
												},
												Optional: true,
											},
										},
										Required: true,
									},
									"time_frame": schema.SingleNestedAttribute{
										Attributes: timeFrameAttributes(),
										Required:   true,
									},
									"termination_date": schema.StringAttribute{
										Optional: true,
										Computed: true,
										Default:  stringdefault.StaticString(""),
									},
								},
								Optional: true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("one_time")),
						},
					},
				},
				Required:            true,
				MarkdownDescription: "Exactly one of `one_time` or `recurring` must be set.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Alert Scheduler enabled. If set to `false`, the alert scheduler will be disabled. True by default.",
			},
		},
		MarkdownDescription: "Coralogix alerts-scheduler.",
	}
}

func metaLabelsAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key": schema.StringAttribute{
			Required: true,
		},
		"value": schema.StringAttribute{
			Optional: true,
		},
	}
}

func timeFrameAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"start_time": schema.StringAttribute{
			Required: true,
		},
		"end_time": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("duration")),
			},
			MarkdownDescription: "The end time of the time frame. In a isodate format. For example, `2021-01-01T00:00:00.000`.",
		},
		"duration": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"for_over": schema.Int64Attribute{
					Required:            true,
					MarkdownDescription: "The number of time units to wait before the alert is triggered. For example, if the frequency is set to `hours` and the value is set to `2`, the alert will be triggered after 2 hours.",
				},
				"frequency": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.OneOf(validDurationFrequencies...),
					},
					MarkdownDescription: "The time unit to wait before the alert is triggered. Can be `minutes`, `hours` or `days`.",
				},
			},
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("end_time")),
			},
			MarkdownDescription: "The duration from the start time to wait.",
		},
		"time_zone": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf(validTimeZones...),
			},
		},
	}
}

func (r *AlertsSchedulerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AlertsSchedulerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *AlertsSchedulerResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertSchedulerRule, diags := extractAlertsScheduler(ctx, plan, nil)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	createRequest := alertscheduler.CreateAlertSchedulerRuleRequestDataStructure{
		AlertSchedulerRule: alertSchedulerRule,
	}
	log.Printf("[INFO] Creating new alerts-scheduler: %s", utils.FormatJSON(createRequest))
	createResp, httpResp, err := r.client.
		AlertSchedulerRuleServiceCreateAlertSchedulerRule(ctx).
		CreateAlertSchedulerRuleRequestDataStructure(createRequest).
		Execute()
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error creating alerts-scheduler",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Create", createRequest))
		return
	}
	alertSchedulerRule = createResp.AlertSchedulerRule
	log.Printf("[INFO] Submitted new alerts-scheduler: %s", utils.FormatJSON(alertSchedulerRule))

	plan, diags = flattenAlertScheduler(ctx, alertSchedulerRule)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenAlertScheduler(ctx context.Context, scheduler alertscheduler.AlertSchedulerRule) (*AlertsSchedulerResourceModel, diag.Diagnostics) {
	metaLabels, diags := flattenAlertsSchedulerMetaLabels(ctx, scheduler.GetMetaLabels())
	if diags.HasError() {
		return nil, diags
	}

	filter, diags := flattenFilter(ctx, scheduler.Filter)
	if diags.HasError() {
		return nil, diags
	}

	schedule, diags := flattenSchedule(ctx, scheduler.Schedule)
	if diags.HasError() {
		return nil, diags
	}

	return &AlertsSchedulerResourceModel{
		ID:          types.StringValue(scheduler.GetUniqueIdentifier()),
		Name:        types.StringValue(scheduler.GetName()),
		Description: types.StringValue(scheduler.GetDescription()),
		MetaLabels:  metaLabels,
		Filter:      filter,
		Schedule:    schedule,
		Enabled:     types.BoolValue(scheduler.GetEnabled()),
	}, nil
}

func flattenAlertsSchedulerMetaLabels(ctx context.Context, labels []alertscheduler.MetaLabelsProtobufV1MetaLabel) (types.Set, diag.Diagnostics) {
	if len(labels) == 0 {
		return types.SetNull(types.ObjectType{AttrTypes: labelModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	labelsElements := make([]attr.Value, 0, len(labels))
	for _, label := range labels {
		flattenedLabel := MetaLabelModel{
			Key:   types.StringValue(label.GetKey()),
			Value: utils.StringPointerToTypeString(label.Value),
		}
		labelElement, diags := types.ObjectValueFrom(ctx, labelModelAttr(), flattenedLabel)
		if diags.HasError() {
			diagnostics = append(diagnostics, diags...)
			continue
		}
		labelsElements = append(labelsElements, labelElement)
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: labelModelAttr()}, labelsElements)
}

func labelModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"key":   types.StringType,
		"value": types.StringType,
	}
}

func flattenFilter(ctx context.Context, filter *alertscheduler.AlertSchedulerRuleProtobufV1Filter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(filterModelAttr()), nil
	}

	var filterModel FilterModel
	if filter.AlertSchedulerRuleProtobufV1FilterAlertMetaLabels != nil {
		metaLabels, diags := flattenAlertsSchedulerMetaLabels(ctx, filter.AlertSchedulerRuleProtobufV1FilterAlertMetaLabels.AlertMetaLabels.GetValue())
		if diags.HasError() {
			return types.ObjectNull(filterModelAttr()), diags
		}
		filterModel.MetaLabels = metaLabels
		filterModel.AlertsUniqueIDs = types.SetNull(types.StringType)
		filterModel.WhatExpression = types.StringValue(filter.AlertSchedulerRuleProtobufV1FilterAlertMetaLabels.GetWhatExpression())
	} else if filter.AlertSchedulerRuleProtobufV1FilterAlertUniqueIds != nil {
		filterModel.AlertsUniqueIDs = utils.StringSliceToTypeStringSet(filter.AlertSchedulerRuleProtobufV1FilterAlertUniqueIds.AlertUniqueIds.GetValue())
		filterModel.MetaLabels = types.SetNull(types.ObjectType{AttrTypes: labelModelAttr()})
		filterModel.WhatExpression = types.StringValue(filter.AlertSchedulerRuleProtobufV1FilterAlertUniqueIds.GetWhatExpression())
	} else {
		return types.ObjectNull(filterModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten filter", "unknown filter type")}
	}

	return types.ObjectValueFrom(ctx, filterModelAttr(), filterModel)
}

func filterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"what_expression": types.StringType,
		"meta_labels": types.SetType{
			ElemType: types.ObjectType{AttrTypes: labelModelAttr()},
		},
		"alerts_unique_ids": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func flattenSchedule(ctx context.Context, schedule *alertscheduler.Schedule) (types.Object, diag.Diagnostics) {
	if schedule == nil {
		return types.ObjectNull(scheduleModelAttr()), nil
	}

	var scheduleModel ScheduleModel
	if schedule.ScheduleOneTime != nil {
		scheduleModel.Operation = types.StringValue(protoToSchemaScheduleOperation[schedule.ScheduleOneTime.GetScheduleOperation()])
		oneTime, diags := flattenOneTime(ctx, schedule.ScheduleOneTime.OneTime)
		if diags.HasError() {
			return types.ObjectNull(scheduleModelAttr()), diags
		}
		scheduleModel.OneTime = oneTime
		scheduleModel.Recurring = types.ObjectNull(recurringModelAttr())
	} else if schedule.ScheduleRecurring != nil {
		scheduleModel.Operation = types.StringValue(protoToSchemaScheduleOperation[schedule.ScheduleRecurring.GetScheduleOperation()])
		recurring, diags := flattenRecurring(ctx, schedule.ScheduleRecurring.Recurring)
		if diags.HasError() {
			return types.ObjectNull(scheduleModelAttr()), diags
		}
		scheduleModel.Recurring = recurring
		scheduleModel.OneTime = types.ObjectNull(oneTimeModelAttr())
	} else {
		return types.ObjectNull(scheduleModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten schedule", "unknown schedule type")}
	}

	return types.ObjectValueFrom(ctx, scheduleModelAttr(), scheduleModel)
}

func flattenRecurring(ctx context.Context, recurring *alertscheduler.Recurring) (types.Object, diag.Diagnostics) {
	if recurring == nil {
		return types.ObjectNull(recurringModelAttr()), nil
	}

	var recurringModel RecurringModel
	if recurring.RecurringSchedule != nil {
		dynamic, diags := flattenDynamic(ctx, recurring.RecurringSchedule.Schedule)
		if diags.HasError() {
			return types.ObjectNull(recurringModelAttr()), diags
		}
		recurringModel.Dynamic = dynamic
	} else if recurring.RecurringAlwaysActive != nil {
		recurringModel.Dynamic = types.ObjectNull(dynamicModelAttr())
	} else {
		return types.ObjectNull(recurringModelAttr()), nil
	}

	return types.ObjectValueFrom(ctx, recurringModelAttr(), recurringModel)
}

func flattenDynamic(ctx context.Context, dynamic *alertscheduler.RecurringDynamic) (types.Object, diag.Diagnostics) {
	if dynamic == nil {
		return types.ObjectNull(dynamicModelAttr()), nil
	}

	frequency, diags := flattenFrequency(ctx, dynamic)
	if diags.HasError() {
		return types.ObjectNull(dynamicModelAttr()), diags
	}

	var timeFrame types.Object
	var repeatEvery int32
	var terminationDate string

	if dynamic.RecurringDynamicDaily != nil {
		timeFrame, diags = flattenAlertsSchedulerTimeFrame(ctx, dynamic.RecurringDynamicDaily.Timeframe)
		if diags.HasError() {
			return types.ObjectNull(dynamicModelAttr()), diags
		}
		repeatEvery = dynamic.RecurringDynamicDaily.GetRepeatEvery()
		terminationDate = dynamic.RecurringDynamicDaily.GetTerminationDate()
	} else if dynamic.RecurringDynamicWeekly != nil {
		timeFrame, diags = flattenAlertsSchedulerTimeFrame(ctx, dynamic.RecurringDynamicWeekly.Timeframe)
		if diags.HasError() {
			return types.ObjectNull(dynamicModelAttr()), diags
		}
		repeatEvery = dynamic.RecurringDynamicWeekly.GetRepeatEvery()
		terminationDate = dynamic.RecurringDynamicWeekly.GetTerminationDate()
	} else if dynamic.RecurringDynamicMonthly != nil {
		timeFrame, diags = flattenAlertsSchedulerTimeFrame(ctx, dynamic.RecurringDynamicMonthly.Timeframe)
		if diags.HasError() {
			return types.ObjectNull(dynamicModelAttr()), diags
		}
		repeatEvery = dynamic.RecurringDynamicMonthly.GetRepeatEvery()
		terminationDate = dynamic.RecurringDynamicMonthly.GetTerminationDate()
	} else {
		return types.ObjectNull(dynamicModelAttr()), nil
	}

	dynamicModel := DynamicModel{
		RepeatEvery:    types.Int64Value(int64(repeatEvery)),
		Frequency:      frequency,
		TimeFrame:      timeFrame,
		TerminationDay: types.StringValue(terminationDate),
	}

	return types.ObjectValueFrom(ctx, dynamicModelAttr(), dynamicModel)
}

func flattenFrequency(ctx context.Context, dynamic *alertscheduler.RecurringDynamic) (types.Object, diag.Diagnostics) {
	if dynamic == nil {
		return types.ObjectNull(frequencyModelAttr()), nil
	}

	var frequencyModel FrequencyModel
	if dynamic.RecurringDynamicDaily != nil {
		frequencyModel.Daily = types.ObjectNull(map[string]attr.Type{})
		frequencyModel.Weekly = types.ObjectNull(weeklyModelAttr())
		frequencyModel.Monthly = types.ObjectNull(monthlyModelAttr())
	} else if dynamic.RecurringDynamicWeekly != nil {
		weekly, diags := flattenWeekly(ctx, dynamic.RecurringDynamicWeekly.Weekly)
		if diags.HasError() {
			return types.ObjectNull(frequencyModelAttr()), diags
		}
		frequencyModel.Weekly = weekly
		frequencyModel.Daily = types.ObjectNull(map[string]attr.Type{})
		frequencyModel.Monthly = types.ObjectNull(monthlyModelAttr())
	} else if dynamic.RecurringDynamicMonthly != nil {
		monthly, diags := flattenMonthly(ctx, dynamic.RecurringDynamicMonthly.Monthly)
		if diags.HasError() {
			return types.ObjectNull(frequencyModelAttr()), diags
		}
		frequencyModel.Monthly = monthly
		frequencyModel.Daily = types.ObjectNull(map[string]attr.Type{})
		frequencyModel.Weekly = types.ObjectNull(weeklyModelAttr())
	} else {
		return types.ObjectNull(frequencyModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten frequency", "unknown frequency type")}
	}

	return types.ObjectValueFrom(ctx, frequencyModelAttr(), frequencyModel)
}

func flattenWeekly(ctx context.Context, weekly *alertscheduler.Weekly) (types.Object, diag.Diagnostics) {
	if weekly == nil {
		return types.ObjectNull(weeklyModelAttr()), nil
	}

	daysOfWeek := make([]string, 0, len(weekly.GetDaysOfWeek()))
	for _, day := range weekly.GetDaysOfWeek() {
		daysOfWeek = append(daysOfWeek, protoToDaysValue[day])
	}
	weeklyModel := WeeklyModel{
		Days: utils.StringSliceToTypeStringSet(daysOfWeek),
	}

	return types.ObjectValueFrom(ctx, weeklyModelAttr(), weeklyModel)

}

func flattenMonthly(ctx context.Context, monthly *alertscheduler.Monthly) (types.Object, diag.Diagnostics) {
	if monthly == nil {
		return types.ObjectNull(monthlyModelAttr()), nil
	}

	monthlyModel := MonthlyModel{
		Days: utils.Int32SliceToTypeInt64Set(monthly.GetDaysOfMonth()),
	}

	return types.ObjectValueFrom(ctx, monthlyModelAttr(), monthlyModel)
}

func flattenOneTime(ctx context.Context, time *alertscheduler.OneTime) (types.Object, diag.Diagnostics) {
	if time == nil {
		return types.ObjectNull(oneTimeModelAttr()), nil
	}

	timeFrame, diags := flattenAlertsSchedulerTimeFrame(ctx, time.Timeframe)
	if diags.HasError() {
		return types.ObjectNull(oneTimeModelAttr()), diags
	}

	oneTimeModel := OneTimeModel{
		TimeFrame: timeFrame,
	}

	return types.ObjectValueFrom(ctx, oneTimeModelAttr(), oneTimeModel)
}

func flattenAlertsSchedulerTimeFrame(ctx context.Context, timeFrame *alertscheduler.Timeframe) (types.Object, diag.Diagnostics) {
	if timeFrame == nil {
		return types.ObjectNull(timeFrameModelAttr()), nil
	}

	var timeFrameModel TimeFrameModel
	if timeFrame.TimeframeEndTime != nil {
		timeFrameModel.StartTime = types.StringValue(timeFrame.TimeframeEndTime.GetStartTime())
		timeFrameModel.TimeZone = types.StringValue(timeFrame.TimeframeEndTime.GetTimezone())
		timeFrameModel.EndTime = types.StringValue(timeFrame.TimeframeEndTime.GetEndTime())
		timeFrameModel.Duration = types.ObjectNull(durationModelAttr())
	} else if timeFrame.TimeframeDuration != nil {
		timeFrameModel.StartTime = types.StringValue(timeFrame.TimeframeDuration.GetStartTime())
		timeFrameModel.TimeZone = types.StringValue(timeFrame.TimeframeDuration.GetTimezone())
		var diags diag.Diagnostics
		timeFrameModel.Duration, diags = flattenAlertsSchedulerDuration(ctx, timeFrame.TimeframeDuration.Duration)
		if diags.HasError() {
			return types.ObjectNull(timeFrameModelAttr()), diags
		}
		timeFrameModel.EndTime = types.StringNull()
	} else {
		return types.ObjectNull(timeFrameModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten time frame", "unknown timeframe type")}
	}

	return types.ObjectValueFrom(ctx, timeFrameModelAttr(), timeFrameModel)
}

func flattenAlertsSchedulerDuration(ctx context.Context, duration *alertscheduler.V1Duration) (types.Object, diag.Diagnostics) {
	if duration == nil {
		return types.ObjectNull(durationModelAttr()), nil
	}

	durationModel := DurationModel{
		ForOver:   types.Int64Value(int64(duration.GetForOver())),
		Frequency: types.StringValue(protoToSchemaDurationFrequency[duration.GetFrequency()]),
	}

	return types.ObjectValueFrom(ctx, durationModelAttr(), durationModel)
}

func scheduleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"operation": types.StringType,
		"one_time": types.ObjectType{
			AttrTypes: oneTimeModelAttr(),
		},
		"recurring": types.ObjectType{
			AttrTypes: recurringModelAttr(),
		},
	}
}

func oneTimeModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"time_frame": types.ObjectType{
			AttrTypes: timeFrameModelAttr(),
		},
	}
}

func durationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"for_over":  types.Int64Type,
		"frequency": types.StringType,
	}
}

func recurringModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"dynamic": types.ObjectType{
			AttrTypes: dynamicModelAttr(),
		},
	}
}

func dynamicModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"repeat_every": types.Int64Type,
		"frequency": types.ObjectType{
			AttrTypes: frequencyModelAttr(),
		},
		"time_frame": types.ObjectType{
			AttrTypes: timeFrameModelAttr(),
		},
		"termination_date": types.StringType,
	}
}

func frequencyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"daily": types.ObjectType{
			AttrTypes: map[string]attr.Type{},
		},
		"weekly": types.ObjectType{
			AttrTypes: weeklyModelAttr(),
		},
		"monthly": types.ObjectType{
			AttrTypes: monthlyModelAttr(),
		},
	}
}

func weeklyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"days": types.SetType{
			ElemType: types.StringType,
		},
	}
}

func monthlyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"days": types.SetType{
			ElemType: types.Int64Type,
		},
	}
}

func timeFrameModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"start_time": types.StringType,
		"end_time":   types.StringType,
		"duration": types.ObjectType{
			AttrTypes: durationModelAttr(),
		},
		"time_zone": types.StringType,
	}
}

func extractAlertsScheduler(ctx context.Context, plan *AlertsSchedulerResourceModel, id *string) (alertscheduler.AlertSchedulerRule, diag.Diagnostics) {
	metaLabels, diags := extractAlertsSchedulerMetaLabels(ctx, plan.MetaLabels)
	if diags.HasError() {
		return alertscheduler.AlertSchedulerRule{}, diags
	}

	filter, diags := extractFilter(ctx, plan.Filter)
	if diags.HasError() {
		return alertscheduler.AlertSchedulerRule{}, diags
	}

	schedule, diags := extractSchedule(ctx, plan.Schedule)
	if diags.HasError() {
		return alertscheduler.AlertSchedulerRule{}, diags
	}

	return alertscheduler.AlertSchedulerRule{
		UniqueIdentifier: id,
		Name:             alertscheduler.PtrString(plan.Name.ValueString()),
		Description:      utils.TypeStringToStringPointer(plan.Description),
		MetaLabels:       metaLabels,
		Filter:           filter,
		Schedule:         schedule,
		Enabled:          alertscheduler.PtrBool(plan.Enabled.ValueBool()),
	}, nil
}

func extractAlertsSchedulerMetaLabels(ctx context.Context, labels types.Set) ([]alertscheduler.MetaLabelsProtobufV1MetaLabel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var labelsObjects []types.Object
	var expandedLabels []alertscheduler.MetaLabelsProtobufV1MetaLabel
	labels.ElementsAs(ctx, &labelsObjects, true)

	for _, lo := range labelsObjects {
		var label MetaLabelModel
		if dg := lo.As(ctx, &label, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLabel := alertscheduler.MetaLabelsProtobufV1MetaLabel{
			Key:   utils.TypeStringToStringPointer(label.Key),
			Value: utils.TypeStringToStringPointer(label.Value),
		}
		expandedLabels = append(expandedLabels, expandedLabel)
	}

	return expandedLabels, diags
}

func extractFilter(ctx context.Context, filter types.Object) (*alertscheduler.AlertSchedulerRuleProtobufV1Filter, diag.Diagnostics) {
	if filter.IsNull() || filter.IsUnknown() {
		return nil, nil
	}

	var filterModel FilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	whatExpression := filterModel.WhatExpression.ValueString()

	if !(filterModel.AlertsUniqueIDs.IsNull() || filterModel.AlertsUniqueIDs.IsUnknown()) {
		ids, diags := utils.TypeStringElementsToStringSlice(ctx, filterModel.AlertsUniqueIDs.Elements())
		if diags.HasError() {
			return nil, diags
		}
		return &alertscheduler.AlertSchedulerRuleProtobufV1Filter{
			AlertSchedulerRuleProtobufV1FilterAlertUniqueIds: &alertscheduler.AlertSchedulerRuleProtobufV1FilterAlertUniqueIds{
				WhatExpression: alertscheduler.PtrString(whatExpression),
				AlertUniqueIds: &alertscheduler.AlertUniqueIds{
					Value: ids,
				},
			},
		}, nil
	} else if !(filterModel.MetaLabels.IsNull() || filterModel.MetaLabels.IsUnknown()) {
		metaLabels, diags := extractAlertsSchedulerMetaLabels(ctx, filterModel.MetaLabels)
		if diags.HasError() {
			return nil, diags
		}
		return &alertscheduler.AlertSchedulerRuleProtobufV1Filter{
			AlertSchedulerRuleProtobufV1FilterAlertMetaLabels: &alertscheduler.AlertSchedulerRuleProtobufV1FilterAlertMetaLabels{
				WhatExpression: alertscheduler.PtrString(whatExpression),
				AlertMetaLabels: &alertscheduler.MetaLabels{
					Value: metaLabels,
				},
			},
		}, nil
	}

	return &alertscheduler.AlertSchedulerRuleProtobufV1Filter{
		AlertSchedulerRuleProtobufV1FilterAlertUniqueIds: &alertscheduler.AlertSchedulerRuleProtobufV1FilterAlertUniqueIds{
			WhatExpression: alertscheduler.PtrString(whatExpression),
			AlertUniqueIds: &alertscheduler.AlertUniqueIds{
				Value: nil,
			},
		},
	}, nil
}

func extractSchedule(ctx context.Context, schedule types.Object) (*alertscheduler.Schedule, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(schedule) {
		return nil, nil
	}

	var scheduleModel ScheduleModel
	if diags := schedule.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	operation := schemaToProtoScheduleOperation[scheduleModel.Operation.ValueString()]

	if !(scheduleModel.OneTime.IsNull() || scheduleModel.OneTime.IsUnknown()) {
		oneTime, diags := extractOneTime(ctx, scheduleModel.OneTime)
		if diags.HasError() {
			return nil, diags
		}
		return &alertscheduler.Schedule{
			ScheduleOneTime: &alertscheduler.ScheduleOneTime{
				OneTime:           oneTime,
				ScheduleOperation: &operation,
			},
		}, nil
	} else if !(scheduleModel.Recurring.IsNull() || scheduleModel.Recurring.IsUnknown()) {
		recurring, diags := extractRecurring(ctx, scheduleModel.Recurring)
		if diags.HasError() {
			return nil, diags
		}
		return &alertscheduler.Schedule{
			ScheduleRecurring: &alertscheduler.ScheduleRecurring{
				Recurring:         recurring,
				ScheduleOperation: &operation,
			},
		}, nil
	}

	return nil, nil
}

func extractOneTime(ctx context.Context, oneTimeObject types.Object) (*alertscheduler.OneTime, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(oneTimeObject) {
		return nil, nil
	}

	var oneTimeModel OneTimeModel
	if diags := oneTimeObject.As(ctx, &oneTimeModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := extractTimeFrame(ctx, oneTimeModel.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return &alertscheduler.OneTime{
		Timeframe: timeFrame,
	}, nil
}

func extractTimeFrame(ctx context.Context, timeFrame types.Object) (*alertscheduler.Timeframe, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(timeFrame) {
		return nil, nil
	}

	var timeFrameModel TimeFrameModel
	if diags := timeFrame.As(ctx, &timeFrameModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	startTime := timeFrameModel.StartTime.ValueString()
	timezone := timeFrameModel.TimeZone.ValueString()

	if !(timeFrameModel.Duration.IsNull() || timeFrameModel.Duration.IsUnknown()) {
		duration, diags := extractDuration(ctx, timeFrameModel.Duration)
		if diags.HasError() {
			return nil, diags
		}
		return &alertscheduler.Timeframe{
			TimeframeDuration: &alertscheduler.TimeframeDuration{
				StartTime: alertscheduler.PtrString(startTime),
				Timezone:  alertscheduler.PtrString(timezone),
				Duration:  duration,
			},
		}, nil
	} else if !(timeFrameModel.EndTime.IsNull() || timeFrameModel.EndTime.IsUnknown()) {
		return &alertscheduler.Timeframe{
			TimeframeEndTime: &alertscheduler.TimeframeEndTime{
				StartTime: alertscheduler.PtrString(startTime),
				Timezone:  alertscheduler.PtrString(timezone),
				EndTime:   alertscheduler.PtrString(timeFrameModel.EndTime.ValueString()),
			},
		}, nil
	}

	return nil, nil
}

func extractDuration(ctx context.Context, duration types.Object) (*alertscheduler.V1Duration, diag.Diagnostics) {
	if duration.IsNull() || duration.IsUnknown() {
		return nil, nil
	}
	var durationModel DurationModel
	if diags := duration.As(ctx, &durationModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	freq := schemaToProtoDurationFrequency[durationModel.Frequency.ValueString()]
	return &alertscheduler.V1Duration{
		ForOver:   alertscheduler.PtrInt32(int32(durationModel.ForOver.ValueInt64())),
		Frequency: &freq,
	}, nil
}

func extractRecurring(ctx context.Context, recurring types.Object) (*alertscheduler.Recurring, diag.Diagnostics) {
	if recurring.IsNull() || recurring.IsUnknown() {
		return nil, nil
	}

	var recurringModel RecurringModel
	if diags := recurring.As(ctx, &recurringModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if !(recurringModel.Dynamic.IsNull() || recurringModel.Dynamic.IsUnknown()) {
		dynamic, diags := extractDynamic(ctx, recurringModel.Dynamic)
		if diags.HasError() {
			return nil, diags
		}
		return &alertscheduler.Recurring{
			RecurringSchedule: &alertscheduler.RecurringSchedule{
				Schedule: dynamic,
			},
		}, nil
	}

	return &alertscheduler.Recurring{
		RecurringAlwaysActive: &alertscheduler.RecurringAlwaysActive{},
	}, nil
}

func extractDynamic(ctx context.Context, dynamic types.Object) (*alertscheduler.RecurringDynamic, diag.Diagnostics) {
	if dynamic.IsNull() || dynamic.IsUnknown() {
		return nil, nil
	}

	var dynamicModel DynamicModel
	if diags := dynamic.As(ctx, &dynamicModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	timeFrame, diags := extractTimeFrame(ctx, dynamicModel.TimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	repeatEvery := int32(dynamicModel.RepeatEvery.ValueInt64())
	terminationDate := utils.TypeStringToStringPointer(dynamicModel.TerminationDay)

	return expandFrequency(ctx, dynamicModel.Frequency, timeFrame, repeatEvery, terminationDate)
}

func expandFrequency(ctx context.Context, frequency types.Object, timeFrame *alertscheduler.Timeframe, repeatEvery int32, terminationDate *string) (*alertscheduler.RecurringDynamic, diag.Diagnostics) {
	var frequencyModel FrequencyModel
	if diags := frequency.As(ctx, &frequencyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if daily := frequencyModel.Daily; !(daily.IsNull() || daily.IsUnknown()) {
		return &alertscheduler.RecurringDynamic{
			RecurringDynamicDaily: &alertscheduler.RecurringDynamicDaily{
				Daily:           make(map[string]interface{}),
				Timeframe:       timeFrame,
				RepeatEvery:     &repeatEvery,
				TerminationDate: terminationDate,
			},
		}, nil
	} else if weekly := frequencyModel.Weekly; !(weekly.IsNull() || weekly.IsUnknown()) {
		var weeklyModel WeeklyModel
		if diags := weekly.As(ctx, &weeklyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		days, diags := utils.TypeStringElementsToStringSlice(ctx, weeklyModel.Days.Elements())
		if diags.HasError() {
			return nil, diags
		}
		daysValues := make([]int32, len(days))
		for i, day := range days {
			daysValues[i] = daysToProtoValue[day]
		}

		return &alertscheduler.RecurringDynamic{
			RecurringDynamicWeekly: &alertscheduler.RecurringDynamicWeekly{
				Weekly: &alertscheduler.Weekly{
					DaysOfWeek: daysValues,
				},
				Timeframe:       timeFrame,
				RepeatEvery:     &repeatEvery,
				TerminationDate: terminationDate,
			},
		}, nil
	} else if monthly := frequencyModel.Monthly; !(monthly.IsNull() || monthly.IsUnknown()) {
		var monthlyModel MonthlyModel
		if diags := monthly.As(ctx, &monthlyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		days, diags := utils.TypeInt64SliceToInt32Slice(ctx, monthlyModel.Days.Elements())
		if diags.HasError() {
			return nil, diags
		}

		return &alertscheduler.RecurringDynamic{
			RecurringDynamicMonthly: &alertscheduler.RecurringDynamicMonthly{
				Monthly: &alertscheduler.Monthly{
					DaysOfMonth: days,
				},
				Timeframe:       timeFrame,
				RepeatEvery:     &repeatEvery,
				TerminationDate: terminationDate,
			},
		}, nil
	}

	return nil, nil
}

func (r *AlertsSchedulerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *AlertsSchedulerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Reading alerts-scheduler: %s", id)
	getAlertsSchedulerResp, httpResp, err := r.client.
		AlertSchedulerRuleServiceGetAlertSchedulerRule(ctx, id).
		Execute()
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("alerts-scheduler %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading alerts-scheduler",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Read", id),
			)
		}
		return
	}
	alertsScheduler := getAlertsSchedulerResp.AlertSchedulerRule
	log.Printf("[INFO] Received alerts-scheduler: %s", utils.FormatJSON(alertsScheduler))

	state, diags = flattenAlertScheduler(ctx, alertsScheduler)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *AlertsSchedulerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *AlertsSchedulerResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state *AlertsSchedulerResourceModel
	req.State.Get(ctx, &state)
	id := state.ID.ValueString()
	alertsScheduler, diags := extractAlertsScheduler(ctx, plan, &id)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	updateRequest := alertscheduler.UpdateAlertSchedulerRuleRequestDataStructure{
		AlertSchedulerRule: alertsScheduler,
	}
	log.Printf("[INFO] Updating alerts-scheduler: %s", utils.FormatJSON(updateRequest))
	updateResp, httpResp, err := r.client.
		AlertSchedulerRuleServiceUpdateAlertSchedulerRule(ctx).
		UpdateAlertSchedulerRuleRequestDataStructure(updateRequest).
		Execute()
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating alerts-scheduler",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Update", updateRequest),
		)
		return
	}
	log.Printf("[INFO] Submitted updated alerts-scheduler: %s", utils.FormatJSON(updateResp))

	getAlertsSchedulerResp, httpResp, err := r.client.
		AlertSchedulerRuleServiceGetAlertSchedulerRule(ctx, id).
		Execute()
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("alerts-scheduler %s is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading alerts-scheduler",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Read", id),
			)
		}
		return
	}
	log.Printf("[INFO] Received alerts-scheduler: %s", utils.FormatJSON(getAlertsSchedulerResp))

	plan, diags = flattenAlertScheduler(ctx, getAlertsSchedulerResp.AlertSchedulerRule)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *AlertsSchedulerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *AlertsSchedulerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting alerts-scheduler %s", id)
	_, httpResp, err := r.client.
		AlertSchedulerRuleServiceDeleteAlertSchedulerRule(ctx, id).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			log.Printf("[INFO] alerts-scheduler %s not found, considering deleted", id)
			return
		}
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting alerts-scheduler %s", id),
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Delete", id),
		)
		return
	}
	log.Printf("[INFO] alerts-scheduler %s deleted", id)
}
