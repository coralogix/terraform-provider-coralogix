package dataplans

import (
	"context"
	"strings"
	"testing"

	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExtractTcoPolicyLogWithTargets(t *testing.T) {
	ctx := context.Background()
	targets := logTargetsList(t, ctx, []TCOPolicyLogTargetModel{
		{
			Dataset:                    types.StringValue("logs"),
			Dataspace:                  types.StringValue("default"),
			Priority:                   types.StringValue("high"),
			ArchiveRetentionID:         types.StringNull(),
			QuotaBasedPriorityOverride: types.ObjectNull(quotaBasedPriorityOverrideAttributes()),
		},
		{
			Dataset:                    types.StringValue("logs"),
			Dataspace:                  types.StringValue("payments"),
			Priority:                   types.StringValue("low"),
			ArchiveRetentionID:         types.StringValue("retention-id"),
			QuotaBasedPriorityOverride: quotaOverrideObject(t, ctx),
		},
	})

	request, diags := extractTcoPolicyLog(ctx, baseLogPolicyModel(targets))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if request.Policy.Priority != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED {
		t.Fatalf("expected unspecified top-level priority, got %v", request.Policy.Priority)
	}
	if request.Policy.ArchiveRetention != nil {
		t.Fatalf("expected nil top-level archive retention, got %#v", request.Policy.ArchiveRetention)
	}
	if request.Policy.PriorityOverride != nil {
		t.Fatalf("expected nil top-level priority override, got %#v", request.Policy.PriorityOverride)
	}
	if len(request.Policy.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(request.Policy.Targets))
	}
	if got := request.Policy.Targets[1].GetDataspace(); got != "payments" {
		t.Fatalf("expected second target dataspace payments, got %q", got)
	}
	if request.Policy.Targets[1].PriorityOverride == nil {
		t.Fatal("expected target-level priority override")
	}
}

func TestFlattenTCOLogsPolicyWithTargets(t *testing.T) {
	ctx := context.Background()
	priority := tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW
	dataset := "logs"
	dataspace := "payments"
	policy := tcoPolicys.Policy{
		PolicyLogRules: &tcoPolicys.PolicyLogRules{
			Id:          "policy-id",
			Name:        "targeted-log-policy",
			Enabled:     true,
			LogRules:    tcoPolicys.LogRules{},
			Priority:    tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED,
			Description: stringPointer(""),
			Targets: []tcoPolicys.V1Target{
				{
					Dataset:          &dataset,
					Dataspace:        &dataspace,
					Priority:         &priority,
					PriorityOverride: quotaOverrideAPI(),
				},
			},
		},
	}

	model, diags := flattenTCOLogsPolicy(ctx, policy)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !model.Priority.IsNull() {
		t.Fatalf("expected top-level priority to be null, got %q", model.Priority.ValueString())
	}
	if !model.QuotaBasedPriorityOverride.IsNull() {
		t.Fatal("expected top-level quota override to be null")
	}

	var targets []TCOPolicyLogTargetModel
	if diags := model.Targets.ElementsAs(ctx, &targets, false); diags.HasError() {
		t.Fatalf("unexpected target diagnostics: %v", diags)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if got := targets[0].Dataspace.ValueString(); got != "payments" {
		t.Fatalf("expected dataspace payments, got %q", got)
	}
	if targets[0].QuotaBasedPriorityOverride.IsNull() {
		t.Fatal("expected target-level quota override")
	}
}

func TestExtractTcoPolicyTraceWithTargets(t *testing.T) {
	ctx := context.Background()
	targets := traceTargetsList(t, ctx, []TCOPolicyTraceTargetModel{
		{
			Dataset:            types.StringValue("spans"),
			Dataspace:          types.StringValue("default"),
			Priority:           types.StringValue("high"),
			ArchiveRetentionID: types.StringNull(),
		},
		{
			Dataset:            types.StringValue("spans"),
			Dataspace:          types.StringValue("payments"),
			Priority:           types.StringValue("medium"),
			ArchiveRetentionID: types.StringValue("retention-id"),
		},
	})

	request, diags := extractTcoPolicyTraces(ctx, baseTracePolicyModel(targets))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if request.Policy.Priority != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED {
		t.Fatalf("expected unspecified top-level priority, got %v", request.Policy.Priority)
	}
	if len(request.Policy.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(request.Policy.Targets))
	}
	if got := request.Policy.Targets[1].GetDataspace(); got != "payments" {
		t.Fatalf("expected second target dataspace payments, got %q", got)
	}
}

