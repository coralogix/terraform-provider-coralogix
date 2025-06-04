// Copyright 2024 Coralogix Ltd.
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

package coralogix

import (
	"context"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

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
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	_                              resource.ResourceWithConfigure   = &AlertsSchedulerResource{}
	_                              resource.ResourceWithImportState = &AlertsSchedulerResource{}
	protoToSchemaDurationFrequency                                  = map[cxsdk.DurationFrequency]string{
		cxsdk.DurationFrequencyMinute: "minutes",
		cxsdk.DurationFrequencyHour:   "hours",
		cxsdk.DurationFrequencyDay:    "days",
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
	protoToSchemaScheduleOperation = map[cxsdk.ScheduleOperation]string{
		cxsdk.ScheduleOperationActivate:    "active",
		cxsdk.ScheduleOperationUnspecified: "unspecified",
		cxsdk.ScheduleOperationMute:        "mute",
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
	client *cxsdk.AlertSchedulerClient
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

	alertScheduler, diags := extractAlertsScheduler(ctx, plan, nil)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	createAlertSchedulerRequest := &cxsdk.CreateAlertSchedulerRuleRequest{
		AlertSchedulerRule: alertScheduler,
	}
	alertsSchedulerStr := protojson.Format(createAlertSchedulerRequest)
	log.Printf("[INFO] Creating new alerts-scheduler: %s", alertsSchedulerStr)
	createResp, err := r.client.Create(ctx, createAlertSchedulerRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error creating alerts-scheduler",
			utils.FormatRpcErrors(err, cxsdk.CreateAlertSchedulerRuleRPC, alertsSchedulerStr))
		return
	}
	alertScheduler = createResp.GetAlertSchedulerRule()
	log.Printf("[INFO] Submitted new alerts-scheduler: %s", protojson.Format(alertScheduler))

	plan, diags = flattenAlertScheduler(ctx, alertScheduler)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenAlertScheduler(ctx context.Context, scheduler *cxsdk.AlertSchedulerRule) (*AlertsSchedulerResourceModel, diag.Diagnostics) {
	metaLabels, diags := flattenAlertsSchedulerMetaLabels(ctx, scheduler.GetMetaLabels())
	if diags.HasError() {
		return nil, diags
	}

	filter, diags := flattenFilter(ctx, scheduler.GetFilter())
	if diags.HasError() {
		return nil, diags
	}

	schedule, diags := flattenSchedule(ctx, scheduler.GetSchedule())
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

func flattenAlertsSchedulerMetaLabels(ctx context.Context, labels []*cxsdk.MetaLabel) (types.Set, diag.Diagnostics) {
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

func flattenFilter(ctx context.Context, filter *cxsdk.AlertSchedulerFilter) (types.Object, diag.Diagnostics) {
	if filter == nil {
		return types.ObjectNull(filterModelAttr()), nil
	}

	var filterModel FilterModel
	switch filterType := filter.WhichAlerts.(type) {
	case *cxsdk.AlertSchedulerFilterMetaLabels:
		metaLabels, diags := flattenAlertsSchedulerMetaLabels(ctx, filterType.AlertMetaLabels.GetValue())
		if diags.HasError() {
			return types.ObjectNull(filterModelAttr()), diags
		}
		filterModel.MetaLabels = metaLabels
		filterModel.AlertsUniqueIDs = types.SetNull(types.StringType)
	case *cxsdk.AlertSchedulerFilterUniqueIDs:
		filterModel.AlertsUniqueIDs = utils.StringSliceToTypeStringSet(filterType.AlertUniqueIds.GetValue())
		filterModel.MetaLabels = types.SetNull(types.ObjectType{AttrTypes: labelModelAttr()})
	default:
		return types.ObjectNull(filterModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten filter", fmt.Sprintf("unknown filter type: %T", filterType))}
	}

	filterModel.WhatExpression = types.StringValue(filter.GetWhatExpression())

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

func flattenSchedule(ctx context.Context, schedule *cxsdk.Schedule) (types.Object, diag.Diagnostics) {
	if schedule == nil {
		return types.ObjectNull(scheduleModelAttr()), nil
	}

	var scheduleModel ScheduleModel
	scheduleModel.Operation = types.StringValue(protoToSchemaScheduleOperation[schedule.GetScheduleOperation()])
	switch scheduleType := schedule.Scheduler.(type) {
	case *cxsdk.ScheduleOneTime:
		oneTime, diags := flattenOneTime(ctx, scheduleType.OneTime)
		if diags.HasError() {
			return types.ObjectNull(scheduleModelAttr()), diags
		}
		scheduleModel.OneTime = oneTime
		scheduleModel.Recurring = types.ObjectNull(recurringModelAttr())
	case *cxsdk.ScheduleRecurring:
		recurring, diags := flattenRecurring(ctx, scheduleType.Recurring)
		if diags.HasError() {
			return types.ObjectNull(scheduleModelAttr()), diags
		}
		scheduleModel.Recurring = recurring
		scheduleModel.OneTime = types.ObjectNull(oneTimeModelAttr())
	default:
		return types.ObjectNull(scheduleModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten schedule", fmt.Sprintf("unknown filter type: %T", scheduleType))}
	}

	return types.ObjectValueFrom(ctx, scheduleModelAttr(), scheduleModel)
}

func flattenRecurring(ctx context.Context, recurring *cxsdk.Recurring) (types.Object, diag.Diagnostics) {
	if recurring == nil {
		return types.ObjectNull(recurringModelAttr()), nil
	}

	var recurringModel RecurringModel
	dynamic, diags := flattenDynamic(ctx, recurring.GetDynamic())
	if diags.HasError() {
		return types.ObjectNull(recurringModelAttr()), diags
	}
	recurringModel.Dynamic = dynamic

	return types.ObjectValueFrom(ctx, recurringModelAttr(), recurringModel)
}

func flattenDynamic(ctx context.Context, dynamic *cxsdk.RecurringDynamicInner) (types.Object, diag.Diagnostics) {
	if dynamic == nil {
		return types.ObjectNull(dynamicModelAttr()), nil
	}

	frequency, diags := flattenFrequency(ctx, dynamic)
	if diags.HasError() {
		return types.ObjectNull(dynamicModelAttr()), diags
	}

	timeFrame, diags := flattenAlertsSchedulerTimeFrame(ctx, dynamic.GetTimeframe())
	if diags.HasError() {
		return types.ObjectNull(dynamicModelAttr()), diags
	}

	dynamicModel := DynamicModel{
		RepeatEvery:    types.Int64Value(int64(dynamic.GetRepeatEvery())),
		Frequency:      frequency,
		TimeFrame:      timeFrame,
		TerminationDay: types.StringValue(dynamic.GetTerminationDate()),
	}

	return types.ObjectValueFrom(ctx, dynamicModelAttr(), dynamicModel)
}

func flattenFrequency(ctx context.Context, dynamic *cxsdk.RecurringDynamicInner) (types.Object, diag.Diagnostics) {
	if dynamic == nil {
		return types.ObjectNull(frequencyModelAttr()), nil
	}

	var frequencyModel FrequencyModel
	switch frequencyType := dynamic.GetFrequency().(type) {
	case *cxsdk.RecurringDynamicDaily:
		frequencyModel.Daily = types.ObjectNull(map[string]attr.Type{})
		frequencyModel.Weekly = types.ObjectNull(weeklyModelAttr())
		frequencyModel.Monthly = types.ObjectNull(monthlyModelAttr())
	case *cxsdk.RecurringDynamicWeekly:
		weekly, diags := flattenWeekly(ctx, frequencyType.Weekly)
		if diags.HasError() {
			return types.ObjectNull(frequencyModelAttr()), diags
		}
		frequencyModel.Weekly = weekly
		frequencyModel.Daily = types.ObjectNull(map[string]attr.Type{})
		frequencyModel.Monthly = types.ObjectNull(monthlyModelAttr())
	case *cxsdk.RecurringDynamicMonthly:
		monthly, diags := flattenMonthly(ctx, frequencyType.Monthly)
		if diags.HasError() {
			return types.ObjectNull(frequencyModelAttr()), diags
		}
		frequencyModel.Monthly = monthly
		frequencyModel.Daily = types.ObjectNull(map[string]attr.Type{})
		frequencyModel.Weekly = types.ObjectNull(weeklyModelAttr())
	default:
		return types.ObjectNull(frequencyModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten frequency", fmt.Sprintf("unknown filter type: %T", frequencyType))}
	}

	return types.ObjectValueFrom(ctx, frequencyModelAttr(), frequencyModel)
}

func flattenWeekly(ctx context.Context, weekly *cxsdk.Weekly) (types.Object, diag.Diagnostics) {
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

func flattenMonthly(ctx context.Context, monthly *cxsdk.Monthly) (types.Object, diag.Diagnostics) {
	if monthly == nil {
		return types.ObjectNull(monthlyModelAttr()), nil
	}

	monthlyModel := MonthlyModel{
		Days: utils.Int32SliceToTypeInt64Set(monthly.GetDaysOfMonth()),
	}

	return types.ObjectValueFrom(ctx, monthlyModelAttr(), monthlyModel)
}

func flattenOneTime(ctx context.Context, time *cxsdk.OneTime) (types.Object, diag.Diagnostics) {
	if time == nil {
		return types.ObjectNull(oneTimeModelAttr()), nil
	}

	timeFrame, diags := flattenAlertsSchedulerTimeFrame(ctx, time.GetTimeframe())
	if diags.HasError() {
		return types.ObjectNull(oneTimeModelAttr()), diags
	}

	oneTimeModel := OneTimeModel{
		TimeFrame: timeFrame,
	}

	return types.ObjectValueFrom(ctx, oneTimeModelAttr(), oneTimeModel)
}

func flattenAlertsSchedulerTimeFrame(ctx context.Context, timeFrame *cxsdk.Timeframe) (types.Object, diag.Diagnostics) {
	if timeFrame == nil {
		return types.ObjectNull(timeFrameModelAttr()), nil
	}

	var timeFrameModel TimeFrameModel
	timeFrameModel.StartTime = types.StringValue(timeFrame.GetStartTime())
	timeFrameModel.TimeZone = types.StringValue(timeFrame.GetTimezone())
	switch untilType := timeFrame.GetUntil().(type) {
	case *cxsdk.TimeframeEndTime:
		timeFrameModel.EndTime = types.StringValue(untilType.EndTime)
		timeFrameModel.Duration = types.ObjectNull(durationModelAttr())
	case *cxsdk.TimeframeDuration:
		var diags diag.Diagnostics
		timeFrameModel.Duration, diags = flattenAlertsSchedulerDuration(ctx, untilType.Duration)
		if diags.HasError() {
			return types.ObjectNull(timeFrameModelAttr()), diags
		}
		timeFrameModel.EndTime = types.StringNull()
	default:
		return types.ObjectNull(timeFrameModelAttr()), diag.Diagnostics{diag.NewErrorDiagnostic("error flatten time frame", fmt.Sprintf("unknown filter type: %T", untilType))}
	}

	return types.ObjectValueFrom(ctx, timeFrameModelAttr(), timeFrameModel)
}

func flattenAlertsSchedulerDuration(ctx context.Context, duration *cxsdk.AlertSchedulerDuration) (types.Object, diag.Diagnostics) {
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

func extractAlertsScheduler(ctx context.Context, plan *AlertsSchedulerResourceModel, id *string) (*cxsdk.AlertSchedulerRule, diag.Diagnostics) {
	metaLabels, diags := extractAlertsSchedulerMetaLabels(ctx, plan.MetaLabels)
	if diags.HasError() {
		return nil, diags
	}

	filter, diags := extractFilter(ctx, plan.Filter)
	if diags.HasError() {
		return nil, diags
	}

	schedule, diags := extractSchedule(ctx, plan.Schedule)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.AlertSchedulerRule{
		UniqueIdentifier: id,
		Name:             plan.Name.ValueString(),
		Description:      utils.TypeStringToStringPointer(plan.Description),
		MetaLabels:       metaLabels,
		Filter:           filter,
		Schedule:         schedule,
		Enabled:          plan.Enabled.ValueBool(),
	}, nil
}

func extractAlertsSchedulerMetaLabels(ctx context.Context, labels types.Set) ([]*cxsdk.MetaLabel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var labelsObjects []types.Object
	var expandedLabels []*cxsdk.MetaLabel
	labels.ElementsAs(ctx, &labelsObjects, true)

	for _, lo := range labelsObjects {
		var label MetaLabelModel
		if dg := lo.As(ctx, &label, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedLabel := &cxsdk.MetaLabel{
			Key:   label.Key.ValueString(),
			Value: utils.TypeStringToStringPointer(label.Value),
		}
		expandedLabels = append(expandedLabels, expandedLabel)
	}

	return expandedLabels, diags
}

func extractFilter(ctx context.Context, filter types.Object) (*cxsdk.AlertSchedulerFilter, diag.Diagnostics) {
	if filter.IsNull() || filter.IsUnknown() {
		return nil, nil
	}

	var filterModel FilterModel
	if diags := filter.As(ctx, &filterModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	whatExpression := filterModel.WhatExpression.ValueString()

	if !(filterModel.AlertsUniqueIDs.IsNull() || filterModel.AlertsUniqueIDs.IsUnknown()) {
		ids, diags := utils.TypeStringSliceToStringSlice(ctx, filterModel.AlertsUniqueIDs.Elements())
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.AlertSchedulerFilter{
			WhatExpression: whatExpression,
			WhichAlerts: &cxsdk.AlertSchedulerFilterUniqueIDs{
				AlertUniqueIds: &cxsdk.AlertUniqueIDs{
					Value: ids,
				},
			},
		}, nil
	} else if !(filterModel.MetaLabels.IsNull() || filterModel.MetaLabels.IsUnknown()) {
		metaLabels, diags := extractAlertsSchedulerMetaLabels(ctx, filterModel.MetaLabels)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.AlertSchedulerFilter{
			WhatExpression: whatExpression,
			WhichAlerts: &cxsdk.AlertSchedulerFilterMetaLabels{
				AlertMetaLabels: &cxsdk.MetaLabels{
					Value: metaLabels,
				},
			},
		}, nil
	}

	return &cxsdk.AlertSchedulerFilter{
		WhatExpression: whatExpression,
		WhichAlerts: &cxsdk.AlertSchedulerFilterUniqueIDs{
			AlertUniqueIds: &cxsdk.AlertUniqueIDs{
				Value: nil,
			},
		},
	}, nil
}

func extractSchedule(ctx context.Context, schedule types.Object) (*cxsdk.Schedule, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(schedule) {
		return nil, nil
	}

	var scheduleModel ScheduleModel
	if diags := schedule.As(ctx, &scheduleModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	scheduler := &cxsdk.Schedule{
		ScheduleOperation: schemaToProtoScheduleOperation[scheduleModel.Operation.ValueString()],
	}

	if !(scheduleModel.OneTime.IsNull() || scheduleModel.OneTime.IsUnknown()) {
		oneTime, diags := extractOneTime(ctx, scheduleModel.OneTime)
		if diags.HasError() {
			return nil, diags
		}
		scheduler.Scheduler = oneTime
		return scheduler, nil
	} else if !(scheduleModel.Recurring.IsNull() || scheduleModel.Recurring.IsUnknown()) {
		recurring, diags := extractRecurring(ctx, scheduleModel.Recurring)
		if diags.HasError() {
			return nil, diags
		}
		scheduler.Scheduler = recurring
		return scheduler, nil
	}

	return nil, nil
}

func extractOneTime(ctx context.Context, oneTimeObject types.Object) (*cxsdk.ScheduleOneTime, diag.Diagnostics) {
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

	return &cxsdk.ScheduleOneTime{
		OneTime: &cxsdk.OneTime{
			Timeframe: timeFrame,
		},
	}, nil
}

func extractTimeFrame(ctx context.Context, timeFrame types.Object) (*cxsdk.Timeframe, diag.Diagnostics) {
	if utils.ObjIsNullOrUnknown(timeFrame) {
		return nil, nil
	}

	var timeFrameModel TimeFrameModel
	if diags := timeFrame.As(ctx, &timeFrameModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	expandedTimeFrame := &cxsdk.Timeframe{
		StartTime: timeFrameModel.StartTime.ValueString(),
		Timezone:  timeFrameModel.TimeZone.ValueString(),
	}
	expandedTimeFrame, diags := expandTimeFrameUntil(ctx, timeFrameModel, expandedTimeFrame)
	if diags.HasError() {
		return nil, diags
	}

	return expandedTimeFrame, nil
}

func expandTimeFrameUntil(ctx context.Context, timeFrameModel TimeFrameModel, expandedTimeFrame *cxsdk.Timeframe) (*cxsdk.Timeframe, diag.Diagnostics) {
	if !(timeFrameModel.Duration.IsNull() || timeFrameModel.Duration.IsUnknown()) {
		duration, diags := extractDuration(ctx, timeFrameModel.Duration)
		if diags.HasError() {
			return nil, diags
		}
		expandedTimeFrame.Until = duration
	} else if !(timeFrameModel.EndTime.IsNull() || timeFrameModel.EndTime.IsUnknown()) {
		expandedTimeFrame.Until = &cxsdk.TimeframeEndTime{
			EndTime: timeFrameModel.EndTime.ValueString(),
		}
	}
	return expandedTimeFrame, nil
}

func extractDuration(ctx context.Context, duration types.Object) (*cxsdk.TimeframeDuration, diag.Diagnostics) {
	if duration.IsNull() || duration.IsUnknown() {
		return nil, nil
	}
	var durationModel DurationModel
	if diags := duration.As(ctx, &durationModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	return &cxsdk.TimeframeDuration{
		Duration: &cxsdk.AlertSchedulerDuration{
			ForOver:   int32(durationModel.ForOver.ValueInt64()),
			Frequency: schemaToProtoDurationFrequency[durationModel.Frequency.ValueString()],
		},
	}, nil
}

func extractRecurring(ctx context.Context, recurring types.Object) (*cxsdk.ScheduleRecurring, diag.Diagnostics) {
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
		return &cxsdk.ScheduleRecurring{
			Recurring: &cxsdk.Recurring{
				Condition: dynamic,
			},
		}, nil
	}

	return &cxsdk.ScheduleRecurring{
		Recurring: &cxsdk.Recurring{
			Condition: &cxsdk.RecurringAlways{
				Always: &cxsdk.RecurringAlwaysInner{},
			},
		},
	}, nil
}

func extractDynamic(ctx context.Context, dynamic types.Object) (*cxsdk.RecurringDynamic, diag.Diagnostics) {
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

	expandedDynamic := &cxsdk.RecurringDynamic{
		Dynamic: &cxsdk.RecurringDynamicInner{
			RepeatEvery:     int32(dynamicModel.RepeatEvery.ValueInt64()),
			Timeframe:       timeFrame,
			TerminationDate: utils.TypeStringToStringPointer(dynamicModel.TerminationDay),
		},
	}
	expandedDynamic.Dynamic, diags = expandFrequency(ctx, expandedDynamic.Dynamic, dynamicModel.Frequency)
	if diags.HasError() {
		return nil, diags
	}

	return expandedDynamic, nil
}

func expandFrequency(ctx context.Context, dynamic *cxsdk.RecurringDynamicInner, frequency types.Object) (*cxsdk.RecurringDynamicInner, diag.Diagnostics) {
	var frequencyModel FrequencyModel
	if diags := frequency.As(ctx, &frequencyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	if daily := frequencyModel.Daily; !(daily.IsNull() || daily.IsUnknown()) {
		dynamic.Frequency = &cxsdk.RecurringDynamicDaily{
			Daily: &cxsdk.Daily{},
		}
	} else if weekly := frequencyModel.Weekly; !(weekly.IsNull() || weekly.IsUnknown()) {
		var weeklyModel WeeklyModel
		if diags := weekly.As(ctx, &weeklyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		days, diags := utils.TypeStringSliceToStringSlice(ctx, weeklyModel.Days.Elements())
		if diags.HasError() {
			return nil, diags
		}
		daysValues := make([]int32, len(days))
		for i, day := range days {
			daysValues[i] = daysToProtoValue[day]
		}

		dynamic.Frequency = &cxsdk.RecurringDynamicWeekly{
			Weekly: &cxsdk.Weekly{
				DaysOfWeek: daysValues,
			},
		}
	} else if monthly := frequencyModel.Monthly; !(monthly.IsNull() || monthly.IsUnknown()) {
		var monthlyModel MonthlyModel
		if diags := monthly.As(ctx, &monthlyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}

		days, diags := utils.TypeInt64SliceToInt32Slice(ctx, monthlyModel.Days.Elements())
		if diags.HasError() {
			return nil, diags
		}

		dynamic.Frequency = &cxsdk.RecurringDynamicMonthly{
			Monthly: &cxsdk.Monthly{
				DaysOfMonth: days,
			},
		}
	}

	return dynamic, nil
}

func (r *AlertsSchedulerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *AlertsSchedulerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed alerts-scheduler value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading alerts-scheduler: %s", id)
	getAlertsSchedulerReq := &cxsdk.GetAlertSchedulerRuleRequest{AlertSchedulerRuleId: id}
	getAlertsSchedulerResp, err := r.client.Get(ctx, getAlertsSchedulerReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("alerts-scheduler %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading alerts-scheduler",
				utils.FormatRpcErrors(err, cxsdk.GetAlertSchedulerRuleRPC, protojson.Format(getAlertsSchedulerReq)),
			)
		}
		return
	}
	alertsScheduler := getAlertsSchedulerResp.GetAlertSchedulerRule()
	log.Printf("[INFO] Received alerts-scheduler: %s", protojson.Format(alertsScheduler))

	state, diags = flattenAlertScheduler(ctx, alertsScheduler)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *AlertsSchedulerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *AlertsSchedulerResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state *AlertsSchedulerResourceModel
	req.State.Get(ctx, &state)
	id := new(string)
	*id = state.ID.ValueString()
	alertsScheduler, diags := extractAlertsScheduler(ctx, plan, id)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	updateAlertsSchedulerReq := &cxsdk.UpdateAlertSchedulerRuleRequest{
		AlertSchedulerRule: alertsScheduler,
	}
	log.Printf("[INFO] Updating alerts-scheduler: %s", protojson.Format(updateAlertsSchedulerReq))
	updateAlertsSchedulerResp, err := r.client.Update(ctx, updateAlertsSchedulerReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating alerts-scheduler",
			utils.FormatRpcErrors(err, cxsdk.UpdateAlertSchedulerRuleRPC, protojson.Format(updateAlertsSchedulerReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated alerts-scheduler: %s", protojson.Format(updateAlertsSchedulerResp))

	// Get refreshed alerts-scheduler value from Coralogix
	getAlertsSchedulerReq := &cxsdk.GetAlertSchedulerRuleRequest{AlertSchedulerRuleId: updateAlertsSchedulerResp.GetAlertSchedulerRule().GetId()}
	getAlertsSchedulerResp, err := r.client.Get(ctx, getAlertsSchedulerReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("alerts-scheduler %s is in state, but no longer exists in Coralogix backend", *id),
				fmt.Sprintf("%s will be recreated when you apply", *id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading alerts-scheduler",
				utils.FormatRpcErrors(err, cxsdk.GetAlertSchedulerRuleRPC, protojson.Format(getAlertsSchedulerReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received alerts-scheduler: %s", protojson.Format(getAlertsSchedulerResp))

	plan, diags = flattenAlertScheduler(ctx, getAlertsSchedulerResp.GetAlertSchedulerRule())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
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
	deleteReq := &cxsdk.DeleteAlertSchedulerRuleRequest{AlertSchedulerRuleId: id}
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting alerts-scheduler %s", id),
			utils.FormatRpcErrors(err, cxsdk.DeleteAlertSchedulerRuleRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] alerts-scheduler %s deleted", id)
}
