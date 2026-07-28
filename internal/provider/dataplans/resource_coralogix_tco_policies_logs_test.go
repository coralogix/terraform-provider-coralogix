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

package dataplans

import (
	"context"
	"strings"
	"testing"

	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func tcoNullOverride() types.Object {
	return types.ObjectNull(quotaBasedPriorityOverrideAttributes())
}

func tcoQuotaOverride(t *testing.T) types.Object {
	ctx := context.Background()
	tiers, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: usageTierAttributes()}, []UsageTierModel{
		{DailyQuotaPercentage: types.Float64Value(50), Priority: types.StringValue("medium")},
	})
	if diags.HasError() {
		t.Fatalf("build usage tiers: %v", diags)
	}
	obj, diags := types.ObjectValueFrom(ctx, quotaBasedPriorityOverrideAttributes(), QuotaBasedPriorityOverrideModel{UsageTiers: tiers})
	if diags.HasError() {
		t.Fatalf("build override: %v", diags)
	}
	return obj
}

func tcoQuotaOverrideWith(t *testing.T, tiers ...UsageTierModel) types.Object {
	ctx := context.Background()
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: usageTierAttributes()}, tiers)
	if diags.HasError() {
		t.Fatalf("build usage tiers: %v", diags)
	}
	obj, diags := types.ObjectValueFrom(ctx, quotaBasedPriorityOverrideAttributes(), QuotaBasedPriorityOverrideModel{UsageTiers: list})
	if diags.HasError() {
		t.Fatalf("build override: %v", diags)
	}
	return obj
}

func tcoTier(percentage float64, priority string) UsageTierModel {
	return UsageTierModel{
		DailyQuotaPercentage: types.Float64Value(percentage),
		Priority:             types.StringValue(priority),
	}
}

func tcoTarget(dataset, dataspace string, priority types.String, override types.Object) TCOTargetModel {
	return TCOTargetModel{
		Dataset:                    types.StringValue(dataset),
		Dataspace:                  types.StringValue(dataspace),
		Priority:                   priority,
		ArchiveRetentionID:         types.StringNull(),
		QuotaBasedPriorityOverride: override,
	}
}

func tcoTargetsList(t *testing.T, targets ...TCOTargetModel) types.List {
	ctx := context.Background()
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: tcoTargetAttributes()}, targets)
	if diags.HasError() {
		t.Fatalf("build targets list: %v", diags)
	}
	return list
}

func TestExtractTcoPolicyLogWithTargets(t *testing.T) {
	ctx := context.Background()
	plan := TCOPolicyLogsModel{
		Name:                       types.StringValue("multi-target"),
		Description:                types.StringValue("routes to two targets"),
		Enabled:                    types.BoolValue(true),
		Priority:                   types.StringNull(),
		Applications:               types.ObjectNull(tcoPolicyRuleAttributes()),
		Subsystems:                 types.ObjectNull(tcoPolicyRuleAttributes()),
		Severities:                 types.SetNull(types.StringType),
		ArchiveRetentionID:         types.StringNull(),
		DpxlExpression:             types.StringNull(),
		QuotaBasedPriorityOverride: tcoNullOverride(),
		Targets: tcoTargetsList(t,
			tcoTarget("logs", "default", types.StringValue("high"), tcoNullOverride()),
			tcoTarget("audit_logs", "default", types.StringValue("low"), tcoNullOverride()),
		),
	}

	rq, diags := extractTcoPolicyLog(ctx, plan)
	if diags.HasError() {
		t.Fatalf("extractTcoPolicyLog: %v", diags)
	}

	if rq.Policy.Priority != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_UNSPECIFIED {
		t.Fatalf("policy-level priority = %q, want UNSPECIFIED when targets are set", rq.Policy.Priority)
	}
	if rq.Policy.ArchiveRetention != nil || rq.Policy.PriorityOverride != nil {
		t.Fatalf("policy-level archive/override must be nil when targets are set")
	}
	if got := len(rq.Policy.Targets); got != 2 {
		t.Fatalf("targets len = %d, want 2", got)
	}
	if got := rq.Policy.Targets[0].GetDataset(); got != "logs" {
		t.Fatalf("targets[0].dataset = %q, want logs", got)
	}
	if got := rq.Policy.Targets[0].GetPriority(); got != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_HIGH {
		t.Fatalf("targets[0].priority = %q, want HIGH", got)
	}
	if got := rq.Policy.Targets[1].GetDataset(); got != "audit_logs" {
		t.Fatalf("targets[1].dataset = %q, want audit_logs", got)
	}
	if got := rq.Policy.Targets[1].GetPriority(); got != tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW {
		t.Fatalf("targets[1].priority = %q, want LOW", got)
	}
}

