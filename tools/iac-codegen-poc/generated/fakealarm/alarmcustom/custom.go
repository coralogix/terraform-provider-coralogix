// Package alarmcustom is handwritten. It has the attributes and converters
// of the fields that spec/fake/alarm.overrides.yaml marks custom (D21):
// their Terraform shape has no generator rule, as alerts of_the_last and
// latency_threshold_ms. The generated package generated/fakealarm calls
// them; this package must not import it.
package alarmcustom

import (
	"fmt"
	"math/big"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk/go/openapi/gen/fake_boards_service"
)

// windowValues are the Terraform names of the fixed time windows.
var windowValues = map[string]sdk.TimeWindowValue{
	"5_MINUTES": sdk.TIMEWINDOWVALUE_TIME_WINDOW_VALUE_MINUTES_5_OR_UNSPECIFIED,
	"1_HOUR":    sdk.TIMEWINDOWVALUE_TIME_WINDOW_VALUE_HOURS_1,
}

var duration = regexp.MustCompile(`^[0-9]+s$`)

// MetricRuleOfTheLastAttribute is one string for the API oneOf TimeWindow:
// a fixed time window or a duration.
func MetricRuleOfTheLastAttribute() schema.Attribute {
	return schema.StringAttribute{
		Optional:            true,
		MarkdownDescription: "The time window: 5_MINUTES, 1_HOUR, or a duration in seconds, for example 90s.",
		Validators: []validator.String{stringvalidator.Any(
			stringvalidator.OneOf("5_MINUTES", "1_HOUR"),
			stringvalidator.RegexMatches(duration, "must be a duration in seconds, for example 90s"),
		)},
	}
}

// ExpandMetricRuleOfTheLast sends a fixed time window for its name, and a
// duration otherwise.
func ExpandMetricRuleOfTheLast(_ path.Path, v types.String, _ *diag.Diagnostics) *sdk.TimeWindow {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	if w, ok := windowValues[v.ValueString()]; ok {
		return &sdk.TimeWindow{SpecificValue: &w}
	}
	d := v.ValueString()
	return &sdk.TimeWindow{DynamicDuration: &d}
}

// FlattenMetricRuleOfTheLast reads either arm into the string.
func FlattenMetricRuleOfTheLast(p path.Path, v *sdk.TimeWindow, diags *diag.Diagnostics) types.String {
	switch {
	case v == nil:
		return types.StringNull()
	case v.SpecificValue != nil:
		for name, w := range windowValues {
			if w == *v.SpecificValue {
				return types.StringValue(name)
			}
		}
		diags.AddAttributeError(p, "Unknown API value", fmt.Sprintf("The API returned the time window %q, which has no Terraform name.", *v.SpecificValue))
		return types.StringNull()
	case v.DynamicDuration != nil:
		return types.StringValue(*v.DynamicDuration)
	}
	return types.StringNull()
}

// TracingRuleLatencyMsAttribute is a Number, for the decimal string of the
// API.
func TracingRuleLatencyMsAttribute() schema.Attribute {
	return schema.NumberAttribute{Optional: true, MarkdownDescription: "The latency in milliseconds, a whole number."}
}

// ExpandTracingRuleLatencyMs sends the whole number as a decimal string.
func ExpandTracingRuleLatencyMs(p path.Path, v types.Number, diags *diag.Diagnostics) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	n := v.ValueBigFloat()
	if !n.IsInt() || n.Sign() < 0 {
		diags.AddAttributeError(p, "Invalid value", fmt.Sprintf("%s is not a whole number of at least 0.", n.String()))
		return nil
	}
	s := n.Text('f', 0)
	return &s
}

// FlattenTracingRuleLatencyMs parses the decimal string.
func FlattenTracingRuleLatencyMs(p path.Path, v *string, diags *diag.Diagnostics) types.Number {
	if v == nil {
		return types.NumberNull()
	}
	n, ok := new(big.Float).SetString(*v)
	if !ok || !n.IsInt() {
		diags.AddAttributeError(p, "Invalid API value", fmt.Sprintf("The API returned %q, which is not a whole number.", *v))
		return types.NumberNull()
	}
	return types.NumberValue(n)
}
