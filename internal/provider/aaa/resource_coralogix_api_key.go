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

package aaa

import (
	"context"
	"fmt"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/apikey"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"golang.org/x/exp/slices"
)

// NewApiKeyResource returns coralogix_api_key, which is generated from the API contract
// (internal/provider/generated/apikey). Only the state upgrade from schema version 0 is handwritten:
// it translates the roles of that version to permissions, and it makes no call to the API, so a
// hashed key keeps the value that only its state holds.
func NewApiKeyResource() resource.Resource {
	return &apiKeyResource{Resource: apikey.NewResource(apikey.Hooks{AfterRead: keepStoredKeyValue, BeforeWrite: omitUnchangedAccessPolicy}).(*apikey.Resource)}
}

type apiKeyResource struct {
	*apikey.Resource
}

var _ resource.ResourceWithUpgradeState = &apiKeyResource{}

// Update tells the BeforeWrite hook whether the plan changes access_policy, which the generated
// Update cannot tell, because a hook sees the configuration and the body only.
func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.Resource.Update(context.WithValue(ctx, sendAccessPolicyKey{}, accessPolicyChanged(ctx, req.Plan, req.State)), req, resp)
}

func resourceSchemaV0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "ApiKey ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Api Key name.",
			},
			"value": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Api Key value.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"team_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("user_id"),
							),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"user_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("team_id"),
							),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
				},
				Required:            true,
				MarkdownDescription: "Api Key Owner. It can either be a team_id or a user_id ",
			},

			"active": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Api Key Is Active.",
			},
			"hashed": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Api Key Is Hashed.",
			},
			"roles": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Api Key Roles",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
		MarkdownDescription: "Coralogix Api keys. For more info please review - https://coralogix.com/docs/user-guides/account-management/api-keys/api-keys/.",
	}
}

// UpgradeState upgrades the state of schema version 0, which had roles instead of permissions and
// presets, and no organisation owner.
func (r *apiKeyResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := resourceSchemaV0()

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schemaV0,

			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				type ownerV0 struct {
					UserId types.String `tfsdk:"user_id"`
					TeamId types.String `tfsdk:"team_id"`
				}
				type modelV0 struct {
					ID     types.String `tfsdk:"id"`
					Name   types.String `tfsdk:"name"`
					Owner  *ownerV0     `tfsdk:"owner"`
					Active types.Bool   `tfsdk:"active"`
					Hashed types.Bool   `tfsdk:"hashed"`
					Value  types.String `tfsdk:"value"`
					Roles  types.Set    `tfsdk:"roles"`
				}
				var dataV0 modelV0
				resp.Diagnostics.Append(req.State.Get(ctx, &dataV0)...)
				if resp.Diagnostics.HasError() {
					return
				}
				permissions, diags := mapRolesToPermissions(dataV0.Roles)
				if diags.HasError() {
					resp.Diagnostics.Append(diags...)
					return
				}
				if permissions.IsNull() {
					permissions = types.SetValueMust(types.StringType, []attr.Value{})
				}

				// The state starts with a null value for every attribute of the current schema, and the
				// upgrade sets what version 0 had. An attribute that the API contract adds later is null,
				// as it is for a key that no configuration has set, so the upgrade needs no edit for it.
				current, ok := resp.State.Schema.Type().TerraformType(ctx).(tftypes.Object)
				if !ok {
					resp.Diagnostics.AddError("Unexpected schema", "The schema of coralogix_api_key is not an object.")
					return
				}
				nulls := make(map[string]tftypes.Value, len(current.AttributeTypes))
				for name, typ := range current.AttributeTypes {
					nulls[name] = tftypes.NewValue(typ, nil)
				}
				resp.State.Raw = tftypes.NewValue(current, nulls)
				carry := map[string]any{
					"id":          dataV0.ID,
					"name":        dataV0.Name,
					"active":      dataV0.Active,
					"hashed":      dataV0.Hashed,
					"value":       dataV0.Value,
					"permissions": permissions,
				}
				for name, value := range carry {
					resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), value)...)
				}
				if dataV0.Owner != nil {
					resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("owner").AtName("user_id"), dataV0.Owner.UserId)...)
					resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("owner").AtName("team_id"), dataV0.Owner.TeamId)...)
				}
			},
		},
	}
}

func mapRolesToPermissions(roles types.Set) (types.Set, diag.Diagnostics) {
	permissions := []string{}
	for _, role := range roles.Elements() {
		mappedPermissions, diags := mapRoleToPermission(role.(types.String))
		if diags.HasError() {
			return types.SetNull(types.StringType), diags
		}
		for _, m := range mappedPermissions {
			if !slices.Contains(permissions, m) {
				permissions = append(permissions, m)
			}
		}
	}
	return utils.StringSliceToTypeStringSet(permissions), diag.Diagnostics{}
}