func TestFlattenTCOTargetsRoundTrip(t *testing.T) {
	ctx := context.Background()
	high := tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_HIGH
	low := tcoPolicys.QUOTAV1PRIORITY_PRIORITY_TYPE_LOW
	logs := "logs"
	auditLogs := "audit_logs"
	dataspace := "default"

	targets, diags := flattenTCOTargets(ctx, []tcoPolicys.V1Target{
		{Dataset: &logs, Dataspace: &dataspace, Priority: &high},
		{Dataset: &auditLogs, Priority: &low},
	})
	if diags.HasError() {
		t.Fatalf("flattenTCOTargets: %v", diags)
	}

	var models []TCOTargetModel
	if diags := targets.ElementsAs(ctx, &models, false); diags.HasError() {
		t.Fatalf("ElementsAs: %v", diags)
	}
	if len(models) != 2 {
		t.Fatalf("targets len = %d, want 2", len(models))
	}
	if models[0].Dataset.ValueString() != "logs" || models[0].Priority.ValueString() != "high" {
		t.Fatalf("targets[0] = %+v, want logs/high", models[0])
	}
	if models[1].Dataspace.ValueString() != "default" {
		t.Fatalf("targets[1].dataspace = %q, want default fallback", models[1].Dataspace.ValueString())
	}
	if models[1].Dataset.ValueString() != "audit_logs" || models[1].Priority.ValueString() != "low" {
		t.Fatalf("targets[1] = %+v, want audit_logs/low", models[1])
	}

	empty, diags := flattenTCOTargets(ctx, nil)
	if diags.HasError() {
		t.Fatalf("flattenTCOTargets(nil): %v", diags)
	}
	if !empty.IsNull() {
		t.Fatalf("flattenTCOTargets(nil) = %v, want null", empty)
	}
}