func TestExtractTcoPolicyLogLegacyCompatibility(t *testing.T) {
	ctx := context.Background()
	model := baseLogPolicyModel(types.ListNull(types.ObjectType{AttrTypes: tcoPolicyLogTargetAttributes()}))
	model.Priority = types.StringValue("medium")
	model.ArchiveRetentionID = types.StringValue("retention-id")
	model.QuotaBasedPriorityOverride = quotaOverrideObject(t, ctx)

	request, diags := extractTcoPolicyLog(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if request.Policy.Priority != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_MEDIUM {
		t.Fatalf("expected medium top-level priority, got %v", request.Policy.Priority)
	}
	if request.Policy.ArchiveRetention == nil {
		t.Fatal("expected top-level archive retention")
	}
	if request.Policy.PriorityOverride == nil {
		t.Fatal("expected top-level priority override")
	}
	if len(request.Policy.Targets) != 0 {
		t.Fatalf("expected no targets, got %d", len(request.Policy.Targets))
	}
}

func TestExtractTcoPolicyTraceLegacyCompatibility(t *testing.T) {
	ctx := context.Background()
	model := baseTracePolicyModel(types.ListNull(types.ObjectType{AttrTypes: tcoPolicyTraceTargetAttributes()}))
	model.Priority = types.StringValue("low")
	model.ArchiveRetentionID = types.StringValue("retention-id")

	request, diags := extractTcoPolicyTraces(ctx, model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if request.Policy.Priority != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW {
		t.Fatalf("expected low top-level priority, got %v", request.Policy.Priority)
	}
	if request.Policy.ArchiveRetention == nil {
		t.Fatal("expected top-level archive retention")
	}
	if len(request.Policy.Targets) != 0 {
		t.Fatalf("expected no targets, got %d", len(request.Policy.Targets))
	}
}

func TestExtractTcoPolicyRejectsAmbiguousAndMissingTargets(t *testing.T) {
	ctx := context.Background()
	targets := logTargetsList(t, ctx, []TCOPolicyLogTargetModel{
		{
			Dataset:                    types.StringValue("logs"),
			Dataspace:                  types.StringValue("default"),
			Priority:                   types.StringValue("high"),
			ArchiveRetentionID:         types.StringNull(),
			QuotaBasedPriorityOverride: types.ObjectNull(quotaBasedPriorityOverrideAttributes()),
		},
	})

	mixedLog := baseLogPolicyModel(targets)
	mixedLog.Priority = types.StringValue("high")
	_, diags := extractTcoPolicyLog(ctx, mixedLog)
	assertDiagnosticContains(t, diags, "cannot mix targets")

	missingLog := baseLogPolicyModel(types.ListNull(types.ObjectType{AttrTypes: tcoPolicyLogTargetAttributes()}))
	_, diags = extractTcoPolicyLog(ctx, missingLog)
	assertDiagnosticContains(t, diags, "must use either targets or legacy top-level priority")

	mixedTrace := baseTracePolicyModel(traceTargetsList(t, ctx, []TCOPolicyTraceTargetModel{
		{
			Dataset:            types.StringValue("spans"),
			Dataspace:          types.StringValue("default"),
			Priority:           types.StringValue("high"),
			ArchiveRetentionID: types.StringNull(),
		},
	}))
	mixedTrace.Priority = types.StringValue("high")
	_, diags = extractTcoPolicyTraces(ctx, mixedTrace)
	assertDiagnosticContains(t, diags, "cannot mix targets")

	missingTrace := baseTracePolicyModel(types.ListNull(types.ObjectType{AttrTypes: tcoPolicyTraceTargetAttributes()}))
	_, diags = extractTcoPolicyTraces(ctx, missingTrace)
	assertDiagnosticContains(t, diags, "must use either targets or legacy top-level priority")
}

func baseLogPolicyModel(targets types.List) TCOPolicyLogsModel {
	return TCOPolicyLogsModel{
		Name:                       types.StringValue("targeted-log-policy"),
		Description:                types.StringValue(""),
		Enabled:                    types.BoolValue(true),
		Priority:                   types.StringNull(),
		Applications:               types.ObjectNull(tcoPolicyRuleAttributes()),
		Subsystems:                 types.ObjectNull(tcoPolicyRuleAttributes()),
		Severities:                 types.SetValueMust(types.StringType, []attr.Value{types.StringValue("info")}),
		ArchiveRetentionID:         types.StringNull(),
		DpxlExpression:             types.StringNull(),
		QuotaBasedPriorityOverride: types.ObjectNull(quotaBasedPriorityOverrideAttributes()),
		Targets:                    targets,
	}
}

func baseTracePolicyModel(targets types.List) TCOPolicyTracesModel {
	return TCOPolicyTracesModel{
		Name:               types.StringValue("targeted-trace-policy"),
		Description:        types.StringValue(""),
		Enabled:            types.BoolValue(true),
		Priority:           types.StringNull(),
		Applications:       types.ObjectNull(tcoPolicyRuleAttributes()),
		Subsystems:         types.ObjectNull(tcoPolicyRuleAttributes()),
		ArchiveRetentionID: types.StringNull(),
		Services:           types.ObjectNull(tcoPolicyRuleAttributes()),
		Actions:            types.ObjectNull(tcoPolicyRuleAttributes()),
		Tags:               types.MapValueMust(types.ObjectType{AttrTypes: tcoPolicyRuleAttributes()}, map[string]attr.Value{}),
		Targets:            targets,
	}
}

func logTargetsList(t *testing.T, ctx context.Context, targets []TCOPolicyLogTargetModel) types.List {
	t.Helper()
	targetsList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: tcoPolicyLogTargetAttributes()}, targets)
	if diags.HasError() {
		t.Fatalf("unexpected log target diagnostics: %v", diags)
	}
	return targetsList
}