func mapRoleToPermission(role types.String) ([]string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	switch r := strings.ToLower(role.ValueString()); r {
	case "rum ingress":
		return []string{"rum-ingress:SendData"}, diags
	case "send data":
		return []string{
			"cloud-metadata-ingress:SendData", "logs.data-ingress:SendData", "metrics.data-ingress:SendData", "spans.data-ingress:SendData"}, diags
	case "coralogix cli":
		return []string{"data-usage:Read", "org-quota:Manage", "org-quota:Read", "org-teams:Manage", "org-teams:ReadConfig", "team-members:Manage", "team-members:ReadConfig", "team-scim:Manage", "team-scim:ReadConfig", "team-sso:Manage", "team-sso:ReadConfig", "team-quota:Manage", "team-quota:Read"}, diags
	case "scim":
		return []string{"team-groups:Manage", "team-groups:ReadConfig", "team-members:Manage", "team-members:ReadConfig", "team-roles:ReadConfig"}, diags
	case "role management":
		return []string{"team-roles:Manage", "team-roles:ReadConfig"}, diags
	case "trigger webhook":
		return []string{"contextual-data:SendData"}, diags
	case "legacy api key":
		return []string{"alerts:ReadConfig", "alerts:UpdateConfig", "cloud-metadata-enrichment:ReadConfig", "cloud-metadata-enrichment:UpdateConfig", "data-usage:Read", "geo-enrichment:ReadConfig", "geo-enrichment:UpdateConfig", "grafana:Read", "grafana:Update", "logs.data-setup#low:ReadConfig", "logs.data-setup#low:UpdateConfig", "logs.events2metrics:ReadConfig", "logs.events2metrics:UpdateConfig", "logs.tco:ReadPolicies", "logs.tco:UpdatePolicies", "metrics.data-analytics#high:Read", "metrics.data-analytics#low:Read", "metrics.data-setup#high:ReadConfig", "metrics.data-setup#high:UpdateConfig", "metrics.data-setup#low:ReadConfig", "metrics.data-setup#low:UpdateConfig", "metrics.recording-rules:ReadConfig", "metrics.recording-rules:UpdateConfig", "metrics.tco:ReadPolicies", "metrics.tco:UpdatePolicies", "outbound-webhooks:ReadConfig", "outbound-webhooks:UpdateConfig", "parsing-rules:ReadConfig", "parsing-rules:UpdateConfig", "security-enrichment:ReadConfig", "security-enrichment:UpdateConfig", "serverless:Read", "service-catalog:ReadDimensionsConfig", "service-catalog:ReadSLIConfig", "service-catalog:UpdateDimensionsConfig", "service-catalog:UpdateSLIConfig", "service-map:Read", "source-mapping:UploadMapping", "spans.data-api#high:ReadData", "spans.data-api#low:ReadData", "spans.data-setup#low:ReadConfig", "spans.data-setup#low:UpdateConfig", "spans.events2metrics:ReadConfig", "spans.events2metrics:UpdateConfig", "spans.tco:ReadPolicies", "spans.tco:UpdatePolicies", "team-actions:ReadConfig", "team-actions:UpdateConfig", "team-api-keys-security-settings:Manage", "team-api-keys-security-settings:ReadConfig", "team-api-keys:Manage", "team-api-keys:ReadConfig", "team-custom-enrichment:ReadConfig", "team-custom-enrichment:ReadData", "team-custom-enrichment:UpdateConfig", "team-custom-enrichment:UpdateData", "team-dashboards:Read", "team-dashboards:Update", "user-actions:ReadConfig", "user-actions:UpdateConfig", "user-dashboards:Read", "user-dashboards:Update", "version-benchmark-tags:Read", "logs.alerts:ReadConfig", "logs.alerts:UpdateConfig", "spans.alerts:ReadConfig", "spans.alerts:UpdateConfig", "metrics.alerts:ReadConfig", "metrics.alerts:UpdateConfig", "livetail:Read", "service-catalog:Read", "version-benchmark-tags:Update", "service-catalog:ReadApdexConfig", "service-catalog:UpdateApdexConfig", "service-catalog:Update", "team-quota:Manage", "team-quota:Read"}, diags
	case "query data legacy":
		return []string{"logs.data-api#high:ReadData", "logs.data-api#low:ReadData", "metrics.data-api#high:ReadData", "metrics.data-api#low:ReadData", "opensearch-dashboards:Read", "opensearch-dashboards:Update", "snowbit.cspm:Read", "snowbit.sspm:Read", "spans.data-api#high:ReadData", "spans.data-api#low:ReadData", "livetail:Read"}, diags
	}
	diags.AddError("Invalid role", fmt.Sprintf("Unable to translate role '%v' into permissions", role))
	return []string{}, diags
}
