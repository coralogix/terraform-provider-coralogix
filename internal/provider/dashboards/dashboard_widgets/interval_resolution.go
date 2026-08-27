// Copyright 2026 Coralogix Ltd.
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
	"fmt"
	"math"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
)

// wireDurationValidator accepts only the form the API returns, because these
// attributes are sent and read back without conversion. The API rewrites
// anything else - `1.5s` becomes `1.500s` and `1.0s` becomes `1s` - which
// Terraform then reports as a change on every plan. Rejecting it at plan time
// names the attribute and the value to write instead.
type wireDurationValidator struct{}

func (v wireDurationValidator) Description(_ context.Context) string {
	return "a duration in seconds as the API writes it, such as 900s, 15s or 1.500s"
}

func (v wireDurationValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v wireDurationValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	duration := new(durationpb.Duration)
	if err := protojson.Unmarshal([]byte(fmt.Sprintf("%q", value)), duration); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute Value",
			fmt.Sprintf("Attribute %s must be a duration in seconds, such as %q or %q, got %q.",
				req.Path, "900s", "1.500s", value),
		)
		return
	}

	// Re-encoding gives the exact form the API stores, so any difference is a
	// value that would come back rewritten.
	encoded, err := protojson.Marshal(duration)
	if err != nil {
		return
	}
	canonical := string(encoded[1 : len(encoded)-1])
	if canonical != value {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute Value",
			fmt.Sprintf("Attribute %s is stored by the API as %q, so %q would be reported as a change on every plan. Write %q instead.",
				req.Path, canonical, value, canonical),
		)
	}
}

// IntervalResolutionSchema is the interval resolution the API exposes on a bar
// chart x-axis and a line chart query definition. At most one of auto and
// manual: the API rejects both together, and accepts neither.
func IntervalResolutionSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"auto": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"maximum_data_points": IntervalResolutionMaximumDataPointsSchema("The most data points to display. The calculated interval keeps within this limit."),
					"minimum_interval":    IntervalResolutionDurationSchema("The smallest interval the calculation may choose."),
				},
				MarkdownDescription: "Let the backend choose the interval, within the constraints below.",
			},
			"manual": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"interval":            IntervalResolutionRequiredDurationSchema("The fixed interval for time buckets."),
					"maximum_data_points": IntervalResolutionMaximumDataPointsSchema("The most data points the selected interval may produce."),
					"minimum_interval":    IntervalResolutionDurationSchema("The smallest interval the selected one may be."),
				},
				MarkdownDescription: "Set the interval yourself.",
			},
			"use_advanced_limit": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether the maximum data points and minimum interval fields are editable in the Coralogix UI. It does not change how the interval is calculated.",
			},
		},
		Validators: []validator.Object{
			AtMostOneOfChildren("auto", "manual"),
		},
		MarkdownDescription: "How time is grouped into buckets. Set at most one of `auto` and `manual`; omit both to leave the choice to the backend.",
	}
}

// IntervalResolutionDurationSchema is an optional duration in the form the API
// writes: a number of seconds with an `s`.
func IntervalResolutionDurationSchema(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		Validators:          []validator.String{wireDurationValidator{}},
		MarkdownDescription: description + " Written as a number of seconds with an `s`, for example `900s` or `15s`, which is the form the API stores and returns.",
	}
}

// IntervalResolutionRequiredDurationSchema is the manual interval, which the API
// declares as a plain field and rejects when empty.
func IntervalResolutionRequiredDurationSchema(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Required:            true,
		Validators:          []validator.String{wireDurationValidator{}},
		MarkdownDescription: description + " Written as a number of seconds with an `s`, for example `900s` or `15s`, which is the form the API stores and returns.",
	}
}

// IntervalResolutionAttr mirrors IntervalResolutionSchema. A schema attribute
// without a matching entry here fails an apply with a Value Conversion Error
// naming only the top-level path.
func IntervalResolutionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"auto": types.ObjectType{AttrTypes: map[string]attr.Type{
			"maximum_data_points": types.Int64Type,
			"minimum_interval":    types.StringType,
		}},
		"manual": types.ObjectType{AttrTypes: map[string]attr.Type{
			"interval":            types.StringType,
			"maximum_data_points": types.Int64Type,
			"minimum_interval":    types.StringType,
		}},
		"use_advanced_limit": types.BoolType,
	}
}

// ExpandIntervalResolution converts the interval resolution. The union is
// re-checked here rather than trusting AtMostOneOfChildren, which returns early
// while either arm is unknown: the API rejects both arms together when the
// request is marshalled, with an error naming generated JSON fields and no HCL
// path.
func ExpandIntervalResolution(m *IntervalResolutionModel) (*dashboardservice.IntervalResolution, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	if m.Auto != nil && m.Manual != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Invalid Attribute Combination",
			"Only one of `auto` and `manual` can be configured. Omit both to leave the choice to the backend.",
		)}
	}

	resolution := &dashboardservice.IntervalResolution{
		UseAdvancedLimit: m.UseAdvancedLimit.ValueBoolPointer(),
	}
	switch {
	case m.Auto != nil:
		resolution.Auto = &dashboardservice.AutoIntervalResolution{
			MaximumDataPoints: expandInt32Pointer(m.Auto.MaximumDataPoints),
			MinimumInterval:   m.Auto.MinimumInterval.ValueStringPointer(),
		}
	case m.Manual != nil:
		resolution.Manual = &dashboardservice.ManualIntervalResolution{
			Interval:          m.Manual.Interval.ValueString(),
			MaximumDataPoints: expandInt32Pointer(m.Manual.MaximumDataPoints),
			MinimumInterval:   m.Manual.MinimumInterval.ValueStringPointer(),
		}
	}

	return resolution, nil
}

// FlattenIntervalResolution reads the interval resolution back. A resolution
// with neither mode is kept, not dropped: omitting both is a documented way to
// leave the choice to the backend, so `time_buckets = {}` is a configuration a
// user can write and it has to survive the read. Only an x-axis with no kind at
// all reads back as absent, because the x-axis requires exactly one kind and a
// block with every child null is state no configuration can produce.
func FlattenIntervalResolution(resolution *dashboardservice.IntervalResolution) *IntervalResolutionModel {
	if resolution == nil {
		return nil
	}

	model := &IntervalResolutionModel{
		UseAdvancedLimit: types.BoolPointerValue(resolution.UseAdvancedLimit),
	}
	switch {
	case resolution.Auto != nil:
		model.Auto = &AutoIntervalResolutionModel{
			MaximumDataPoints: int32PointerToInt64Type(resolution.Auto.MaximumDataPoints),
			MinimumInterval:   types.StringPointerValue(resolution.Auto.MinimumInterval),
		}
	case resolution.Manual != nil:
		model.Manual = &ManualIntervalResolutionModel{
			Interval:          types.StringValue(resolution.Manual.Interval),
			MaximumDataPoints: int32PointerToInt64Type(resolution.Manual.MaximumDataPoints),
			MinimumInterval:   types.StringPointerValue(resolution.Manual.MinimumInterval),
		}
	}

	return model
}

// IntervalResolutionMaximumDataPointsSchema bounds the value to what the API's
// int32 field can hold. The conversion is an unchecked cast, so a larger number
// wraps - 2147483648 arrives as -2147483648 - and the request carries a value
// the user never wrote.
func IntervalResolutionMaximumDataPointsSchema(description string) schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional: true,
		Validators: []validator.Int64{
			int64validator.Between(math.MinInt32, math.MaxInt32),
		},
		MarkdownDescription: description,
	}
}