func traceTargetsList(t *testing.T, ctx context.Context, targets []TCOPolicyTraceTargetModel) types.List {
	t.Helper()
	targetsList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: tcoPolicyTraceTargetAttributes()}, targets)
	if diags.HasError() {
		t.Fatalf("unexpected trace target diagnostics: %v", diags)
	}
	return targetsList
}

func quotaOverrideObject(t *testing.T, ctx context.Context) types.Object {
	t.Helper()
	usageTiers, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: usageTierAttributes()}, []UsageTierModel{
		{
			DailyQuotaPercentage: types.Float64Value(0.01),
			Priority:             types.StringValue("low"),
		},
		{
			DailyQuotaPercentage: types.Float64Value(0.03),
			Priority:             types.StringValue("medium"),
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected usage tier diagnostics: %v", diags)
	}
	object, diags := types.ObjectValueFrom(ctx, quotaBasedPriorityOverrideAttributes(), QuotaBasedPriorityOverrideModel{
		UsageTiers: usageTiers,
	})
	if diags.HasError() {
		t.Fatalf("unexpected quota override diagnostics: %v", diags)
	}
	return object
}

func quotaOverrideAPI() *tcoPolicys.PriorityOverride {
	low := tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW
	return &tcoPolicys.PriorityOverride{
		QuotaBased: &tcoPolicys.QuotaBased{
			UsageTiers: []tcoPolicys.UsageTier{
				{
					DailyQuotaPercentage: float64Pointer(0.01),
					Priority:             &low,
				},
			},
		},
	}
}

func assertDiagnosticContains(t *testing.T, diags diag.Diagnostics, expected string) {
	t.Helper()
	for _, err := range diags.Errors() {
		if strings.Contains(err.Summary(), expected) || strings.Contains(err.Detail(), expected) {
			return
		}
	}
	t.Fatalf("expected diagnostic containing %q, got %v", expected, diags)
}

func stringPointer(value string) *string {
	return &value
}

func float64Pointer(value float64) *float64 {
	return &value
}