func TestValidateTCOTargets(t *testing.T) {
	ctx := context.Background()
	nullOverride := tcoNullOverride()
	overrideSet := tcoQuotaOverride(t)
	noTargets := types.ListNull(types.ObjectType{AttrTypes: tcoTargetAttributes()})
	unknownTargets := types.ListUnknown(types.ObjectType{AttrTypes: tcoTargetAttributes()})

	cases := []struct {
		name         string
		model        TCOPolicyLogsModel
		wantErr      bool
		wantContains string
	}{
		{
			name: "no targets and no priority is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets:                    noTargets,
			},
			wantErr:      true,
			wantContains: "required when `targets` is not set",
		},
		{
			name: "no targets with a policy-level priority is accepted",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("high"),
				QuotaBasedPriorityOverride: nullOverride,
				Targets:                    noTargets,
			},
			wantErr: false,
		},
		{
			name: "policy-level quota override without a fallback priority is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: overrideSet,
				Targets:                    noTargets,
			},
			wantErr:      true,
			wantContains: "required when `targets` is not set",
		},
		{
			name: "multi-target routing with per-target priorities is accepted",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("high"), nullOverride),
					tcoTarget("audit_logs", "default", types.StringValue("low"), nullOverride),
				),
			},
			wantErr: false,
		},
		{
			name: "priority at both the policy level and per-target is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("high"),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("low"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "must not be set at the policy level",
		},
		{
			name: "policy-level archive_retention_id with targets is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				ArchiveRetentionID:         types.StringValue("retention-id"),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("high"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "must not be set at the policy level",
		},
		{
			name: "duplicate targets by dataspace and dataset are rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("high"), nullOverride),
					tcoTarget("logs", "default", types.StringValue("low"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "unique",
		},
		{
			name: "a target with a quota override but no priority is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringNull(), overrideSet),
				),
			},
			wantErr:      true,
			wantContains: "Every target must set `priority`",
		},
		{
			name: "a target with no priority is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringNull(), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "Every target must set `priority`",
		},
		{
			name: "unknown targets defers validation even without a policy-level priority",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets:                    unknownTargets,
			},
			wantErr: false,
		},
		{
			name: "unknown policy-level priority with no targets defers validation",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringUnknown(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets:                    noTargets,
			},
			wantErr: false,
		},
		{
			name: "target with an unknown priority defers the priority-presence check",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringUnknown(), nullOverride),
				),
			},
			wantErr: false,
		},
		{
			name: "target with an unknown dataspace defers the uniqueness check",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					TCOTargetModel{
						Dataset:                    types.StringValue("logs"),
						Dataspace:                  types.StringUnknown(),
						Priority:                   types.StringValue("high"),
						ArchiveRetentionID:         types.StringNull(),
						QuotaBasedPriorityOverride: nullOverride,
					},
					tcoTarget("logs", "default", types.StringValue("low"), nullOverride),
				),
			},
			wantErr: false,
		},
		{
			name: "high priority on a dataset other than logs is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("audit_logs", "default", types.StringValue("high"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "only available when routing to",
		},
		{
			name: "block priority on a dataset other than logs is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("audit_logs", "default", types.StringValue("block"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "only available when routing to",
		},
		{
			name: "a usage tier escalating to block on a dataset other than logs is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("audit_logs", "default", types.StringValue("low"),
						tcoQuotaOverrideWith(t, tcoTier(80, "block"))),
				),
			},
			wantErr:      true,
			wantContains: "only available when routing to",
		},
		{
			name: "an unknown dataset defers the per-dataset priority check",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					TCOTargetModel{
						Dataset:                    types.StringUnknown(),
						Dataspace:                  types.StringValue("default"),
						Priority:                   types.StringValue("block"),
						ArchiveRetentionID:         types.StringNull(),
						QuotaBasedPriorityOverride: nullOverride,
					},
				),
			},
			wantErr: false,
		},
		{
			name: "a policy-level fallback less restrictive than the last tier is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("medium"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, tcoTier(80, "low")),
				Targets:                    noTargets,
			},
			wantErr:      true,
			wantContains: "at least as restrictive as",
		},
		{
			name: "a per-target fallback less restrictive than the last tier is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("medium"),
						tcoQuotaOverrideWith(t, tcoTier(80, "low"))),
				),
			},
			wantErr:      true,
			wantContains: "at least as restrictive as",
		},
		{
			name: "a fallback equal to the last tier is accepted",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("low"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, tcoTier(80, "low")),
				Targets:                    noTargets,
			},
			wantErr: false,
		},
		{
			name: "a fallback more restrictive than the last tier is accepted",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("block"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, tcoTier(50, "medium"), tcoTier(80, "low")),
				Targets:                    noTargets,
			},
			wantErr: false,
		},
		{
			name: "descending usage tiers are rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("block"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, tcoTier(80, "medium"), tcoTier(50, "low")),
				Targets:                    noTargets,
			},
			wantErr:      true,
			wantContains: "ascending `daily_quota_percentage`",
		},
		{
			// The second target satisfies "at least one target must set priority", so
			// nothing else masks the missing fallback on the first one.
			name: "an unknown target quota override without a target priority is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("audit_logs", "default", types.StringNull(),
						types.ObjectUnknown(quotaBasedPriorityOverrideAttributes())),
					tcoTarget("logs", "default", types.StringValue("high"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "Every target must set `priority`",
		},
		{
			// priority is Optional-only, so unknown means the user configured it with
			// an apply-time expression. Deferring would let the conflict through and
			// the policy-level value would be silently dropped at apply.
			name: "an unknown policy-level priority alongside targets is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringUnknown(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("high"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "must not be set at the policy level",
		},
		{
			name: "an unknown policy-level archive_retention_id alongside targets is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				ArchiveRetentionID:         types.StringUnknown(),
				QuotaBasedPriorityOverride: nullOverride,
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("high"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "must not be set at the policy level",
		},
		{
			name: "an unknown policy-level quota override alongside targets is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringNull(),
				QuotaBasedPriorityOverride: types.ObjectUnknown(quotaBasedPriorityOverrideAttributes()),
				Targets: tcoTargetsList(t,
					tcoTarget("logs", "default", types.StringValue("high"), nullOverride),
				),
			},
			wantErr:      true,
			wantContains: "must not be set at the policy level",
		},
		{
			name: "usage tiers that relax priority as quota fills are rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("block"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, tcoTier(50, "low"), tcoTier(80, "medium")),
				Targets:                    noTargets,
			},
			wantErr:      true,
			wantContains: "cannot become less restrictive as quota fills",
		},
		{
			name: "a usage tier following a terminal block tier is rejected",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("block"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, tcoTier(50, "block"), tcoTier(80, "block")),
				Targets:                    noTargets,
			},
			wantErr:      true,
			wantContains: "terminal usage tier",
		},
		{
			name: "monotonic usage tiers that repeat then tighten are accepted",
			model: TCOPolicyLogsModel{
				Priority:                   types.StringValue("block"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, tcoTier(25, "medium"), tcoTier(50, "medium"), tcoTier(80, "low")),
				Targets:                    noTargets,
			},
			wantErr: false,
		},
		{
			name: "an unknown tier priority defers the monotonicity check",
			model: TCOPolicyLogsModel{
				Priority: types.StringValue("block"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t,
					tcoTier(25, "low"),
					UsageTierModel{DailyQuotaPercentage: types.Float64Value(50), Priority: types.StringUnknown()},
					tcoTier(80, "medium"),
				),
				Targets: noTargets,
			},
			wantErr: false,
		},
		{
			name: "an unknown tier priority defers the fallback check",
			model: TCOPolicyLogsModel{
				Priority: types.StringValue("medium"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t, UsageTierModel{
					DailyQuotaPercentage: types.Float64Value(80),
					Priority:             types.StringUnknown(),
				}),
				Targets: noTargets,
			},
			wantErr: false,
		},
		{
			name: "an unknown tier percentage defers the ordering check",
			model: TCOPolicyLogsModel{
				Priority: types.StringValue("block"),
				QuotaBasedPriorityOverride: tcoQuotaOverrideWith(t,
					tcoTier(80, "medium"),
					UsageTierModel{DailyQuotaPercentage: types.Float64Unknown(), Priority: types.StringValue("low")},
				),
				Targets: noTargets,
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &resource.ValidateConfigResponse{}
			validateTCOTargets(ctx, tc.model, resp)

			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Fatalf("HasError = %v, want %v; diagnostics: %v", got, tc.wantErr, resp.Diagnostics)
			}
			if tc.wantContains == "" {
				return
			}
			for _, d := range resp.Diagnostics.Errors() {
				if strings.Contains(d.Detail(), tc.wantContains) || strings.Contains(d.Summary(), tc.wantContains) {
					return
				}
			}
			t.Fatalf("no diagnostic contains %q; diagnostics: %v", tc.wantContains, resp.Diagnostics)
		})
	}
}
