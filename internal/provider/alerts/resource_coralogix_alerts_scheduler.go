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
	"time"

	alertscheduler "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_scheduler_rule_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alertsscheduler"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NewAlertsSchedulerResource returns the generated coralogix_alerts_scheduler resource. The hooks
// keep the released handling of start_time and time_zone, which the API stores in other forms.
func NewAlertsSchedulerResource() resource.Resource {
	return alertsscheduler.NewResource(alertsscheduler.Hooks{
		BeforeWrite:   sendUTCAsUTCPlus0,
		BeforeFlatten: normalizeStartTimes,
		AfterRead:     keepTimeFrameSpelling,
	})
}

// timeFramePaths are the Terraform paths of the two time frames of a rule.
var timeFramePaths = []path.Path{
	path.Root("schedule").AtName("one_time").AtName("time_frame"),
	path.Root("schedule").AtName("recurring").AtName("dynamic").AtName("time_frame"),
}

// ruleTimeframes returns the time frames that the rule holds.
func ruleTimeframes(rule *alertscheduler.AlertSchedulerRule) []*alertscheduler.Timeframe {
	var out []*alertscheduler.Timeframe
	if oneTime := rule.Schedule.OneTime; oneTime != nil {
		out = append(out, &oneTime.Timeframe)
	}
	if recurring := rule.Schedule.Recurring; recurring != nil && recurring.Schedule != nil {
		out = append(out, &recurring.Schedule.Timeframe)
	}
	return out
}

// sendUTCAsUTCPlus0 sends time_zone UTC as UTC+0, the spelling that the API stores.
func sendUTCAsUTCPlus0(_ context.Context, _ tfsdk.Config, body any) diag.Diagnostics {
	var rule *alertscheduler.AlertSchedulerRule
	switch b := body.(type) {
	case *alertscheduler.CreateAlertSchedulerRuleRequestDataStructure:
		rule = &b.AlertSchedulerRule
	case *alertscheduler.UpdateAlertSchedulerRuleRequestDataStructure:
		rule = &b.AlertSchedulerRule
	default:
		return diag.Diagnostics{diag.NewErrorDiagnostic("Unexpected request body", fmt.Sprintf("The alerts scheduler request body is %T.", body))}
	}
	for _, tf := range ruleTimeframes(rule) {
		if tf.Timezone == "UTC" {
			tf.Timezone = "UTC+0"
		}
	}
	return nil
}

// normalizeStartTimes reads each start time in the form that the API returns, without a zone.
func normalizeStartTimes(_ context.Context, rule *alertscheduler.AlertSchedulerRule) diag.Diagnostics {
	for _, tf := range ruleTimeframes(rule) {
		if tf.StartTime != nil {
			normalized := normalizeStartTimeFromAPI(*tf.StartTime)
			tf.StartTime = &normalized
		}
	}
	return nil
}

// priorData is the plan after Create or Update, or the state before Read.
type priorData interface {
	GetAttribute(ctx context.Context, p path.Path, target interface{}) diag.Diagnostics
}

// keepTimeFrameSpelling keeps the start_time and time_zone of prior when the API returned the same
// instant or zone in another form: a start time with or without Z, or UTC as UTC+0.
func keepTimeFrameSpelling(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	data, ok := prior.(priorData)
	if !ok || data == nil {
		return nil
	}
	var diags diag.Diagnostics
	for _, frame := range timeFramePaths {
		diags.Append(keepEquivalent(ctx, state, data, frame.AtName("start_time"), startTimeSemanticallyEqual)...)
		diags.Append(keepEquivalent(ctx, state, data, frame.AtName("time_zone"), timeZoneEquivalent)...)
	}
	return diags
}

// keepEquivalent sets the state string at p to the prior one when equal says that they are the same.
func keepEquivalent(ctx context.Context, state *tfsdk.State, prior priorData, p path.Path, equal func(prior, state string) bool) diag.Diagnostics {
	var current, previous types.String
	if diags := state.GetAttribute(ctx, p, &current); diags.HasError() {
		return nil // the time frame is not set
	}
	if diags := prior.GetAttribute(ctx, p, &previous); diags.HasError() {
		return nil
	}
	if current.IsNull() || current.IsUnknown() || previous.IsNull() || previous.IsUnknown() || current.Equal(previous) {
		return nil
	}
	if !equal(previous.ValueString(), current.ValueString()) {
		return nil
	}
	return state.SetAttribute(ctx, p, previous)
}

const (
	startTimeFormatCanonical   = "2006-01-02T15:04:05.000" // output format and parse format without Z
	startTimeFormatWithMillisZ = "2006-01-02T15:04:05.000Z"
	startTimeFormatZ           = "2006-01-02T15:04:05Z"
	startTimeFormatNoMillis    = "2006-01-02T15:04:05" // fallback parse format
)

// startTimeFormats are formats the API may return or users may send for start_time.
var startTimeFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	startTimeFormatWithMillisZ,
	startTimeFormatCanonical,
	startTimeFormatZ,
}

// normalizeStartTimeFromAPI parses start_time from the API and returns it in the format the API uses (no Z suffix).
func normalizeStartTimeFromAPI(s string) string {
	if s == "" {
		return s
	}
	if t, ok := parseStartTime(s); ok {
		return t.Format(startTimeFormatCanonical)
	}
	return s
}

func parseStartTime(s string) (t time.Time, ok bool) {
	for _, layout := range startTimeFormats {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed.UTC(), true
		}
	}
	if parsed, err := time.Parse(startTimeFormatNoMillis, s); err == nil {
		return parsed.UTC(), true
	}
	return time.Time{}, false
}

// startTimeSemanticallyEqual returns true if a and b represent the same instant (for comparison with API response).
func startTimeSemanticallyEqual(a, b string) bool {
	if a == b {
		return true
	}
	at, okA := parseStartTime(a)
	bt, okB := parseStartTime(b)
	return okA && okB && at.Equal(bt)
}

// timeZoneEquivalent returns true if plan and state represent the same zone (e.g. "UTC" and "UTC+0").
func timeZoneEquivalent(planTZ, stateTZ string) bool {
	if planTZ == stateTZ {
		return true
	}
	return (planTZ == "UTC" && stateTZ == "UTC+0") || (planTZ == "UTC+0" && stateTZ == "UTC")
}
