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

package dataplans

import (
	"context"
	"testing"

	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestExpandQuotaRuleCreateLog(t *testing.T) {
	ctx := context.Background()
	plan := quotaRuleLogPlan(t, ctx)

	request, diags := expandQuotaRuleCreate(ctx, plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if request.CreatePolicyRequestLogRules == nil {
		t.Fatal("expected log-rule create request")
	}
	if request.CreatePolicyRequestSpanRules != nil {
		t.Fatal("did not expect span-rule create request")
	}

	logRequest := request.CreatePolicyRequestLogRules
	if logRequest.GetName() != "terraform quota rule" {
		t.Fatalf("expected name to round-trip, got %q", logRequest.GetName())
	}
	if logRequest.GetDisabled() {
		t.Fatal("enabled=true should expand to disabled=false")
	}
	if logRequest.GetPriority() != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED {
		t.Fatalf("expected unspecified policy priority when targets are configured, got %s", logRequest.GetPriority())
	}
	if len(logRequest.LogRules.GetSeverities()) != 2 {
		t.Fatalf("expected 2 severities, got %d", len(logRequest.LogRules.GetSeverities()))
	}
	if logRequest.ApplicationRule == nil || logRequest.ApplicationRule.GetName() != "prod" {
		t.Fatalf("expected application rule prod, got %#v", logRequest.ApplicationRule)
	}
	if logRequest.PriorityOverride == nil || logRequest.PriorityOverride.QuotaBased == nil || len(logRequest.PriorityOverride.QuotaBased.UsageTiers) != 1 {
		t.Fatalf("expected top-level quota-based priority override, got %#v", logRequest.PriorityOverride)
	}
	if len(logRequest.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(logRequest.Targets))
	}
	if got := logRequest.Targets[0].GetDataspace(); got != "payments" {
		t.Fatalf("expected target dataspace payments, got %q", got)
	}
	if logRequest.Targets[0].PriorityOverride == nil {
		t.Fatal("expected target quota-based priority override")
	}
}

func TestExpandQuotaRuleCreateSpan(t *testing.T) {
	ctx := context.Background()
	serviceRule := quotaRuleTestRuleObject(t, ctx, "is", []string{"checkout"})
	tagRule := quotaRuleTestRuleObject(t, ctx, "includes", []string{"POST"})
	tagRules := types.MapValueMust(
		types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()},
		map[string]attr.Value{"tags.http.method": tagRule},
	)
	spanRules := types.ObjectValueMust(quotaRuleSpanRulesAttributes(), map[string]attr.Value{
		"service_rule": serviceRule,
		"action_rule":  types.ObjectNull(tcoPolicyRuleAttributes()),
		"tag_rules":    tagRules,
	})
	plan := QuotaRuleModel{
		Name:                       types.StringValue("terraform span quota rule"),
		Description:                types.StringValue("span policy"),
		Enabled:                    types.BoolValue(false),
		Priority:                   types.StringValue("high"),
		ApplicationRule:            types.ObjectNull(tcoPolicyRuleAttributes()),
		SubsystemRule:              types.ObjectNull(tcoPolicyRuleAttributes()),
		ArchiveRetentionID:         types.StringNull(),
		LogRules:                   types.ObjectNull(quotaRuleLogRulesAttributes()),
		SpanRules:                  spanRules,
		QuotaBasedPriorityOverride: types.ObjectNull(quotaBasedPriorityOverrideAttributes()),
		Targets:                    types.ListNull(types.ObjectType{AttrTypes: quotaRuleTargetAttributes()}),
	}

	request, diags := expandQuotaRuleCreate(ctx, plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if request.CreatePolicyRequestSpanRules == nil {
		t.Fatal("expected span-rule create request")
	}

	spanRequest := request.CreatePolicyRequestSpanRules
	if !spanRequest.GetDisabled() {
		t.Fatal("enabled=false should expand to disabled=true")
	}
	if spanRequest.SpanRules.ServiceRule == nil || spanRequest.SpanRules.ServiceRule.GetName() != "checkout" {
		t.Fatalf("expected service rule checkout, got %#v", spanRequest.SpanRules.ServiceRule)
	}
	if len(spanRequest.SpanRules.TagRules) != 1 {
		t.Fatalf("expected 1 tag rule, got %d", len(spanRequest.SpanRules.TagRules))
	}
	if got := spanRequest.SpanRules.TagRules[0].GetTagName(); got != "tags.http.method" {
		t.Fatalf("expected tag name tags.http.method, got %q", got)
	}
	if got := spanRequest.SpanRules.TagRules[0].GetTagValue(); got != "POST" {
		t.Fatalf("expected tag value POST, got %q", got)
	}
}

func TestFlattenQuotaRulePolicyLog(t *testing.T) {
	ctx := context.Background()
	description := "managed by terraform"
	dataspace := "payments"
	dataset := "service_logs"
	targetPriority := tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW

	state, diags := flattenQuotaRulePolicy(ctx, &tcoPolicys.Policy{
		PolicyLogRules: &tcoPolicys.PolicyLogRules{
			Id:          "policy-id",
			Name:        "terraform quota rule",
			Description: &description,
			Enabled:     true,
			Order:       7,
			Priority:    tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_HIGH,
			LogRules: tcoPolicys.LogRules{
				DpxlExpression: stringPtr("<v1> $d.severity == 'INFO'"),
			},
			Targets: []tcoPolicys.V1Target{
				{
					Dataset:   &dataset,
					Dataspace: &dataspace,
					Priority:  &targetPriority,
				},
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if state.ID.ValueString() != "policy-id" {
		t.Fatalf("expected id policy-id, got %q", state.ID.ValueString())
	}
	if state.Priority.ValueString() != "high" {
		t.Fatalf("expected priority high, got %q", state.Priority.ValueString())
	}
	if state.SpanRules.IsNull() != true {
		t.Fatal("span_rules should be null for a log quota rule")
	}

	var logRules QuotaRuleLogRulesModel
	if dgs := state.LogRules.As(ctx, &logRules, basetypes.ObjectAsOptions{}); dgs.HasError() {
		t.Fatalf("unexpected log rule diagnostics: %v", dgs)
	}
	if logRules.DpxlExpression.ValueString() != "<v1> $d.severity == 'INFO'" {
		t.Fatalf("expected dpxl expression to round-trip, got %q", logRules.DpxlExpression.ValueString())
	}

	var targets []QuotaRuleTargetModel
	if dgs := state.Targets.ElementsAs(ctx, &targets, false); dgs.HasError() {
		t.Fatalf("unexpected target diagnostics: %v", dgs)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Dataspace.ValueString() != "payments" {
		t.Fatalf("expected target dataspace payments, got %q", targets[0].Dataspace.ValueString())
	}
	if targets[0].Priority.ValueString() != "low" {
		t.Fatalf("expected target priority low, got %q", targets[0].Priority.ValueString())
	}
}

func TestValidateQuotaRuleRejectsMissingSourceType(t *testing.T) {
	diags := validateQuotaRuleModel(context.Background(), QuotaRuleModel{
		LogRules:  types.ObjectNull(quotaRuleLogRulesAttributes()),
		SpanRules: types.ObjectNull(quotaRuleSpanRulesAttributes()),
	})
	if !diags.HasError() {
		t.Fatal("expected missing source-type diagnostic")
	}
}

func TestValidateQuotaRuleRejectsMultipleSourceTypes(t *testing.T) {
	ctx := context.Background()
	diags := validateQuotaRuleModel(ctx, QuotaRuleModel{
		LogRules:  quotaRuleTestLogRulesObject(t, ctx, []string{"info"}, ""),
		SpanRules: quotaRuleTestEmptySpanRulesObject(),
	})
	if !diags.HasError() {
		t.Fatal("expected multiple source-type diagnostic")
	}
}

func TestValidateQuotaRuleRejectsInvalidLogMatcher(t *testing.T) {
	ctx := context.Background()
	diags := validateQuotaRuleModel(ctx, QuotaRuleModel{
		LogRules:  quotaRuleTestLogRulesObject(t, ctx, []string{"info"}, "<v1> $d.severity == 'INFO'"),
		SpanRules: types.ObjectNull(quotaRuleSpanRulesAttributes()),
	})
	if !diags.HasError() {
		t.Fatal("expected invalid log matcher diagnostic")
	}
}

func TestValidateQuotaRuleRejectsPolicyPriorityWithTargets(t *testing.T) {
	ctx := context.Background()
	diags := validateQuotaRuleModel(ctx, quotaRuleLogPlan(t, ctx))
	if !diags.HasError() {
		t.Fatal("expected policy priority with targets diagnostic")
	}
}

func quotaRuleLogPlan(t *testing.T, ctx context.Context) QuotaRuleModel {
	return QuotaRuleModel{
		Name:                       types.StringValue("terraform quota rule"),
		Description:                types.StringValue("managed by terraform"),
		Enabled:                    types.BoolValue(true),
		Priority:                   types.StringValue("medium"),
		ApplicationRule:            quotaRuleTestRuleObject(t, ctx, "is", []string{"prod"}),
		SubsystemRule:              types.ObjectNull(tcoPolicyRuleAttributes()),
		ArchiveRetentionID:         types.StringValue("archive-retention-id"),
		LogRules:                   quotaRuleTestLogRulesObject(t, ctx, []string{"info", "error"}, ""),
		SpanRules:                  types.ObjectNull(quotaRuleSpanRulesAttributes()),
		QuotaBasedPriorityOverride: quotaRuleTestPriorityOverrideObject(t, ctx),
		Targets: types.ListValueMust(
			types.ObjectType{AttrTypes: quotaRuleTargetAttributes()},
			[]attr.Value{quotaRuleTestTargetObject(t, ctx)},
		),
	}
}

func quotaRuleTestLogRulesObject(t *testing.T, ctx context.Context, severities []string, dpxlExpression string) types.Object {
	t.Helper()

	severityValues := make([]attr.Value, 0, len(severities))
	for _, severity := range severities {
		severityValues = append(severityValues, types.StringValue(severity))
	}
	severitiesSet := types.SetNull(types.StringType)
	if len(severityValues) > 0 {
		severitiesSet = types.SetValueMust(types.StringType, severityValues)
	}
	dpxlValue := types.StringNull()
	if dpxlExpression != "" {
		dpxlValue = types.StringValue(dpxlExpression)
	}

	object, diags := types.ObjectValueFrom(ctx, quotaRuleLogRulesAttributes(), QuotaRuleLogRulesModel{
		Severities:     severitiesSet,
		DpxlExpression: dpxlValue,
	})
	if diags.HasError() {
		t.Fatalf("unexpected log rules diagnostics: %v", diags)
	}
	return object
}

func quotaRuleTestEmptySpanRulesObject() types.Object {
	return types.ObjectValueMust(quotaRuleSpanRulesAttributes(), map[string]attr.Value{
		"service_rule": types.ObjectNull(tcoPolicyRuleAttributes()),
		"action_rule":  types.ObjectNull(tcoPolicyRuleAttributes()),
		"tag_rules":    types.MapNull(types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()}),
	})
}

func quotaRuleTestRuleObject(t *testing.T, ctx context.Context, ruleType string, names []string) types.Object {
	t.Helper()

	nameValues := make([]attr.Value, 0, len(names))
	for _, name := range names {
		nameValues = append(nameValues, types.StringValue(name))
	}
	object, diags := types.ObjectValueFrom(ctx, tcoPolicyRuleAttributes(), TCORuleModel{
		RuleType: types.StringValue(ruleType),
		Names:    types.SetValueMust(types.StringType, nameValues),
	})
	if diags.HasError() {
		t.Fatalf("unexpected rule diagnostics: %v", diags)
	}
	return object
}

func quotaRuleTestPriorityOverrideObject(t *testing.T, ctx context.Context) types.Object {
	t.Helper()

	usageTier := UsageTierModel{
		DailyQuotaPercentage: types.Float64Value(80),
		Priority:             types.StringValue("low"),
	}
	usageTiers, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: usageTierAttributes()}, []UsageTierModel{usageTier})
	if diags.HasError() {
		t.Fatalf("unexpected usage tier diagnostics: %v", diags)
	}
	object, diags := types.ObjectValueFrom(ctx, quotaBasedPriorityOverrideAttributes(), QuotaBasedPriorityOverrideModel{
		UsageTiers: usageTiers,
	})
	if diags.HasError() {
		t.Fatalf("unexpected priority override diagnostics: %v", diags)
	}
	return object
}

func quotaRuleTestTargetObject(t *testing.T, ctx context.Context) types.Object {
	t.Helper()

	object, diags := types.ObjectValueFrom(ctx, quotaRuleTargetAttributes(), QuotaRuleTargetModel{
		Dataset:                    types.StringValue("service_logs"),
		Dataspace:                  types.StringValue("payments"),
		Priority:                   types.StringValue("low"),
		ArchiveRetentionID:         types.StringNull(),
		QuotaBasedPriorityOverride: quotaRuleTestPriorityOverrideObject(t, ctx),
	})
	if diags.HasError() {
		t.Fatalf("unexpected target diagnostics: %v", diags)
	}
	return object
}

func stringPtr(value string) *string {
	return &value
}
